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

package cluster

const (
	// DefaultKindKey is the key in the config map for the default kind setting
	DefaultKindKey = "default-kind"
	// DefaultNamespaceKey is the key in the config map for the default namespace setting
	DefaultNamespaceKey = "default-namespace"

	// AllowedNamespacesKey is the key in the config map for an optional comma-separated list of namespaces which the
	// resolver is allowed to access. Defaults to empty, meaning all namespaces are allowed.
	AllowedNamespacesKey = "allowed-namespaces"
	// BlockedNamespacesKey is the key in the config map for an optional comma-separated list of namespaces which the
	// resolver is blocked from accessing. Defaults to empty, meaning no namespaces are blocked.
	BlockedNamespacesKey = "blocked-namespaces"
)
