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
	"k8s.io/apimachinery/pkg/api/resource"
	corev1resources "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
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
		},
	}}
	for _, test := range tests {
		versions := []apis.Convertible{&v1.PipelineRun{}}
		for _, version := range versions {
			t.Run(test.name, func(t *testing.T) {
				ver := version
				if err := test.in.ConvertTo(context.Background(), ver); err != nil {
					t.Errorf("ConvertTo() = %v", err)
					return
				}
				t.Logf("ConvertTo() = %#v", ver)
				got := &v1beta1.PipelineRun{}
				if err := got.ConvertFrom(context.Background(), ver); err != nil {
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
					Name: "test-bundle-name",
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
				if d := cmp.Diff(test.want, got); d != "" {
					t.Errorf("roundtrip %s", diff.PrintWantGot(d))
				}
			})
		}
	}
}
