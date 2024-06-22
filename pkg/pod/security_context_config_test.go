/*
Copyright 2024 The Tekton Authors

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

package pod

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
)

func TestGetSecurityContext(t *testing.T) {
	tests := []struct {
		name                    string
		config                  SecurityContextConfig
		isWindows               bool
		expectedSecurityContext *corev1.SecurityContext
	}{
		{
			name: "Linux with read-only root filesystem",
			config: SecurityContextConfig{
				SetSecurityContext:        true,
				SetReadOnlyRootFilesystem: true,
			},
			isWindows: false,
			expectedSecurityContext: &corev1.SecurityContext{
				AllowPrivilegeEscalation: &allowPrivilegeEscalation,
				Capabilities: &corev1.Capabilities{
					Drop: []corev1.Capability{"ALL"},
				},
				RunAsNonRoot:           &runAsNonRoot,
				SeccompProfile:         &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault},
				ReadOnlyRootFilesystem: &readOnlyRootFilesystem,
			},
		},
		{
			name: "Linux without read-only root filesystem",
			config: SecurityContextConfig{
				SetSecurityContext:        true,
				SetReadOnlyRootFilesystem: false,
			},
			isWindows: false,
			expectedSecurityContext: &corev1.SecurityContext{
				AllowPrivilegeEscalation: &allowPrivilegeEscalation,
				Capabilities: &corev1.Capabilities{
					Drop: []corev1.Capability{"ALL"},
				},
				RunAsNonRoot:   &runAsNonRoot,
				SeccompProfile: &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault},
			},
		},
		{
			name: "Windows",
			config: SecurityContextConfig{
				SetSecurityContext:        true,
				SetReadOnlyRootFilesystem: true,
			},
			isWindows: true,
			expectedSecurityContext: &corev1.SecurityContext{
				RunAsNonRoot: &runAsNonRoot,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.config.GetSecurityContext(tt.isWindows)
			if diff := cmp.Diff(tt.expectedSecurityContext, got); diff != "" {
				t.Errorf("GetSecurityContext() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
