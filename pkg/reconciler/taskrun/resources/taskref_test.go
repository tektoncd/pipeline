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
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/sigstore/sigstore/pkg/signature"
	"github.com/tektoncd/pipeline/pkg/apis/config"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/pkg/client/clientset/versioned/fake"
	"github.com/tektoncd/pipeline/pkg/reconciler/taskrun/resources"
	"github.com/tektoncd/pipeline/pkg/trustedresources"
	"github.com/tektoncd/pipeline/test"
	"github.com/tektoncd/pipeline/test/diff"
	"github.com/tektoncd/pipeline/test/parse"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	fakek8s "k8s.io/client-go/kubernetes/fake"
	"knative.dev/pkg/logging"
)

var (
	simpleNamespacedTask = &v1.Task{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "simple",
			Namespace: "default",
		},
		TypeMeta: metav1.TypeMeta{
			APIVersion: "tekton.dev/v1",
			Kind:       "Task",
		},
		Spec: v1.TaskSpec{
			Steps: []v1.Step{{
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
	sampleRefSource = &v1.RefSource{
		URI: "abc.com",
		Digest: map[string]string{
			"sha1": "a123",
		},
		EntryPoint: "foo/bar",
	}
	unsignedV1Task = v1.Task{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "tekton.dev/v1",
			Kind:       "Task"},
		ObjectMeta: metav1.ObjectMeta{
			Name:        "task",
			Annotations: map[string]string{"foo": "bar"},
		},
		Spec: v1.TaskSpec{
			Steps: []v1.Step{{
				Image: "ubuntu",
				Name:  "echo",
			}},
		},
	}
	verificationResultCmp = cmp.Comparer(func(x, y trustedresources.VerificationResult) bool {
		return x.VerificationResultType == y.VerificationResultType && (errors.Is(x.Err, y.Err) || errors.Is(y.Err, x.Err))
	})
)

func TestGetTaskKind(t *testing.T) {
	testCases := []struct {
		name     string
		tr       *v1.TaskRun
		expected v1.TaskKind
	}{
		{
			name: "no task ref",
			tr: &v1.TaskRun{
				Spec: v1.TaskRunSpec{
					TaskSpec: &v1.TaskSpec{},
				},
			},
			expected: v1.NamespacedTaskKind,
		}, {
			name: "no kind on task ref",
			tr: &v1.TaskRun{
				Spec: v1.TaskRunSpec{
					TaskRef: &v1.TaskRef{Name: "whatever"},
				},
			},
			expected: v1.NamespacedTaskKind,
		}, {
			name: "kind on task ref",
			tr: &v1.TaskRun{
				Spec: v1.TaskRunSpec{
					TaskRef: &v1.TaskRef{
						Name: "whatever",
						Kind: "something-else",
					},
				},
			},
			expected: v1.TaskKind("something-else"),
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
		ref      *v1.TaskRef
		expected runtime.Object
		wantErr  bool
	}{
		{
			name: "local-task",
			tasks: []runtime.Object{
				&v1.Task{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "simple",
						Namespace: "default",
					},
				},
				&v1.Task{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "dummy",
						Namespace: "default",
					},
				},
			},
			ref: &v1.TaskRef{
				Name: "simple",
			},
			expected: &v1.Task{
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
			ref: &v1.TaskRef{
				Name: "cluster-task",
				Kind: "ClusterTask",
			},
			expected: &v1.Task{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "tekton.dev/v1",
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
			ref: &v1.TaskRef{
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

			task, refSource, _, err := lc.GetTask(ctx, tc.ref.Name)
			if tc.wantErr && err == nil {
				t.Fatal("Expected error but found nil instead")
			} else if !tc.wantErr && err != nil {
				t.Fatalf("Received unexpected error ( %#v )", err)
			}

			if d := cmp.Diff(task, tc.expected); tc.expected != nil && d != "" {
				t.Error(diff.PrintWantGot(d))
			}

			// local cluster tasks have empty source for now. This may be changed in future.
			if refSource != nil {
				t.Errorf("expected refsource is nil, but got %v", refSource)
			}
		})
	}
}

func TestGetTaskFunc_Local(t *testing.T) {
	ctx := context.Background()

	testcases := []struct {
		name         string
		localTasks   []runtime.Object
		remoteTasks  []runtime.Object
		ref          *v1.TaskRef
		expected     runtime.Object
		expectedKind v1.TaskKind
	}{
		{
			name:       "local-task",
			localTasks: []runtime.Object{simpleNamespacedTask},
			remoteTasks: []runtime.Object{
				&v1.Task{
					ObjectMeta: metav1.ObjectMeta{
						Name: "simple",
					},
					TypeMeta: metav1.TypeMeta{
						APIVersion: "tekton.dev/v1",
						Kind:       "Task",
					},
				},
				&v1.Task{
					ObjectMeta: metav1.ObjectMeta{
						Name: "dummy",
					},
					TypeMeta: metav1.TypeMeta{
						APIVersion: "tekton.dev/v1",
						Kind:       "Task",
					},
				},
			},
			ref: &v1.TaskRef{
				Name: "simple",
			},
			expected:     simpleNamespacedTask,
			expectedKind: v1.NamespacedTaskKind,
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
			ref: &v1.TaskRef{
				Name: "simple",
				Kind: v1.ClusterTaskRefKind,
			},
			expected: &v1.Task{
				ObjectMeta: metav1.ObjectMeta{
					Name: "simple",
				},
				TypeMeta: metav1.TypeMeta{
					APIVersion: "tekton.dev/v1",
					Kind:       "Task",
				},
				Spec: v1.TaskSpec{
					Steps: []v1.Step{{
						Image: "something",
					}},
				},
			},
			expectedKind: v1.NamespacedTaskKind,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			tektonclient := fake.NewSimpleClientset(tc.localTasks...)
			kubeclient := fakek8s.NewSimpleClientset(&corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "default",
					Name:      "default",
				},
			})

			trForFunc := &v1.TaskRun{
				ObjectMeta: metav1.ObjectMeta{Name: "some-tr"},
				Spec: v1.TaskRunSpec{
					TaskRef: tc.ref,
				},
			}
			fn := resources.GetTaskFunc(ctx, kubeclient, tektonclient, nil, trForFunc, tc.ref, "", "default", "default", nil /*VerificationPolicies*/)

			task, refSource, _, err := fn(ctx, tc.ref.Name)
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
	kubeclient := fakek8s.NewSimpleClientset(&corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "default",
		},
	})

	name := "anyname-really"
	TaskSpec := v1.TaskSpec{
		Steps: []v1.Step{{
			Image: "myimage",
			Script: `
#!/usr/bin/env bash
echo hello
`,
		}},
	}

	TaskRun := &v1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default"},
		Spec: v1.TaskRunSpec{
			TaskRef: &v1.TaskRef{
				// Using simple here to show that, it won't fetch the simple Taskspec,
				// which is different from the TaskSpec above
				Name: "simple",
			},
			ServiceAccountName: "default",
		},
		Status: v1.TaskRunStatus{TaskRunStatusFields: v1.TaskRunStatusFields{
			TaskSpec: &TaskSpec,
			Provenance: &v1.Provenance{
				RefSource: sampleRefSource.DeepCopy(),
			},
		}},
	}
	expectedTask := &v1.Task{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
		},
		Spec: TaskSpec,
	}

	fn := resources.GetTaskFuncFromTaskRun(ctx, kubeclient, tektonclient, nil, TaskRun, []*v1alpha1.VerificationPolicy{})

	actualTask, actualRefSource, _, err := fn(ctx, name)
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
	ctx := config.EnableStableAPIFields(context.Background())
	cfg := config.FromContextOrDefaults(ctx)
	ctx = config.ToContext(ctx, cfg)
	taskRef := &v1.TaskRef{ResolverRef: v1.ResolverRef{Resolver: "git"}}

	testcases := []struct {
		name     string
		taskYAML string
		wantTask *v1.Task
		wantErr  bool
	}{{
		name: "v1beta1 task",
		taskYAML: strings.Join([]string{
			"kind: Task",
			"apiVersion: tekton.dev/v1beta1",
			taskYAMLString,
		}, "\n"),
		wantTask: parse.MustParseV1Task(t, taskYAMLString),
	}, {
		name: "v1beta1 task with beta features",
		taskYAML: strings.Join([]string{
			"kind: Task",
			"apiVersion: tekton.dev/v1beta1",
			taskYAMLStringWithBetaFeatures,
		}, "\n"),
		wantTask: parse.MustParseV1Task(t, taskYAMLStringWithBetaFeatures),
	}, {
		name: "v1beta1 cluster task",
		taskYAML: strings.Join([]string{
			"kind: ClusterTask",
			"apiVersion: tekton.dev/v1beta1",
			taskYAMLString,
		}, "\n"),
		wantTask: parse.MustParseV1Task(t, taskYAMLString),
	}, {
		name: "v1 task",
		taskYAML: strings.Join([]string{
			"kind: Task",
			"apiVersion: tekton.dev/v1",
			taskYAMLString,
		}, "\n"),
		wantTask: parse.MustParseV1Task(t, taskYAMLString),
	}, {
		name: "v1 task with beta features",
		taskYAML: strings.Join([]string{
			"kind: Task",
			"apiVersion: tekton.dev/v1",
			taskYAMLStringWithBetaFeatures,
		}, "\n"),
		wantErr: true,
	}, {
		name: "v1 task without defaults",
		taskYAML: strings.Join([]string{
			"kind: Task",
			"apiVersion: tekton.dev/v1",
			remoteTaskYamlWithoutDefaults,
		}, "\n"),
		wantTask: parse.MustParseV1Task(t, remoteTaskYamlWithoutDefaults),
	}}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			resolved := test.NewResolvedResource([]byte(tc.taskYAML), nil /* annotations */, sampleRefSource.DeepCopy(), nil /* data error */)
			requester := test.NewRequester(resolved, nil)
			tr := &v1.TaskRun{
				ObjectMeta: metav1.ObjectMeta{Namespace: "default"},
				Spec: v1.TaskRunSpec{
					TaskRef:            taskRef,
					ServiceAccountName: "default",
				},
			}
			fn := resources.GetTaskFunc(ctx, nil, nil, requester, tr, tr.Spec.TaskRef, "", "default", "default", nil /*VerificationPolicies*/)

			resolvedTask, resolvedRefSource, _, err := fn(ctx, taskRef.Name)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected an error when calling taskfunc but got none")
				}
			} else {
				if err != nil {
					t.Fatalf("failed to call taskfn: %s", err.Error())
				}

				if d := cmp.Diff(sampleRefSource, resolvedRefSource); d != "" {
					t.Errorf("refSources did not match: %s", diff.PrintWantGot(d))
				}

				if d := cmp.Diff(tc.wantTask, resolvedTask); d != "" {
					t.Errorf("resolvedTask did not match: %s", diff.PrintWantGot(d))
				}
			}
		})
	}
}

