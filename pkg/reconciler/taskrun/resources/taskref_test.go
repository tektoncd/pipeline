/*
 Copyright 2020 The Tekton Authors

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

package resources_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-containerregistry/pkg/registry"
	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/pkg/client/clientset/versioned/fake"
	"github.com/tektoncd/pipeline/pkg/reconciler/taskrun/resources"
	"github.com/tektoncd/pipeline/pkg/trustedresources"
	"github.com/tektoncd/pipeline/test"
	"github.com/tektoncd/pipeline/test/diff"
	"github.com/tektoncd/pipeline/test/parse"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	fakek8s "k8s.io/client-go/kubernetes/fake"
	"knative.dev/pkg/logging"
	logtesting "knative.dev/pkg/logging/testing"
)

var (
	simpleNamespacedTask = &v1beta1.Task{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "simple",
			Namespace: "default",
		},
		TypeMeta: metav1.TypeMeta{
			APIVersion: "tekton.dev/v1beta1",
			Kind:       "Task",
		},
		Spec: v1beta1.TaskSpec{
			Steps: []v1beta1.Step{{
				Image: "something",
			}},
		},
	}
	simpleClusterTask = &v1beta1.ClusterTask{
		ObjectMeta: metav1.ObjectMeta{
			Name: "simple",
		},
		TypeMeta: metav1.TypeMeta{
			APIVersion: "tekton.dev/v1beta1",
			Kind:       "ClusterTask",
		},
		Spec: v1beta1.TaskSpec{
			Steps: []v1beta1.Step{{
				Image: "something",
			}},
		},
	}
	sampleRefSource = &v1beta1.RefSource{
		URI: "abc.com",
		Digest: map[string]string{
			"sha1": "a123",
		},
		EntryPoint: "foo/bar",
	}
)

func TestGetTaskKind(t *testing.T) {
	testCases := []struct {
		name     string
		tr       *v1beta1.TaskRun
		expected v1beta1.TaskKind
	}{
		{
			name: "no task ref",
			tr: &v1beta1.TaskRun{
				Spec: v1beta1.TaskRunSpec{
					TaskSpec: &v1beta1.TaskSpec{},
				},
			},
			expected: v1beta1.NamespacedTaskKind,
		}, {
			name: "no kind on task ref",
			tr: &v1beta1.TaskRun{
				Spec: v1beta1.TaskRunSpec{
					TaskRef: &v1beta1.TaskRef{Name: "whatever"},
				},
			},
			expected: v1beta1.NamespacedTaskKind,
		}, {
			name: "kind on task ref",
			tr: &v1beta1.TaskRun{
				Spec: v1beta1.TaskRunSpec{
					TaskRef: &v1beta1.TaskRef{
						Name: "whatever",
						Kind: "something-else",
					},
				},
			},
			expected: v1beta1.TaskKind("something-else"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tk := resources.GetTaskKind(tc.tr)
			if tk != tc.expected {
				t.Errorf("expected kind %s, but got %s", tc.expected, tk)
			}
		})
	}
}

func TestLocalTaskRef(t *testing.T) {
	testcases := []struct {
		name     string
		tasks    []runtime.Object
		ref      *v1beta1.TaskRef
		expected runtime.Object
		wantErr  bool
	}{
		{
			name: "local-task",
			tasks: []runtime.Object{
				&v1beta1.Task{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "simple",
						Namespace: "default",
					},
				},
				&v1beta1.Task{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "dummy",
						Namespace: "default",
					},
				},
			},
			ref: &v1beta1.TaskRef{
				Name: "simple",
			},
			expected: &v1beta1.Task{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "simple",
					Namespace: "default",
				},
			},
			wantErr: false,
		},
		{
			name: "local-clustertask",
			tasks: []runtime.Object{
				&v1beta1.ClusterTask{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cluster-task",
					},
				},
				&v1beta1.ClusterTask{
					ObjectMeta: metav1.ObjectMeta{
						Name: "dummy-task",
					},
				},
			},
			ref: &v1beta1.TaskRef{
				Name: "cluster-task",
				Kind: "ClusterTask",
			},
			expected: &v1beta1.Task{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "tekton.dev/v1beta1",
					Kind:       "Task",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster-task",
				},
			},
			wantErr: false,
		},
		{
			name:  "task-not-found",
			tasks: []runtime.Object{},
			ref: &v1beta1.TaskRef{
				Name: "simple",
			},
			expected: nil,
			wantErr:  true,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			ctx, cancel := context.WithCancel(ctx)
			defer cancel()
			tektonclient := fake.NewSimpleClientset(tc.tasks...)

			lc := &resources.LocalTaskRefResolver{
				Namespace:    "default",
				Kind:         tc.ref.Kind,
				Tektonclient: tektonclient,
			}

			task, refSource, err := lc.GetTask(ctx, tc.ref.Name)
			if tc.wantErr && err == nil {
				t.Fatal("Expected error but found nil instead")
			} else if !tc.wantErr && err != nil {
				t.Fatalf("Received unexpected error ( %#v )", err)
			}

			if d := cmp.Diff(task, tc.expected); tc.expected != nil && d != "" {
				t.Error(diff.PrintWantGot(d))
			}

			// local cluster tasks have empty RefSource for now. This may be changed in future.
			if refSource != nil {
				t.Errorf("expected refSource is nil, but got %v", refSource)
			}
		})
	}
}

func TestGetTaskFunc(t *testing.T) {
	// Set up a fake registry to push an image to.
	s := httptest.NewServer(registry.New())
	defer s.Close()
	u, err := url.Parse(s.URL)
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	cfg := config.NewStore(logtesting.TestLogger(t))
	cfg.OnConfigChanged(&corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: config.GetFeatureFlagsConfigName()},
		Data: map[string]string{
			"enable-tekton-oci-bundles": "true",
		},
	})
	ctx = cfg.ToContext(ctx)

	testcases := []struct {
		name         string
		localTasks   []runtime.Object
		remoteTasks  []runtime.Object
		ref          *v1beta1.TaskRef
		expected     runtime.Object
		expectedKind v1beta1.TaskKind
	}{
		{
			name:       "remote-task",
			localTasks: []runtime.Object{simpleNamespacedTask},
			remoteTasks: []runtime.Object{
				&v1beta1.Task{
					ObjectMeta: metav1.ObjectMeta{
						Name: "simple",
					},
					TypeMeta: metav1.TypeMeta{
						APIVersion: "tekton.dev/v1beta1",
						Kind:       "Task",
					},
				},
				&v1beta1.Task{
					ObjectMeta: metav1.ObjectMeta{
						Name: "dummy",
					},
					TypeMeta: metav1.TypeMeta{
						APIVersion: "tekton.dev/v1beta1",
						Kind:       "Task",
					},
				},
			},
			ref: &v1beta1.TaskRef{
				Name:   "simple",
				Bundle: u.Host + "/remote-task",
			},
			expected: &v1beta1.Task{
				ObjectMeta: metav1.ObjectMeta{
					Name: "simple",
				},
				TypeMeta: metav1.TypeMeta{
					APIVersion: "tekton.dev/v1beta1",
					Kind:       "Task",
				},
			},
			expectedKind: v1beta1.NamespacedTaskKind,
		}, {
			name:       "remote-task-without-defaults",
			localTasks: []runtime.Object{},
			remoteTasks: []runtime.Object{
				&v1beta1.Task{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "simple",
						Namespace: "default",
					},
					TypeMeta: metav1.TypeMeta{
						APIVersion: "tekton.dev/v1beta1",
						Kind:       "Task",
					},
					Spec: v1beta1.TaskSpec{
						Steps: []v1beta1.Step{{
							Image: "something",
						}},
						Params: []v1beta1.ParamSpec{{
							Name: "foo",
						},
						},
					},
				}},
			ref: &v1beta1.TaskRef{
				Name:   "simple",
				Bundle: u.Host + "/remote-task-without-defaults",
			},
			expected: &v1beta1.Task{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "simple",
					Namespace: "default",
				},
				TypeMeta: metav1.TypeMeta{
					APIVersion: "tekton.dev/v1beta1",
					Kind:       "Task",
				},
				Spec: v1beta1.TaskSpec{
					Steps: []v1beta1.Step{{
						Image: "something",
					}},
					Params: []v1beta1.ParamSpec{{
						Name: "foo",
					}},
				},
			},
			expectedKind: v1beta1.NamespacedTaskKind,
		}, {
			name:       "local-task",
			localTasks: []runtime.Object{simpleNamespacedTask},
			remoteTasks: []runtime.Object{
				&v1beta1.Task{
					ObjectMeta: metav1.ObjectMeta{
						Name: "simple",
					},
					TypeMeta: metav1.TypeMeta{
						APIVersion: "tekton.dev/v1beta1",
						Kind:       "Task",
					},
				},
				&v1beta1.Task{
					ObjectMeta: metav1.ObjectMeta{
						Name: "dummy",
					},
					TypeMeta: metav1.TypeMeta{
						APIVersion: "tekton.dev/v1beta1",
						Kind:       "Task",
					},
				},
			},
			ref: &v1beta1.TaskRef{
				Name: "simple",
			},
			expected:     simpleNamespacedTask,
			expectedKind: v1beta1.NamespacedTaskKind,
		}, {
			name: "remote-cluster-task",
			localTasks: []runtime.Object{
				&v1beta1.ClusterTask{
					TypeMeta:   metav1.TypeMeta{APIVersion: "v1beta1", Kind: "ClusterTask"},
					ObjectMeta: metav1.ObjectMeta{Name: "simple"},
					Spec:       v1beta1.TaskSpec{Params: []v1beta1.ParamSpec{{Name: "foo"}}},
				},
			},
			remoteTasks: []runtime.Object{
				&v1beta1.ClusterTask{
					TypeMeta:   metav1.TypeMeta{APIVersion: "tekton.dev/v1beta1", Kind: "ClusterTask"},
					ObjectMeta: metav1.ObjectMeta{Name: "simple"},
				},
				&v1beta1.ClusterTask{
					TypeMeta:   metav1.TypeMeta{APIVersion: "tekton.dev/v1beta1", Kind: "ClusterTask"},
					ObjectMeta: metav1.ObjectMeta{Name: "dummy"},
				},
			},
			ref: &v1beta1.TaskRef{
				Name:       "simple",
				APIVersion: "tekton.dev/v1beta1",
				Kind:       v1beta1.ClusterTaskKind,
				Bundle:     u.Host + "/remote-cluster-task",
			},
			expected: &v1beta1.Task{
				ObjectMeta: metav1.ObjectMeta{Name: "simple"},
				TypeMeta: metav1.TypeMeta{
					APIVersion: "tekton.dev/v1beta1",
					Kind:       "Task",
				},
			},
			expectedKind: v1beta1.ClusterTaskKind,
		}, {
			name:       "local-cluster-task",
			localTasks: []runtime.Object{simpleClusterTask},
			remoteTasks: []runtime.Object{
				&v1beta1.ClusterTask{
					TypeMeta:   metav1.TypeMeta{APIVersion: "tekton.dev/v1alpha1", Kind: "ClusterTask"},
					ObjectMeta: metav1.ObjectMeta{Name: "simple"},
				},
				&v1beta1.ClusterTask{
					TypeMeta:   metav1.TypeMeta{APIVersion: "tekton.dev/v1alpha1", Kind: "ClusterTask"},
					ObjectMeta: metav1.ObjectMeta{Name: "dummy"},
				},
			},
			ref: &v1beta1.TaskRef{
				Name: "simple",
				Kind: v1beta1.ClusterTaskKind,
			},
			expected: &v1beta1.Task{
				ObjectMeta: metav1.ObjectMeta{
					Name: "simple",
				},
				TypeMeta: metav1.TypeMeta{
					APIVersion: "tekton.dev/v1beta1",
					Kind:       "Task",
				},
				Spec: v1beta1.TaskSpec{
					Steps: []v1beta1.Step{{
						Image: "something",
					}},
				},
			},
			expectedKind: v1beta1.NamespacedTaskKind,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			tektonclient := fake.NewSimpleClientset(tc.localTasks...)
			kubeclient := fakek8s.NewSimpleClientset(&v1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "default",
					Name:      "default",
				},
			})

			_, err := test.CreateImage(u.Host+"/"+tc.name, tc.remoteTasks...)
			if err != nil {
				t.Fatalf("failed to upload test image: %s", err.Error())
			}

			trForFunc := &v1beta1.TaskRun{
				ObjectMeta: metav1.ObjectMeta{Name: "some-tr"},
				Spec: v1beta1.TaskRunSpec{
					TaskRef: tc.ref,
				},
			}
			fn := resources.GetTaskFunc(ctx, kubeclient, tektonclient, nil, trForFunc, tc.ref, "", "default", "default", nil /*VerificationPolicies*/)

			task, refSource, err := fn(ctx, tc.ref.Name)
			if err != nil {
				t.Fatalf("failed to call taskfn: %s", err.Error())
			}

			if diff := cmp.Diff(task, tc.expected); tc.expected != nil && diff != "" {
				t.Error(diff)
			}

			// local cluster task has empty RefSource for now. This may be changed in future.
			if refSource != nil {
				t.Errorf("expected refSource is nil, but got %v", refSource)
			}
		})
	}
}

