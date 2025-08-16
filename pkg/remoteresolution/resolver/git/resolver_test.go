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
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/jenkins-x/go-scm/scm"
	"github.com/jenkins-x/go-scm/scm/driver/fake"
	"github.com/jenkins-x/go-scm/scm/factory"
	resolverconfig "github.com/tektoncd/pipeline/pkg/apis/config/resolver"
	pipelinev1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/apis/resolution/v1beta1"
	"github.com/tektoncd/pipeline/pkg/internal/resolution"
	ttesting "github.com/tektoncd/pipeline/pkg/reconciler/testing"
	"github.com/tektoncd/pipeline/pkg/remoteresolution/resolver/framework"
	frtesting "github.com/tektoncd/pipeline/pkg/remoteresolution/resolver/framework/testing"
	common "github.com/tektoncd/pipeline/pkg/resolution/common"
	resolutionframework "github.com/tektoncd/pipeline/pkg/resolution/resolver/framework"
	frameworktesting "github.com/tektoncd/pipeline/pkg/resolution/resolver/framework/testing"
	"github.com/tektoncd/pipeline/pkg/resolution/resolver/git"
	gitresolution "github.com/tektoncd/pipeline/pkg/resolution/resolver/git"
	"github.com/tektoncd/pipeline/test"
	"github.com/tektoncd/pipeline/test/diff"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/system"
	_ "knative.dev/pkg/system/testing"
)

func TestGetSelector(t *testing.T) {
	resolver := Resolver{}
	sel := resolver.GetSelector(t.Context())
	if typ, has := sel[common.LabelKeyResolverType]; !has {
		t.Fatalf("unexpected selector: %v", sel)
	} else if typ != LabelValueGitResolverType {
		t.Fatalf("unexpected type: %q", typ)
	}
}

