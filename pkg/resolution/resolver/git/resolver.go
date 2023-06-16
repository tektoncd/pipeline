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
	"os"
	"strings"
	"time"

	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-git/v5"
	gitcfg "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/jenkins-x/go-scm/scm"
	"github.com/jenkins-x/go-scm/scm/factory"
	resolverconfig "github.com/tektoncd/pipeline/pkg/apis/config/resolver"
	pipelinev1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	resolutioncommon "github.com/tektoncd/pipeline/pkg/resolution/common"
	"github.com/tektoncd/pipeline/pkg/resolution/resolver/framework"
	"go.uber.org/zap"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/cache"
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

	// yamlContentType is the content type to use when returning yaml
	yamlContentType string = "application/x-yaml"

	// ConfigMapName is the git resolver's config map
	ConfigMapName = "git-resolver-config"

	// cacheSize is the size of the LRU secrets cache
	cacheSize = 1024
	// ttl is the time to live for a cache entry
	ttl = 5 * time.Minute
)

var _ framework.Resolver = &Resolver{}

// Resolver implements a framework.Resolver that can fetch files from git.
type Resolver struct {
	kubeClient kubernetes.Interface
	logger     *zap.SugaredLogger
	cache      *cache.LRUExpireCache
	ttl        time.Duration

	// Used in testing
	clientFunc func(string, string, string, ...factory.ClientOptionFunc) (*scm.Client, error)
}

