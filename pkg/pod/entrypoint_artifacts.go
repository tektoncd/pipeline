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

package pod

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strconv"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
)

// artifactEntrypointArgs generates entrypoint CLI args for artifact transport
// based on the TaskSpec's artifact declarations and the artifact storage config.
func artifactEntrypointArgs(taskSpec *v1.TaskSpec, backend string, ociRepository string, insecure bool, inlineThreshold int) []string {
	if taskSpec == nil || taskSpec.Artifacts == nil {
		return nil
	}

	var args []string

	if len(taskSpec.Artifacts.Inputs) > 0 {
		type artifactInput struct {
			Name string `json:"name"`
			URI  string `json:"uri"`
			Path string `json:"path"`
		}
		var inputs []artifactInput
		for _, decl := range taskSpec.Artifacts.Inputs {
			inputs = append(inputs, artifactInput{
				Name: decl.Name,
				// URI will be resolved at runtime from pipeline bindings or params
				Path: filepath.Join(pipeline.ArtifactsDir, "inputs", decl.Name),
			})
		}
		data, _ := json.Marshal(inputs) //nolint:errchkjson // marshaling plain structs cannot fail
		args = append(args, "-artifact_inputs", string(data))
	}

	if len(taskSpec.Artifacts.Outputs) > 0 {
		type artifactOutput struct {
			Name       string          `json:"name"`
			Type       v1.ArtifactType `json:"type"`
			Path       string          `json:"path"`
			Repository string          `json:"repository,omitempty"`
			MediaType  string          `json:"mediaType,omitempty"`
			Subject    bool            `json:"subject,omitempty"`
		}
		var outputs []artifactOutput
		for _, decl := range taskSpec.Artifacts.Outputs {
			artifactType := decl.Type
			if artifactType == "" {
				artifactType = v1.ArtifactTypeContent
			}
			out := artifactOutput{
				Name:    decl.Name,
				Type:    artifactType,
				Subject: decl.Subject,
			}
			if artifactType == v1.ArtifactTypeReference {
				// Reference artifacts: the step writes the URI+digest to a file;
				// Tekton does not transport or store any content.
				out.Path = filepath.Join(pipeline.ArtifactsDir, "outputs", decl.Name+".uri")
			} else {
				// Content artifacts: the step writes data under this directory.
				// When content storage is configured, the entrypoint uploads it;
				// otherwise (TEP-0192 "disabled storage" default) it is still
				// digested and recorded, just not uploaded -- leave Repository
				// unset rather than building a broken "/<name>" path.
				out.Path = filepath.Join(pipeline.ArtifactsDir, "outputs", decl.Name)
				if ociRepository != "" {
					out.Repository = fmt.Sprintf("%s/%s", ociRepository, decl.Name)
				}
				out.MediaType = decl.MediaType
			}
			outputs = append(outputs, out)
		}
		data, _ := json.Marshal(outputs) //nolint:errchkjson // marshaling plain structs cannot fail
		args = append(args, "-artifact_outputs", string(data))
	}

	if backend != "" {
		args = append(args, "-artifact_backend", backend)
	}

	if insecure {
		args = append(args, "-artifact_insecure")
	}

	if inlineThreshold > 0 {
		args = append(args, "-artifact_inline_threshold", strconv.Itoa(inlineThreshold))
	}

	return args
}
