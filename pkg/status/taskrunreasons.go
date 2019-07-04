package status

const (
	// reasonCouldntGetTask indicates that the reason for the failure status is that the
	// Task couldn't be found
	ReasonCouldntGetTask = "CouldntGetTask"

	// reasonFailedResolution indicated that the reason for failure status is
	// that references within the TaskRun could not be resolved
	ReasonFailedResolution = "TaskRunResolutionFailed"

	// reasonFailedValidation indicated that the reason for failure status is
	// that taskrun failed runtime validation
	ReasonFailedValidation = "TaskRunValidationFailed"

	// reasonRunning indicates that the reason for the inprogress status is that the TaskRun
	// is just starting to be reconciled
	ReasonRunning = "Running"

	// reasonBuilding indicates that the reason for the in-progress status is that the TaskRun
	// is just being built
	ReasonBuilding = "Building"

	// reasonTimedOut indicates that the TaskRun has taken longer than its configured timeout
	ReasonTimedOut = "TaskRunTimeout"

	// reasonExceededResourceQuota indicates that the TaskRun failed to create a pod due to
	// a ResourceQuota in the namespace
	ReasonExceededResourceQuota = "ExceededResourceQuota"

	// reasonExceededNodeResources indicates that the TaskRun's pod has failed to start due
	// to resource constraints on the node
	ReasonExceededNodeResources = "ExceededNodeResources"

	// ReasonSucceeded indicates that the reason for the finished status is that all of the steps
	// completed successfully
	ReasonSucceeded = "Succeeded"

	// ReasonFailed indicates that the reason for the failure status is unknown or that one of the steps failed
	ReasonFailed = "Failed"
)