func TestGetTaskFuncFromTaskRunSpecAlreadyFetched(t *testing.T) {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	tektonclient := fake.NewSimpleClientset(simpleNamespacedTask)
	kubeclient := fakek8s.NewSimpleClientset(&v1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "default",
		},
	})

	name := "anyname-really"
	TaskSpec := v1beta1.TaskSpec{
		Steps: []v1beta1.Step{{
			Image: "myimage",
			Script: `
#!/usr/bin/env bash
echo hello
`,
		}},
	}

	TaskRun := &v1beta1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default"},
		Spec: v1beta1.TaskRunSpec{
			TaskRef: &v1beta1.TaskRef{
				// Using simple here to show that, it won't fetch the simple Taskspec,
				// which is different from the TaskSpec above
				Name: "simple",
			},
			ServiceAccountName: "default",
		},
		Status: v1beta1.TaskRunStatus{TaskRunStatusFields: v1beta1.TaskRunStatusFields{
			TaskSpec: &TaskSpec,
			Provenance: &v1beta1.Provenance{
				RefSource: sampleRefSource.DeepCopy(),
			},
		}},
	}
	expectedTask := &v1beta1.Task{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
		},
		Spec: TaskSpec,
	}

	fn := resources.GetTaskFuncFromTaskRun(ctx, kubeclient, tektonclient, nil, TaskRun, []*v1alpha1.VerificationPolicy{})

	actualTask, actualRefSource, err := fn(ctx, name)
	if err != nil {
		t.Fatalf("failed to call Taskfn: %s", err.Error())
	}

	if diff := cmp.Diff(actualTask, expectedTask); expectedTask != nil && diff != "" {
		t.Error(diff)
	}

	if d := cmp.Diff(sampleRefSource, actualRefSource); d != "" {
		t.Errorf("refSources did not match: %s", diff.PrintWantGot(d))
	}
}

