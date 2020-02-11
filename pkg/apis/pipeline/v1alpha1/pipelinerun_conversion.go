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

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha2"
	"knative.dev/pkg/apis"
)

var _ apis.Convertible = (*PipelineRun)(nil)

// ConvertUp implements api.Convertible
func (source *PipelineRun) ConvertUp(ctx context.Context, obj apis.Convertible) error {
	switch sink := obj.(type) {
	case *v1alpha2.PipelineRun:
		sink.ObjectMeta = source.ObjectMeta
		if err := source.Spec.ConvertUp(ctx, &sink.Spec); err != nil {
			return err
		}
		sink.Status = source.Status
		return nil
	default:
		return fmt.Errorf("unknown version, got: %T", sink)
	}
}

func (source *PipelineRunSpec) ConvertUp(ctx context.Context, sink *v1alpha2.PipelineRunSpec) error {
	sink.PipelineRef = source.PipelineRef
	if source.PipelineSpec != nil {
		sink.PipelineSpec = &v1alpha2.PipelineSpec{}
		if err := source.PipelineSpec.ConvertUp(ctx, sink.PipelineSpec); err != nil {
			return err
		}
	}
	sink.Resources = source.Resources
	sink.Params = source.Params
	sink.ServiceAccountName = source.ServiceAccountName
	sink.ServiceAccountNames = source.ServiceAccountNames
	sink.Status = source.Status
	sink.Timeout = source.Timeout
	sink.PodTemplate = source.PodTemplate
	sink.Workspaces = source.Workspaces
	sink.LimitRangeName = source.LimitRangeName
	return nil
}

// ConvertDown implements api.Convertible
func (sink *PipelineRun) ConvertDown(ctx context.Context, obj apis.Convertible) error {
	switch source := obj.(type) {
	case *v1alpha2.PipelineRun:
		sink.ObjectMeta = source.ObjectMeta
		if err := sink.Spec.ConvertDown(ctx, &source.Spec); err != nil {
			return err
		}
		sink.Status = source.Status
		return nil
	default:
		return fmt.Errorf("unknown version, got: %T", sink)
	}
}

func (sink *PipelineRunSpec) ConvertDown(ctx context.Context, source *v1alpha2.PipelineRunSpec) error {
	sink.PipelineRef = source.PipelineRef
	if source.PipelineSpec != nil {
		sink.PipelineSpec = &PipelineSpec{}
		if err := sink.PipelineSpec.ConvertDown(ctx, *source.PipelineSpec); err != nil {
			return err
		}
	}
	sink.Resources = source.Resources
	sink.Params = source.Params
	sink.ServiceAccountName = source.ServiceAccountName
	sink.ServiceAccountNames = source.ServiceAccountNames
	sink.Status = source.Status
	sink.Timeout = source.Timeout
	sink.PodTemplate = source.PodTemplate
	sink.Workspaces = source.Workspaces
	sink.LimitRangeName = source.LimitRangeName
	return nil
}
