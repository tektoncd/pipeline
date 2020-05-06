/*
Copyright 2020 The Tekton Authors

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

// nolint: golint
package v1alpha1

import (
	"context"
	"fmt"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"knative.dev/pkg/apis"
)

const FinallyFieldName = "finally"

var _ apis.Convertible = (*Pipeline)(nil)

// ConvertTo implements api.Convertible
func (source *Pipeline) ConvertTo(ctx context.Context, obj apis.Convertible) error {
	switch sink := obj.(type) {
	case *v1beta1.Pipeline:
		sink.ObjectMeta = source.ObjectMeta
		return source.Spec.ConvertTo(ctx, &sink.Spec)
	default:
		return fmt.Errorf("unknown version, got: %T", sink)
	}
}

func (source *PipelineSpec) ConvertTo(ctx context.Context, sink *v1beta1.PipelineSpec) error {
	sink.Resources = source.Resources
	sink.Params = source.Params
	sink.Workspaces = source.Workspaces
	sink.Description = source.Description
	if len(source.Tasks) > 0 {
		sink.Tasks = make([]v1beta1.PipelineTask, len(source.Tasks))
		for i := range source.Tasks {
			if err := source.Tasks[i].ConvertTo(ctx, &sink.Tasks[i]); err != nil {
				return err
			}
		}
	}
	sink.Finally = nil
	return nil
}

func (source *PipelineTask) ConvertTo(ctx context.Context, sink *v1beta1.PipelineTask) error {
	sink.Name = source.Name
	sink.TaskRef = source.TaskRef
	if source.TaskSpec != nil {
		sink.TaskSpec = &v1beta1.TaskSpec{}
		if err := source.TaskSpec.ConvertTo(ctx, sink.TaskSpec); err != nil {
			return err
		}
	}
	sink.Conditions = source.Conditions
	sink.Retries = source.Retries
	sink.RunAfter = source.RunAfter
	sink.Resources = source.Resources
	sink.Params = source.Params
	sink.Workspaces = source.Workspaces
	sink.Timeout = source.Timeout
	return nil
}

// ConvertFrom implements api.Convertible
func (sink *Pipeline) ConvertFrom(ctx context.Context, obj apis.Convertible) error {
	switch source := obj.(type) {
	case *v1beta1.Pipeline:
		sink.ObjectMeta = source.ObjectMeta
		return sink.Spec.ConvertFrom(ctx, source.Spec)
	default:
		return fmt.Errorf("unknown version, got: %T", sink)
	}
}

func (sink *PipelineSpec) ConvertFrom(ctx context.Context, source v1beta1.PipelineSpec) error {
	sink.Resources = source.Resources
	sink.Params = source.Params
	sink.Workspaces = source.Workspaces
	sink.Description = source.Description
	if len(source.Tasks) > 0 {
		sink.Tasks = make([]PipelineTask, len(source.Tasks))
		for i := range source.Tasks {
			if err := sink.Tasks[i].ConvertFrom(ctx, source.Tasks[i]); err != nil {
				return err
			}
		}
	}
	// finally clause was introduced in v1beta1 and not available in v1alpha1
	if len(source.Finally) > 0 {
		return ConvertErrorf(FinallyFieldName, ConversionErrorFieldNotAvailableMsg)
	}
	return nil
}

func (sink *PipelineTask) ConvertFrom(ctx context.Context, source v1beta1.PipelineTask) error {
	sink.Name = source.Name
	sink.TaskRef = source.TaskRef
	if source.TaskSpec != nil {
		sink.TaskSpec = &TaskSpec{}
		if err := sink.TaskSpec.ConvertFrom(ctx, source.TaskSpec); err != nil {
			return err
		}
	}
	sink.Conditions = source.Conditions
	sink.Retries = source.Retries
	sink.RunAfter = source.RunAfter
	sink.Resources = source.Resources
	sink.Params = source.Params
	sink.Workspaces = source.Workspaces
	sink.Timeout = source.Timeout
	return nil
}
