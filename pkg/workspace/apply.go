package workspace

import (
	"fmt"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/names"
	corev1 "k8s.io/api/core/v1"
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

// GetVolumes will return a dictionary where the keys are the names of the workspaces bound in
// wb and the value is the Volume to use. If the same Volume is bound twice, the resulting volumes
// will both have the same name to prevent the same Volume from being attached to a pod twice.
func GetVolumes(wb []v1alpha1.WorkspaceBinding) map[string]corev1.Volume {
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

func getDeclaredWorkspace(name string, w []v1alpha1.WorkspaceDeclaration) (*v1alpha1.WorkspaceDeclaration, error) {
	for _, workspace := range w {
		if workspace.Name == name {
			return &workspace, nil
		}
	}
	// Trusting validation to ensure
	return nil, fmt.Errorf("even though validation should have caught it, bound workspace %s did not exist in declared workspaces", name)
}

// Apply will update the StepTemplate and Volumes declaration in ts so that the workspaces
// specified through wb combined with the declared workspaces in ts will be available for
// all containers in the resulting pod.
func Apply(ts v1alpha1.TaskSpec, wb []v1alpha1.WorkspaceBinding) (*v1alpha1.TaskSpec, error) {
	// If there are no bound workspaces, we don't need to do anything
	if len(wb) == 0 {
		return &ts, nil
	}

	v := GetVolumes(wb)
	addedVolumes := map[string]struct{}{}

	// Initialize StepTemplate if it hasn't been already
	if ts.StepTemplate == nil {
		ts.StepTemplate = &corev1.Container{}
	}

	for i := range wb {
		w, err := getDeclaredWorkspace(wb[i].Name, ts.Workspaces)
		if err != nil {
			return nil, err
		}
		// Get the volume we should be using for this binding
		vv := v[wb[i].Name]

		ts.StepTemplate.VolumeMounts = append(ts.StepTemplate.VolumeMounts, corev1.VolumeMount{
			Name:      vv.Name,
			MountPath: w.GetMountPath(),
			SubPath:   wb[i].SubPath,
			ReadOnly:  w.ReadOnly,
		})

		// Only add this volume if it hasn't already been added
		if _, ok := addedVolumes[vv.Name]; !ok {
			ts.Volumes = append(ts.Volumes, vv)
			addedVolumes[vv.Name] = struct{}{}
		}
	}
	return &ts, nil
}
