/*
Copyright 2023 The Tekton Authors

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

package v1beta1_test

import (
	"encoding/hex"
	"testing"

	"github.com/google/go-cmp/cmp"
	v1beta1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/test/diff"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestTask_Checksum(t *testing.T) {
	tests := []struct {
		name string
		task *v1beta1.Task
	}{{
		name: "task ignore uid",
		task: &v1beta1.Task{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "tekton.dev/v1beta1",
				Kind:       "Task"},
			ObjectMeta: metav1.ObjectMeta{
				Name:        "task",
				Namespace:   "task-ns",
				UID:         "abc",
				Labels:      map[string]string{"label": "foo"},
				Annotations: map[string]string{"foo": "bar"},
			},
			Spec: v1beta1.TaskSpec{
				Steps: []v1beta1.Step{{
					Image: "ubuntu",
					Name:  "echo",
				}},
			},
		},
	}, {
		name: "task ignore system annotations",
		task: &v1beta1.Task{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "tekton.dev/v1beta1",
				Kind:       "Task"},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "task",
				Namespace: "task-ns",
				UID:       "abc",
				Labels:    map[string]string{"label": "foo"},
				Annotations: map[string]string{
					"foo":                       "bar",
					"kubectl-client-side-apply": "client",
					"kubectl.kubernetes.io/last-applied-configuration": "config",
				},
			},
			Spec: v1beta1.TaskSpec{
				Steps: []v1beta1.Step{{
					Image: "ubuntu",
					Name:  "echo",
				}},
			},
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sha, err := tt.task.Checksum()
			if err != nil {
				t.Fatalf("Error computing checksuum: %v", err)
			}

			if d := cmp.Diff(hex.EncodeToString(sha), "c913fb33ce186f8a98e77eb2885495da71103de323a1dc420d1df1809a10dfd4"); d != "" {
				t.Error(diff.PrintWantGot(d))
			}
		})
	}
}
