package v1beta1

type Artifact struct {
	Name   string          `json:"name,omitempty"`
	Values []ArtifactValue `json:"values,omitempty"`
}

type ArtifactValue struct {
	Digest map[string]string `json:"digest,omitempty"`
	Uri    string            `json:"uri,omitempty"`
}

type TaskRunStepArtifact = Artifact

type StepArtifact struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type Artifacts struct {
	Inputs  []Artifact `json:"inputs"`
	Outputs []Artifact `json:"outputs"`
}
