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

	"github.com/tektoncd/pipeline/pkg/apis/version"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/tektoncd/pipeline/pkg/apis/config"
	cfgtesting "github.com/tektoncd/pipeline/pkg/apis/config/testing"
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
				ResourcesResult: []v1beta1.PipelineResourceResult{{
					Key:          "digest",
					Value:        "sha256:1234",
					ResourceName: "source-image",
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
				ctx := cfgtesting.SetEmbeddedStatus(context.Background(), t, config.MinimalEmbeddedStatus)
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

func TestPipelineRunConversionEmbeddedStatusRoundTrip(t *testing.T) {
	taskRuns["tr-0"] = trs
	runs["r-0"] = rrs

	tests := []struct {
		name           string
		in             *v1beta1.PipelineRun
		embeddedStatus string
	}{{
		name: "full embedded-status with child taskruns",
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
		embeddedStatus: config.FullEmbeddedStatus,
	}, {
		name: "full embedded-status with child runs",
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
					Runs: runs,
				},
			},
		},
		embeddedStatus: config.FullEmbeddedStatus,
	}, {
		name: "both embedded-status: runs, taskRuns and childReferences",
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
					TaskRuns:        taskRuns,
					Runs:            runs,
					ChildReferences: childRefRuns,
				},
			},
		},
		embeddedStatus: config.BothEmbeddedStatus,
	}, {
		name: "minimal embedded-status: childReferences",
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
					ChildReferences: childRefRuns,
				},
			},
		},
		embeddedStatus: config.MinimalEmbeddedStatus,
	}}
	for _, test := range tests {
		versions := []apis.Convertible{&v1.PipelineRun{}}
		for _, version := range versions {
			t.Run(test.name, func(t *testing.T) {
				ver := version
				ctx := cfgtesting.SetEmbeddedStatus(context.Background(), t, test.embeddedStatus)

				if err := test.in.ConvertTo(ctx, ver); err != nil {
					t.Errorf("ConvertTo() = %v", err)
				}
				t.Logf("ConvertTo() = %#v", ver)
				got := &v1beta1.PipelineRun{}

				if err := got.ConvertFrom(ctx, ver); err != nil {
					t.Errorf("ConvertFrom() = %v", err)
				}
				t.Logf("ConvertFrom() = %#v", got)
				if d := cmp.Diff(test.in, got, cmpopts.EquateEmpty()); d != "" {
					t.Errorf("roundtrip %s", diff.PrintWantGot(d))
				}
			})
		}
	}
}