func TestGetTaskFunc_RemoteResolution_ReplacedParams(t *testing.T) {
	ctx := context.Background()
	cfg := config.FromContextOrDefaults(ctx)
	ctx = config.ToContext(ctx, cfg)
	task := parse.MustParseV1Task(t, taskYAMLString)
	taskRef := &v1.TaskRef{
		ResolverRef: v1.ResolverRef{
			Resolver: "git",
			Params: []v1.Param{{
				Name:  "foo",
				Value: *v1.NewStructuredValues("$(params.resolver-param)"),
			}, {
				Name:  "bar",
				Value: *v1.NewStructuredValues("$(context.taskRun.name)"),
			}},
		},
	}
	taskYAML := strings.Join([]string{
		"kind: Task",
		"apiVersion: tekton.dev/v1",
		taskYAMLString,
	}, "\n")

	resolved := test.NewResolvedResource([]byte(taskYAML), nil, sampleRefSource.DeepCopy(), nil)
	requester := &test.Requester{
		ResolvedResource: resolved,
		Params: v1.Params{{
			Name:  "foo",
			Value: *v1.NewStructuredValues("bar"),
		}, {
			Name:  "bar",
			Value: *v1.NewStructuredValues("test-task"),
		}},
	}
	tr := &v1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-task",
			Namespace: "default",
		},
		Spec: v1.TaskRunSpec{
			TaskRef:            taskRef,
			ServiceAccountName: "default",
			Params: []v1.Param{{
				Name:  "resolver-param",
				Value: *v1.NewStructuredValues("bar"),
			}},
		},
	}
	fn := resources.GetTaskFunc(ctx, nil, nil, requester, tr, tr.Spec.TaskRef, "", "default", "default", nil /*VerificationPolicies*/)

	resolvedTask, resolvedRefSource, _, err := fn(ctx, taskRef.Name)
	if err != nil {
		t.Fatalf("failed to call pipelinefn: %s", err.Error())
	}

	if d := cmp.Diff(task, resolvedTask); d != "" {
		t.Errorf("resolvedTask did not match: %s", diff.PrintWantGot(d))
	}

	if d := cmp.Diff(sampleRefSource, resolvedRefSource); d != "" {
		t.Errorf("refSources did not match: %s", diff.PrintWantGot(d))
	}

	taskRefNotMatching := &v1.TaskRef{
		ResolverRef: v1.ResolverRef{
			Resolver: "git",
			Params: []v1.Param{{
				Name:  "foo",
				Value: *v1.NewStructuredValues("$(params.resolver-param)"),
			}, {
				Name:  "bar",
				Value: *v1.NewStructuredValues("$(context.taskRun.name)"),
			}},
		},
	}

	trNotMatching := &v1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "other-task",
			Namespace: "default",
		},
		Spec: v1.TaskRunSpec{
			TaskRef:            taskRefNotMatching,
			ServiceAccountName: "default",
			Params: []v1.Param{{
				Name:  "resolver-param",
				Value: *v1.NewStructuredValues("banana"),
			}},
		},
	}
	fnNotMatching := resources.GetTaskFunc(ctx, nil, nil, requester, trNotMatching, trNotMatching.Spec.TaskRef, "", "default", "default", nil /*VerificationPolicies*/)

	_, _, _, err = fnNotMatching(ctx, taskRefNotMatching.Name)
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
	taskRef := &v1.TaskRef{ResolverRef: v1.ResolverRef{Resolver: "git"}}
	resolvesTo := []byte("INVALID YAML")
	resource := test.NewResolvedResource(resolvesTo, nil, nil, nil)
	requester := test.NewRequester(resource, nil)
	tr := &v1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default"},
		Spec: v1.TaskRunSpec{
			TaskRef:            taskRef,
			ServiceAccountName: "default",
		},
	}
	fn := resources.GetTaskFunc(ctx, nil, nil, requester, tr, tr.Spec.TaskRef, "", "default", "default", nil /*VerificationPolicies*/)
	if _, _, _, err := fn(ctx, taskRef.Name); err == nil {
		t.Fatalf("expected error due to invalid pipeline data but saw none")
	}
}

