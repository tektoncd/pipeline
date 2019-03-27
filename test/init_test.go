// +build e2e

/*
Copyright 2018 Knative Authors LLC
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
	"flag"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/ghodss/yaml"

	knativetest "github.com/knative/pkg/test"
	"github.com/knative/pkg/test/logging"
	"github.com/tektoncd/pipeline/pkg/names"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"

	// Mysteriously by k8s libs, or they fail to create `KubeClient`s from config. Apparently just importing it is enough. @_@ side effects @_@. https://github.com/kubernetes/client-go/issues/242
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
)

var initMetrics sync.Once

func setup(t *testing.T) (*clients, string) {
	t.Helper()
	namespace := names.SimpleNameGenerator.RestrictLengthWithRandomSuffix("arendelle")

	initializeLogsAndMetrics(t)

	c := newClients(t, knativetest.Flags.Kubeconfig, knativetest.Flags.Cluster, namespace)
	createNamespace(t, namespace, c.KubeClient)
	verifyServiceAccountExistence(t, namespace, c.KubeClient)
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

func tearDown(t *testing.T, cs *clients, namespace string) {
	t.Helper()
	if cs.KubeClient == nil {
		return
	}
	if t.Failed() {
		header(t.Logf, fmt.Sprintf("Dumping objects from %s", namespace))
		bs, err := getCRDYaml(cs, namespace)
		if err != nil {
			t.Error(err)
		} else {
			t.Log(string(bs))
		}
	}

	t.Logf("Deleting namespace %s", namespace)
	if err := cs.KubeClient.Kube.CoreV1().Namespaces().Delete(namespace, &metav1.DeleteOptions{}); err != nil {
		t.Errorf("Failed to delete namespace %s: %s", namespace, err)
	}
}

func initializeLogsAndMetrics(t *testing.T) {
	initMetrics.Do(func() {
		flag.Parse()
		flag.Set("alsologtostderr", "true")
		logging.InitializeLogger(knativetest.Flags.LogVerbose)

		if knativetest.Flags.EmitMetrics {
			logging.InitializeMetricExporter(t.Name())
		}
	})
}

func createNamespace(t *testing.T, namespace string, kubeClient *knativetest.KubeClient) {
	t.Logf("Create namespace %s to deploy to", namespace)
	if _, err := kubeClient.Kube.CoreV1().Namespaces().Create(&corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
		},
	}); err != nil {
		t.Fatalf("Failed to create namespace %s for tests: %s", namespace, err)
	}
}

func verifyServiceAccountExistence(t *testing.T, namespace string, kubeClient *knativetest.KubeClient) {
	defaultSA := "default"
	t.Logf("Verify SA %q is created in namespace %q", defaultSA, namespace)

	if err := wait.PollImmediate(interval, timeout, func() (bool, error) {
		_, err := kubeClient.Kube.CoreV1().ServiceAccounts(namespace).Get(defaultSA, metav1.GetOptions{})
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
	fmt.Fprintf(os.Stderr, "Using kubeconfig at `%s` with cluster `%s`", knativetest.Flags.Kubeconfig, knativetest.Flags.Cluster)
	c := m.Run()
	os.Exit(c)
}

func getCRDYaml(cs *clients, ns string) ([]byte, error) {
	var output []byte
	printOrAdd := func(kind, name string, i interface{}) {
		bs, err := yaml.Marshal(i)
		if err != nil {
			return
		}
		output = append(output, []byte("\n---\n")...)
		output = append(output, bs...)
	}

	ps, err := cs.PipelineClient.List(metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("could not get pipeline %s", err)
	}
	for _, i := range ps.Items {
		printOrAdd("Pipeline", i.Name, i)
	}

	prs, err := cs.PipelineResourceClient.List(metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("could not get pipelinerun resource %s", err)
	}
	for _, i := range prs.Items {
		printOrAdd("PipelineResource", i.Name, i)
	}

	prrs, err := cs.PipelineRunClient.List(metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("could not get pipelinerun %s", err)
	}
	for _, i := range prrs.Items {
		printOrAdd("PipelineRun", i.Name, i)
	}

	ts, err := cs.TaskClient.List(metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("could not get tasks %s", err)
	}
	for _, i := range ts.Items {
		printOrAdd("Task", i.Name, i)
	}
	trs, err := cs.TaskRunClient.List(metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("could not get taskrun %s", err)
	}
	for _, i := range trs.Items {
		printOrAdd("TaskRun", i.Name, i)
	}

	pods, err := cs.KubeClient.Kube.CoreV1().Pods(ns).List(metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("could not get pods %s", err)
	}
	for _, i := range pods.Items {
		printOrAdd("Pod", i.Name, i)
	}

	return output, nil
}
