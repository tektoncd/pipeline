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
	"sort"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/reconciler/pipelinerun/resources"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var pvcDir = "/pvc"

func TestGetOutputSteps(t *testing.T) {
	r1 := &v1alpha1.PipelineResource{
		ObjectMeta: metav1.ObjectMeta{
			Name:     "resource1",
			SelfLink: "/apis/tekton.dev/pipelineresources/resource1",
		},
	}
	r2 := &v1alpha1.PipelineResource{
		ObjectMeta: metav1.ObjectMeta{
			Name:     "resource2",
			SelfLink: "/apis/tekton.dev/pipelineresources/resource2",
		},
	}
	r3 := &v1alpha1.PipelineResource{
		Spec: v1alpha1.PipelineResourceSpec{
			Type: v1alpha1.PipelineResourceTypeGit,
			Params: []v1alpha1.ResourceParam{{
				Name:  "url",
				Value: "https://github.com/tektoncd/pipeline.git",
			}},
			SecretParams: nil,
		},
	}
	tcs := []struct {
		name                       string
		outputs                    map[string]*v1alpha1.PipelineResource
		expectedtaskOuputResources []v1alpha1.TaskResourceBinding
		pipelineTaskName           string
	}{{
		name:    "single output",
		outputs: map[string]*v1alpha1.PipelineResource{"test-output": r1},
		expectedtaskOuputResources: []v1alpha1.TaskResourceBinding{{
			PipelineResourceBinding: v1alpha1.PipelineResourceBinding{
				Name:        "test-output",
				ResourceRef: &v1alpha1.PipelineResourceRef{Name: "resource1"},
			},
			Paths: []string{"/pvc/test-taskname/test-output"},
		}},
		pipelineTaskName: "test-taskname",
	}, {
		name: "multiple-outputs",
		outputs: map[string]*v1alpha1.PipelineResource{
			"test-output":   r1,
			"test-output-2": r2,
		},
		expectedtaskOuputResources: []v1alpha1.TaskResourceBinding{{
			PipelineResourceBinding: v1alpha1.PipelineResourceBinding{
				Name:        "test-output",
				ResourceRef: &v1alpha1.PipelineResourceRef{Name: "resource1"},
			},
			Paths: []string{"/pvc/test-multiple-outputs/test-output"},
		}, {
			PipelineResourceBinding: v1alpha1.PipelineResourceBinding{
				Name:        "test-output-2",
				ResourceRef: &v1alpha1.PipelineResourceRef{Name: "resource2"},
			},
			Paths: []string{"/pvc/test-multiple-outputs/test-output-2"},
		}},
		pipelineTaskName: "test-multiple-outputs",
	}, {
		name:    "single output with resource spec",
		outputs: map[string]*v1alpha1.PipelineResource{"test-output": r3},
		expectedtaskOuputResources: []v1alpha1.TaskResourceBinding{{
			PipelineResourceBinding: v1alpha1.PipelineResourceBinding{
				Name:         "test-output",
				ResourceSpec: &r3.Spec,
			},
			Paths: []string{"/pvc/test-taskname/test-output"},
		}},
		pipelineTaskName: "test-taskname",
	}, {
		name: "multiple-outputs-with-resource-spec",
		outputs: map[string]*v1alpha1.PipelineResource{
			"test-output-1": r3,
			"test-output-2": r3,
		},
		expectedtaskOuputResources: []v1alpha1.TaskResourceBinding{{
			PipelineResourceBinding: v1alpha1.PipelineResourceBinding{
				Name:         "test-output-1",
				ResourceSpec: &r3.Spec,
			},
			Paths: []string{"/pvc/test-multiple-outputs-with-resource-spec/test-output-1"},
		}, {
			PipelineResourceBinding: v1alpha1.PipelineResourceBinding{
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
			sort.SliceStable(postTasks, func(i, j int) bool { return postTasks[i].Name < postTasks[j].Name })
			if d := cmp.Diff(postTasks, tc.expectedtaskOuputResources); d != "" {
				t.Errorf("error comparing post steps: %s", d)
			}
		})
	}
}

func TestGetInputSteps(t *testing.T) {
	r1 := &v1alpha1.PipelineResource{
		ObjectMeta: metav1.ObjectMeta{
			Name:     "resource1",
			SelfLink: "/apis/tekton.dev/pipelineresources/resource1",
		},
	}
	r2 := &v1alpha1.PipelineResource{
		Spec: v1alpha1.PipelineResourceSpec{
			Type: v1alpha1.PipelineResourceTypeGit,
			Params: []v1alpha1.ResourceParam{{
				Name:  "url",
				Value: "https://github.com/tektoncd/pipeline.git",
			}},
			SecretParams: nil,
		},
	}
	tcs := []struct {
		name                       string
		inputs                     map[string]*v1alpha1.PipelineResource
		pipelineTask               *v1alpha1.PipelineTask
		expectedtaskInputResources []v1alpha1.TaskResourceBinding
	}{
		{
			name:   "task-with-a-constraint",
			inputs: map[string]*v1alpha1.PipelineResource{"test-input": r1},
			pipelineTask: &v1alpha1.PipelineTask{
				Resources: &v1alpha1.PipelineTaskResources{
					Inputs: []v1alpha1.PipelineTaskInputResource{{
						Name: "test-input",
						From: []string{"prev-task-1"},
					}},
				},
			},
			expectedtaskInputResources: []v1alpha1.TaskResourceBinding{{
				PipelineResourceBinding: v1alpha1.PipelineResourceBinding{
					ResourceRef: &v1alpha1.PipelineResourceRef{Name: "resource1"},
					Name:        "test-input",
				},
				Paths: []string{"/pvc/prev-task-1/test-input"},
			}},
		}, {
			name:   "task-with-no-input-constraint",
			inputs: map[string]*v1alpha1.PipelineResource{"test-input": r1},
			expectedtaskInputResources: []v1alpha1.TaskResourceBinding{{
				PipelineResourceBinding: v1alpha1.PipelineResourceBinding{
					ResourceRef: &v1alpha1.PipelineResourceRef{Name: "resource1"},
					Name:        "test-input",
				},
			}},
			pipelineTask: &v1alpha1.PipelineTask{
				Name: "sample-test-task",
				Resources: &v1alpha1.PipelineTaskResources{
					Inputs: []v1alpha1.PipelineTaskInputResource{{
						Name: "test-input",
					}},
				},
			},
		}, {
			name:   "task-with-multiple-constraints",
			inputs: map[string]*v1alpha1.PipelineResource{"test-input": r1},
			pipelineTask: &v1alpha1.PipelineTask{
				Resources: &v1alpha1.PipelineTaskResources{
					Inputs: []v1alpha1.PipelineTaskInputResource{{
						Name: "test-input",
						From: []string{"prev-task-1", "prev-task-2"},
					}},
				},
			},
			expectedtaskInputResources: []v1alpha1.TaskResourceBinding{{
				PipelineResourceBinding: v1alpha1.PipelineResourceBinding{
					ResourceRef: &v1alpha1.PipelineResourceRef{Name: "resource1"},
					Name:        "test-input",
				},
				Paths: []string{"/pvc/prev-task-1/test-input", "/pvc/prev-task-2/test-input"},
			}},
		}, {
			name:   "task-with-a-constraint-with-resource-spec",
			inputs: map[string]*v1alpha1.PipelineResource{"test-input": r2},
			pipelineTask: &v1alpha1.PipelineTask{
				Resources: &v1alpha1.PipelineTaskResources{
					Inputs: []v1alpha1.PipelineTaskInputResource{{
						Name: "test-input",
						From: []string{"prev-task-1"},
					}},
				},
			},
			expectedtaskInputResources: []v1alpha1.TaskResourceBinding{{
				PipelineResourceBinding: v1alpha1.PipelineResourceBinding{
					ResourceSpec: &r2.Spec,
					Name:         "test-input",
				},
				Paths: []string{"/pvc/prev-task-1/test-input"},
			}},
		}, {
			name:   "task-with-no-input-constraint-but-with-resource-spec",
			inputs: map[string]*v1alpha1.PipelineResource{"test-input": r2},
			expectedtaskInputResources: []v1alpha1.TaskResourceBinding{{
				PipelineResourceBinding: v1alpha1.PipelineResourceBinding{
					ResourceSpec: &r2.Spec,
					Name:         "test-input",
				},
			}},
			pipelineTask: &v1alpha1.PipelineTask{
				Name: "sample-test-task",
				Resources: &v1alpha1.PipelineTaskResources{
					Inputs: []v1alpha1.PipelineTaskInputResource{{
						Name: "test-input",
					}},
				},
			},
		}, {
			name:   "task-with-multiple-constraints-with-resource-spec",
			inputs: map[string]*v1alpha1.PipelineResource{"test-input": r2},
			pipelineTask: &v1alpha1.PipelineTask{
				Resources: &v1alpha1.PipelineTaskResources{
					Inputs: []v1alpha1.PipelineTaskInputResource{{
						Name: "test-input",
						From: []string{"prev-task-1", "prev-task-2"},
					}},
				},
			},
			expectedtaskInputResources: []v1alpha1.TaskResourceBinding{{
				PipelineResourceBinding: v1alpha1.PipelineResourceBinding{
					ResourceSpec: &r2.Spec,
					Name:         "test-input",
				},
				Paths: []string{"/pvc/prev-task-1/test-input", "/pvc/prev-task-2/test-input"},
			}},
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			taskInputResources := resources.GetInputSteps(tc.inputs, tc.pipelineTask.Resources.Inputs, pvcDir)
			sort.SliceStable(taskInputResources, func(i, j int) bool { return taskInputResources[i].Name < taskInputResources[j].Name })
			if d := cmp.Diff(tc.expectedtaskInputResources, taskInputResources); d != "" {
				t.Errorf("error comparing task resource inputs: %s", d)
			}

		})
	}
}

func TestWrapSteps(t *testing.T) {
	r1 := &v1alpha1.PipelineResource{
		ObjectMeta: metav1.ObjectMeta{
			Name:     "resource1",
			SelfLink: "/apis/tekton.dev/pipelineresources/resource1",
		},
	}
	r2 := &v1alpha1.PipelineResource{
		Spec: v1alpha1.PipelineResourceSpec{
			Type: v1alpha1.PipelineResourceTypeGit,
			Params: []v1alpha1.ResourceParam{{
				Name:  "url",
				Value: "https://github.com/tektoncd/pipeline.git",
			}},
			SecretParams: nil,
		},
	}
	inputs := map[string]*v1alpha1.PipelineResource{
		"test-input":   r1,
		"test-input-2": r1,
		"test-input-3": r2,
	}
	outputs := map[string]*v1alpha1.PipelineResource{
		"test-output":   r1,
		"test-output-2": r2,
	}

	pt := &v1alpha1.PipelineTask{
		Name: "test-task",
		Resources: &v1alpha1.PipelineTaskResources{
			Inputs: []v1alpha1.PipelineTaskInputResource{{
				Name: "test-input",
				From: []string{"prev-task"},
			}},
		},
	}

	taskRunSpec := &v1alpha1.TaskRunSpec{}
	resources.WrapSteps(taskRunSpec, pt, inputs, outputs, pvcDir)

	expectedtaskInputResources := []v1alpha1.TaskResourceBinding{{
		PipelineResourceBinding: v1alpha1.PipelineResourceBinding{
			ResourceRef: &v1alpha1.PipelineResourceRef{Name: "resource1"},
			Name:        "test-input",
		},
		Paths: []string{"/pvc/prev-task/test-input"},
	}, {
		PipelineResourceBinding: v1alpha1.PipelineResourceBinding{
			ResourceRef: &v1alpha1.PipelineResourceRef{Name: "resource1"},
			Name:        "test-input-2",
		},
	}, {
		PipelineResourceBinding: v1alpha1.PipelineResourceBinding{
			ResourceSpec: &r2.Spec,
			Name:         "test-input-3",
		},
	}}
	expectedtaskOuputResources := []v1alpha1.TaskResourceBinding{{
		PipelineResourceBinding: v1alpha1.PipelineResourceBinding{
			ResourceRef: &v1alpha1.PipelineResourceRef{Name: "resource1"},
			Name:        "test-output",
		},
		Paths: []string{"/pvc/test-task/test-output"},
	}, {
		PipelineResourceBinding: v1alpha1.PipelineResourceBinding{
			ResourceSpec: &r2.Spec,
			Name:         "test-output-2",
		},
		Paths: []string{"/pvc/test-task/test-output-2"},
	}}

	sort.SliceStable(expectedtaskInputResources, func(i, j int) bool { return expectedtaskInputResources[i].Name < expectedtaskInputResources[j].Name })
	sort.SliceStable(expectedtaskOuputResources, func(i, j int) bool { return expectedtaskOuputResources[i].Name < expectedtaskOuputResources[j].Name })

	if d := cmp.Diff(taskRunSpec.Inputs.Resources, expectedtaskInputResources, cmpopts.SortSlices(func(x, y v1alpha1.TaskResourceBinding) bool { return x.Name < y.Name })); d != "" {
		t.Errorf("error comparing input resources: %s", d)
	}
	if d := cmp.Diff(taskRunSpec.Outputs.Resources, expectedtaskOuputResources, cmpopts.SortSlices(func(x, y v1alpha1.TaskResourceBinding) bool { return x.Name < y.Name })); d != "" {
		t.Errorf("error comparing output resources: %s", d)
	}
}
