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

package pipeline

import (
	"context"
	"errors"
	"fmt"

	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GetPipeline is a function used to retrieve Pipelines.
type GetPipeline func(context.Context, string) (v1beta1.PipelineObject, error)

// GetPipelineData will retrieve the Pipeline metadata and Spec associated with the
// provided PipelineRun. This can come from a reference Pipeline or from the PipelineRun's
// metadata and embedded PipelineSpec.
func GetPipelineData(ctx context.Context, pipelineRun *v1beta1.PipelineRun, getPipeline GetPipeline) (*metav1.ObjectMeta, *v1beta1.PipelineSpec, error) {
	pipelineMeta := metav1.ObjectMeta{}
	pipelineSpec := v1beta1.PipelineSpec{}
	cfg := config.FromContextOrDefaults(ctx)
	switch {
	case pipelineRun.Spec.PipelineRef != nil && pipelineRun.Spec.PipelineRef.Name != "":
		// Get related pipeline for pipelinerun
		t, err := getPipeline(ctx, pipelineRun.Spec.PipelineRef.Name)
		if err != nil {
			return nil, nil, fmt.Errorf("error when listing pipelines for pipelineRun %s: %w", pipelineRun.Name, err)
		}
		pipelineMeta = t.PipelineMetadata()
		pipelineSpec = t.PipelineSpec()
	case pipelineRun.Spec.PipelineSpec != nil:
		pipelineMeta = pipelineRun.ObjectMeta
		pipelineSpec = *pipelineRun.Spec.PipelineSpec
	case cfg.FeatureFlags.EnableAPIFields == config.AlphaAPIFields && pipelineRun.Spec.PipelineRef != nil && pipelineRun.Spec.PipelineRef.Resolver != "":
		pipeline, err := getPipeline(ctx, "")
		switch {
		case err != nil:
			return nil, nil, err
		case pipeline == nil:
			return nil, nil, errors.New("resolution of remote resource completed successfully but no pipeline was returned")
		default:
			pipelineMeta = pipeline.PipelineMetadata()
			pipelineSpec = pipeline.PipelineSpec()
		}
	default:
		return nil, nil, fmt.Errorf("pipelineRun %s not providing PipelineRef or PipelineSpec", pipelineRun.Name)
	}
	return &pipelineMeta, &pipelineSpec, nil
}