func TestPipelineRunConversionEmbeddedStatusConvertTo(t *testing.T) {
	taskRuns["tr-0"] = trs
	runs["r-0"] = rrs
	tests := []struct {
		name           string
		in             *v1beta1.PipelineRun
		want           *v1.PipelineRun
		embeddedStatus string
	}{{
		name: "v1beta1 to v1",
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
					ChildReferences: append(childRefTaskRuns, childRefRuns...),
					TaskRuns:        taskRuns,
					Runs:            runs,
				},
			},
		},
		want: &v1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "foo",
				Namespace: "bar",
				Annotations: map[string]string{
					"tekton.dev/v1beta1Runs":     "H4sIAAAAAAAA/zSNQQvCMAxG/8t37mBee9ejJ2+yQ2QZFmtamlQHo/9duuEpCby8t6EMI/yGHDLHIHwjfV3pzfDIJsMIBzWyqh0qrDWawt83yAEtKcHhQ7H260EFbXLg1QpdAsdZ4aXG2By+T5bzmgurhiSHJEiuBo+ZF6rRht2zR1PmQpYKPIL8C/2pbzvR5wlTm1r7BQAA//8hZzWqxgAAAA==",
					"tekton.dev/v1beta1TaskRuns": "H4sIAAAAAAAA/6RSPW8bMQz9L2/WIbH7MWgrUhfo0qFxp+AG4cTYhM/UQaSSGMb990J359YusnWjRL7Hx0eeYbm5hz9j4IF6FtoGPfwIR4LHYAIHtWBFp5IUL5kUG6lhTdOg8E9n5CLCsquVaiEbxS8GL6XvRweZgUtNU1Fw6JJYYKEMPxE1N/mxdej6VOLmhcTmJhbyjgwee7Ph7k5ZDqsbkV2SyMZJ4PFLDpJe6xBHUg27qgAOmSyfHlIRg78fR/cu6/o/Wdv5h0kfF5anK5plmNNQwY+l64gixauW+BZ6rf72QW2bg+iE23K1cfK0dVcLwdxPU8kd6U/S0tvU4kAneETekRocXkJfar3uw/rTZ79af/iIv8CFbH40fKyzVWLlSF3Ii2jKR5ZgFKsx9Mb2kCLBrypP0MmiTc4p3xh0+fnnMhyeWVj3790KLZCbI5mVNJfcpPH7V/g5ajhibEeH1z3J5m3IpPrHbZah1B1Heg6lt2b2wiENlIOlSs9y8ahCaoR2bMfxdwAAAP//JmZFBycDAAA=",
				},
			},
			Spec: v1.PipelineRunSpec{
				PipelineRef: &v1.PipelineRef{
					Name: "test",
				},
			},
			Status: v1.PipelineRunStatus{
				PipelineRunStatusFields: v1.PipelineRunStatusFields{
					ChildReferences: []v1.ChildStatusReference{{
						TypeMeta:         runtime.TypeMeta{Kind: "TaskRun", APIVersion: "tekton.dev/v1beta1"},
						Name:             "tr-0",
						PipelineTaskName: "ptn",
						WhenExpressions:  []v1.WhenExpression{{Input: "default-value", Operator: "in", Values: []string{"val"}}},
					}, {TypeMeta: runtime.TypeMeta{Kind: "Run", APIVersion: "tekton.dev/v1alpha1"},
						Name:             "r-0",
						PipelineTaskName: "ptn-0",
						WhenExpressions:  []v1.WhenExpression{{Input: "default-value-0", Operator: "in", Values: []string{"val-0", "val-1"}}},
					}},
				},
			},
		},
	}}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			v1PipelineRun := &v1.PipelineRun{}
			ctx := cfgtesting.SetEmbeddedStatus(context.Background(), t, config.FullEmbeddedStatus)
			if err := test.in.ConvertTo(ctx, v1PipelineRun); err != nil {
				t.Errorf("ConvertTo() = %v", err)
			}
			t.Logf("ConvertTo() = %#v", v1PipelineRun)
			if d := cmp.Diff(test.want, v1PipelineRun); d != "" {
				t.Errorf("v1beta1 ConvertTo v1 %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestPipelineRunConversionEmbeddedStatusConvertFrom(t *testing.T) {
	taskRuns["tr-0"] = trs
	runs["r-0"] = rrs
	childRefs := []v1.ChildStatusReference{{
		TypeMeta:         runtime.TypeMeta{Kind: "TaskRun", APIVersion: "tekton.dev/v1beta1"},
		Name:             "tr-0",
		PipelineTaskName: "ptn",
		WhenExpressions:  []v1.WhenExpression{{Input: "default-value", Operator: "in", Values: []string{"val"}}},
	}, {TypeMeta: runtime.TypeMeta{Kind: "Run", APIVersion: "tekton.dev/v1alpha1"},
		Name:             "r-0",
		PipelineTaskName: "ptn-0",
		WhenExpressions:  []v1.WhenExpression{{Input: "default-value-0", Operator: "in", Values: []string{"val-0", "val-1"}}},
	}}

	v1beta1RunsAnnotation, err := version.CompressAndEncode([]byte(`{"r-0":{"pipelineTaskName":"ptn-0","status":{"results":[{"name":"foo","value":"bar"}],"extraFields":null},"whenExpressions":[{"input":"default-value-0","operator":"in","values":["val-0","val-1"]}]}}`))
	if err != nil {
		t.Errorf(err.Error())
	}
	v1beta1TaskRunsAnnotation, err := version.CompressAndEncode([]byte(`{"tr-0":{"pipelineTaskName":"ptn","status":{"podName":"pod-name","steps":[{"running":{"startedAt":null},"name":"running-step","container":"step-running-step"}],"cloudEvents":[{"target":"http//sink1","status":{"condition":"Unknown","message":"","retryCount":0}},{"target":"http//sink2","status":{"condition":"Unknown","message":"","retryCount":0}}],"retriesStatus":[{"conditions":[{"type":"Succeeded","status":"False","lastTransitionTime":null}],"podName":""}],"resourcesResult":[{"key":"digest","value":"sha256:1234","resourceName":"source-image"}],"sidecars":[{"terminated":{"exitCode":1,"reason":"Error","message":"Error","startedAt":null,"finishedAt":null},"name":"error","container":"sidecar-error","imageID":"image-id"}]},"whenExpressions":[{"input":"default-value","operator":"in","values":["val"]}]}}`))
	if err != nil {
		t.Errorf(err.Error())
	}

	annotations := map[string]string{
		"tekton.dev/v1beta1Runs":     v1beta1RunsAnnotation,
		"tekton.dev/v1beta1TaskRuns": v1beta1TaskRunsAnnotation,
	}

	tests := []struct {
		name           string
		in             *v1.PipelineRun
		want           *v1beta1.PipelineRun
		embeddedStatus string
	}{{
		name: "both",
		in: &v1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "foo",
				Namespace:   "bar",
				Annotations: annotations,
			},
			Spec: v1.PipelineRunSpec{
				PipelineRef: &v1.PipelineRef{
					Name: "test",
				},
			},
			Status: v1.PipelineRunStatus{
				PipelineRunStatusFields: v1.PipelineRunStatusFields{
					ChildReferences: childRefs,
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
					ChildReferences: append(childRefTaskRuns, childRefRuns...),
					TaskRuns:        taskRuns,
					Runs:            runs,
				},
			},
		},
		embeddedStatus: config.BothEmbeddedStatus,
	}, {
		name: "minimal",
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
					ChildReferences: childRefs,
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
					ChildReferences: append(childRefTaskRuns, childRefRuns...),
				},
			},
		},
		embeddedStatus: config.MinimalEmbeddedStatus,
	}, {
		name: "full",
		in: &v1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "foo",
				Namespace:   "bar",
				Annotations: annotations,
			},
			Spec: v1.PipelineRunSpec{
				PipelineRef: &v1.PipelineRef{
					Name: "test",
				},
			},
			Status: v1.PipelineRunStatus{
				PipelineRunStatusFields: v1.PipelineRunStatusFields{
					ChildReferences: childRefs,
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
					TaskRuns: taskRuns,
					Runs:     runs,
				},
			},
		},
		embeddedStatus: config.FullEmbeddedStatus,
	}}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			v1beta1PipelineRun := &v1beta1.PipelineRun{}
			ctx := cfgtesting.SetEmbeddedStatus(context.Background(), t, test.embeddedStatus)
			if err := v1beta1PipelineRun.ConvertFrom(ctx, test.in.DeepCopy()); err != nil {
				t.Errorf("ConvertFrom() = %v", err)
			}
			t.Logf("ConvertFrom() = %#v", v1beta1PipelineRun)
			if d := cmp.Diff(test.want, v1beta1PipelineRun, cmpopts.EquateEmpty()); d != "" {
				t.Errorf("v1beta1 ConvertFrom v1 %s", diff.PrintWantGot(d))
			}
		})
	}
}
