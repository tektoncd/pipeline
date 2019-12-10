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

package artifacts

import (
	"fmt"
	"os"
	"strings"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/system"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	// PVCSizeKey is the name of the configmap entry that specifies the size of the PVC to create
	PVCSizeKey = "size"

	// DefaultPVCSize is the default size of the PVC to create
	DefaultPVCSize = "5Gi"

	// PVCStorageClassNameKey is the name of the configmap entry that specifies the storage class of the PVC to create
	PVCStorageClassNameKey = "storageClassName"

	// BucketLocationKey is the name of the configmap entry that specifies
	// loction of the bucket.
	BucketLocationKey = "location"

	// BucketServiceAccountSecretName is the name of the configmap entry that specifies
	// the name of the secret that will provide the servie account with bucket access.
	// This secret must  have a key called serviceaccount that will have a value with
	// the service account with access to the bucket
	BucketServiceAccountSecretName = "bucket.service.account.secret.name"

	// BucketServiceAccountSecretKey is the name of the configmap entry that specifies
	// the secret key that will have a value with the service account json with access
	// to the bucket
	BucketServiceAccountSecretKey = "bucket.service.account.secret.key"

	// BucketServiceAccountFieldName is the name of the configmap entry that specifies
	// the field name that should be used for the service account.
	// Valid values: GOOGLE_APPLICATION_CREDENTIALS, BOTO_CONFIG. Defaults to GOOGLE_APPLICATION_CREDENTIALS.
	BucketServiceAccountFieldName = "bucket.service.account.field.name"
)

// GetBucketConfigName returns the name of the configmap containing all
// customizations for the storage bucket.
func GetBucketConfigName() string {
	if e := os.Getenv("CONFIG_ARTIFACT_BUCKET_NAME"); e != "" {
		return e
	}
	return "config-artifact-bucket"
}

// GetPVCConfigName returns the name of the configmap containing all
// customizations for the storage PVC.
func GetPVCConfigName() string {
	if e := os.Getenv("CONFIG_ARTIFACT_PVC_NAME"); e != "" {
		return e
	}
	return "config-artifact-pvc"
}

// ArtifactStorageInterface is an interface to define the steps to copy
// an pipeline artifact to/from temporary storage
type ArtifactStorageInterface interface {
	GetCopyToStorageFromSteps(name, sourcePath, destinationPath string) []v1alpha1.Step
	GetCopyFromStorageToSteps(name, sourcePath, destinationPath string) []v1alpha1.Step
	GetSecretsVolumes() []corev1.Volume
	GetType() string
	StorageBasePath(pr *v1alpha1.PipelineRun) string
}

// ArtifactStorageNone is used when no storage is needed.
type ArtifactStorageNone struct{}

// GetCopyToStorageFromSteps returns no containers because none are needed.
func (a *ArtifactStorageNone) GetCopyToStorageFromSteps(name, sourcePath, destinationPath string) []v1alpha1.Step {
	return nil
}

// GetCopyFromStorageToSteps returns no containers because none are needed.
func (a *ArtifactStorageNone) GetCopyFromStorageToSteps(name, sourcePath, destinationPath string) []v1alpha1.Step {
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
func InitializeArtifactStorage(images pipeline.Images, pr *v1alpha1.PipelineRun, ps *v1alpha1.PipelineSpec, c kubernetes.Interface, logger *zap.SugaredLogger) (ArtifactStorageInterface, error) {
	// Artifact storage is needed under the following condition:
	//  Any Task in the pipeline contains an Output resource
	//  AND that Output resource is one of the AllowedOutputResource types.

	needStorage := false
	// Build an index of resources used in the pipeline that are an AllowedOutputResource
	possibleOutputs := map[string]struct{}{}
	for _, r := range ps.Resources {
		if _, ok := v1alpha1.AllowedOutputResources[r.Type]; ok {
			possibleOutputs[r.Name] = struct{}{}
		}
	}

	// Use that index to see if any of these are used as OutputResources.
	for _, t := range ps.Tasks {
		if t.Resources != nil {
			for _, o := range t.Resources.Outputs {
				if _, ok := possibleOutputs[o.Resource]; ok {
					needStorage = true
				}
			}
		}
	}
	if !needStorage {
		return &ArtifactStorageNone{}, nil
	}

	configMap, err := c.CoreV1().ConfigMaps(system.GetNamespace()).Get(GetBucketConfigName(), metav1.GetOptions{})
	shouldCreatePVC, err := ConfigMapNeedsPVC(configMap, err, logger)
	if err != nil {
		return nil, err
	}
	if shouldCreatePVC {
		pvc, err := createPVC(pr, c)
		if err != nil {
			return nil, err
		}
		return &v1alpha1.ArtifactPVC{Name: pr.Name, PersistentVolumeClaim: pvc, ShellImage: images.ShellImage}, nil
	}

	return NewArtifactBucketConfigFromConfigMap(images)(configMap)
}

// CleanupArtifactStorage will delete the PipelineRun's artifact storage PVC if it exists. The PVC is created for using
// an output workspace or artifacts from one Task to another Task. No other PVCs will be impacted by this cleanup.
func CleanupArtifactStorage(pr *v1alpha1.PipelineRun, c kubernetes.Interface, logger *zap.SugaredLogger) error {
	configMap, err := c.CoreV1().ConfigMaps(system.GetNamespace()).Get(GetBucketConfigName(), metav1.GetOptions{})
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
		return false, fmt.Errorf("couldn't determine if PVC was needed from config map: %w", err)
	}
	if configMap == nil {
		return true, nil
	}
	if configMap.Data == nil {
		logger.Warn("the configmap has no data")
		return true, nil
	}
	location, ok := configMap.Data[BucketLocationKey]
	if !ok {
		return true, nil
	}
	logger.Warnf("the configmap key %q is empty", BucketLocationKey)
	if strings.TrimSpace(location) == "" {
		return true, nil
	}
	return false, nil
}

