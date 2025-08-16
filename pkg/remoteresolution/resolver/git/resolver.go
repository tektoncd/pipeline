/*
Copyright 2025 The Tekton Authors

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
	pipelinev1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/apis/resolution/v1beta1"
	"github.com/tektoncd/pipeline/pkg/remoteresolution/resolver/framework"
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
	disabledError = "cannot handle resolution request, enable-git-resolver feature flag not true"

	// ResolverName defines the git resolver's name.
	ResolverName string = "Git"

	// LabelValueGitResolverType is the value to use for the
	// resolution.tekton.dev/type label on resource requests
	LabelValueGitResolverType string = "git"

	// cacheSize is the size of the LRU secrets cache
	cacheSize = 1024
	// ttl is the time to live for a cache entry
	ttl = 5 * time.Minute

	// git revision parameter name
	RevisionParam = "revision"
)

// Resolver implements a framework.Resolver that can fetch files from git.
type Resolver struct {
	kubeClient kubernetes.Interface
	logger     *zap.SugaredLogger
	cache      *k8scache.LRUExpireCache
	ttl        time.Duration

	// Function for creating a SCM client so we can change it in tests.
	clientFunc func(string, string, string, ...factory.ClientOptionFunc) (*scm.Client, error)
}

// Ensure Resolver implements ImmutabilityChecker
var _ framework.ImmutabilityChecker = (*Resolver)(nil)

// Initialize performs any setup required by the git resolver.
func (r *Resolver) Initialize(ctx context.Context) error {
	r.kubeClient = kubeclient.Get(ctx)
	r.logger = logging.FromContext(ctx).Named(ResolverName)
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
func (r *Resolver) GetName(ctx context.Context) string {
	return ResolverName
}

// GetSelector returns the labels that resource requests are required to have for
// the gitresolver to process them.
func (r *Resolver) GetSelector(ctx context.Context) map[string]string {
	return map[string]string{
		resolutioncommon.LabelKeyResolverType: LabelValueGitResolverType,
	}
}

// Validate returns an error if the given parameter map is not
// valid for a resource request targeting the gitresolver.
func (r *Resolver) Validate(ctx context.Context, req *v1beta1.ResolutionRequestSpec) error {
	return git.ValidateParams(ctx, req.Params)
}

// IsImmutable implements ImmutabilityChecker.IsImmutable
// Returns true if the revision parameter is a commit SHA (40-character hex string)
func (r *Resolver) IsImmutable(ctx context.Context, params []pipelinev1.Param) bool {
	var revision string
	for _, param := range params {
		if param.Name == RevisionParam {
			revision = param.Value.StringVal
			break
		}
	}

	return isCommitSHA(revision)
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
	params, err := git.PopulateDefaultParams(ctx, req.Params)
	if err != nil {
		return nil, err
	}
	if r.useCache(ctx, req) {
		return framework.RunCacheOperations(
			ctx,
			req.Params,
			LabelValueGitResolverType,
			func() (resolutionframework.ResolvedResource, error) {
				return r.resolveViaGit(ctx, params)
			},
		)
	}
	return r.resolveViaGit(ctx, params)
}

func (r *Resolver) useCache(ctx context.Context, req *v1beta1.ResolutionRequestSpec) bool {
	return framework.ShouldUseCache(ctx, r, req.Params, "git")
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

// isCommitSHA checks if the given string looks like a git commit SHA.
// A valid commit SHA is exactly 40 characters of hexadecimal.
func isCommitSHA(revision string) bool {
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

var _ resolutionframework.ConfigWatcher = &Resolver{}

// GetConfigName returns the name of the git resolver's configmap.
func (r *Resolver) GetConfigName(context.Context) string {
	return git.ConfigMapName
}

var _ resolutionframework.TimedResolution = &Resolver{}

// GetResolutionTimeout returns the configured timeout for git resolution requests.
func (r *Resolver) GetResolutionTimeout(ctx context.Context, defaultTimeout time.Duration, params map[string]string) (time.Duration, error) {
	if git.IsDisabled(ctx) {
		return defaultTimeout, errors.New(disabledError)
	}
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
