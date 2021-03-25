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

package v1beta1_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/pod"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/pkg/contexts"
	"github.com/tektoncd/pipeline/test/diff"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	logtesting "knative.dev/pkg/logging/testing"
)

func TestPipelineRunSpec_SetDefaults(t *testing.T) {
	cases := []struct {
		desc  string
		prs   *v1beta1.PipelineRunSpec
		want  *v1beta1.PipelineRunSpec
		ctxFn func(context.Context) context.Context
	}{
		{
			desc: "timeout is nil",
			prs:  &v1beta1.PipelineRunSpec{},
			want: &v1beta1.PipelineRunSpec{
				ServiceAccountName: config.DefaultServiceAccountValue,
				Timeout:            &metav1.Duration{Duration: config.DefaultTimeoutMinutes * time.Minute},
			},
		},
		{
			desc: "timeout is not nil",
			prs: &v1beta1.PipelineRunSpec{
				Timeout: &metav1.Duration{Duration: 500 * time.Millisecond},
			},
			want: &v1beta1.PipelineRunSpec{
				ServiceAccountName: config.DefaultServiceAccountValue,
				Timeout:            &metav1.Duration{Duration: 500 * time.Millisecond},
			},
		},
		{
			desc: "pod template is nil",
			prs:  &v1beta1.PipelineRunSpec{},
			want: &v1beta1.PipelineRunSpec{
				ServiceAccountName: config.DefaultServiceAccountValue,
				Timeout:            &metav1.Duration{Duration: config.DefaultTimeoutMinutes * time.Minute},
			},
		},
		{
			desc: "pod template is not nil",
			prs: &v1beta1.PipelineRunSpec{
				PodTemplate: &pod.Template{
					NodeSelector: map[string]string{
						"label": "value",
					},
				},
			},
			want: &v1beta1.PipelineRunSpec{
				ServiceAccountName: config.DefaultServiceAccountValue,
				Timeout:            &metav1.Duration{Duration: config.DefaultTimeoutMinutes * time.Minute},
				PodTemplate: &pod.Template{
					NodeSelector: map[string]string{
						"label": "value",
					},
				},
			},
		},
		{
			desc: "implicit params",
			ctxFn: func(ctx context.Context) context.Context {
				cfg := config.FromContextOrDefaults(ctx)
				cfg.FeatureFlags = &config.FeatureFlags{EnableAPIFields: "alpha"}
				return config.ToContext(ctx, cfg)
			},
			prs: &v1beta1.PipelineRunSpec{
				Params: []v1beta1.Param{
					{
						Name: "foo",
						Value: v1beta1.ArrayOrString{
							StringVal: "a",
						},
					},
					{
						Name: "bar",
						Value: v1beta1.ArrayOrString{
							ArrayVal: []string{"b"},
						},
					},
				},
				PipelineSpec: &v1beta1.PipelineSpec{
					Tasks: []v1beta1.PipelineTask{{
						TaskSpec: &v1beta1.EmbeddedTask{
							TaskSpec: v1beta1.TaskSpec{},
						},
					}},
				},
			},
			want: &v1beta1.PipelineRunSpec{
				ServiceAccountName: config.DefaultServiceAccountValue,
				Timeout:            &metav1.Duration{Duration: config.DefaultTimeoutMinutes * time.Minute},
				Params: []v1beta1.Param{
					{
						Name: "foo",
						Value: v1beta1.ArrayOrString{
							StringVal: "a",
						},
					},
					{
						Name: "bar",
						Value: v1beta1.ArrayOrString{
							ArrayVal: []string{"b"},
						},
					},
				},
				PipelineSpec: &v1beta1.PipelineSpec{
					Tasks: []v1beta1.PipelineTask{{
						TaskSpec: &v1beta1.EmbeddedTask{
							TaskSpec: v1beta1.TaskSpec{
								Params: []v1beta1.ParamSpec{
									{
										Name: "foo",
										Type: v1beta1.ParamTypeString,
									},
									{
										Name: "bar",
										Type: v1beta1.ParamTypeArray,
									},
								},
							},
						},
						Params: []v1beta1.Param{
							{
								Name: "foo",
								Value: v1beta1.ArrayOrString{
									Type:      v1beta1.ParamTypeString,
									StringVal: "$(params.foo)",
								},
							},
							{
								Name: "bar",
								Value: v1beta1.ArrayOrString{
									Type:     v1beta1.ParamTypeArray,
									ArrayVal: []string{"$(params.bar[*])"},
								},
							},
						},
					}},
					Params: []v1beta1.ParamSpec{
						{
							Name: "foo",
							Type: v1beta1.ParamTypeString,
						},
						{
							Name: "bar",
							Type: v1beta1.ParamTypeArray,
						},
					},
				},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			ctx := context.Background()
			if tc.ctxFn != nil {
				ctx = tc.ctxFn(ctx)
			}
			tc.prs.SetDefaults(ctx)

			sortPS := func(x, y v1beta1.ParamSpec) bool {
				return x.Name < y.Name
			}
			sortP := func(x, y v1beta1.Param) bool {
				return x.Name < y.Name
			}
			if d := cmp.Diff(tc.want, tc.prs, cmpopts.SortSlices(sortPS), cmpopts.SortSlices(sortP)); d != "" {
				t.Errorf("Mismatch of PipelineRunSpec %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestPipelineRunDefaulting(t *testing.T) {
	tests := []struct {
		name string
		in   *v1beta1.PipelineRun
		want *v1beta1.PipelineRun
		wc   func(context.Context) context.Context
	}{{
		name: "empty no context",
		in:   &v1beta1.PipelineRun{},
		want: &v1beta1.PipelineRun{
			Spec: v1beta1.PipelineRunSpec{
				ServiceAccountName: config.DefaultServiceAccountValue,
				Timeout:            &metav1.Duration{Duration: config.DefaultTimeoutMinutes * time.Minute},
			},
		},
	}, {
		name: "PipelineRef upgrade context",
		in: &v1beta1.PipelineRun{
			Spec: v1beta1.PipelineRunSpec{
				PipelineRef: &v1beta1.PipelineRef{Name: "foo"},
			},
		},
		want: &v1beta1.PipelineRun{
			Spec: v1beta1.PipelineRunSpec{
				PipelineRef:        &v1beta1.PipelineRef{Name: "foo"},
				ServiceAccountName: config.DefaultServiceAccountValue,
				Timeout:            &metav1.Duration{Duration: config.DefaultTimeoutMinutes * time.Minute},
			},
		},
		wc: contexts.WithUpgradeViaDefaulting,
	}, {
		name: "Embedded PipelineSpec default",
		in: &v1beta1.PipelineRun{
			Spec: v1beta1.PipelineRunSpec{
				PipelineSpec: &v1beta1.PipelineSpec{
					Params: []v1beta1.ParamSpec{{
						Name: "foo",
					}},
				},
			},
		},
		want: &v1beta1.PipelineRun{
			Spec: v1beta1.PipelineRunSpec{
				PipelineSpec: &v1beta1.PipelineSpec{
					Params: []v1beta1.ParamSpec{{
						Name: "foo",
						Type: "string",
					}},
				},
				ServiceAccountName: config.DefaultServiceAccountValue,
				Timeout:            &metav1.Duration{Duration: config.DefaultTimeoutMinutes * time.Minute},
			},
		},
		wc: contexts.WithUpgradeViaDefaulting,
	}, {
		name: "PipelineRef default config context",
		in: &v1beta1.PipelineRun{
			Spec: v1beta1.PipelineRunSpec{
				PipelineRef: &v1beta1.PipelineRef{Name: "foo"},
			},
		},
		want: &v1beta1.PipelineRun{
			Spec: v1beta1.PipelineRunSpec{
				PipelineRef:        &v1beta1.PipelineRef{Name: "foo"},
				ServiceAccountName: config.DefaultServiceAccountValue,
				Timeout:            &metav1.Duration{Duration: 5 * time.Minute},
			},
		},
		wc: func(ctx context.Context) context.Context {
			s := config.NewStore(logtesting.TestLogger(t))
			s.OnConfigChanged(&corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name: config.GetDefaultsConfigName(),
				},
				Data: map[string]string{
					"default-timeout-minutes": "5",
				},
			})
			return s.ToContext(ctx)
		},
	}, {
		name: "PipelineRef default config context with sa",
		in: &v1beta1.PipelineRun{
			Spec: v1beta1.PipelineRunSpec{
				PipelineRef: &v1beta1.PipelineRef{Name: "foo"},
			},
		},
		want: &v1beta1.PipelineRun{
			Spec: v1beta1.PipelineRunSpec{
				PipelineRef:        &v1beta1.PipelineRef{Name: "foo"},
				Timeout:            &metav1.Duration{Duration: 5 * time.Minute},
				ServiceAccountName: "tekton",
			},
		},
		wc: func(ctx context.Context) context.Context {
			s := config.NewStore(logtesting.TestLogger(t))
			s.OnConfigChanged(&corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name: config.GetDefaultsConfigName(),
				},
				Data: map[string]string{
					"default-timeout-minutes": "5",
					"default-service-account": "tekton",
				},
			})
			return s.ToContext(ctx)
		},
	}, {
		name: "PipelineRef pod template is coming from default config pod template",
		in: &v1beta1.PipelineRun{
			Spec: v1beta1.PipelineRunSpec{
				PipelineRef: &v1beta1.PipelineRef{Name: "foo"},
			},
		},
		want: &v1beta1.PipelineRun{
			Spec: v1beta1.PipelineRunSpec{
				PipelineRef:        &v1beta1.PipelineRef{Name: "foo"},
				Timeout:            &metav1.Duration{Duration: 5 * time.Minute},
				ServiceAccountName: "tekton",
				PodTemplate: &pod.Template{
					NodeSelector: map[string]string{
						"label": "value",
					},
				},
			},
		},
		wc: func(ctx context.Context) context.Context {
			s := config.NewStore(logtesting.TestLogger(t))
			s.OnConfigChanged(&corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name: config.GetDefaultsConfigName(),
				},
				Data: map[string]string{
					"default-timeout-minutes": "5",
					"default-service-account": "tekton",
					"default-pod-template":    "nodeSelector: { 'label': 'value' }",
				},
			})
			return s.ToContext(ctx)
		},
	}, {
		name: "PipelineRef pod template nodeselector takes precedence over default config pod template nodeselector",
		in: &v1beta1.PipelineRun{
			Spec: v1beta1.PipelineRunSpec{
				PipelineRef: &v1beta1.PipelineRef{Name: "foo"},
				PodTemplate: &pod.Template{
					NodeSelector: map[string]string{
						"label2": "value2",
					},
				},
			},
		},
		want: &v1beta1.PipelineRun{
			Spec: v1beta1.PipelineRunSpec{
				PipelineRef:        &v1beta1.PipelineRef{Name: "foo"},
				Timeout:            &metav1.Duration{Duration: 5 * time.Minute},
				ServiceAccountName: "tekton",
				PodTemplate: &pod.Template{
					NodeSelector: map[string]string{
						"label2": "value2",
					},
				},
			},
		},
		wc: func(ctx context.Context) context.Context {
			s := config.NewStore(logtesting.TestLogger(t))
			s.OnConfigChanged(&corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name: config.GetDefaultsConfigName(),
				},
				Data: map[string]string{
					"default-timeout-minutes": "5",
					"default-service-account": "tekton",
					"default-pod-template":    "nodeSelector: { 'label': 'value' }",
				},
			})
			return s.ToContext(ctx)
		},
	}, {
		name: "PipelineRef pod template merges non competing fields with default config pod template",
		in: &v1beta1.PipelineRun{
			Spec: v1beta1.PipelineRunSpec{
				PipelineRef: &v1beta1.PipelineRef{Name: "foo"},
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
		want: &v1beta1.PipelineRun{
			Spec: v1beta1.PipelineRunSpec{
				PipelineRef:        &v1beta1.PipelineRef{Name: "foo"},
				Timeout:            &metav1.Duration{Duration: 5 * time.Minute},
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
		},
		wc: func(ctx context.Context) context.Context {
			s := config.NewStore(logtesting.TestLogger(t))
			s.OnConfigChanged(&corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name: config.GetDefaultsConfigName(),
				},
				Data: map[string]string{
					"default-timeout-minutes": "5",
					"default-service-account": "tekton",
					"default-pod-template":    "nodeSelector: { 'label': 'value' }\nhostNetwork: true",
				},
			})
			return s.ToContext(ctx)
		},
	}}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.in
			ctx := context.Background()
			if tc.wc != nil {
				ctx = tc.wc(ctx)
			}
			got.SetDefaults(ctx)
			if !cmp.Equal(got, tc.want, ignoreUnexportedResources) {
				d := cmp.Diff(got, tc.want, ignoreUnexportedResources)
				t.Errorf("SetDefaults %s", diff.PrintWantGot(d))
			}
		})
	}
}
