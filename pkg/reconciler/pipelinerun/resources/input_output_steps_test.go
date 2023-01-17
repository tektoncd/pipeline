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
package resources_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	resourcev1alpha1 "github.com/tektoncd/pipeline/pkg/apis/resource/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/reconciler/pipelinerun/resources"
	"github.com/tektoncd/pipeline/test/diff"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var pvcDir = "/pvc"

func TestGetOutputSteps(t *testing.T) {
	r1 := &resourcev1alpha1.PipelineResource{
		ObjectMeta: metav1.ObjectMeta{
			Name:     "resource1",
			SelfLink: "/apis/tekton.dev/pipelineresources/resource1",
		},
	}
	r2 := &resourcev1alpha1.PipelineResource{
		ObjectMeta: metav1.ObjectMeta{
			Name:     "resource2",
			SelfLink: "/apis/tekton.dev/pipelineresources/resource2",
		},
	}
	r3 := &resourcev1alpha1.PipelineResource{
		Spec: resourcev1alpha1.PipelineResourceSpec{
			Type: resourcev1alpha1.PipelineResourceTypeGit,
			Params: []resourcev1alpha1.ResourceParam{{
				Name:  "url",
				Value: "https://github.com/tektoncd/pipeline.git",
			}},
			SecretParams: nil,
		},
	}
	tcs := []struct {
		name                       string
		outputs                    map[string]*resourcev1alpha1.PipelineResource
		expectedtaskOuputResources []v1beta1.TaskResourceBinding
		pipelineTaskName           string
	}{{
		name:    "single output",
		outputs: map[string]*resourcev1alpha1.PipelineResource{"test-output": r1},
		expectedtaskOuputResources: []v1beta1.TaskResourceBinding{{
			PipelineResourceBinding: v1beta1.PipelineResourceBinding{
				Name:        "test-output",
				ResourceRef: &v1beta1.PipelineResourceRef{Name: "resource1"},
			},
			Paths: []string{"/pvc/test-taskname/test-output"},
		}},
		pipelineTaskName: "test-taskname",
	}, {
		name: "multiple-outputs",
		outputs: map[string]*resourcev1alpha1.PipelineResource{
			"test-output":   r1,
			"test-output-2": r2,
		},
		expectedtaskOuputResources: []v1beta1.TaskResourceBinding{{
			PipelineResourceBinding: v1beta1.PipelineResourceBinding{
				Name:        "test-output",
				ResourceRef: &v1beta1.PipelineResourceRef{Name: "resource1"},
			},
			Paths: []string{"/pvc/test-multiple-outputs/test-output"},
		}, {
			PipelineResourceBinding: v1beta1.PipelineResourceBinding{
				Name:        "test-output-2",
				ResourceRef: &v1beta1.PipelineResourceRef{Name: "resource2"},
			},
			Paths: []string{"/pvc/test-multiple-outputs/test-output-2"},
		}},
		pipelineTaskName: "test-multiple-outputs",
	}, {
		name:    "single output with resource spec",
		outputs: map[string]*resourcev1alpha1.PipelineResource{"test-output": r3},
		expectedtaskOuputResources: []v1beta1.TaskResourceBinding{{
			PipelineResourceBinding: v1beta1.PipelineResourceBinding{
				Name:         "test-output",
				ResourceSpec: &r3.Spec,
			},
			Paths: []string{"/pvc/test-taskname/test-output"},
		}},
		pipelineTaskName: "test-taskname",
	}, {
		name: "multiple-outputs-with-resource-spec",
		outputs: map[string]*resourcev1alpha1.PipelineResource{
			"test-output-1": r3,
			"test-output-2": r3,
		},
		expectedtaskOuputResources: []v1beta1.TaskResourceBinding{{
			PipelineResourceBinding: v1beta1.PipelineResourceBinding{
				Name:         "test-output-1",
				ResourceSpec: &r3.Spec,
			},
			Paths: []string{"/pvc/test-multiple-outputs-with-resource-spec/test-output-1"},
		}, {
			PipelineResourceBinding: v1beta1.PipelineResourceBinding{
				Name:         "test-output-2",
				ResourceSpec: &r3.Spec,
			},
			Paths: []string{"/pvc/test-multiple-outputs-with-resource-spec/test-output-2"},
		}},
		pipelineTaskName: "test-multiple-outputs-with-resource-spec",
	}}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			postTasks := resources.GetOutputSteps(tc.outputs, tc.pipelineTaskName, pvcDir)
			if d := cmp.Diff(tc.expectedtaskOuputResources, postTasks, cmpopts.SortSlices(lessTaskResourceBindings)); d != "" {
				t.Errorf("error comparing post steps %s", diff.PrintWantGot(d))
			}
		})
	}
}

func lessTaskResourceBindings(i, j v1beta1.TaskResourceBinding) bool {
	return i.Name < j.Name
}