func TestGetTaskFunc_V1beta1Task_VerifyNoError(t *testing.T) {
	ctx := context.Background()
	signer, _, k8sclient, vps := test.SetupVerificationPolicies(t)
	tektonclient := fake.NewSimpleClientset()

	unsignedTask := test.GetUnsignedTask("test-task")
	unsignedTaskBytes, err := json.Marshal(unsignedTask)
	unsignedV1Task := &v1.Task{}
	unsignedTask.ConvertTo(ctx, unsignedV1Task)
	unsignedV1Task.APIVersion = "tekton.dev/v1"
	unsignedV1Task.Kind = "Task"

	if err != nil {
		t.Fatal("fail to marshal task", err)
	}
	noMatchPolicyRefSource := &v1.RefSource{
		URI: "abc.com",
	}
	requesterUnmatched := bytesToRequester(unsignedTaskBytes, noMatchPolicyRefSource)

	signedTask, err := test.GetSignedV1beta1Task(unsignedTask, signer, "signed")
	if err != nil {
		t.Fatal("fail to sign task", err)
	}
	signedV1Task := &v1.Task{}
	signedTask.ConvertTo(ctx, signedV1Task)
	signedV1Task.APIVersion = "tekton.dev/v1"
	signedV1Task.Kind = "Task"
	signedTaskBytes, err := json.Marshal(signedTask)
	if err != nil {
		t.Fatal("fail to marshal task", err)
	}
	matchPolicyRefSource := &v1.RefSource{
		URI: "	https://github.com/tektoncd/catalog.git",
	}
	requesterMatched := bytesToRequester(signedTaskBytes, matchPolicyRefSource)

	warnPolicyRefSource := &v1.RefSource{
		URI: "	warnVP",
	}
	requesterUnsignedMatched := bytesToRequester(unsignedTaskBytes, warnPolicyRefSource)

	taskRef := &v1.TaskRef{ResolverRef: v1.ResolverRef{Resolver: "git"}}

	testcases := []struct {
		name                       string
		requester                  *test.Requester
		verificationNoMatchPolicy  string
		policies                   []*v1alpha1.VerificationPolicy
		expected                   runtime.Object
		expectedRefSource          *v1.RefSource
		expectedVerificationResult *trustedresources.VerificationResult
	}{{
		name:                       "signed task with matching policy pass verification with enforce no match policy",
		requester:                  requesterMatched,
		verificationNoMatchPolicy:  config.FailNoMatchPolicy,
		policies:                   vps,
		expected:                   signedV1Task,
		expectedRefSource:          matchPolicyRefSource,
		expectedVerificationResult: &trustedresources.VerificationResult{VerificationResultType: trustedresources.VerificationPass},
	}, {
		name:                       "signed task with matching policy pass verification with warn no match policy",
		requester:                  requesterMatched,
		verificationNoMatchPolicy:  config.WarnNoMatchPolicy,
		policies:                   vps,
		expected:                   signedV1Task,
		expectedRefSource:          matchPolicyRefSource,
		expectedVerificationResult: &trustedresources.VerificationResult{VerificationResultType: trustedresources.VerificationPass},
	}, {
		name:                       "signed task with matching policy pass verification with ignore no match policy",
		requester:                  requesterMatched,
		verificationNoMatchPolicy:  config.IgnoreNoMatchPolicy,
		policies:                   vps,
		expected:                   signedV1Task,
		expectedRefSource:          matchPolicyRefSource,
		expectedVerificationResult: &trustedresources.VerificationResult{VerificationResultType: trustedresources.VerificationPass},
	}, {
		name:                       "warn unsigned task without matching policies",
		requester:                  requesterUnmatched,
		verificationNoMatchPolicy:  config.WarnNoMatchPolicy,
		policies:                   vps,
		expected:                   unsignedV1Task,
		expectedRefSource:          noMatchPolicyRefSource,
		expectedVerificationResult: &trustedresources.VerificationResult{VerificationResultType: trustedresources.VerificationWarn, Err: trustedresources.ErrNoMatchedPolicies},
	}, {
		name:                       "task fails warn mode policy return warn VerificationResult",
		requester:                  requesterUnsignedMatched,
		verificationNoMatchPolicy:  config.FailNoMatchPolicy,
		policies:                   vps,
		expected:                   unsignedV1Task,
		expectedRefSource:          warnPolicyRefSource,
		expectedVerificationResult: &trustedresources.VerificationResult{VerificationResultType: trustedresources.VerificationWarn, Err: trustedresources.ErrResourceVerificationFailed},
	}, {
		name:                       "ignore unsigned task without matching policies",
		requester:                  requesterUnmatched,
		verificationNoMatchPolicy:  config.IgnoreNoMatchPolicy,
		policies:                   vps,
		expected:                   unsignedV1Task,
		expectedRefSource:          noMatchPolicyRefSource,
		expectedVerificationResult: &trustedresources.VerificationResult{VerificationResultType: trustedresources.VerificationSkip},
	},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			ctx = test.SetupTrustedResourceConfig(ctx, tc.verificationNoMatchPolicy)
			tr := &v1.TaskRun{
				ObjectMeta: metav1.ObjectMeta{Namespace: "trusted-resources"},
				Spec: v1.TaskRunSpec{
					TaskRef:            taskRef,
					ServiceAccountName: "default",
				},
			}
			fn := resources.GetTaskFunc(ctx, k8sclient, tektonclient, tc.requester, tr, tr.Spec.TaskRef, "", "default", "default", tc.policies)

			resolvedTask, refSource, vr, err := fn(ctx, taskRef.Name)

			if err != nil {
				t.Fatalf("Received unexpected error ( %#v )", err)
			}

			if d := cmp.Diff(tc.expected, resolvedTask); d != "" {
				t.Errorf("resolvedTask did not match: %s", diff.PrintWantGot(d))
			}

			if d := cmp.Diff(tc.expectedRefSource, refSource); d != "" {
				t.Errorf("refSources did not match: %s", diff.PrintWantGot(d))
			}
			if tc.expectedVerificationResult.VerificationResultType != vr.VerificationResultType && errors.Is(vr.Err, tc.expectedVerificationResult.Err) {
				t.Errorf("VerificationResult mismatch: want %v, got %v", tc.expectedVerificationResult, vr)
			}
		})
	}
}

