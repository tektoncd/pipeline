/*
Copyright 2021 The Tekton Authors

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
package affinityassistant_test

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/pkg/internal/affinityassistant"
	"github.com/tektoncd/pipeline/pkg/workspace"
	"github.com/tektoncd/pipeline/test/diff"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestNewTransformer(t *testing.T) {
	for _, tc := range []struct {
		description string
		annotations map[string]string
		expected    *corev1.Affinity
	}{{
		description: "no affinity annotation",
		annotations: map[string]string{
			"foo": "bar",
		},
	}, {
		description: "affinity annotation",
		annotations: map[string]string{
			"foo": "bar",
			workspace.AnnotationAffinityAssistantName: "baz",
		},
		expected: &corev1.Affinity{
			PodAffinity: &corev1.PodAffinity{
				RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{{
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							workspace.LabelInstance:  "baz",
							workspace.LabelComponent: workspace.ComponentNameAffinityAssistant,
						},
					},
					TopologyKey: "kubernetes.io/hostname",
				}},
			},
		},
	}, {
		description: "affinity annotation with a different name",
		annotations: map[string]string{
			workspace.AnnotationAffinityAssistantName: "helloworld",
		},
		expected: &corev1.Affinity{
			PodAffinity: &corev1.PodAffinity{
				RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{{
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							workspace.LabelInstance:  "helloworld",
							workspace.LabelComponent: workspace.ComponentNameAffinityAssistant,
						},
					},
					TopologyKey: "kubernetes.io/hostname",
				}},
			},
		},
	}} {
		t.Run(tc.description, func(t *testing.T) {
			ctx := context.Background()
			f := affinityassistant.NewTransformer(ctx, tc.annotations)
			got, err := f(&corev1.Pod{})
			if err != nil {
				t.Fatalf("Transformer failed: %v", err)
			}
			if d := cmp.Diff(tc.expected, got.Spec.Affinity); d != "" {
				t.Errorf("AffinityAssistant diff: %s", diff.PrintWantGot(d))
			}
		})
	}
}
