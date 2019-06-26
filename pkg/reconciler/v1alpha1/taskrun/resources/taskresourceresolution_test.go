/*
Copyright 2019 The Tekton Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either extress or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package resources

import (
	"testing"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"golang.org/x/xerrors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestResolveTaskRun(t *testing.T) {
	inputs := []v1alpha1.TaskResourceBinding{{
		Name: "repoToBuildFrom",
		ResourceRef: v1alpha1.PipelineResourceRef{
			Name: "git-repo",
		},
	}, {
		Name: "clusterToUse",
		ResourceRef: v1alpha1.PipelineResourceRef{
			Name: "k8s-cluster",
		},
	}, {
		Name: "clusterspecToUse",
		ResourceSpec: &v1alpha1.PipelineResourceSpec{
			Type: v1alpha1.PipelineResourceTypeCluster,
		},
	}}

	outputs := []v1alpha1.TaskResourceBinding{{
		Name: "imageToBuild",
		ResourceRef: v1alpha1.PipelineResourceRef{
			Name: "image",
		},
	}, {
		Name: "gitRepoToUpdate",
		ResourceRef: v1alpha1.PipelineResourceRef{
			Name: "another-git-repo",
		},
	}, {
		Name: "gitspecToUse",
		ResourceSpec: &v1alpha1.PipelineResourceSpec{
			Type: v1alpha1.PipelineResourceTypeGit,
		},
	}}

	taskName := "orchestrate"
	kind := v1alpha1.NamespacedTaskKind
	taskSpec := v1alpha1.TaskSpec{
		Steps: []corev1.Container{{
			Name: "step1",
		}}}

	resources := []*v1alpha1.PipelineResource{{
		ObjectMeta: metav1.ObjectMeta{
			Name: "git-repo",
		},
	}, {
		ObjectMeta: metav1.ObjectMeta{
			Name: "k8s-cluster",
		},
	}, {
		ObjectMeta: metav1.ObjectMeta{
			Name: "image",
		},
	}, {
		ObjectMeta: metav1.ObjectMeta{
			Name: "another-git-repo",
		},
	}}
	resourceIndex := 0
	gr := func(n string) (*v1alpha1.PipelineResource, error) {
		r := resources[resourceIndex]
		resourceIndex++
		return r, nil
	}

	rtr, err := ResolveTaskResources(&taskSpec, taskName, kind, inputs, outputs, gr)
	if err != nil {
		t.Fatalf("Did not expect error trying to resolve TaskRun: %s", err)
	}

	if rtr.TaskName != "orchestrate" {
		t.Errorf("Expected task name `orchestrate` Task but got: %v", rtr.TaskName)
	}
	if rtr.TaskSpec == nil || len(rtr.TaskSpec.Steps) != 1 || rtr.TaskSpec.Steps[0].Name != "step1" {
		t.Errorf("Task not resolved, expected task's spec to be used but spec was: %v", rtr.TaskSpec)
	}

	if len(rtr.Inputs) == 3 {
		r, ok := rtr.Inputs["repoToBuildFrom"]
		if !ok {
			t.Errorf("Expected value present in map for `repoToBuildFrom' but it was missing")
		} else {
			if r.Name != "git-repo" {
				t.Errorf("Expected to use resource `git-repo` for `repoToBuildFrom` but used %s", r.Name)
			}
		}
		r, ok = rtr.Inputs["clusterToUse"]
		if !ok {
			t.Errorf("Expected value present in map for `clusterToUse' but it was missing")
		} else {
			if r.Name != "k8s-cluster" {
				t.Errorf("Expected to use resource `k8s-cluster` for `clusterToUse` but used %s", r.Name)
			}
		}
		r, ok = rtr.Inputs["clusterspecToUse"]
		if !ok {
			t.Errorf("Expected value present in map for `clusterspecToUse' but it was missing")
		} else {
			if r.Spec.Type != v1alpha1.PipelineResourceTypeCluster {
				t.Errorf("Expected to use resource to be of type `cluster` for `clusterspecToUse` but got %s", r.Spec.Type)
			}
		}

	} else {
		t.Errorf("Expected 2 resolved inputs but instead had: %v", rtr.Inputs)
	}

	if len(rtr.Outputs) == 3 {
		r, ok := rtr.Outputs["imageToBuild"]
		if !ok {
			t.Errorf("Expected value present in map for `imageToBuild' but it was missing")
		} else {
			if r.Name != "image" {
				t.Errorf("Expected to use resource `image` for `imageToBuild` but used %s", r.Name)
			}
		}
		r, ok = rtr.Outputs["gitRepoToUpdate"]
		if !ok {
			t.Errorf("Expected value present in map for `gitRepoToUpdate' but it was missing")
		} else {
			if r.Name != "another-git-repo" {
				t.Errorf("Expected to use resource `another-git-repo` for `gitRepoToUpdate` but used %s", r.Name)
			}
		}
		r, ok = rtr.Outputs["gitspecToUse"]
		if !ok {
			t.Errorf("Expected value present in map for `gitspecToUse' but it was missing")
		} else {
			if r.Spec.Type != v1alpha1.PipelineResourceTypeGit {
				t.Errorf("Expected to use resource type `git` for but got %s", r.Spec.Type)
			}
		}
	} else {
		t.Errorf("Expected 2 resolved outputs but instead had: %v", rtr.Outputs)
	}
}

func TestResolveTaskRun_missingOutput(t *testing.T) {
	outputs := []v1alpha1.TaskResourceBinding{{
		Name: "repoToUpdate",
		ResourceRef: v1alpha1.PipelineResourceRef{
			Name: "another-git-repo",
		}}}

	gr := func(n string) (*v1alpha1.PipelineResource, error) { return nil, xerrors.New("nope") }
	_, err := ResolveTaskResources(&v1alpha1.TaskSpec{}, "orchestrate", v1alpha1.NamespacedTaskKind, []v1alpha1.TaskResourceBinding{}, outputs, gr)
	if err == nil {
		t.Fatalf("Expected to get error because output resource couldn't be resolved")
	}
}

func TestResolveTaskRun_missingInput(t *testing.T) {
	inputs := []v1alpha1.TaskResourceBinding{{
		Name: "repoToBuildFrom",
		ResourceRef: v1alpha1.PipelineResourceRef{
			Name: "git-repo",
		}}}
	gr := func(n string) (*v1alpha1.PipelineResource, error) { return nil, xerrors.New("nope") }

	_, err := ResolveTaskResources(&v1alpha1.TaskSpec{}, "orchestrate", v1alpha1.NamespacedTaskKind, inputs, []v1alpha1.TaskResourceBinding{}, gr)
	if err == nil {
		t.Fatalf("Expected to get error because output resource couldn't be resolved")
	}
}

func TestResolveTaskRun_noResources(t *testing.T) {
	taskSpec := v1alpha1.TaskSpec{
		Steps: []corev1.Container{{
			Name: "step1",
		}}}

	gr := func(n string) (*v1alpha1.PipelineResource, error) { return &v1alpha1.PipelineResource{}, nil }

	rtr, err := ResolveTaskResources(&taskSpec, "orchestrate", v1alpha1.NamespacedTaskKind, []v1alpha1.TaskResourceBinding{}, []v1alpha1.TaskResourceBinding{}, gr)
	if err != nil {
		t.Fatalf("Did not expect error trying to resolve TaskRun: %s", err)
	}

	if rtr.TaskName != "orchestrate" {
		t.Errorf("Task not resolved, expected `orchestrate` Task but got: %v", rtr.TaskName)
	}
	if rtr.TaskSpec == nil || len(rtr.TaskSpec.Steps) != 1 || rtr.TaskSpec.Steps[0].Name != "step1" {
		t.Errorf("Task not resolved, expected task's spec to be used but spec was: %v", rtr.TaskSpec)
	}

	if len(rtr.Inputs) != 0 {
		t.Errorf("Did not expect any outputs to be resolved when none specified but had %v", rtr.Inputs)
	}
	if len(rtr.Outputs) != 0 {
		t.Errorf("Did not expect any outputs to be resolved when none specified but had %v", rtr.Outputs)
	}
}
