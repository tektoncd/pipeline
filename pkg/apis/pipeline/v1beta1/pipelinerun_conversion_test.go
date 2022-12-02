/*
Copyright 2020 The Tetkon Authors

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

package v1beta1_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/pkg/apis/config"
	pod "github.com/tektoncd/pipeline/pkg/apis/pipeline/pod"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	runv1alpha1 "github.com/tektoncd/pipeline/pkg/apis/run/v1alpha1"
	"github.com/tektoncd/pipeline/test/diff"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	corev1resources "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/selection"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

var (
	childRefTaskRuns = []v1beta1.ChildStatusReference{{
		TypeMeta:         runtime.TypeMeta{Kind: "TaskRun", APIVersion: "v1beta1"},
		Name:             "tr-0",
		PipelineTaskName: "ptn",
		WhenExpressions:  []v1beta1.WhenExpression{{Input: "default-value", Operator: "in", Values: []string{"val"}}},
	}}
	childRefRuns = []v1beta1.ChildStatusReference{{
		TypeMeta:         runtime.TypeMeta{Kind: "Run", APIVersion: "v1alpha1"},
		Name:             "r-0",
		PipelineTaskName: "ptn-0",
		WhenExpressions:  []v1beta1.WhenExpression{{Input: "default-value-0", Operator: "in", Values: []string{"val-0", "val-1"}}},
	}}
	trs = &v1beta1.PipelineRunTaskRunStatus{
		PipelineTaskName: "ptn",
		Status: &v1beta1.TaskRunStatus{
			TaskRunStatusFields: v1beta1.TaskRunStatusFields{
				PodName:        "pod-name",
				StartTime:      &metav1.Time{Time: time.Now()},
				CompletionTime: &metav1.Time{Time: time.Now().Add(1 * time.Minute)},
			},
		},
		WhenExpressions: v1beta1.WhenExpressions{{
			Input:    "default-value",
			Operator: selection.In,
			Values:   []string{"val"},
		}},
	}
	rrs = &v1beta1.PipelineRunRunStatus{
		PipelineTaskName: "ptn-0",
		Status: &runv1alpha1.RunStatus{
			RunStatusFields: runv1alpha1.RunStatusFields{
				StartTime: &metav1.Time{Time: now},
			},
		},
		WhenExpressions: v1beta1.WhenExpressions{{
			Input:    "default-value-0",
			Operator: selection.In,
			Values:   []string{"val-0", "val-1"},
		}},
	}
)

func TestPipelineRunConversionBadType(t *testing.T) {
	good, bad := &v1beta1.PipelineRun{}, &v1beta1.Pipeline{}

	if err := good.ConvertTo(context.Background(), bad); err == nil {
		t.Errorf("ConvertTo() = %#v, wanted error", bad)
	}

	if err := good.ConvertFrom(context.Background(), bad); err == nil {
		t.Errorf("ConvertFrom() = %#v, wanted error", good)
	}
}

func TestPipelineRunConversion(t *testing.T) {
	tests := []struct {
		name string
		in   *v1beta1.PipelineRun
	}{{
		name: "simple pipelinerun",
		in: &v1beta1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "foo",
				Namespace: "bar",
			},
			Spec: v1beta1.PipelineRunSpec{
				PipelineRef: &v1beta1.PipelineRef{Name: "pipeline-1"},
			},
		}}, {
		name: "pipelinerun conversion all non deprecated fields",
		in: &v1beta1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "foo",
				Namespace: "bar",
			},
			Spec: v1beta1.PipelineRunSpec{
				PipelineRef: &v1beta1.PipelineRef{Name: "pipeline-1"},
				PipelineSpec: &v1beta1.PipelineSpec{
					Params: []v1beta1.ParamSpec{{
						Name: "foo",
						Type: "string",
					}},
				},
				Params: []v1beta1.Param{{
					Name:  "foo",
					Value: *v1beta1.NewStructuredValues("value"),
				}, {
					Name:  "bar",
					Value: *v1beta1.NewStructuredValues("value"),
				}},
				ServiceAccountName: "test-sa",
				Status:             v1beta1.PipelineRunSpecStatusPending,
				Timeouts: &v1beta1.TimeoutFields{
					Pipeline: &metav1.Duration{Duration: 25 * time.Minute},
					Finally:  &metav1.Duration{Duration: 1 * time.Hour},
					Tasks:    &metav1.Duration{Duration: 1 * time.Hour},
				},
				PodTemplate: &pod.Template{
					NodeSelector: map[string]string{
						"label": "value",
					},
					SecurityContext: &corev1.PodSecurityContext{
						RunAsNonRoot: &ttrue,
					},
					HostNetwork: false,
				},
				Workspaces: []v1beta1.WorkspaceBinding{{
					Name:     "workspace",
					EmptyDir: &corev1.EmptyDirVolumeSource{},
				}},
				TaskRunSpecs: []v1beta1.PipelineTaskRunSpec{
					{
						PipelineTaskName:       "bar",
						TaskServiceAccountName: "test-tsa",
						TaskPodTemplate: &pod.Template{
							NodeSelector: map[string]string{
								"label": "value",
							},
							SecurityContext: &corev1.PodSecurityContext{
								RunAsNonRoot: &ttrue,
							},
							HostNetwork: false,
						},
						StepOverrides: []v1beta1.TaskRunStepOverride{{
							Name: "test-so",
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{corev1.ResourceMemory: corev1resources.MustParse("1Gi")},
							}},
						},
						SidecarOverrides: []v1beta1.TaskRunSidecarOverride{{
							Name: "test-so",
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{corev1.ResourceMemory: corev1resources.MustParse("1Gi")},
							}},
						},
						Metadata: &v1beta1.PipelineTaskMetadata{
							Labels: map[string]string{
								"foo": "bar",
							},
							Annotations: map[string]string{
								"foo": "bar",
							},
						},
						ComputeResources: &corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("2"),
								corev1.ResourceMemory: resource.MustParse("2Gi"),
							},
						},
					},
				},
			},
			Status: v1beta1.PipelineRunStatus{
				Status: duckv1.Status{
					Conditions: duckv1.Conditions{{
						Type:    apis.ConditionSucceeded,
						Status:  corev1.ConditionTrue,
						Reason:  "Completed",
						Message: "All tasks finished running",
					}},
					ObservedGeneration: 1,
				},
				PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{
					StartTime:      &metav1.Time{Time: time.Now()},
					CompletionTime: &metav1.Time{Time: time.Now().Add(1 * time.Minute)},
					PipelineResults: []v1beta1.PipelineRunResult{{
						Name: "pipeline-result-1",
						Value: *v1beta1.NewObject(map[string]string{
							"pkey1": "val1",
							"pkey2": "rae",
						})}, {
						Name: "pipeline-result-2",
						Value: *v1beta1.NewObject(map[string]string{
							"pkey1": "val2",
							"pkey2": "rae2",
						}),
					}},
					PipelineSpec: &v1beta1.PipelineSpec{
						Tasks: []v1beta1.PipelineTask{{
							Name: "mytask",
							TaskRef: &v1beta1.TaskRef{
								Name: "mytask",
							},
						}},
					},
					SkippedTasks: []v1beta1.SkippedTask{
						{
							Name:   "skipped-1",
							Reason: v1beta1.WhenExpressionsSkip,
							WhenExpressions: []v1beta1.WhenExpression{{
								Input:    "foo",
								Operator: "notin",
								Values:   []string{"foo", "bar"},
							}},
						}, {
							Name:   "skipped-2",
							Reason: v1beta1.MissingResultsSkip,
						},
					},
					ChildReferences: []v1beta1.ChildStatusReference{
						{
							TypeMeta:         runtime.TypeMeta{Kind: "TaskRun"},
							Name:             "t1",
							PipelineTaskName: "task-1",
							WhenExpressions: []v1beta1.WhenExpression{{
								Input:    "foo",
								Operator: "notin",
								Values:   []string{"foo", "bar"},
							}},
						},
						{
							TypeMeta:         runtime.TypeMeta{Kind: "Run"},
							Name:             "t2",
							PipelineTaskName: "task-2",
						},
					},
					FinallyStartTime: &metav1.Time{Time: time.Now()},
					Provenance: &v1beta1.Provenance{
						ConfigSource: &v1beta1.ConfigSource{
							URI:    "test-uri",
							Digest: map[string]string{"sha256": "digest"},
						},
					},
				},
			},
		},
	}}
	for _, test := range tests {
		versions := []apis.Convertible{&v1.PipelineRun{}}
		for _, version := range versions {
			t.Run(test.name, func(t *testing.T) {
				ver := version
				ctx := withMinimalEmbeddedStatus(context.Background())
				if err := test.in.ConvertTo(ctx, ver); err != nil {
					t.Errorf("ConvertTo() = %v", err)
					return
				}
				t.Logf("ConvertTo() = %#v", ver)
				got := &v1beta1.PipelineRun{}
				if err := got.ConvertFrom(ctx, ver); err != nil {
					t.Errorf("ConvertFrom() = %v", err)
				}
				t.Logf("ConvertFrom() = %#v", got)
				if d := cmp.Diff(test.in, got); d != "" {
					t.Errorf("roundtrip %s", diff.PrintWantGot(d))
				}
			})
		}
	}
}

func TestPipelineRunConversionFromDeprecated(t *testing.T) {
	taskRuns := make(map[string]*v1beta1.PipelineRunTaskRunStatus)
	runs := make(map[string]*v1beta1.PipelineRunRunStatus)
	trs0 := &v1beta1.PipelineRunTaskRunStatus{
		PipelineTaskName: "ptn",
		Status: &v1beta1.TaskRunStatus{
			TaskRunStatusFields: v1beta1.TaskRunStatusFields{
				PodName:        "pod-name",
				StartTime:      &metav1.Time{Time: time.Now()},
				CompletionTime: &metav1.Time{Time: time.Now().Add(1 * time.Minute)},
			},
		},
		WhenExpressions: v1beta1.WhenExpressions{{
			Input:    "default-value",
			Operator: selection.In,
			Values:   []string{"val"},
		}},
	}
	trs1 := &v1beta1.PipelineRunTaskRunStatus{
		PipelineTaskName: "ptn-1",
		Status: &v1beta1.TaskRunStatus{
			TaskRunStatusFields: v1beta1.TaskRunStatusFields{
				PodName:        "pod-name-1",
				StartTime:      &metav1.Time{Time: time.Now()},
				CompletionTime: &metav1.Time{Time: time.Now().Add(1 * time.Minute)},
			},
		},
		WhenExpressions: v1beta1.WhenExpressions{{
			Input:    "default-value-1",
			Operator: selection.In,
			Values:   []string{"val-1", "val-2"},
		}},
	}
	rrs0 := &v1beta1.PipelineRunRunStatus{
		PipelineTaskName: "ptn-0",
		Status: &runv1alpha1.RunStatus{
			RunStatusFields: runv1alpha1.RunStatusFields{
				StartTime: &metav1.Time{Time: now},
			},
		},
		WhenExpressions: v1beta1.WhenExpressions{{
			Input:    "default-value-0",
			Operator: selection.In,
			Values:   []string{"val-0", "val-1"},
		}},
	}
	rrs1 := &v1beta1.PipelineRunRunStatus{
		PipelineTaskName: "ptn-1",
		Status: &runv1alpha1.RunStatus{
			RunStatusFields: runv1alpha1.RunStatusFields{
				StartTime: &metav1.Time{Time: now},
			},
		},
		WhenExpressions: v1beta1.WhenExpressions{{
			Input:    "default-value-1",
			Operator: selection.In,
			Values:   []string{"val-0", "val-1"},
		}},
	}
	taskRuns["tr-0"] = trs0
	taskRuns["tr-1"] = trs1
	runs["r-0"] = rrs0
	runs["r-1"] = rrs1
	tests := []struct {
		name string
		in   *v1beta1.PipelineRun
		want *v1beta1.PipelineRun
	}{{
		name: "resources",
		in: &v1beta1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "foo",
				Namespace: "bar",
			},
			Spec: v1beta1.PipelineRunSpec{
				Resources: []v1beta1.PipelineResourceBinding{
					{
						Name:        "git-resource",
						ResourceRef: &v1beta1.PipelineResourceRef{Name: "sweet-resource"},
					}, {
						Name:        "image-resource",
						ResourceRef: &v1beta1.PipelineResourceRef{Name: "sweet-resource2"},
					},
				},
			},
		},
		want: &v1beta1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "foo",
				Namespace: "bar",
			},
			Spec: v1beta1.PipelineRunSpec{
				Resources: []v1beta1.PipelineResourceBinding{
					{
						Name:        "git-resource",
						ResourceRef: &v1beta1.PipelineResourceRef{Name: "sweet-resource"},
					}, {
						Name:        "image-resource",
						ResourceRef: &v1beta1.PipelineResourceRef{Name: "sweet-resource2"},
					},
				},
			},
		},
	}, {
		name: "timeout to timeouts",
		in: &v1beta1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "foo",
				Namespace: "bar",
			},
			Spec: v1beta1.PipelineRunSpec{
				Timeout: &metav1.Duration{Duration: 5 * time.Minute},
			},
		},
		want: &v1beta1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "foo",
				Namespace: "bar",
			},
			Spec: v1beta1.PipelineRunSpec{
				Timeouts: &v1beta1.TimeoutFields{
					Pipeline: &metav1.Duration{Duration: 5 * time.Minute},
				},
			},
		},
	}, {
		name: "bundle",
		in: &v1beta1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "foo",
				Namespace: "bar",
			},
			Spec: v1beta1.PipelineRunSpec{
				PipelineRef: &v1beta1.PipelineRef{
					Name:   "test-bundle-name",
					Bundle: "test-bundle",
				},
			},
		},
		want: &v1beta1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "foo",
				Namespace: "bar",
			},
			Spec: v1beta1.PipelineRunSpec{
				PipelineRef: &v1beta1.PipelineRef{
					ResolverRef: v1beta1.ResolverRef{
						Resolver: "bundles",
						Params: []v1beta1.Param{
							{Name: "bundle", Value: v1beta1.ParamValue{StringVal: "test-bundle", Type: "string"}},
							{Name: "name", Value: v1beta1.ParamValue{StringVal: "test-bundle-name", Type: "string"}},
							{Name: "kind", Value: v1beta1.ParamValue{StringVal: "Task", Type: "string"}},
						},
					},
				},
			},
		},
	}, {
		name: "taskRuns",
		in: &v1beta1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "foo",
				Namespace: "bar",
			},
			Spec: v1beta1.PipelineRunSpec{
				PipelineRef: &v1beta1.PipelineRef{
					Name: "test",
				},
			},
			Status: v1beta1.PipelineRunStatus{
				PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{
					TaskRuns: taskRuns,
				},
			},
		},
		want: &v1beta1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "foo",
				Namespace: "bar",
			},
			Spec: v1beta1.PipelineRunSpec{
				PipelineRef: &v1beta1.PipelineRef{
					Name: "test",
				},
			},
			Status: v1beta1.PipelineRunStatus{
				PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{
					ChildReferences: []v1beta1.ChildStatusReference{{
						TypeMeta:         runtime.TypeMeta{Kind: "TaskRun"},
						Name:             "tr-0",
						PipelineTaskName: "ptn",
						WhenExpressions:  []v1beta1.WhenExpression{{Input: "default-value", Operator: "in", Values: []string{"val"}}},
					}, {
						TypeMeta:         runtime.TypeMeta{Kind: "TaskRun"},
						Name:             "tr-1",
						PipelineTaskName: "ptn-1",
						WhenExpressions:  []v1beta1.WhenExpression{{Input: "default-value-1", Operator: "in", Values: []string{"val-1", "val-2"}}},
					}},
				},
			},
		},
	}, {
		name: "runs",
		in: &v1beta1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "foo",
				Namespace: "bar",
			},
			Spec: v1beta1.PipelineRunSpec{
				PipelineRef: &v1beta1.PipelineRef{
					Name: "test-runs",
				},
			},
			Status: v1beta1.PipelineRunStatus{
				PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{
					Runs:     runs,
					TaskRuns: taskRuns,
				},
			},
		},
		want: &v1beta1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "foo",
				Namespace: "bar",
			},
			Spec: v1beta1.PipelineRunSpec{
				PipelineRef: &v1beta1.PipelineRef{
					Name: "test-runs",
				},
			},
			Status: v1beta1.PipelineRunStatus{
				PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{
					ChildReferences: []v1beta1.ChildStatusReference{{
						TypeMeta:         runtime.TypeMeta{Kind: "TaskRun"},
						Name:             "tr-0",
						PipelineTaskName: "ptn",
						WhenExpressions:  []v1beta1.WhenExpression{{Input: "default-value", Operator: "in", Values: []string{"val"}}},
					}, {
						TypeMeta:         runtime.TypeMeta{Kind: "TaskRun"},
						Name:             "tr-1",
						PipelineTaskName: "ptn-1",
						WhenExpressions:  []v1beta1.WhenExpression{{Input: "default-value-1", Operator: "in", Values: []string{"val-1", "val-2"}}},
					}, {
						TypeMeta:         runtime.TypeMeta{Kind: "Run"},
						Name:             "r-0",
						PipelineTaskName: "ptn-0",
						WhenExpressions:  []v1beta1.WhenExpression{{Input: "default-value-0", Operator: "in", Values: []string{"val-0", "val-1"}}},
					}, {
						TypeMeta:         runtime.TypeMeta{Kind: "Run"},
						Name:             "r-1",
						PipelineTaskName: "ptn-1",
						WhenExpressions:  []v1beta1.WhenExpression{{Input: "default-value-1", Operator: "in", Values: []string{"val-0", "val-1"}}},
					}},
				},
			},
		},
	}}
	for _, test := range tests {
		versions := []apis.Convertible{&v1.PipelineRun{}}
		for _, version := range versions {
			t.Run(test.name, func(t *testing.T) {
				ver := version
				ctx := withMinimalEmbeddedStatus(context.Background())
				if err := test.in.ConvertTo(ctx, ver); err != nil {
					t.Errorf("ConvertTo() = %v", err)
				}
				t.Logf("ConvertTo() = %#v", ver)
				got := &v1beta1.PipelineRun{}
				if err := got.ConvertFrom(ctx, ver); err != nil {
					t.Errorf("ConvertFrom() = %v", err)
				}
				t.Logf("ConvertFrom() = %#v", got)
				if d := cmp.Diff(test.want, got); d != "" {
					t.Errorf("roundtrip %s", diff.PrintWantGot(d))
				}
			})
		}
	}
}

func TestPipelineRunConversionMinimalEmbeddedStatus(t *testing.T) {
	taskRuns := make(map[string]*v1beta1.PipelineRunTaskRunStatus)
	runs := make(map[string]*v1beta1.PipelineRunRunStatus)
	taskRuns["tr-0"] = trs
	runs["r-0"] = rrs

	tests := []struct {
		name string
		in   *v1beta1.PipelineRun
		want *v1beta1.PipelineRun
	}{{
		name: "taskRuns",
		in: &v1beta1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "foo",
				Namespace: "bar",
			},
			Spec: v1beta1.PipelineRunSpec{
				PipelineRef: &v1beta1.PipelineRef{
					Name: "test",
				},
			},
			Status: v1beta1.PipelineRunStatus{
				PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{
					TaskRuns: taskRuns,
				},
			},
		},
		want: &v1beta1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "foo",
				Namespace: "bar",
			},
			Spec: v1beta1.PipelineRunSpec{
				PipelineRef: &v1beta1.PipelineRef{
					Name: "test",
				},
			},
			Status: v1beta1.PipelineRunStatus{
				PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{
					ChildReferences: childRefTaskRuns,
				},
			},
		},
	}, {
		name: "runs",
		in: &v1beta1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "foo",
				Namespace: "bar",
			},
			Spec: v1beta1.PipelineRunSpec{
				PipelineRef: &v1beta1.PipelineRef{
					Name: "test-runs",
				},
			},
			Status: v1beta1.PipelineRunStatus{
				PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{
					Runs:     runs,
					TaskRuns: taskRuns,
				},
			},
		},
		want: &v1beta1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "foo",
				Namespace: "bar",
			},
			Spec: v1beta1.PipelineRunSpec{
				PipelineRef: &v1beta1.PipelineRef{
					Name: "test-runs",
				},
			},
			Status: v1beta1.PipelineRunStatus{
				PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{
					ChildReferences: append(childRefTaskRuns, childRefRuns...),
				},
			},
		},
	}, {
		name: "runs and childReference",
		in: &v1beta1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "foo",
				Namespace: "bar",
			},
			Spec: v1beta1.PipelineRunSpec{
				PipelineRef: &v1beta1.PipelineRef{
					Name: "test-runs",
				},
			},
			Status: v1beta1.PipelineRunStatus{
				PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{
					Runs:            runs,
					ChildReferences: childRefRuns,
				},
			},
		},
		want: &v1beta1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "foo",
				Namespace: "bar",
			},
			Spec: v1beta1.PipelineRunSpec{
				PipelineRef: &v1beta1.PipelineRef{
					Name: "test-runs",
				},
			},
			Status: v1beta1.PipelineRunStatus{
				PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{
					ChildReferences: childRefRuns,
				},
			},
		},
	},
		{
			name: "taskruns, runs and childReference",
			in: &v1beta1.PipelineRun{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "bar",
				},
				Spec: v1beta1.PipelineRunSpec{
					PipelineRef: &v1beta1.PipelineRef{
						Name: "test-runs",
					},
				},
				Status: v1beta1.PipelineRunStatus{
					PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{
						Runs:            runs,
						TaskRuns:        taskRuns,
						ChildReferences: append(childRefRuns, childRefTaskRuns...),
					},
				},
			},
			want: &v1beta1.PipelineRun{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "bar",
				},
				Spec: v1beta1.PipelineRunSpec{
					PipelineRef: &v1beta1.PipelineRef{
						Name: "test-runs",
					},
				},
				Status: v1beta1.PipelineRunStatus{
					PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{
						ChildReferences: append(childRefRuns, childRefTaskRuns...),
					},
				},
			},
		}}
	for _, test := range tests {
		versions := []apis.Convertible{&v1.PipelineRun{}}
		for _, version := range versions {
			t.Run(test.name, func(t *testing.T) {
				ctx := withMinimalEmbeddedStatus(context.Background())
				ver := version

				if err := test.in.ConvertTo(ctx, ver); err != nil {
					t.Errorf("ConvertTo() = %v", err)
				}
				t.Logf("ConvertTo() = %#v", ver)
				got := &v1beta1.PipelineRun{}

				if err := got.ConvertFrom(ctx, ver); err != nil {
					t.Errorf("ConvertFrom() = %v", err)
				}
				t.Logf("ConvertFrom() = %#v", got)
				if d := cmp.Diff(test.want, got); d != "" {
					t.Errorf("roundtrip %s", diff.PrintWantGot(d))
				}
			})
		}
	}
}

func TestPipelineRunConversionEmbeddedStatusError(t *testing.T) {
	tests := []struct {
		name           string
		in             *v1.PipelineRun
		embeddedStatus string
		wantErr        bool
	}{{
		name: "v1 to v1beta1 with full embedded status invalid",
		in: &v1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "foo",
				Namespace: "bar",
			},
			Spec: v1.PipelineRunSpec{
				PipelineRef: &v1.PipelineRef{
					Name: "test",
				},
			},
			Status: v1.PipelineRunStatus{
				PipelineRunStatusFields: v1.PipelineRunStatusFields{
					ChildReferences: []v1.ChildStatusReference{{
						TypeMeta:         runtime.TypeMeta{Kind: "TaskRun", APIVersion: "v1beta1"},
						Name:             "tr-0",
						PipelineTaskName: "ptn",
						WhenExpressions:  []v1.WhenExpression{{Input: "default-value", Operator: "in", Values: []string{"val"}}},
					}},
				},
			},
		},
		embeddedStatus: config.FullEmbeddedStatus,
		wantErr:        true,
	}, {
		name: "v1 to v1beta1 status with both embedded status invalid",
		in: &v1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "foo",
				Namespace: "bar",
			},
			Spec: v1.PipelineRunSpec{
				PipelineRef: &v1.PipelineRef{
					Name: "test",
				},
			},
			Status: v1.PipelineRunStatus{
				PipelineRunStatusFields: v1.PipelineRunStatusFields{
					ChildReferences: []v1.ChildStatusReference{{
						TypeMeta:         runtime.TypeMeta{Kind: "TaskRun", APIVersion: "v1beta1"},
						Name:             "tr-0",
						PipelineTaskName: "ptn",
						WhenExpressions:  []v1.WhenExpression{{Input: "default-value", Operator: "in", Values: []string{"val"}}},
					}},
				},
			},
		},
		embeddedStatus: config.BothEmbeddedStatus,
		wantErr:        true,
	}}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			featureFlags, _ := config.NewFeatureFlagsFromMap(map[string]string{
				"embedded-status": test.embeddedStatus,
			})
			cfg := &config.Config{FeatureFlags: featureFlags}
			ctx := config.ToContext(context.Background(), cfg)

			v1beta1PipelineRun := &v1beta1.PipelineRun{}

			err := v1beta1PipelineRun.ConvertFrom(ctx, test.in)
			if err == nil && test.wantErr {
				t.Errorf("expected error but received nil")
			}
		})
	}
}

func withMinimalEmbeddedStatus(ctx context.Context) context.Context {
	featureFlags, _ := config.NewFeatureFlagsFromMap(map[string]string{
		"embedded-status": config.MinimalEmbeddedStatus,
	})
	cfg := &config.Config{FeatureFlags: featureFlags}
	return config.ToContext(context.Background(), cfg)
}
