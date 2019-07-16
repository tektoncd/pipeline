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
	"path/filepath"
	"strings"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/client-go/kubernetes"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	"github.com/tektoncd/pipeline/pkg/names"
	"golang.org/x/xerrors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// DefaultPvcSize is the default size of the PVC to create
	DefaultPvcSize = "5Gi"

	// VolumeMountDirBase will be joined with the name of the Resource itself to produce the location
	// where the volume is mounted
	VolumeMountDirBase = "/volumeresource"
)

// SetupPVC is a PipelineResourceSetupInterface that can idempotently create the PVC
// that is expected by the VolumeResource.
type SetupPVC struct{}

// Setup creates an instance of the PVC required by VolumeResource, unless it already exists.
// The PVC will have the same name as the PipelineResource.
func (n SetupPVC) Setup(r PipelineResourceInterface, o []metav1.OwnerReference, c kubernetes.Interface) error {
	v, ok := r.(*VolumeResource)
	if !ok {
		return xerrors.Errorf("Setup expected to be called with instance of VolumeResource but was called with %v", r)
	}
	return ApplyPVC(v.Name, v.Namespace, &v.StorageClassName, v.ParsedSize, o, c.CoreV1().PersistentVolumeClaims(v.Namespace).Get, c.CoreV1().PersistentVolumeClaims(v.Namespace).Create)
}

// CreatePVC is a function that creates a PVC from the specified spec.
type CreatePVC func(*corev1.PersistentVolumeClaim) (*corev1.PersistentVolumeClaim, error)

// GetPVC retrieves the requested PVC and returns an error if it can't be found.
type GetPVC func(name string, options metav1.GetOptions) (*corev1.PersistentVolumeClaim, error)

// ApplyPVC will create a PVC with the requested name, namespace, size, storageClassName and owner references,
// unless a PVC with the same name in the same namespace already exists.
func ApplyPVC(name, namespace string, storageClassName *string, size resource.Quantity, o []metav1.OwnerReference, get GetPVC, create CreatePVC) error {
	if _, err := get(name, metav1.GetOptions{}); err != nil {
		if !errors.IsNotFound(err) {
			return xerrors.Errorf("failed to retrieve Persistent Volume Claim %q for VolumeResource: %w", name, err)
		}
		pvcSpec := &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Namespace:       namespace,
				Name:            name,
				OwnerReferences: o,
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
				Resources: corev1.ResourceRequirements{
					Requests: map[corev1.ResourceName]resource.Quantity{
						corev1.ResourceStorage: size,
					},
				},
				StorageClassName: storageClassName,
			},
		}
		if _, err = create(pvcSpec); err != nil {
			return xerrors.Errorf("failed to create Persistent Volume Claim %q for VolumeResource: %w", name, err)
		}
	}
	return nil
}

// VolumeResource is a volume which can be used to share data between Tasks.
type VolumeResource struct {
	// Name of the underlying PipelineResource that this was instantiated for.
	Name string
	// Namespace of the underlying PipelineResource that this was instantiated for.
	Namespace string
	// Type of this PipelineResource. Exists because string comparison is easier than introspecting.
	Type PipelineResourceType
	// SubPath is the path to directory on the underlying PVC to either copy from or to. SubPath is always relative
	// to the mount path of the PVC.
	SubPath string
	// StorageClassName is the name of the storage class to use when creating the PVC.
	StorageClassName string
	// ParsedSize is the size that was reuquested by the user for the PVC.
	ParsedSize resource.Quantity

	BashNoopImage string `json:"-"`
}

// GetSetup returns a PipelineResourceSetupInterface ti think we can probably give more visitbhat can create the backing PVC if needed.
func (s VolumeResource) GetSetup() PipelineResourceSetupInterface { return SetupPVC{} }

