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

package v1beta1_test

import (
	"context"
	"testing"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestWorkspaceBindingValidateValid(t *testing.T) {
	for _, tc := range []struct {
		name    string
		binding *v1beta1.WorkspaceBinding
		wc      func(context.Context) context.Context
	}{{
		name: "Valid PVC",
		binding: &v1beta1.WorkspaceBinding{
			Name: "beth",
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: "pool-party",
			},
		},
	}, {
		name: "Valid volumeClaimTemplate",
		binding: &v1beta1.WorkspaceBinding{
			Name: "beth",
			VolumeClaimTemplate: &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name: "mypvc",
				},
				Spec: corev1.PersistentVolumeClaimSpec{
					AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							"storage": resource.MustParse("1Gi"),
						},
					},
				},
			},
		},
	}, {
		name: "Valid emptyDir",
		binding: &v1beta1.WorkspaceBinding{
			Name:     "beth",
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	}, {
		name: "Valid configMap",
		binding: &v1beta1.WorkspaceBinding{
			Name: "beth",
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: "a-configmap-name",
				},
			},
		},
	}, {
		name: "Valid secret",
		binding: &v1beta1.WorkspaceBinding{
			Name: "beth",
			Secret: &corev1.SecretVolumeSource{
				SecretName: "my-secret",
			},
		},
	}, {
		name: "Valid projected",
		binding: &v1beta1.WorkspaceBinding{
			Name: "beth",
			Projected: &corev1.ProjectedVolumeSource{
				Sources: []corev1.VolumeProjection{{
					ConfigMap: &corev1.ConfigMapProjection{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "a-configmap-name",
						},
					},
				}, {
					Secret: &corev1.SecretProjection{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "my-secret",
						},
					},
				}},
			},
		},
	}, {
		name: "Valid csi",
		binding: &v1beta1.WorkspaceBinding{
			Name: "beth",
			CSI: &corev1.CSIVolumeSource{
				Driver: "my-csi",
			},
		},
	}} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			if tc.wc != nil {
				ctx = tc.wc(ctx)
			}
			if err := tc.binding.Validate(ctx); err != nil {
				t.Errorf("didnt expect error for valid binding but got: %v", err)
			}
		})
	}
}

func TestWorkspaceBindingValidateInvalid(t *testing.T) {
	for _, tc := range []struct {
		name    string
		binding *v1beta1.WorkspaceBinding
		wc      func(context.Context) context.Context
	}{{
		name:    "no binding provided",
		binding: nil,
	}, {
		name: "Provided both pvc and emptydir",
		binding: &v1beta1.WorkspaceBinding{
			Name:     "beth",
			EmptyDir: &corev1.EmptyDirVolumeSource{},
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: "pool-party",
			},
		},
	}, {
		name: "Provided neither pvc nor emptydir",
		binding: &v1beta1.WorkspaceBinding{
			Name: "beth",
		},
	}, {
		name: "Provided pvc without claim name",
		binding: &v1beta1.WorkspaceBinding{
			Name:                  "beth",
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{},
		},
	}, {
		name: "Provide configmap without a name",
		binding: &v1beta1.WorkspaceBinding{
			Name:      "beth",
			ConfigMap: &corev1.ConfigMapVolumeSource{},
		},
	}, {
		name: "Provide secret without a secretName",
		binding: &v1beta1.WorkspaceBinding{
			Name:   "beth",
			Secret: &corev1.SecretVolumeSource{},
		},
	}, {
		name: "Provide projected without sources",
		binding: &v1beta1.WorkspaceBinding{
			Name:      "beth",
			Projected: &corev1.ProjectedVolumeSource{},
		},
	}, {
		name: "Provide csi without a driver",
		binding: &v1beta1.WorkspaceBinding{
			Name: "beth",
			CSI: &corev1.CSIVolumeSource{
				Driver: "",
			},
		},
	}} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			if tc.wc != nil {
				ctx = tc.wc(ctx)
			}
			if err := tc.binding.Validate(ctx); err == nil {
				t.Errorf("expected error for invalid binding but didn't get any!")
			}
		})
	}
}
