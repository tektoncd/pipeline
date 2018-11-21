/*
Copyright 2018 The Knative Authors

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

package resources

import (
	"path/filepath"

	"fmt"

	"github.com/knative/build-pipeline/pkg/apis/pipeline/v1alpha1"
	buildv1alpha1 "github.com/knative/build/pkg/apis/build/v1alpha1"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
)

var (
	pvcDir       = "/pvc"
	workspaceDir = "/workspace"
)

// WrapPostBuildSteps adds steps to run after sources and before steps to override the source with passedconstraint source
// also adds step to collect in the end to collect
func WrapPostBuildSteps(
	taskRun *v1alpha1.TaskRun,
	b *buildv1alpha1.Build,
	logger *zap.SugaredLogger,
) {
	var postBuildSteps = make(map[string][]string)
	for _, postStep := range taskRun.Spec.PostBuiltSteps {
		postBuildSteps[postStep.Name] = postStep.Paths
	}

	for _, source := range b.Spec.Sources {
		if paths, ok := postBuildSteps[source.Name]; ok {
			var newSteps []corev1.Container
			for _, path := range paths {
				var sPath string
				if source.TargetPath == "" {
					sPath = workspaceDir
				} else {
					sPath = filepath.Join(workspaceDir, source.TargetPath)
				}
				newSteps = append(newSteps, []corev1.Container{{
					Name:         fmt.Sprintf("source-copy-make-%s", source.Name),
					Image:        "ubuntu",
					Command:      []string{"/bin/mkdir"},
					Args:         []string{"-p", path},
					VolumeMounts: []corev1.VolumeMount{getPvcMount(taskRun.Spec.PVCName)},
				}, {
					Name:         fmt.Sprintf("source-copy-%s", source.Name),
					Image:        "ubuntu",
					Command:      []string{"/bin/cp"},
					Args:         []string{"-r", fmt.Sprintf("%s/.", sPath), path},
					VolumeMounts: []corev1.VolumeMount{getPvcMount(taskRun.Spec.PVCName)},
				}}...)
			}
			b.Spec.Steps = append(b.Spec.Steps, newSteps...)
		}
	}
}

// WrapPreBuildSteps adds steps to run after sources and before steps to override the source with passedconstraint source
// also adds step to collect in the end to collect
func WrapPreBuildSteps(
	taskRun *v1alpha1.TaskRun,
	b *buildv1alpha1.Build,
	logger *zap.SugaredLogger,
) {
	var preBuildSteps = make(map[string][]string)
	for _, preStep := range taskRun.Spec.PreBuiltSteps {
		preBuildSteps[preStep.Name] = preStep.Paths
	}

	for _, source := range b.Spec.Sources {
		if paths, ok := preBuildSteps[source.Name]; ok {
			var newSteps []corev1.Container
			for i, path := range paths {
				var dPath string
				if source.TargetPath == "" {
					dPath = workspaceDir
				} else {
					dPath = filepath.Join(workspaceDir, source.TargetPath)
				}
				newSteps = append(newSteps, corev1.Container{
					Name:         fmt.Sprintf("source-copy-%s-%d", source.Name, i),
					Image:        "ubuntu",
					Command:      []string{"/bin/cp"},
					Args:         []string{"-r", fmt.Sprintf("%s/.", path), dPath},
					VolumeMounts: []corev1.VolumeMount{getPvcMount(taskRun.Spec.PVCName)},
				})
			}
			b.Spec.Steps = append(newSteps, b.Spec.Steps...)
		}
	}
}

func getPvcMount(name string) corev1.VolumeMount {
	return corev1.VolumeMount{
		Name:      name,   // taskrun pvc name
		MountPath: pvcDir, // nothing should be mounted here
	}
}

// Attach pipelinerun pvc
func GetPVCVolume(name string) corev1.Volume {
	return corev1.Volume{
		Name: name,
		VolumeSource: corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: name},
		},
	}
}
