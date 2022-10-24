package config

const (
	// TektonHermeticEnvVar is the env var we set in containers to indicate they should be run hermetically
	TektonHermeticEnvVar = "TEKTON_HERMETIC"
	// ReleaseAnnotation is the annotation set on TaskRun pods specifying the version of Pipelines
	ReleaseAnnotation = "pipeline.tekton.dev/release"
)
