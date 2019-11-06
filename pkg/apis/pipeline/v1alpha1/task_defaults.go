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

package v1alpha1

import (
	"context"
	"fmt"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha2"
)

func (t *Task) SetDefaults(ctx context.Context) {
	t.Spec.SetDefaults(ctx)
}

// SetDefaults set any defaults for the task spec
func (ts *TaskSpec) SetDefaults(ctx context.Context) {
	if v1alpha2.IsUpgradeViaDefaulting(ctx) {
		fmt.Println("********************************************")
		v := v1alpha2.TaskSpec{}
		if ts.ConvertUp(ctx, &v) == nil {
			fmt.Printf("v: %+v\n", v)
			fmt.Printf("v.inputs: %+v\n", v.Inputs)
			fmt.Printf("v.outputs: %+v\n", v.Outputs)
			alpha1 := TaskSpec{}
			if alpha1.ConvertDown(ctx, v) == nil {
				*ts = alpha1
			}
		}
	}
	fmt.Printf("ts: %+v\n", ts)
	if ts.Inputs != nil {
		ts.Inputs.SetDefaults(ctx)
	}
}

func (inputs *Inputs) SetDefaults(ctx context.Context) {
	for i := range inputs.DeprecatedParams {
		inputs.DeprecatedParams[i].SetDefaults(ctx)
	}
}
