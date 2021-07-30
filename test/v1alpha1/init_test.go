// +build e2e

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

	"github.com/ghodss/yaml"
	"github.com/tektoncd/pipeline/pkg/names"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	knativetest "knative.dev/pkg/test"
	"knative.dev/pkg/test/logging"
	"knative.dev/pkg/test/logstream"

	// Mysteriously by k8s libs, or they fail to create `KubeClient`s from config. Apparently just importing it is enough. @_@ side effects @_@. https://github.com/kubernetes/client-go/issues/242
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	// Mysteriously by k8s libs, or they fail to create `KubeClient`s when using oidc authentication. Apparently just importing it is enough. @_@ side effects @_@. https://github.com/kubernetes/client-go/issues/345
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
)

var initMetrics sync.Once
var skipRootUserTests = false

func init() {
	flag.BoolVar(&skipRootUserTests, "skipRootUserTests", false, "Skip tests that require root user")
}

func setup(ctx context.Context, t *testing.T, fn ...func(context.Context, *testing.T, *clients, string)) (*clients, string) {
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

func header(logf logging.FormatLogger, text string) {
	left := "### "
	right := " ###"
	txt := left + text + right
	bar := strings.Repeat("#", len(txt))
	logf(bar)
	logf(txt)
	logf(bar)
}

func tearDown(ctx context.Context, t *testing.T, cs *clients, namespace string) {
	t.Helper()
	if cs.KubeClient == nil {
		return
	}
	if t.Failed() {
		header(t.Logf, fmt.Sprintf("Dumping objects from %s", namespace))
		bs, err := getCRDYaml(ctx, cs, namespace)
		if err != nil {
			t.Error(err)
		} else {
			t.Log(string(bs))
		}
		header(t.Logf, fmt.Sprintf("Dumping logs from Pods in the %s", namespace))
		taskruns, err := cs.TaskRunClient.List(ctx, metav1.ListOptions{})
		if err != nil {
			t.Errorf("Error getting TaskRun list %s", err)
		}
		for _, tr := range taskruns.Items {
			if tr.Status.PodName != "" {
				CollectPodLogs(ctx, cs, tr.Status.PodName, namespace, t.Logf)
			}
		}
	}

	if os.Getenv("TEST_KEEP_NAMESPACES") == "" {
		t.Logf("Deleting namespace %s", namespace)
		if err := cs.KubeClient.CoreV1().Namespaces().Delete(ctx, namespace, metav1.DeleteOptions{}); err != nil {
			t.Errorf("Failed to delete namespace %s: %s", namespace, err)
		}
	}
}

func initializeLogsAndMetrics(t *testing.T) {
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

func verifyServiceAccountExistence(ctx context.Context, t *testing.T, namespace string, kubeClient kubernetes.Interface) {
	defaultSA := "default"
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

	ps, err := cs.PipelineClient.List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("could not get pipeline: %w", err)
	}
	for _, i := range ps.Items {
		printOrAdd(i)
	}

	prs, err := cs.PipelineResourceClient.List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("could not get pipelinerun resource: %w", err)
	}
	for _, i := range prs.Items {
		printOrAdd(i)
	}

	prrs, err := cs.PipelineRunClient.List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("could not get pipelinerun: %w", err)
	}
	for _, i := range prrs.Items {
		printOrAdd(i)
	}

	ts, err := cs.TaskClient.List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("could not get tasks: %w", err)
	}
	for _, i := range ts.Items {
		printOrAdd(i)
	}
	trs, err := cs.TaskRunClient.List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("could not get taskrun: %w", err)
	}
	for _, i := range trs.Items {
		printOrAdd(i)
	}

	pods, err := cs.KubeClient.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("could not get pods: %w", err)
	}
	for _, i := range pods.Items {
		printOrAdd(i)
	}

	return output, nil
}
