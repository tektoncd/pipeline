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

package v1alpha1_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/test/diff"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	logtesting "knative.dev/pkg/logging/testing"
)

func TestTaskRunSpec_SetDefaults(t *testing.T) {
	cases := []struct {
		desc string
		trs  *v1alpha1.TaskRunSpec
		want *v1alpha1.TaskRunSpec
	}{{
		desc: "taskref is nil",
		trs: &v1alpha1.TaskRunSpec{
			TaskRef: nil,
			Timeout: &metav1.Duration{Duration: 500 * time.Millisecond},
		},
		want: &v1alpha1.TaskRunSpec{
			TaskRef:            nil,
			ServiceAccountName: config.DefaultServiceAccountValue,
			Timeout:            &metav1.Duration{Duration: 500 * time.Millisecond},
		},
	}, {
		desc: "taskref kind is empty",
		trs: &v1alpha1.TaskRunSpec{
			TaskRef: &v1alpha1.TaskRef{},
			Timeout: &metav1.Duration{Duration: 500 * time.Millisecond},
		},
		want: &v1alpha1.TaskRunSpec{
			TaskRef:            &v1alpha1.TaskRef{Kind: v1alpha1.NamespacedTaskKind},
			ServiceAccountName: config.DefaultServiceAccountValue,
			Timeout:            &metav1.Duration{Duration: 500 * time.Millisecond},
		},
	}, {
		desc: "timeout is nil",
		trs: &v1alpha1.TaskRunSpec{
			TaskRef: &v1alpha1.TaskRef{Kind: v1alpha1.ClusterTaskKind},
		},
		want: &v1alpha1.TaskRunSpec{
			TaskRef:            &v1alpha1.TaskRef{Kind: v1alpha1.ClusterTaskKind},
			ServiceAccountName: config.DefaultServiceAccountValue,
			Timeout:            &metav1.Duration{Duration: config.DefaultTimeoutMinutes * time.Minute},
		},
	}, {
		desc: "pod template is nil",
		trs:  &v1alpha1.TaskRunSpec{},
		want: &v1alpha1.TaskRunSpec{
			ServiceAccountName: config.DefaultServiceAccountValue,
			Timeout:            &metav1.Duration{Duration: config.DefaultTimeoutMinutes * time.Minute},
		},
	}, {
		desc: "pod template is not nil",
		trs: &v1alpha1.TaskRunSpec{
			PodTemplate: &v1alpha1.PodTemplate{
				NodeSelector: map[string]string{
					"label": "value",
				},
			},
		},
		want: &v1alpha1.TaskRunSpec{
			ServiceAccountName: config.DefaultServiceAccountValue,
			Timeout:            &metav1.Duration{Duration: config.DefaultTimeoutMinutes * time.Minute},
			PodTemplate: &v1alpha1.PodTemplate{
				NodeSelector: map[string]string{
					"label": "value",
				},
			},
		},
	}, {
		desc: "embedded taskSpec",
		trs: &v1alpha1.TaskRunSpec{
			TaskSpec: &v1alpha1.TaskSpec{
				Inputs: &v1alpha1.Inputs{
					Params: []v1alpha1.ParamSpec{{
						Name: "param-name",
					}},
				},
			},
		},
		want: &v1alpha1.TaskRunSpec{
			TaskSpec: &v1alpha1.TaskSpec{
				Inputs: &v1alpha1.Inputs{
					Params: []v1alpha1.ParamSpec{{
						Name: "param-name",
						Type: v1alpha1.ParamTypeString,
					}},
				},
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
				t.Errorf("Mismatch of TaskRunSpec %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestTaskRunDefaulting(t *testing.T) {
	tests := []struct {
		name string
		in   *v1alpha1.TaskRun
		want *v1alpha1.TaskRun
		wc   func(context.Context) context.Context
	}{{
		name: "empty no context",
		in:   &v1alpha1.TaskRun{},
		want: &v1alpha1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{"app.kubernetes.io/managed-by": "tekton-pipelines"},
			},
			Spec: v1alpha1.TaskRunSpec{
				ServiceAccountName: config.DefaultServiceAccountValue,
				Timeout:            &metav1.Duration{Duration: config.DefaultTimeoutMinutes * time.Minute},
			},
		},
	}, {
		name: "TaskRef default to namespace kind",
		in: &v1alpha1.TaskRun{
			Spec: v1alpha1.TaskRunSpec{
				TaskRef: &v1alpha1.TaskRef{Name: "foo"},
			},
		},
		want: &v1alpha1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{"app.kubernetes.io/managed-by": "tekton-pipelines"},
			},
			Spec: v1alpha1.TaskRunSpec{
				TaskRef:            &v1alpha1.TaskRef{Name: "foo", Kind: v1alpha1.NamespacedTaskKind},
				ServiceAccountName: config.DefaultServiceAccountValue,
				Timeout:            &metav1.Duration{Duration: config.DefaultTimeoutMinutes * time.Minute},
			},
		},
	}, {
		name: "TaskRef default config context",
		in: &v1alpha1.TaskRun{
			Spec: v1alpha1.TaskRunSpec{
				TaskRef: &v1alpha1.TaskRef{Name: "foo"},
			},
		},
		want: &v1alpha1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{"app.kubernetes.io/managed-by": "tekton-pipelines"},
			},
			Spec: v1alpha1.TaskRunSpec{
				TaskRef:            &v1alpha1.TaskRef{Name: "foo", Kind: v1alpha1.NamespacedTaskKind},
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
		name: "TaskRef default config context with SA",
		in: &v1alpha1.TaskRun{
			Spec: v1alpha1.TaskRunSpec{
				TaskRef: &v1alpha1.TaskRef{Name: "foo"},
			},
		},
		want: &v1alpha1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{"app.kubernetes.io/managed-by": "tekton-pipelines"},
			},
			Spec: v1alpha1.TaskRunSpec{
				TaskRef:            &v1alpha1.TaskRef{Name: "foo", Kind: v1alpha1.NamespacedTaskKind},
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
		in: &v1alpha1.TaskRun{
			Spec: v1alpha1.TaskRunSpec{
				TaskRef: &v1alpha1.TaskRef{Name: "foo"},
			},
		},
		want: &v1alpha1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{"app.kubernetes.io/managed-by": "something-else"},
			},
			Spec: v1alpha1.TaskRunSpec{
				TaskRef:            &v1alpha1.TaskRef{Name: "foo", Kind: v1alpha1.NamespacedTaskKind},
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
		in: &v1alpha1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{"app.kubernetes.io/managed-by": "user-specified"},
			},
			Spec: v1alpha1.TaskRunSpec{
				TaskRef: &v1alpha1.TaskRef{Name: "foo"},
			},
		},
		want: &v1alpha1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{"app.kubernetes.io/managed-by": "user-specified"},
			},
			Spec: v1alpha1.TaskRunSpec{
				TaskRef:            &v1alpha1.TaskRef{Name: "foo", Kind: v1alpha1.NamespacedTaskKind},
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
		in: &v1alpha1.TaskRun{
			Spec: v1alpha1.TaskRunSpec{
				TaskRef: &v1alpha1.TaskRef{Name: "foo"},
			},
		},
		want: &v1alpha1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{"app.kubernetes.io/managed-by": "tekton-pipelines"},
			},
			Spec: v1alpha1.TaskRunSpec{
				TaskRef:            &v1alpha1.TaskRef{Name: "foo", Kind: v1alpha1.NamespacedTaskKind},
				Timeout:            &metav1.Duration{Duration: 5 * time.Minute},
				ServiceAccountName: "tekton",
				PodTemplate: &v1alpha1.PodTemplate{
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
		name: "TaskRef pod template takes precedence over default config pod template",
		in: &v1alpha1.TaskRun{
			Spec: v1alpha1.TaskRunSpec{
				TaskRef: &v1alpha1.TaskRef{Name: "foo"},
				PodTemplate: &v1alpha1.PodTemplate{
					NodeSelector: map[string]string{
						"label2": "value2",
					},
				},
			},
		},
		want: &v1alpha1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{"app.kubernetes.io/managed-by": "tekton-pipelines"},
			},
			Spec: v1alpha1.TaskRunSpec{
				TaskRef:            &v1alpha1.TaskRef{Name: "foo", Kind: v1alpha1.NamespacedTaskKind},
				Timeout:            &metav1.Duration{Duration: 5 * time.Minute},
				ServiceAccountName: "tekton",
				PodTemplate: &v1alpha1.PodTemplate{
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
	}}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.in
			ctx := context.Background()
			if tc.wc != nil {
				ctx = tc.wc(ctx)
			}
			got.SetDefaults(ctx)
			if d := cmp.Diff(tc.want, got, ignoreUnexportedResources); d != "" {
				t.Errorf("SetDefaults %s", diff.PrintWantGot(d))
			}
		})
	}
}