func TestGetTaskFunc_RemoteResolution(t *testing.T) {
	ctx := context.Background()
	cfg := config.FromContextOrDefaults(ctx)
	ctx = config.ToContext(ctx, cfg)
	task := parse.MustParseV1beta1Task(t, taskYAMLString)
	taskRef := &v1beta1.TaskRef{ResolverRef: v1beta1.ResolverRef{Resolver: "git"}}

	testcases := []struct {
		name     string
		taskYAML string
	}{{
		name: "v1beta1 task",
		taskYAML: strings.Join([]string{
			"kind: Task",
			"apiVersion: tekton.dev/v1beta1",
			taskYAMLString,
		}, "\n"),
	}, {
		name: "v1 task",
		taskYAML: strings.Join([]string{
			"kind: Task",
			"apiVersion: tekton.dev/v1",
			taskYAMLString,
		}, "\n"),
	}}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			resolved := test.NewResolvedResource([]byte(tc.taskYAML), nil /* annotations */, sampleRefSource.DeepCopy(), nil /* data error */)
			requester := test.NewRequester(resolved, nil)
			tr := &v1beta1.TaskRun{
				ObjectMeta: metav1.ObjectMeta{Namespace: "default"},
				Spec: v1beta1.TaskRunSpec{
					TaskRef:            taskRef,
					ServiceAccountName: "default",
				},
			}
			fn := resources.GetTaskFunc(ctx, nil, nil, requester, tr, tr.Spec.TaskRef, "", "default", "default", nil /*VerificationPolicies*/)

			resolvedTask, resolvedRefSource, err := fn(ctx, taskRef.Name)
			if err != nil {
				t.Fatalf("failed to call pipelinefn: %s", err.Error())
			}

			if d := cmp.Diff(sampleRefSource, resolvedRefSource); d != "" {
				t.Errorf("refSources did not match: %s", diff.PrintWantGot(d))
			}

			if d := cmp.Diff(task, resolvedTask); d != "" {
				t.Errorf("resolvedTask did not match: %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestGetTaskFunc_RemoteResolution_ReplacedParams(t *testing.T) {
	ctx := context.Background()
	cfg := config.FromContextOrDefaults(ctx)
	ctx = config.ToContext(ctx, cfg)
	task := parse.MustParseV1beta1Task(t, taskYAMLString)
	taskRef := &v1beta1.TaskRef{
		ResolverRef: v1beta1.ResolverRef{
			Resolver: "git",
			Params: v1beta1.Params{{
				Name:  "foo",
				Value: *v1beta1.NewStructuredValues("$(params.resolver-param)"),
			}, {
				Name:  "bar",
				Value: *v1beta1.NewStructuredValues("$(context.taskRun.name)"),
			}},
		},
	}
	taskYAML := strings.Join([]string{
		"kind: Task",
		"apiVersion: tekton.dev/v1beta1",
		taskYAMLString,
	}, "\n")

	resolved := test.NewResolvedResource([]byte(taskYAML), nil, sampleRefSource.DeepCopy(), nil)
	requester := &test.Requester{
		ResolvedResource: resolved,
		Params: v1beta1.Params{{
			Name:  "foo",
			Value: *v1beta1.NewStructuredValues("bar"),
		}, {
			Name:  "bar",
			Value: *v1beta1.NewStructuredValues("test-task"),
		}},
	}
	tr := &v1beta1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-task",
			Namespace: "default",
		},
		Spec: v1beta1.TaskRunSpec{
			TaskRef:            taskRef,
			ServiceAccountName: "default",
			Params: v1beta1.Params{{
				Name:  "resolver-param",
				Value: *v1beta1.NewStructuredValues("bar"),
			}},
		},
	}
	fn := resources.GetTaskFunc(ctx, nil, nil, requester, tr, tr.Spec.TaskRef, "", "default", "default", nil /*VerificationPolicies*/)

	resolvedTask, resolvedRefSource, err := fn(ctx, taskRef.Name)
	if err != nil {
		t.Fatalf("failed to call pipelinefn: %s", err.Error())
	}

	if d := cmp.Diff(task, resolvedTask); d != "" {
		t.Errorf("resolvedTask did not match: %s", diff.PrintWantGot(d))
	}

	if d := cmp.Diff(sampleRefSource, resolvedRefSource); d != "" {
		t.Errorf("refSources did not match: %s", diff.PrintWantGot(d))
	}

	taskRefNotMatching := &v1beta1.TaskRef{
		ResolverRef: v1beta1.ResolverRef{
			Resolver: "git",
			Params: v1beta1.Params{{
				Name:  "foo",
				Value: *v1beta1.NewStructuredValues("$(params.resolver-param)"),
			}, {
				Name:  "bar",
				Value: *v1beta1.NewStructuredValues("$(context.taskRun.name)"),
			}},
		},
	}

	trNotMatching := &v1beta1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "other-task",
			Namespace: "default",
		},
		Spec: v1beta1.TaskRunSpec{
			TaskRef:            taskRefNotMatching,
			ServiceAccountName: "default",
			Params: v1beta1.Params{{
				Name:  "resolver-param",
				Value: *v1beta1.NewStructuredValues("banana"),
			}},
		},
	}
	fnNotMatching := resources.GetTaskFunc(ctx, nil, nil, requester, trNotMatching, trNotMatching.Spec.TaskRef, "", "default", "default", nil /*VerificationPolicies*/)

	_, _, err = fnNotMatching(ctx, taskRefNotMatching.Name)
	if err == nil {
		t.Fatal("expected error for non-matching params, did not get one")
	}
	if !strings.Contains(err.Error(), `StringVal: "banana"`) {
		t.Fatalf("did not receive expected error, got '%s'", err.Error())
	}
}

