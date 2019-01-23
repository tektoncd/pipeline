/*
Copyright 2018 The Knative Authors.

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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestTaskRun_GetBuildPodRef(t *testing.T) {
	tr := TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "taskrunname",
			Namespace: "testns",
		},
	}
	if d := cmp.Diff(tr.GetBuildPodRef(), corev1.ObjectReference{
		APIVersion: "v1",
		Kind:       "Pod",
		Namespace:  "testns",
		Name:       "taskrunname",
	}); d != "" {
		t.Fatalf("taskrun build pod ref mismatch: %s", d)
	}
}

func TestTaskRun_GetPipelineRunPVCName(t *testing.T) {
	tests := []struct {
		name            string
		tr              *TaskRun
		expectedPVCName string
	}{{
		name: "invalid owner reference",
		tr: &TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "taskrunname",
				Namespace: "testns",
				OwnerReferences: []metav1.OwnerReference{{
					Kind: "SomeOtherOwner",
					Name: "testpr",
				}},
			},
		},
		expectedPVCName: "",
	}, {
		name: "valid pipelinerun owner",
		tr: &TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "taskrunname",
				Namespace: "testns",
				OwnerReferences: []metav1.OwnerReference{{
					Kind: "PipelineRun",
					Name: "testpr",
				}},
			},
		},
		expectedPVCName: "testpr-pvc",
	}, {
		name:            "nil taskrun",
		expectedPVCName: "",
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.tr.GetPipelineRunPVCName() != tt.expectedPVCName {
				t.Fatalf("taskrun pipeline run pvc name mismatch: got %s ; expected %s", tt.tr.GetPipelineRunPVCName(), tt.expectedPVCName)
			}
		})
	}
}

func TestTaskRun_HasPipelineRun(t *testing.T) {
	tests := []struct {
		name string
		tr   *TaskRun
		want bool
	}{{
		name: "invalid owner reference",
		tr: &TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "taskrunname",
				Namespace: "testns",
				OwnerReferences: []metav1.OwnerReference{{
					Kind: "SomeOtherOwner",
					Name: "testpr",
				}},
			},
		},
		want: false,
	}, {
		name: "valid pipelinerun owner",
		tr: &TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "taskrunname",
				Namespace: "testns",
				OwnerReferences: []metav1.OwnerReference{{
					Kind: "PipelineRun",
					Name: "testpr",
				}},
			},
		},
		want: true,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.tr.HasPipelineRunOwnerReference() != tt.want {
				t.Fatalf("taskrun pipeline run pvc name mismatch: got %s ; expected %t", tt.tr.GetPipelineRunPVCName(), tt.want)
			}
		})
	}
}
