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
	"context"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/apis/resolution/v1alpha1"
	ttesting "github.com/tektoncd/pipeline/pkg/reconciler/testing"
	resolutioncommon "github.com/tektoncd/pipeline/pkg/resolution/common"
	"github.com/tektoncd/pipeline/pkg/resolution/resolver/framework"
	frtesting "github.com/tektoncd/pipeline/pkg/resolution/resolver/framework/testing"
	"github.com/tektoncd/pipeline/test"
	"github.com/tektoncd/pipeline/test/diff"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/system"

	_ "knative.dev/pkg/system/testing"
)

func TestGetSelector(t *testing.T) {
	resolver := Resolver{}
	sel := resolver.GetSelector(resolverContext())
	if typ, has := sel[resolutioncommon.LabelKeyResolverType]; !has {
		t.Fatalf("unexpected selector: %v", sel)
	} else if typ != LabelValueGitResolverType {
		t.Fatalf("unexpected type: %q", typ)
	}
}

func TestValidateParams(t *testing.T) {
	resolver := Resolver{}

	paramsWithRevision := map[string]string{
		PathParam:     "bar",
		RevisionParam: "baz",
	}

	if err := resolver.ValidateParams(resolverContext(), paramsWithRevision); err != nil {
		t.Fatalf("unexpected error validating params: %v", err)
	}
}

func TestValidateParamsNotEnabled(t *testing.T) {
	resolver := Resolver{}

	var err error

	someParams := map[string]string{
		PathParam:     "bar",
		RevisionParam: "baz",
	}
	err = resolver.ValidateParams(context.Background(), someParams)
	if err == nil {
		t.Fatalf("expected disabled err")
	}
	if d := cmp.Diff(disabledError, err.Error()); d != "" {
		t.Errorf("unexpected error: %s", diff.PrintWantGot(d))
	}
}

func TestValidateParamsMissing(t *testing.T) {
	resolver := Resolver{}

	var err error

	paramsMissingPath := map[string]string{
		URLParam:      "foo",
		RevisionParam: "baz",
	}
	err = resolver.ValidateParams(resolverContext(), paramsMissingPath)
	if err == nil {
		t.Fatalf("expected missing pathInRepo err")
	}
}

func TestGetResolutionTimeoutDefault(t *testing.T) {
	resolver := Resolver{}
	defaultTimeout := 30 * time.Minute
	timeout := resolver.GetResolutionTimeout(resolverContext(), defaultTimeout)
	if timeout != defaultTimeout {
		t.Fatalf("expected default timeout to be returned")
	}
}

func TestGetResolutionTimeoutCustom(t *testing.T) {
	resolver := Resolver{}
	defaultTimeout := 30 * time.Minute
	configTimeout := 5 * time.Second
	config := map[string]string{
		ConfigFieldTimeout: configTimeout.String(),
	}
	ctx := framework.InjectResolverConfigToContext(resolverContext(), config)
	timeout := resolver.GetResolutionTimeout(ctx, defaultTimeout)
	if timeout != configTimeout {
		t.Fatalf("expected timeout from config to be returned")
	}
}

func TestResolveNotEnabled(t *testing.T) {
	resolver := Resolver{}

	var err error

	someParams := map[string]string{
		PathParam:     "bar",
		RevisionParam: "baz",
	}
	_, err = resolver.Resolve(context.Background(), someParams)
	if err == nil {
		t.Fatalf("expected disabled err")
	}
	if d := cmp.Diff(disabledError, err.Error()); d != "" {
		t.Errorf("unexpected error: %s", diff.PrintWantGot(d))
	}
}

