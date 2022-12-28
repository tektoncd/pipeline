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

func TestNewTransformerWithNodeAffinity(t *testing.T) {
	nodeAffinity := &corev1.NodeAffinity{
		RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
			NodeSelectorTerms: []corev1.NodeSelectorTerm{{
				MatchFields: []corev1.NodeSelectorRequirement{{
					Key:      "kubernetes.io/hostname",
					Operator: corev1.NodeSelectorOpNotIn,
					Values:   []string{"192.0.0.1"},
				}},
			}},
		},
	}

	podAffinityTerm := &corev1.PodAffinityTerm{
		LabelSelector: &metav1.LabelSelector{
			MatchLabels: map[string]string{
				"from": "podtemplate",
			},
		},
		TopologyKey: "kubernetes.io/hostname",
	}

	for _, tc := range []struct {
		description string
		annotations map[string]string
		pod         *corev1.Pod
		expected    *corev1.Affinity
	}{{
		description: "no affinity annotation and pod contains nodeAffinity",
		annotations: map[string]string{
			"foo": "bar",
		},
		pod: &corev1.Pod{
			Spec: corev1.PodSpec{
				Affinity: &corev1.Affinity{
					NodeAffinity: nodeAffinity,
				},
			},
		},
		expected: &corev1.Affinity{
			NodeAffinity: nodeAffinity,
		},
	}, {
		description: "affinity annotation and pod contains nodeAffinity",
		annotations: map[string]string{
			"foo": "bar",
			workspace.AnnotationAffinityAssistantName: "baz",
		},
		pod: &corev1.Pod{
			Spec: corev1.PodSpec{
				Affinity: &corev1.Affinity{
					NodeAffinity: nodeAffinity,
				},
			},
		},

		expected: &corev1.Affinity{
			NodeAffinity: nodeAffinity,
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
		description: "affinity annotation with a different name and pod contains podAffinity",
		annotations: map[string]string{
			workspace.AnnotationAffinityAssistantName: "helloworld",
		},
		pod: &corev1.Pod{
			Spec: corev1.PodSpec{
				Affinity: &corev1.Affinity{PodAffinity: &corev1.PodAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution:  []corev1.PodAffinityTerm{*podAffinityTerm},
					PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{{Weight: 100, PodAffinityTerm: *podAffinityTerm}},
				}},
			},
		},

		expected: &corev1.Affinity{
			PodAffinity: &corev1.PodAffinity{
				RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{*podAffinityTerm, {
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							workspace.LabelInstance:  "helloworld",
							workspace.LabelComponent: workspace.ComponentNameAffinityAssistant,
						},
					},
					TopologyKey: "kubernetes.io/hostname",
				}},
				PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{
					{Weight: 100, PodAffinityTerm: *podAffinityTerm},
				},
			},
		},
	}, {
		description: "affinity annotation with a different name and pod contains podAffinity and nodeAffinity",
		annotations: map[string]string{
			workspace.AnnotationAffinityAssistantName: "helloworld",
		},
		pod: &corev1.Pod{
			Spec: corev1.PodSpec{
				Affinity: &corev1.Affinity{
					PodAffinity: &corev1.PodAffinity{
						RequiredDuringSchedulingIgnoredDuringExecution:  []corev1.PodAffinityTerm{*podAffinityTerm},
						PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{{Weight: 100, PodAffinityTerm: *podAffinityTerm}},
					},
					NodeAffinity: nodeAffinity},
			},
		},

		expected: &corev1.Affinity{
			PodAffinity: &corev1.PodAffinity{
				RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
					*podAffinityTerm,
					{
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								workspace.LabelInstance:  "helloworld",
								workspace.LabelComponent: workspace.ComponentNameAffinityAssistant,
							},
						},
						TopologyKey: "kubernetes.io/hostname",
					},
				},
				PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{
					{Weight: 100, PodAffinityTerm: *podAffinityTerm},
				},
			},
			NodeAffinity: nodeAffinity,
		},
	},
	} {
		t.Run(tc.description, func(t *testing.T) {
			ctx := context.Background()
			f := affinityassistant.NewTransformer(ctx, tc.annotations)
			got, err := f(tc.pod)
			if err != nil {
				t.Fatalf("Transformer failed: %v", err)
			}
			if d := cmp.Diff(tc.expected, got.Spec.Affinity); d != "" {
				t.Errorf("AffinityAssistant diff: %s", diff.PrintWantGot(d))
			}
		})
	}
}
