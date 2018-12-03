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

	"github.com/google/go-cmp/cmp"

	"github.com/knative/build-pipeline/pkg/apis/pipeline/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var mytask1 = &v1alpha1.Task{
	ObjectMeta: metav1.ObjectMeta{
		Namespace: "namespace",
		Name:      "mytask1",
	},
	Spec: v1alpha1.TaskSpec{
		Inputs: &v1alpha1.Inputs{
			Resources: []v1alpha1.TaskResource{{
				Name: "myresource1",
				Type: v1alpha1.PipelineResourceTypeGit,
			}},
		},
	},
}

var mytask2 = &v1alpha1.Task{
	ObjectMeta: metav1.ObjectMeta{
		Namespace: "namespace",
		Name:      "mytask2",
	},
	Spec: v1alpha1.TaskSpec{
		Inputs: &v1alpha1.Inputs{
			Resources: []v1alpha1.TaskResource{{
				Name: "myresource1",
				Type: v1alpha1.PipelineResourceTypeGit,
			}},
		},
	},
}

var mypipelinetasks = []v1alpha1.PipelineTask{{
	Name:    "mypipelinetask1",
	TaskRef: v1alpha1.TaskRef{Name: "mytask1"},
}, {
	Name:    "mypipelinetask2",
	TaskRef: v1alpha1.TaskRef{Name: "mytask2"},
	ResourceDependencies: []v1alpha1.ResourceDependency{{
		Name:       "myresource1",
		ProvidedBy: []string{"mypipelinetask1"},
	}},
}}

var mytaskruns = []v1alpha1.TaskRun{{
	ObjectMeta: metav1.ObjectMeta{
		Namespace: "namespace",
		Name:      "pipelinerun-mytask1",
	},
	Spec: v1alpha1.TaskRunSpec{},
}, {
	ObjectMeta: metav1.ObjectMeta{
		Namespace: "namespace",
		Name:      "pipelinerun-mytask2",
	},
	Spec: v1alpha1.TaskRunSpec{},
}}

func TestCanTaskRun(t *testing.T) {
	tcs := []struct {
		name             string
		state            []*ResolvedPipelineRunTask
		canSecondTaskRun bool
	}{
		{
			name: "first-task-not-started",
			state: []*ResolvedPipelineRunTask{{
				PipelineTask: &mypipelinetasks[0],
				TaskRunName:  "pipelinerun-mytask1",
				TaskRun:      nil,
			}, {
				PipelineTask: &mypipelinetasks[1],
				TaskRunName:  "pipelinerun-mytask2",
				TaskRun:      nil,
			}},
			canSecondTaskRun: false,
		},
		{
			name: "first-task-running",
			state: []*ResolvedPipelineRunTask{{
				PipelineTask: &mypipelinetasks[0],
				TaskRunName:  "pipelinerun-mytask1",
				TaskRun:      makeStarted(mytaskruns[0]),
			}, {
				PipelineTask: &mypipelinetasks[1],
				TaskRunName:  "pipelinerun-mytask2",
				TaskRun:      nil,
			}},
			canSecondTaskRun: false,
		},
		{
			name: "first-task-failed",
			state: []*ResolvedPipelineRunTask{{
				PipelineTask: &mypipelinetasks[0],
				TaskRunName:  "pipelinerun-mytask1",
				TaskRun:      makeFailed(mytaskruns[0]),
			}, {
				PipelineTask: &mypipelinetasks[1],
				TaskRunName:  "pipelinerun-mytask2",
				TaskRun:      nil,
			}},
			canSecondTaskRun: false,
		},
		{
			name: "first-task-finished",
			state: []*ResolvedPipelineRunTask{{
				PipelineTask: &mypipelinetasks[0],
				TaskRunName:  "pipelinerun-mytask1",
				TaskRun:      makeSucceeded(mytaskruns[0]),
			}, {
				PipelineTask: &mypipelinetasks[1],
				TaskRunName:  "pipelinerun-mytask2",
				TaskRun:      nil,
			}},
			canSecondTaskRun: true,
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			cantaskrun := canTaskRun(&mypipelinetasks[1], tc.state)
			if d := cmp.Diff(cantaskrun, tc.canSecondTaskRun); d != "" {
				t.Fatalf("Expected second task availability to run should be %t, but different state returned: %s", tc.canSecondTaskRun, d)
			}
		})
	}
}
