package resolution

const (
	// ReasonTaskRunResolutionFailed indicates that references within the
	// TaskRun could not be resolved.
	ReasonTaskRunResolutionFailed = "TaskRunResolutionFailed"

	// ReasonCouldntGetTask indicates that a reference to a task did not
	// successfully resolve to a task object. This is distinct from
	// ReasonTaskRunResolutionFailed because it indicates a failure
	// fetching the referenced Task rather than failure to interpret the
	// reference.
	ReasonCouldntGetTask = "CouldntGetTask"

	// ReasonPipelineRunResolutionFailed indicates that references within the
	// PipelineRun could not be resolved.
	ReasonPipelineRunResolutionFailed = "PipelineRunResolutionFailed"

	// ReasonCouldntGetPipeline indicates that a reference to a pipeline did
	// not successfully resolve to a pipeline object.
	ReasonCouldntGetPipeline = "CouldntGetPipeline"
)
