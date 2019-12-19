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

package v1alpha1

import (
	"github.com/tektoncd/pipeline/pkg/substitution"
	corev1 "k8s.io/api/core/v1"
)

func ApplyContainerReplacements(step *corev1.Container, stringReplacements map[string]string, arrayReplacements map[string][]string) {
	step.Name = substitution.ApplyReplacements(step.Name, stringReplacements)
	step.Image = substitution.ApplyReplacements(step.Image, stringReplacements)

	// Use ApplyArrayReplacements here, as additional args may be added via an array parameter.
	var newArgs []string
	for _, a := range step.Args {
		newArgs = append(newArgs, substitution.ApplyArrayReplacements(a, stringReplacements, arrayReplacements)...)
	}
	step.Args = newArgs

	for ie, e := range step.Env {
		step.Env[ie].Value = substitution.ApplyReplacements(e.Value, stringReplacements)
		if step.Env[ie].ValueFrom != nil {
			if e.ValueFrom.SecretKeyRef != nil {
				step.Env[ie].ValueFrom.SecretKeyRef.LocalObjectReference.Name = substitution.ApplyReplacements(e.ValueFrom.SecretKeyRef.LocalObjectReference.Name, stringReplacements)
				step.Env[ie].ValueFrom.SecretKeyRef.Key = substitution.ApplyReplacements(e.ValueFrom.SecretKeyRef.Key, stringReplacements)
			}
			if e.ValueFrom.ConfigMapKeyRef != nil {
				step.Env[ie].ValueFrom.ConfigMapKeyRef.LocalObjectReference.Name = substitution.ApplyReplacements(e.ValueFrom.ConfigMapKeyRef.LocalObjectReference.Name, stringReplacements)
				step.Env[ie].ValueFrom.ConfigMapKeyRef.Key = substitution.ApplyReplacements(e.ValueFrom.ConfigMapKeyRef.Key, stringReplacements)
			}
		}
	}

	for ie, e := range step.EnvFrom {
		step.EnvFrom[ie].Prefix = substitution.ApplyReplacements(e.Prefix, stringReplacements)
		if e.ConfigMapRef != nil {
			step.EnvFrom[ie].ConfigMapRef.LocalObjectReference.Name = substitution.ApplyReplacements(e.ConfigMapRef.LocalObjectReference.Name, stringReplacements)
		}
		if e.SecretRef != nil {
			step.EnvFrom[ie].SecretRef.LocalObjectReference.Name = substitution.ApplyReplacements(e.SecretRef.LocalObjectReference.Name, stringReplacements)
		}
	}
	step.WorkingDir = substitution.ApplyReplacements(step.WorkingDir, stringReplacements)

	// Use ApplyArrayReplacements here, as additional commands may be added via an array parameter.
	var newCommand []string
	for _, c := range step.Command {
		newCommand = append(newCommand, substitution.ApplyArrayReplacements(c, stringReplacements, arrayReplacements)...)
	}
	step.Command = newCommand

	for iv, v := range step.VolumeMounts {
		step.VolumeMounts[iv].Name = substitution.ApplyReplacements(v.Name, stringReplacements)
		step.VolumeMounts[iv].MountPath = substitution.ApplyReplacements(v.MountPath, stringReplacements)
		step.VolumeMounts[iv].SubPath = substitution.ApplyReplacements(v.SubPath, stringReplacements)
	}
}
