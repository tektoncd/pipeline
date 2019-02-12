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

	"github.com/knative/build-pipeline/pkg/names"
	knativetest "github.com/knative/pkg/test"
	"github.com/knative/pkg/test/logging"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	// Mysteriously by k8s libs, or they fail to create `KubeClient`s from config. Apparently just importing it is enough. @_@ side effects @_@. https://github.com/kubernetes/client-go/issues/242
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
)

var metricMutex *sync.Mutex = &sync.Mutex{}

// getContextLogger is a temporary workaround for the fact that `logging.GetContextLogger`
// manipulates global state and so causes race conditions. Once this fix is contributed
// upstream we can remove this wrapper.
func getContextLogger(n string) *logging.BaseLogger {
	// We need to use a mutext to access `GetContextLogger` b/c it set global state for
	// collecting metrics :'(
	metricMutex.Lock()
	logger := logging.GetContextLogger(n)
	metricMutex.Unlock()
	return logger
}

func setup(t *testing.T, logger *logging.BaseLogger) (*clients, string) {
	t.Helper()
	namespace := names.SimpleNameGenerator.GenerateName("arendelle")

	c := newClients(t, knativetest.Flags.Kubeconfig, knativetest.Flags.Cluster, namespace)
	createNamespace(namespace, logger, c.KubeClient)

	return c, namespace
}

func header(logger *logging.BaseLogger, text string) {
	left := "### "
	right := " ###"
	txt := left + text + right
	bar := strings.Repeat("#", len(txt))
	logger.Info(bar)
	logger.Info(txt)
	logger.Info(bar)
}

func tearDown(t *testing.T, logger *logging.BaseLogger, cs *clients, namespace string) {
	t.Helper()
	if cs.KubeClient == nil {
		return
	}
	if t.Failed() {
		header(logger, fmt.Sprintf("Dumping objects from %s", namespace))
		bs, err := getCRDYaml(cs, namespace)
		if err != nil {
			logger.Error(err)
		} else {
			logger.Info(string(bs))
		}
	}

	logger.Infof("Deleting namespace %s", namespace)
	if err := cs.KubeClient.Kube.CoreV1().Namespaces().Delete(namespace, &metav1.DeleteOptions{}); err != nil {
		logger.Errorf("Failed to delete namespace %s: %s", namespace, err)
	}
}

func initializeLogsAndMetrics() {
	flag.Parse()
	flag.Set("alsologtostderr", "true")
	logging.InitializeLogger(knativetest.Flags.LogVerbose)

	if knativetest.Flags.EmitMetrics {
		logging.InitializeMetricExporter()
	}
}

func createNamespace(namespace string, logger *logging.BaseLogger, kubeClient *knativetest.KubeClient) {
	logger.Infof("Create namespace %s to deploy to", namespace)
	if _, err := kubeClient.Kube.CoreV1().Namespaces().Create(&corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
		},
	}); err != nil {
		logger.Fatalf("Failed to create namespace %s for tests: %s", namespace, err)
	}
}

// TestMain initializes anything global needed by the tests. Right now this is just log and metric
// setup since the log and metric libs we're using use global state :(
func TestMain(m *testing.M) {
	initializeLogsAndMetrics()
	logger := logging.GetContextLogger("TestMain")
	logger.Infof("Using kubeconfig at `%s` with cluster `%s`", knativetest.Flags.Kubeconfig, knativetest.Flags.Cluster)
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
