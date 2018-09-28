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

package test

import (
	"testing"

	knativetest "github.com/knative/pkg/test"
	"github.com/knative/pkg/test/logging"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	// Mysteriously by k8s libs, or they fail to create `KubeClient`s from config. Apparently just importing it is enough. @_@ side effects @_@. https://github.com/kubernetes/client-go/issues/242
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
)

// TODO: randomly create a namespace on start of test execution, tear it down at the end
const (
	namespace = "arendelle"
)

func setup(t *testing.T) *Clients {
	clients, err := NewClients(knativetest.Flags.Kubeconfig, knativetest.Flags.Cluster, namespace)
	if err != nil {
		t.Fatalf("Couldn't initialize clients: %v", err)
	}
	return clients
}

// TestPipeline is just a dummy test right now to make sure the whole integration test
// setup and execution is working.
func TestPipeline(t *testing.T) {
	clients := setup(t)
	logger := logging.GetContextLogger(t.Name())

	p, err := clients.PipelineClient.List(metav1.ListOptions{})
	if err != nil {
		t.Fatalf("Couldn't list Pipelines in the cluster (did you deploy the CRDs to %s?): %s", knativetest.Flags.Cluster, err)
	}
	logger.Infof("Listed pipelines: %s", p.Items)
}
