package resources

import v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"

// ResolvedPipeline contains the data that is needed to execute
// a child (PinP) PipelineRun.
type ResolvedPipeline struct {
	PipelineName string
	Kind         string
	PipelineSpec *v1.PipelineSpec
}

// GetPipelineRun is a function used to retrieve child (PinP) PipelineRuns
type GetPipelineRun func(name string) (*v1.PipelineRun, error)