// NewVolumeResource instantiates the VolumeResource by parsing its params.
func NewVolumeResource(images pipeline.Images, r *PipelineResource) (*VolumeResource, error) {
	if r.Spec.Type != PipelineResourceTypeStorage {
		return nil, xerrors.Errorf("VolumeResource: Cannot create a volume resource from a %s Pipeline Resource", r.Spec.Type)
	}
	s := &VolumeResource{
		Name:          r.Name,
		Namespace:     r.Namespace,
		Type:          r.Spec.Type,
		BashNoopImage: images.BashNoopImage,
	}
	size := DefaultPvcSize
	for _, param := range r.Spec.Params {
		switch {
		case strings.EqualFold(param.Name, "Size"):
			size = param.Value
		case strings.EqualFold(param.Name, "SubPath"):
			s.SubPath = param.Value
		case strings.EqualFold(param.Name, "StorageClassName"):
			s.StorageClassName = param.Value
		}
	}
	var err error
	s.ParsedSize, err = resource.ParseQuantity(size)
	if err != nil {
		return nil, xerrors.Errorf("failed to parse size for VolumeResource %q: %w", r.Name, err)
	}
	return s, nil
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
func (s VolumeResource) Replacements() map[string]string {
	return map[string]string{
		"name":             s.Name,
		"type":             string(s.Type),
		"subPath":          s.SubPath,
		"size":             s.ParsedSize.String(),
		"storageClassName": s.StorageClassName,
	}
}

// GetOutputTaskModifier returns the TaskModifier that adds steps to copy data from the sourcePath
// on disk onto the Volume so it can be persisted.
func (s VolumeResource) GetOutputTaskModifier(_ *TaskSpec, sourcePath string) (TaskModifier, error) {
	// Cant' use filepath.Join because Join does a "clean" which removes the trailing "."
	pathToCopy := fmt.Sprintf("%s/.", sourcePath)
	steps := []Step{
		CreateDirStep(s.BashNoopImage, s.Name, filepath.Join(s.getVolumeMountDirPath(), s.SubPath), []corev1.VolumeMount{s.getPvcMount()}),
		{Container: corev1.Container{
			Name:    names.SimpleNameGenerator.RestrictLengthWithRandomSuffix(fmt.Sprintf("upload-copy-%s", s.Name)),
			Image:   s.BashNoopImage,
			Command: []string{"/ko-app/bash"},
			Args: []string{
				"-args", strings.Join([]string{"cp", "-r", pathToCopy, filepath.Join(s.getVolumeMountDirPath(), s.SubPath)}, " "),
			},
			VolumeMounts: []corev1.VolumeMount{s.getPvcMount()},
		}}}
	return &InternalTaskModifier{
		StepsToAppend: steps,
		Volumes:       s.getVolumeSpec(),
	}, nil
}

// GetInputTaskModifier returns the steps that are needed to copy data from the volume to the
// sourcePath on disk so that the steps in the Task will have access to it.
func (s VolumeResource) GetInputTaskModifier(_ *TaskSpec, sourcePath string) (TaskModifier, error) {
	// Cant' use filepath.Join because Join does a "clean" which removes the trailing "."
	pathToCopy := fmt.Sprintf("%s/.", filepath.Join(s.getVolumeMountDirPath(), s.SubPath))
	steps := []Step{
		CreateDirStep(s.BashNoopImage, s.Name, sourcePath, nil),
		{Container: corev1.Container{
			Name:    names.SimpleNameGenerator.RestrictLengthWithRandomSuffix(fmt.Sprintf("download-copy-%s", s.Name)),
			Image:   s.BashNoopImage,
			Command: []string{"/ko-app/bash"},
			Args: []string{
				"-args", strings.Join([]string{"cp", "-r", pathToCopy, sourcePath}, " "),
			},
			VolumeMounts: []corev1.VolumeMount{s.getPvcMount()},
		}}}
	return &InternalTaskModifier{
		StepsToPrepend: steps,
		Volumes:        s.getVolumeSpec(),
	}, nil
}

func (s VolumeResource) getPvcMount() corev1.VolumeMount {
	return corev1.VolumeMount{
		Name:      s.Name,
		MountPath: s.getVolumeMountDirPath(),
	}
}

func (s VolumeResource) getVolumeMountDirPath() string {
	return fmt.Sprintf("%s-%s", VolumeMountDirBase, s.Name)
}

func (s VolumeResource) getVolumeSpec() []corev1.Volume {
	return []corev1.Volume{{
		Name: s.Name,
		VolumeSource: corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: s.Name},
		},
	}}
}

// GetSecretParams returns nothing because the VolumeResource does not use secrets.
func (s VolumeResource) GetSecretParams() []SecretParam { return nil }
