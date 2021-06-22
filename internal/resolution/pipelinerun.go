/*
Copyright 2021 The Tekton Authors

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

package resolution

import (
	"context"
	"fmt"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	clientset "github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	"github.com/tektoncd/pipeline/pkg/reconciler/pipelinerun/resources"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type PipelineRunResolutionRequest struct {
	KubeClientSet     kubernetes.Interface
	PipelineClientSet clientset.Interface
	PipelineRun       *v1beta1.PipelineRun

	ResolvedPipelineMeta *metav1.ObjectMeta
	ResolvedPipelineSpec *v1beta1.PipelineSpec
}

// PipelineRunResolutionRequest.Resolve implements the default resolution behaviour
// for a Tekton Pipelines pipelinerun. It resolves the pipeline associated with the pipelinerun
// from one of three places:
//
// - in-line in the pipelinerun's spec.pipelineSpec
// - from the cluster via the pipelinerun's spec.pipelineRef
// - (when relevant feature flag enabled) from a Tekton Bundle indicated by the pipelinerun's spec.pipelineRef.bundle field.
//
// If a pipeline is resolved correctly from the pipelinerun then the ResolvedPipelineMeta
// and ResolvedPipelineSpec fields of the PipelineRunResolutionRequest will be
// populated after this method returns.
//
// If an error occurs during any part in the resolution process it will be
// returned with both a human-readable Message and a machine-readable Reason
// embedded in a *resolution.Error.
func (req *PipelineRunResolutionRequest) Resolve(ctx context.Context) error {
	if req.PipelineRun.Status.PipelineSpec != nil {
		return ErrorPipelineRunAlreadyResolved
	}

	pipelinerunKey := fmt.Sprintf("%s/%s", req.PipelineRun.Namespace, req.PipelineRun.Name)

	getPipelineFunc, err := resources.GetPipelineFunc(ctx, req.KubeClientSet, req.PipelineClientSet, req.PipelineRun)
	if err != nil {
		return NewError(ReasonCouldntGetPipeline, fmt.Errorf("Error retrieving pipeline for pipelinerun %q: %w", pipelinerunKey, err))
	}

	pipelineMeta, pipelineSpec, err := resources.GetPipelineData(ctx, req.PipelineRun, getPipelineFunc)
	if err != nil {
		return NewError(ReasonCouldntGetPipeline, fmt.Errorf("Error retrieving pipeline for pipelinerun %q: %w", pipelinerunKey, err))
	}

	req.ResolvedPipelineMeta = pipelineMeta
	req.ResolvedPipelineSpec = pipelineSpec

	return nil
}
