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

func (source *TaskRun) ConvertUp(ctx context.Context, obj apis.Convertible) error {
	switch sink := obj.(type) {
	case *v1alpha2.TaskRun:
		sink.ObjectMeta = source.ObjectMeta
		return source.Spec.ConvertUp(ctx, &sink.Spec)
	default:
		return fmt.Errorf("unknown version, got: %T", sink)
	}
}

func (source *TaskRunSpec) ConvertUp(ctx context.Context, sink *v1alpha2.TaskRunSpec) error {
	if len(source.Inputs.DeprecatedParams) > 0 {
		for _, p := range source.Inputs.DeprecatedParams {
			param := v1alpha2.Param{
				Name: p.Name,
				Value: v1alpha2.ArrayOrString{
					Type:      v1alpha2.ParamType(p.Value.Type),
					StringVal: p.Value.StringVal,
					ArrayVal:  p.Value.ArrayVal,
				},
			}
			sink.Params = append(sink.Params, param)
		}
	}
	return nil
}

func (sink *TaskRun) ConvertDown(ctx context.Context, obj apis.Convertible) error {
	switch source := obj.(type) {
	case *v1alpha2.TaskRun:
		sink.ObjectMeta = source.ObjectMeta
		return sink.Spec.ConvertDown(ctx, source.Spec)
	default:
		return fmt.Errorf("unknown version, got: %T", source)
	}
}

func (sink *TaskRunSpec) ConvertDown(ctx context.Context, source v1alpha2.TaskRunSpec) error {
	if len(source.Params) > 0 {
		for _, p := range source.Params {
			param := Param{
				Name: p.Name,
				Value: ArrayOrString{
					Type:      ParamType(p.Value.Type),
					StringVal: p.Value.StringVal,
					ArrayVal:  p.Value.ArrayVal,
				},
			}
			sink.Inputs.DeprecatedParams = append(sink.Inputs.DeprecatedParams, param)
		}
	}
	return nil
}
