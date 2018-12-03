/*
Copyright 2018 The Knative Authors

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
	"github.com/knative/build-pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/knative/build-pipeline/pkg/reconciler/v1alpha1/pipelinerun/resources"
)

func Test_GetOutputSteps(t *testing.T) {
	tcs := []struct {
		name                       string
		taskResourceBinding        []v1alpha1.TaskResourceBinding
		expectedtaskOuputResources []v1alpha1.TaskResourceBinding
		pipelineTaskName           string
	}{{
		name: "output",
		taskResourceBinding: []v1alpha1.TaskResourceBinding{{
			Name: "test-output",
			ResourceRef: v1alpha1.PipelineResourceRef{
				Name: "resource1",
			},
		}},
		expectedtaskOuputResources: []v1alpha1.TaskResourceBinding{{
			Name:        "test-output",
			ResourceRef: v1alpha1.PipelineResourceRef{Name: "resource1"},
			Paths:       []string{"/pvc/test-taskname/test-output"},
		}},
		pipelineTaskName: "test-taskname",
	}, {
		name: "multiple-outputs",
		taskResourceBinding: []v1alpha1.TaskResourceBinding{{
			Name:        "test-output",
			ResourceRef: v1alpha1.PipelineResourceRef{Name: "resource1"},
		}, {
			Name:        "test-output-2",
			ResourceRef: v1alpha1.PipelineResourceRef{Name: "resource2"},
		}},
		expectedtaskOuputResources: []v1alpha1.TaskResourceBinding{{
			Name:        "test-output",
			ResourceRef: v1alpha1.PipelineResourceRef{Name: "resource1"},
			Paths:       []string{"/pvc/test-multiple-outputs/test-output"},
		}, {
			Name:        "test-output-2",
			ResourceRef: v1alpha1.PipelineResourceRef{Name: "resource2"},
			Paths:       []string{"/pvc/test-multiple-outputs/test-output-2"},
		}},
		pipelineTaskName: "test-multiple-outputs",
	}}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			postTasks := resources.GetOutputSteps(tc.taskResourceBinding, tc.pipelineTaskName)
			if d := cmp.Diff(postTasks, tc.expectedtaskOuputResources); d != "" {
				t.Errorf("error comparing post steps: %s", d)
			}
		})
	}
}

func Test_GetInputSteps(t *testing.T) {
	tcs := []struct {
		name                       string
		taskResourceBinding        []v1alpha1.TaskResourceBinding
		pipelineTask               *v1alpha1.PipelineTask
		expectedtaskInputResources []v1alpha1.TaskResourceBinding
	}{
		{
			name: "task-with-a-constraint",
			taskResourceBinding: []v1alpha1.TaskResourceBinding{{
				Name:        "test-input",
				ResourceRef: v1alpha1.PipelineResourceRef{Name: "resource1"},
			}},
			pipelineTask: &v1alpha1.PipelineTask{
				ResourceDependencies: []v1alpha1.ResourceDependency{{
					Name:       "test-input",
					ProvidedBy: []string{"prev-task-1"},
				}},
			},
			expectedtaskInputResources: []v1alpha1.TaskResourceBinding{{
				ResourceRef: v1alpha1.PipelineResourceRef{Name: "resource1"},
				Name:        "test-input",
				Paths:       []string{"/pvc/prev-task-1/test-input"},
			}},
		}, {
			name: "task-with-no-input-constraint",
			taskResourceBinding: []v1alpha1.TaskResourceBinding{{
				Name: "test-input",
				ResourceRef: v1alpha1.PipelineResourceRef{
					Name: "resource1",
				},
			}},
			expectedtaskInputResources: []v1alpha1.TaskResourceBinding{{
				ResourceRef: v1alpha1.PipelineResourceRef{Name: "resource1"},
				Name:        "test-input",
			}},
			pipelineTask: &v1alpha1.PipelineTask{
				Name: "sample-test-task",
				ResourceDependencies: []v1alpha1.ResourceDependency{{
					Name: "test-input",
				}},
			},
		}, {
			name: "task-with-multiple-constraints",
			taskResourceBinding: []v1alpha1.TaskResourceBinding{{
				Name:        "test-input",
				ResourceRef: v1alpha1.PipelineResourceRef{Name: "resource1"},
			}},
			pipelineTask: &v1alpha1.PipelineTask{
				ResourceDependencies: []v1alpha1.ResourceDependency{{
					Name:       "test-input",
					ProvidedBy: []string{"prev-task-1", "prev-task-2"},
				}},
			},
			expectedtaskInputResources: []v1alpha1.TaskResourceBinding{{
				ResourceRef: v1alpha1.PipelineResourceRef{Name: "resource1"},
				Name:        "test-input",
				Paths:       []string{"/pvc/prev-task-1/test-input", "/pvc/prev-task-2/test-input"},
			}},
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			taskInputResources := resources.GetInputSteps(tc.taskResourceBinding, tc.pipelineTask)
			if d := cmp.Diff(tc.expectedtaskInputResources, taskInputResources); d != "" {
				t.Errorf("error comparing task resource inputs: %s", d)
			}

		})
	}
}

func Test_WrapSteps(t *testing.T) {
	taskRunSpec := &v1alpha1.TaskRunSpec{}
	pipelineResources := []v1alpha1.PipelineTaskResource{{
		Name: "test-task",
		Inputs: []v1alpha1.TaskResourceBinding{{
			Name:        "test-input",
			ResourceRef: v1alpha1.PipelineResourceRef{Name: "resource1"},
		}, {
			Name:        "test-input-2",
			ResourceRef: v1alpha1.PipelineResourceRef{Name: "resource1"},
		}},
		Outputs: []v1alpha1.TaskResourceBinding{{
			Name:        "test-output",
			ResourceRef: v1alpha1.PipelineResourceRef{Name: "resource1"},
		}},
	}}

	pt := &v1alpha1.PipelineTask{
		Name: "test-task",
		ResourceDependencies: []v1alpha1.ResourceDependency{{
			Name:       "test-input",
			ProvidedBy: []string{"prev-task"},
		}},
	}

	resources.WrapSteps(taskRunSpec, pipelineResources, pt)

	expectedtaskInputResources := []v1alpha1.TaskResourceBinding{{
		ResourceRef: v1alpha1.PipelineResourceRef{Name: "resource1"},
		Name:        "test-input",
		Paths:       []string{"/pvc/prev-task/test-input"},
	}, {
		ResourceRef: v1alpha1.PipelineResourceRef{Name: "resource1"},
		Name:        "test-input-2",
	}}
	expectedtaskOuputResources := []v1alpha1.TaskResourceBinding{{
		ResourceRef: v1alpha1.PipelineResourceRef{Name: "resource1"},
		Name:        "test-output",
		Paths:       []string{"/pvc/test-task/test-output"},
	}}

	if d := cmp.Diff(taskRunSpec.Inputs.Resources, expectedtaskInputResources); d != "" {
		t.Errorf("error comparing input resources: %s", d)
	}
	if d := cmp.Diff(taskRunSpec.Outputs.Resources, expectedtaskOuputResources); d != "" {
		t.Errorf("error comparing output resources: %s", d)
	}
}
