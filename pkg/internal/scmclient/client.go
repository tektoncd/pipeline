/*
Copyright 2026 The Tekton Authors

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

package scmclient

import (
	"context"
	"fmt"
)

// SCMClient is an interface for fetching resources from a source control provider
type SCMClient interface {
	GetFileContent(ctx context.Context, org, repo, path, ref string) ([]byte, error)
	GetCommitSHA(ctx context.Context, org, repo, ref string) (string, error)
	GetCloneURL(ctx context.Context, org, repo string) (string, error)
}

// New returns an SCMClient for the given scmType, serverURL, and token
func New(scmType, serverURL, token string) (SCMClient, error) {
	switch scmType {
	case "github":
		return newGitHubClient(serverURL, token), nil
	case "gitlab":
		return newGitLabClient(serverURL, token), nil
	case "gitea":
		return newGiteaClient(serverURL, token), nil
	case "bitbucketcloud":
		return newBitbucketCloudClient(serverURL, token), nil
	case "bitbucketserver":
		return newBitbucketServerClient(serverURL, token), nil
	case "azure":
		return newAzureClient(serverURL, token), nil
	default:
		return nil, fmt.Errorf("unsupported scm type %q", scmType)
	}
}
