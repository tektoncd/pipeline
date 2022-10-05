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

// ConfigTektonHubCatalog is the configuration field name for controlling
// the Tekton Hub catalog to fetch the remote resource from.
const ConfigTektonHubCatalog = "default-tekton-hub-catalog"

// ConfigArtifactHubTaskCatalog is the configuration field name for controlling
// the Artifact Hub Task catalog to fetch the remote resource from.
const ConfigArtifactHubTaskCatalog = "default-artifact-hub-task-catalog"

// ConfigArtifactHubPipelineCatalog is the configuration field name for controlling
// the Artifact Hub Pipeline catalog to fetch the remote resource from.
const ConfigArtifactHubPipelineCatalog = "default-artifact-hub-pipeline-catalog"

// ConfigKind is the configuration field name for controlling
// what the layer name in the hub image is.
const ConfigKind = "default-kind"

// ConfigType is the configuration field name for controlling
// the hub type to pull the resource from.
const ConfigType = "default-type"