func TestResolve(t *testing.T) {
	withTemporaryGitConfig(t)

	testCases := []struct {
		name            string
		commits         []commitForRepo
		revision        string
		useNthCommit    int
		specificCommit  string
		pathInRepo      string
		expectedContent []byte
		expectedErr     error
	}{
		{
			name: "single commit with default revision",
			commits: []commitForRepo{{
				Dir:      "foo/bar",
				Filename: "somefile",
				Content:  "some content",
			}},
			pathInRepo:      "foo/bar/somefile",
			expectedContent: []byte("some content"),
		}, {
			name: "branch revision",
			commits: []commitForRepo{{
				Dir:      "foo/bar",
				Filename: "somefile",
				Content:  "some content",
				Branch:   "other-revision",
			}, {
				Dir:      "foo/bar",
				Filename: "somefile",
				Content:  "wrong content",
			}},
			revision:        "other-revision",
			pathInRepo:      "foo/bar/somefile",
			expectedContent: []byte("some content"),
		}, {
			name: "commit revision",
			commits: []commitForRepo{{
				Dir:      "foo/bar",
				Filename: "somefile",
				Content:  "some content",
			}, {
				Dir:      "foo/bar",
				Filename: "somefile",
				Content:  "different content",
			}},
			pathInRepo:      "foo/bar/somefile",
			useNthCommit:    1,
			expectedContent: []byte("different content"),
		}, {
			name: "tag revision",
			commits: []commitForRepo{{
				Dir:      "foo/bar",
				Filename: "somefile",
				Content:  "some content",
				Tag:      "tag1",
			}, {
				Dir:      "foo/bar",
				Filename: "somefile",
				Content:  "different content",
			}},
			pathInRepo:      "foo/bar/somefile",
			revision:        "tag1",
			expectedContent: []byte("some content"),
		}, {
			name: "file does not exist",
			commits: []commitForRepo{{
				Dir:      "foo/bar",
				Filename: "somefile",
				Content:  "some content",
			}},
			pathInRepo:  "foo/bar/some other file",
			expectedErr: errors.New(`error opening file "foo/bar/some other file": file does not exist`),
		}, {
			name: "revision does not exist",
			commits: []commitForRepo{{
				Dir:      "foo/bar",
				Filename: "somefile",
				Content:  "some content",
			}},
			revision:    "does-not-exist",
			pathInRepo:  "foo/bar/some other file",
			expectedErr: errors.New("revision error: reference not found"),
		}, {
			name: "commit does not exist",
			commits: []commitForRepo{{
				Dir:      "foo/bar",
				Filename: "somefile",
				Content:  "some content",
			}},
			specificCommit: "does-not-exist",
			pathInRepo:     "foo/bar/some other file",
			expectedErr:    errors.New("revision error: reference not found"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			repoPath, commits := createTestRepo(t, tc.commits)
			resolver := &Resolver{}

			params := map[string]string{
				URLParam:  repoPath,
				PathParam: tc.pathInRepo,
			}

			switch {
			case tc.useNthCommit > 0:
				params[RevisionParam] = commits[plumbing.Master.Short()][tc.useNthCommit]
			case tc.specificCommit != "":
				params[RevisionParam] = hex.EncodeToString([]byte(tc.specificCommit))
			default:
				params[RevisionParam] = tc.revision
			}

			v := map[string]string{
				ConfigRevision: plumbing.Master.Short(),
			}
			output, err := resolver.Resolve(context.WithValue(resolverContext(), struct{}{}, v), params)
			if tc.expectedErr != nil {
				if err == nil {
					t.Fatalf("expected err '%v' but didn't get one", tc.expectedErr)
				}
				if tc.expectedErr.Error() != err.Error() {
					t.Fatalf("expected err '%v' but got '%v'", tc.expectedErr, err)
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error resolving: %v", err)
				}

				expectedResource := &ResolvedGitResource{
					Content: tc.expectedContent,
				}
				switch {
				case tc.useNthCommit > 0:
					expectedResource.Revision = commits[plumbing.Master.Short()][tc.useNthCommit]
				case tc.revision == "":
					expectedResource.Revision = plumbing.Master.Short()
				default:
					expectedResource.Revision = tc.revision
				}

				if d := cmp.Diff(expectedResource, output); d != "" {
					t.Errorf("unexpected resource from Resolve: %s", diff.PrintWantGot(d))
				}
			}
		})
	}
}