// Initialize performs any setup required by the gitresolver.
func (r *Resolver) Initialize(ctx context.Context) error {
	r.kubeClient = kubeclient.Get(ctx)
	r.logger = logging.FromContext(ctx)
	r.cache = cache.NewLRUExpireCache(cacheSize)
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
func (r *Resolver) ValidateParams(ctx context.Context, params []pipelinev1.Param) error {
	if r.isDisabled(ctx) {
		return errors.New(disabledError)
	}

	_, err := populateDefaultParams(ctx, params)
	if err != nil {
		return err
	}
	return nil
}

// Resolve performs the work of fetching a file from git given a map of
// parameters.
func (r *Resolver) Resolve(ctx context.Context, origParams []pipelinev1.Param) (framework.ResolvedResource, error) {
	if r.isDisabled(ctx) {
		return nil, errors.New(disabledError)
	}

	params, err := populateDefaultParams(ctx, origParams)
	if err != nil {
		return nil, err
	}

	if params[urlParam] != "" {
		return r.resolveAnonymousGit(ctx, params)
	}

	return r.resolveAPIGit(ctx, params)
}

func (r *Resolver) resolveAPIGit(ctx context.Context, params map[string]string) (framework.ResolvedResource, error) {
	// If we got here, the "repo" param was specified, so use the API approach
	scmType, serverURL, err := r.getSCMTypeAndServerURL(ctx)
	if err != nil {
		return nil, err
	}
	apiToken, err := r.getAPIToken(ctx)
	if err != nil {
		return nil, err
	}
	scmClient, err := r.clientFunc(scmType, serverURL, string(apiToken))
	if err != nil {
		return nil, fmt.Errorf("failed to create SCM client: %w", err)
	}

	orgRepo := fmt.Sprintf("%s/%s", params[orgParam], params[repoParam])
	path := params[pathParam]
	ref := params[revisionParam]

	// fetch the actual content from a file in the repo
	content, _, err := scmClient.Contents.Find(ctx, orgRepo, path, ref)
	if err != nil {
		return nil, fmt.Errorf("couldn't fetch resource content: %w", err)
	}
	if content == nil || len(content.Data) == 0 {
		return nil, fmt.Errorf("no content for resource in %s %s", orgRepo, path)
	}

	// find the actual git commit sha by the ref
	commit, _, err := scmClient.Git.FindCommit(ctx, orgRepo, ref)
	if err != nil || commit == nil {
		return nil, fmt.Errorf("couldn't fetch the commit sha for the ref %s in the repo: %w", ref, err)
	}

	// fetch the repository URL
	repo, _, err := scmClient.Repositories.Find(ctx, orgRepo)
	if err != nil {
		return nil, fmt.Errorf("couldn't fetch repository: %w", err)
	}

	return &resolvedGitResource{
		Content:  content.Data,
		Revision: commit.Sha,
		Org:      params[orgParam],
		Repo:     params[repoParam],
		Path:     content.Path,
		URL:      repo.Clone,
	}, nil
}

func (r *Resolver) resolveAnonymousGit(ctx context.Context, params map[string]string) (framework.ResolvedResource, error) {
	conf := framework.GetResolverConfigFromContext(ctx)
	repo := params[urlParam]
	if repo == "" {
		if urlString, ok := conf[defaultURLKey]; ok {
			repo = urlString
		} else {
			return nil, fmt.Errorf("default Git Repo Url was not set during installation of the git resolver")
		}
	}
	revision := params[revisionParam]
	if revision == "" {
		if revisionString, ok := conf[defaultRevisionKey]; ok {
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
			return nil, fmt.Errorf("unexpected fetch error: %w", err)
		}
	}

	w, err := repository.Worktree()
	if err != nil {
		return nil, fmt.Errorf("worktree error: %w", err)
	}

	h, err := repository.ResolveRevision(plumbing.Revision(revision))
	if err != nil {
		return nil, fmt.Errorf("revision error: %w", err)
	}

	err = w.Checkout(&git.CheckoutOptions{
		Hash: *h,
	})
	if err != nil {
		return nil, fmt.Errorf("checkout error: %w", err)
	}

	path := params[pathParam]

	f, err := filesystem.Open(path)
	if err != nil {
		return nil, fmt.Errorf("error opening file %q: %w", path, err)
	}

	buf := &bytes.Buffer{}
	_, err = io.Copy(buf, f)
	if err != nil {
		return nil, fmt.Errorf("error reading file %q: %w", path, err)
	}

	return &resolvedGitResource{
		Revision: h.String(),
		Content:  buf.Bytes(),
		URL:      params[urlParam],
		Path:     params[pathParam],
	}, nil
}

var _ framework.ConfigWatcher = &Resolver{}

// GetConfigName returns the name of the git resolver's configmap.
func (r *Resolver) GetConfigName(context.Context) string {
	return ConfigMapName
}

var _ framework.TimedResolution = &Resolver{}

// GetResolutionTimeout returns a time.Duration for the amount of time a
// single git fetch may take. This can be configured with the
// fetch-timeout field in the git-resolver-config configmap.
func (r *Resolver) GetResolutionTimeout(ctx context.Context, defaultTimeout time.Duration) time.Duration {
	conf := framework.GetResolverConfigFromContext(ctx)
	if timeoutString, ok := conf[defaultTimeoutKey]; ok {
		timeout, err := time.ParseDuration(timeoutString)
		if err == nil {
			return timeout
		}
	}
	return defaultTimeout
}

func (r *Resolver) isDisabled(ctx context.Context) bool {
	cfg := resolverconfig.FromContextOrDefaults(ctx)
	return !cfg.FeatureFlags.EnableGitResolver
}

// resolvedGitResource implements framework.ResolvedResource and returns
// the resolved file []byte data and an annotation map for any metadata.
type resolvedGitResource struct {
	Revision string
	Content  []byte
	Org      string
	Repo     string
	Path     string
	URL      string
}

var _ framework.ResolvedResource = &resolvedGitResource{}

// Data returns the bytes of the file resolved from git.
func (r *resolvedGitResource) Data() []byte {
	return r.Content
}

// Annotations returns the metadata that accompanies the file fetched
// from git.
func (r *resolvedGitResource) Annotations() map[string]string {
	m := map[string]string{
		AnnotationKeyRevision:                     r.Revision,
		AnnotationKeyPath:                         r.Path,
		AnnotationKeyURL:                          r.URL,
		resolutioncommon.AnnotationKeyContentType: yamlContentType,
	}

	if r.Org != "" {
		m[AnnotationKeyOrg] = r.Org
	}
	if r.Repo != "" {
		m[AnnotationKeyRepo] = r.Repo
	}

	return m
}

// RefSource is the source reference of the remote data that records where the remote
// file came from including the url, digest and the entrypoint.
func (r *resolvedGitResource) RefSource() *pipelinev1.RefSource {
	return &pipelinev1.RefSource{
		URI: spdxGit(r.URL),
		Digest: map[string]string{
			"sha1": r.Revision,
		},
		EntryPoint: r.Path,
	}
}

type secretCacheKey struct {
	ns   string
	name string
	key  string
}

func (r *Resolver) getSCMTypeAndServerURL(ctx context.Context) (string, string, error) {
	conf := framework.GetResolverConfigFromContext(ctx)

	scmType, ok := conf[SCMTypeKey]
	if !ok || scmType == "" {
		return "", "", fmt.Errorf("missing or empty %s value in configmap", SCMTypeKey)
	}

	serverURL := conf[ServerURLKey]

	return scmType, serverURL, nil
}

func (r *Resolver) getAPIToken(ctx context.Context) ([]byte, error) {
	conf := framework.GetResolverConfigFromContext(ctx)

	cacheKey := secretCacheKey{}

	ok := false

	if cacheKey.name, ok = conf[APISecretNameKey]; !ok || cacheKey.name == "" {
		err := fmt.Errorf("cannot get API token, required when specifying '%s' param, '%s' not specified in config", repoParam, APISecretNameKey)
		r.logger.Info(err)
		return nil, err
	}
	if cacheKey.key, ok = conf[APISecretKeyKey]; !ok || cacheKey.key == "" {
		err := fmt.Errorf("cannot get API token, required when specifying '%s' param, '%s' not specified in config", repoParam, APISecretKeyKey)
		r.logger.Info(err)
		return nil, err
	}
	if cacheKey.ns, ok = conf[APISecretNamespaceKey]; !ok {
		cacheKey.ns = os.Getenv("SYSTEM_NAMESPACE")
	}

	val, ok := r.cache.Get(cacheKey)
	if ok {
		return val.([]byte), nil
	}

	secret, err := r.kubeClient.CoreV1().Secrets(cacheKey.ns).Get(ctx, cacheKey.name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			notFoundErr := fmt.Errorf("cannot get API token, secret %s not found in namespace %s", cacheKey.name, cacheKey.ns)
			r.logger.Info(notFoundErr)
			return nil, notFoundErr
		}
		wrappedErr := fmt.Errorf("error reading API token from secret %s in namespace %s: %w", cacheKey.name, cacheKey.ns, err)
		r.logger.Info(wrappedErr)
		return nil, wrappedErr
	}

	secretVal, ok := secret.Data[cacheKey.key]
	if !ok {
		err := fmt.Errorf("cannot get API token, key %s not found in secret %s in namespace %s", cacheKey.key, cacheKey.name, cacheKey.ns)
		r.logger.Info(err)
		return nil, err
	}
	r.cache.Add(cacheKey, secretVal, r.ttl)
	return secretVal, nil
}

