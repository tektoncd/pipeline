/*
Copyright 2024 The Tekton Authors

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

package v1

import (
	"github.com/google/go-cmp/cmp"
)

// Algorithm Standard cryptographic hash algorithm
type Algorithm string

// ArtifactDeclarations declares what artifacts a task consumes and produces.
type ArtifactDeclarations struct {
	// Inputs declares artifacts that the task expects to consume.
	// +listType=atomic
	Inputs []ArtifactDeclaration `json:"inputs,omitempty"`
	// Outputs declares artifacts that the task produces.
	// +listType=atomic
	Outputs []ArtifactDeclaration `json:"outputs,omitempty"`
}

// ArtifactType distinguishes how Tekton handles the artifact.
//   - "reference": the step already stored the content elsewhere (e.g. pushed
//     an image); Tekton only records the URI and digest.
//   - "content": the step writes data to a path; Tekton uploads, downloads and
//     verifies it transparently across Tasks.
type ArtifactType string

const (
	// ArtifactTypeContent means Tekton manages storage and transport of the artifact.
	ArtifactTypeContent ArtifactType = "content"
	// ArtifactTypeReference means the step handles storage; Tekton only records metadata.
	ArtifactTypeReference ArtifactType = "reference"
)

// ArtifactDeclaration describes a single artifact input or output.
type ArtifactDeclaration struct {
	// Name of the artifact (used in path substitution and pipeline bindings)
	Name string `json:"name"`
	// Description of the artifact.
	// +optional
	Description string `json:"description,omitempty"`
	// Type of the artifact: "reference" or "content". Defaults to "content".
	// +optional
	Type ArtifactType `json:"type,omitempty"`
	// MediaType hint for the artifact (e.g., application/vnd.tekton.artifact.junit.v1+xml)
	// +optional
	MediaType string `json:"mediaType,omitempty"`
	// Subject marks this artifact as the primary build output — the SLSA
	// attestation subject. Multiple artifacts can be subjects.
	// +optional
	Subject bool `json:"subject,omitempty"`
	// Value surfaces an artifact produced by a Step, using
	// $(steps.<step>.artifacts.<name>). Mirrors TaskResult.Value. When set,
	// Type and MediaType are inherited from the referenced step artifact.
	// +optional
	Value string `json:"value,omitempty"`
}

// StepArtifacts declares the artifact outputs a Step or StepAction produces.
// Only outputs are declared at step scope — inputs are fetched once at Task
// scope and passed to steps as params.
type StepArtifacts struct {
	// Outputs declares artifacts produced by this Step or StepAction.
	// +listType=atomic
	Outputs []ArtifactDeclaration `json:"outputs,omitempty"`
}

// PipelineArtifactDeclarations declares artifacts at the Pipeline boundary.
type PipelineArtifactDeclarations struct {
	// +listType=atomic
	Inputs []ArtifactDeclaration `json:"inputs,omitempty"`
	// +listType=atomic
	Outputs []PipelineArtifactOutput `json:"outputs,omitempty"`
}

// PipelineArtifactOutput exposes a Task artifact as a Pipeline output.
type PipelineArtifactOutput struct {
	Name string `json:"name"`
	From string `json:"from"`
}

// PipelineRunArtifacts supplies artifacts to a PipelineRun.
type PipelineRunArtifacts struct {
	// +listType=atomic
	Inputs []Artifact `json:"inputs,omitempty"`
}

type PipelineTaskArtifacts struct {
	// Inputs binds artifact inputs from other tasks' outputs.
	// +listType=atomic
	Inputs []PipelineTaskArtifactBinding `json:"inputs,omitempty"`
}

// PipelineTaskArtifactBinding binds an artifact input to an output from another task.
type PipelineTaskArtifactBinding struct {
	// Name matches the artifact input name in the referenced Task.
	Name string `json:"name"`
	// From references an output from another task: "tasks.<taskName>.outputs.<artifactName>"
	From string `json:"from"`
}

// Artifact represents an artifact within a system, potentially containing multiple values
// associated with it.
type Artifact struct {
	// The artifact's identifying category name
	Name string `json:"name,omitempty"`
	// A collection of values related to the artifact
	Values []ArtifactValue `json:"values,omitempty"`
	// Indicate if the artifact is a build output or a by-product.
	// This is the legacy form of Subject, still populated from the legacy
	// provenance format.
	BuildOutput bool `json:"buildOutput,omitempty"`
	// Subject marks this artifact as the primary build output (the SLSA
	// attestation subject), as declared in the Task's artifact outputs.
	Subject bool `json:"subject,omitempty"`
}

// ArtifactValue represents a specific value or data element within an Artifact.
type ArtifactValue struct {
	Digest map[Algorithm]string `json:"digest,omitempty"` // Algorithm-specific digests for verifying the content (e.g., SHA256)
	Uri    string               `json:"uri,omitempty"`    // Location where the artifact value can be retrieved
	// Ref is a backend-specific storage reference. Present only for content
	// artifacts stored externally; absent for reference artifacts and inline content.
	// +optional
	Ref *ArtifactStorageRef `json:"ref,omitempty"`
	// content size in bytes, letting consumers and UIs know the artifact size before fetching.
	// +optional
	Size int64 `json:"size,omitempty"`
	// Inline holds small content embedded directly in the status (base64-encoded),
	// avoiding external storage overhead.
	// +optional
	Inline string `json:"inline,omitempty"`
}

// ArtifactStorageRef identifies where a content artifact is stored in a backend.
type ArtifactStorageRef struct {
	Backend     string `json:"backend"`
	Location    string `json:"location"`
	Digest      string `json:"digest"`
	ContentType string `json:"contentType,omitempty"`
}

// TaskRunStepArtifact represents an artifact produced or used by a step within a task run.
// It directly uses the Artifact type for its structure.
type TaskRunStepArtifact = Artifact

// Artifacts represents the collection of input and output artifacts associated with
// a task run or a similar process. Artifacts in this context are units of data or resources
// that the process either consumes as input or produces as output.
type Artifacts struct {
	// +listType=atomic
	Inputs []Artifact `json:"inputs,omitempty"`
	// +listType=atomic
	Outputs []Artifact `json:"outputs,omitempty"`
}

func (a *Artifacts) Merge(another *Artifacts) {
	inputMap := make(map[string][]ArtifactValue)
	var newInputs []Artifact

	for _, v := range a.Inputs {
		inputMap[v.Name] = v.Values
	}
	if another != nil {
		for _, v := range another.Inputs {
			_, ok := inputMap[v.Name]
			if !ok {
				inputMap[v.Name] = []ArtifactValue{}
			}
			for _, vv := range v.Values {
				exists := false
				for _, av := range inputMap[v.Name] {
					if cmp.Equal(vv, av) {
						exists = true
						break
					}
				}
				if !exists {
					inputMap[v.Name] = append(inputMap[v.Name], vv)
				}
			}
		}
	}

	for k, v := range inputMap {
		newInputs = append(newInputs, Artifact{
			Name:   k,
			Values: v,
		})
	}

	outputMap := make(map[string]Artifact)
	var newOutputs []Artifact
	for _, v := range a.Outputs {
		outputMap[v.Name] = v
	}

	if another != nil {
		for _, v := range another.Outputs {
			_, ok := outputMap[v.Name]
			if !ok {
				outputMap[v.Name] = Artifact{Name: v.Name, Values: []ArtifactValue{}, BuildOutput: v.BuildOutput, Subject: v.Subject}
			}
			// only update buildOutput to true.
			// Do not convert to false if it was true before.
			if v.BuildOutput {
				art := outputMap[v.Name]
				art.BuildOutput = v.BuildOutput
				outputMap[v.Name] = art
			}
			if v.Subject {
				art := outputMap[v.Name]
				art.Subject = true
				outputMap[v.Name] = art
			}
			for _, vv := range v.Values {
				exists := false
				for _, av := range outputMap[v.Name].Values {
					if cmp.Equal(vv, av) {
						exists = true
						break
					}
				}
				if !exists {
					art := outputMap[v.Name]
					art.Values = append(art.Values, vv)
					outputMap[v.Name] = art
				}
			}
		}
	}

	for _, v := range outputMap {
		newOutputs = append(newOutputs, Artifact{
			Name:        v.Name,
			Values:      v.Values,
			BuildOutput: v.BuildOutput,
			Subject:     v.Subject,
		})
	}
	a.Inputs = newInputs
	a.Outputs = newOutputs
}
