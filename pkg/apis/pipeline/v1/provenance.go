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

import "github.com/tektoncd/pipeline/pkg/apis/config"

// Provenance contains metadata about resources used in the TaskRun/PipelineRun
// such as the source from where a remote build definition was fetched.
// This field aims to carry minimum amoumt of metadata in *Run status so that
// Tekton Chains can capture them in the provenance.
type Provenance struct {
	// RefSource identifies the source where a remote task/pipeline came from.
	RefSource *RefSource `json:"refSource,omitempty"`

	// FeatureFlags identifies the feature flags that were used during the task/pipeline run
	FeatureFlags *config.FeatureFlags `json:"featureFlags,omitempty"`
}

// RefSource contains the information that can uniquely identify where a remote
// built definition came from i.e. Git repositories, Tekton Bundles in OCI registry
// and hub.
type RefSource struct {
	// URI indicates the identity of the source of the build definition.
	// Example: "https://github.com/tektoncd/catalog"
	URI string `json:"uri,omitempty"`

	// Digest is a collection of cryptographic digests for the contents of the artifact specified by URI.
	// Example: {"sha1": "f99d13e554ffcb696dee719fa85b695cb5b0f428"}
	Digest map[string]string `json:"digest,omitempty"`

	// EntryPoint identifies the entry point into the build. This is often a path to a
	// build definition file and/or a target label within that file.
	// Example: "task/git-clone/0.8/git-clone.yaml"
	EntryPoint string `json:"entryPoint,omitempty"`
}
