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

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/go-git/go-billy/v5/memfs"
	git "github.com/go-git/go-git/v5"
	gitcfg "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/storage/memory"
	resolverconfig "github.com/tektoncd/pipeline/pkg/apis/config/resolver"
	resolutioncommon "github.com/tektoncd/pipeline/pkg/resolution/common"
	"github.com/tektoncd/pipeline/pkg/resolution/resolver/framework"
)

const disabledError = "cannot handle resolution request, enable-git-resolver feature flag not true"

// LabelValueGitResolverType is the value to use for the
// resolution.tekton.dev/type label on resource requests
const LabelValueGitResolverType string = "git"

// GitResolverName is the name that the git resolver should be
// associated with
const GitResolverName string = "Git"

// YAMLContentType is the content type to use when returning yaml
const YAMLContentType string = "application/x-yaml"

var _ framework.Resolver = &Resolver{}

// Resolver implements a framework.Resolver that can fetch files from git.
type Resolver struct{}

// Initialize performs any setup required by the gitresolver.
func (r *Resolver) Initialize(ctx context.Context) error {
	return nil
}

// GetName returns the string name that the gitresolver should be
// associated with.
func (r *Resolver) GetName(_ context.Context) string {
	return GitResolverName
}

// GetSelector returns the labels that resource requests are required to have for
// the gitresolver to process them.
func (r *Resolver) GetSelector(_ context.Context) map[string]string {
	return map[string]string{
		resolutioncommon.LabelKeyResolverType: LabelValueGitResolverType,
	}
}

// ValidateParams returns an error if the given parameter map is not
// valid for a resource request targeting the gitresolver.
func (r *Resolver) ValidateParams(ctx context.Context, params map[string]string) error {
	if r.isDisabled(ctx) {
		return errors.New(disabledError)
	}

	required := []string{
		PathParam,
	}
	missing := []string{}
	if params == nil {
		missing = required
	} else {
		for _, p := range required {
			v, has := params[p]
			if !has || v == "" {
				missing = append(missing, p)
			}
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing %v", strings.Join(missing, ", "))
	}

	// TODO(sbwsg): validate repo url is well-formed, git:// or https://
	// TODO(sbwsg): validate pathInRepo is valid relative pathInRepo

	return nil
}

// Resolve performs the work of fetching a file from git given a map of
// parameters.
func (r *Resolver) Resolve(ctx context.Context, params map[string]string) (framework.ResolvedResource, error) {
	if r.isDisabled(ctx) {
		return nil, errors.New(disabledError)
	}

	conf := framework.GetResolverConfigFromContext(ctx)
	repo := params[URLParam]
	if repo == "" {
		if urlString, ok := conf[ConfigURL]; ok {
			repo = urlString
		} else {
			return nil, fmt.Errorf("default Git Repo Url was not set during installation of the git resolver")
		}
	}
	revision := params[RevisionParam]
	if revision == "" {
		if revisionString, ok := conf[ConfigRevision]; ok {
			revision = revisionString
		} else {
			return nil, fmt.Errorf("default Git Revision was not set during installation of the git resolver")
		}
	}

	cloneOpts := &git.CloneOptions{
		URL: repo,
	}
	filesystem := memfs.New()
	repository, err := git.Clone(memory.NewStorage(), filesystem, cloneOpts)
	if err != nil {
		return nil, fmt.Errorf("clone error: %w", err)
	}

	// try fetch the branch when the given revision refers to a branch name
	refSpec := gitcfg.RefSpec(fmt.Sprintf("+refs/heads/%s:refs/remotes/%s", revision, revision))
	err = repository.Fetch(&git.FetchOptions{
		RefSpecs: []gitcfg.RefSpec{refSpec},
	})
	if err != nil {
		var fetchErr git.NoMatchingRefSpecError
		if !errors.As(err, &fetchErr) {
			return nil, fmt.Errorf("unexpected fetch error: %v", err)
		}
	}

	w, err := repository.Worktree()
	if err != nil {
		return nil, fmt.Errorf("worktree error: %v", err)
	}

	h, err := repository.ResolveRevision(plumbing.Revision(revision))
	if err != nil {
		return nil, fmt.Errorf("revision error: %v", err)
	}

	err = w.Checkout(&git.CheckoutOptions{
		Hash: *h,
	})
	if err != nil {
		return nil, fmt.Errorf("checkout error: %v", err)
	}

	path := params[PathParam]

	f, err := filesystem.Open(path)
	if err != nil {
		return nil, fmt.Errorf("error opening file %q: %v", path, err)
	}

	buf := &bytes.Buffer{}
	_, err = io.Copy(buf, f)
	if err != nil {
		return nil, fmt.Errorf("error reading file %q: %v", path, err)
	}

	return &ResolvedGitResource{
		Revision: revision,
		Content:  buf.Bytes(),
	}, nil
}

var _ framework.ConfigWatcher = &Resolver{}

// GetConfigName returns the name of the git resolver's configmap.
func (r *Resolver) GetConfigName(context.Context) string {
	return "git-resolver-config"
}

var _ framework.TimedResolution = &Resolver{}

// GetResolutionTimeout returns a time.Duration for the amount of time a
// single git fetch may take. This can be configured with the
// fetch-timeout field in the git-resolver-config configmap.
func (r *Resolver) GetResolutionTimeout(ctx context.Context, defaultTimeout time.Duration) time.Duration {
	conf := framework.GetResolverConfigFromContext(ctx)
	if timeoutString, ok := conf[ConfigFieldTimeout]; ok {
		timeout, err := time.ParseDuration(timeoutString)
		if err == nil {
			return timeout
		}
	}
	return defaultTimeout
}

func (r *Resolver) isDisabled(ctx context.Context) bool {
	cfg := resolverconfig.FromContextOrDefaults(ctx)
	if cfg.FeatureFlags.EnableGitResolver {
		return false
	}

	return true
}

// ResolvedGitResource implements framework.ResolvedResource and returns
// the resolved file []byte data and an annotation map for any metadata.
type ResolvedGitResource struct {
	Revision string
	Content  []byte
}

var _ framework.ResolvedResource = &ResolvedGitResource{}

// Data returns the bytes of the file resolved from git.
func (r *ResolvedGitResource) Data() []byte {
	return r.Content
}

// Annotations returns the metadata that accompanies the file fetched
// from git.
func (r *ResolvedGitResource) Annotations() map[string]string {
	return map[string]string{
		AnnotationKeyRevision:                     r.Revision,
		resolutioncommon.AnnotationKeyContentType: YAMLContentType,
	}
}
