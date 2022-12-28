/*
Copyright 2019 The Tekton Authors

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
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/pod"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/test/diff"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	logtesting "knative.dev/pkg/logging/testing"
)

var (
	ignoreUnexportedResources = cmpopts.IgnoreUnexported()
	ttrue                     = true
)

func TestTaskRunSpec_SetDefaults(t *testing.T) {
	cases := []struct {
		desc string
		trs  *v1.TaskRunSpec
		want *v1.TaskRunSpec
	}{{
		desc: "taskref is nil",
		trs: &v1.TaskRunSpec{
			TaskRef: nil,
			Timeout: &metav1.Duration{Duration: 500 * time.Millisecond},
		},
		want: &v1.TaskRunSpec{
			TaskRef:            nil,
			ServiceAccountName: config.DefaultServiceAccountValue,
			Timeout:            &metav1.Duration{Duration: 500 * time.Millisecond},
		},
	}, {
		desc: "taskref kind is empty",
		trs: &v1.TaskRunSpec{
			TaskRef: &v1.TaskRef{},
			Timeout: &metav1.Duration{Duration: 500 * time.Millisecond},
		},
		want: &v1.TaskRunSpec{
			TaskRef:            &v1.TaskRef{Kind: v1.NamespacedTaskKind},
			ServiceAccountName: config.DefaultServiceAccountValue,
			Timeout:            &metav1.Duration{Duration: 500 * time.Millisecond},
		},
	}, {
		desc: "pod template is nil",
		trs:  &v1.TaskRunSpec{},
		want: &v1.TaskRunSpec{
			ServiceAccountName: config.DefaultServiceAccountValue,
			Timeout:            &metav1.Duration{Duration: config.DefaultTimeoutMinutes * time.Minute},
		},
	}, {
		desc: "pod template is not nil",
		trs: &v1.TaskRunSpec{
			PodTemplate: &pod.Template{
				NodeSelector: map[string]string{
					"label": "value",
				},
			},
		},
		want: &v1.TaskRunSpec{
			ServiceAccountName: config.DefaultServiceAccountValue,
			Timeout:            &metav1.Duration{Duration: config.DefaultTimeoutMinutes * time.Minute},
			PodTemplate: &pod.Template{
				NodeSelector: map[string]string{
					"label": "value",
				},
			},
		},
	}, {
		desc: "embedded taskSpec",
		trs: &v1.TaskRunSpec{
			TaskSpec: &v1.TaskSpec{
				Params: []v1.ParamSpec{{
					Name: "param-name",
				}},
			},
		},
		want: &v1.TaskRunSpec{
			TaskSpec: &v1.TaskSpec{
				Params: []v1.ParamSpec{{
					Name: "param-name",
					Type: v1.ParamTypeString,
				}},
			},
			ServiceAccountName: config.DefaultServiceAccountValue,
			Timeout:            &metav1.Duration{Duration: config.DefaultTimeoutMinutes * time.Minute},
		},
	}}
	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			ctx := context.Background()
			tc.trs.SetDefaults(ctx)

			if d := cmp.Diff(tc.want, tc.trs); d != "" {
				t.Errorf("Mismatch of TaskRunSpec: %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestTaskRunDefaulting(t *testing.T) {
	tests := []struct {
		name string
		in   *v1.TaskRun
		want *v1.TaskRun
		wc   func(context.Context) context.Context
	}{{
		name: "empty no context",
		in:   &v1.TaskRun{},
		want: &v1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{"app.kubernetes.io/managed-by": "tekton-pipelines"},
			},
			Spec: v1.TaskRunSpec{
				ServiceAccountName: config.DefaultServiceAccountValue,
				Timeout:            &metav1.Duration{Duration: config.DefaultTimeoutMinutes * time.Minute},
			},
		},
	}, {
		name: "TaskRef default to namespace kind",
		in: &v1.TaskRun{
			Spec: v1.TaskRunSpec{
				TaskRef: &v1.TaskRef{Name: "foo"},
			},
		},
		want: &v1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{"app.kubernetes.io/managed-by": "tekton-pipelines"},
			},
			Spec: v1.TaskRunSpec{
				TaskRef:            &v1.TaskRef{Name: "foo", Kind: v1.NamespacedTaskKind},
				ServiceAccountName: config.DefaultServiceAccountValue,
				Timeout:            &metav1.Duration{Duration: config.DefaultTimeoutMinutes * time.Minute},
			},
		},
	}, {
		name: "TaskRef default config context",
		in: &v1.TaskRun{
			Spec: v1.TaskRunSpec{
				TaskRef: &v1.TaskRef{Name: "foo"},
			},
		},
		want: &v1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{"app.kubernetes.io/managed-by": "tekton-pipelines"},
			},
			Spec: v1.TaskRunSpec{
				ServiceAccountName: config.DefaultServiceAccountValue,
				TaskRef:            &v1.TaskRef{Name: "foo", Kind: v1.NamespacedTaskKind},
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
		name: "TaskRef default config context with SA",
		in: &v1.TaskRun{
			Spec: v1.TaskRunSpec{
				TaskRef: &v1.TaskRef{Name: "foo"},
			},
		},
		want: &v1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{"app.kubernetes.io/managed-by": "tekton-pipelines"},
			},
			Spec: v1.TaskRunSpec{
				TaskRef:            &v1.TaskRef{Name: "foo", Kind: v1.NamespacedTaskKind},
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
		name: "TaskRun managed-by set in config",
		in: &v1.TaskRun{
			Spec: v1.TaskRunSpec{
				TaskRef: &v1.TaskRef{Name: "foo"},
			},
		},
		want: &v1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{"app.kubernetes.io/managed-by": "something-else"},
			},
			Spec: v1.TaskRunSpec{
				TaskRef:            &v1.TaskRef{Name: "foo", Kind: v1.NamespacedTaskKind},
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
					"default-timeout-minutes":        "5",
					"default-managed-by-label-value": "something-else",
				},
			})
			return s.ToContext(ctx)
		},
	}, {
		name: "TaskRun managed-by set in request and config (request wins)",
		in: &v1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{"app.kubernetes.io/managed-by": "user-specified"},
			},
			Spec: v1.TaskRunSpec{
				TaskRef: &v1.TaskRef{Name: "foo"},
			},
		},
		want: &v1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{"app.kubernetes.io/managed-by": "user-specified"},
			},
			Spec: v1.TaskRunSpec{
				TaskRef:            &v1.TaskRef{Name: "foo", Kind: v1.NamespacedTaskKind},
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
					"default-timeout-minutes":        "5",
					"default-managed-by-label-value": "something-else",
				},
			})
			return s.ToContext(ctx)
		},
	}, {
		name: "TaskRef pod template is coming from default config pod template",
		in: &v1.TaskRun{
			Spec: v1.TaskRunSpec{
				TaskRef: &v1.TaskRef{Name: "foo"},
			},
		},
		want: &v1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{"app.kubernetes.io/managed-by": "tekton-pipelines"},
			},
			Spec: v1.TaskRunSpec{
				TaskRef:            &v1.TaskRef{Name: "foo", Kind: v1.NamespacedTaskKind},
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
		name: "TaskRef pod template NodeSelector takes precedence over default config pod template NodeSelector",
		in: &v1.TaskRun{
			Spec: v1.TaskRunSpec{
				TaskRef: &v1.TaskRef{Name: "foo"},
				PodTemplate: &pod.Template{
					NodeSelector: map[string]string{
						"label2": "value2",
					},
				},
			},
		},
		want: &v1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{"app.kubernetes.io/managed-by": "tekton-pipelines"},
			},
			Spec: v1.TaskRunSpec{
				TaskRef:            &v1.TaskRef{Name: "foo", Kind: v1.NamespacedTaskKind},
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
		name: "TaskRef pod template merges non competing fields with default config pod template",
		in: &v1.TaskRun{
			Spec: v1.TaskRunSpec{
				TaskRef: &v1.TaskRef{Name: "foo"},
				PodTemplate: &pod.Template{
					SecurityContext: &corev1.PodSecurityContext{
						RunAsNonRoot: &ttrue,
					},
				},
			},
		},
		want: &v1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{"app.kubernetes.io/managed-by": "tekton-pipelines"},
			},
			Spec: v1.TaskRunSpec{
				TaskRef:            &v1.TaskRef{Name: "foo", Kind: v1.NamespacedTaskKind},
				Timeout:            &metav1.Duration{Duration: 5 * time.Minute},
				ServiceAccountName: "tekton",
				PodTemplate: &pod.Template{
					NodeSelector: map[string]string{
						"label": "value",
					},
					SecurityContext: &corev1.PodSecurityContext{
						RunAsNonRoot: &ttrue,
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
