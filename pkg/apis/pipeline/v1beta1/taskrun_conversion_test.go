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
	"github.com/tektoncd/pipeline/test/diff"
	corev1 "k8s.io/api/core/v1"
	corev1resources "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

const (
	breakpointOnFailure = "onFailure"
)

func TestTaskRunConversionBadType(t *testing.T) {
	good, bad := &v1beta1.TaskRun{}, &v1beta1.Task{}

	if err := good.ConvertTo(context.Background(), bad); err == nil {
		t.Errorf("ConvertTo() = %#v, wanted error", bad)
	}

	if err := good.ConvertFrom(context.Background(), bad); err == nil {
		t.Errorf("ConvertFrom() = %#v, wanted error", good)
	}
}

func TestTaskRunConversion(t *testing.T) {
	tests := []struct {
		name string
		in   *v1beta1.TaskRun
	}{{
		name: "simple taskrun",
		in: &v1beta1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "foo",
				Namespace: "bar",
			},
			Spec: v1beta1.TaskRunSpec{
				TaskRef: &v1beta1.TaskRef{Name: "test-task"},
			},
		},
	}, {
		name: "taskrun conversion deprecated step fields",
		in: &v1beta1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "foo",
				Namespace: "bar",
			},
			Spec: v1beta1.TaskRunSpec{
				TaskSpec: &v1beta1.TaskSpec{
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
						DeprecatedName:           "name",
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
		},
	}, {
		name: "taskrun conversion all non deprecated fields",
		in: &v1beta1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "foo",
				Namespace: "bar",
			},
			Spec: v1beta1.TaskRunSpec{
				Debug: &v1beta1.TaskRunDebug{
					Breakpoint: []string{breakpointOnFailure},
				},
				Params: v1beta1.Params{{
					Name: "param-task-1",
					Value: v1beta1.ParamValue{
						ArrayVal: []string{"value-task-1"},
						Type:     "string",
					},
				}},
				ServiceAccountName: "test-sa",
				TaskRef:            &v1beta1.TaskRef{Name: "test-task"},
				TaskSpec: &v1beta1.TaskSpec{
					Params: []v1beta1.ParamSpec{{
						Name: "param-name",
						Type: "string",
					}},
				},
				Status:        "test-task-run-spec-status",
				StatusMessage: v1beta1.TaskRunSpecStatusMessage("test-status-message"),
				Timeout:       &metav1.Duration{Duration: 5 * time.Second},
				PodTemplate: &pod.Template{
					NodeSelector: map[string]string{
						"label": "value",
					},
				},
				Workspaces: []v1beta1.WorkspaceBinding{{
					Name:    "workspace-volumeclaimtemplate",
					SubPath: "/foo/bar/baz",
					VolumeClaimTemplate: &corev1.PersistentVolumeClaim{
						ObjectMeta: metav1.ObjectMeta{
							Name: "pvc",
						},
						Spec: corev1.PersistentVolumeClaimSpec{},
					},
				}, {
					Name:                  "workspace-pvc",
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{},
				}, {
					Name:     "workspace-emptydir",
					EmptyDir: &corev1.EmptyDirVolumeSource{},
				}, {
					Name: "workspace-configmap",
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "configbar",
						},
					},
				}, {
					Name:   "workspace-secret",
					Secret: &corev1.SecretVolumeSource{SecretName: "sname"},
				}, {
					Name: "workspace-projected",
					Projected: &corev1.ProjectedVolumeSource{
						Sources: []corev1.VolumeProjection{{
							ConfigMap: &corev1.ConfigMapProjection{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: "projected-configmap",
								},
							},
							Secret: &corev1.SecretProjection{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: "projected-secret",
								},
							},
							ServiceAccountToken: &corev1.ServiceAccountTokenProjection{
								Audience: "projected-sat",
							},
						}},
					},
				}, {
					Name: "workspace-csi",
					CSI: &corev1.CSIVolumeSource{
						NodePublishSecretRef: &corev1.LocalObjectReference{
							Name: "projected-csi",
						},
						VolumeAttributes: map[string]string{"key": "attribute-val"},
					},
				},
				},
				StepOverrides: []v1beta1.TaskRunStepOverride{{
					Name: "task-1",
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{corev1.ResourceMemory: corev1resources.MustParse("1Gi")},
					}},
				},
				SidecarOverrides: []v1beta1.TaskRunSidecarOverride{{
					Name: "task-1",
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{corev1.ResourceMemory: corev1resources.MustParse("1Gi")},
					}},
				},
				ComputeResources: &corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceMemory: corev1resources.MustParse("1Gi"),
					},
				},
			},
			Status: v1beta1.TaskRunStatus{
				Status: duckv1.Status{
					Conditions: []apis.Condition{
						{
							Type:    apis.ConditionSucceeded,
							Status:  corev1.ConditionTrue,
							Reason:  "Completed",
							Message: "All tasks finished running",
						},
					},
					ObservedGeneration: 1,
				},
				TaskRunStatusFields: v1beta1.TaskRunStatusFields{
					PodName:        "pod-name",
					StartTime:      &metav1.Time{Time: time.Now()},
					CompletionTime: &metav1.Time{Time: time.Now().Add(1 * time.Minute)},
					Steps: []v1beta1.StepState{{
						ContainerState: corev1.ContainerState{
							Terminated: &corev1.ContainerStateTerminated{
								ExitCode: 123,
							}},

						Name:          "failure",
						ContainerName: "step-failure",
						ImageID:       "image-id",
					}},
					Sidecars: []v1beta1.SidecarState{{
						ContainerState: corev1.ContainerState{
							Terminated: &corev1.ContainerStateTerminated{
								ExitCode: 123,
							}},

						Name:          "failure",
						ContainerName: "step-failure",
						ImageID:       "image-id",
					}},
					RetriesStatus: []v1beta1.TaskRunStatus{{
						Status: duckv1.Status{
							Conditions: []apis.Condition{{
								Type:   apis.ConditionSucceeded,
								Status: corev1.ConditionFalse,
							}},
						},
					}},
					TaskRunResults: []v1beta1.TaskRunResult{{
						Name:  "resultName",
						Type:  v1beta1.ResultsTypeObject,
						Value: *v1beta1.NewObject(map[string]string{"hello": "world"}),
					}},
					TaskSpec: &v1beta1.TaskSpec{
						Description: "test",
						Steps: []v1beta1.Step{{
							Image: "foo",
						}},
						Volumes: []corev1.Volume{{}},
						Params: []v1beta1.ParamSpec{{
							Name:        "param-1",
							Type:        v1beta1.ParamTypeString,
							Description: "My first param",
						}},
					},
					Provenance: &v1beta1.Provenance{
						RefSource: &v1beta1.RefSource{
							URI:    "test-uri",
							Digest: map[string]string{"sha256": "digest"},
						},
						FeatureFlags: config.DefaultFeatureFlags.DeepCopy(),
					}},
			},
		},
	}}

	for _, test := range tests {
		versions := []apis.Convertible{&v1.TaskRun{}}
		for _, version := range versions {
			t.Run(test.name, func(t *testing.T) {
				ver := version
				if err := test.in.ConvertTo(context.Background(), ver); err != nil {
					t.Errorf("ConvertTo() = %v", err)
					return
				}
				t.Logf("ConvertTo() =%v", ver)
				got := &v1beta1.TaskRun{}
				if err := got.ConvertFrom(context.Background(), ver); err != nil {
					t.Errorf("ConvertFrom() = %v", err)
				}
				t.Logf("ConvertFrom() =%v", got)
				if d := cmp.Diff(test.in, got); d != "" {
					t.Errorf("roundtrip %s", diff.PrintWantGot(d))
				}
			})
		}
	}
}

