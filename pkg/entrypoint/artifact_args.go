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

package entrypoint

import (
	"encoding/json"
)

// ParseArtifactInputs parses a JSON-encoded list of ArtifactInput.
func ParseArtifactInputs(s string) ([]ArtifactInput, error) {
	if s == "" {
		return nil, nil
	}
	var inputs []ArtifactInput
	if err := json.Unmarshal([]byte(s), &inputs); err != nil {
		return nil, err
	}
	return inputs, nil
}

// ParseArtifactOutputs parses a JSON-encoded list of ArtifactOutput.
func ParseArtifactOutputs(s string) ([]ArtifactOutput, error) {
	if s == "" {
		return nil, nil
	}
	var outputs []ArtifactOutput
	if err := json.Unmarshal([]byte(s), &outputs); err != nil {
		return nil, err
	}
	return outputs, nil
}
