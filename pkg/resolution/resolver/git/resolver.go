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
	"regexp"
	"strings"
	"time"

	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-git/v5"
	gitcfg "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	plumbTransport "github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/jenkins-x/go-scm/scm"
	"github.com/jenkins-x/go-scm/scm/factory"
	resolverconfig "github.com/tektoncd/pipeline/pkg/apis/config/resolver"
	pipelinev1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	common "github.com/tektoncd/pipeline/pkg/resolution/common"
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
//
// Deprecated: Use [github.com/tektoncd/pipeline/pkg/remoteresolution/resolver/git.Resolver] instead.
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
		common.LabelKeyResolverType: labelValueGitResolverType,
	}
}

// ValidateParams returns an error if the given parameter map is not
// valid for a resource request targeting the gitresolver.
func (r *Resolver) ValidateParams(ctx context.Context, params []pipelinev1.Param) error {
	return ValidateParams(ctx, params)
}

// Resolve performs the work of fetching a file from git given a map of
// parameters.
func (r *Resolver) Resolve(ctx context.Context, origParams []pipelinev1.Param) (framework.ResolvedResource, error) {
	if IsDisabled(ctx) {
		return nil, errors.New(disabledError)
	}

	params, err := PopulateDefaultParams(ctx, origParams)
	if err != nil {
		return nil, err
	}

	g := &GitResolver{
		Params:     params,
		Logger:     r.logger,
		Cache:      r.cache,
		TTL:        r.ttl,
		KubeClient: r.kubeClient,
	}

	if params[UrlParam] != "" {
		return g.ResolveGitClone(ctx)
	}

	return g.ResolveAPIGit(ctx, r.clientFunc)
}

func ValidateParams(ctx context.Context, params []pipelinev1.Param) error {
	if IsDisabled(ctx) {
		return errors.New(disabledError)
	}

	_, err := PopulateDefaultParams(ctx, params)
	if err != nil {
		return err
	}
	return nil
}

// validateRepoURL validates if the given URL is a valid git, http, https URL or
// starting with a / (a local repository).
func validateRepoURL(url string) bool {
	// Explanation:
	pattern := `^(/|[^@]+@[^:]+|(git|https?)://)`
	re := regexp.MustCompile(pattern)
	return re.MatchString(url)
}

type GitResolver struct {
	Params     map[string]string
	Logger     *zap.SugaredLogger
	Cache      *cache.LRUExpireCache
	TTL        time.Duration
	KubeClient kubernetes.Interface
}

