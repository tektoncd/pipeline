/*
Copyright 2018 Knative Authors LLC
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

package logkey

const (
	// ControllerType is the key used for controller type in structured logs
	ControllerType = "build.knative.dev/controller"

	// Namespace is the key used for namespace in structured logs
	Namespace = "build.knative.dev/namespace"

	// Build is the key used for build name in structured logs
	Build = "build.knative.dev/build"

	// BuildTemplate is the key used for build name in structured logs
	BuildTemplate = "build.knative.dev/buildtemplate"

	// JSONConfig is the key used for JSON configurations (not to be confused by the Configuration object)
	JSONConfig = "build.knative.dev/jsonconfig"

	// Kind is the key used to represent kind of an object in logs
	Kind = "build.knative.dev/kind"

	// Name is the key used to represent name of an object in logs
	Name = "build.knative.dev/name"

	// Operation is the key used to represent an operation in logs
	Operation = "build.knative.dev/operation"

	// Resource is the key used to represent a resource in logs
	Resource = "build.knative.dev/resource"

	// SubResource is a generic key used to represent a sub-resource in logs
	SubResource = "build.knative.dev/subresource"

	// UserInfo is the key used to represent a user information in logs
	UserInfo = "build.knative.dev/userinfo"

	// Pod is the key used to represent a pod's name in logs
	Pod = "build.knative.dev/pod"
)
