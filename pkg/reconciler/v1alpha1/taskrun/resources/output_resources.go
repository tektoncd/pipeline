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
	"github.com/knative/build-pipeline/pkg/apis/pipeline/v1alpha1"
	buildv1alpha1 "github.com/knative/build/pkg/apis/build/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

var pvcMount = corev1.VolumeMount{
	Name:      "test", // how to choose this path
	MountPath: "/pvc", // nothing should be mounted here
}

// AddStep adds a step in end to copy alll resources into pvc
// /workspace into test/taskname/
// TOOD: (if source has targetpath then use source for copy command to be targetPath)
// func AddStep(task *v1alpha1.Task,
// 	taskRun *v1alpha1.TaskRun,
// 	pipelineResourceLister listers.PipelineResourceLister,
// 	b *buildv1alpha1.Build) error {

// 	if task.Spec.Outputs == nil {
// 		return nil
// 	}
// 	for _, output := range task.Spec.Outputs.Resources {
// 		b.Spec.Steps = append(b.Spec.Steps, corev1.Container{
// 			Name:    fmt.Sprintf("source-copy-%s", output.Name),
// 			Image:   "ubuntu",
// 			Command: []string{"/bin/cp"},
// 			// TODO: (use targetpath in future)
// 			Args:         []string{"-r", "/workspace/", fmt.Sprintf("/test/%s", task.Name)},
// 			VolumeMounts: []corev1.VolumeMount{pvcMount},
// 		})
// 	}
// 	return nil
// }

// OutputStep adds steps to run after sources and before steps to override the source with passedconstraint source
// also adds step to collect in the end to collect
func OutputStep(
	taskRun *v1alpha1.TaskRun,
	b *buildv1alpha1.BuildSpec,
) {
	b.Steps = append(taskRun.Spec.PreBuiltSteps, b.Steps...)
	b.Steps = append(b.Steps, taskRun.Spec.PostBuiltSteps...)
}
