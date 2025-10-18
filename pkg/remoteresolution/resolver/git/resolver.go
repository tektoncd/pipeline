/*
Copyright 2024 The Tekton Authors

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
	"context"
	"errors"
	"time"

	"github.com/jenkins-x/go-scm/scm"
	"github.com/jenkins-x/go-scm/scm/factory"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/apis/resolution/v1beta1"
	"github.com/tektoncd/pipeline/pkg/remoteresolution/resolver/framework"
	"github.com/tektoncd/pipeline/pkg/remoteresolution/resolver/framework/cache"
	resolutioncommon "github.com/tektoncd/pipeline/pkg/resolution/common"
	resolutionframework "github.com/tektoncd/pipeline/pkg/resolution/resolver/framework"
	"github.com/tektoncd/pipeline/pkg/resolution/resolver/git"
	"go.uber.org/zap"
	k8scache "k8s.io/apimachinery/pkg/util/cache"
	"k8s.io/client-go/kubernetes"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	"knative.dev/pkg/logging"
)

const (
	disabledError   = "cannot handle resolution request, enable-git-resolver feature flag not true"
	gitResolverName = "Git"

	// labelValueGitResolverType is the value to use for the
	// resolution.tekton.dev/type label on resource requests
	labelValueGitResolverType = "git"

	// size of the LRU secrets cache
	cacheSize = 1024
	// the time to live for a cache entry
	ttl = 5 * time.Minute

	// git revision parameter name
	revisionParam = "revision"
)

var _ framework.Resolver = (*Resolver)(nil)
var _ resolutionframework.ConfigWatcher = (*Resolver)(nil)
var _ cache.ImmutabilityChecker = (*Resolver)(nil)
var _ resolutionframework.TimedResolution = (*Resolver)(nil)

// Resolver implements a framework.Resolver that can fetch files from git.
type Resolver struct {
	kubeClient kubernetes.Interface
	logger     *zap.SugaredLogger
	cache      *k8scache.LRUExpireCache
	ttl        time.Duration

	// Function for creating a SCM client so we can change it in tests.
	clientFunc func(string, string, string, ...factory.ClientOptionFunc) (*scm.Client, error)
}

// Initialize performs any setup required by the git resolver.
func (r *Resolver) Initialize(ctx context.Context) error {
	r.kubeClient = kubeclient.Get(ctx)
	r.logger = logging.FromContext(ctx).Named(gitResolverName)
	if r.cache == nil {
		r.cache = k8scache.NewLRUExpireCache(cacheSize)
	}
	if r.ttl == 0 {
		r.ttl = ttl
	}
	if r.clientFunc == nil {
		r.clientFunc = factory.NewClient
	}
	return nil
}

// GetName returns the string name that the git resolver should be
// associated with.
func (r *Resolver) GetName(_ context.Context) string {
	return gitResolverName
}

// GetSelector returns the labels that resource requests are required to have for
// the gitresolver to process them.
func (r *Resolver) GetSelector(_ context.Context) map[string]string {
	return map[string]string{
		resolutioncommon.LabelKeyResolverType: labelValueGitResolverType,
	}
}

// GetConfigName returns the name of the git resolver's configmap.
func (r *Resolver) GetConfigName(_ context.Context) string {
	return git.ConfigMapName
}

// Validate returns an error if the given parameter map is not
// valid for a resource request targeting the gitresolver.
func (r *Resolver) Validate(ctx context.Context, req *v1beta1.ResolutionRequestSpec) error {
	return git.ValidateParams(ctx, req.Params)
}

// IsImmutable implements ImmutabilityChecker.IsImmutable
// Returns true if the revision parameter is a commit SHA (40-character hex string)
func (r *Resolver) IsImmutable(params []v1.Param) bool {
	var revision string
	for _, param := range params {
		if param.Name == revisionParam {
			revision = param.Value.StringVal
			break
		}
	}

	// Checks if the given string looks like a git commit SHA.
	// A valid commit SHA is exactly 40 characters of hexadecimal.
	if len(revision) != 40 {
		return false
	}
	for _, r := range revision {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')) {
			return false
		}
	}
	return true
}

// GetResolutionTimeout returns the configured timeout for git resolution requests.
func (r *Resolver) GetResolutionTimeout(ctx context.Context, defaultTimeout time.Duration, params map[string]string) (time.Duration, error) {
	conf, err := git.GetScmConfigForParamConfigKey(ctx, params)
	if err != nil {
		return time.Duration(0), err
	}
	if timeoutString := conf.Timeout; timeoutString != "" {
		timeout, err := time.ParseDuration(timeoutString)
		if err != nil {
			return time.Duration(0), err
		}
		return timeout, nil
	}
	return defaultTimeout, nil
}

// Resolve performs the work of fetching a file from git given a map of
// parameters.
func (r *Resolver) Resolve(ctx context.Context, req *v1beta1.ResolutionRequestSpec) (resolutionframework.ResolvedResource, error) {
	if len(req.Params) == 0 {
		return nil, errors.New("no params")
	}

	if git.IsDisabled(ctx) {
		return nil, errors.New(disabledError)
	}

	// Verify resolver was initialized - critical fields must be set by Initialize()
	if r.kubeClient == nil || r.logger == nil || r.cache == nil {
		return nil, errors.New("git resolver not properly initialized: Initialize() must be called before Resolve()")
	}

	params, err := git.PopulateDefaultParams(ctx, req.Params)
	if err != nil {
		return nil, err
	}

	if cache.ShouldUse(ctx, r, req.Params, labelValueGitResolverType) {
		return cache.Use(
			ctx,
			req.Params,
			labelValueGitResolverType,
			func() (resolutionframework.ResolvedResource, error) {
				return r.resolveViaGit(ctx, params)
			},
		)
	}
	return r.resolveViaGit(ctx, params)
}

func (r *Resolver) resolveViaGit(ctx context.Context, params map[string]string) (resolutionframework.ResolvedResource, error) {
	g := &git.GitResolver{
		KubeClient: r.kubeClient,
		Logger:     r.logger,
		Cache:      r.cache,
		TTL:        r.ttl,
		Params:     params,
	}

	if params[git.UrlParam] != "" {
		return g.ResolveGitClone(ctx)
	}

	return g.ResolveAPIGit(ctx, r.clientFunc)
}
