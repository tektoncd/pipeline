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

package workspace_test

import (
	"context"
	"errors"
	"testing"

	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	workspace "github.com/tektoncd/pipeline/pkg/workspace"
	corev1 "k8s.io/api/core/v1"
)

func TestValidateBindingsValid(t *testing.T) {
	for _, tc := range []struct {
		name         string
		declarations []v1.WorkspaceDeclaration
		bindings     []v1.WorkspaceBinding
	}{{
		name:         "no bindings provided or required",
		declarations: nil,
		bindings:     nil,
	}, {
		name:         "empty list of bindings provided and required",
		declarations: []v1.WorkspaceDeclaration{},
		bindings:     []v1.WorkspaceBinding{},
	}, {
		name: "Successfully bound with PVC",
		declarations: []v1.WorkspaceDeclaration{{
			Name: "beth",
		}},
		bindings: []v1.WorkspaceBinding{{
			Name: "beth",
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: "pool-party",
			},
		}},
	}, {
		name: "Successfully bound with emptyDir",
		declarations: []v1.WorkspaceDeclaration{{
			Name: "beth",
		}},
		bindings: []v1.WorkspaceBinding{{
			Name:     "beth",
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		}},
	}, {
		name: "Included optional workspace",
		declarations: []v1.WorkspaceDeclaration{{
			Name:     "beth",
			Optional: true,
		}},
		bindings: []v1.WorkspaceBinding{{
			Name:     "beth",
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		}},
	}, {
		name: "Omitted optional workspace",
		declarations: []v1.WorkspaceDeclaration{{
			Name:     "beth",
			Optional: true,
		}},
		bindings: []v1.WorkspaceBinding{},
	}} {
		t.Run(tc.name, func(t *testing.T) {
			if err := workspace.ValidateBindings(context.Background(), tc.declarations, tc.bindings); err != nil {
				t.Errorf("didnt expect error for valid bindings but got: %v", err)
			}
		})
	}
}

func TestValidateBindingsInvalid(t *testing.T) {
	for _, tc := range []struct {
		name         string
		declarations []v1.WorkspaceDeclaration
		bindings     []v1.WorkspaceBinding
	}{{
		name: "Didn't provide binding matching declared workspace",
		declarations: []v1.WorkspaceDeclaration{{
			Name: "beth",
		}},
		bindings: []v1.WorkspaceBinding{{
			Name:     "kate",
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		}, {
			Name:     "beth",
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		}},
	}, {
		name: "Provided a binding that wasn't needed",
		declarations: []v1.WorkspaceDeclaration{{
			Name: "randall",
		}, {
			Name: "beth",
		}},
		bindings: []v1.WorkspaceBinding{{
			Name:     "beth",
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		}},
	}, {
		name: "Provided both pvc and emptydir",
		declarations: []v1.WorkspaceDeclaration{{
			Name: "beth",
		}},
		bindings: []v1.WorkspaceBinding{{
			Name:     "beth",
			EmptyDir: &corev1.EmptyDirVolumeSource{},
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: "pool-party",
			},
		}},
	}, {
		name: "Provided neither pvc nor emptydir",
		declarations: []v1.WorkspaceDeclaration{{
			Name: "beth",
		}},
		bindings: []v1.WorkspaceBinding{{
			Name: "beth",
		}},
	}, {
		name: "Provided pvc without claim name",
		declarations: []v1.WorkspaceDeclaration{{
			Name: "beth",
		}},
		bindings: []v1.WorkspaceBinding{{
			Name:                  "beth",
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{},
		}},
	}, {
		name: "Mismatch between declarations and bindings",
		declarations: []v1.WorkspaceDeclaration{{
			Name:     "Notbeth",
			Optional: true,
		}},
		bindings: []v1.WorkspaceBinding{{
			Name:     "beth",
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		}},
	}} {
		t.Run(tc.name, func(t *testing.T) {
			if err := workspace.ValidateBindings(context.Background(), tc.declarations, tc.bindings); err == nil {
				t.Errorf("expected error for invalid bindings but didn't get any!")
			}
		})
	}
}

func TestValidateOnlyOnePVCIsUsed_Valid(t *testing.T) {
	for _, tc := range []struct {
		name     string
		bindings []v1.WorkspaceBinding
	}{{
		name:     "an error is not returned when no bindings are given",
		bindings: []v1.WorkspaceBinding{},
	}, {
		name: "an error is not returned when volume claims are not used",
		bindings: []v1.WorkspaceBinding{{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		}, {
			Secret: &corev1.SecretVolumeSource{},
		}},
	}, {
		name: "an error is not returned when one PV claim is used in two bindings",
		bindings: []v1.WorkspaceBinding{{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: "foo",
			},
		}, {
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: "foo",
			},
		}},
	}, {
		name: "an error is not returned when one PV claim is used in two bindings with different subpaths",
		bindings: []v1.WorkspaceBinding{{
			SubPath: "/pathA",
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: "foo",
			},
		}, {
			SubPath: "/pathB",
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: "foo",
			},
		}},
	}} {
		t.Run(tc.name, func(t *testing.T) {
			if err := workspace.ValidateOnlyOnePVCIsUsed(tc.bindings); err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidateOnlyOnePVCIsUsed_Invalid(t *testing.T) {
	validationError := errors.New("more than one PersistentVolumeClaim is bound")
	for _, tc := range []struct {
		name     string
		bindings []v1.WorkspaceBinding
		wantErr  error
	}{{
		name: "an error is returned when two different PV claims are used",
		bindings: []v1.WorkspaceBinding{{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: "foo",
			},
		}, {
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: "bar",
			},
		}},
		wantErr: validationError,
	}, {
		name: "an error is returned when a PVC and volume claim template are mixed",
		bindings: []v1.WorkspaceBinding{{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: "foo",
			},
		}, {
			Name:                "bar",
			VolumeClaimTemplate: &corev1.PersistentVolumeClaim{},
		}},
		wantErr: validationError,
	}} {
		t.Run(tc.name, func(t *testing.T) {
			err := workspace.ValidateOnlyOnePVCIsUsed(tc.bindings)
			if err == nil || (tc.wantErr.Error() != err.Error()) {
				t.Errorf("expected %v received %v", tc.wantErr, err)
			}
		})
	}
}
