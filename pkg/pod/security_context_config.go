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
	corev1 "k8s.io/api/core/v1"
)

var (
	// Used in security context of pod init containers
	allowPrivilegeEscalation = false
	runAsNonRoot             = true
	readOnlyRootFilesystem   = true

	// LinuxSecurityContext allow init containers to run in namespaces
	// with "restricted" pod security admission
	// See https://kubernetes.io/docs/concepts/security/pod-security-standards/#restricted
	LinuxSecurityContext = &corev1.SecurityContext{
		AllowPrivilegeEscalation: &allowPrivilegeEscalation,
		Capabilities: &corev1.Capabilities{
			Drop: []corev1.Capability{"ALL"},
		},
		RunAsNonRoot: &runAsNonRoot,
		SeccompProfile: &corev1.SeccompProfile{
			Type: corev1.SeccompProfileTypeRuntimeDefault,
		},
	}

	// WindowsSecurityContext adds securityContext that is supported by Windows OS.
	WindowsSecurityContext = &corev1.SecurityContext{
		RunAsNonRoot: &runAsNonRoot,
	}
)

// SecurityContextConfig is configuration for setting security context for init containers and affinity assistant container.
type SecurityContextConfig struct {
	SetSecurityContext        bool
	SetReadOnlyRootFilesystem bool
}

func (c SecurityContextConfig) GetSecurityContext(isWindows bool) *corev1.SecurityContext {
	if isWindows {
		return WindowsSecurityContext
	}

	if !c.SetReadOnlyRootFilesystem {
		return LinuxSecurityContext
	}

	securityContext := LinuxSecurityContext.DeepCopy()
	securityContext.ReadOnlyRootFilesystem = &readOnlyRootFilesystem
	return securityContext
}
