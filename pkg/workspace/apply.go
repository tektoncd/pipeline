/*
Copyright 2020 The Tekton Authors

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

package workspace

import (
	"context"
	"fmt"

	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/pkg/names"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"
)

const (
	volumeNameBase = "ws"
)

// nameVolumeMap is a map from a workspace's name to its Volume.
type nameVolumeMap map[string]corev1.Volume

// setVolumeSource assigns a volume to a workspace's name.
func (nvm nameVolumeMap) setVolumeSource(workspaceName string, volumeName string, source corev1.VolumeSource) {
	nvm[workspaceName] = corev1.Volume{
		Name:         volumeName,
		VolumeSource: source,
	}
}

// CreateVolumes will return a dictionary where the keys are the names of the workspaces bound in
// wb and the value is a newly-created Volume to use. If the same Volume is bound twice, the
// resulting volumes will both have the same name to prevent the same Volume from being attached
// to a pod twice. The names of the returned volumes will be a short random string starting "ws-".
func CreateVolumes(wb []v1beta1.WorkspaceBinding) map[string]corev1.Volume {
	pvcs := map[string]corev1.Volume{}
	v := make(nameVolumeMap)
	for _, w := range wb {
		name := names.SimpleNameGenerator.RestrictLengthWithRandomSuffix(volumeNameBase)
		switch {
		case w.PersistentVolumeClaim != nil:
			// If it's a PVC, we need to check if we've encountered it before so we avoid mounting it twice
			if vv, ok := pvcs[w.PersistentVolumeClaim.ClaimName]; ok {
				v[w.Name] = vv
			} else {
				pvc := *w.PersistentVolumeClaim
				v.setVolumeSource(w.Name, name, corev1.VolumeSource{PersistentVolumeClaim: &pvc})
				pvcs[pvc.ClaimName] = v[w.Name]
			}
		case w.EmptyDir != nil:
			ed := *w.EmptyDir
			v.setVolumeSource(w.Name, name, corev1.VolumeSource{EmptyDir: &ed})
		case w.ConfigMap != nil:
			cm := *w.ConfigMap
			v.setVolumeSource(w.Name, name, corev1.VolumeSource{ConfigMap: &cm})
		case w.Secret != nil:
			s := *w.Secret
			v.setVolumeSource(w.Name, name, corev1.VolumeSource{Secret: &s})
		}
	}
	return v
}

func getDeclaredWorkspace(name string, w []v1beta1.WorkspaceDeclaration) (*v1beta1.WorkspaceDeclaration, error) {
	for _, workspace := range w {
		if workspace.Name == name {
			return &workspace, nil
		}
	}
	// Trusting validation to ensure
	return nil, fmt.Errorf("even though validation should have caught it, bound workspace %s did not exist in declared workspaces", name)
}

// Apply will update the StepTemplate, Sidecars and Volumes declaration in ts so that the workspaces
// specified through wb combined with the declared workspaces in ts will be available for
// all Step and Sidecar containers in the resulting pod.
func Apply(ctx context.Context, ts v1beta1.TaskSpec, wb []v1beta1.WorkspaceBinding, v map[string]corev1.Volume) (*v1beta1.TaskSpec, error) {
	// If there are no bound workspaces, we don't need to do anything
	if len(wb) == 0 {
		return &ts, nil
	}

	addedVolumes := sets.NewString()

	// Initialize StepTemplate if it hasn't been already
	if ts.StepTemplate == nil {
		ts.StepTemplate = &corev1.Container{}
	}

	isolatedWorkspaces := sets.NewString()

	alphaAPIEnabled := config.FromContextOrDefaults(ctx).FeatureFlags.EnableAPIFields == config.AlphaAPIFields

	if alphaAPIEnabled {
		for _, step := range ts.Steps {
			for _, workspaceUsage := range step.Workspaces {
				isolatedWorkspaces.Insert(workspaceUsage.Name)
			}
		}
		for _, sidecar := range ts.Sidecars {
			for _, workspaceUsage := range sidecar.Workspaces {
				isolatedWorkspaces.Insert(workspaceUsage.Name)
			}
		}
	}

	for i := range wb {
		w, err := getDeclaredWorkspace(wb[i].Name, ts.Workspaces)
		if err != nil {
			return nil, err
		}
		// Get the volume we should be using for this binding
		vv := v[wb[i].Name]

		volumeMount := corev1.VolumeMount{
			Name:      vv.Name,
			MountPath: w.GetMountPath(),
			SubPath:   wb[i].SubPath,
			ReadOnly:  w.ReadOnly,
		}

		if alphaAPIEnabled {
			if isolatedWorkspaces.Has(w.Name) {
				mountAsIsolatedWorkspace(ts, w.Name, volumeMount)
			} else {
				mountAsSharedWorkspace(ts, volumeMount)
			}
		} else {
			// Prior to the alpha feature gate only Steps may receive workspaces.
			ts.StepTemplate.VolumeMounts = append(ts.StepTemplate.VolumeMounts, volumeMount)
		}

		// Only add this volume if it hasn't already been added
		if !addedVolumes.Has(vv.Name) {
			ts.Volumes = append(ts.Volumes, vv)
			addedVolumes.Insert(vv.Name)
		}
	}
	return &ts, nil
}

// mountAsSharedWorkspace takes a volumeMount and adds it to all the steps and sidecars in
// a TaskSpec.
func mountAsSharedWorkspace(ts v1beta1.TaskSpec, volumeMount corev1.VolumeMount) {
	ts.StepTemplate.VolumeMounts = append(ts.StepTemplate.VolumeMounts, volumeMount)

	for i := range ts.Sidecars {
		AddSidecarVolumeMount(&ts.Sidecars[i], volumeMount)
	}
}

// mountAsIsolatedWorkspace takes a volumeMount and adds it only to the steps and sidecars
// that have requested access to it.
func mountAsIsolatedWorkspace(ts v1beta1.TaskSpec, workspaceName string, volumeMount corev1.VolumeMount) {
	for i := range ts.Steps {
		step := &ts.Steps[i]
		for _, workspaceUsage := range step.Workspaces {
			if workspaceUsage.Name == workspaceName {
				vm := volumeMount
				if workspaceUsage.MountPath != "" {
					vm.MountPath = workspaceUsage.MountPath
				}
				step.VolumeMounts = append(step.VolumeMounts, vm)
				break
			}
		}
	}
	for i := range ts.Sidecars {
		sidecar := &ts.Sidecars[i]
		for _, workspaceUsage := range sidecar.Workspaces {
			if workspaceUsage.Name == workspaceName {
				vm := volumeMount
				if workspaceUsage.MountPath != "" {
					vm.MountPath = workspaceUsage.MountPath
				}
				sidecar.VolumeMounts = append(sidecar.VolumeMounts, vm)
				break
			}
		}
	}
}

// AddSidecarVolumeMount is a helper to add a volumeMount to the sidecar unless its
// MountPath would conflict with another of the sidecar's existing volume mounts.
func AddSidecarVolumeMount(sidecar *v1beta1.Sidecar, volumeMount corev1.VolumeMount) {
	for j := range sidecar.VolumeMounts {
		if sidecar.VolumeMounts[j].MountPath == volumeMount.MountPath {
			return
		}
	}
	sidecar.VolumeMounts = append(sidecar.VolumeMounts, volumeMount)
}
