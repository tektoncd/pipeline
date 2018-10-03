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

// namespace is the namespace that will be created before all tests run and will be torn down once
// the tests complete. It will be generated randomly so that tests can run back to back without
// interfering with each other.
var namespace string

func initializeLogsAndMetrics() {
	flag.Parse()
	flag.Set("alsologtostderr", "true")
	logging.InitializeLogger(knativetest.Flags.LogVerbose)

	if knativetest.Flags.EmitMetrics {
		logging.InitializeMetricExporter()
	}
}

func createNamespace(namespace string, logger *logging.BaseLogger) *knativetest.KubeClient {
	kubeClient, err := knativetest.NewKubeClient(knativetest.Flags.Kubeconfig, knativetest.Flags.Cluster)
	if err != nil {
		logger.Fatalf("failed to create kubeclient from config file at %s: %s", knativetest.Flags.Kubeconfig, err)
	}
	logger.Infof("Create namespace %s to deploy to", namespace)
	if _, err := kubeClient.Kube.CoreV1().Namespaces().Create(&corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
		},
	}); err != nil {
		logger.Fatalf("Failed to create namespace %s for tests: %s", namespace, err)
	}
	return kubeClient
}

func tearDownMain(kubeClient *knativetest.KubeClient, logger *logging.BaseLogger) {
	if kubeClient != nil {
		logger.Infof("Deleting namespace %s", namespace)
		if err := kubeClient.Kube.CoreV1().Namespaces().Delete(namespace, &metav1.DeleteOptions{}); err != nil {
			logger.Errorf("Failed to delete namespace %s: %s", namespace, err)
		}
	}
}

// TestMain initializes anything global needed by the tests. Right now this is just log and metric
// setup since the log and metric libs we're using use global state :(
func TestMain(m *testing.M) {
	initializeLogsAndMetrics()
	logger := logging.GetContextLogger("TestMain")
	logger.Infof("Using kubeconfig at `%s` with cluster `%s`", knativetest.Flags.Kubeconfig, knativetest.Flags.Cluster)

	namespace = AppendRandomString("arendelle")
	kubeClient := createNamespace(namespace, logger)
	knativetest.CleanupOnInterrupt(func() { tearDownMain(kubeClient, logger) }, logger)

	c := m.Run()

	tearDownMain(kubeClient, logger)
	os.Exit(c)
}
