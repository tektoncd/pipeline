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
	"knative.dev/pkg/apis"
)

func (source *Task) ConvertUp(ctx context.Context, obj apis.Convertible) error {
	switch sink := obj.(type) {
	case *v1alpha2.Task:
		sink.ObjectMeta = source.ObjectMeta
		return source.Spec.ConvertUp(ctx, &sink.Spec)
	default:
		return fmt.Errorf("unknown version, got: %T", sink)
	}
}

func (sink *Task) ConvertDown(ctx context.Context, obj apis.Convertible) error {
	switch source := obj.(type) {
	case *v1alpha2.Task:
		sink.ObjectMeta = source.ObjectMeta
		return sink.Spec.ConvertDown(ctx, source.Spec)
	default:
		return fmt.Errorf("unknown version, got: %T", source)
	}
}

func (source *TaskSpec) ConvertUp(ctx context.Context, sink *v1alpha2.TaskSpec) error {
	if source.Inputs != nil && len(source.Inputs.DeprecatedParams) > 0 && len(source.Params) > 0 {
		return fmt.Errorf("Cannot have inputs.params and params at the same time")
	}
	// FIXME(vdemeester) handle errors here
	if source.Inputs != nil {
		sink.Inputs = &v1alpha2.Inputs{}
	}
	source.Inputs.ConvertUp(ctx, sink.Inputs)
	if source.Outputs != nil {
		sink.Outputs = &v1alpha2.Outputs{}
	}
	source.Outputs.ConvertUp(ctx, sink.Outputs)
	fmt.Println("-*-*-*-*-*-*-*-*-*-*-*-*-*-*-*-*")
	fmt.Println("source.inputs", source.Inputs)
	fmt.Println("sink.inputs", sink.Inputs)
	fmt.Println("source.outpust", source.Outputs)
	fmt.Println("sink.outputs", sink.Outputs)

	if source.Inputs != nil && len(source.Inputs.DeprecatedParams) > 0 {
		sink.Params = make([]v1alpha2.ParamSpec, len(source.Inputs.DeprecatedParams))
		for i := range source.Inputs.DeprecatedParams {
			source.Inputs.DeprecatedParams[i].ConvertUp(ctx, &sink.Params[i])
		}
	} else {
		sink.Params = make([]v1alpha2.ParamSpec, len(sink.Params))
		for i := range source.Params {
			source.Params[i].ConvertUp(ctx, &sink.Params[i])
		}
	}
	sink.Steps = make([]v1alpha2.Step, len(source.Steps))
	for i := range source.Steps {
		source.Steps[i].ConvertUp(ctx, &sink.Steps[i])
	}

	sink.StepTemplate = source.StepTemplate
	sink.Sidecars = source.Sidecars
	sink.Volumes = source.Volumes
	return nil
}

func (sink *TaskSpec) ConvertDown(ctx context.Context, source v1alpha2.TaskSpec) error {
	// FIXME(vdemeester) make this better (aka handle errors, …)
	if source.Inputs != nil {
		sink.Inputs = &Inputs{}
	}
	sink.Inputs.ConvertDown(ctx, source.Inputs)
	if source.Outputs != nil {
		sink.Outputs = &Outputs{}
	}
	sink.Outputs.ConvertDown(ctx, source.Outputs)

	sink.Params = make([]ParamSpec, len(source.Params))
	for i := range source.Params {
		sink.Params[i].ConvertDown(ctx, source.Params[i])
	}
	sink.Steps = make([]Step, len(source.Steps))
	for i := range source.Steps {
		sink.Steps[i].ConvertDown(ctx, source.Steps[i])
	}

	sink.StepTemplate = source.StepTemplate
	sink.Sidecars = source.Sidecars
	sink.Volumes = source.Volumes
	return nil
}

func (source *Inputs) ConvertUp(ctx context.Context, sink *v1alpha2.Inputs) {
	if source == nil {
		return
	}
	sink.Resources = make([]v1alpha2.TaskResource, len(source.Resources))
	for i := range source.Resources {
		source.Resources[i].ConvertUp(ctx, &sink.Resources[i])
	}
}

func (sink *Inputs) ConvertDown(ctx context.Context, source *v1alpha2.Inputs) {
	if source == nil {
		return
	}
	sink.Resources = make([]TaskResource, len(source.Resources))
	for i := range source.Resources {
		sink.Resources[i].ConvertDown(ctx, source.Resources[i])
	}
}

func (sink *TaskResource) ConvertDown(ctx context.Context, source v1alpha2.TaskResource) {
	sink.Name = source.Name
	sink.Type = PipelineResourceType(source.Type)
	sink.TargetPath = source.TargetPath
}

func (source *TaskResource) ConvertUp(ctx context.Context, sink *v1alpha2.TaskResource) {
	sink.Name = source.Name
	sink.Type = v1alpha2.PipelineResourceType(source.Type)
	sink.TargetPath = source.TargetPath
}

func (sink *Outputs) ConvertDown(ctx context.Context, source *v1alpha2.Outputs) {
	if source == nil {
		return
	}
	sink.Resources = make([]TaskResource, len(source.Resources))
	for i := range source.Resources {
		sink.Resources[i].ConvertDown(ctx, source.Resources[i])
	}
	sink.Results = make([]TestResult, len(source.Results))
	for i := range source.Results {
		sink.Results[i].ConvertDown(ctx, source.Results[i])
	}
}

func (source *Outputs) ConvertUp(ctx context.Context, sink *v1alpha2.Outputs) {
	if source == nil {
		return
	}
	sink.Resources = make([]v1alpha2.TaskResource, len(source.Resources))
	for i := range source.Resources {
		source.Resources[i].ConvertUp(ctx, &sink.Resources[i])
	}
	sink.Results = make([]v1alpha2.TestResult, len(source.Results))
	for i := range source.Results {
		source.Results[i].ConvertUp(ctx, &sink.Results[i])
	}
}

func (sink *TestResult) ConvertDown(ctx context.Context, source v1alpha2.TestResult) {
	sink.Name = source.Name
	sink.Format = source.Format
	sink.Path = source.Path
}

func (source *TestResult) ConvertUp(ctx context.Context, sink *v1alpha2.TestResult) {
	sink.Name = source.Name
	sink.Format = source.Format
	sink.Path = source.Path
}

func (sink *Step) ConvertDown(ctx context.Context, source v1alpha2.Step) {
	sink.Container = source.Container
	sink.Script = source.Script
}

func (source *Step) ConvertUp(ctx context.Context, sink *v1alpha2.Step) {
	sink.Container = source.Container
	sink.Script = source.Script
}

func (sink *ParamSpec) ConvertDown(ctx context.Context, source v1alpha2.ParamSpec) {
	sink.Name = source.Name
	sink.Type = ParamType(source.Type)
	sink.Description = source.Description
	if source.Default != nil {
		sink.Default = &ArrayOrString{
			Type:      ParamType(source.Default.Type),
			StringVal: source.Default.StringVal,
			ArrayVal:  source.Default.ArrayVal,
		}
	}
}

func (source *ParamSpec) ConvertUp(ctx context.Context, sink *v1alpha2.ParamSpec) {
	sink.Name = source.Name
	sink.Type = v1alpha2.ParamType(source.Type)
	sink.Description = source.Description
	if source.Default != nil {
		sink.Default = &v1alpha2.ArrayOrString{
			Type:      v1alpha2.ParamType(source.Default.Type),
			StringVal: source.Default.StringVal,
			ArrayVal:  source.Default.ArrayVal,
		}
	}
}