func TestController(t *testing.T) {
	withTemporaryGitConfig(t)

	testCases := []struct {
		name           string
		commits        []commitForRepo
		revision       string
		useNthCommit   int
		specificCommit string
		pathInRepo     string
		expectedStatus *v1alpha1.ResolutionRequestStatus
		expectedErr    error
	}{
		{
			name: "single commit",
			commits: []commitForRepo{{
				Dir:      "foo/bar",
				Filename: "somefile",
				Content:  "some content",
			}},
			pathInRepo: "foo/bar/somefile",
			expectedStatus: &v1alpha1.ResolutionRequestStatus{
				Status: duckv1.Status{
					Annotations: map[string]string{
						"content-type": "application/x-yaml",
					},
				},
				ResolutionRequestStatusFields: v1alpha1.ResolutionRequestStatusFields{
					Data: base64.StdEncoding.Strict().EncodeToString([]byte("some content")),
				},
			},
		}, {
			name: "with branch",
			commits: []commitForRepo{{
				Dir:      "foo/bar",
				Filename: "somefile",
				Content:  "some content",
				Branch:   "other-revision",
			}, {
				Dir:      "foo/bar",
				Filename: "somefile",
				Content:  "wrong content",
			}},
			revision:   "other-revision",
			pathInRepo: "foo/bar/somefile",
			expectedStatus: &v1alpha1.ResolutionRequestStatus{
				Status: duckv1.Status{
					Annotations: map[string]string{
						"content-type": "application/x-yaml",
					},
				},
				ResolutionRequestStatusFields: v1alpha1.ResolutionRequestStatusFields{
					Data: base64.StdEncoding.Strict().EncodeToString([]byte("some content")),
				},
			},
		}, {
			name: "with tag",
			commits: []commitForRepo{{
				Dir:      "foo/bar",
				Filename: "somefile",
				Content:  "some content",
				Tag:      "tag1",
			}, {
				Dir:      "foo/bar",
				Filename: "somefile",
				Content:  "wrong content",
			}},
			revision:   "tag1",
			pathInRepo: "foo/bar/somefile",
			expectedStatus: &v1alpha1.ResolutionRequestStatus{
				Status: duckv1.Status{
					Annotations: map[string]string{
						"content-type": "application/x-yaml",
					},
				},
				ResolutionRequestStatusFields: v1alpha1.ResolutionRequestStatusFields{
					Data: base64.StdEncoding.Strict().EncodeToString([]byte("some content")),
				},
			},
		}, {
			name: "earlier specific commit",
			commits: []commitForRepo{{
				Dir:      "foo/bar",
				Filename: "somefile",
				Content:  "some content",
			}, {
				Dir:      "foo/bar",
				Filename: "somefile",
				Content:  "different content",
			}},
			pathInRepo:   "foo/bar/somefile",
			useNthCommit: 1,
			expectedStatus: &v1alpha1.ResolutionRequestStatus{
				Status: duckv1.Status{
					Annotations: map[string]string{
						"content-type": "application/x-yaml",
					},
				},
				ResolutionRequestStatusFields: v1alpha1.ResolutionRequestStatusFields{
					Data: base64.StdEncoding.Strict().EncodeToString([]byte("different content")),
				},
			},
		}, {
			name: "file does not exist",
			commits: []commitForRepo{{
				Dir:      "foo/bar",
				Filename: "somefile",
				Content:  "some content",
			}},
			pathInRepo: "foo/bar/some other file",
			expectedStatus: &v1alpha1.ResolutionRequestStatus{
				Status: duckv1.Status{
					Conditions: duckv1.Conditions{{
						Type:   apis.ConditionSucceeded,
						Status: corev1.ConditionFalse,
						Reason: resolutioncommon.ReasonResolutionFailed,
					}},
				},
			},
			expectedErr: errors.New(`error getting "Git" "foo/rr": error opening file "foo/bar/some other file": file does not exist`),
		}, {
			name: "branch does not exist",
			commits: []commitForRepo{{
				Dir:      "foo/bar",
				Filename: "somefile",
				Content:  "some content",
			}},
			revision:   "does-not-exist",
			pathInRepo: "foo/bar/some other file",
			expectedStatus: &v1alpha1.ResolutionRequestStatus{
				Status: duckv1.Status{
					Conditions: duckv1.Conditions{{
						Type:   apis.ConditionSucceeded,
						Status: corev1.ConditionFalse,
						Reason: resolutioncommon.ReasonResolutionFailed,
					}},
				},
			},
			expectedErr: errors.New(`error getting "Git" "foo/rr": revision error: reference not found`),
		}, {
			name: "commit does not exist",
			commits: []commitForRepo{{
				Dir:      "foo/bar",
				Filename: "somefile",
				Content:  "some content",
			}},
			specificCommit: "does-not-exist",
			pathInRepo:     "foo/bar/some other file",
			expectedStatus: &v1alpha1.ResolutionRequestStatus{
				Status: duckv1.Status{
					Conditions: duckv1.Conditions{{
						Type:   apis.ConditionSucceeded,
						Status: corev1.ConditionFalse,
						Reason: resolutioncommon.ReasonResolutionFailed,
					}},
				},
			},
			expectedErr: errors.New(`error getting "Git" "foo/rr": revision error: reference not found`),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, _ := ttesting.SetupFakeContext(t)

			repoPath, commits := createTestRepo(t, tc.commits)

			request := createRequest(repoPath, tc.pathInRepo, tc.revision, tc.specificCommit, tc.useNthCommit, commits)
			resolver := &Resolver{}

			var expectedStatus *v1alpha1.ResolutionRequestStatus
			if tc.expectedStatus != nil {
				expectedStatus = tc.expectedStatus.DeepCopy()

				if tc.expectedErr == nil {
					reqParams := request.Spec.Parameters
					// Add the expected commit to the expected status annotations, but only if we expect success.
					if cmt, ok := reqParams[RevisionParam]; ok {
						expectedStatus.Annotations[AnnotationKeyRevision] = cmt
					} else {
						expectedStatus.Annotations[AnnotationKeyRevision] = plumbing.Master.Short()
					}
				} else {
					expectedStatus.Status.Conditions[0].Message = tc.expectedErr.Error()
				}
			}
			d := test.Data{
				ConfigMaps: []*corev1.ConfigMap{{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resolver.GetConfigName(ctx),
						Namespace: system.Namespace(),
					},
					Data: map[string]string{
						ConfigFieldTimeout: "1m",
						ConfigRevision:     plumbing.Master.Short(),
					},
				}, {
					ObjectMeta: metav1.ObjectMeta{Namespace: system.Namespace(), Name: config.GetFeatureFlagsConfigName()},
					Data: map[string]string{
						"enable-git-resolver": "true",
					},
				}},
				ResolutionRequests: []*v1alpha1.ResolutionRequest{request},
			}

			frtesting.RunResolverReconcileTest(ctx, t, d, resolver, request, expectedStatus, tc.expectedErr)
		})
	}
}

