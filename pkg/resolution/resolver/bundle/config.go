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

package bundle

const (
	// ConfigMapName is the bundle resolver's config map
	ConfigMapName = "bundleresolver-config"
	// ConfigServiceAccount is the configuration field name for controlling
	// the Service Account name to use for bundle requests.
	ConfigServiceAccount = "default-service-account"
	// ConfigKind is the configuration field name for controlling
	// what the layer name in the bundle image is.
	ConfigKind = "default-kind"
)
