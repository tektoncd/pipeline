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

package workspace

import (
	"testing"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

func TestValidateBindingsValid(t *testing.T) {
	for _, tc := range []struct {
		name         string
		declarations []v1alpha1.WorkspaceDeclaration
		bindings     []v1alpha1.WorkspaceBinding
	}{{
		name:         "no bindings provided or required",
		declarations: nil,
		bindings:     nil,
	}, {
		name:         "empty list of bindings provided and required",
		declarations: []v1alpha1.WorkspaceDeclaration{},
		bindings:     []v1alpha1.WorkspaceBinding{},
	}, {
		name: "Successfully bound with PVC",
		declarations: []v1alpha1.WorkspaceDeclaration{{
			Name: "beth",
		}},
		bindings: []v1alpha1.WorkspaceBinding{{
			Name: "beth",
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: "pool-party",
			},
		}},
	}, {
		name: "Successfully bound with emptyDir",
		declarations: []v1alpha1.WorkspaceDeclaration{{
			Name: "beth",
		}},
		bindings: []v1alpha1.WorkspaceBinding{{
			Name:     "beth",
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		}},
	}} {
		t.Run(tc.name, func(t *testing.T) {
			if err := ValidateBindings(tc.declarations, tc.bindings); err != nil {
				t.Errorf("didnt expect error for valid bindings but got: %v", err)
			}
		})
	}

}

func TestValidateBindingsInvalid(t *testing.T) {
	for _, tc := range []struct {
		name         string
		declarations []v1alpha1.WorkspaceDeclaration
		bindings     []v1alpha1.WorkspaceBinding
	}{{
		name: "Didn't provide binding matching declared workspace",
		declarations: []v1alpha1.WorkspaceDeclaration{{
			Name: "beth",
		}},
		bindings: []v1alpha1.WorkspaceBinding{{
			Name:     "kate",
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		}, {
			Name:     "beth",
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		}},
	}, {
		name: "Provided a binding that wasn't needed",
		declarations: []v1alpha1.WorkspaceDeclaration{{
			Name: "randall",
		}, {
			Name: "beth",
		}},
		bindings: []v1alpha1.WorkspaceBinding{{
			Name:     "beth",
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		}},
	}, {
		name: "Provided both pvc and emptydir",
		declarations: []v1alpha1.WorkspaceDeclaration{{
			Name: "beth",
		}},
		bindings: []v1alpha1.WorkspaceBinding{{
			Name:     "beth",
			EmptyDir: &corev1.EmptyDirVolumeSource{},
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: "pool-party",
			},
		}},
	}, {
		name: "Provided neither pvc nor emptydir",
		declarations: []v1alpha1.WorkspaceDeclaration{{
			Name: "beth",
		}},
		bindings: []v1alpha1.WorkspaceBinding{{
			Name: "beth",
		}},
	}, {
		name: "Provided pvc without claim name",
		declarations: []v1alpha1.WorkspaceDeclaration{{
			Name: "beth",
		}},
		bindings: []v1alpha1.WorkspaceBinding{{
			Name:                  "beth",
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{},
		}},
	}} {
		t.Run(tc.name, func(t *testing.T) {
			if err := ValidateBindings(tc.declarations, tc.bindings); err == nil {
				t.Errorf("expected error for invalid bindings but didn't get any!")
			}
		})
	}
}
