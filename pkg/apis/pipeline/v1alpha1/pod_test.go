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
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

func TestCombinedPodTemplateTakesPrecedence(t *testing.T) {
	affinity := &corev1.Affinity{
		NodeAffinity: &corev1.NodeAffinity{},
	}
	nodeSelector := map[string]string{
		"banana": "chocolat",
	}
	tolerations := []corev1.Toleration{{
		Key:   "banana",
		Value: "chocolat",
	}}

	template := v1alpha1.PodTemplate{
		NodeSelector: map[string]string{
			"foo": "bar",
			"bar": "baz",
		},
		Tolerations: []corev1.Toleration{{
			Key:   "foo",
			Value: "bar",
		}},
		Affinity: &corev1.Affinity{
			PodAffinity: &corev1.PodAffinity{},
		},
	}
	// Same as template above
	want := v1alpha1.PodTemplate{
		NodeSelector: map[string]string{
			"foo": "bar",
			"bar": "baz",
		},
		Tolerations: []corev1.Toleration{{
			Key:   "foo",
			Value: "bar",
		}},
		Affinity: &corev1.Affinity{
			PodAffinity: &corev1.PodAffinity{},
		},
	}

	got := v1alpha1.CombinedPodTemplate(template, nodeSelector, tolerations, affinity)
	if d := cmp.Diff(got, want); d != "" {
		t.Errorf("Diff:\n%s", d)
	}
}
func TestCombinedPodTemplate(t *testing.T) {
	nodeSelector := map[string]string{
		"banana": "chocolat",
	}
	tolerations := []corev1.Toleration{{
		Key:   "banana",
		Value: "chocolat",
	}}

	template := v1alpha1.PodTemplate{
		Tolerations: []corev1.Toleration{{
			Key:   "foo",
			Value: "bar",
		}},
		Affinity: &corev1.Affinity{
			PodAffinity: &corev1.PodAffinity{},
		},
	}
	// Same as template above
	want := v1alpha1.PodTemplate{
		NodeSelector: map[string]string{
			"banana": "chocolat",
		},
		Tolerations: []corev1.Toleration{{
			Key:   "foo",
			Value: "bar",
		}},
		Affinity: &corev1.Affinity{
			PodAffinity: &corev1.PodAffinity{},
		},
	}

	got := v1alpha1.CombinedPodTemplate(template, nodeSelector, tolerations, nil)
	if d := cmp.Diff(got, want); d != "" {
		t.Errorf("Diff:\n%s", d)
	}
}

func TestCombinedPodTemplateOnlyDeprecated(t *testing.T) {
	template := v1alpha1.PodTemplate{}
	affinity := &corev1.Affinity{
		NodeAffinity: &corev1.NodeAffinity{},
	}

	nodeSelector := map[string]string{
		"foo": "bar",
	}
	tolerations := []corev1.Toleration{{
		Key:   "foo",
		Value: "bar",
	}}

	want := v1alpha1.PodTemplate{
		NodeSelector: map[string]string{
			"foo": "bar",
		},
		Tolerations: []corev1.Toleration{{
			Key:   "foo",
			Value: "bar",
		}},
		Affinity: affinity,
	}

	got := v1alpha1.CombinedPodTemplate(template, nodeSelector, tolerations, affinity)
	if d := cmp.Diff(got, want); d != "" {
		t.Errorf("Diff:\n%s", d)
	}
}
