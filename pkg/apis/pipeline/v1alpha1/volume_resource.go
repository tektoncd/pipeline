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
	"fmt"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"path/filepath"
	"strings"

	"github.com/tektoncd/pipeline/pkg/names"
	"golang.org/x/xerrors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	// DefaultPvcSize is the default size of the PVC to create
	DefaultPvcSize = "5Gi"

	// VolumeMountDir is where the volume resource will be mounted
	VolumeMountDir = "/volumeresource"
)

// VolumeResource is a volume from which to get artifacts which is required
// by a Build/Task for context (e.g. a archive from which to build an image).
type VolumeResource struct {
	Name           string               `json:"name"`
	Type           PipelineResourceType `json:"type"`
	DestinationDir string               `json:"destinationDir"`
	SourceDir      string               `json:"sourceDir"`
	Size           string               `json:"size"`
}

// NewVolumeResource creates a new volume resource to pass to a Task
func NewVolumeResource(r *PipelineResource) (*VolumeResource, error) {
	if r.Spec.Type != PipelineResourceTypeVolume {
		return nil, xerrors.Errorf("VolumeResource: Cannot create a volume resource from a %s Pipeline Resource", r.Spec.Type)
	}
	size := DefaultPvcSize

	for _, param := range r.Spec.Params {
		switch {
		case strings.EqualFold(param.Name, "Size"):
			size = param.Value
		}
	}

	return &VolumeResource{
		Name: r.Name,
		Type: r.Spec.Type,
		Size: size,
	}, nil
}

// GetName returns the name of the resource
func (s VolumeResource) GetName() string {
	return s.Name
}

// GetType returns the type of the resource, in this case "volume"
func (s VolumeResource) GetType() PipelineResourceType {
	return PipelineResourceTypeVolume
}

// Replacements is used for template replacement on an VolumeResource inside of a Taskrun.
func (s *VolumeResource) Replacements() map[string]string {
	return map[string]string{
		"name": s.Name,
		"type": string(s.Type),
		"path": s.DestinationDir,
	}
}

// SetSourceDirectory sets the source directory at runtime like where is the resource going to be copied from
func (s *VolumeResource) SetSourceDirectory(srcDir string) { s.SourceDir = srcDir }

// SetDestinationDirectory sets the destination directory at runtime like where is the resource going to be copied to
func (s *VolumeResource) SetDestinationDirectory(destDir string) { s.DestinationDir = destDir }

// GetVolume returns the volume for this resource, creating it if necessary.
func (s *VolumeResource) GetVolume(c kubernetes.Interface, tr *TaskRun) (*corev1.Volume, error) {
	_, err := c.CoreV1().PersistentVolumeClaims(tr.Namespace).Get(s.Name, metav1.GetOptions{})
	if err != nil {
		if !errors.IsNotFound(err) {
			return nil, xerrors.Errorf("failed to get claim Persistent Volume %q due to error: %w", tr.Name, err)
		}
		pvcSize, err := resource.ParseQuantity(s.Size)
		if err != nil {
			return nil, xerrors.Errorf("failed to create Persistent Volume spec for %q due to error: %w", tr.Name, err)
		}
		pvcSpec := s.GetPVCSpec(tr, pvcSize)
		_, err = c.CoreV1().PersistentVolumeClaims(tr.Namespace).Create(pvcSpec)
		if err != nil {
			return nil, xerrors.Errorf("failed to claim Persistent Volume %q due to error: %w", tr.Name, err)
		}
	}

	return &corev1.Volume{
		Name: s.Name,
		VolumeSource: corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: s.Name},
		},
	}, nil
}

// GetUploadContainerSpec gets container spec for gcs resource to be uploaded like
// set environment variable from secret params and set volume mounts for those secrets
func (s *VolumeResource) GetUploadContainerSpec() ([]corev1.Container, error) {
	if s.DestinationDir == "" {
		return nil, xerrors.Errorf("VolumeResource: Expect Destination Directory param to be set: %s", s.Name)
	}
	return []corev1.Container{{
		Name:    names.SimpleNameGenerator.RestrictLengthWithRandomSuffix(fmt.Sprintf("upload-mkdir-%s", s.Name)),
		Image:   *BashNoopImage,
		Command: []string{"/ko-app/bash"},
		Args: []string{
			"-args", strings.Join([]string{"mkdir", "-p", filepath.Join(VolumeMountDir, s.DestinationDir)}, " "),
		},
		VolumeMounts: []corev1.VolumeMount{s.GetPvcMount()},
	}, {
		Name:    names.SimpleNameGenerator.RestrictLengthWithRandomSuffix(fmt.Sprintf("upload-copy-%s", s.Name)),
		Image:   *BashNoopImage,
		Command: []string{"/ko-app/bash"},
		Args: []string{
			"-args", strings.Join([]string{"cp", "-r", fmt.Sprintf("%s/.", s.SourceDir), filepath.Join(VolumeMountDir, s.DestinationDir)}, " "),
		},
		VolumeMounts: []corev1.VolumeMount{s.GetPvcMount()},
	}}, nil
}

// GetDownloadContainerSpec returns an array of container specs to download gcs storage object
func (s *VolumeResource) GetDownloadContainerSpec() ([]corev1.Container, error) {
	if s.DestinationDir == "" {
		return nil, xerrors.Errorf("VolumeResource: Expect Destination Directory param to be set %s", s.Name)
	}

	return []corev1.Container{
		CreateDirContainer(s.Name, s.DestinationDir), {
			Name:    names.SimpleNameGenerator.RestrictLengthWithRandomSuffix(fmt.Sprintf("download-copy-%s", s.Name)),
			Image:   *BashNoopImage,
			Command: []string{"/ko-app/bash"},
			Args: []string{
				"-args", strings.Join([]string{"cp", "-r", fmt.Sprintf("%s/.", filepath.Join(VolumeMountDir, s.SourceDir)), s.DestinationDir}, " "),
			},
			VolumeMounts: []corev1.VolumeMount{s.GetPvcMount()},
		}}, nil
}

// GetPVCSpec returns the PVC to create for a given TaskRun
func (s *VolumeResource) GetPVCSpec(tr *TaskRun, pvcSize resource.Quantity) *corev1.PersistentVolumeClaim {
	return &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       tr.Namespace,
			Name:            s.Name,
			OwnerReferences: tr.GetOwnerReference(),
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources: corev1.ResourceRequirements{
				Requests: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceStorage: pvcSize,
				},
			},
		},
	}
}

// GetPvcMount returns a mounting of the volume with the mount path /volumeresource
func (s *VolumeResource) GetPvcMount() corev1.VolumeMount {
	return corev1.VolumeMount{
		Name:      s.Name,         // resource pvc name
		MountPath: VolumeMountDir, // nothing should be mounted here
	}
}
