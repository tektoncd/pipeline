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
	"os"
	"testing"

	knativetest "github.com/knative/pkg/test"
	"github.com/knative/pkg/test/logging"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func setup(t *testing.T, logger *logging.BaseLogger) (*clients, string) {
	namespace := AppendRandomString("arendelle")

	c, err := newClients(knativetest.Flags.Kubeconfig, knativetest.Flags.Cluster, namespace)
	if err != nil {
		t.Fatalf("Couldn't initialize clients: %v", err)
	}

	createNamespace(namespace, logger, c.KubeClient)

	return c, namespace
}

func tearDown(logger *logging.BaseLogger, kubeClient *knativetest.KubeClient, namespace string) {
	if kubeClient != nil {
		logger.Infof("Deleting namespace %s", namespace)
		if err := kubeClient.Kube.CoreV1().Namespaces().Delete(namespace, &metav1.DeleteOptions{}); err != nil {
			logger.Errorf("Failed to delete namespace %s: %s", namespace, err)
		}
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
