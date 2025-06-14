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
	"github.com/tektoncd/pipeline/pkg/apis/resolution/v1beta1"
	"github.com/tektoncd/pipeline/pkg/remoteresolution/cache"
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

	// labelValueGitResolverType is the value to use for the
	// resolution.tekton.dev/type label on resource requests
	labelValueGitResolverType string = "git"

	// gitResolverName is the name that the git resolver should be
	// associated with
	gitResolverName string = "Git"

	// ConfigMapName is the git resolver's config map
	ConfigMapName = "git-resolver-config"

	// cacheSize is the size of the LRU secrets cache
	cacheSize = 1024

	// ttl is the time to live for a cache entry
	ttl = 5 * time.Minute

	// CacheModeAlways means always use cache regardless of git revision
	CacheModeAlways = "always"
	// CacheModeNever means never use cache regardless of git revision
	CacheModeNever = "never"
	// CacheModeAuto means use cache only when git revision is a commit hash
	CacheModeAuto = "auto"
)

// IsCommitHash checks if the given string is a valid git commit hash.
// A valid git commit hash is a 40-character hexadecimal string.
func IsCommitHash(s string) bool {
	if len(s) != 40 {
		return false
	}
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

var _ framework.Resolver = &Resolver{}

// Resolver implements a framework.Resolver that can fetch files from git.
type Resolver struct {
	kubeClient kubernetes.Interface
	logger     *zap.SugaredLogger
	cache      *k8scache.LRUExpireCache
	ttl        time.Duration

	// Used in testing
	clientFunc func(string, string, string, ...factory.ClientOptionFunc) (*scm.Client, error)
}

// Initialize performs any setup required by the gitresolver.
func (r *Resolver) Initialize(ctx context.Context) error {
	r.kubeClient = kubeclient.Get(ctx)
	r.logger = logging.FromContext(ctx)
	r.cache = k8scache.NewLRUExpireCache(cacheSize)
	r.ttl = ttl
	if r.clientFunc == nil {
		r.clientFunc = factory.NewClient
	}
	return nil
}

// GetName returns the string name that the gitresolver should be
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

// ValidateParams returns an error if the given parameter map is not
// valid for a resource request targeting the gitresolver.
func (r *Resolver) Validate(ctx context.Context, req *v1beta1.ResolutionRequestSpec) error {
	if len(req.Params) > 0 {
		return git.ValidateParams(ctx, req.Params)
	}
	// Remove this error once validate url has been implemented.
	return errors.New("cannot validate request. the Validate method has not been implemented.")
}

// ShouldUseCache determines if caching should be used based on the cache mode and bundle reference.
func ShouldUseCache(params map[string]string) bool {
	cacheMode := params[git.CacheParam]
	gitRevision := params[git.RevisionParam]
	switch cacheMode {
	case CacheModeAlways:
		return true
	case CacheModeNever:
		return false
	case CacheModeAuto, "": // default to auto if not specified
		return IsCommitHash(gitRevision)
	default: // invalid cache mode defaults to auto
		return IsCommitHash(gitRevision)
	}
}

// Resolve performs the work of fetching a file from git given a map of
// parameters.
func (r *Resolver) Resolve(ctx context.Context, req *v1beta1.ResolutionRequestSpec) (resolutionframework.ResolvedResource, error) {

	if len(req.Params) > 0 {
		if git.IsDisabled(ctx) {
			return nil, errors.New(disabledError)
		}

		params, err := git.PopulateDefaultParams(ctx, req.Params)
		if err != nil {
			return nil, err
		}

		// Check if caching should be enabled
		useCache := ShouldUseCache(params)

		if useCache {
			// Initialize cache logger
			cache.GetGlobalCache().InitializeLogger(ctx)

			// Generate cache key
			cacheKey, err := cache.GenerateCacheKey(labelValueGitResolverType, req.Params)
			if err != nil {
				return nil, err
			}

			// Check cache first
			if cached, ok := cache.GetGlobalCache().Get(cacheKey); ok {
				if resource, ok := cached.(resolutionframework.ResolvedResource); ok {
					return resource, nil
				}
			}
		}

		g := &git.GitResolver{
			KubeClient: r.kubeClient,
			Logger:     r.logger,
			Cache:      r.cache,
			TTL:        r.ttl,
			Params:     params,
		}

		var resource resolutionframework.ResolvedResource
		if params[git.UrlParam] != "" {
			resource, err = g.ResolveGitClone(ctx)
		} else {
			resource, err = g.ResolveAPIGit(ctx, r.clientFunc)
		}
		if err != nil {
			return nil, err
		}

		// Cache the result if caching is enabled
		if useCache {
			cacheKey, _ := cache.GenerateCacheKey(labelValueGitResolverType, req.Params)
			cache.GetGlobalCache().Add(cacheKey, resource)
		}

		return resource, nil
	}
	return nil, errors.New("the Resolve method has not been implemented.")
}

var _ resolutionframework.ConfigWatcher = &Resolver{}

// GetConfigName returns the name of the git resolver's configmap.
func (r *Resolver) GetConfigName(context.Context) string {
	return ConfigMapName
}

var _ resolutionframework.TimedResolution = &Resolver{}

// GetResolutionTimeout returns a time.Duration for the amount of time a
// single git fetch may take. This can be configured with the
// fetch-timeout field in the git-resolver-config configmap.
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
