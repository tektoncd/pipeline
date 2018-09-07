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

package v1beta1

import (
	"log"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/knative/build-pipeline/test"
	"golang.org/x/net/context"
	runtime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	controllerClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

var client controllerClient.Client

const namespace = "default"

// assertResourceIsEuqal will retrive the resource with the name name into the fetched object
// and compare that value to expected.
func assertResourceIsEqual(t *testing.T, name string, expected, fetched runtime.Object) {
	t.Helper()

	key := types.NamespacedName{Name: name, Namespace: namespace}

	c := context.Background()
	if err := client.Get(c, key, fetched); err != nil {
		t.Errorf("Couldn't retreive created resource %s from namespace %s: %s", name, namespace, err)
	}

	if !reflect.DeepEqual(fetched, expected) {
		t.Errorf("Fetched resource was different from created resource. Fetched: %v. Expected: %v.", fetched, expected)
	}
}

// createResource will load the resource at fileName into the object created. It will retrieve the
// object using name into fetched and assert that the retrieved object is equal to the one created.
func createResource(t *testing.T, fileName, name string, created, fetched runtime.Object) {
	t.Helper()

	if err := test.DecodeTypeFromYamlSample(fileName, created); err != nil {
		t.Fatalf("couldn't load resource from %s: %s", fileName, err)
	}

	// Create the resource
	c := context.Background()
	if err := client.Create(c, created); err != nil {
		t.Fatalf("Controller failed to create resource from %s: %s", fileName, err)
	}

	// Retreive it
	assertResourceIsEqual(t, name, created, fetched)
}

// updateResoruce will update the resource with the name name to the values in updated, the retrieve
// name into expected and assert that the values match updated.
func updateResource(t *testing.T, name string, updated, fetched runtime.Object) {
	c := context.Background()

	if err := client.Update(c, updated); err != nil {
		t.Errorf("Failed to update resource %s: %s", name, err)
	}
	assertResourceIsEqual(t, name, updated, fetched)
}

// deleteResource will delete resource, then try to retrieve it via the name ane assert that this fails.
func deleteResource(t *testing.T, name string, resource runtime.Object) {
	c := context.Background()

	if err := client.Delete(c, resource); err != nil {
		t.Fatalf("Failed to delete resource %s: %s", name, err)
	}

	// It should no longer exist
	key := types.NamespacedName{Name: name, Namespace: namespace}
	err := client.Get(c, key, resource)
	if err == nil {
		t.Fatalf("Expected to get an error retrieving deleted resource %s", name)
	}
}

func TestMain(m *testing.M) {
	var cfg *rest.Config

	t := &envtest.Environment{
		CRDDirectoryPaths: []string{filepath.Join("..", "..", "..", "..", "config", "crds")},
	}

	err := SchemeBuilder.AddToScheme(scheme.Scheme)
	if err != nil {
		log.Fatal(err)
	}

	if cfg, err = t.Start(); err != nil {
		log.Fatal(err)
	}

	if client, err = controllerClient.New(cfg, controllerClient.Options{Scheme: scheme.Scheme}); err != nil {
		log.Fatal(err)
	}

	code := m.Run()
	t.Stop()
	os.Exit(code)
}

func TestPipeline(t *testing.T) {
	created, fetched := &Pipeline{}, &Pipeline{}

	createResource(t, test.PipelineFile, test.PipelineName, created, fetched)

	updated := created.DeepCopy()
	updated.Labels = map[string]string{"hello": "world"}
	updateResource(t, test.PipelineName, updated, fetched)

	deleteResource(t, test.PipelineName, created)
}

func TestPipelineParams(t *testing.T) {
	created, fetched := &PipelineParams{}, &PipelineParams{}

	createResource(t, test.PipelineParamsFile, test.PipelineParamsName, created, fetched)

	updated := created.DeepCopy()
	updated.Labels = map[string]string{"hello": "world"}
	updateResource(t, test.PipelineParamsName, updated, fetched)

	deleteResource(t, test.PipelineParamsName, created)
}

func TestPipelineRun(t *testing.T) {
	created, fetched := &PipelineRun{}, &PipelineRun{}

	createResource(t, test.PipelineRunFile, test.PipelineRunName, created, fetched)

	updated := created.DeepCopy()
	updated.Labels = map[string]string{"hello": "world"}
	updateResource(t, test.PipelineRunName, updated, fetched)

	deleteResource(t, test.PipelineRunName, created)
}

func TestTask(t *testing.T) {
	created, fetched := &Task{}, &Task{}

	createResource(t, test.TaskFile, test.TaskName, created, fetched)

	updated := created.DeepCopy()
	updated.Labels = map[string]string{"hello": "world"}
	updateResource(t, test.TaskName, updated, fetched)

	deleteResource(t, test.TaskName, created)
}

func TestTaskRun(t *testing.T) {
	created, fetched := &TaskRun{}, &TaskRun{}

	createResource(t, test.TaskRunFile, test.TaskRunName, created, fetched)

	updated := created.DeepCopy()
	updated.Labels = map[string]string{"hello": "world"}
	updateResource(t, test.TaskRunName, updated, fetched)

	deleteResource(t, test.TaskRunName, created)
}
