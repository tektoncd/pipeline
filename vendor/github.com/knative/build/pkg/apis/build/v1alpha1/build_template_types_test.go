/*
Copyright 2018 The Knative Authors

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

package v1alpha1

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
)

func TestBuildTemplateSpec(t *testing.T) {
	c := BuildTemplate{
		Spec: BuildTemplateSpec{
			Steps: []corev1.Container{{
				Name: "build-spec",
			}},
		},
	}

	expectedBuildSpec := BuildTemplateSpec{Steps: []corev1.Container{{Name: "build-spec"}}}

	if a := cmp.Diff(c.TemplateSpec(), expectedBuildSpec); a != "" {
		t.Errorf("templateSpec mismatch; expected: %v got: %v", expectedBuildSpec, a)
	}
}

func TestBuildTemplateGroupVersionKind(t *testing.T) {
	c := BuildTemplate{}

	expectedKind := "BuildTemplate"
	if c.GetGroupVersionKind().Kind != expectedKind {
		t.Errorf("GetGroupVersionKind mismatch; expected: %v got: %v", expectedKind, c.GetGroupVersionKind().Kind)
	}
}
