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

package v1alpha1

import (
	"context"
	"fmt"
	"strings"

	pipelinev1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/apis/resolution/v1beta1"
	"knative.dev/pkg/apis"
)

var _ apis.Convertible = (*ResolutionRequest)(nil)

// ConvertTo implements apis.Convertible
func (rr *ResolutionRequest) ConvertTo(ctx context.Context, sink apis.Convertible) error {
	if apis.IsInDelete(ctx) {
		return nil
	}
	switch sink := sink.(type) {
	case *v1beta1.ResolutionRequest:
		sink.ObjectMeta = rr.ObjectMeta
		rr.Status.convertTo(ctx, &sink.Status)
		return rr.Spec.ConvertTo(ctx, &sink.Spec)
	default:
		return fmt.Errorf("unknown version, got: %T", sink)
	}
}

// ConvertTo converts a v1alpha1.ResolutionRequestSpec to a v1beta1.ResolutionRequestSpec
func (rrs *ResolutionRequestSpec) ConvertTo(ctx context.Context, sink *v1beta1.ResolutionRequestSpec) error {
	for k, v := range rrs.Parameters {
		sink.Params = append(sink.Params, pipelinev1.Param{
			Name: k,
			Value: pipelinev1.ParamValue{
				Type:      pipelinev1.ParamTypeString,
				StringVal: v,
			},
		})
	}

	return nil
}

// convertTo converts a v1alpha1.ResolutionRequestStatus to a v1beta1.ResolutionRequestStatus
func (rrs *ResolutionRequestStatus) convertTo(ctx context.Context, sink *v1beta1.ResolutionRequestStatus) {
	sink.Data = rrs.Data
	if rrs.RefSource != nil {
		refSource := pipelinev1.RefSource{}
		refSource.URI = rrs.RefSource.URI
		refSource.EntryPoint = rrs.RefSource.EntryPoint
		digest := make(map[string]string)
		for k, v := range rrs.RefSource.Digest {
			digest[k] = v
		}
		refSource.Digest = digest
		sink.RefSource = &refSource
	}
}

// ConvertFrom implements apis.Convertible
func (rr *ResolutionRequest) ConvertFrom(ctx context.Context, from apis.Convertible) error {
	if apis.IsInDelete(ctx) {
		return nil
	}
	switch from := from.(type) {
	case *v1beta1.ResolutionRequest:
		rr.ObjectMeta = from.ObjectMeta
		rr.Status.convertFrom(ctx, &from.Status)
		return rr.Spec.ConvertFrom(ctx, &from.Spec)
	default:
		return fmt.Errorf("unknown version, got: %T", from)
	}
}

// ConvertFrom converts a v1beta1.ResolutionRequestSpec to a v1alpha1.ResolutionRequestSpec
func (rrs *ResolutionRequestSpec) ConvertFrom(ctx context.Context, from *v1beta1.ResolutionRequestSpec) error {
	var nonStringParams []string

	for _, p := range from.Params {
		if p.Value.Type != pipelinev1.ParamTypeString {
			nonStringParams = append(nonStringParams, p.Name)
		} else {
			if rrs.Parameters == nil {
				rrs.Parameters = make(map[string]string)
			}
			rrs.Parameters[p.Name] = p.Value.StringVal
		}
	}

	if len(nonStringParams) > 0 {
		return fmt.Errorf("cannot convert v1beta1 to v1alpha, non-string type parameter(s) found: %s", strings.Join(nonStringParams, ", "))
	}

	return nil
}

// convertTo converts a v1alpha1.ResolutionRequestStatus to a v1beta1.ResolutionRequestStatus
func (rrs *ResolutionRequestStatus) convertFrom(ctx context.Context, from *v1beta1.ResolutionRequestStatus) {
	rrs.Data = from.Data

	if from.RefSource != nil {
		refSource := pipelinev1.RefSource{}
		refSource.URI = from.RefSource.URI
		refSource.EntryPoint = from.RefSource.EntryPoint
		digest := make(map[string]string)
		for k, v := range from.RefSource.Digest {
			digest[k] = v
		}
		refSource.Digest = digest
		rrs.RefSource = &refSource
	} else if from.Source != nil {
		refSource := pipelinev1.RefSource{}
		refSource.URI = from.Source.URI
		refSource.EntryPoint = from.Source.EntryPoint
		digest := make(map[string]string)
		for k, v := range from.Source.Digest {
			digest[k] = v
		}
		refSource.Digest = digest
		rrs.RefSource = &refSource
	}
}