func populateDefaultParams(ctx context.Context, params []pipelinev1.Param) (map[string]string, error) {
	conf := framework.GetResolverConfigFromContext(ctx)

	paramsMap := make(map[string]string)
	for _, p := range params {
		paramsMap[p.Name] = p.Value.StringVal
	}

	var missingParams []string

	if _, ok := paramsMap[revisionParam]; !ok {
		if defaultRevision, ok := conf[defaultRevisionKey]; ok {
			paramsMap[revisionParam] = defaultRevision
		} else {
			missingParams = append(missingParams, revisionParam)
		}
	}
	if _, ok := paramsMap[pathParam]; !ok {
		missingParams = append(missingParams, pathParam)
	}

	if paramsMap[urlParam] != "" && paramsMap[repoParam] != "" {
		return nil, fmt.Errorf("cannot specify both '%s' and '%s'", urlParam, repoParam)
	}

	if paramsMap[urlParam] == "" && paramsMap[repoParam] == "" {
		if urlString, ok := conf[defaultURLKey]; ok {
			paramsMap[urlParam] = urlString
		} else {
			return nil, fmt.Errorf("must specify one of '%s' or '%s'", urlParam, repoParam)
		}
	}

	if paramsMap[repoParam] != "" {
		if _, ok := paramsMap[orgParam]; !ok {
			if defaultOrg, ok := conf[defaultOrgKey]; ok {
				paramsMap[orgParam] = defaultOrg
			} else {
				return nil, fmt.Errorf("'%s' is required when '%s' is specified", orgParam, repoParam)
			}
		}
	}
	if len(missingParams) > 0 {
		return nil, fmt.Errorf("missing required git resolver params: %s", strings.Join(missingParams, ", "))
	}

	// TODO(sbwsg): validate repo url is well-formed, git:// or https://
	// TODO(sbwsg): validate pathInRepo is valid relative pathInRepo
	return paramsMap, nil
}

// supports the SPDX format which is recommended by in-toto
// ref: https://spdx.dev/spdx-specification-21-web-version/#h.49x2ik5
// ref: https://github.com/in-toto/attestation/blob/main/spec/field_types.md
func spdxGit(url string) string {
	return "git+" + url
}