// GetArtifactStorage returns the storage interface to enable
// consumer code to get a container step for copy to/from storage
func GetArtifactStorage(images pipeline.Images, prName string, c kubernetes.Interface, logger *zap.SugaredLogger) (ArtifactStorageInterface, error) {
	configMap, err := c.CoreV1().ConfigMaps(system.GetNamespace()).Get(GetBucketConfigName(), metav1.GetOptions{})
	pvc, err := ConfigMapNeedsPVC(configMap, err, logger)
	if err != nil {
		return nil, fmt.Errorf("couldn't determine if PVC was needed from config map: %w", err)
	}
	if pvc {
		return &v1alpha1.ArtifactPVC{Name: prName, ShellImage: images.ShellImage}, nil
	}
	return NewArtifactBucketConfigFromConfigMap(images)(configMap)
}

// NewArtifactBucketConfigFromConfigMap creates a Bucket from the supplied ConfigMap
func NewArtifactBucketConfigFromConfigMap(images pipeline.Images) func(configMap *corev1.ConfigMap) (*v1alpha1.ArtifactBucket, error) {
	return func(configMap *corev1.ConfigMap) (*v1alpha1.ArtifactBucket, error) {
		c := &v1alpha1.ArtifactBucket{
			ShellImage:  images.ShellImage,
			GsutilImage: images.GsutilImage,
		}

		if configMap.Data == nil {
			return c, nil
		}
		if location, ok := configMap.Data[BucketLocationKey]; !ok {
			c.Location = ""
		} else {
			c.Location = location
		}
		sp := v1alpha1.SecretParam{}
		if secretName, ok := configMap.Data[BucketServiceAccountSecretName]; ok {
			if secretKey, ok := configMap.Data[BucketServiceAccountSecretKey]; ok {
				sp.FieldName = "GOOGLE_APPLICATION_CREDENTIALS"
				if fieldName, ok := configMap.Data[BucketServiceAccountFieldName]; ok {
					sp.FieldName = fieldName
				}
				sp.SecretName = secretName
				sp.SecretKey = secretKey
				c.Secrets = append(c.Secrets, sp)
			}
		}
		return c, nil
	}
}

func createPVC(pr *v1alpha1.PipelineRun, c kubernetes.Interface) (*corev1.PersistentVolumeClaim, error) {
	if _, err := c.CoreV1().PersistentVolumeClaims(pr.Namespace).Get(GetPVCName(pr), metav1.GetOptions{}); err != nil {
		if errors.IsNotFound(err) {

			configMap, err := c.CoreV1().ConfigMaps(system.GetNamespace()).Get(GetPVCConfigName(), metav1.GetOptions{})
			if err != nil && !errors.IsNotFound(err) {
				return nil, fmt.Errorf("failed to get PVC ConfigMap %s for %q due to error: %w", GetPVCConfigName(), pr.Name, err)
			}
			var pvcSizeStr string
			var pvcStorageClassNameStr string
			if configMap != nil {
				pvcSizeStr = configMap.Data[PVCSizeKey]
				pvcStorageClassNameStr = configMap.Data[PVCStorageClassNameKey]
			}
			if pvcSizeStr == "" {
				pvcSizeStr = DefaultPVCSize
			}
			pvcSize, err := resource.ParseQuantity(pvcSizeStr)
			if err != nil {
				return nil, fmt.Errorf("failed to create Persistent Volume spec for %q due to error: %w", pr.Name, err)
			}
			var pvcStorageClassName *string
			if pvcStorageClassNameStr == "" {
				pvcStorageClassName = nil
			} else {
				pvcStorageClassName = &pvcStorageClassNameStr
			}

			pvcSpec := GetPVCSpec(pr, pvcSize, pvcStorageClassName)
			pvc, err := c.CoreV1().PersistentVolumeClaims(pr.Namespace).Create(pvcSpec)
			if err != nil {
				return nil, fmt.Errorf("failed to claim Persistent Volume %q due to error: %w", pr.Name, err)
			}
			return pvc, nil
		}
		return nil, fmt.Errorf("failed to get claim Persistent Volume %q due to error: %w", pr.Name, err)
	}
	return nil, nil
}

func deletePVC(pr *v1alpha1.PipelineRun, c kubernetes.Interface) error {
	if _, err := c.CoreV1().PersistentVolumeClaims(pr.Namespace).Get(GetPVCName(pr), metav1.GetOptions{}); err != nil {
		if !errors.IsNotFound(err) {
			return fmt.Errorf("failed to get Persistent Volume %q due to error: %w", GetPVCName(pr), err)
		}
	} else if err := c.CoreV1().PersistentVolumeClaims(pr.Namespace).Delete(GetPVCName(pr), &metav1.DeleteOptions{}); err != nil {
		return fmt.Errorf("failed to delete Persistent Volume %q due to error: %w", pr.Name, err)
	}
	return nil
}

// GetPVCSpec returns the PVC to create for a given PipelineRun
func GetPVCSpec(pr *v1alpha1.PipelineRun, pvcSize resource.Quantity, storageClassName *string) *corev1.PersistentVolumeClaim {
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
			StorageClassName: storageClassName,
		},
	}
}

// GetPVCName returns the name that should be used for the PVC for a PipelineRun
func GetPVCName(pr *v1alpha1.PipelineRun) string {
	return fmt.Sprintf("%s-pvc", pr.Name)
}
