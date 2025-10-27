//go:build conformance || e2e || examples || featureflags
// +build conformance e2e examples featureflags

/*
Copyright 2023 The Tekton Authors

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

package test

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/tektoncd/pipeline/pkg/apis/config"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/names"
	"github.com/tektoncd/pipeline/pkg/remoteresolution/resolver/framework/cache"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp" // Mysteriously by k8s libs, or they fail to create `KubeClient`s when using oidc authentication. Apparently just importing it is enough. @_@ side effects @_@. https://github.com/kubernetes/client-go/issues/345
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/system"
	knativetest "knative.dev/pkg/test"
	"knative.dev/pkg/test/logging" // Mysteriously by k8s libs, or they fail to create `KubeClient`s from config. Apparently just importing it is enough. @_@ side effects @_@. https://github.com/kubernetes/client-go/issues/242
	"knative.dev/pkg/test/logstream"
	"sigs.k8s.io/yaml"
)

const (
	sleepDuration = 15 * time.Second
)

var (
	initMetrics             sync.Once
	ignoreTypeMeta          = cmpopts.IgnoreFields(metav1.TypeMeta{}, "Kind", "APIVersion")
	ignoreObjectMeta        = cmpopts.IgnoreFields(metav1.ObjectMeta{}, "ResourceVersion", "UID", "CreationTimestamp", "Generation", "ManagedFields", "Labels", "Annotations", "OwnerReferences")
	ignoreCondition         = cmpopts.IgnoreFields(apis.Condition{}, "LastTransitionTime.Inner.Time", "Message")
	ignoreConditions        = cmpopts.IgnoreFields(duckv1.Status{}, "Conditions")
	ignoreStepState         = cmpopts.IgnoreFields(v1.StepState{}, "ImageID", "TerminationReason", "Provenance")
	ignoreContainerStates   = cmpopts.IgnoreFields(corev1.ContainerState{}, "Terminated")
	ignorePipelineRunStatus = cmpopts.IgnoreFields(v1.PipelineRunStatusFields{}, "StartTime", "CompletionTime", "FinallyStartTime", "ChildReferences", "Provenance")
	ignoreTaskRunStatus     = cmpopts.IgnoreFields(v1.TaskRunStatusFields{}, "StartTime", "CompletionTime", "Provenance")
	// ignoreSATaskRunSpec ignores the service account in the TaskRunSpec as it may differ across platforms
	ignoreSATaskRunSpec = cmpopts.IgnoreFields(v1.TaskRunSpec{}, "ServiceAccountName")
	// ignoreSAPipelineRunSpec ignores the service account in the PipelineRunSpec as it may differ across platforms
	ignoreSAPipelineRunSpec = cmpopts.IgnoreFields(v1.PipelineTaskRunTemplate{}, "ServiceAccountName")
)

func setup(ctx context.Context, t *testing.T, fn ...func(context.Context, *testing.T, *clients, string)) (*clients, string) {
	t.Helper()
	skipIfExcluded(t)

	cache.Get(ctx).Clear()

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
	t.Log(bar)
	t.Log(txt)
	t.Log(bar)
}

func tearDown(ctx context.Context, t *testing.T, cs *clients, namespace string) {
	t.Helper()
	if cs.KubeClient == nil {
		return
	}
	if t.Failed() {
		header(t, "Dumping objects from "+namespace)
		bs, err := getCRDYaml(ctx, cs, namespace)
		if err != nil {
			t.Error(err)
		} else {
			t.Log(string(bs))
		}
		header(t, "Dumping logs from Pods in the "+namespace)
		taskRuns, err := cs.V1TaskRunClient.List(ctx, metav1.ListOptions{})
		if err != nil {
			t.Errorf("Error listing TaskRuns: %s", err)
		}
		for _, tr := range taskRuns.Items {
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

	if err := wait.PollUntilContextTimeout(ctx, interval, timeout, true, func(context.Context) (bool, error) {
		_, err := kubeClient.CoreV1().ServiceAccounts(namespace).Get(ctx, defaultSA, metav1.GetOptions{})
		if err != nil && errors.IsNotFound(err) {
			return false, nil
		}
		return true, err
	}); err != nil {
		t.Fatalf("Failed to get SA %q in namespace %q for tests: %s", defaultSA, namespace, err)
	}
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

// updateConfigMap updates the config map for specified @name with values. We can't use the one from knativetest because
// it assumes that Data is already a non-nil map, and by default, it isn't!
func updateConfigMap(ctx context.Context, client kubernetes.Interface, name string, configName string, values map[string]string) error {
	configMap, err := client.CoreV1().ConfigMaps(name).Get(ctx, configName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	if configMap.Data == nil {
		configMap.Data = make(map[string]string)
	}

	for key, value := range values {
		configMap.Data[key] = value
	}

	_, err = client.CoreV1().ConfigMaps(name).Update(ctx, configMap, metav1.UpdateOptions{})
	return err
}

// This method is necessary because PipelineRunTaskRunStatus and TaskRunStatus
// don't have an IsFailed method.
func isFailed(t *testing.T, taskRunName string, conds duckv1.Conditions) bool {
	t.Helper()
	for _, c := range conds {
		if c.Type == apis.ConditionSucceeded {
			if c.Status != corev1.ConditionFalse {
				t.Errorf("TaskRun status %q is not failed, got %q", taskRunName, c.Status)
			}
			return true
		}
	}
	t.Errorf("TaskRun status %q had no Succeeded condition", taskRunName)
	return false
}

func isSuccessful(t *testing.T, taskRunName string, conds duckv1.Conditions) bool {
	t.Helper()
	for _, c := range conds {
		if c.Type == apis.ConditionSucceeded {
			if c.Status != corev1.ConditionTrue {
				t.Errorf("TaskRun status %q is not succeeded, got %q", taskRunName, c.Status)
			}
			return true
		}
	}
	t.Errorf("TaskRun status %q had no Succeeded condition", taskRunName)
	return false
}

func isCancelled(t *testing.T, taskRunName string, conds duckv1.Conditions) bool {
	t.Helper()
	for _, c := range conds {
		if c.Type == apis.ConditionSucceeded {
			return true
		}
	}
	t.Errorf("TaskRun status %q had no Succeeded condition", taskRunName)
	return false
}