func TestValidateParams(t *testing.T) {
	tests := []struct {
		name    string
		wantErr string
		params  map[string]string
	}{
		{
			name: "params with revision",
			params: map[string]string{
				gitresolution.UrlParam:      "http://foo/bar/hello/moto",
				gitresolution.PathParam:     "bar",
				gitresolution.RevisionParam: "baz",
			},
		},
		{
			name: "https url",
			params: map[string]string{
				gitresolution.UrlParam:      "https://foo/bar/hello/moto",
				gitresolution.PathParam:     "bar",
				gitresolution.RevisionParam: "baz",
			},
		},
		{
			name: "https url with username password",
			params: map[string]string{
				gitresolution.UrlParam:      "https://user:pass@foo/bar/hello/moto",
				gitresolution.PathParam:     "bar",
				gitresolution.RevisionParam: "baz",
			},
		},
		{
			name: "git server url",
			params: map[string]string{
				gitresolution.UrlParam:      "git://repo/hello/moto",
				gitresolution.PathParam:     "bar",
				gitresolution.RevisionParam: "baz",
			},
		},
		{
			name: "git url from a local repository",
			params: map[string]string{
				gitresolution.UrlParam:      "/tmp/repo",
				gitresolution.PathParam:     "bar",
				gitresolution.RevisionParam: "baz",
			},
		},
		{
			name: "git url from a git ssh repository",
			params: map[string]string{
				gitresolution.UrlParam:      "git@host.com:foo/bar",
				gitresolution.PathParam:     "bar",
				gitresolution.RevisionParam: "baz",
			},
		},
		{
			name: "bad url",
			params: map[string]string{
				gitresolution.UrlParam:      "foo://bar",
				gitresolution.PathParam:     "path",
				gitresolution.RevisionParam: "revision",
			},
			wantErr: "invalid git repository url: foo://bar",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolver := Resolver{}
			err := resolver.Validate(t.Context(), &v1beta1.ResolutionRequestSpec{Params: toParams(tt.params)})
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error validating params: %v", err)
				}
				return
			}

			if d := cmp.Diff(tt.wantErr, err.Error()); d != "" {
				t.Errorf("unexpected error: %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestValidateParamsNotEnabled(t *testing.T) {
	resolver := Resolver{}

	var err error

	someParams := map[string]string{
		gitresolution.PathParam:     "bar",
		gitresolution.RevisionParam: "baz",
	}
	err = resolver.Validate(resolverDisabledContext(), &v1beta1.ResolutionRequestSpec{Params: toParams(someParams)})
	if err == nil {
		t.Fatalf("expected disabled err")
	}
	if d := cmp.Diff(disabledError, err.Error()); d != "" {
		t.Errorf("unexpected error: %s", diff.PrintWantGot(d))
	}
}

func TestValidateParams_Failure(t *testing.T) {
	testCases := []struct {
		name        string
		params      map[string]string
		expectedErr string
	}{
		{
			name: "missing multiple",
			params: map[string]string{
				gitresolution.OrgParam:  "abcd1234",
				gitresolution.RepoParam: "foo",
			},
			expectedErr: fmt.Sprintf("missing required git resolver params: %s, %s", gitresolution.RevisionParam, gitresolution.PathParam),
		}, {
			name: "no repo or url",
			params: map[string]string{
				gitresolution.RevisionParam: "abcd1234",
				gitresolution.PathParam:     "/foo/bar",
			},
			expectedErr: "must specify one of 'url' or 'repo'",
		}, {
			name: "both repo and url",
			params: map[string]string{
				gitresolution.RevisionParam: "abcd1234",
				gitresolution.PathParam:     "/foo/bar",
				gitresolution.UrlParam:      "http://foo",
				gitresolution.RepoParam:     "foo",
			},
			expectedErr: "cannot specify both 'url' and 'repo'",
		}, {
			name: "no org with repo",
			params: map[string]string{
				gitresolution.RevisionParam: "abcd1234",
				gitresolution.PathParam:     "/foo/bar",
				gitresolution.RepoParam:     "foo",
			},
			expectedErr: "'org' is required when 'repo' is specified",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			resolver := &Resolver{}
			err := resolver.Validate(t.Context(), &v1beta1.ResolutionRequestSpec{Params: toParams(tc.params)})
			if err == nil {
				t.Fatalf("got no error, but expected: %s", tc.expectedErr)
			}
			if d := cmp.Diff(tc.expectedErr, err.Error()); d != "" {
				t.Errorf("error did not match: %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestGetResolutionTimeoutDefault(t *testing.T) {
	resolver := Resolver{}
	defaultTimeout := 30 * time.Minute
	timeout, err := resolver.GetResolutionTimeout(t.Context(), defaultTimeout, map[string]string{})
	if err != nil {
		t.Fatalf("couldn't get default-timeout: %v", err)
	}
	if timeout != defaultTimeout {
		t.Fatalf("expected default timeout to be returned")
	}
}

func TestGetResolutionTimeoutCustom(t *testing.T) {
	resolver := Resolver{}
	defaultTimeout := 30 * time.Minute
	configTimeout := 5 * time.Second
	config := map[string]string{
		gitresolution.DefaultTimeoutKey: configTimeout.String(),
	}
	ctx := resolutionframework.InjectResolverConfigToContext(t.Context(), config)
	timeout, err := resolver.GetResolutionTimeout(ctx, defaultTimeout, map[string]string{})
	if err != nil {
		t.Fatalf("couldn't get default-timeout: %v", err)
	}
	if timeout != configTimeout {
		t.Fatalf("expected timeout from config to be returned")
	}
}

func TestGetResolutionTimeoutCustomIdentifier(t *testing.T) {
	resolver := Resolver{}
	defaultTimeout := 30 * time.Minute
	configTimeout := 5 * time.Second
	identifierConfigTImeout := 10 * time.Second
	config := map[string]string{
		gitresolution.DefaultTimeoutKey:          configTimeout.String(),
		"foo." + gitresolution.DefaultTimeoutKey: identifierConfigTImeout.String(),
	}
	ctx := resolutionframework.InjectResolverConfigToContext(t.Context(), config)
	timeout, err := resolver.GetResolutionTimeout(ctx, defaultTimeout, map[string]string{"configKey": "foo"})
	if err != nil {
		t.Fatalf("couldn't get default-timeout: %v", err)
	}
	if timeout != identifierConfigTImeout {
		t.Fatalf("expected timeout from config to be returned")
	}
}

func TestResolveNotEnabled(t *testing.T) {
	resolver := Resolver{}

	var err error

	someParams := map[string]string{
		gitresolution.PathParam:     "bar",
		gitresolution.RevisionParam: "baz",
	}
	_, err = resolver.Resolve(resolverDisabledContext(), &v1beta1.ResolutionRequestSpec{Params: toParams(someParams)})
	if err == nil {
		t.Fatalf("expected disabled err")
	}
	if d := cmp.Diff(disabledError, err.Error()); d != "" {
		t.Errorf("unexpected error: %s", diff.PrintWantGot(d))
	}
}

type params struct {
	url        string
	revision   string
	pathInRepo string
	org        string
	repo       string
	token      string
	tokenKey   string
	namespace  string
	serverURL  string
	scmType    string
	configKey  string
}

func TestResolve(t *testing.T) {
	// local repo set up for anonymous cloning
	// ----
	commits := []commitForRepo{{
		Dir:      "foo/",
		Filename: "old",
		Content:  "old content in test branch",
		Branch:   "test-branch",
	}, {
		Dir:      "foo/",
		Filename: "new",
		Content:  "new content in test branch",
		Branch:   "test-branch",
	}, {
		Dir:      "./",
		Filename: "released",
		Content:  "released content in main branch and in tag v1",
		Tag:      "v1",
	}}

	anonFakeRepoURL, commitSHAsInAnonRepo := createTestRepo(t, commits)

	// local repo set up for scm cloning
	// ----
	testOrg := "test-org"
	testRepo := "test-repo"

	refsDir := filepath.Join("testdata", "test-org", "test-repo", "refs")
	mainPipelineYAML, err := os.ReadFile(filepath.Join(refsDir, "main", "pipelines", "example-pipeline.yaml"))
	if err != nil {
		t.Fatalf("couldn't read main pipeline: %v", err)
	}
	otherPipelineYAML, err := os.ReadFile(filepath.Join(refsDir, "other", "pipelines", "example-pipeline.yaml"))
	if err != nil {
		t.Fatalf("couldn't read other pipeline: %v", err)
	}

	mainTaskYAML, err := os.ReadFile(filepath.Join(refsDir, "main", "tasks", "example-task.yaml"))
	if err != nil {
		t.Fatalf("couldn't read main task: %v", err)
	}

	commitSHAsInSCMRepo := []string{"abc", "xyz"}

	scmFakeRepoURL := fmt.Sprintf("https://fake/%s/%s.git", testOrg, testRepo)
	resolver := &Resolver{
		clientFunc: func(driver string, serverURL string, token string, opts ...factory.ClientOptionFunc) (*scm.Client, error) {
			scmClient, scmData := fake.NewDefault()

			// repository service
			scmData.Repositories = []*scm.Repository{{
				FullName: fmt.Sprintf("%s/%s", testOrg, testRepo),
				Clone:    scmFakeRepoURL,
			}}

			// git service
			scmData.Commits = map[string]*scm.Commit{
				"main":  {Sha: commitSHAsInSCMRepo[0]},
				"other": {Sha: commitSHAsInSCMRepo[1]},
			}
			return scmClient, nil
		},
	}

	testCases := []struct {
		name              string
		args              *params
		config            map[string]string
		apiToken          string
		expectedCommitSHA string
		expectedStatus    *v1beta1.ResolutionRequestStatus
		expectedErr       error
		configIdentifer   string
	}{{
		name: "clone: default revision main",
		args: &params{
			pathInRepo: "./released",
			url:        anonFakeRepoURL,
		},
		expectedCommitSHA: commitSHAsInAnonRepo[2],
		expectedStatus:    resolution.CreateResolutionRequestStatusWithData([]byte("released content in main branch and in tag v1")),
	}, {
		name: "clone: revision is tag name",
		args: &params{
			revision:   "v1",
			pathInRepo: "./released",
			url:        anonFakeRepoURL,
		},
		expectedCommitSHA: commitSHAsInAnonRepo[2],
		expectedStatus:    resolution.CreateResolutionRequestStatusWithData([]byte("released content in main branch and in tag v1")),
	}, {
		name: "clone: revision is the full tag name i.e. refs/tags/v1",
		args: &params{
			revision:   "refs/tags/v1",
			pathInRepo: "./released",
			url:        anonFakeRepoURL,
		},
		expectedCommitSHA: commitSHAsInAnonRepo[2],
		expectedStatus:    resolution.CreateResolutionRequestStatusWithData([]byte("released content in main branch and in tag v1")),
	}, {
		name: "clone: revision is a branch name",
		args: &params{
			revision:   "test-branch",
			pathInRepo: "foo/new",
			url:        anonFakeRepoURL,
		},
		expectedCommitSHA: commitSHAsInAnonRepo[1],
		expectedStatus:    resolution.CreateResolutionRequestStatusWithData([]byte("new content in test branch")),
	}, {
		name: "clone: revision is a specific commit sha",
		args: &params{
			revision:   commitSHAsInAnonRepo[0],
			pathInRepo: "foo/old",
			url:        anonFakeRepoURL,
		},
		expectedCommitSHA: commitSHAsInAnonRepo[0],
		expectedStatus:    resolution.CreateResolutionRequestStatusWithData([]byte("old content in test branch")),
	}, {
		name: "clone: file does not exist",
		args: &params{
			pathInRepo: "foo/non-exist",
			url:        anonFakeRepoURL,
		},
		expectedErr: createError(`error opening file "foo/non-exist": file does not exist`),
	}, {
		name: "clone: revision does not exist",
		args: &params{
			revision:   "non-existent-revision",
			pathInRepo: "foo/new",
			url:        anonFakeRepoURL,
		},
		expectedErr: createError("git fetch error: fatal: couldn't find remote ref non-existent-revision: exit status 128"),
	}, {
		name: "api: successful task from params api information",
		args: &params{
			revision:   "main",
			pathInRepo: "tasks/example-task.yaml",
			org:        testOrg,
			repo:       testRepo,
			token:      "token-secret",
			tokenKey:   "token",
			namespace:  "foo",
		},
		config: map[string]string{
			gitresolution.ServerURLKey: "fake",
			gitresolution.SCMTypeKey:   "fake",
		},
		apiToken:          "some-token",
		expectedCommitSHA: commitSHAsInSCMRepo[0],
		expectedStatus:    resolution.CreateResolutionRequestStatusWithData(mainTaskYAML),
	}, {
		name: "api: successful task",
		args: &params{
			revision:   "main",
			pathInRepo: "tasks/example-task.yaml",
			org:        testOrg,
			repo:       testRepo,
		},
		config: map[string]string{
			gitresolution.ServerURLKey:          "fake",
			gitresolution.SCMTypeKey:            "fake",
			gitresolution.APISecretNameKey:      "token-secret",
			gitresolution.APISecretKeyKey:       "token",
			gitresolution.APISecretNamespaceKey: system.Namespace(),
		},
		apiToken:          "some-token",
		expectedCommitSHA: commitSHAsInSCMRepo[0],
		expectedStatus:    resolution.CreateResolutionRequestStatusWithData(mainTaskYAML),
	}, {
		name: "api: successful task from params api information with identifier",
		args: &params{
			revision:   "main",
			pathInRepo: "tasks/example-task.yaml",
			org:        testOrg,
			repo:       testRepo,
			token:      "token-secret",
			tokenKey:   "token",
			namespace:  "foo",
			configKey:  "test",
		},
		config: map[string]string{
			"test." + gitresolution.ServerURLKey: "fake",
			"test." + gitresolution.SCMTypeKey:   "fake",
		},
		configIdentifer:   "test.",
		apiToken:          "some-token",
		expectedCommitSHA: commitSHAsInSCMRepo[0],
		expectedStatus:    resolution.CreateResolutionRequestStatusWithData(mainTaskYAML),
	}, {
		name: "api: successful task with identifier",
		args: &params{
			revision:   "main",
			pathInRepo: "tasks/example-task.yaml",
			org:        testOrg,
			repo:       testRepo,
			configKey:  "test",
		},
		config: map[string]string{
			"test." + gitresolution.ServerURLKey:          "fake",
			"test." + gitresolution.SCMTypeKey:            "fake",
			"test." + gitresolution.APISecretNameKey:      "token-secret",
			"test." + gitresolution.APISecretKeyKey:       "token",
			"test." + gitresolution.APISecretNamespaceKey: system.Namespace(),
		},
		configIdentifer:   "test.",
		apiToken:          "some-token",
		expectedCommitSHA: commitSHAsInSCMRepo[0],
		expectedStatus:    resolution.CreateResolutionRequestStatusWithData(mainTaskYAML),
	}, {
		name: "api: successful pipeline",
		args: &params{
			revision:   "main",
			pathInRepo: "pipelines/example-pipeline.yaml",
			org:        testOrg,
			repo:       testRepo,
		},
		config: map[string]string{
			gitresolution.ServerURLKey:          "fake",
			gitresolution.SCMTypeKey:            "fake",
			gitresolution.APISecretNameKey:      "token-secret",
			gitresolution.APISecretKeyKey:       "token",
			gitresolution.APISecretNamespaceKey: system.Namespace(),
		},
		apiToken:          "some-token",
		expectedCommitSHA: commitSHAsInSCMRepo[0],
		expectedStatus:    resolution.CreateResolutionRequestStatusWithData(mainPipelineYAML),
	}, {
		name: "api: successful pipeline with default revision",
		args: &params{
			pathInRepo: "pipelines/example-pipeline.yaml",
			org:        testOrg,
			repo:       testRepo,
		},
		config: map[string]string{
			gitresolution.ServerURLKey:          "fake",
			gitresolution.SCMTypeKey:            "fake",
			gitresolution.APISecretNameKey:      "token-secret",
			gitresolution.APISecretKeyKey:       "token",
			gitresolution.APISecretNamespaceKey: system.Namespace(),
			gitresolution.DefaultRevisionKey:    "other",
		},
		apiToken:          "some-token",
		expectedCommitSHA: commitSHAsInSCMRepo[1],
		expectedStatus:    resolution.CreateResolutionRequestStatusWithData(otherPipelineYAML),
	}, {
		name: "api: successful override scm type and server URL from user params",

		args: &params{
			revision:   "main",
			pathInRepo: "tasks/example-task.yaml",
			org:        testOrg,
			repo:       testRepo,
			token:      "token-secret",
			tokenKey:   "token",
			namespace:  "foo",
			scmType:    "fake",
			serverURL:  "fake",
		},
		config: map[string]string{
			gitresolution.ServerURLKey: "notsofake",
			gitresolution.SCMTypeKey:   "definitivelynotafake",
		},
		apiToken:          "some-token",
		expectedCommitSHA: commitSHAsInSCMRepo[0],
		expectedStatus:    resolution.CreateResolutionRequestStatusWithData(mainTaskYAML),
	}, {
		name: "api: file does not exist",
		args: &params{
			revision:   "main",
			pathInRepo: "pipelines/other-pipeline.yaml",
			org:        testOrg,
			repo:       testRepo,
		},
		config: map[string]string{
			gitresolution.ServerURLKey:          "fake",
			gitresolution.SCMTypeKey:            "fake",
			gitresolution.APISecretNameKey:      "token-secret",
			gitresolution.APISecretKeyKey:       "token",
			gitresolution.APISecretNamespaceKey: system.Namespace(),
		},
		apiToken:       "some-token",
		expectedStatus: resolution.CreateResolutionRequestFailureStatus(),
		expectedErr:    createError("couldn't fetch resource content: file testdata/test-org/test-repo/refs/main/pipelines/other-pipeline.yaml does not exist: stat testdata/test-org/test-repo/refs/main/pipelines/other-pipeline.yaml: no such file or directory"),
	}, {
		name: "api: token not found",
		args: &params{
			revision:   "main",
			pathInRepo: "pipelines/example-pipeline.yaml",
			org:        testOrg,
			repo:       testRepo,
		},
		config: map[string]string{
			gitresolution.ServerURLKey:          "fake",
			gitresolution.SCMTypeKey:            "fake",
			gitresolution.APISecretNameKey:      "token-secret",
			gitresolution.APISecretKeyKey:       "token",
			gitresolution.APISecretNamespaceKey: system.Namespace(),
		},
		expectedStatus: resolution.CreateResolutionRequestFailureStatus(),
		expectedErr:    createError("cannot get API token, secret token-secret not found in namespace " + system.Namespace()),
	}, {
		name: "api: token secret name not specified",
		args: &params{
			revision:   "main",
			pathInRepo: "pipelines/example-pipeline.yaml",
			org:        testOrg,
			repo:       testRepo,
		},
		config: map[string]string{
			gitresolution.ServerURLKey:          "fake",
			gitresolution.SCMTypeKey:            "fake",
			gitresolution.APISecretKeyKey:       "token",
			gitresolution.APISecretNamespaceKey: system.Namespace(),
		},
		apiToken:       "some-token",
		expectedStatus: resolution.CreateResolutionRequestFailureStatus(),
		expectedErr:    createError("cannot get API token, required when specifying 'repo' param, 'api-token-secret-name' not specified in config"),
	}, {
		name: "api: token secret key not specified",
		args: &params{
			revision:   "main",
			pathInRepo: "pipelines/example-pipeline.yaml",
			org:        testOrg,
			repo:       testRepo,
		},
		config: map[string]string{
			gitresolution.ServerURLKey:          "fake",
			gitresolution.SCMTypeKey:            "fake",
			gitresolution.APISecretNameKey:      "token-secret",
			gitresolution.APISecretNamespaceKey: system.Namespace(),
		},
		apiToken:       "some-token",
		expectedStatus: resolution.CreateResolutionRequestFailureStatus(),
		expectedErr:    createError("cannot get API token, required when specifying 'repo' param, 'api-token-secret-key' not specified in config"),
	}, {
		name: "api: SCM type not specified",
		args: &params{
			revision:   "main",
			pathInRepo: "pipelines/example-pipeline.yaml",
			org:        testOrg,
			repo:       testRepo,
		},
		config: map[string]string{
			gitresolution.APISecretNameKey:      "token-secret",
			gitresolution.APISecretKeyKey:       "token",
			gitresolution.APISecretNamespaceKey: system.Namespace(),
		},
		apiToken:          "some-token",
		expectedCommitSHA: commitSHAsInSCMRepo[0],
		expectedStatus:    resolution.CreateResolutionRequestStatusWithData(mainPipelineYAML),
	}}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, _ := ttesting.SetupFakeContext(t)

			cfg := tc.config
			if cfg == nil {
				cfg = make(map[string]string)
			}
			cfg[tc.configIdentifer+gitresolution.DefaultTimeoutKey] = "1m"
			if cfg[tc.configIdentifer+gitresolution.DefaultRevisionKey] == "" {
				cfg[tc.configIdentifer+gitresolution.DefaultRevisionKey] = defaultBranch
			}

			request := createRequest(tc.args)

			d := test.Data{
				ConfigMaps: []*corev1.ConfigMap{{
					ObjectMeta: metav1.ObjectMeta{
						Name:      git.ConfigMapName,
						Namespace: resolverconfig.ResolversNamespace(system.Namespace()),
					},
					Data: cfg,
				}, {
					ObjectMeta: metav1.ObjectMeta{
						Namespace: resolverconfig.ResolversNamespace(system.Namespace()),
						Name:      resolverconfig.GetFeatureFlagsConfigName(),
					},
					Data: map[string]string{
						"enable-git-resolver": "true",
					},
				}},
				ResolutionRequests: []*v1beta1.ResolutionRequest{request},
			}

			var expectedStatus *v1beta1.ResolutionRequestStatus
			if tc.expectedStatus != nil {
				expectedStatus = tc.expectedStatus.DeepCopy()

				if tc.expectedErr == nil {
					// status.annotations
					if expectedStatus.Annotations == nil {
						expectedStatus.Annotations = make(map[string]string)
					}
					expectedStatus.Annotations[common.AnnotationKeyContentType] = "application/x-yaml"
					expectedStatus.Annotations[gitresolution.AnnotationKeyRevision] = tc.expectedCommitSHA
					expectedStatus.Annotations[gitresolution.AnnotationKeyPath] = tc.args.pathInRepo

					if tc.args.url != "" {
						expectedStatus.Annotations[gitresolution.AnnotationKeyURL] = anonFakeRepoURL
					} else {
						expectedStatus.Annotations[gitresolution.AnnotationKeyOrg] = testOrg
						expectedStatus.Annotations[gitresolution.AnnotationKeyRepo] = testRepo
						expectedStatus.Annotations[gitresolution.AnnotationKeyURL] = scmFakeRepoURL
					}

					// status.refSource
					expectedStatus.RefSource = &pipelinev1.RefSource{
						URI: "git+" + expectedStatus.Annotations[gitresolution.AnnotationKeyURL],
						Digest: map[string]string{
							"sha1": tc.expectedCommitSHA,
						},
						EntryPoint: tc.args.pathInRepo,
					}
					expectedStatus.Source = expectedStatus.RefSource
				} else {
					expectedStatus.Status.Conditions[0].Message = tc.expectedErr.Error()
				}
			}

			frtesting.RunResolverReconcileTest(ctx, t, d, resolver, request, expectedStatus, tc.expectedErr, func(resolver framework.Resolver, testAssets test.Assets) {
				var secretName, secretNameKey, secretNamespace string
				if tc.config[tc.configIdentifer+gitresolution.APISecretNameKey] != "" && tc.config[tc.configIdentifer+gitresolution.APISecretNamespaceKey] != "" && tc.config[tc.configIdentifer+gitresolution.APISecretKeyKey] != "" && tc.apiToken != "" {
					secretName, secretNameKey, secretNamespace = tc.config[tc.configIdentifer+gitresolution.APISecretNameKey], tc.config[tc.configIdentifer+gitresolution.APISecretKeyKey], tc.config[tc.configIdentifer+gitresolution.APISecretNamespaceKey]
				}
				if tc.args.token != "" && tc.args.namespace != "" && tc.args.tokenKey != "" {
					secretName, secretNameKey, secretNamespace = tc.args.token, tc.args.tokenKey, tc.args.namespace
				}
				if secretName == "" || secretNameKey == "" || secretNamespace == "" {
					return
				}
				tokenSecret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      secretName,
						Namespace: secretNamespace,
					},
					Data: map[string][]byte{
						secretNameKey: []byte(base64.StdEncoding.Strict().EncodeToString([]byte(tc.apiToken))),
					},
					Type: corev1.SecretTypeOpaque,
				}
				if _, err := testAssets.Clients.Kube.CoreV1().Secrets(secretNamespace).Create(ctx, tokenSecret, metav1.CreateOptions{}); err != nil {
					t.Fatalf("failed to create test token secret: %v", err)
				}
			})
		})
	}
}

const defaultBranch string = "main"

func getGitCmd(t *testing.T, dir string) func(...string) *exec.Cmd {
	t.Helper()
	withTemporaryGitConfig(t)
	return func(args ...string) *exec.Cmd {
		args = append(
			[]string{
				"-C", dir,
				"-c", "user.email=test@test.com",
				"-c", "user.name=PipelinesTests",
			},
			args...)
		return exec.Command("git", args...)
	}
}

// createTestRepo is used to instantiate a local test repository with the desired commits.
func createTestRepo(t *testing.T, commits []commitForRepo) (string, []string) {
	t.Helper()
	commitSHAs := []string{}

	tempDir := t.TempDir()
	gitCmd := getGitCmd(t, tempDir)
	err := gitCmd("init", "-b", "main").Run()
	if err != nil {
		t.Fatalf("couldn't create test repo: %v", err)
	}

	writeAndCommitToTestRepo(t, tempDir, "", "README", []byte("This is a test"))

	hashesByBranch := make(map[string][]string)

	// Iterate over the commits and add them.
	for _, cmt := range commits {
		branch := cmt.Branch
		if branch == "" {
			branch = defaultBranch
		}

		// If we're given a revision, check out that revision.
		checkoutCmd := gitCmd("checkout")
		if _, ok := hashesByBranch[branch]; !ok && branch != defaultBranch {
			checkoutCmd.Args = append(checkoutCmd.Args, "-b")
		}
		checkoutCmd.Args = append(checkoutCmd.Args, branch)

		if err := checkoutCmd.Run(); err != nil {
			t.Fatalf("couldn't do checkout of %s: %v", branch, err)
		}

		hash := writeAndCommitToTestRepo(t, tempDir, cmt.Dir, cmt.Filename, []byte(cmt.Content))
		commitSHAs = append(commitSHAs, hash)

		if _, ok := hashesByBranch[branch]; !ok {
			hashesByBranch[branch] = []string{hash}
		} else {
			hashesByBranch[branch] = append(hashesByBranch[branch], hash)
		}

		if cmt.Tag != "" {
			err = gitCmd("tag", cmt.Tag).Run()
			if err != nil {
				t.Fatalf("couldn't add tag for %s: %v", cmt.Tag, err)
			}
		}
	}

	return tempDir, commitSHAs
}

// commitForRepo provides the directory, filename, content and revision for a test commit.
type commitForRepo struct {
	Dir      string
	Filename string
	Content  string
	Branch   string
	Tag      string
}

func writeAndCommitToTestRepo(t *testing.T, repoDir string, subPath string, filename string, content []byte) string {
	t.Helper()

	gitCmd := getGitCmd(t, repoDir)

	targetDir := repoDir
	if subPath != "" {
		targetDir = filepath.Join(targetDir, subPath)
		fi, err := os.Stat(targetDir)
		if os.IsNotExist(err) {
			if err := os.MkdirAll(targetDir, 0o700); err != nil {
				t.Fatalf("couldn't create directory %s in worktree: %v", targetDir, err)
			}
		} else if err != nil {
			t.Fatalf("checking if directory %s in worktree exists: %v", targetDir, err)
		}
		if fi != nil && !fi.IsDir() {
			t.Fatalf("%s already exists but is not a directory", targetDir)
		}
	}

	outfile := filepath.Join(targetDir, filename)
	if err := os.WriteFile(outfile, content, 0o600); err != nil {
		t.Fatalf("couldn't write content to file %s: %v", outfile, err)
	}

	err := gitCmd("add", outfile).Run()
	if err != nil {
		t.Fatalf("couldn't add file %s to git: %v", outfile, err)
	}

	err = gitCmd("commit", "-m", "adding file for test").Run()
	if err != nil {
		t.Fatalf("couldn't perform commit for test: %v", err)
	}

	out, err := gitCmd("rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatalf("couldn't parse HEAD revision: %v", err)
	}

	return strings.TrimSpace(string(out))
}

// withTemporaryGitConfig resets the .gitconfig for the duration of the test.
func withTemporaryGitConfig(t *testing.T) {
	t.Helper()
	gitConfigDir := t.TempDir()
	key := "GIT_CONFIG_GLOBAL"
	t.Setenv(key, filepath.Join(gitConfigDir, "config"))
}

func createRequest(args *params) *v1beta1.ResolutionRequest {
	rr := &v1beta1.ResolutionRequest{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "resolution.tekton.dev/v1beta1",
			Kind:       "ResolutionRequest",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:              "rr",
			Namespace:         "foo",
			CreationTimestamp: metav1.Time{Time: time.Now()},
			Labels: map[string]string{
				common.LabelKeyResolverType: LabelValueGitResolverType,
			},
		},
		Spec: v1beta1.ResolutionRequestSpec{
			Params: []pipelinev1.Param{{
				Name:  gitresolution.PathParam,
				Value: *pipelinev1.NewStructuredValues(args.pathInRepo),
			}},
		},
	}

	if args.revision != "" {
		rr.Spec.Params = append(rr.Spec.Params, pipelinev1.Param{
			Name:  gitresolution.RevisionParam,
			Value: *pipelinev1.NewStructuredValues(args.revision),
		})
	}

	if args.serverURL != "" {
		rr.Spec.Params = append(rr.Spec.Params, pipelinev1.Param{
			Name:  gitresolution.ServerURLParam,
			Value: *pipelinev1.NewStructuredValues(args.serverURL),
		})
	}
	if args.scmType != "" {
		rr.Spec.Params = append(rr.Spec.Params, pipelinev1.Param{
			Name:  gitresolution.ScmTypeParam,
			Value: *pipelinev1.NewStructuredValues(args.scmType),
		})
	}

	if args.url != "" {
		rr.Spec.Params = append(rr.Spec.Params, pipelinev1.Param{
			Name:  gitresolution.UrlParam,
			Value: *pipelinev1.NewStructuredValues(args.url),
		})
	} else {
		rr.Spec.Params = append(rr.Spec.Params, pipelinev1.Param{
			Name:  gitresolution.RepoParam,
			Value: *pipelinev1.NewStructuredValues(args.repo),
		})
		rr.Spec.Params = append(rr.Spec.Params, pipelinev1.Param{
			Name:  gitresolution.OrgParam,
			Value: *pipelinev1.NewStructuredValues(args.org),
		})
		if args.token != "" {
			rr.Spec.Params = append(rr.Spec.Params, pipelinev1.Param{
				Name:  gitresolution.TokenParam,
				Value: *pipelinev1.NewStructuredValues(args.token),
			})
			rr.Spec.Params = append(rr.Spec.Params, pipelinev1.Param{
				Name:  gitresolution.TokenKeyParam,
				Value: *pipelinev1.NewStructuredValues(args.tokenKey),
			})
		}
	}

	if args.configKey != "" {
		rr.Spec.Params = append(rr.Spec.Params, pipelinev1.Param{
			Name:  gitresolution.ConfigKeyParam,
			Value: *pipelinev1.NewStructuredValues(args.configKey),
		})
	}

	return rr
}

func resolverDisabledContext() context.Context {
	return frameworktesting.ContextWithGitResolverDisabled(context.Background())
}

func createError(msg string) error {
	return &common.GetResourceError{
		ResolverName: ResolverName,
		Key:          "foo/rr",
		Original:     errors.New(msg),
	}
}

func toParams(m map[string]string) []pipelinev1.Param {
	var params []pipelinev1.Param

	for k, v := range m {
		params = append(params, pipelinev1.Param{
			Name:  k,
			Value: *pipelinev1.NewStructuredValues(v),
		})
	}

	return params
}
