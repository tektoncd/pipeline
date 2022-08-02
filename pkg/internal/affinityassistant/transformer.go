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

package affinityassistant

import (
	"context"

	"github.com/tektoncd/pipeline/pkg/pod"
	"github.com/tektoncd/pipeline/pkg/workspace"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NewTransformer returns a pod.Transformer that will pod affinity if needed
func NewTransformer(_ context.Context, annotations map[string]string) pod.Transformer {
	return func(p *corev1.Pod) (*corev1.Pod, error) {
		// Using node affinity on taskRuns sharing PVC workspace.  When Affinity Assistant
		// is disabled, an affinityAssistantName is not set.
		if affinityAssistantName := annotations[workspace.AnnotationAffinityAssistantName]; affinityAssistantName != "" {
			if p.Spec.Affinity == nil {
				p.Spec.Affinity = &corev1.Affinity{}
			}
			mergeAffinityWithAffinityAssistant(p.Spec.Affinity, affinityAssistantName)
		}
		return p, nil
	}
}

func mergeAffinityWithAffinityAssistant(affinity *corev1.Affinity, affinityAssistantName string) {
	podAffinityTerm := podAffinityTermUsingAffinityAssistant(affinityAssistantName)

	if affinity.PodAffinity == nil {
		affinity.PodAffinity = &corev1.PodAffinity{}
	}

	affinity.PodAffinity.RequiredDuringSchedulingIgnoredDuringExecution =
		append(affinity.PodAffinity.RequiredDuringSchedulingIgnoredDuringExecution, *podAffinityTerm)
}

// podAffinityTermUsingAffinityAssistant achieves pod Affinity term for taskRun
// pods so that the taskRun is scheduled to the Node where the Affinity Assistant pod
// is scheduled.
func podAffinityTermUsingAffinityAssistant(affinityAssistantName string) *corev1.PodAffinityTerm {
	return &corev1.PodAffinityTerm{LabelSelector: &metav1.LabelSelector{
		MatchLabels: map[string]string{
			workspace.LabelInstance:  affinityAssistantName,
			workspace.LabelComponent: workspace.ComponentNameAffinityAssistant,
		},
	},
		TopologyKey: "kubernetes.io/hostname",
	}
}
