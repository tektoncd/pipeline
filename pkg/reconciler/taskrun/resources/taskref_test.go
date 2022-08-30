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
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-containerregistry/pkg/registry"
	"github.com/tektoncd/pipeline/pkg/apis/config"
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
	sampleConfigSource = &v1beta1.ConfigSource{
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
			expected: &v1beta1.ClusterTask{
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

			task, configSource, err := lc.GetTask(ctx, tc.ref.Name)
			if tc.wantErr && err == nil {
				t.Fatal("Expected error but found nil instead")
			} else if !tc.wantErr && err != nil {
				t.Fatalf("Received unexpected error ( %#v )", err)
			}

			if d := cmp.Diff(task, tc.expected); tc.expected != nil && d != "" {
				t.Error(diff.PrintWantGot(d))
			}

			// local cluster tasks have empty source for now. This may be changed in future.
			if configSource != nil {
				t.Errorf("expected configsource is nil, but got %v", configSource)
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
			expected: &v1beta1.ClusterTask{
				ObjectMeta: metav1.ObjectMeta{Name: "simple"},
				TypeMeta: metav1.TypeMeta{
					APIVersion: "tekton.dev/v1beta1",
					Kind:       "ClusterTask",
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
			expected:     simpleClusterTask,
			expectedKind: v1beta1.ClusterTaskKind,
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
			fn, err := resources.GetTaskFunc(ctx, kubeclient, tektonclient, nil, trForFunc, tc.ref, "", "default", "default")
			if err != nil {
				t.Fatalf("failed to get task fn: %s", err.Error())
			}

			task, configSource, err := fn(ctx, tc.ref.Name)
			if err != nil {
				t.Fatalf("failed to call taskfn: %s", err.Error())
			}

			if diff := cmp.Diff(task, tc.expected); tc.expected != nil && diff != "" {
				t.Error(diff)
			}

			// local cluster task and  bundle task have empty source for now. This may be changed in future.
			if configSource != nil {
				t.Errorf("expected configsource is nil, but got %v", configSource)
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
				ConfigSource: sampleConfigSource.DeepCopy(),
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

	fn, err := resources.GetTaskFuncFromTaskRun(ctx, kubeclient, tektonclient, nil, TaskRun)
	if err != nil {
		t.Fatalf("failed to get Task fn: %s", err.Error())
	}
	actualTask, actualConfigSource, err := fn(ctx, name)
	if err != nil {
		t.Fatalf("failed to call Taskfn: %s", err.Error())
	}

	if diff := cmp.Diff(actualTask, expectedTask); expectedTask != nil && diff != "" {
		t.Error(diff)
	}

	if d := cmp.Diff(sampleConfigSource, actualConfigSource); d != "" {
		t.Errorf("configSources did not match: %s", diff.PrintWantGot(d))
	}

}

func TestGetTaskFunc_RemoteResolution(t *testing.T) {
	ctx := context.Background()
	cfg := config.FromContextOrDefaults(ctx)
	ctx = config.ToContext(ctx, cfg)
	task := parse.MustParseV1beta1Task(t, taskYAMLString)
	taskRef := &v1beta1.TaskRef{ResolverRef: v1beta1.ResolverRef{Resolver: "git"}}
	taskYAML := strings.Join([]string{
		"kind: Task",
		"apiVersion: tekton.dev/v1beta1",
		taskYAMLString,
	}, "\n")
	resolved := test.NewResolvedResource([]byte(taskYAML), nil, sampleConfigSource.DeepCopy(), nil)
	requester := test.NewRequester(resolved, nil)
	tr := &v1beta1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default"},
		Spec: v1beta1.TaskRunSpec{
			TaskRef:            taskRef,
			ServiceAccountName: "default",
		},
	}
	fn, err := resources.GetTaskFunc(ctx, nil, nil, requester, tr, tr.Spec.TaskRef, "", "default", "default")
	if err != nil {
		t.Fatalf("failed to get task fn: %s", err.Error())
	}

	resolvedTask, resolvedConfigSource, err := fn(ctx, taskRef.Name)
	if err != nil {
		t.Fatalf("failed to call pipelinefn: %s", err.Error())
	}

	if d := cmp.Diff(sampleConfigSource, resolvedConfigSource); d != "" {
		t.Errorf("configSources did not match: %s", diff.PrintWantGot(d))
	}

	if d := cmp.Diff(task, resolvedTask); d != "" {
		t.Error(d)
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
			Params: []v1beta1.Param{{
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

	resolved := test.NewResolvedResource([]byte(taskYAML), nil, sampleConfigSource.DeepCopy(), nil)
	requester := &test.Requester{
		ResolvedResource: resolved,
		Params: []v1beta1.Param{{
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
			Params: []v1beta1.Param{{
				Name:  "resolver-param",
				Value: *v1beta1.NewStructuredValues("bar"),
			}},
		},
	}
	fn, err := resources.GetTaskFunc(ctx, nil, nil, requester, tr, tr.Spec.TaskRef, "", "default", "default")
	if err != nil {
		t.Fatalf("failed to get task fn: %s", err.Error())
	}

	resolvedTask, resolvedConfigSource, err := fn(ctx, taskRef.Name)
	if err != nil {
		t.Fatalf("failed to call pipelinefn: %s", err.Error())
	}

	if d := cmp.Diff(task, resolvedTask); d != "" {
		t.Error(d)
	}

	if d := cmp.Diff(sampleConfigSource, resolvedConfigSource); d != "" {
		t.Errorf("configSources did not match: %s", diff.PrintWantGot(d))
	}

	taskRefNotMatching := &v1beta1.TaskRef{
		ResolverRef: v1beta1.ResolverRef{
			Resolver: "git",
			Params: []v1beta1.Param{{
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
			Params: []v1beta1.Param{{
				Name:  "resolver-param",
				Value: *v1beta1.NewStructuredValues("banana"),
			}},
		},
	}
	fnNotMatching, err := resources.GetTaskFunc(ctx, nil, nil, requester, trNotMatching, trNotMatching.Spec.TaskRef, "", "default", "default")
	if err != nil {
		t.Fatalf("failed to get task fn: %s", err.Error())
	}

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
	fn, err := resources.GetTaskFunc(ctx, nil, nil, requester, tr, tr.Spec.TaskRef, "", "default", "default")
	if err != nil {
		t.Fatalf("failed to get pipeline fn: %s", err.Error())
	}
	if _, _, err := fn(ctx, taskRef.Name); err == nil {
		t.Fatalf("expected error due to invalid pipeline data but saw none")
	}
}

func TestLocalTaskRef_TrustedResourceVerification_Success(t *testing.T) {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	signer, secretpath, err := test.GetSignerFromFile(ctx, t)
	if err != nil {
		t.Fatal(err)
	}

	unsignedTask := test.GetUnsignedTask("test-task")
	signedTask, err := test.GetSignedTask(unsignedTask, signer, "test-signed")
	if err != nil {
		t.Fatal("fail to sign task", err)
	}
	// attack another signed task
	signedTask2, err := test.GetSignedTask(test.GetUnsignedTask("test-task2"), signer, "test-signed2")
	if err != nil {
		t.Fatal("fail to sign task", err)
	}

	tamperedTask := signedTask2.DeepCopy()
	if tamperedTask.Annotations == nil {
		tamperedTask.Annotations = make(map[string]string)
	}
	tamperedTask.Annotations["random"] = "attack"

	tektonclient := fake.NewSimpleClientset(signedTask, unsignedTask, tamperedTask)
	testcases := []struct {
		name                     string
		ref                      *v1beta1.TaskRef
		resourceVerificationMode string
		expected                 runtime.Object
	}{
		{
			name: "local signed task with enforce policy",
			ref: &v1beta1.TaskRef{
				Name: "test-signed",
			},
			resourceVerificationMode: config.EnforceResourceVerificationMode,
			expected:                 signedTask,
		}, {
			name: "local unsigned task with warn policy",
			ref: &v1beta1.TaskRef{
				Name: "test-task",
			},
			resourceVerificationMode: config.WarnResourceVerificationMode,
			expected:                 unsignedTask,
		}, {
			name: "local signed task with warn policy",
			ref: &v1beta1.TaskRef{
				Name: "test-signed",
			},
			resourceVerificationMode: config.WarnResourceVerificationMode,
			expected:                 signedTask,
		}, {
			name: "local tampered task with warn policy",
			ref: &v1beta1.TaskRef{
				Name: "test-signed2",
			},
			resourceVerificationMode: config.SkipResourceVerificationMode,
			expected:                 tamperedTask,
		}, {
			name: "local unsigned task with skip policy",
			ref: &v1beta1.TaskRef{
				Name: "test-task",
			},
			resourceVerificationMode: config.SkipResourceVerificationMode,
			expected:                 unsignedTask,
		}, {
			name: "local signed task with skip policy",
			ref: &v1beta1.TaskRef{
				Name: "test-signed",
			},
			resourceVerificationMode: config.SkipResourceVerificationMode,
			expected:                 signedTask,
		}, {
			name: "local tampered task with skip policy",
			ref: &v1beta1.TaskRef{
				Name: "test-signed2",
			},
			resourceVerificationMode: config.WarnResourceVerificationMode,
			expected:                 tamperedTask,
		},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			ctx = test.SetupTrustedResourceConfig(ctx, secretpath, tc.resourceVerificationMode)
			lc := &resources.LocalTaskRefResolver{
				Namespace:    "trusted-resources",
				Kind:         tc.ref.Kind,
				Tektonclient: tektonclient,
			}

			task, source, err := lc.GetTask(ctx, tc.ref.Name)
			if err != nil {
				t.Fatalf("Received unexpected error %#v", err)
			}

			if d := cmp.Diff(task, tc.expected); d != "" {
				t.Error(diff.PrintWantGot(d))
			}

			if source != nil {
				t.Errorf("expected source for local task is nil, but got %v", source)
			}
		})
	}
}

func TestLocalTaskRef_TrustedResourceVerification_Error(t *testing.T) {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	signer, secretpath, err := test.GetSignerFromFile(ctx, t)
	if err != nil {
		t.Fatal(err)
	}

	unsignedTask := test.GetUnsignedTask("test-task")
	signedTask, err := test.GetSignedTask(unsignedTask, signer, "test-signed")
	if err != nil {
		t.Fatal("fail to sign task", err)
	}
	// attack another signed task
	signedTask2, err := test.GetSignedTask(test.GetUnsignedTask("test-task2"), signer, "test-signed2")
	if err != nil {
		t.Fatal("fail to sign task", err)
	}

	tamperedTask := signedTask2.DeepCopy()
	if tamperedTask.Annotations == nil {
		tamperedTask.Annotations = make(map[string]string)
	}
	tamperedTask.Annotations["random"] = "attack"

	tektonclient := fake.NewSimpleClientset(signedTask, unsignedTask, tamperedTask)
	testcases := []struct {
		name                     string
		ref                      *v1beta1.TaskRef
		resourceVerificationMode string
		expected                 runtime.Object
		expectedErr              error
	}{
		{
			name: "local unsigned task with enforce policy",
			ref: &v1beta1.TaskRef{
				Name: "test-task",
			},
			resourceVerificationMode: config.EnforceResourceVerificationMode,
			expected:                 nil,
			expectedErr:              trustedresources.ErrorResourceVerificationFailed,
		}, {
			name: "local tampered task with enforce policy",
			ref: &v1beta1.TaskRef{
				Name: "test-signed2",
			},
			resourceVerificationMode: config.EnforceResourceVerificationMode,
			expected:                 nil,
			expectedErr:              trustedresources.ErrorResourceVerificationFailed,
		},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			ctx = test.SetupTrustedResourceConfig(ctx, secretpath, tc.resourceVerificationMode)
			lc := &resources.LocalTaskRefResolver{
				Namespace:    "trusted-resources",
				Kind:         tc.ref.Kind,
				Tektonclient: tektonclient,
			}

			task, source, err := lc.GetTask(ctx, tc.ref.Name)
			if err == nil || !errors.Is(err, tc.expectedErr) {
				t.Fatalf("Expected error %v but found %v instead", tc.expectedErr, err)
			}
			if d := cmp.Diff(task, tc.expected); d != "" {
				t.Error(diff.PrintWantGot(d))
			}
			if source != nil {
				t.Errorf("expected source for local task is nil, but got %v", source)
			}
		})
	}
}

func TestGetTaskFunc_RemoteResolution_TrustedResourceVerification_Success(t *testing.T) {
	ctx := context.Background()
	signer, secretpath, err := test.GetSignerFromFile(ctx, t)
	if err != nil {
		t.Fatal(err)
	}

	unsignedTask := test.GetUnsignedTask("test-task")
	unsignedTaskBytes, err := json.Marshal(unsignedTask)
	if err != nil {
		t.Fatal("fail to marshal task", err)
	}

	resolvedUnsigned := test.NewResolvedResource(unsignedTaskBytes, nil, sampleConfigSource.DeepCopy(), nil)
	requesterUnsigned := test.NewRequester(resolvedUnsigned, nil)

	signedTask, err := test.GetSignedTask(unsignedTask, signer, "signed")
	if err != nil {
		t.Fatal("fail to sign task", err)
	}
	signedTaskBytes, err := json.Marshal(signedTask)
	if err != nil {
		t.Fatal("fail to marshal task", err)
	}

	resolvedSigned := test.NewResolvedResource(signedTaskBytes, nil, sampleConfigSource.DeepCopy(), nil)
	requesterSigned := test.NewRequester(resolvedSigned, nil)

	tamperedTask := signedTask.DeepCopy()
	tamperedTask.Annotations["random"] = "attack"
	tamperedTaskBytes, err := json.Marshal(tamperedTask)
	if err != nil {
		t.Fatal("fail to marshal task", err)
	}
	resolvedTampered := test.NewResolvedResource(tamperedTaskBytes, nil, sampleConfigSource.DeepCopy(), nil)
	requesterTampered := test.NewRequester(resolvedTampered, nil)

	taskRef := &v1beta1.TaskRef{ResolverRef: v1beta1.ResolverRef{Resolver: "git"}}

	testcases := []struct {
		name                     string
		requester                *test.Requester
		resourceVerificationMode string
		expected                 runtime.Object
	}{{
		name:                     "signed task with enforce policy",
		requester:                requesterSigned,
		resourceVerificationMode: config.EnforceResourceVerificationMode,
		expected:                 signedTask,
	}, {
		name:                     "unsigned task with warn policy",
		requester:                requesterUnsigned,
		resourceVerificationMode: config.WarnResourceVerificationMode,
		expected:                 unsignedTask,
	}, {
		name:                     "signed task with warn policy",
		requester:                requesterSigned,
		resourceVerificationMode: config.WarnResourceVerificationMode,
		expected:                 signedTask,
	}, {
		name:                     "tampered task with warn policy",
		requester:                requesterTampered,
		resourceVerificationMode: config.SkipResourceVerificationMode,
		expected:                 tamperedTask,
	}, {
		name:                     "unsigned task with skip policy",
		requester:                requesterUnsigned,
		resourceVerificationMode: config.SkipResourceVerificationMode,
		expected:                 unsignedTask,
	}, {
		name:                     "signed task with skip policy",
		requester:                requesterSigned,
		resourceVerificationMode: config.SkipResourceVerificationMode,
		expected:                 signedTask,
	}, {
		name:                     "tampered task with skip policy",
		requester:                requesterTampered,
		resourceVerificationMode: config.WarnResourceVerificationMode,
		expected:                 tamperedTask,
	},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			ctx = test.SetupTrustedResourceConfig(ctx, secretpath, tc.resourceVerificationMode)
			tr := &v1beta1.TaskRun{
				ObjectMeta: metav1.ObjectMeta{Namespace: "trusted-resources"},
				Spec: v1beta1.TaskRunSpec{
					TaskRef:            taskRef,
					ServiceAccountName: "default",
				},
			}
			fn, err := resources.GetTaskFunc(ctx, nil, nil, tc.requester, tr, tr.Spec.TaskRef, "", "default", "default")
			if err != nil {
				t.Fatalf("failed to get task fn: %s", err.Error())
			}

			resolvedTask, source, err := fn(ctx, taskRef.Name)

			if err != nil {
				t.Fatalf("Received unexpected error ( %#v )", err)
			}

			if d := cmp.Diff(tc.expected, resolvedTask); d != "" {
				t.Error(d)
			}

			if d := cmp.Diff(sampleConfigSource, source); d != "" {
				t.Errorf("configSources did not match: %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestGetTaskFunc_RemoteResolution_TrustedResourceVerification_Error(t *testing.T) {
	ctx := context.Background()
	signer, secretpath, err := test.GetSignerFromFile(ctx, t)
	if err != nil {
		t.Fatal(err)
	}

	unsignedTask := test.GetUnsignedTask("test-task")
	unsignedTaskBytes, err := json.Marshal(unsignedTask)
	if err != nil {
		t.Fatal("fail to marshal task", err)
	}

	resolvedUnsigned := test.NewResolvedResource(unsignedTaskBytes, nil, sampleConfigSource.DeepCopy(), nil)
	requesterUnsigned := test.NewRequester(resolvedUnsigned, nil)

	signedTask, err := test.GetSignedTask(unsignedTask, signer, "signed")
	if err != nil {
		t.Fatal("fail to sign task", err)
	}

	tamperedTask := signedTask.DeepCopy()
	tamperedTask.Annotations["random"] = "attack"
	tamperedTaskBytes, err := json.Marshal(tamperedTask)
	if err != nil {
		t.Fatal("fail to marshal task", err)
	}
	resolvedTampered := test.NewResolvedResource(tamperedTaskBytes, nil, sampleConfigSource.DeepCopy(), nil)
	requesterTampered := test.NewRequester(resolvedTampered, nil)

	taskRef := &v1beta1.TaskRef{ResolverRef: v1beta1.ResolverRef{Resolver: "git"}}

	testcases := []struct {
		name                     string
		requester                *test.Requester
		resourceVerificationMode string
		expected                 runtime.Object
		expectedErr              error
	}{{
		name:                     "unsigned task with enforce policy",
		requester:                requesterUnsigned,
		resourceVerificationMode: config.EnforceResourceVerificationMode,
		expected:                 nil,
		expectedErr:              trustedresources.ErrorResourceVerificationFailed,
	}, {
		name:                     "tampered task with enforce policy",
		requester:                requesterTampered,
		resourceVerificationMode: config.EnforceResourceVerificationMode,
		expected:                 nil,
		expectedErr:              trustedresources.ErrorResourceVerificationFailed,
	},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			ctx = test.SetupTrustedResourceConfig(ctx, secretpath, tc.resourceVerificationMode)
			tr := &v1beta1.TaskRun{
				ObjectMeta: metav1.ObjectMeta{Namespace: "trusted-resources"},
				Spec: v1beta1.TaskRunSpec{
					TaskRef:            taskRef,
					ServiceAccountName: "default",
				},
			}
			fn, err := resources.GetTaskFunc(ctx, nil, nil, tc.requester, tr, tr.Spec.TaskRef, "", "default", "default")
			if err != nil {
				t.Fatalf("failed to get task fn: %s", err.Error())
			}

			resolvedTask, source, err := fn(ctx, taskRef.Name)

			if err == nil || !errors.Is(err, tc.expectedErr) {
				t.Fatalf("Expected error %v but found %v instead", tc.expectedErr, err)
			}

			if d := cmp.Diff(tc.expected, resolvedTask); d != "" {
				t.Error(d)
			}

			if source != nil {
				t.Errorf("expected source is nil, but got: %v", source)
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
