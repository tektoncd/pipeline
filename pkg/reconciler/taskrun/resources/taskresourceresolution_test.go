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

package resources

import (
	"testing"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
)

func TestResolveTaskRun(t *testing.T) {
	taskName := "orchestrate"
	kind := v1beta1.NamespacedTaskKind
	taskSpec := v1beta1.TaskSpec{
		Steps: []v1beta1.Step{{
			Name: "step1",
		},
		}}

	rtr, err := ResolveTaskResources(&taskSpec, taskName, kind)
	if err != nil {
		t.Fatalf("Did not expect error trying to resolve TaskRun: %s", err)
	}

	if rtr.TaskName != "orchestrate" {
		t.Errorf("Expected task name `orchestrate` Task but got: %v", rtr.TaskName)
	}
	if rtr.TaskSpec == nil || len(rtr.TaskSpec.Steps) != 1 || rtr.TaskSpec.Steps[0].Name != "step1" {
		t.Errorf("Task not resolved, expected task's spec to be used but spec was: %v", rtr.TaskSpec)
	}
}