// createTestRepo is used to instantiate a local test repository with the desired commits.
func createTestRepo(t *testing.T, commits []commitForRepo) (string, map[string][]string) {
	t.Helper()
	tempDir := t.TempDir()

	repo, err := git.PlainInit(tempDir, false)

	worktree, err := repo.Worktree()
	if err != nil {
		t.Fatalf("getting test worktree: %v", err)
	}
	if worktree == nil {
		t.Fatal("test worktree not created")
	}

	startingHash := writeAndCommitToTestRepo(t, worktree, tempDir, "", "README", []byte("This is a test"))

	hashesByBranch := make(map[string][]string)

	// Iterate over the commits and add them.
	for _, cmt := range commits {
		branch := cmt.Branch
		if branch == "" {
			branch = plumbing.Master.Short()
		}

		// If we're given a revision, check out that revision.
		coOpts := &git.CheckoutOptions{
			Branch: plumbing.NewBranchReferenceName(branch),
		}

		if _, ok := hashesByBranch[branch]; !ok && branch != plumbing.Master.Short() {
			coOpts.Hash = plumbing.NewHash(startingHash.String())
			coOpts.Create = true
		}

		if err := worktree.Checkout(coOpts); err != nil {
			t.Fatalf("couldn't do checkout of %s: %v", branch, err)
		}

		hash := writeAndCommitToTestRepo(t, worktree, tempDir, cmt.Dir, cmt.Filename, []byte(cmt.Content))

		if _, ok := hashesByBranch[branch]; !ok {
			hashesByBranch[branch] = []string{hash.String()}
		} else {
			hashesByBranch[branch] = append(hashesByBranch[branch], hash.String())
		}

		if cmt.Tag != "" {
			_, err = repo.CreateTag(cmt.Tag, hash, &git.CreateTagOptions{
				Message: cmt.Tag,
				Tagger: &object.Signature{
					Name:  "Someone",
					Email: "someone@example.com",
					When:  time.Now(),
				},
			})
		}
		if err != nil {
			t.Fatalf("couldn't add tag for %s: %v", cmt.Tag, err)
		}
	}

	return tempDir, hashesByBranch
}

