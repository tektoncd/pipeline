/*
 Copyright 2019 Knative Authors LLC
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

package merge

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func TestCombineStepsWithStepTemplate(t *testing.T) {
	resourceQuantityCmp := cmp.Comparer(func(x, y resource.Quantity) bool {
		return x.Cmp(y) == 0
	})

	for _, tc := range []struct {
		name     string
		template *corev1.Container
		steps    []corev1.Container
		expected []corev1.Container
	}{{
		name:     "nil-template",
		template: nil,
		steps: []corev1.Container{{
			Image: "some-image",
		}},
		expected: []corev1.Container{{
			Image: "some-image",
		}},
	}, {
		name: "not-overlapping",
		template: &corev1.Container{
			Command: []string{"/somecmd"},
		},
		steps: []corev1.Container{{
			Image: "some-image",
		}},
		expected: []corev1.Container{{
			Command: []string{"/somecmd"},
			Image:   "some-image",
		}},
	}, {
		name: "overwriting-one-field",
		template: &corev1.Container{
			Image:   "some-image",
			Command: []string{"/somecmd"},
		},
		steps: []corev1.Container{{
			Image: "some-other-image",
		}},
		expected: []corev1.Container{{
			Command: []string{"/somecmd"},
			Image:   "some-other-image",
		}},
	}, {
		name: "merge-and-overwrite-slice",
		template: &corev1.Container{
			Env: []corev1.EnvVar{
				{
					Name:  "KEEP_THIS",
					Value: "A_VALUE",
				}, {
					Name:  "SOME_KEY",
					Value: "ORIGINAL_VALUE",
				},
			},
		},
		steps: []corev1.Container{{
			Env: []corev1.EnvVar{
				{
					Name:  "NEW_KEY",
					Value: "A_VALUE",
				}, {
					Name:  "SOME_KEY",
					Value: "NEW_VALUE",
				},
			},
		}},
		expected: []corev1.Container{{
			Env: []corev1.EnvVar{
				{
					Name:  "NEW_KEY",
					Value: "A_VALUE",
				}, {
					Name:  "KEEP_THIS",
					Value: "A_VALUE",
				}, {
					Name:  "SOME_KEY",
					Value: "NEW_VALUE",
				},
			},
		}},
	}} {
		t.Run(tc.name, func(t *testing.T) {
			result, err := CombineStepsWithStepTemplate(tc.template, tc.steps)
			if err != nil {
				t.Errorf("expected no error. Got error %v", err)
			}

			if d := cmp.Diff(tc.expected, result, resourceQuantityCmp); d != "" {
				t.Errorf("Combined steps don't match, diff: %s", d)
			}
		})
	}
}
