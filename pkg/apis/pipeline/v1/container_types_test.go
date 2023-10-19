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

package v1

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	v1 "k8s.io/api/core/v1"
)

func TestSidecar_ToK8sContainer(t *testing.T) {
	tests := []struct {
		name string
		args *Sidecar
		want *v1.Container
	}{
		{
			name: "test converted to expect k8s container",
			args: &Sidecar{
				Name:             "test",
				Image:            "nginx",
				Command:          []string{"test cmd"},
				Args:             []string{"test args"},
				WorkingDir:       "test dir",
				Ports:            []v1.ContainerPort{{ContainerPort: 8080}},
				EnvFrom:          []v1.EnvFromSource{{Prefix: "test env"}},
				Env:              []v1.EnvVar{{Name: "test env"}},
				ComputeResources: v1.ResourceRequirements{},
				VolumeMounts:     []v1.VolumeMount{{Name: "test volume"}},
				VolumeDevices:    []v1.VolumeDevice{{Name: "test device"}},
				LivenessProbe: &v1.Probe{
					InitialDelaySeconds: 10,
					TimeoutSeconds:      10,
					PeriodSeconds:       10,
					SuccessThreshold:    3,
					FailureThreshold:    3,
				},
				ReadinessProbe: &v1.Probe{
					InitialDelaySeconds: 10,
					TimeoutSeconds:      10,
					PeriodSeconds:       10,
					SuccessThreshold:    3,
					FailureThreshold:    3,
				},
				StartupProbe: &v1.Probe{
					InitialDelaySeconds: 10,
					TimeoutSeconds:      10,
					PeriodSeconds:       10,
					SuccessThreshold:    3,
					FailureThreshold:    3,
				},
				TerminationMessagePath: "test",
				ImagePullPolicy:        "IfNotPresent",
			},
			want: &v1.Container{
				Name:          "test",
				Image:         "nginx",
				Command:       []string{"test cmd"},
				Args:          []string{"test args"},
				WorkingDir:    "test dir",
				Ports:         []v1.ContainerPort{{ContainerPort: 8080}},
				EnvFrom:       []v1.EnvFromSource{{Prefix: "test env"}},
				Env:           []v1.EnvVar{{Name: "test env"}},
				Resources:     v1.ResourceRequirements{},
				VolumeMounts:  []v1.VolumeMount{{Name: "test volume"}},
				VolumeDevices: []v1.VolumeDevice{{Name: "test device"}},
				LivenessProbe: &v1.Probe{
					InitialDelaySeconds: 10,
					TimeoutSeconds:      10,
					PeriodSeconds:       10,
					SuccessThreshold:    3,
					FailureThreshold:    3,
				},
				ReadinessProbe: &v1.Probe{
					InitialDelaySeconds: 10,
					TimeoutSeconds:      10,
					PeriodSeconds:       10,
					SuccessThreshold:    3,
					FailureThreshold:    3,
				},
				StartupProbe: &v1.Probe{
					InitialDelaySeconds: 10,
					TimeoutSeconds:      10,
					PeriodSeconds:       10,
					SuccessThreshold:    3,
					FailureThreshold:    3,
				},
				TerminationMessagePath: "test",
				ImagePullPolicy:        "IfNotPresent",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.args.ToK8sContainer()
			if !cmp.Equal(got, tt.want) {
				t.Errorf("ToK8sContainer() diff: %v", cmp.Diff(got, tt.want, cmpopts.IgnoreUnexported()))
			}
		})
	}
}
