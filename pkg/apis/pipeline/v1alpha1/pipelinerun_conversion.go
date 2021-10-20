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

package v1alpha1

import (
	"context"
	"fmt"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"knative.dev/pkg/apis"
)

var _ apis.Convertible = (*PipelineRun)(nil)

// ConvertTo implements api.Convertible
func (pr *PipelineRun) ConvertTo(ctx context.Context, obj apis.Convertible) error {
	switch sink := obj.(type) {
	case *v1beta1.PipelineRun:
		sink.ObjectMeta = pr.ObjectMeta
		if err := pr.Spec.ConvertTo(ctx, &sink.Spec); err != nil {
			return err
		}
		sink.Status = pr.Status

		spec := &v1beta1.PipelineSpec{}
		if err := deserializeFinally(&sink.ObjectMeta, spec); err != nil {
			return err
		}
		if len(spec.Finally) > 0 {
			if sink.Spec.PipelineSpec == nil {
				sink.Spec.PipelineSpec = spec
			} else {
				sink.Spec.PipelineSpec.Finally = spec.Finally
			}
		}
		return nil
	default:
		return fmt.Errorf("unknown version, got: %T", sink)
	}
}

// ConvertTo implements api.Convertible
func (prs *PipelineRunSpec) ConvertTo(ctx context.Context, sink *v1beta1.PipelineRunSpec) error {
	sink.PipelineRef = prs.PipelineRef
	if prs.PipelineSpec != nil {
		sink.PipelineSpec = &v1beta1.PipelineSpec{}
		if err := prs.PipelineSpec.ConvertTo(ctx, sink.PipelineSpec); err != nil {
			return err
		}
	}
	sink.Resources = prs.Resources
	sink.Params = prs.Params
	sink.ServiceAccountName = prs.ServiceAccountName
	sink.ServiceAccountNames = prs.ServiceAccountNames
	sink.Status = prs.Status
	sink.Timeout = prs.Timeout
	sink.PodTemplate = prs.PodTemplate
	sink.Workspaces = prs.Workspaces
	return nil
}

// ConvertFrom implements api.Convertible
func (pr *PipelineRun) ConvertFrom(ctx context.Context, obj apis.Convertible) error {
	switch source := obj.(type) {
	case *v1beta1.PipelineRun:
		pr.ObjectMeta = source.ObjectMeta
		if err := pr.Spec.ConvertFrom(ctx, &source.Spec); err != nil {
			return err
		}
		pr.Status = source.Status

		ps := source.Spec.PipelineSpec
		if ps != nil && ps.Finally != nil {
			if err := serializeFinally(&pr.ObjectMeta, ps.Finally); err != nil {
				return err
			}
		}
		return nil
	default:
		return fmt.Errorf("unknown version, got: %T", pr)
	}
}

// ConvertFrom implements api.Convertible
func (prs *PipelineRunSpec) ConvertFrom(ctx context.Context, source *v1beta1.PipelineRunSpec) error {
	prs.PipelineRef = source.PipelineRef
	if source.PipelineSpec != nil {
		prs.PipelineSpec = &PipelineSpec{}
		if err := prs.PipelineSpec.ConvertFrom(ctx, *source.PipelineSpec); err != nil {
			return err
		}
	}
	prs.Resources = source.Resources
	prs.Params = source.Params
	prs.ServiceAccountName = source.ServiceAccountName
	prs.ServiceAccountNames = source.ServiceAccountNames
	prs.Status = source.Status
	prs.Timeout = source.Timeout
	prs.PodTemplate = source.PodTemplate
	prs.Workspaces = source.Workspaces
	return nil
}
