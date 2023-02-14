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
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
)

// ResolvedTaskResources contains all the data that is needed to execute
// the TaskRun: the TaskRun, it's Task and the PipelineResources it needs.
type ResolvedTaskResources struct {
	TaskName string
	Kind     v1beta1.TaskKind
	TaskSpec *v1beta1.TaskSpec
}

// ResolveTaskResources looks up PipelineResources referenced by inputs and outputs and returns
// a structure that unites the resolved references and the Task Spec. If referenced PipelineResources
// can't be found, an error is returned.
func ResolveTaskResources(ts *v1beta1.TaskSpec, taskName string, kind v1beta1.TaskKind) (*ResolvedTaskResources, error) {
	rtr := ResolvedTaskResources{
		TaskName: taskName,
		TaskSpec: ts,
		Kind:     kind,
	}
	return &rtr, nil
}
