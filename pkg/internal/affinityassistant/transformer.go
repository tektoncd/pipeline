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
		// Using node affinity on taskRuns sharing PVC workspace, with an Affinity Assistant
		// is mutually exclusive with other affinity on taskRun pods. If other
		// affinity is wanted, that should be added on the Affinity Assistant pod unless
		// assistant is disabled. When Affinity Assistant is disabled, an affinityAssistantName is not set.
		if affinityAssistantName := annotations[workspace.AnnotationAffinityAssistantName]; affinityAssistantName != "" {
			p.Spec.Affinity = nodeAffinityUsingAffinityAssistant(affinityAssistantName)
		}
		return p, nil
	}
}

// nodeAffinityUsingAffinityAssistant achieves Node Affinity for taskRun pods
// sharing PVC workspace by setting PodAffinity so that taskRuns is
// scheduled to the Node were the Affinity Assistant pod is scheduled.
func nodeAffinityUsingAffinityAssistant(affinityAssistantName string) *corev1.Affinity {
	return &corev1.Affinity{
		PodAffinity: &corev1.PodAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{{
				LabelSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						workspace.LabelInstance:  affinityAssistantName,
						workspace.LabelComponent: workspace.ComponentNameAffinityAssistant,
					},
				},
				TopologyKey: "kubernetes.io/hostname",
			}},
		},
	}
}
