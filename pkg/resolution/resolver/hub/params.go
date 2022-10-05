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

package hub

// DefaultArtifactHubURL is the default url for the Artifact hub api
const DefaultArtifactHubURL = "https://artifacthub.io/api/v1/packages/tekton-%s/%s/%s/%s"

// TektonHubYamlEndpoint is the suffix for a private custom Tekton hub instance
const TektonHubYamlEndpoint = "v1/resource/%s/%s/%s/%s/yaml"

// ArtifactHubYamlEndpoint is the suffix for a private custom Artifact hub instance
const ArtifactHubYamlEndpoint = "api/v1/packages/tekton-%s/%s/%s/%s"

// ParamName is the parameter defining what the layer name in the bundle
// image is.
const ParamName = "name"

// ParamKind is the parameter defining what the layer kind in the bundle
// image is.
const ParamKind = "kind"

// ParamVersion is the parameter defining what the layer version in the bundle
// image is.
const ParamVersion = "version"

// ParamCatalog is the parameter defining what the catalog in the bundle
// image is.
const ParamCatalog = "catalog"

// ParamType is the parameter defining what the hub type to pull the resource from.
const ParamType = "type"
