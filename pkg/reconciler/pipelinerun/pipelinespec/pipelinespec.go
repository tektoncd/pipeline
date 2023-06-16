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

	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	resolutionutil "github.com/tektoncd/pipeline/pkg/internal/resolution"
	"github.com/tektoncd/pipeline/pkg/trustedresources"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GetPipeline is a function used to retrieve Pipelines.
// VerificationResult is the result from trusted resources if the feature is enabled.
type GetPipeline func(context.Context, string) (*v1.Pipeline, *v1.RefSource, *trustedresources.VerificationResult, error)

// GetPipelineData will retrieve the Pipeline metadata and Spec associated with the
// provided PipelineRun. This can come from a reference Pipeline or from the PipelineRun's
// metadata and embedded PipelineSpec.
func GetPipelineData(ctx context.Context, pipelineRun *v1.PipelineRun, getPipeline GetPipeline) (*resolutionutil.ResolvedObjectMeta, *v1.PipelineSpec, error) {
	pipelineMeta := metav1.ObjectMeta{}
	var refSource *v1.RefSource
	var verificationResult *trustedresources.VerificationResult
	pipelineSpec := v1.PipelineSpec{}
	switch {
	case pipelineRun.Spec.PipelineRef != nil && pipelineRun.Spec.PipelineRef.Name != "":
		// Get related pipeline for pipelinerun
		p, source, vr, err := getPipeline(ctx, pipelineRun.Spec.PipelineRef.Name)
		if err != nil {
			return nil, nil, fmt.Errorf("error when listing pipelines for pipelineRun %s: %w", pipelineRun.Name, err)
		}
		pipelineMeta = p.PipelineMetadata()
		pipelineSpec = p.PipelineSpec()
		refSource = source
		verificationResult = vr
	case pipelineRun.Spec.PipelineSpec != nil:
		pipelineMeta = pipelineRun.ObjectMeta
		pipelineSpec = *pipelineRun.Spec.PipelineSpec
		// TODO: if we want to set RefSource for embedded pipeline, set it here.
		// https://github.com/tektoncd/pipeline/issues/5522
	case pipelineRun.Spec.PipelineRef != nil && pipelineRun.Spec.PipelineRef.Resolver != "":
		pipeline, source, vr, err := getPipeline(ctx, "")
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
		verificationResult = vr
	default:
		return nil, nil, fmt.Errorf("pipelineRun %s not providing PipelineRef or PipelineSpec", pipelineRun.Name)
	}

	pipelineSpec.SetDefaults(ctx)
	return &resolutionutil.ResolvedObjectMeta{
		ObjectMeta:         &pipelineMeta,
		RefSource:          refSource,
		VerificationResult: verificationResult,
	}, &pipelineSpec, nil
}
