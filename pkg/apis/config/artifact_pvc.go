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

package config

import (
	"os"

	corev1 "k8s.io/api/core/v1"
)

const (
	// DefaultPVCSize is the default size of the PVC to create
	DefaultPVCSize = "5Gi"

	// PVCSizeKey is the name of the configmap entry that specifies the size of the PVC to create
	PVCSizeKey = "size"

	// PVCStorageClassNameKey is the name of the configmap entry that specifies the storage class of the PVC to create
	PVCStorageClassNameKey = "storageClassName"
)

// ArtifactPVC holds the configurations for the artifacts PVC
// +k8s:deepcopy-gen=true
type ArtifactPVC struct {
	Size             string
	StorageClassName string
}

// GetArtifactPVCConfigName returns the name of the configmap containing all
// customizations for the storage PVC.
func GetArtifactPVCConfigName() string {
	if e := os.Getenv("CONFIG_ARTIFACT_PVC_NAME"); e != "" {
		return e
	}
	return "config-artifact-pvc"
}

// Equals returns true if two Configs are identical
func (cfg *ArtifactPVC) Equals(other *ArtifactPVC) bool {
	if cfg == nil && other == nil {
		return true
	}

	if cfg == nil || other == nil {
		return false
	}

	return other.Size == cfg.Size &&
		other.StorageClassName == cfg.StorageClassName
}

// NewArtifactPVCFromMap returns a Config given a map corresponding to a ConfigMap
func NewArtifactPVCFromMap(cfgMap map[string]string) (*ArtifactPVC, error) {
	tc := ArtifactPVC{
		Size: DefaultPVCSize,
	}

	if size, ok := cfgMap[PVCSizeKey]; ok {
		tc.Size = size
	}

	if storageClassName, ok := cfgMap[PVCStorageClassNameKey]; ok {
		tc.StorageClassName = storageClassName
	}

	return &tc, nil
}

// NewArtifactPVCFromConfigMap returns a Config for the given configmap
func NewArtifactPVCFromConfigMap(config *corev1.ConfigMap) (*ArtifactPVC, error) {
	return NewArtifactPVCFromMap(config.Data)
}
