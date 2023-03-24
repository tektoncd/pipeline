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

package pipelinespec

import (
	"context"
	"errors"
	"fmt"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	resolutionutil "github.com/tektoncd/pipeline/pkg/internal/resolution"
	"github.com/tektoncd/pipeline/pkg/trustedresources"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GetPipeline is a function used to retrieve Pipelines.
type GetPipeline func(context.Context, string) (*v1beta1.Pipeline, *v1beta1.RefSource, error)

// GetPipelineData will retrieve the Pipeline metadata and Spec associated with the
// provided PipelineRun. This can come from a reference Pipeline or from the PipelineRun's
// metadata and embedded PipelineSpec.
func GetPipelineData(ctx context.Context, pipelineRun *v1beta1.PipelineRun, getPipeline GetPipeline) (*resolutionutil.ResolvedObjectMeta, *v1beta1.PipelineSpec, error) {
	pipelineMeta := metav1.ObjectMeta{}
	var refSource *v1beta1.RefSource
	pipelineSpec := v1beta1.PipelineSpec{}
	switch {
	case pipelineRun.Spec.PipelineRef != nil && pipelineRun.Spec.PipelineRef.Name != "":
		// Get related pipeline for pipelinerun
		p, source, err := getPipeline(ctx, pipelineRun.Spec.PipelineRef.Name)
		var verificationErr *trustedresources.VerificationError
		if err != nil {
			if errors.As(err, &verificationErr) {
				if trustedresources.IsVerificationResultError(err) {
					return nil, nil, err
				}
			} else {
				return nil, nil, fmt.Errorf("error when listing pipelines for pipelineRun %s: %w", pipelineRun.Name, err)
			}
		}
		pipelineMeta = p.PipelineMetadata()
		pipelineSpec = p.PipelineSpec()
		refSource = source
		pipelineSpec.SetDefaults(ctx)
		return &resolutionutil.ResolvedObjectMeta{
			ObjectMeta: &pipelineMeta,
			RefSource:  refSource,
		}, &pipelineSpec, err
	case pipelineRun.Spec.PipelineSpec != nil:
		pipelineMeta = pipelineRun.ObjectMeta
		pipelineSpec = *pipelineRun.Spec.PipelineSpec
		// TODO: if we want to set RefSource for embedded pipeline, set it here.
		// https://github.com/tektoncd/pipeline/issues/5522
	case pipelineRun.Spec.PipelineRef != nil && pipelineRun.Spec.PipelineRef.Resolver != "":
		pipeline, source, err := getPipeline(ctx, "")
		switch {
		case err != nil:
			return nil, nil, err
		case pipeline == nil:
			return nil, nil, errors.New("resolution of remote resource completed successfully but no pipeline was returned")
		default:
			pipelineMeta = pipeline.PipelineMetadata()
			pipelineSpec = pipeline.PipelineSpec()
		}
		refSource = source
	default:
		return nil, nil, fmt.Errorf("pipelineRun %s not providing PipelineRef or PipelineSpec", pipelineRun.Name)
	}

	pipelineSpec.SetDefaults(ctx)
	return &resolutionutil.ResolvedObjectMeta{
		ObjectMeta: &pipelineMeta,
		RefSource:  refSource,
	}, &pipelineSpec, nil
}
