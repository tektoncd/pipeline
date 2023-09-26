/*
Copyright 2022 The Tekton Authors

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

package v1

import (
	"context"

	"knative.dev/pkg/apis"
)

var _ apis.Defaultable = (*Task)(nil)

// SetDefaults implements apis.Defaultable
func (t *Task) SetDefaults(ctx context.Context) {
	t.Spec.SetDefaults(ctx)
}

// SetDefaults set any defaults for the task spec
func (ts *TaskSpec) SetDefaults(ctx context.Context) {
	if len(ts.Inputs.ParamSpecs)!=0 && len(ts.Params)==0{
		for name,value := range ts.Inputs.ParamSpecs{
			psc:=value
			psc.Name=name
			ts.Params = append(ts.Params, psc)
		}
	}
	if len(ts.Outputs.ResultSpecs)!=0 && len(ts.Results)==0{
		for name,value := range ts.Outputs.ResultSpecs{
			rs:=TaskResult{Name: name, Type: value.Type, Properties: value.Properties, Description: value.Description}
			ts.Results = append(ts.Results, rs)
		}
	}
	for i := range ts.Params {
		ts.Params[i].SetDefaults(ctx)
	}
	for i := range ts.Results {
		ts.Results[i].SetDefaults(ctx)
	}
}
