package v1beta1

type Artifact struct {
	Name   string          `json:"name,omitempty"`
	Values []ArtifactValue `json:"values"`
}

type ArtifactValue struct {
	Digest string `json:"digest,omitempty"`
	Uri    string `json:"uri,omitempty"`
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
