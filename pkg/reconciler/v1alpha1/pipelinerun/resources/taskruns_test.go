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

package resources

import (
	"testing"

	"k8s.io/apimachinery/pkg/api/errors"

	"github.com/knative/build-pipeline/pkg/apis/pipeline/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	namespace = "foo"
)

func TestGetNextPipelineRunTaskRun(t *testing.T) {
	ps := []*v1alpha1.Pipeline{{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pipeline",
			Namespace: namespace,
		},
		Spec: v1alpha1.PipelineSpec{
			Tasks: []v1alpha1.PipelineTask{{
				Name:    "unit-test-1",
				TaskRef: v1alpha1.TaskRef{Name: "unit-test-task"},
			}, {
				Name:    "unit-test-2",
				TaskRef: v1alpha1.TaskRef{Name: "unit-test-task"},
			}},
		}}}
	pr := v1alpha1.PipelineRun{ObjectMeta: metav1.ObjectMeta{
		Name:      "mypipelinerun",
		Namespace: namespace,
	}}
	tcs := []struct {
		name                 string
		expectedPipelineTask string
		expectedTaskRunName  string
		getTaskRun           GetTaskRun
	}{
		{
			name:                 "shd-kick-first-task",
			expectedPipelineTask: ps[0].Spec.Tasks[0].Name,
			expectedTaskRunName:  "mypipelinerun-unit-test-1",
			getTaskRun: func(ns, name string) (*v1alpha1.TaskRun, error) {
				return nil, errors.NewNotFound(v1alpha1.Resource("taskrun"), name)
			},
		},
		{
			name:                 "shd-kick-second-task",
			expectedPipelineTask: ps[0].Spec.Tasks[1].Name,
			expectedTaskRunName:  "mypipelinerun-unit-test-2",
			getTaskRun: func(ns, name string) (*v1alpha1.TaskRun, error) {
				// Return the first TaskRun as if it has already been created
				if name == "mypipelinerun-unit-test-1" {
					return &v1alpha1.TaskRun{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "mypipelinerun-unit-test-1",
							Namespace: namespace,
						},
						Spec: v1alpha1.TaskRunSpec{},
					}, nil
				}
				return nil, errors.NewNotFound(v1alpha1.Resource("taskrun"), name)
			},
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			pipelineTaskName, trName, err := GetNextPipelineRunTaskRun(tc.getTaskRun, ps[0], pr.Name)
			if err != nil {
				t.Fatalf("Got error getting name of next Task to Run: %s", err)
			}
			if pipelineTaskName != tc.expectedPipelineTask {
				t.Errorf("Expected to try to create %s but was %s instead", tc.expectedPipelineTask, pipelineTaskName)
			}
			if trName != tc.expectedTaskRunName {
				t.Errorf("Expected to return TaskRun name %s but was %s instead", tc.expectedTaskRunName, trName)
			}
		})
	}
}
