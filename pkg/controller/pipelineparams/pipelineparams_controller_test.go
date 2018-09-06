/*
Copyright 2018 The Knative Authors.

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

package pipelineparams

import (
	"log"
	"os"
	"path/filepath"
	"testing"

	"github.com/knative/build-pipeline/pkg/apis"
	pipelinev1beta1 "github.com/knative/build-pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/knative/build-pipeline/test"
	"golang.org/x/net/context"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var cfg *rest.Config

func TestMain(m *testing.M) {
	t := &envtest.Environment{
		CRDDirectoryPaths: []string{filepath.Join("..", "..", "..", "config", "crds")},
	}
	apis.AddToScheme(scheme.Scheme)

	var err error
	if cfg, err = t.Start(); err != nil {
		log.Fatal(err)
	}

	code := m.Run()
	t.Stop()
	os.Exit(code)
}

func TestReconcile(t *testing.T) {
	expectedRequest := reconcile.Request{NamespacedName: types.NamespacedName{Name: test.PipelineParamsName, Namespace: "default"}}
	depKey := types.NamespacedName{Name: test.PipelineParamsName + "-deployment", Namespace: "default"}

	// Setup the Manager and Controller.  Wrap the Controller Reconcile function so it writes each request to a
	// channel when it is finished.
	mgr, err := manager.New(cfg, manager.Options{})
	if err != nil {
		t.Fatalf("Failed to create new controller manager: %s", err)
	}
	client := mgr.GetClient()

	recFn, requests := test.SetupTestReconcile(newReconciler(mgr))
	if err := add(mgr, recFn); err != nil {
		t.Fatalf("Failed to add reconcile function to manager: %s", err)
	}
	defer close(test.StartTestManager(t, mgr))

	instance := &pipelinev1beta1.PipelineParams{}
	if err := test.DecodeTypeFromYamlSample(test.PipelineParamsFile, instance); err != nil {
		t.Fatalf("couldn't load resource from %s: %s", test.PipelineParamsFile, err)
	}

	// Create the object and expect the Reconcile and Deployment to be created
	c := context.Background()
	if err := client.Create(c, instance); err != nil {
		t.Fatalf("Failed to create instance of resource %s: %s", test.PipelineParamsName, err)
	}
	defer client.Delete(c, instance)

	test.WaitForReconcile(t, requests, expectedRequest)
	deploy := test.PollDeployment(t, client, depKey)

	// Delete the Deployment and expect Reconcile to be called for Deployment deletion
	if err := client.Delete(c, deploy); err != nil {
		t.Errorf("Failed to delete the deployment for %s: %s", test.PipelineParamsName, err)
	}

	test.WaitForReconcile(t, requests, expectedRequest)
	deploy = test.PollDeployment(t, client, depKey)

	// Manually delete Deployment since GC isn't enabled in the test control plane and apparently
	// we don't trust that the previous delete was enough to make this happen for reasons
	// known only to the authors of kubebuilder
	if err := client.Delete(c, deploy); err != nil {
		t.Errorf("Failed to delete the deployment for %s: %s", test.PipelineParamsName, err)
	}
}
