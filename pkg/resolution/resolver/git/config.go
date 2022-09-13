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

package git

const (
	// defaultTimeoutKey is the configuration field name for controlling
	// the maximum duration of a resolution request for a file from git.
	defaultTimeoutKey = "fetch-timeout"

	// defaultURLKey is the configuration field name for controlling
	// the git url to fetch the remote resource from.
	defaultURLKey = "default-url"

	// defaultRevisionKey is the configuration field name for controlling
	// the revision to fetch the remote resource from.
	defaultRevisionKey = "default-revision"

	// defaultOrgKey is the configuration field name for setting a default organization when using the SCM API.
	defaultOrgKey = "default-org"

	// ServerURLKey is the config map key for the SCM provider URL
	ServerURLKey = "server-url"
	// SCMTypeKey is the config map key for the SCM provider type
	SCMTypeKey = "scm-type"
	// APISecretNameKey is the config map key for the token secret's name
	APISecretNameKey = "api-token-secret-name"
	// APISecretKeyKey is the config map key for the containing the token within the token secret
	APISecretKeyKey = "api-token-secret-key"
	// APISecretNamespaceKey is the config map key for the token secret's namespace
	APISecretNamespaceKey = "api-token-secret-namespace"
)
