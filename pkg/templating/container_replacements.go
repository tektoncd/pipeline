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

package templating

import (
	corev1 "k8s.io/api/core/v1"
)

func ApplyContainerReplacements(container *corev1.Container, stringReplacements map[string]string, arrayReplacements map[string][]string) {
	container.Name = ApplyReplacements(container.Name, stringReplacements)
	container.Image = ApplyReplacements(container.Image, stringReplacements)

	//Use ApplyArrayReplacements here, as additional args may be added via an array parameter.
	var newArgs []string
	for _, a := range container.Args {
		newArgs = append(newArgs, ApplyArrayReplacements(a, stringReplacements, arrayReplacements)...)
	}
	container.Args = newArgs

	for ie, e := range container.Env {
		container.Env[ie].Value = ApplyReplacements(e.Value, stringReplacements)
		if container.Env[ie].ValueFrom != nil {
			if e.ValueFrom.SecretKeyRef != nil {
				container.Env[ie].ValueFrom.SecretKeyRef.LocalObjectReference.Name = ApplyReplacements(e.ValueFrom.SecretKeyRef.LocalObjectReference.Name, stringReplacements)
				container.Env[ie].ValueFrom.SecretKeyRef.Key = ApplyReplacements(e.ValueFrom.SecretKeyRef.Key, stringReplacements)
			}
			if e.ValueFrom.ConfigMapKeyRef != nil {
				container.Env[ie].ValueFrom.ConfigMapKeyRef.LocalObjectReference.Name = ApplyReplacements(e.ValueFrom.ConfigMapKeyRef.LocalObjectReference.Name, stringReplacements)
				container.Env[ie].ValueFrom.ConfigMapKeyRef.Key = ApplyReplacements(e.ValueFrom.ConfigMapKeyRef.Key, stringReplacements)
			}
		}
	}

	for ie, e := range container.EnvFrom {
		container.EnvFrom[ie].Prefix = ApplyReplacements(e.Prefix, stringReplacements)
		if e.ConfigMapRef != nil {
			container.EnvFrom[ie].ConfigMapRef.LocalObjectReference.Name = ApplyReplacements(e.ConfigMapRef.LocalObjectReference.Name, stringReplacements)
		}
		if e.SecretRef != nil {
			container.EnvFrom[ie].SecretRef.LocalObjectReference.Name = ApplyReplacements(e.SecretRef.LocalObjectReference.Name, stringReplacements)
		}
	}
	container.WorkingDir = ApplyReplacements(container.WorkingDir, stringReplacements)

	//Use ApplyArrayReplacements here, as additional commands may be added via an array parameter.
	var newCommand []string
	for _, c := range container.Command {
		newCommand = append(newCommand, ApplyArrayReplacements(c, stringReplacements, arrayReplacements)...)
	}
	container.Command = newCommand

	for iv, v := range container.VolumeMounts {
		container.VolumeMounts[iv].Name = ApplyReplacements(v.Name, stringReplacements)
		container.VolumeMounts[iv].MountPath = ApplyReplacements(v.MountPath, stringReplacements)
		container.VolumeMounts[iv].SubPath = ApplyReplacements(v.SubPath, stringReplacements)
	}
}
