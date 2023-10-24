/*
 *
 * Copyright 2019 The Tekton Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 * /
 */

package v1

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
)

func TestStepGetVarSubstitutionExpressions(t *testing.T) {
	s := Step{
		Name:            "$(tasks.task1.results.stepName)",
		Image:           "$(tasks.task1.results.imageResult)",
		ImagePullPolicy: corev1.PullPolicy("$(tasks.task1.results.imagePullPolicy)"),
		Script:          "substitution within string $(tasks.task1.results.scriptResult)",
		WorkingDir:      "$(tasks.task1.results.workingDir)",
		Command: []string{
			"$(tasks.task2.results.command[*])",
		},
		Args: []string{
			"$(tasks.task2.results.args[*])",
		},
		Env: []corev1.EnvVar{
			{
				Name:  "env1",
				Value: "$(tasks.task2.results.env1)",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						Key: "$(tasks.task2.results.secretKeyRef)",
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "$(tasks.task2.results.secretNameRef)",
						},
					},
					ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
						Key: "$(tasks.task2.results.configMapKeyRef)",
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "$(tasks.task2.results.configMapNameRef)",
						},
					},
				},
			},
		},
	}
	subRefExpressions := s.GetVarSubstitutionExpressions()
	wantRefExpressions := []string{
		"tasks.task1.results.stepName",
		"tasks.task1.results.imageResult",
		"tasks.task1.results.imagePullPolicy",
		"tasks.task1.results.scriptResult",
		"tasks.task1.results.workingDir",
		"tasks.task2.results.command[*]",
		"tasks.task2.results.args[*]",
		"tasks.task2.results.env1",
		"tasks.task2.results.secretKeyRef",
		"tasks.task2.results.secretNameRef",
		"tasks.task2.results.configMapKeyRef",
		"tasks.task2.results.configMapNameRef",
	}
	if d := cmp.Diff(wantRefExpressions, subRefExpressions); d != "" {
		t.Fatalf("Unexpected result (-want, +got): %s", d)
	}
}

func TestSidecarGetVarSubstitutionExpressions(t *testing.T) {
	s := Sidecar{
		Name:            "$(tasks.task1.results.sidecarName)",
		Image:           "$(tasks.task1.results.sidecarImage)",
		ImagePullPolicy: corev1.PullPolicy("$(tasks.task1.results.sidecarImagePullPolicy)"),
		Env: []corev1.EnvVar{
			{
				Name:  "env1",
				Value: "$(tasks.task2.results.sidecarEnv1)",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						Key: "$(tasks.task2.results.sidecarSecretKeyRef)",
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "$(tasks.task2.results.sidecarSecretNameRef)",
						},
					},
					ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
						Key: "$(tasks.task2.results.sidecarConfigMapKeyRef)",
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "$(tasks.task2.results.sidecarConfigMapNameRef)",
						},
					},
				},
			},
		},
	}
	subRefExpressions := s.GetVarSubstitutionExpressions()
	wantRefExpressions := []string{
		"tasks.task1.results.sidecarName",
		"tasks.task1.results.sidecarImage",
		"tasks.task1.results.sidecarImagePullPolicy",
		"tasks.task2.results.sidecarEnv1",
		"tasks.task2.results.sidecarSecretKeyRef",
		"tasks.task2.results.sidecarSecretNameRef",
		"tasks.task2.results.sidecarConfigMapKeyRef",
		"tasks.task2.results.sidecarConfigMapNameRef",
	}
	if d := cmp.Diff(wantRefExpressions, subRefExpressions); d != "" {
		t.Fatalf("Unexpected result (-want, +got): %s", d)
	}
}