func (g *GitResolver) ResolveGitClone(ctx context.Context) (framework.ResolvedResource, error) {
	conf, err := GetScmConfigForParamConfigKey(ctx, g.Params)
	if err != nil {
		return nil, err
	}
	repo := g.Params[UrlParam]
	if repo == "" {
		urlString := conf.URL
		if urlString == "" {
			return nil, errors.New("default Git Repo Url was not set during installation of the git resolver")
		}
	}
	revision := g.Params[RevisionParam]
	if revision == "" {
		revisionString := conf.Revision
		if revisionString == "" {
			return nil, errors.New("default Git Revision was not set during installation of the git resolver")
		}
	}

	cloneOpts := &git.CloneOptions{
		URL: repo,
	}

	secretRef := &secretCacheKey{
		name: g.Params[GitTokenParam],
		key:  g.Params[GitTokenKeyParam],
	}
	if secretRef.name != "" {
		if secretRef.key == "" {
			secretRef.key = DefaultTokenKeyParam
		}
		secretRef.ns = common.RequestNamespace(ctx)
	} else {
		secretRef = nil
	}

	auth := plumbTransport.AuthMethod(nil)
	if secretRef != nil {
		gitToken, err := g.getAPIToken(ctx, secretRef, GitTokenKeyParam)
		if err != nil {
			return nil, err
		}
		auth = &http.BasicAuth{
			Username: "git",
			Password: string(gitToken),
		}
		cloneOpts.Auth = auth
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
		Auth:     auth,
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

	path := g.Params[PathParam]

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
		URL:      g.Params[UrlParam],
		Path:     g.Params[PathParam],
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
func (r *Resolver) GetResolutionTimeout(ctx context.Context, defaultTimeout time.Duration, params map[string]string) (time.Duration, error) {
	conf, err := GetScmConfigForParamConfigKey(ctx, params)
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

func PopulateDefaultParams(ctx context.Context, params []pipelinev1.Param) (map[string]string, error) {
	paramsMap := make(map[string]string)
	for _, p := range params {
		paramsMap[p.Name] = p.Value.StringVal
	}

	conf, err := GetScmConfigForParamConfigKey(ctx, paramsMap)
	if err != nil {
		return nil, err
	}

	var missingParams []string

	if _, ok := paramsMap[RevisionParam]; !ok {
		defaultRevision := conf.Revision
		if defaultRevision != "" {
			paramsMap[RevisionParam] = defaultRevision
		} else {
			missingParams = append(missingParams, RevisionParam)
		}
	}
	if _, ok := paramsMap[PathParam]; !ok {
		missingParams = append(missingParams, PathParam)
	}

	if paramsMap[UrlParam] != "" && paramsMap[RepoParam] != "" {
		return nil, fmt.Errorf("cannot specify both '%s' and '%s'", UrlParam, RepoParam)
	}

	if paramsMap[UrlParam] == "" && paramsMap[RepoParam] == "" {
		urlString := conf.URL
		if urlString != "" {
			paramsMap[UrlParam] = urlString
		} else {
			return nil, fmt.Errorf("must specify one of '%s' or '%s'", UrlParam, RepoParam)
		}
	}

	if paramsMap[RepoParam] != "" {
		if _, ok := paramsMap[OrgParam]; !ok {
			defaultOrg := conf.Org
			if defaultOrg != "" {
				paramsMap[OrgParam] = defaultOrg
			} else {
				return nil, fmt.Errorf("'%s' is required when '%s' is specified", OrgParam, RepoParam)
			}
		}
	}
	if len(missingParams) > 0 {
		return nil, fmt.Errorf("missing required git resolver params: %s", strings.Join(missingParams, ", "))
	}

	// validate the url params if we are not using the SCM API
	if paramsMap[RepoParam] == "" && paramsMap[OrgParam] == "" && !validateRepoURL(paramsMap[UrlParam]) {
		return nil, fmt.Errorf("invalid git repository url: %s", paramsMap[UrlParam])
	}

	// TODO(sbwsg): validate pathInRepo is valid relative pathInRepo
	return paramsMap, nil
}

// supports the SPDX format which is recommended by in-toto
// ref: https://spdx.dev/spdx-specification-21-web-version/#h.49x2ik5
// ref: https://github.com/in-toto/attestation/blob/main/spec/field_types.md
func spdxGit(url string) string {
	return "git+" + url
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
		AnnotationKeyRevision:           r.Revision,
		AnnotationKeyPath:               r.Path,
		AnnotationKeyURL:                r.URL,
		common.AnnotationKeyContentType: yamlContentType,
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

func (g *GitResolver) ResolveAPIGit(ctx context.Context, clientFunc func(string, string, string, ...factory.ClientOptionFunc) (*scm.Client, error)) (framework.ResolvedResource, error) {
	// If we got here, the "repo" param was specified, so use the API approach
	scmType, serverURL, err := getSCMTypeAndServerURL(ctx, g.Params)
	if err != nil {
		return nil, err
	}
	secretRef := &secretCacheKey{
		name: g.Params[TokenParam],
		key:  g.Params[TokenKeyParam],
	}
	if secretRef.name != "" {
		if secretRef.key == "" {
			secretRef.key = DefaultTokenKeyParam
		}
		secretRef.ns = common.RequestNamespace(ctx)
	} else {
		secretRef = nil
	}
	apiToken, err := g.getAPIToken(ctx, secretRef, APISecretNameKey)
	if err != nil {
		return nil, err
	}
	scmClient, err := clientFunc(scmType, serverURL, string(apiToken))
	if err != nil {
		return nil, fmt.Errorf("failed to create SCM client: %w", err)
	}

	orgRepo := fmt.Sprintf("%s/%s", g.Params[OrgParam], g.Params[RepoParam])
	path := g.Params[PathParam]
	ref := g.Params[RevisionParam]

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
		Org:      g.Params[OrgParam],
		Repo:     g.Params[RepoParam],
		Path:     content.Path,
		URL:      repo.Clone,
	}, nil
}

func (g *GitResolver) getAPIToken(ctx context.Context, apiSecret *secretCacheKey, key string) ([]byte, error) {
	conf, err := GetScmConfigForParamConfigKey(ctx, g.Params)
	if err != nil {
		return nil, err
	}

	ok := false

	// NOTE(chmouel): only cache secrets when user hasn't passed params in their resolver configuration
	cacheSecret := false
	if apiSecret == nil {
		cacheSecret = true
		apiSecret = &secretCacheKey{}
	}

	if apiSecret.name == "" {
		apiSecret.name = conf.APISecretName
		if apiSecret.name == "" {
			err := fmt.Errorf("cannot get API token, required when specifying '%s' param, '%s' not specified in config", RepoParam, key)
			g.Logger.Info(err)
			return nil, err
		}
	}
	if apiSecret.key == "" {
		apiSecret.key = conf.APISecretKey
		if apiSecret.key == "" {
			err := fmt.Errorf("cannot get API token, required when specifying '%s' param, '%s' not specified in config", RepoParam, APISecretKeyKey)
			g.Logger.Info(err)
			return nil, err
		}
	}
	if apiSecret.ns == "" {
		apiSecret.ns = conf.APISecretNamespace
		if apiSecret.ns == "" {
			apiSecret.ns = os.Getenv("SYSTEM_NAMESPACE")
		}
	}

	if cacheSecret {
		val, ok := g.Cache.Get(apiSecret)
		if ok {
			return val.([]byte), nil
		}
	}

	secret, err := g.KubeClient.CoreV1().Secrets(apiSecret.ns).Get(ctx, apiSecret.name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			notFoundErr := fmt.Errorf("cannot get API token, secret %s not found in namespace %s", apiSecret.name, apiSecret.ns)
			g.Logger.Info(notFoundErr)
			return nil, notFoundErr
		}
		wrappedErr := fmt.Errorf("error reading API token from secret %s in namespace %s: %w", apiSecret.name, apiSecret.ns, err)
		g.Logger.Info(wrappedErr)
		return nil, wrappedErr
	}

	secretVal, ok := secret.Data[apiSecret.key]
	if !ok {
		err := fmt.Errorf("cannot get API token, key %s not found in secret %s in namespace %s", apiSecret.key, apiSecret.name, apiSecret.ns)
		g.Logger.Info(err)
		return nil, err
	}
	if cacheSecret {
		g.Cache.Add(apiSecret, secretVal, ttl)
	}
	return secretVal, nil
}

func getSCMTypeAndServerURL(ctx context.Context, params map[string]string) (string, string, error) {
	conf, err := GetScmConfigForParamConfigKey(ctx, params)
	if err != nil {
		return "", "", err
	}

	var scmType, serverURL string
	if key, ok := params[ScmTypeParam]; ok {
		scmType = key
	}
	if scmType == "" {
		scmType = conf.SCMType
	}
	if key, ok := params[ServerURLParam]; ok {
		serverURL = key
	}
	if serverURL == "" {
		serverURL = conf.ServerURL
	}
	return scmType, serverURL, nil
}

func IsDisabled(ctx context.Context) bool {
	cfg := resolverconfig.FromContextOrDefaults(ctx)
	return !cfg.FeatureFlags.EnableGitResolver
}

func GetScmConfigForParamConfigKey(ctx context.Context, params map[string]string) (ScmConfig, error) {
	gitResolverConfig, err := GetGitResolverConfig(ctx)
	if err != nil {
		return ScmConfig{}, err
	}
	if configKeyToUse, ok := params[ConfigKeyParam]; ok {
		if config, exist := gitResolverConfig[configKeyToUse]; exist {
			return config, nil
		}
		return ScmConfig{}, fmt.Errorf("no git resolver configuration found for configKey %s", configKeyToUse)
	}
	return gitResolverConfig["default"], nil
}
