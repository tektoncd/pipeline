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

// Artifact represents an artifact within a system, potentially containing multiple values
// associated with it.
type Artifact struct {
	// The artifact's identifying category name
	Name string `json:"name,omitempty"`
	// A collection of values related to the artifact
	Values []ArtifactValue `json:"values,omitempty"`
	// Indicate if the artifact is a build output or a by-product
	BuildOutput bool `json:"buildOutput,omitempty"`
}

// ArtifactValue represents a specific value or data element within an Artifact.
type ArtifactValue struct {
	Digest map[Algorithm]string `json:"digest,omitempty"` // Algorithm-specific digests for verifying the content (e.g., SHA256)
	Uri    string               `json:"uri,omitempty"`    // Location where the artifact value can be retrieved
}

// TaskRunStepArtifact represents an artifact produced or used by a step within a task run.
// It directly uses the Artifact type for its structure.
type TaskRunStepArtifact = Artifact

// Artifacts represents the collection of input and output artifacts associated with
// a task run or a similar process. Artifacts in this context are units of data or resources
// that the process either consumes as input or produces as output.
type Artifacts struct {
	Inputs  []Artifact `json:"inputs,omitempty"`
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
				outputMap[v.Name] = Artifact{Name: v.Name, Values: []ArtifactValue{}, BuildOutput: v.BuildOutput}
			}
			// only update buildOutput to true.
			// Do not convert to false if it was true before.
			if v.BuildOutput {
				art := outputMap[v.Name]
				art.BuildOutput = v.BuildOutput
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
		})
	}
	a.Inputs = newInputs
	a.Outputs = newOutputs
}
