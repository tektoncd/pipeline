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

// FakeSCMClient is a test helper for SCMClient.
// Set the fields to control what each method returns.
type FakeSCMClient struct {
	FileContent map[string][]byte // key: "org/repo/path/ref"
	CommitSHAs  map[string]string // key: "org/repo/ref"
	CloneURLs   map[string]string // key: "org/repo"
	Err         error             // if set, all methods return this error
}

func (f *FakeSCMClient) GetFileContent(_ context.Context, org, repo, path, ref string) ([]byte, error) {
	if f.Err != nil {
		return nil, f.Err
	}
	key := org + "/" + repo + "/" + path + "/" + ref
	content, ok := f.FileContent[key]
	if !ok {
		return nil, fmt.Errorf("file %s does not exist", path)
	}
	return content, nil
}

func (f *FakeSCMClient) GetCommitSHA(_ context.Context, org, repo, ref string) (string, error) {
	if f.Err != nil {
		return "", f.Err
	}
	return f.CommitSHAs[org+"/"+repo+"/"+ref], nil
}

func (f *FakeSCMClient) GetCloneURL(_ context.Context, org, repo string) (string, error) {
	if f.Err != nil {
		return "", f.Err
	}
	return f.CloneURLs[org+"/"+repo], nil
}