// commitForRepo provides the directory, filename, content and revision for a test commit.
type commitForRepo struct {
	Dir      string
	Filename string
	Content  string
	Branch   string
	Tag      string
}

func writeAndCommitToTestRepo(t *testing.T, worktree *git.Worktree, repoDir string, subPath string, filename string, content []byte) plumbing.Hash {
	t.Helper()

	targetDir := repoDir
	if subPath != "" {
		targetDir = filepath.Join(targetDir, subPath)
		fi, err := os.Stat(targetDir)
		if os.IsNotExist(err) {
			if err := os.MkdirAll(targetDir, 0700); err != nil {
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
	if err := ioutil.WriteFile(outfile, content, 0600); err != nil {
		t.Fatalf("couldn't write content to file %s: %v", outfile, err)
	}

	_, err := worktree.Add(filepath.Join(subPath, filename))
	if err != nil {
		t.Fatalf("couldn't add file %s to git: %v", outfile, err)
	}

	hash, err := worktree.Commit("adding file for test", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Someone",
			Email: "someone@example.com",
			When:  time.Now(),
		},
	})
	if err != nil {
		t.Fatalf("couldn't perform commit for test: %v", err)
	}

	return hash
}

// withTemporaryGitConfig resets the .gitconfig for the duration of the test.
func withTemporaryGitConfig(t *testing.T) {
	gitConfigDir := t.TempDir()
	key := "GIT_CONFIG_GLOBAL"
	t.Setenv(key, filepath.Join(gitConfigDir, "config"))
}

func createRequest(repoURL, pathInRepo, revision, specificCommit string, useNthCommit int, commitsByBranch map[string][]string) *v1alpha1.ResolutionRequest {
	rr := &v1alpha1.ResolutionRequest{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "resolution.tekton.dev/v1alpha1",
			Kind:       "ResolutionRequest",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:              "rr",
			Namespace:         "foo",
			CreationTimestamp: metav1.Time{Time: time.Now()},
			Labels: map[string]string{
				resolutioncommon.LabelKeyResolverType: LabelValueGitResolverType,
			},
		},
		Spec: v1alpha1.ResolutionRequestSpec{
			Parameters: map[string]string{
				URLParam:  repoURL,
				PathParam: pathInRepo,
			},
		},
	}

	switch {
	case useNthCommit > 0:
		rr.Spec.Parameters[RevisionParam] = commitsByBranch[plumbing.Master.Short()][useNthCommit]
	case specificCommit != "":
		rr.Spec.Parameters[RevisionParam] = hex.EncodeToString([]byte(specificCommit))
	case revision != "":
		rr.Spec.Parameters[RevisionParam] = revision
	}

	return rr
}

func resolverContext() context.Context {
	return frtesting.ContextWithGitResolverEnabled(context.Background())
}
