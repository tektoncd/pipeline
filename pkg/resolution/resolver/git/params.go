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

import "github.com/tektoncd/pipeline/pkg/resolution/resource"

const (
	// UrlParam is the git repo Url when using the anonymous/full clone approach
	UrlParam string = resource.ParamURL
	// OrgParam is the organization to find the repository in when using the SCM API approach
	OrgParam = "org"
	// RepoParam is the repository to use when using the SCM API approach
	RepoParam = "repo"
	// PathParam is the pathInRepo into the git repo where a file is located. This is used with both approaches.
	PathParam string = "pathInRepo"
	// RevisionParam is the git revision that a file should be fetched from. This is used with both approaches.
	RevisionParam string = "revision"
	// TokenParam is an optional reference to a secret name for SCM API authentication
	TokenParam string = "token"
	// TokenKeyParam is an optional reference to a key in the TokenParam secret for SCM API authentication
	TokenKeyParam string = "tokenKey"
	// GitTokenParam is an optional reference to a secret name when using go-git for git authentication
	GitTokenParam string = "gitToken"
	// GitTokenParam is an optional reference to a secret name when using go-git for git authentication
	GitTokenKeyParam string = "gitTokenKey"
	// DefaultTokenKeyParam is the default key in the TokenParam secret for SCM API authentication
	DefaultTokenKeyParam string = "token"
	// scmTypeParam is an optional string overriding the scm-type configuration (ie: github, gitea, gitlab etc..)
	ScmTypeParam string = "scmType"
	// serverURLParam is an optional string to the server URL for the SCM API to connect to
	ServerURLParam string = "serverURL"
	// ConfigKeyParam is an optional string to provid which scm configuration to use from git resolver configmap
	ConfigKeyParam string = "configKey"
)
