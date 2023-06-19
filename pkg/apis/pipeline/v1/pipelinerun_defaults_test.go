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

package v1_test

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/tektoncd/pipeline/pkg/apis/config"
	cfgtesting "github.com/tektoncd/pipeline/pkg/apis/config/testing"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/pod"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/test/diff"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
)

func TestPipelineRunSpec_SetDefaults(t *testing.T) {
	cases := []struct {
		desc string
		prs  *v1.PipelineRunSpec
		want *v1.PipelineRunSpec
	}{
		{
			desc: "timeouts is nil",
			prs:  &v1.PipelineRunSpec{},
			want: &v1.PipelineRunSpec{
				TaskRunTemplate: v1.PipelineTaskRunTemplate{
					ServiceAccountName: config.DefaultServiceAccountValue,
				},
				Timeouts: &v1.TimeoutFields{
					Pipeline: &metav1.Duration{Duration: config.DefaultTimeoutMinutes * time.Minute},
				},
			},
		},
		{
			desc: "timeouts is not nil",
			prs: &v1.PipelineRunSpec{
				Timeouts: &v1.TimeoutFields{},
			},
			want: &v1.PipelineRunSpec{
				TaskRunTemplate: v1.PipelineTaskRunTemplate{
					ServiceAccountName: config.DefaultServiceAccountValue,
				},
				Timeouts: &v1.TimeoutFields{
					Pipeline: &metav1.Duration{Duration: config.DefaultTimeoutMinutes * time.Minute},
				},
			},
		},
		{
			desc: "timeouts.pipeline is not nil",
			prs: &v1.PipelineRunSpec{
				Timeouts: &v1.TimeoutFields{
					Pipeline: &metav1.Duration{Duration: (config.DefaultTimeoutMinutes + 1) * time.Minute},
				},
			},
			want: &v1.PipelineRunSpec{
				TaskRunTemplate: v1.PipelineTaskRunTemplate{
					ServiceAccountName: config.DefaultServiceAccountValue,
				},
				Timeouts: &v1.TimeoutFields{
					Pipeline: &metav1.Duration{Duration: (config.DefaultTimeoutMinutes + 1) * time.Minute},
				},
			},
		},
		{
			desc: "timeouts.pipeline is nil with timeouts.tasks and timeouts.finally",
			prs: &v1.PipelineRunSpec{
				Timeouts: &v1.TimeoutFields{
					Tasks:   &metav1.Duration{Duration: (config.DefaultTimeoutMinutes + 1) * time.Minute},
					Finally: &metav1.Duration{Duration: (config.DefaultTimeoutMinutes + 1) * time.Minute},
				},
			},
			want: &v1.PipelineRunSpec{
				TaskRunTemplate: v1.PipelineTaskRunTemplate{
					ServiceAccountName: config.DefaultServiceAccountValue,
				},
				Timeouts: &v1.TimeoutFields{
					Pipeline: &metav1.Duration{Duration: (config.DefaultTimeoutMinutes) * time.Minute},
					Tasks:    &metav1.Duration{Duration: (config.DefaultTimeoutMinutes + 1) * time.Minute},
					Finally:  &metav1.Duration{Duration: (config.DefaultTimeoutMinutes + 1) * time.Minute},
				},
			},
		},
		{
			desc: "pod template is nil",
			prs:  &v1.PipelineRunSpec{},
			want: &v1.PipelineRunSpec{
				TaskRunTemplate: v1.PipelineTaskRunTemplate{
					ServiceAccountName: config.DefaultServiceAccountValue,
				},
				Timeouts: &v1.TimeoutFields{
					Pipeline: &metav1.Duration{Duration: config.DefaultTimeoutMinutes * time.Minute},
				},
			},
		},
		{
			desc: "pod template is not nil",
			prs: &v1.PipelineRunSpec{
				TaskRunTemplate: v1.PipelineTaskRunTemplate{
					PodTemplate: &pod.Template{
						NodeSelector: map[string]string{
							"label": "value",
						},
					},
				},
			},
			want: &v1.PipelineRunSpec{
				TaskRunTemplate: v1.PipelineTaskRunTemplate{
					ServiceAccountName: config.DefaultServiceAccountValue,
					PodTemplate: &pod.Template{
						NodeSelector: map[string]string{
							"label": "value",
						},
					},
				},
				Timeouts: &v1.TimeoutFields{
					Pipeline: &metav1.Duration{Duration: config.DefaultTimeoutMinutes * time.Minute},
				},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			ctx := context.Background()
			tc.prs.SetDefaults(ctx)

			sortParamSpecs := func(x, y v1.ParamSpec) bool {
				return x.Name < y.Name
			}
			sortParams := func(x, y v1.Param) bool {
				return x.Name < y.Name
			}
			sortTasks := func(x, y v1.PipelineTask) bool {
				return x.Name < y.Name
			}
			if d := cmp.Diff(tc.want, tc.prs, cmpopts.SortSlices(sortParamSpecs), cmpopts.SortSlices(sortParams), cmpopts.SortSlices(sortTasks)); d != "" {
				t.Errorf("Mismatch of PipelineRunSpec %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestPipelineRunDefaulting(t *testing.T) {
	const defaultTimeoutMinutes = 5
	tests := []struct {
		name     string
		in       *v1.PipelineRun
		want     *v1.PipelineRun
		defaults map[string]string
	}{{
		name: "empty no context",
		in:   &v1.PipelineRun{},
		want: &v1.PipelineRun{
			Spec: v1.PipelineRunSpec{
				TaskRunTemplate: v1.PipelineTaskRunTemplate{
					ServiceAccountName: config.DefaultServiceAccountValue,
				},
				Timeouts: &v1.TimeoutFields{
					Pipeline: &metav1.Duration{Duration: config.DefaultTimeoutMinutes * time.Minute},
				},
			},
		},
	}, {
		name: "Embedded PipelineSpec default",
		in: &v1.PipelineRun{
			Spec: v1.PipelineRunSpec{
				PipelineSpec: &v1.PipelineSpec{
					Params: []v1.ParamSpec{{
						Name: "foo",
					}},
				},
			},
		},
		want: &v1.PipelineRun{
			Spec: v1.PipelineRunSpec{
				PipelineSpec: &v1.PipelineSpec{
					Params: []v1.ParamSpec{{
						Name: "foo",
						Type: "string",
					}},
				},
				TaskRunTemplate: v1.PipelineTaskRunTemplate{
					ServiceAccountName: config.DefaultServiceAccountValue,
				},
				Timeouts: &v1.TimeoutFields{
					Pipeline: &metav1.Duration{Duration: config.DefaultTimeoutMinutes * time.Minute},
				},
			},
		},
	}, {
		name: "PipelineRef default config context",
		in: &v1.PipelineRun{
			Spec: v1.PipelineRunSpec{
				PipelineRef: &v1.PipelineRef{Name: "foo"},
			},
		},
		want: &v1.PipelineRun{
			Spec: v1.PipelineRunSpec{
				PipelineRef: &v1.PipelineRef{Name: "foo"},
				TaskRunTemplate: v1.PipelineTaskRunTemplate{
					ServiceAccountName: config.DefaultServiceAccountValue,
				},
				Timeouts: &v1.TimeoutFields{
					Pipeline: &metav1.Duration{Duration: config.DefaultTimeoutMinutes * time.Minute},
				},
			},
		},
	}, {
		name: "PipelineRef default config context with sa",
		in: &v1.PipelineRun{
			Spec: v1.PipelineRunSpec{
				PipelineRef: &v1.PipelineRef{Name: "foo"},
			},
		},
		want: &v1.PipelineRun{
			Spec: v1.PipelineRunSpec{
				PipelineRef: &v1.PipelineRef{Name: "foo"},
				TaskRunTemplate: v1.PipelineTaskRunTemplate{
					ServiceAccountName: "tekton",
				},
				Timeouts: &v1.TimeoutFields{
					Pipeline: &metav1.Duration{Duration: defaultTimeoutMinutes * time.Minute},
				},
			},
		},
		defaults: map[string]string{
			"default-timeout-minutes": strconv.Itoa(defaultTimeoutMinutes),
			"default-service-account": "tekton",
		},
	}, {
		name: "PipelineRef pod template is coming from default config pod template",
		in: &v1.PipelineRun{
			Spec: v1.PipelineRunSpec{
				PipelineRef: &v1.PipelineRef{Name: "foo"},
			},
		},
		want: &v1.PipelineRun{
			Spec: v1.PipelineRunSpec{
				PipelineRef: &v1.PipelineRef{Name: "foo"},
				TaskRunTemplate: v1.PipelineTaskRunTemplate{
					ServiceAccountName: "tekton",
					PodTemplate: &pod.Template{
						NodeSelector: map[string]string{
							"label": "value",
						},
					},
				},
				Timeouts: &v1.TimeoutFields{
					Pipeline: &metav1.Duration{Duration: defaultTimeoutMinutes * time.Minute},
				},
			},
		},
		defaults: map[string]string{
			"default-timeout-minutes": strconv.Itoa(defaultTimeoutMinutes),
			"default-service-account": "tekton",
			"default-pod-template":    "nodeSelector: { 'label': 'value' }",
		},
	}, {
		name: "PipelineRef pod template nodeselector takes precedence over default config pod template nodeselector",
		in: &v1.PipelineRun{
			Spec: v1.PipelineRunSpec{
				PipelineRef: &v1.PipelineRef{Name: "foo"},
				TaskRunTemplate: v1.PipelineTaskRunTemplate{
					PodTemplate: &pod.Template{
						NodeSelector: map[string]string{
							"label2": "value2",
						},
					},
				},
			},
		},
		want: &v1.PipelineRun{
			Spec: v1.PipelineRunSpec{
				PipelineRef: &v1.PipelineRef{Name: "foo"},
				TaskRunTemplate: v1.PipelineTaskRunTemplate{
					ServiceAccountName: "tekton",
					PodTemplate: &pod.Template{
						NodeSelector: map[string]string{
							"label2": "value2",
						},
					},
				},
				Timeouts: &v1.TimeoutFields{
					Pipeline: &metav1.Duration{Duration: defaultTimeoutMinutes * time.Minute},
				},
			},
		},
		defaults: map[string]string{
			"default-timeout-minutes": strconv.Itoa(defaultTimeoutMinutes),
			"default-service-account": "tekton",
			"default-pod-template":    "nodeSelector: { 'label': 'value' }",
		},
	}, {
		name: "PipelineRef pod template merges non competing fields with default config pod template",
		in: &v1.PipelineRun{
			Spec: v1.PipelineRunSpec{
				PipelineRef: &v1.PipelineRef{Name: "foo"},
				TaskRunTemplate: v1.PipelineTaskRunTemplate{
					PodTemplate: &pod.Template{
						NodeSelector: map[string]string{
							"label2": "value2",
						},
						SecurityContext: &corev1.PodSecurityContext{
							RunAsNonRoot: &ttrue,
						},
						HostNetwork: false,
					},
				},
			},
		},
		want: &v1.PipelineRun{
			Spec: v1.PipelineRunSpec{
				PipelineRef: &v1.PipelineRef{Name: "foo"},
				TaskRunTemplate: v1.PipelineTaskRunTemplate{
					ServiceAccountName: "tekton",
					PodTemplate: &pod.Template{
						NodeSelector: map[string]string{
							"label2": "value2",
						},
						SecurityContext: &corev1.PodSecurityContext{
							RunAsNonRoot: &ttrue,
						},
						HostNetwork: true,
					},
				},
				Timeouts: &v1.TimeoutFields{
					Pipeline: &metav1.Duration{Duration: defaultTimeoutMinutes * time.Minute},
				},
			},
		},
		defaults: map[string]string{
			"default-timeout-minutes": strconv.Itoa(defaultTimeoutMinutes),
			"default-service-account": "tekton",
			"default-pod-template":    "nodeSelector: { 'label': 'value' }\nhostNetwork: true",
		},
	}, {
		name: "PipelineRef uses default resolver",
		in:   &v1.PipelineRun{Spec: v1.PipelineRunSpec{PipelineRef: &v1.PipelineRef{}}},
		want: &v1.PipelineRun{
			Spec: v1.PipelineRunSpec{
				TaskRunTemplate: v1.PipelineTaskRunTemplate{
					ServiceAccountName: config.DefaultServiceAccountValue,
				},
				Timeouts: &v1.TimeoutFields{
					Pipeline: &metav1.Duration{Duration: config.DefaultTimeoutMinutes * time.Minute},
				},
				PipelineRef: &v1.PipelineRef{
					ResolverRef: v1.ResolverRef{
						Resolver: "git",
					},
				},
			},
		},
		defaults: map[string]string{
			"default-resolver-type": "git",
		},
	}, {
		name: "PipelineRef user-provided resolver overwrites default resolver",
		in: &v1.PipelineRun{
			Spec: v1.PipelineRunSpec{
				PipelineRef: &v1.PipelineRef{
					ResolverRef: v1.ResolverRef{
						Resolver: "hub",
					},
				},
			},
		},
		want: &v1.PipelineRun{
			Spec: v1.PipelineRunSpec{
				TaskRunTemplate: v1.PipelineTaskRunTemplate{
					ServiceAccountName: config.DefaultServiceAccountValue,
				},
				Timeouts: &v1.TimeoutFields{
					Pipeline: &metav1.Duration{Duration: config.DefaultTimeoutMinutes * time.Minute},
				},
				PipelineRef: &v1.PipelineRef{
					ResolverRef: v1.ResolverRef{
						Resolver: "hub",
					},
				},
			},
		},
		defaults: map[string]string{
			"default-resolver-type": "git",
		},
	}, {
		name: "Reserved Tekton annotations are not filtered on update",
		in: &v1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{"chains.tekton.dev/signed": "true", "foo": "bar"},
			},
			Spec: v1.PipelineRunSpec{
				PipelineRef: &v1.PipelineRef{
					Name: "foo",
				},
			},
		},
		want: &v1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{"chains.tekton.dev/signed": "true", "foo": "bar"},
			},
			Spec: v1.PipelineRunSpec{
				TaskRunTemplate: v1.PipelineTaskRunTemplate{
					ServiceAccountName: config.DefaultServiceAccountValue,
				},
				Timeouts: &v1.TimeoutFields{
					Pipeline: &metav1.Duration{Duration: config.DefaultTimeoutMinutes * time.Minute},
				},
				PipelineRef: &v1.PipelineRef{
					Name: "foo",
				},
			},
		},
	}}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := cfgtesting.SetDefaults(context.Background(), t, tc.defaults)
			got := tc.in
			got.SetDefaults(ctx)
			if !cmp.Equal(got, tc.want, ignoreUnexportedResources) {
				d := cmp.Diff(got, tc.want, ignoreUnexportedResources)
				t.Errorf("SetDefaults %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestPipelineRunDefaultingOnCreate(t *testing.T) {
	tests := []struct {
		name     string
		in       *v1.PipelineRun
		want     *v1.PipelineRun
		defaults map[string]string
	}{{
		name: "Reserved Tekton annotations are filtered on create",
		in: &v1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{"chains.tekton.dev/signed": "true", "results.tekton.dev/hello": "world", "tekton.dev/foo": "bar", "foo": "bar"},
			},
			Spec: v1.PipelineRunSpec{
				PipelineRef: &v1.PipelineRef{
					Name: "foo",
				},
			},
		},
		want: &v1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{"tekton.dev/foo": "bar", "foo": "bar"},
			},
			Spec: v1.PipelineRunSpec{
				TaskRunTemplate: v1.PipelineTaskRunTemplate{
					ServiceAccountName: config.DefaultServiceAccountValue,
				},
				Timeouts: &v1.TimeoutFields{
					Pipeline: &metav1.Duration{Duration: config.DefaultTimeoutMinutes * time.Minute},
				},
				PipelineRef: &v1.PipelineRef{
					Name: "foo",
				},
			},
		},
	}}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := apis.WithinCreate(cfgtesting.SetDefaults(context.Background(), t, tc.defaults))
			got := tc.in
			got.SetDefaults(ctx)
			if !cmp.Equal(got, tc.want, ignoreUnexportedResources) {
				d := cmp.Diff(got, tc.want, ignoreUnexportedResources)
				t.Errorf("SetDefaults %s", diff.PrintWantGot(d))
			}
		})
	}
}
