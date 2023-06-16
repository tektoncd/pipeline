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

package container

import (
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/substitution"
	corev1 "k8s.io/api/core/v1"
)

// applyStepReplacements returns a StepContainer with variable interpolation applied.
func applyStepReplacements(step *v1.Step, stringReplacements map[string]string, arrayReplacements map[string][]string) {
	c := step.ToK8sContainer()
	applyContainerReplacements(c, stringReplacements, arrayReplacements)
	step.SetContainerFields(*c)
}

// applySidecarReplacements returns a SidecarContainer with variable interpolation applied.
func applySidecarReplacements(sidecar *v1.Sidecar, stringReplacements map[string]string, arrayReplacements map[string][]string) {
	c := sidecar.ToK8sContainer()
	applyContainerReplacements(c, stringReplacements, arrayReplacements)
	sidecar.SetContainerFields(*c)
}

func applyContainerReplacements(c *corev1.Container, stringReplacements map[string]string, arrayReplacements map[string][]string) {
	c.Name = substitution.ApplyReplacements(c.Name, stringReplacements)
	c.Image = substitution.ApplyReplacements(c.Image, stringReplacements)
	c.ImagePullPolicy = corev1.PullPolicy(substitution.ApplyReplacements(string(c.ImagePullPolicy), stringReplacements))

	// Use ApplyArrayReplacements here, as additional args may be added via an array parameter.
	var newArgs []string
	for _, a := range c.Args {
		newArgs = append(newArgs, substitution.ApplyArrayReplacements(a, stringReplacements, arrayReplacements)...)
	}
	c.Args = newArgs

	for ie, e := range c.Env {
		c.Env[ie].Value = substitution.ApplyReplacements(e.Value, stringReplacements)
		if c.Env[ie].ValueFrom != nil {
			if e.ValueFrom.SecretKeyRef != nil {
				c.Env[ie].ValueFrom.SecretKeyRef.LocalObjectReference.Name = substitution.ApplyReplacements(e.ValueFrom.SecretKeyRef.LocalObjectReference.Name, stringReplacements)
				c.Env[ie].ValueFrom.SecretKeyRef.Key = substitution.ApplyReplacements(e.ValueFrom.SecretKeyRef.Key, stringReplacements)
			}
			if e.ValueFrom.ConfigMapKeyRef != nil {
				c.Env[ie].ValueFrom.ConfigMapKeyRef.LocalObjectReference.Name = substitution.ApplyReplacements(e.ValueFrom.ConfigMapKeyRef.LocalObjectReference.Name, stringReplacements)
				c.Env[ie].ValueFrom.ConfigMapKeyRef.Key = substitution.ApplyReplacements(e.ValueFrom.ConfigMapKeyRef.Key, stringReplacements)
			}
		}
	}

	for ie, e := range c.EnvFrom {
		c.EnvFrom[ie].Prefix = substitution.ApplyReplacements(e.Prefix, stringReplacements)
		if e.ConfigMapRef != nil {
			c.EnvFrom[ie].ConfigMapRef.LocalObjectReference.Name = substitution.ApplyReplacements(e.ConfigMapRef.LocalObjectReference.Name, stringReplacements)
		}
		if e.SecretRef != nil {
			c.EnvFrom[ie].SecretRef.LocalObjectReference.Name = substitution.ApplyReplacements(e.SecretRef.LocalObjectReference.Name, stringReplacements)
		}
	}
	c.WorkingDir = substitution.ApplyReplacements(c.WorkingDir, stringReplacements)

	// Use ApplyArrayReplacements here, as additional commands may be added via an array parameter.
	var newCommand []string
	for _, c := range c.Command {
		newCommand = append(newCommand, substitution.ApplyArrayReplacements(c, stringReplacements, arrayReplacements)...)
	}
	c.Command = newCommand

	for iv, v := range c.VolumeMounts {
		c.VolumeMounts[iv].Name = substitution.ApplyReplacements(v.Name, stringReplacements)
		c.VolumeMounts[iv].MountPath = substitution.ApplyReplacements(v.MountPath, stringReplacements)
		c.VolumeMounts[iv].SubPath = substitution.ApplyReplacements(v.SubPath, stringReplacements)
	}
}
