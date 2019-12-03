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

package v1alpha2_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha2"
	corev1 "k8s.io/api/core/v1"
)

var (
	prependStep = corev1.Container{
		Name:    "prepend-step",
		Image:   "dummy",
		Command: []string{"doit"},
		Args:    []string{"stuff", "things"},
	}
	appendStep = corev1.Container{
		Name:    "append-step",
		Image:   "dummy",
		Command: []string{"doit"},
		Args:    []string{"other stuff", "other things"},
	}
	volume = corev1.Volume{
		Name: "magic-volume",
		VolumeSource: corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: "some-claim"},
		},
	}
)

type TestTaskModifier struct{}

func (tm *TestTaskModifier) GetStepsToPrepend() []v1alpha2.Step {
	return []v1alpha2.Step{{
		Container: prependStep,
	}}
}

func (tm *TestTaskModifier) GetStepsToAppend() []v1alpha2.Step {
	return []v1alpha2.Step{{
		Container: appendStep,
	}}
}

func (tm *TestTaskModifier) GetVolumes() []corev1.Volume {
	return []corev1.Volume{volume}
}

func TestApplyTaskModifier(t *testing.T) {
	testcases := []struct {
		name string
		ts   v1alpha2.TaskSpec
	}{{
		name: "success",
		ts:   v1alpha2.TaskSpec{},
	}, {
		name: "identical volume already added",
		ts: v1alpha2.TaskSpec{
			// Trying to add the same Volume that has already been added shouldn't be an error
			// and it should not be added twice
			Volumes: []corev1.Volume{volume},
		},
	}}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			if err := v1alpha2.ApplyTaskModifier(&tc.ts, &TestTaskModifier{}); err != nil {
				t.Fatalf("Did not expect error modifying TaskSpec but got %v", err)
			}

			expectedTaskSpec := v1alpha2.TaskSpec{
				Steps: []v1alpha2.Step{{
					Container: prependStep,
				}, {
					Container: appendStep,
				}},
				Volumes: []corev1.Volume{
					volume,
				},
			}

			if d := cmp.Diff(expectedTaskSpec, tc.ts); d != "" {
				t.Errorf("TaskSpec was not modified as expected (-want, +got): %s", d)
			}
		})
	}
}

func TestApplyTaskModifier_AlreadyAdded(t *testing.T) {
	testcases := []struct {
		name string
		ts   v1alpha2.TaskSpec
	}{{
		name: "prepend already added",
		ts: v1alpha2.TaskSpec{
			Steps: []v1alpha2.Step{{Container: prependStep}},
		},
	}, {
		name: "append already added",
		ts: v1alpha2.TaskSpec{
			Steps: []v1alpha2.Step{{Container: appendStep}},
		},
	}, {
		name: "both steps already added",
		ts: v1alpha2.TaskSpec{
			Steps: []v1alpha2.Step{{Container: prependStep}, {Container: appendStep}},
		},
	}, {
		name: "both steps already added reverse order",
		ts: v1alpha2.TaskSpec{
			Steps: []v1alpha2.Step{{Container: appendStep}, {Container: prependStep}},
		},
	}, {
		name: "volume with same name but diff content already added",
		ts: v1alpha2.TaskSpec{
			Volumes: []corev1.Volume{{
				Name: "magic-volume",
				VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{},
				},
			}},
		},
	}}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			if err := v1alpha2.ApplyTaskModifier(&tc.ts, &TestTaskModifier{}); err == nil {
				t.Errorf("Expected error when applying values already added but got none")
			}
		})
	}
}
