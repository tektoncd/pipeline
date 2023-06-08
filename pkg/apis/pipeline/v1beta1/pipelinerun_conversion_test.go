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
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/tektoncd/pipeline/pkg/apis/config"
	pod "github.com/tektoncd/pipeline/pkg/apis/pipeline/pod"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	runv1beta1 "github.com/tektoncd/pipeline/pkg/apis/run/v1beta1"
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
		TypeMeta:         runtime.TypeMeta{Kind: "TaskRun", APIVersion: "tekton.dev/v1beta1"},
		Name:             "tr-0",
		PipelineTaskName: "ptn",
		WhenExpressions:  []v1beta1.WhenExpression{{Input: "default-value", Operator: "in", Values: []string{"val"}}},
	}}
	childRefRuns = []v1beta1.ChildStatusReference{{
		TypeMeta:         runtime.TypeMeta{Kind: "Run", APIVersion: "tekton.dev/v1alpha1"},
		Name:             "r-0",
		PipelineTaskName: "ptn-0",
		WhenExpressions:  []v1beta1.WhenExpression{{Input: "default-value-0", Operator: "in", Values: []string{"val-0", "val-1"}}},
	}}
	trs = &v1beta1.PipelineRunTaskRunStatus{
		PipelineTaskName: "ptn",
		Status: &v1beta1.TaskRunStatus{
			TaskRunStatusFields: v1beta1.TaskRunStatusFields{
				PodName: "pod-name",
				RetriesStatus: []v1beta1.TaskRunStatus{{
					Status: duckv1.Status{
						Conditions: []apis.Condition{{
							Type:   apis.ConditionSucceeded,
							Status: corev1.ConditionFalse,
						}},
					},
				}},
				Steps: []v1beta1.StepState{{
					ContainerState: corev1.ContainerState{
						Running: &corev1.ContainerStateRunning{},
					},
					Name:          "running-step",
					ContainerName: "step-running-step",
				}},
				CloudEvents: []v1beta1.CloudEventDelivery{
					{
						Target: "http//sink1",
						Status: v1beta1.CloudEventDeliveryState{
							Condition: v1beta1.CloudEventConditionUnknown,
						},
					},
					{
						Target: "http//sink2",
						Status: v1beta1.CloudEventDeliveryState{
							Condition: v1beta1.CloudEventConditionUnknown,
						},
					},
				},
				Sidecars: []v1beta1.SidecarState{{ContainerState: corev1.ContainerState{
					Terminated: &corev1.ContainerStateTerminated{
						ExitCode: 1,
						Reason:   "Error",
						Message:  "Error",
					},
				},
					Name:          "error",
					ImageID:       "image-id",
					ContainerName: "sidecar-error",
				}},
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
		Status: &runv1beta1.CustomRunStatus{
			CustomRunStatusFields: runv1beta1.CustomRunStatusFields{
				Results: []runv1beta1.CustomRunResult{{
					Name:  "foo",
					Value: "bar",
				}},
			},
		},
		WhenExpressions: v1beta1.WhenExpressions{{
			Input:    "default-value-0",
			Operator: selection.In,
			Values:   []string{"val-0", "val-1"},
		}},
	}
	taskRuns = make(map[string]*v1beta1.PipelineRunTaskRunStatus)
	runs     = make(map[string]*v1beta1.PipelineRunRunStatus)
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
		},
	}, {
		name: "pipelinerun with deprecated fields in step and stepTemplate",
		in: &v1beta1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "foo",
				Namespace: "bar",
			},
			Spec: v1beta1.PipelineRunSpec{
				PipelineSpec: &v1beta1.PipelineSpec{
					Tasks: []v1beta1.PipelineTask{{
						Name: "task-1",
						TaskSpec: &v1beta1.EmbeddedTask{
							TaskSpec: v1beta1.TaskSpec{
								Steps: []v1beta1.Step{{
									DeprecatedLivenessProbe:  &corev1.Probe{InitialDelaySeconds: 1},
									DeprecatedReadinessProbe: &corev1.Probe{InitialDelaySeconds: 2},
									DeprecatedPorts:          []corev1.ContainerPort{{Name: "port"}},
									DeprecatedStartupProbe:   &corev1.Probe{InitialDelaySeconds: 3},
									DeprecatedLifecycle: &corev1.Lifecycle{PostStart: &corev1.LifecycleHandler{Exec: &corev1.ExecAction{
										Command: []string{"lifecycle command"},
									}}},
									DeprecatedTerminationMessagePath:   "path",
									DeprecatedTerminationMessagePolicy: corev1.TerminationMessagePolicy("policy"),
									DeprecatedStdin:                    true,
									DeprecatedStdinOnce:                true,
									DeprecatedTTY:                      true,
								}},
								StepTemplate: &v1beta1.StepTemplate{
									DeprecatedLivenessProbe:  &corev1.Probe{InitialDelaySeconds: 1},
									DeprecatedReadinessProbe: &corev1.Probe{InitialDelaySeconds: 2},
									DeprecatedPorts:          []corev1.ContainerPort{{Name: "port"}},
									DeprecatedStartupProbe:   &corev1.Probe{InitialDelaySeconds: 3},
									DeprecatedLifecycle: &corev1.Lifecycle{PostStart: &corev1.LifecycleHandler{Exec: &corev1.ExecAction{
										Command: []string{"lifecycle command"},
									}}},
									DeprecatedTerminationMessagePath:   "path",
									DeprecatedTerminationMessagePolicy: corev1.TerminationMessagePolicy("policy"),
									DeprecatedStdin:                    true,
									DeprecatedStdinOnce:                true,
									DeprecatedTTY:                      true,
								},
							},
						},
					}, {
						Name: "task-2",
						TaskSpec: &v1beta1.EmbeddedTask{
							TaskSpec: v1beta1.TaskSpec{
								Steps: []v1beta1.Step{{
									DeprecatedLivenessProbe: &corev1.Probe{InitialDelaySeconds: 1},
								}},
								StepTemplate: &v1beta1.StepTemplate{
									DeprecatedLivenessProbe: &corev1.Probe{InitialDelaySeconds: 1},
								},
							},
						},
					}},
				},
			},
		},
	}, {
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
				Params: v1beta1.Params{{
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
						RefSource: &v1beta1.RefSource{
							URI:    "test-uri",
							Digest: map[string]string{"sha256": "digest"},
						},
						FeatureFlags: config.DefaultFeatureFlags.DeepCopy(),
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
				ctx := context.Background()
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
	tests := []struct {
		name string
		in   *v1beta1.PipelineRun
		want *v1beta1.PipelineRun
	}{{
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
						Params: v1beta1.Params{
							{Name: "bundle", Value: v1beta1.ParamValue{StringVal: "test-bundle", Type: "string"}},
							{Name: "name", Value: v1beta1.ParamValue{StringVal: "test-bundle-name", Type: "string"}},
							{Name: "kind", Value: v1beta1.ParamValue{StringVal: "Pipeline", Type: "string"}},
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
				if err := test.in.ConvertTo(context.Background(), ver); err != nil {
					t.Errorf("ConvertTo() = %v", err)
				}
				t.Logf("ConvertTo() = %#v", ver)
				got := &v1beta1.PipelineRun{}
				if err := got.ConvertFrom(context.Background(), ver); err != nil {
					t.Errorf("ConvertFrom() = %v", err)
				}
				t.Logf("ConvertFrom() = %#v", got)
				if d := cmp.Diff(test.want, got, cmpopts.EquateEmpty()); d != "" {
					t.Errorf("roundtrip %s", diff.PrintWantGot(d))
				}
			})
		}
	}
}

func TestPipelineRunConversionRoundTrip(t *testing.T) {
	taskRuns["tr-0"] = trs
	runs["r-0"] = rrs

	tests := []struct {
		name string
		in   *v1beta1.PipelineRun
	}{{
		name: "childReferences",
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
					ChildReferences: append(childRefRuns, childRefTaskRuns...),
				},
			},
		},
	}}
	for _, test := range tests {
		versions := []apis.Convertible{&v1.PipelineRun{}}
		for _, version := range versions {
			t.Run(test.name, func(t *testing.T) {
				ver := version
				ctx := context.Background()

				if err := test.in.ConvertTo(ctx, ver); err != nil {
					t.Errorf("ConvertTo() = %v", err)
				}
				t.Logf("ConvertTo() = %#v", ver)
				got := &v1beta1.PipelineRun{}

				if err := got.ConvertFrom(ctx, ver); err != nil {
					t.Errorf("ConvertFrom() = %v", err)
				}
				t.Logf("ConvertFrom() = %#v", got)
				if d := cmp.Diff(test.in, got, cmpopts.EquateEmpty(), cmpopts.SortSlices(lessChildReferences)); d != "" {
					t.Errorf("roundtrip %s", diff.PrintWantGot(d))
				}
			})
		}
	}
}

func lessChildReferences(i, j v1beta1.ChildStatusReference) bool {
	return i.Name < j.Name
}
