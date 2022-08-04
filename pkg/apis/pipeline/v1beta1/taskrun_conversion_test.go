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
	pod "github.com/tektoncd/pipeline/pkg/apis/pipeline/pod"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/test/diff"
	corev1 "k8s.io/api/core/v1"
	corev1resources "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
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

func TestTaskrunConversion(t *testing.T) {
	versions := []apis.Convertible{&v1.TaskRun{}}

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
				Params: []v1beta1.Param{{
					Name: "param-task-1",
					Value: v1beta1.ParamValue{
						ArrayVal: []string{"value-task-1"},
					},
				}},
				ServiceAccountName: "test-sa",
				TaskRef:            &v1beta1.TaskRef{Name: "test-task"},
				TaskSpec: &v1beta1.TaskSpec{
					Params: []v1beta1.ParamSpec{{
						Name: "param-name",
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
		},
	}}

	for _, test := range tests {
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
	// TODO(#4546): We're just dropping Resources when converting from
	// v1beta1 to v1. Before moving the stored version to v1, we should
	// come up with a better strategy
	versions := []apis.Convertible{&v1.TaskRun{}}
	tests := []struct {
		name string
		in   *v1beta1.TaskRun
		want *v1beta1.TaskRun
	}{{
		name: "input resources",
		in: &v1beta1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "foo",
				Namespace: "bar",
			},
			Spec: v1beta1.TaskRunSpec{
				Resources: &v1beta1.TaskRunResources{
					Inputs: []v1beta1.TaskResourceBinding{{
						PipelineResourceBinding: v1beta1.PipelineResourceBinding{
							ResourceRef: &v1beta1.PipelineResourceRef{
								Name: "the-git-with-branch",
							},
							Name: "gitspace",
						},
						Paths: []string{"test-path"},
					}},
				},
			},
		},
		want: &v1beta1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "foo",
				Namespace: "bar",
			},
			Spec: v1beta1.TaskRunSpec{},
		},
	}, {
		name: "output resources",
		in: &v1beta1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "foo",
				Namespace: "bar",
			},
			Spec: v1beta1.TaskRunSpec{
				Resources: &v1beta1.TaskRunResources{
					Outputs: []v1beta1.TaskResourceBinding{{
						PipelineResourceBinding: v1beta1.PipelineResourceBinding{
							ResourceRef: &v1beta1.PipelineResourceRef{
								Name: "the-git-with-branch",
							},
							Name: "gitspace",
						},
						Paths: []string{"test-path"},
					}},
				},
			},
		},
		want: &v1beta1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "foo",
				Namespace: "bar",
			},
			Spec: v1beta1.TaskRunSpec{},
		},
	}}
	for _, test := range tests {
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
