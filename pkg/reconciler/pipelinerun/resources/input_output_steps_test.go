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

func TestGetInputSteps(t *testing.T) {
	r1 := &resourcev1alpha1.PipelineResource{
		ObjectMeta: metav1.ObjectMeta{
			Name:     "resource1",
			SelfLink: "/apis/tekton.dev/pipelineresources/resource1",
		},
	}
	tcs := []struct {
		name                       string
		inputs                     map[string]*resourcev1alpha1.PipelineResource
		pipelineTask               *v1beta1.PipelineTask
		expectedtaskInputResources []v1beta1.TaskResourceBinding
	}{
		{
			name:   "task-with-a-constraint",
			inputs: map[string]*resourcev1alpha1.PipelineResource{"test-input": r1},
			pipelineTask: &v1beta1.PipelineTask{
				Resources: &v1beta1.PipelineTaskResources{
					Inputs: []v1beta1.PipelineTaskInputResource{{
						Name: "test-input",
						From: []string{"prev-task-1"},
					}},
				},
			},
			expectedtaskInputResources: []v1beta1.TaskResourceBinding{{
				PipelineResourceBinding: v1beta1.PipelineResourceBinding{
					ResourceRef: &v1beta1.PipelineResourceRef{Name: "resource1"},
					Name:        "test-input",
				},
			}},
		}, {
			name:   "task-with-no-input-constraint",
			inputs: map[string]*resourcev1alpha1.PipelineResource{"test-input": r1},
			expectedtaskInputResources: []v1beta1.TaskResourceBinding{{
				PipelineResourceBinding: v1beta1.PipelineResourceBinding{
					ResourceRef: &v1beta1.PipelineResourceRef{Name: "resource1"},
					Name:        "test-input",
				},
			}},
			pipelineTask: &v1beta1.PipelineTask{
				Name: "sample-test-task",
				Resources: &v1beta1.PipelineTaskResources{
					Inputs: []v1beta1.PipelineTaskInputResource{{
						Name: "test-input",
					}},
				},
			},
		}, {
			name:   "task-with-multiple-constraints",
			inputs: map[string]*resourcev1alpha1.PipelineResource{"test-input": r1},
			pipelineTask: &v1beta1.PipelineTask{
				Resources: &v1beta1.PipelineTaskResources{
					Inputs: []v1beta1.PipelineTaskInputResource{{
						Name: "test-input",
						From: []string{"prev-task-1", "prev-task-2"},
					}},
				},
			},
			expectedtaskInputResources: []v1beta1.TaskResourceBinding{{
				PipelineResourceBinding: v1beta1.PipelineResourceBinding{
					ResourceRef: &v1beta1.PipelineResourceRef{Name: "resource1"},
					Name:        "test-input",
				},
			}},
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			taskInputResources := resources.GetInputSteps(tc.inputs, tc.pipelineTask.Resources.Inputs)
			if d := cmp.Diff(tc.expectedtaskInputResources, taskInputResources, cmpopts.SortSlices(lessTaskResourceBindings)); d != "" {
				t.Errorf("error comparing task resource inputs %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestWrapSteps(t *testing.T) {
	r1 := &resourcev1alpha1.PipelineResource{
		ObjectMeta: metav1.ObjectMeta{
			Name:     "resource1",
			SelfLink: "/apis/tekton.dev/pipelineresources/resource1",
		},
	}
	inputs := map[string]*resourcev1alpha1.PipelineResource{
		"test-input":   r1,
		"test-input-2": r1,
	}
	outputs := map[string]*resourcev1alpha1.PipelineResource{
		"test-output": r1,
	}

	pt := &v1beta1.PipelineTask{
		Name: "test-task",
		Resources: &v1beta1.PipelineTaskResources{
			Inputs: []v1beta1.PipelineTaskInputResource{{
				Name: "test-input",
				From: []string{"prev-task"},
			}},
		},
	}

	taskRunSpec := &v1beta1.TaskRunSpec{}
	resources.WrapSteps(taskRunSpec, pt, inputs, outputs)

	expectedtaskInputResources := []v1beta1.TaskResourceBinding{{
		PipelineResourceBinding: v1beta1.PipelineResourceBinding{
			ResourceRef: &v1beta1.PipelineResourceRef{Name: "resource1"},
			Name:        "test-input",
		},
	}, {
		PipelineResourceBinding: v1beta1.PipelineResourceBinding{
			ResourceRef: &v1beta1.PipelineResourceRef{Name: "resource1"},
			Name:        "test-input-2",
		},
	}}

	if d := cmp.Diff(taskRunSpec.Resources.Inputs, expectedtaskInputResources, cmpopts.SortSlices(lessTaskResourceBindings)); d != "" {
		t.Errorf("error comparing input resources %s", diff.PrintWantGot(d))
	}
}

func lessTaskResourceBindings(i, j v1beta1.TaskResourceBinding) bool {
	return i.Name < j.Name
}
