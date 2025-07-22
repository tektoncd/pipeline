/*
Copyright 2023 The Tekton Authors
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

package http

import "github.com/tektoncd/pipeline/pkg/resolution/resource"

const (
	// UrlParam is the URL to fetch the task from
	UrlParam string = resource.ParamURL

	// HttpBasicAuthUsername is the user name to use for basic auth
	HttpBasicAuthUsername string = "http-username"

	// HttpBasicAuthSecret is the reference to a secret in the PipelineRun or TaskRun namespace to use for basic auth
	HttpBasicAuthSecret string = "http-password-secret"

	// HttpBasicAuthSecretKey is the key in the httpBasicAuthSecret secret to use for basic auth
	HttpBasicAuthSecretKey string = "http-password-secret-key"

	// ParamBasicAuthSecretKey is the parameter defining what key in the secret to use for basic auth
	ParamBasicAuthSecretKey = "secretKey"

	// CacheParam is the parameter defining whether to cache the resolved resource
	CacheParam = "cache"
)