func TestGetTaskFunc_V1beta1Task_VerifyError(t *testing.T) {
	ctx := context.Background()
	signer, _, k8sclient, vps := test.SetupVerificationPolicies(t)
	tektonclient := fake.NewSimpleClientset()

	unsignedTask := test.GetUnsignedTask("test-task")
	unsignedTaskBytes, err := json.Marshal(unsignedTask)
	if err != nil {
		t.Fatal("fail to marshal task", err)
	}
	matchPolicyRefSource := &v1.RefSource{
		URI: "https://github.com/tektoncd/catalog.git",
	}
	requesterUnsigned := bytesToRequester(unsignedTaskBytes, matchPolicyRefSource)

	signedTask, err := test.GetSignedV1beta1Task(unsignedTask, signer, "signed")
	if err != nil {
		t.Fatal("fail to sign task", err)
	}
	signedTaskBytes, err := json.Marshal(signedTask)
	if err != nil {
		t.Fatal("fail to marshal task", err)
	}
	noMatchPolicyRefSource := &v1.RefSource{
		URI: "abc.com",
	}
	requesterUnmatched := bytesToRequester(signedTaskBytes, noMatchPolicyRefSource)

	modifiedTask := signedTask.DeepCopy()
	modifiedTask.Annotations["random"] = "attack"
	modifiedTaskBytes, err := json.Marshal(modifiedTask)
	if err != nil {
		t.Fatal("fail to marshal task", err)
	}
	requesterModified := bytesToRequester(modifiedTaskBytes, matchPolicyRefSource)

	taskRef := &v1.TaskRef{ResolverRef: v1.ResolverRef{Resolver: "git"}}

	testcases := []struct {
		name                           string
		requester                      *test.Requester
		verificationNoMatchPolicy      string
		expected                       *v1.Task
		expectedErr                    error
		expectedVerificationResultType trustedresources.VerificationResultType
	}{{
		name:                           "unsigned task fails verification with fail no match policy",
		requester:                      requesterUnsigned,
		verificationNoMatchPolicy:      config.FailNoMatchPolicy,
		expected:                       nil,
		expectedErr:                    trustedresources.ErrResourceVerificationFailed,
		expectedVerificationResultType: trustedresources.VerificationError,
	}, {
		name:                           "unsigned task fails verification with warn no match policy",
		requester:                      requesterUnsigned,
		verificationNoMatchPolicy:      config.WarnNoMatchPolicy,
		expected:                       nil,
		expectedErr:                    trustedresources.ErrResourceVerificationFailed,
		expectedVerificationResultType: trustedresources.VerificationError,
	}, {
		name:                           "unsigned task fails verification with ignore no match policy",
		requester:                      requesterUnsigned,
		verificationNoMatchPolicy:      config.IgnoreNoMatchPolicy,
		expected:                       nil,
		expectedErr:                    trustedresources.ErrResourceVerificationFailed,
		expectedVerificationResultType: trustedresources.VerificationError,
	}, {
		name:                           "modified task fails verification with fail no match policy",
		requester:                      requesterModified,
		verificationNoMatchPolicy:      config.FailNoMatchPolicy,
		expected:                       nil,
		expectedErr:                    trustedresources.ErrResourceVerificationFailed,
		expectedVerificationResultType: trustedresources.VerificationError,
	}, {
		name:                           "modified task fails verification with warn no match policy",
		requester:                      requesterModified,
		verificationNoMatchPolicy:      config.WarnNoMatchPolicy,
		expected:                       nil,
		expectedErr:                    trustedresources.ErrResourceVerificationFailed,
		expectedVerificationResultType: trustedresources.VerificationError,
	}, {
		name:                           "modified task fails verification with ignore no match policy",
		requester:                      requesterModified,
		verificationNoMatchPolicy:      config.IgnoreNoMatchPolicy,
		expected:                       nil,
		expectedErr:                    trustedresources.ErrResourceVerificationFailed,
		expectedVerificationResultType: trustedresources.VerificationError,
	}, {
		name:                           "unmatched task fails with verification fail no match policy",
		requester:                      requesterUnmatched,
		verificationNoMatchPolicy:      config.FailNoMatchPolicy,
		expected:                       nil,
		expectedErr:                    trustedresources.ErrNoMatchedPolicies,
		expectedVerificationResultType: trustedresources.VerificationError,
	},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			ctx = test.SetupTrustedResourceConfig(ctx, tc.verificationNoMatchPolicy)
			tr := &v1.TaskRun{
				ObjectMeta: metav1.ObjectMeta{Namespace: "trusted-resources"},
				Spec: v1.TaskRunSpec{
					TaskRef:            taskRef,
					ServiceAccountName: "default",
				},
			}
			fn := resources.GetTaskFunc(ctx, k8sclient, tektonclient, tc.requester, tr, tr.Spec.TaskRef, "", "default", "default", vps)

			_, _, vr, _ := fn(ctx, taskRef.Name)
			if !errors.Is(vr.Err, tc.expectedErr) {
				t.Errorf("GetPipelineFunc got %v, want %v", err, tc.expectedErr)
			}
			if tc.expectedVerificationResultType != vr.VerificationResultType {
				t.Errorf("VerificationResultType mismatch, want %d got %d", tc.expectedVerificationResultType, vr.VerificationResultType)
			}
		})
	}
}