func TestGetPipelineFunc_RemoteResolutionInvalidData(t *testing.T) {
	ctx := context.Background()
	cfg := config.FromContextOrDefaults(ctx)
	ctx = config.ToContext(ctx, cfg)
	taskRef := &v1beta1.TaskRef{ResolverRef: v1beta1.ResolverRef{Resolver: "git"}}
	resolvesTo := []byte("INVALID YAML")
	resource := test.NewResolvedResource(resolvesTo, nil, nil, nil)
	requester := test.NewRequester(resource, nil)
	tr := &v1beta1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default"},
		Spec: v1beta1.TaskRunSpec{
			TaskRef:            taskRef,
			ServiceAccountName: "default",
		},
	}
	fn := resources.GetTaskFunc(ctx, nil, nil, requester, tr, tr.Spec.TaskRef, "", "default", "default", nil /*VerificationPolicies*/)
	if _, _, err := fn(ctx, taskRef.Name); err == nil {
		t.Fatalf("expected error due to invalid pipeline data but saw none")
	}
}

func TestGetTaskFunc_VerifySuccess(t *testing.T) {
	// This test case tests the success cases of trusted-resources-verification-no-match-policy when it is set to
	// fail: passed matching policy verification
	// warn and ignore: no matching policies.
	ctx := context.Background()
	signer, _, k8sclient, vps := test.SetupVerificationPolicies(t)
	tektonclient := fake.NewSimpleClientset()

	unsignedTask := test.GetUnsignedTask("test-task")
	unsignedTaskBytes, err := json.Marshal(unsignedTask)
	if err != nil {
		t.Fatal("fail to marshal task", err)
	}
	noMatchPolicyRefSource := &v1beta1.RefSource{
		URI: "abc.com",
		Digest: map[string]string{
			"sha1": "a123",
		},
		EntryPoint: "foo/bar",
	}
	resolvedUnmatched := test.NewResolvedResource(unsignedTaskBytes, nil, noMatchPolicyRefSource, nil)
	requesterUnmatched := test.NewRequester(resolvedUnmatched, nil)

	signedTask, err := test.GetSignedTask(unsignedTask, signer, "signed")
	if err != nil {
		t.Fatal("fail to sign task", err)
	}
	signedTaskBytes, err := json.Marshal(signedTask)
	if err != nil {
		t.Fatal("fail to marshal task", err)
	}
	matchPolicyRefSource := &v1beta1.RefSource{
		URI: "	https://github.com/tektoncd/catalog.git",
		Digest: map[string]string{
			"sha1": "a123",
		},
		EntryPoint: "foo/bar",
	}
	resolvedMatched := test.NewResolvedResource(signedTaskBytes, nil, matchPolicyRefSource, nil)
	requesterMatched := test.NewRequester(resolvedMatched, nil)

	taskRef := &v1beta1.TaskRef{ResolverRef: v1beta1.ResolverRef{Resolver: "git"}}

	testcases := []struct {
		name                      string
		requester                 *test.Requester
		verificationNoMatchPolicy string
		policies                  []*v1alpha1.VerificationPolicy
		expected                  runtime.Object
		expectedRefSource         *v1beta1.RefSource
	}{{
		name:                      "signed task with matching policy pass verification with enforce no match policy",
		requester:                 requesterMatched,
		verificationNoMatchPolicy: config.FailNoMatchPolicy,
		policies:                  vps,
		expected:                  signedTask,
		expectedRefSource:         matchPolicyRefSource,
	}, {
		name:                      "signed task with matching policy pass verification with warn no match policy",
		requester:                 requesterMatched,
		verificationNoMatchPolicy: config.WarnNoMatchPolicy,
		policies:                  vps,
		expected:                  signedTask,
		expectedRefSource:         matchPolicyRefSource,
	}, {
		name:                      "signed task with matching policy pass verification with ignore no match policy",
		requester:                 requesterMatched,
		verificationNoMatchPolicy: config.IgnoreNoMatchPolicy,
		policies:                  vps,
		expected:                  signedTask,
		expectedRefSource:         matchPolicyRefSource,
	}, {
		name:                      "warn unsigned task without matching policies",
		requester:                 requesterUnmatched,
		verificationNoMatchPolicy: config.WarnNoMatchPolicy,
		policies:                  vps,
		expected:                  unsignedTask,
		expectedRefSource:         noMatchPolicyRefSource,
	}, {
		name:                      "allow unsigned task without matching policies",
		requester:                 requesterUnmatched,
		verificationNoMatchPolicy: config.IgnoreNoMatchPolicy,
		policies:                  vps,
		expected:                  unsignedTask,
		expectedRefSource:         noMatchPolicyRefSource,
	}, {
		name:                      "warn no policies",
		requester:                 requesterUnmatched,
		verificationNoMatchPolicy: config.WarnNoMatchPolicy,
		policies:                  []*v1alpha1.VerificationPolicy{},
		expected:                  unsignedTask,
		expectedRefSource:         noMatchPolicyRefSource,
	}, {
		name:                      "allow no policies",
		requester:                 requesterUnmatched,
		verificationNoMatchPolicy: config.IgnoreNoMatchPolicy,
		policies:                  []*v1alpha1.VerificationPolicy{},
		expected:                  unsignedTask,
		expectedRefSource:         noMatchPolicyRefSource,
	},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			ctx = test.SetupTrustedResourceConfig(ctx, tc.verificationNoMatchPolicy)
			tr := &v1beta1.TaskRun{
				ObjectMeta: metav1.ObjectMeta{Namespace: "trusted-resources"},
				Spec: v1beta1.TaskRunSpec{
					TaskRef:            taskRef,
					ServiceAccountName: "default",
				},
			}
			fn := resources.GetTaskFunc(ctx, k8sclient, tektonclient, tc.requester, tr, tr.Spec.TaskRef, "", "default", "default", tc.policies)

			resolvedTask, refSource, err := fn(ctx, taskRef.Name)

			if err != nil {
				t.Fatalf("Received unexpected error ( %#v )", err)
			}

			if d := cmp.Diff(tc.expected, resolvedTask); d != "" {
				t.Errorf("resolvedTask did not match: %s", diff.PrintWantGot(d))
			}

			if d := cmp.Diff(tc.expectedRefSource, refSource); d != "" {
				t.Errorf("configSources did not match: %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestGetTaskFunc_VerifyError(t *testing.T) {
	ctx := context.Background()
	signer, _, k8sclient, vps := test.SetupVerificationPolicies(t)
	tektonclient := fake.NewSimpleClientset()

	unsignedTask := test.GetUnsignedTask("test-task")
	unsignedTaskBytes, err := json.Marshal(unsignedTask)
	if err != nil {
		t.Fatal("fail to marshal task", err)
	}
	matchPolicyRefSource := &v1beta1.RefSource{
		URI: "https://github.com/tektoncd/catalog.git",
		Digest: map[string]string{
			"sha1": "a123",
		},
		EntryPoint: "foo/bar",
	}

	resolvedUnsigned := test.NewResolvedResource(unsignedTaskBytes, nil, matchPolicyRefSource, nil)
	requesterUnsigned := test.NewRequester(resolvedUnsigned, nil)

	signedTask, err := test.GetSignedTask(unsignedTask, signer, "signed")
	if err != nil {
		t.Fatal("fail to sign task", err)
	}
	signedTaskBytes, err := json.Marshal(signedTask)
	if err != nil {
		t.Fatal("fail to marshal task", err)
	}

	noMatchPolicyRefSource := &v1beta1.RefSource{
		URI: "abc.com",
		Digest: map[string]string{
			"sha1": "a123",
		},
		EntryPoint: "foo/bar",
	}
	resolvedUnmatched := test.NewResolvedResource(signedTaskBytes, nil, noMatchPolicyRefSource, nil)
	requesterUnmatched := test.NewRequester(resolvedUnmatched, nil)

	modifiedTask := signedTask.DeepCopy()
	modifiedTask.Annotations["random"] = "attack"
	modifiedTaskBytes, err := json.Marshal(modifiedTask)
	if err != nil {
		t.Fatal("fail to marshal task", err)
	}
	resolvedModified := test.NewResolvedResource(modifiedTaskBytes, nil, matchPolicyRefSource, nil)
	requesterModified := test.NewRequester(resolvedModified, nil)

	taskRef := &v1beta1.TaskRef{ResolverRef: v1beta1.ResolverRef{Resolver: "git"}}

	testcases := []struct {
		name                      string
		requester                 *test.Requester
		verificationNoMatchPolicy string
		expected                  *v1beta1.Task
		expectedErr               error
	}{{
		name:                      "unsigned task with fails verification with fail no match policy",
		requester:                 requesterUnsigned,
		verificationNoMatchPolicy: config.FailNoMatchPolicy,
		expected:                  nil,
		expectedErr:               trustedresources.ErrResourceVerificationFailed,
	}, {
		name:                      "unsigned task with fails verification with warn no match policy",
		requester:                 requesterUnsigned,
		verificationNoMatchPolicy: config.WarnNoMatchPolicy,
		expected:                  nil,
		expectedErr:               trustedresources.ErrResourceVerificationFailed,
	}, {
		name:                      "unsigned task with fails verification with ignore no match policy",
		requester:                 requesterUnsigned,
		verificationNoMatchPolicy: config.IgnoreNoMatchPolicy,
		expected:                  nil,
		expectedErr:               trustedresources.ErrResourceVerificationFailed,
	}, {
		name:                      "modified task fails verification with fail no match policy",
		requester:                 requesterModified,
		verificationNoMatchPolicy: config.FailNoMatchPolicy,
		expected:                  nil,
		expectedErr:               trustedresources.ErrResourceVerificationFailed,
	}, {
		name:                      "modified task fails verification with warn no match policy",
		requester:                 requesterModified,
		verificationNoMatchPolicy: config.WarnNoMatchPolicy,
		expected:                  nil,
		expectedErr:               trustedresources.ErrResourceVerificationFailed,
	}, {
		name:                      "modified task fails verification with ignore no match policy",
		requester:                 requesterModified,
		verificationNoMatchPolicy: config.IgnoreNoMatchPolicy,
		expected:                  nil,
		expectedErr:               trustedresources.ErrResourceVerificationFailed,
	}, {
		name:                      "unmatched task fails with fail no match policy",
		requester:                 requesterUnmatched,
		verificationNoMatchPolicy: config.FailNoMatchPolicy,
		expected:                  nil,
		expectedErr:               trustedresources.ErrNoMatchedPolicies,
	},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			ctx = test.SetupTrustedResourceConfig(ctx, tc.verificationNoMatchPolicy)
			tr := &v1beta1.TaskRun{
				ObjectMeta: metav1.ObjectMeta{Namespace: "trusted-resources"},
				Spec: v1beta1.TaskRunSpec{
					TaskRef:            taskRef,
					ServiceAccountName: "default",
				},
			}
			fn := resources.GetTaskFunc(ctx, k8sclient, tektonclient, tc.requester, tr, tr.Spec.TaskRef, "", "default", "default", vps)

			resolvedTask, resolvedRefSource, err := fn(ctx, taskRef.Name)

			if !errors.Is(err, tc.expectedErr) {
				t.Errorf("GetTaskFunc got %v but want %v", err, tc.expectedErr)
			}

			if d := cmp.Diff(tc.expected, resolvedTask); d != "" {
				t.Errorf("resolvedTask did not match: %s", diff.PrintWantGot(d))
			}

			if resolvedRefSource != nil {
				t.Errorf("refSource is: %v but want is nil", resolvedRefSource)
			}
		})
	}
}

func TestGetTaskFunc_GetFuncError(t *testing.T) {
	ctx := context.Background()
	_, k8sclient, vps := test.SetupMatchAllVerificationPolicies(t, "trusted-resources")
	tektonclient := fake.NewSimpleClientset()

	unsignedTask := test.GetUnsignedTask("test-task")
	unsignedTaskBytes, err := json.Marshal(unsignedTask)
	if err != nil {
		t.Fatal("fail to marshal task", err)
	}

	resolvedUnsigned := test.NewResolvedResource(unsignedTaskBytes, nil, sampleRefSource.DeepCopy(), nil)
	requesterUnsigned := test.NewRequester(resolvedUnsigned, nil)
	resolvedUnsigned.DataErr = fmt.Errorf("resolution error")

	trBundleError := &v1beta1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{Namespace: "trusted-resources"},
		Spec: v1beta1.TaskRunSpec{
			TaskRef: &v1beta1.TaskRef{
				Name:   "taskName",
				Bundle: "bundle",
			},
			ServiceAccountName: "default",
		},
	}

	trResolutionError := &v1beta1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{Namespace: "trusted-resources"},
		Spec: v1beta1.TaskRunSpec{
			TaskRef: &v1beta1.TaskRef{
				Name: "taskName",
				ResolverRef: v1beta1.ResolverRef{
					Resolver: "git",
				},
			},
			ServiceAccountName: "default",
		},
	}

	testcases := []struct {
		name        string
		requester   *test.Requester
		taskrun     v1beta1.TaskRun
		expectedErr error
	}{
		{
			name:        "get error when oci bundle return error",
			requester:   requesterUnsigned,
			taskrun:     *trBundleError,
			expectedErr: fmt.Errorf(`failed to get keychain: serviceaccounts "default" not found`),
		},
		{
			name:        "get error when remote resolution return error",
			requester:   requesterUnsigned,
			taskrun:     *trResolutionError,
			expectedErr: fmt.Errorf("error accessing data from remote resource: %w", resolvedUnsigned.DataErr),
		},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			store := config.NewStore(logging.FromContext(ctx).Named("config-store"))
			featureflags := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "tekton-pipelines",
					Name:      "feature-flags",
				},
				Data: map[string]string{
					"enable-tekton-oci-bundles": "true",
				},
			}
			store.OnConfigChanged(featureflags)
			ctx = store.ToContext(ctx)

			fn := resources.GetTaskFunc(ctx, k8sclient, tektonclient, tc.requester, &tc.taskrun, tc.taskrun.Spec.TaskRef, "", "default", "default", vps)

			_, _, err = fn(ctx, tc.taskrun.Spec.TaskRef.Name)

			if d := cmp.Diff(err.Error(), tc.expectedErr.Error()); d != "" {
				t.Fatalf("Expected error %v but found %v instead", tc.expectedErr, err)
			}
		})
	}
}

// This is missing the kind and apiVersion because those are added by
// the MustParse helpers from the test package.
var taskYAMLString = `
metadata:
  name: foo
spec:
  steps:
  - name: step1
    image: ubuntu
    script: |
      echo "hello world!"
`
