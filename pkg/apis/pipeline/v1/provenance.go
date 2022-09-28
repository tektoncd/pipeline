/*
Copyright 2022 The Tekton Authors
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

// Provenance contains some key authenticated metadata about how a software artifact was
// built (what sources, what inputs/outputs, etc.). For now, it only contains the subfield
// `ConfigSource` that identifies the source where a build config file came from.
// In future, it can be expanded as needed to include more metadata about the build.
// This field aims to be used to carry minimum amount of the authenticated metadata in *Run status
// so that Tekton Chains can pick it up and record in the provenance it generates.
type Provenance struct {
	// ConfigSource identifies the source where a resource came from.
	ConfigSource *ConfigSource `json:"configSource,omitempty"`
}

// ConfigSource identifies the source where a resource came from.
// This can include Git repositories, Task Bundles, file checksums, or other information
// that allows users to identify where the resource came from and what version was used.
type ConfigSource struct {
	// URI indicates the identity of the source of the config.
	// Definition: https://slsa.dev/provenance/v0.2#invocation.configSource.uri
	// Example: "https://github.com/tektoncd/catalog"
	URI string `json:"uri,omitempty"`

	// Digest is a collection of cryptographic digests for the contents of the artifact specified by URI.
	// Definition: https://slsa.dev/provenance/v0.2#invocation.configSource.digest
	// Example: {"sha1": "f99d13e554ffcb696dee719fa85b695cb5b0f428"}
	Digest map[string]string `json:"digest,omitempty"`

	// EntryPoint identifies the entry point into the build. This is often a path to a
	// configuration file and/or a target label within that file.
	// Definition: https://slsa.dev/provenance/v0.2#invocation.configSource.entryPoint
	// Example: "task/git-clone/0.8/git-clone.yaml"
	EntryPoint string `json:"entryPoint,omitempty"`
}
