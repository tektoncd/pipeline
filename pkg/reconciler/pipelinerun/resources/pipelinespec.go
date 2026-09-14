package resources

import (
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/trustedresources"
)

// ResolvedPipeline contains the data that is needed to execute
// a child PipelineRun.
type ResolvedPipeline struct {
	PipelineName string
	Kind         string
	PipelineSpec *v1.PipelineSpec
	// VerificationResult is the result from trusted resources verification
	// for the resolved child Pipeline. Nil for inline PipelineSpec children
	// or when the trusted-resources feature is disabled.
	VerificationResult *trustedresources.VerificationResult
}

// GetPipelineRun is a function used to retrieve child PipelineRuns
type GetPipelineRun func(name string) (*v1.PipelineRun, error)
