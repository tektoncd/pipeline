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

	v1beta1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"knative.dev/pkg/apis"
)

var _ apis.Convertible = (*Run)(nil)

// ConvertTo implements apis.Convertible
func (r *Run) ConvertTo(ctx context.Context, to apis.Convertible) error {
	if apis.IsInDelete(ctx) {
		return nil
	}
	switch sink := to.(type) {
	case *v1beta1.CustomRun:
		sink.ObjectMeta = r.ObjectMeta
		return r.Spec.ConvertTo(ctx, &sink.Spec)
	default:
		return fmt.Errorf("unknown version, got: %T", sink)
	}
}

// ConvertTo implements apis.Convertible
func (rs RunSpec) ConvertTo(ctx context.Context, sink *v1beta1.CustomRunSpec) error {
	if rs.Spec != nil {
		sink.CustomSpec = &v1beta1.EmbeddedCustomRunSpec{}
		err := rs.Spec.ConvertTo(ctx, sink.CustomSpec)
		if err != nil {
			return err
		}
	}

	sink.Retries = rs.Retries
	sink.ServiceAccountName = rs.ServiceAccountName
	sink.Status = v1beta1.CustomRunSpecStatus(rs.Status)
	sink.StatusMessage = v1beta1.CustomRunSpecStatusMessage(rs.StatusMessage)

	return nil
}

// ConvertFrom implements apis.Convertible
func (r *Run) ConvertFrom(ctx context.Context, from apis.Convertible) error {
	switch source := from.(type) {
	case *v1beta1.CustomRun:
		r.ObjectMeta = source.ObjectMeta
		return r.Spec.ConvertFrom(ctx, &source.Spec)
	default:
		return fmt.Errorf("unknown version, got: %T", r)
	}
}

// ConvertFrom implements apis.Convertible
func (rs *RunSpec) ConvertFrom(ctx context.Context, source *v1beta1.CustomRunSpec) error {

	if source.CustomSpec != nil {
		newPipelineSpec := EmbeddedRunSpec{}
		err := newPipelineSpec.ConvertFrom(ctx, source.CustomSpec)
		if err != nil {
			return err
		}
		rs.Spec = &newPipelineSpec
	}

	rs.Retries = source.Retries
	rs.ServiceAccountName = source.ServiceAccountName
	rs.Status = RunSpecStatus(source.Status)
	rs.StatusMessage = RunSpecStatusMessage(source.StatusMessage)

	return nil
}