func TestGetTaskFunc_V1Task_VerifyNoError(t *testing.T) {
	ctx := context.Background()
	signer, _, k8sclient, vps := test.SetupVerificationPolicies(t)
	tektonclient := fake.NewSimpleClientset()

	v1beta1UnsignedTask := &v1beta1.Task{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Task",
			APIVersion: "tekton.dev/v1beta1",
		},
	}
	if err := v1beta1UnsignedTask.ConvertFrom(ctx, unsignedV1Task.DeepCopy()); err != nil {
		t.Error(err)
	}

	unsignedTaskBytes, err := json.Marshal(unsignedV1Task)
	if err != nil {
		t.Fatal("fail to marshal task", err)
	}
	noMatchPolicyRefSource := &v1.RefSource{
		URI: "abc.com",
	}
	requesterUnmatched := bytesToRequester(unsignedTaskBytes, noMatchPolicyRefSource)

	signedV1Task, err := getSignedV1Task(unsignedV1Task.DeepCopy(), signer, "signed")
	if err != nil {
		t.Fatal("fail to sign task", err)
	}
	v1beta1SignedTask := &v1beta1.Task{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Task",
			APIVersion: "tekton.dev/v1beta1",
		},
	}
	if err := v1beta1SignedTask.ConvertFrom(ctx, signedV1Task.DeepCopy()); err != nil {
		t.Error(err)
	}

	signedTaskBytes, err := json.Marshal(signedV1Task)
	if err != nil {
		t.Fatal("fail to marshal task", err)
	}
	matchPolicyRefSource := &v1.RefSource{
		URI: "	https://github.com/tektoncd/catalog.git",
	}
	requesterMatched := bytesToRequester(signedTaskBytes, matchPolicyRefSource)

	warnPolicyRefSource := &v1.RefSource{
		URI: "	warnVP",
	}
	requesterUnsignedMatched := bytesToRequester(unsignedTaskBytes, warnPolicyRefSource)

	taskRef := &v1.TaskRef{ResolverRef: v1.ResolverRef{Resolver: "git"}}

	testcases := []struct {
		name                       string
		requester                  *test.Requester
		verificationNoMatchPolicy  string
		policies                   []*v1alpha1.VerificationPolicy
		expected                   runtime.Object
		expectedRefSource          *v1.RefSource
		expectedVerificationResult *trustedresources.VerificationResult
	}{{
		name:                       "signed task with matching policy pass verification with enforce no match policy",
		requester:                  requesterMatched,
		verificationNoMatchPolicy:  config.FailNoMatchPolicy,
		policies:                   vps,
		expected:                   signedV1Task,
		expectedRefSource:          matchPolicyRefSource,
		expectedVerificationResult: &trustedresources.VerificationResult{VerificationResultType: trustedresources.VerificationPass},
	}, {
		name:                       "signed task with matching policy pass verification with warn no match policy",
		requester:                  requesterMatched,
		verificationNoMatchPolicy:  config.WarnNoMatchPolicy,
		policies:                   vps,
		expected:                   signedV1Task,
		expectedRefSource:          matchPolicyRefSource,
		expectedVerificationResult: &trustedresources.VerificationResult{VerificationResultType: trustedresources.VerificationPass},
	}, {
		name:                       "signed task with matching policy pass verification with ignore no match policy",
		requester:                  requesterMatched,
		verificationNoMatchPolicy:  config.IgnoreNoMatchPolicy,
		policies:                   vps,
		expected:                   signedV1Task,
		expectedRefSource:          matchPolicyRefSource,
		expectedVerificationResult: &trustedresources.VerificationResult{VerificationResultType: trustedresources.VerificationPass},
	}, {
		name:                       "warn unsigned task without matching policies",
		requester:                  requesterUnmatched,
		verificationNoMatchPolicy:  config.WarnNoMatchPolicy,
		policies:                   vps,
		expected:                   &unsignedV1Task,
		expectedRefSource:          noMatchPolicyRefSource,
		expectedVerificationResult: &trustedresources.VerificationResult{VerificationResultType: trustedresources.VerificationWarn, Err: trustedresources.ErrNoMatchedPolicies},
	}, {
		name:                       "task fails warn mode policy return warn VerificationResult",
		requester:                  requesterUnsignedMatched,
		verificationNoMatchPolicy:  config.FailNoMatchPolicy,
		policies:                   vps,
		expected:                   &unsignedV1Task,
		expectedRefSource:          warnPolicyRefSource,
		expectedVerificationResult: &trustedresources.VerificationResult{VerificationResultType: trustedresources.VerificationWarn, Err: trustedresources.ErrResourceVerificationFailed},
	}, {
		name:                       "ignore unsigned task without matching policies",
		requester:                  requesterUnmatched,
		verificationNoMatchPolicy:  config.IgnoreNoMatchPolicy,
		policies:                   vps,
		expected:                   &unsignedV1Task,
		expectedRefSource:          noMatchPolicyRefSource,
		expectedVerificationResult: &trustedresources.VerificationResult{VerificationResultType: trustedresources.VerificationSkip},
	},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			ctx = test.SetupTrustedResourceConfig(ctx, tc.verificationNoMatchPolicy)
			tr := &v1.TaskRun{
				ObjectMeta: metav1.ObjectMeta{Namespace: "trusted-resources"},
				Spec: v1.TaskRunSpec{
					TaskRef:            taskRef,
					ServiceAccountName: "default",
				},
			}
			fn := resources.GetTaskFunc(ctx, k8sclient, tektonclient, tc.requester, tr, tr.Spec.TaskRef, "", "default", "default", tc.policies)

			gotResolvedTask, gotRefSource, gotVerificationResult, err := fn(ctx, taskRef.Name)

			if err != nil {
				t.Fatalf("Received unexpected error ( %#v )", err)
			}

			if d := cmp.Diff(tc.expected, gotResolvedTask); d != "" {
				t.Errorf("resolvedTask did not match: %s", diff.PrintWantGot(d))
			}

			if d := cmp.Diff(tc.expectedRefSource, gotRefSource); d != "" {
				t.Errorf("refSources did not match: %s", diff.PrintWantGot(d))
			}
			if d := cmp.Diff(gotVerificationResult, tc.expectedVerificationResult, verificationResultCmp); d != "" {
				t.Errorf("VerificationResult did not match:%s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestGetTaskFunc_V1Task_VerifyError(t *testing.T) {
	ctx := context.Background()
	signer, _, k8sclient, vps := test.SetupVerificationPolicies(t)
	tektonclient := fake.NewSimpleClientset()

	unsignedTaskBytes, err := json.Marshal(unsignedV1Task)
	if err != nil {
		t.Fatal("fail to marshal task", err)
	}
	matchPolicyRefSource := &v1.RefSource{
		URI: "https://github.com/tektoncd/catalog.git",
	}

	requesterUnsigned := bytesToRequester(unsignedTaskBytes, matchPolicyRefSource)

	signedV1Task, err := getSignedV1Task(unsignedV1Task.DeepCopy(), signer, "signed")
	if err != nil {
		t.Fatal("fail to sign task", err)
	}
	signedTaskBytes, err := json.Marshal(signedV1Task)
	if err != nil {
		t.Fatal("fail to marshal task", err)
	}
	noMatchPolicyRefSource := &v1.RefSource{
		URI: "abc.com",
	}
	requesterUnmatched := bytesToRequester(signedTaskBytes, noMatchPolicyRefSource)

	modifiedTask := signedV1Task.DeepCopy()
	modifiedTask.Annotations["random"] = "attack"
	modifiedTaskBytes, err := json.Marshal(modifiedTask)
	if err != nil {
		t.Fatal("fail to marshal task", err)
	}
	requesterModified := bytesToRequester(modifiedTaskBytes, matchPolicyRefSource)

	taskRef := &v1.TaskRef{ResolverRef: v1.ResolverRef{Resolver: "git"}}

	testcases := []struct {
		name                       string
		requester                  *test.Requester
		verificationNoMatchPolicy  string
		expected                   *v1.Task
		expectedErr                error
		expectedVerificationResult *trustedresources.VerificationResult
	}{{
		name:                       "unsigned task fails verification with fail no match policy",
		requester:                  requesterUnsigned,
		verificationNoMatchPolicy:  config.FailNoMatchPolicy,
		expected:                   nil,
		expectedErr:                trustedresources.ErrResourceVerificationFailed,
		expectedVerificationResult: &trustedresources.VerificationResult{VerificationResultType: trustedresources.VerificationError, Err: trustedresources.ErrResourceVerificationFailed},
	}, {
		name:                       "unsigned task fails verification with warn no match policy",
		requester:                  requesterUnsigned,
		verificationNoMatchPolicy:  config.WarnNoMatchPolicy,
		expected:                   nil,
		expectedErr:                trustedresources.ErrResourceVerificationFailed,
		expectedVerificationResult: &trustedresources.VerificationResult{VerificationResultType: trustedresources.VerificationError, Err: trustedresources.ErrResourceVerificationFailed},
	}, {
		name:                       "unsigned task fails verification with ignore no match policy",
		requester:                  requesterUnsigned,
		verificationNoMatchPolicy:  config.IgnoreNoMatchPolicy,
		expected:                   nil,
		expectedErr:                trustedresources.ErrResourceVerificationFailed,
		expectedVerificationResult: &trustedresources.VerificationResult{VerificationResultType: trustedresources.VerificationError, Err: trustedresources.ErrResourceVerificationFailed},
	}, {
		name:                       "modified task fails verification with fail no match policy",
		requester:                  requesterModified,
		verificationNoMatchPolicy:  config.FailNoMatchPolicy,
		expected:                   nil,
		expectedErr:                trustedresources.ErrResourceVerificationFailed,
		expectedVerificationResult: &trustedresources.VerificationResult{VerificationResultType: trustedresources.VerificationError, Err: trustedresources.ErrResourceVerificationFailed},
	}, {
		name:                       "modified task fails verification with warn no match policy",
		requester:                  requesterModified,
		verificationNoMatchPolicy:  config.WarnNoMatchPolicy,
		expected:                   nil,
		expectedErr:                trustedresources.ErrResourceVerificationFailed,
		expectedVerificationResult: &trustedresources.VerificationResult{VerificationResultType: trustedresources.VerificationError, Err: trustedresources.ErrResourceVerificationFailed},
	}, {
		name:                       "modified task fails verification with ignore no match policy",
		requester:                  requesterModified,
		verificationNoMatchPolicy:  config.IgnoreNoMatchPolicy,
		expected:                   nil,
		expectedVerificationResult: &trustedresources.VerificationResult{VerificationResultType: trustedresources.VerificationError, Err: trustedresources.ErrResourceVerificationFailed},
	}, {
		name:                       "unmatched task fails with verification fail no match policy",
		requester:                  requesterUnmatched,
		verificationNoMatchPolicy:  config.FailNoMatchPolicy,
		expected:                   nil,
		expectedVerificationResult: &trustedresources.VerificationResult{VerificationResultType: trustedresources.VerificationError, Err: trustedresources.ErrNoMatchedPolicies},
	},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			ctx = test.SetupTrustedResourceConfig(ctx, tc.verificationNoMatchPolicy)
			tr := &v1.TaskRun{
				ObjectMeta: metav1.ObjectMeta{Namespace: "trusted-resources"},
				Spec: v1.TaskRunSpec{
					TaskRef:            taskRef,
					ServiceAccountName: "default",
				},
			}
			fn := resources.GetTaskFunc(ctx, k8sclient, tektonclient, tc.requester, tr, tr.Spec.TaskRef, "", "default", "default", vps)

			_, _, gotVerificationResult, _ := fn(ctx, taskRef.Name)
			if d := cmp.Diff(gotVerificationResult, tc.expectedVerificationResult, verificationResultCmp); d != "" {
				t.Errorf("VerificationResult did not match:%s", diff.PrintWantGot(d))
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

	trResolutionError := &v1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{Namespace: "trusted-resources"},
		Spec: v1.TaskRunSpec{
			TaskRef: &v1.TaskRef{
				Name: "taskName",
				ResolverRef: v1.ResolverRef{
					Resolver: "git",
				},
			},
			ServiceAccountName: "default",
		},
	}

	testcases := []struct {
		name        string
		requester   *test.Requester
		taskrun     v1.TaskRun
		expectedErr error
	}{
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

			_, _, _, err = fn(ctx, tc.taskrun.Spec.TaskRef.Name)

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

var taskYAMLStringWithBetaFeatures = `
metadata:
  name: foo
spec:
  steps:
  - name: step1
    image: ubuntu
    script: |
      echo "hello world!"
  results:
  - name: array-result
    type: array
`

var remoteTaskYamlWithoutDefaults = `
metadata:
  name: simple
  namespace: default
spec:
  steps:
  - image: something
  params:
  - name: foo
`

func bytesToRequester(data []byte, source *v1.RefSource) *test.Requester {
	resolved := test.NewResolvedResource(data, nil, source, nil)
	requester := test.NewRequester(resolved, nil)
	return requester
}

func getSignedV1Task(unsigned *v1.Task, signer signature.Signer, name string) (*v1.Task, error) {
	signed := unsigned.DeepCopy()
	signed.Name = name
	if signed.Annotations == nil {
		signed.Annotations = map[string]string{}
	}
	signature, err := signInterface(signer, signed)
	if err != nil {
		return nil, err
	}
	signed.Annotations[trustedresources.SignatureAnnotation] = base64.StdEncoding.EncodeToString(signature)
	return signed, nil
}

func signInterface(signer signature.Signer, i interface{}) ([]byte, error) {
	if signer == nil {
		return nil, fmt.Errorf("signer is nil")
	}
	b, err := json.Marshal(i)
	if err != nil {
		return nil, err
	}
	h := sha256.New()
	h.Write(b)

	sig, err := signer.SignMessage(bytes.NewReader(h.Sum(nil)))
	if err != nil {
		return nil, err
	}

	return sig, nil
}
