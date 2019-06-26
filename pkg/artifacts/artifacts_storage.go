/*
Copyright 2019 The Tekton Authors.

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

package artifacts

import (
	"fmt"
	"strings"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/system"
	"go.uber.org/zap"
	"golang.org/x/xerrors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	// PvcConfigName is the name of the configmap containing all
	// customizations for the storage PVC.
	PvcConfigName = "config-artifact-pvc"

	// PvcSizeKey is the name of the configmap entry that specifies the size of the PVC to create
	PvcSizeKey = "size"

	// DefaultPvcSize is the default size of the PVC to create
	DefaultPvcSize = "5Gi"
)

// ArtifactStorageInterface is an interface to define the steps to copy
// an pipeline artifact to/from temporary storage
type ArtifactStorageInterface interface {
	GetCopyToStorageFromContainerSpec(name, sourcePath, destinationPath string) []corev1.Container
	GetCopyFromStorageToContainerSpec(name, sourcePath, destinationPath string) []corev1.Container
	GetSecretsVolumes() []corev1.Volume
	GetType() string
	StorageBasePath(pr *v1alpha1.PipelineRun) string
}

// ArtifactStorageNone is used when no storage is needed.
type ArtifactStorageNone struct{}

// GetCopyToStorageFromContainerSpec returns no containers because none are needed.
func (a *ArtifactStorageNone) GetCopyToStorageFromContainerSpec(name, sourcePath, destinationPath string) []corev1.Container {
	return nil
}

// GetCopyFromStorageToContainerSpec returns no containers because none are needed.
func (a *ArtifactStorageNone) GetCopyFromStorageToContainerSpec(name, sourcePath, destinationPath string) []corev1.Container {
	return nil
}

// GetSecretsVolumes returns no volumes because none are needed.
func (a *ArtifactStorageNone) GetSecretsVolumes() []corev1.Volume {
	return nil
}

// GetType returns the string "none" to indicate this is the None storage type.
func (a *ArtifactStorageNone) GetType() string {
	return "none"
}

// StorageBasePath returns an empty string because no storage is being used and so
// there is no path that resources should be copied from / to.
func (a *ArtifactStorageNone) StorageBasePath(pr *v1alpha1.PipelineRun) string {
	return ""
}

// InitializeArtifactStorage will check if there is there is a
// bucket configured, create a PVC or return nil if no storage is required.
func InitializeArtifactStorage(pr *v1alpha1.PipelineRun, ts []v1alpha1.PipelineTask, c kubernetes.Interface, logger *zap.SugaredLogger) (ArtifactStorageInterface, error) {
	// Artifact storage is only required if a pipeline has tasks that take inputs from previous tasks.
	needStorage := false
	for _, t := range ts {
		if t.Resources != nil {
			for _, i := range t.Resources.Inputs {
				if len(i.From) != 0 {
					needStorage = true
				}
			}
		}
	}
	if !needStorage {
		return &ArtifactStorageNone{}, nil
	}

	configMap, err := c.CoreV1().ConfigMaps(system.GetNamespace()).Get(v1alpha1.BucketConfigName, metav1.GetOptions{})
	shouldCreatePVC, err := ConfigMapNeedsPVC(configMap, err, logger)
	if err != nil {
		return nil, err
	}
	if shouldCreatePVC {
		pvc, err := createPVC(pr, c)
		if err != nil {
			return nil, err
		}
		return &v1alpha1.ArtifactPVC{Name: pr.Name, PersistentVolumeClaim: pvc}, nil
	}

	return NewArtifactBucketConfigFromConfigMap(configMap)
}

// CleanupArtifactStorage will delete the PipelineRun's artifact storage PVC if it exists. The PVC is created for using
// an output workspace or artifacts from one Task to another Task. No other PVCs will be impacted by this cleanup.
func CleanupArtifactStorage(pr *v1alpha1.PipelineRun, c kubernetes.Interface, logger *zap.SugaredLogger) error {
	configMap, err := c.CoreV1().ConfigMaps(system.GetNamespace()).Get(v1alpha1.BucketConfigName, metav1.GetOptions{})
	shouldCreatePVC, err := ConfigMapNeedsPVC(configMap, err, logger)
	if err != nil {
		return err
	}
	if shouldCreatePVC {
		err = deletePVC(pr, c)
		if err != nil {
			return err
		}
	}
	return nil
}

// ConfigMapNeedsPVC checks if the possibly-nil config map passed to it is configured to use a bucket for artifact storage,
// returning true if instead a PVC is needed.
func ConfigMapNeedsPVC(configMap *corev1.ConfigMap, err error, logger *zap.SugaredLogger) (bool, error) {
	if err != nil {
		if errors.IsNotFound(err) {
			return true, nil
		}
		return false, xerrors.Errorf("couldn't determine if PVC was needed from config map: %w", err)
	}
	if configMap == nil {
		return true, nil
	}
	if configMap.Data == nil {
		logger.Warn("the configmap has no data")
		return true, nil
	}
	if location, ok := configMap.Data[v1alpha1.BucketLocationKey]; !ok {
		return true, nil
	} else {
		logger.Warnf("the configmap key %q is empty", v1alpha1.BucketLocationKey)
		if strings.TrimSpace(location) == "" {
			return true, nil
		}
	}
	return false, nil
}

// GetArtifactStorage returns the storage interface to enable
// consumer code to get a container step for copy to/from storage
func GetArtifactStorage(prName string, c kubernetes.Interface, logger *zap.SugaredLogger) (ArtifactStorageInterface, error) {
	configMap, err := c.CoreV1().ConfigMaps(system.GetNamespace()).Get(v1alpha1.BucketConfigName, metav1.GetOptions{})
	pvc, err := ConfigMapNeedsPVC(configMap, err, logger)
	if err != nil {
		return nil, xerrors.Errorf("couldn't determine if PVC was needed from config map: %w", err)
	}
	if pvc {
		return &v1alpha1.ArtifactPVC{Name: prName}, nil
	}
	return NewArtifactBucketConfigFromConfigMap(configMap)
}

// NewArtifactBucketConfigFromConfigMap creates a Bucket from the supplied ConfigMap
func NewArtifactBucketConfigFromConfigMap(configMap *corev1.ConfigMap) (*v1alpha1.ArtifactBucket, error) {
	c := &v1alpha1.ArtifactBucket{}

	if configMap.Data == nil {
		return c, nil
	}
	if location, ok := configMap.Data[v1alpha1.BucketLocationKey]; !ok {
		c.Location = ""
	} else {
		c.Location = location
	}
	sp := v1alpha1.SecretParam{}
	if secretName, ok := configMap.Data[v1alpha1.BucketServiceAccountSecretName]; ok {
		if secretKey, ok := configMap.Data[v1alpha1.BucketServiceAccountSecretKey]; ok {
			sp.FieldName = "GOOGLE_APPLICATION_CREDENTIALS"
			sp.SecretName = secretName
			sp.SecretKey = secretKey
			c.Secrets = append(c.Secrets, sp)
		}
	}
	return c, nil
}

func createPVC(pr *v1alpha1.PipelineRun, c kubernetes.Interface) (*corev1.PersistentVolumeClaim, error) {
	if _, err := c.CoreV1().PersistentVolumeClaims(pr.Namespace).Get(GetPVCName(pr), metav1.GetOptions{}); err != nil {
		if errors.IsNotFound(err) {

			configMap, err := c.CoreV1().ConfigMaps(system.GetNamespace()).Get(PvcConfigName, metav1.GetOptions{})
			if err != nil && !errors.IsNotFound(err) {
				return nil, xerrors.Errorf("failed to get PVC ConfigMap %s for %q due to error: %w", PvcConfigName, pr.Name, err)
			}
			var pvcSizeStr string
			if configMap != nil {
				pvcSizeStr = configMap.Data[PvcSizeKey]
			}
			if pvcSizeStr == "" {
				pvcSizeStr = DefaultPvcSize
			}
			pvcSize, err := resource.ParseQuantity(pvcSizeStr)
			if err != nil {
				return nil, xerrors.Errorf("failed to create Persistent Volume spec for %q due to error: %w", pr.Name, err)
			}
			pvcSpec := GetPVCSpec(pr, pvcSize)
			pvc, err := c.CoreV1().PersistentVolumeClaims(pr.Namespace).Create(pvcSpec)
			if err != nil {
				return nil, xerrors.Errorf("failed to claim Persistent Volume %q due to error: %w", pr.Name, err)
			}
			return pvc, nil
		}
		return nil, xerrors.Errorf("failed to get claim Persistent Volume %q due to error: %w", pr.Name, err)
	}
	return nil, nil
}

func deletePVC(pr *v1alpha1.PipelineRun, c kubernetes.Interface) error {
	if _, err := c.CoreV1().PersistentVolumeClaims(pr.Namespace).Get(GetPVCName(pr), metav1.GetOptions{}); err != nil {
		if !errors.IsNotFound(err) {
			return xerrors.Errorf("failed to get Persistent Volume %q due to error: %w", GetPVCName(pr), err)
		}
	} else if err := c.CoreV1().PersistentVolumeClaims(pr.Namespace).Delete(GetPVCName(pr), &metav1.DeleteOptions{}); err != nil {
		return xerrors.Errorf("failed to delete Persistent Volume %q due to error: %w", pr.Name, err)
	}
	return nil
}

// GetPVCSpec returns the PVC to create for a given PipelineRun
func GetPVCSpec(pr *v1alpha1.PipelineRun, pvcSize resource.Quantity) *corev1.PersistentVolumeClaim {
	return &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       pr.Namespace,
			Name:            GetPVCName(pr),
			OwnerReferences: pr.GetOwnerReference(),
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

// GetPVCName returns the name that should be used for the PVC for a PipelineRun
func GetPVCName(pr *v1alpha1.PipelineRun) string {
	return fmt.Sprintf("%s-pvc", pr.Name)
}
