package pipeline

const (
	// WorkspaceDir is the root directory used for PipelineResources and (by default) Workspaces
	WorkspaceDir = "/workspace"
	// DefaultResultPath is the path for a task result to create the result file
	DefaultResultPath = "/tekton/results"
	// HomeDir is the HOME directory of PipelineResources
	HomeDir = "/tekton/home"
	// CredsDir is the directory where credentials are placed to meet the legacy credentials
	// helpers image (aka "creds-init") contract
	CredsDir = "/tekton/creds" // #nosec
	// StepsDir is the directory used for a step to store any metadata related to the step
	StepsDir = "/tekton/steps"

	ScriptDir = "/tekton/scripts"

	ArtifactsDir = "/tekton/artifacts"
)
