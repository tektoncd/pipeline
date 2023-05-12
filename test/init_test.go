//go:build conformance || e2e || examples
// +build conformance e2e examples

/*
Copyright 2019 The Tekton Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// This file contains initialization logic for the tests, such as special magical global state that needs to be initialized.

package test

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/names"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp" // Mysteriously by k8s libs, or they fail to create `KubeClient`s when using oidc authentication. Apparently just importing it is enough. @_@ side effects @_@. https://github.com/kubernetes/client-go/issues/345
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
	"knative.dev/pkg/system"
	knativetest "knative.dev/pkg/test"
	"knative.dev/pkg/test/logging" // Mysteriously by k8s libs, or they fail to create `KubeClient`s from config. Apparently just importing it is enough. @_@ side effects @_@. https://github.com/kubernetes/client-go/issues/242
	"knative.dev/pkg/test/logstream"
	"sigs.k8s.io/yaml"
)

var initMetrics sync.Once
var skipRootUserTests = false

func init() {
	flag.BoolVar(&skipRootUserTests, "skipRootUserTests", false, "Skip tests that require root user")
}

func setup(ctx context.Context, t *testing.T, fn ...func(context.Context, *testing.T, *clients, string)) (*clients, string) {
	t.Helper()
	skipIfExcluded(t)

	t.Helper()
	namespace := names.SimpleNameGenerator.RestrictLengthWithRandomSuffix("arendelle")

	initializeLogsAndMetrics(t)

	// Inline controller logs from SYSTEM_NAMESPACE into the t.Log output.
	cancel := logstream.Start(t)
	t.Cleanup(cancel)

	c := newClients(t, knativetest.Flags.Kubeconfig, knativetest.Flags.Cluster, namespace)
	createNamespace(ctx, t, namespace, c.KubeClient)
	verifyServiceAccountExistence(ctx, t, namespace, c.KubeClient)

	for _, f := range fn {
		f(ctx, t, c, namespace)
	}

	return c, namespace
}

func header(t *testing.T, text string) {
	t.Helper()
	left := "### "
	right := " ###"
	txt := left + text + right
	bar := strings.Repeat("#", len(txt))
	t.Logf(bar)
	t.Logf(txt)
	t.Logf(bar)
}

func tearDown(ctx context.Context, t *testing.T, cs *clients, namespace string) {
	t.Helper()
	if cs.KubeClient == nil {
		return
	}
	if t.Failed() {
		header(t, fmt.Sprintf("Dumping objects from %s", namespace))
		bs, err := getCRDYaml(ctx, cs, namespace)
		if err != nil {
			t.Error(err)
		} else {
			t.Log(string(bs))
		}
		header(t, fmt.Sprintf("Dumping logs from Pods in the %s", namespace))
		v1beta1Taskruns, err := cs.V1beta1TaskRunClient.List(ctx, metav1.ListOptions{})
		if err != nil {
			t.Errorf("Error getting v1beta1TaskRun list %s", err)
		}
		for _, tr := range v1beta1Taskruns.Items {
			if tr.Status.PodName != "" {
				CollectPodLogs(ctx, cs, tr.Status.PodName, namespace, t.Logf)
			}
		}
		v1Taskruns, err := cs.V1TaskRunClient.List(ctx, metav1.ListOptions{})
		if err != nil {
			t.Errorf("Error getting v1TaskRun list %s", err)
		}
		for _, tr := range v1Taskruns.Items {
			if tr.Status.PodName != "" {
				CollectPodLogs(ctx, cs, tr.Status.PodName, namespace, t.Logf)
			}
		}
	}

	if os.Getenv("TEST_KEEP_NAMESPACES") == "" && !t.Failed() {
		t.Logf("Deleting namespace %s", namespace)
		if err := cs.KubeClient.CoreV1().Namespaces().Delete(ctx, namespace, metav1.DeleteOptions{}); err != nil {
			t.Errorf("Failed to delete namespace %s: %s", namespace, err)
		}
	} else {
		t.Logf("Not deleting namespace %s", namespace)
	}
}

func initializeLogsAndMetrics(t *testing.T) {
	t.Helper()
	initMetrics.Do(func() {
		flag.Parse()
		flag.Set("alsologtostderr", "true")
		logging.InitializeLogger()

		// if knativetest.Flags.EmitMetrics {
		logging.InitializeMetricExporter(t.Name())
		//}
	})
}

func createNamespace(ctx context.Context, t *testing.T, namespace string, kubeClient kubernetes.Interface) {
	t.Helper()
	t.Logf("Create namespace %s to deploy to", namespace)
	labels := map[string]string{
		"tekton.dev/test-e2e": "true",
	}
	if _, err := kubeClient.CoreV1().Namespaces().Create(ctx, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:   namespace,
			Labels: labels,
		},
	}, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create namespace %s for tests: %s", namespace, err)
	}
}

func getDefaultSA(ctx context.Context, t *testing.T, kubeClient kubernetes.Interface, namespace string) string {
	t.Helper()
	configDefaultsCM, err := kubeClient.CoreV1().ConfigMaps(system.Namespace()).Get(ctx, config.GetDefaultsConfigName(), metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Failed to get ConfigMap `%s`: %s", config.GetDefaultsConfigName(), err)
	}
	actual, ok := configDefaultsCM.Data["default-service-account"]
	if !ok {
		return "default"
	}
	return actual
}

func verifyServiceAccountExistence(ctx context.Context, t *testing.T, namespace string, kubeClient kubernetes.Interface) {
	t.Helper()
	defaultSA := getDefaultSA(ctx, t, kubeClient, namespace)
	t.Logf("Verify SA %q is created in namespace %q", defaultSA, namespace)

	if err := wait.PollImmediate(interval, timeout, func() (bool, error) {
		_, err := kubeClient.CoreV1().ServiceAccounts(namespace).Get(ctx, defaultSA, metav1.GetOptions{})
		if err != nil && errors.IsNotFound(err) {
			return false, nil
		}
		return true, err
	}); err != nil {
		t.Fatalf("Failed to get SA %q in namespace %q for tests: %s", defaultSA, namespace, err)
	}
}

// TestMain initializes anything global needed by the tests. Right now this is just log and metric
// setup since the log and metric libs we're using use global state :(
func TestMain(m *testing.M) {
	flag.Parse()
	c := m.Run()
	fmt.Fprintf(os.Stderr, "Using kubeconfig at `%s` with cluster `%s`\n", knativetest.Flags.Kubeconfig, knativetest.Flags.Cluster)
	os.Exit(c)
}

func getCRDYaml(ctx context.Context, cs *clients, ns string) ([]byte, error) {
	var output []byte
	printOrAdd := func(i interface{}) {
		bs, err := yaml.Marshal(i)
		if err != nil {
			return
		}
		output = append(output, []byte("\n---\n")...)
		output = append(output, bs...)
	}

	v1beta1Pipelines, err := cs.V1beta1PipelineClient.List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("could not get v1beta1 pipeline: %w", err)
	}
	for _, i := range v1beta1Pipelines.Items {
		i.SetManagedFields(nil)
		printOrAdd(i)
	}

	v1beta1PipelineRuns, err := cs.V1beta1PipelineRunClient.List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("could not get v1beta1 pipelinerun: %w", err)
	}
	for _, i := range v1beta1PipelineRuns.Items {
		i.SetManagedFields(nil)
		printOrAdd(i)
	}

	v1beta1Tasks, err := cs.V1beta1TaskClient.List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("could not get v1beta1 tasks: %w", err)
	}
	for _, i := range v1beta1Tasks.Items {
		i.SetManagedFields(nil)
		printOrAdd(i)
	}

	v1beta1ClusterTasks, err := cs.V1beta1ClusterTaskClient.List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("could not get v1beta1 clustertasks: %w", err)
	}
	for _, i := range v1beta1ClusterTasks.Items {
		i.SetManagedFields(nil)
		printOrAdd(i)
	}

	v1beta1TaskRuns, err := cs.V1beta1TaskRunClient.List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("could not get v1beta1 taskruns: %w", err)
	}
	for _, i := range v1beta1TaskRuns.Items {
		i.SetManagedFields(nil)
		printOrAdd(i)
	}

	v1Tasks, err := cs.V1TaskClient.List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("could not get v1 tasks: %w", err)
	}
	for _, i := range v1Tasks.Items {
		i.SetManagedFields(nil)
		printOrAdd(i)
	}

	v1TaskRuns, err := cs.V1TaskRunClient.List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("could not get v1 taskruns: %w", err)
	}
	for _, i := range v1TaskRuns.Items {
		i.SetManagedFields(nil)
		printOrAdd(i)
	}

	v1Pipelines, err := cs.V1PipelineClient.List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("could not get v1 pipeline: %w", err)
	}
	for _, i := range v1Pipelines.Items {
		i.SetManagedFields(nil)
		printOrAdd(i)
	}

	v1PipelineRuns, err := cs.V1PipelineRunClient.List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("could not get v1 pipelinerun: %w", err)
	}
	for _, i := range v1PipelineRuns.Items {
		i.SetManagedFields(nil)
		printOrAdd(i)
	}

	v1beta1CustomRuns, err := cs.V1beta1CustomRunClient.List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("could not get v1beta1 customruns: %w", err)
	}
	for _, i := range v1beta1CustomRuns.Items {
		i.SetManagedFields(nil)
		printOrAdd(i)
	}

	pods, err := cs.KubeClient.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("could not get pods: %w", err)
	}
	for _, i := range pods.Items {
		// Ignore gitea pods for SCM resolver test
		if strings.HasPrefix(i.Name, "gitea-") {
			continue
		}
		i.SetManagedFields(nil)
		printOrAdd(i)
	}

	return output, nil
}