func TestTaskRunConversionFromDeprecated(t *testing.T) {
	tests := []struct {
		name string
		in   *v1beta1.TaskRun
		want *v1beta1.TaskRun
	}{{
		name: "bundle",
		in: &v1beta1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "foo",
				Namespace: "bar",
			},
			Spec: v1beta1.TaskRunSpec{
				TaskRef: &v1beta1.TaskRef{
					Name:   "test-bundle-name",
					Bundle: "test-bundle",
				},
			},
		},
		want: &v1beta1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "foo",
				Namespace: "bar",
			},
			Spec: v1beta1.TaskRunSpec{
				TaskRef: &v1beta1.TaskRef{
					ResolverRef: v1beta1.ResolverRef{
						Resolver: "bundles",
						Params: v1beta1.Params{
							{Name: "bundle", Value: v1beta1.ParamValue{StringVal: "test-bundle", Type: "string"}},
							{Name: "name", Value: v1beta1.ParamValue{StringVal: "test-bundle-name", Type: "string"}},
							{Name: "kind", Value: v1beta1.ParamValue{StringVal: "Task", Type: "string"}},
						},
					},
				},
			},
		},
	}, {
		name: "cloudEvents",
		in: &v1beta1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "foo",
				Namespace: "bar",
			},
			Spec: v1beta1.TaskRunSpec{
				TaskRef: &v1beta1.TaskRef{
					Name: "test-cloud-events",
				},
			},
			Status: v1beta1.TaskRunStatus{
				TaskRunStatusFields: v1beta1.TaskRunStatusFields{
					CloudEvents: []v1beta1.CloudEventDelivery{
						{
							Target: "http//attemptedfailed",
							Status: v1beta1.CloudEventDeliveryState{
								Condition:  v1beta1.CloudEventConditionFailed,
								Error:      "iknewit",
								RetryCount: 1,
							},
						},
						{
							Target: "http//attemptedsucceeded",
							Status: v1beta1.CloudEventDeliveryState{
								Condition:  v1beta1.CloudEventConditionSent,
								RetryCount: 1,
							},
						},
					},
				},
			},
		},
		want: &v1beta1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "foo",
				Namespace: "bar",
			},
			Spec: v1beta1.TaskRunSpec{
				TaskRef: &v1beta1.TaskRef{
					Name: "test-cloud-events",
				},
			},
			Status: v1beta1.TaskRunStatus{
				TaskRunStatusFields: v1beta1.TaskRunStatusFields{
					CloudEvents: []v1beta1.CloudEventDelivery{
						{
							Target: "http//attemptedfailed",
							Status: v1beta1.CloudEventDeliveryState{
								Condition:  v1beta1.CloudEventConditionFailed,
								Error:      "iknewit",
								RetryCount: 1,
							},
						},
						{
							Target: "http//attemptedsucceeded",
							Status: v1beta1.CloudEventDeliveryState{
								Condition:  v1beta1.CloudEventConditionSent,
								RetryCount: 1,
							},
						},
					},
				},
			},
		},
	}}
	for _, test := range tests {
		versions := []apis.Convertible{&v1.TaskRun{}}
		for _, version := range versions {
			t.Run(test.name, func(t *testing.T) {
				ver := version
				if err := test.in.ConvertTo(context.Background(), ver); err != nil {
					t.Errorf("ConvertTo() = %v", err)
				}
				t.Logf("ConvertTo() = %#v", ver)
				got := &v1beta1.TaskRun{}
				if err := got.ConvertFrom(context.Background(), ver); err != nil {
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

func TestTaskRunConvertTo(t *testing.T) {
	tests := []struct {
		name string
		in   *v1beta1.TaskRun
		want *v1.TaskRun
	}{{
		name: "empty param string",
		in: &v1beta1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "foo",
				Namespace: "bar",
			},
			Spec: v1beta1.TaskRunSpec{
				Retries: 1,
				Params: v1beta1.Params{{
					Name: "param-task-0",
					Value: v1beta1.ParamValue{
						StringVal: "param-value-string",
					},
				}, {
					Name: "param-task-1",
					Value: v1beta1.ParamValue{
						ArrayVal: []string{"param-value-string"},
						Type:     "array",
					},
				}},
				TaskSpec: &v1beta1.TaskSpec{
					Params: []v1beta1.ParamSpec{{
						Name: "param-name",
					}, {
						Name: "param-array",
						Type: "array",
					}},
				},
			},
		},
		want: &v1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "foo",
				Namespace: "bar",
			},
			Spec: v1.TaskRunSpec{
				Retries: 1,
				Params: v1.Params{{
					Name: "param-task-0",
					Value: v1.ParamValue{
						StringVal: "param-value-string",
						Type:      "string",
					},
				}, {
					Name: "param-task-1",
					Value: v1.ParamValue{
						ArrayVal: []string{"param-value-string"},
						Type:     "array",
					},
				}},
				TaskSpec: &v1.TaskSpec{
					Params: []v1.ParamSpec{{
						Name: "param-name",
						Type: "string",
					}, {
						Name: "param-array",
						Type: "array",
					}},
				},
			},
		}}}
	for _, test := range tests {
		versions := []apis.Convertible{&v1.TaskRun{}}
		for _, version := range versions {
			t.Run(test.name, func(t *testing.T) {
				ver := version
				if err := test.in.ConvertTo(context.Background(), ver); err != nil {
					t.Errorf("ConvertTo() = %v", err)
				}
				if d := cmp.Diff(test.want, ver); d != "" {
					t.Errorf("ConvertTo() = %v", diff.PrintWantGot(d))
				}
			})
		}
	}
}

func TestTaskRunConvertFrom(t *testing.T) {
	tests := []struct {
		name string
		in   *v1.TaskRun
		want *v1beta1.TaskRun
	}{{
		name: "empty param string",
		in: &v1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "foo",
				Namespace: "bar",
			},
			Spec: v1.TaskRunSpec{
				Retries: 1,
				Params: v1.Params{{
					Name: "param-task-1",
					Value: v1.ParamValue{
						ArrayVal: []string{"value-task-1"},
					},
				}},
				TaskSpec: &v1.TaskSpec{
					Params: []v1.ParamSpec{{
						Name: "param-name",
					}},
				},
			},
		},
		want: &v1beta1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "foo",
				Namespace: "bar",
			},
			Spec: v1beta1.TaskRunSpec{
				Retries: 1,
				Params: v1beta1.Params{{
					Name: "param-task-1",
					Value: v1beta1.ParamValue{
						ArrayVal: []string{"value-task-1"},
						Type:     "string",
					},
				}},
				TaskSpec: &v1beta1.TaskSpec{
					Params: []v1beta1.ParamSpec{{
						Name: "param-name",
						Type: "string",
					}},
				},
			},
		}}}
	for _, test := range tests {
		versions := []apis.Convertible{&v1beta1.TaskRun{}}
		for _, version := range versions {
			t.Run(test.name, func(t *testing.T) {
				got := version
				if err := got.ConvertFrom(context.Background(), test.in); err != nil {
					t.Errorf("ConvertFrom() = %v", err)
				}
				t.Logf("ConvertFrom() =%v", got)
				if d := cmp.Diff(test.want, got); d != "" {
					t.Errorf("roundtrip %s", diff.PrintWantGot(d))
				}
			})
		}
	}
}
