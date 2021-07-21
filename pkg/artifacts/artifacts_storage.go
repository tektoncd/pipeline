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
	"context"
	"fmt"
	"strings"

	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	resourcev1alpha1 "github.com/tektoncd/pipeline/pkg/apis/resource/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/apis/resource/v1alpha1/storage"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/kubernetes"
	"knative.dev/pkg/kmeta"
	"knative.dev/pkg/logging"
)

// ArtifactStorageInterface is an interface to define the steps to copy
// an pipeline artifact to/from temporary storage
type ArtifactStorageInterface interface {
	GetCopyToStorageFromSteps(name, sourcePath, destinationPath string) []v1beta1.Step
	GetCopyFromStorageToSteps(name, sourcePath, destinationPath string) []v1beta1.Step
	GetSecretsVolumes() []corev1.Volume
	GetType() string
	StorageBasePath(pr *v1beta1.PipelineRun) string
}

// ArtifactStorageNone is used when no storage is needed.
type ArtifactStorageNone struct{}

// GetCopyToStorageFromSteps returns no containers because none are needed.
func (a *ArtifactStorageNone) GetCopyToStorageFromSteps(name, sourcePath, destinationPath string) []v1beta1.Step {
	return nil
}

// GetCopyFromStorageToSteps returns no containers because none are needed.
func (a *ArtifactStorageNone) GetCopyFromStorageToSteps(name, sourcePath, destinationPath string) []v1beta1.Step {
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
func (a *ArtifactStorageNone) StorageBasePath(pr *v1beta1.PipelineRun) string {
	return ""
}

// InitializeArtifactStorage will check if there is there is a
// bucket configured, create a PVC or return nil if no storage is required.
func InitializeArtifactStorage(ctx context.Context, images pipeline.Images, pr *v1beta1.PipelineRun, ps *v1beta1.PipelineSpec, c kubernetes.Interface) (ArtifactStorageInterface, error) {
	// Artifact storage is needed under the following condition:
	//  Any Task in the pipeline contains an Output resource
	//  AND that Output resource is one of the AllowedOutputResource types.

	needStorage := false
	// Build an index of resources used in the pipeline that are an AllowedOutputResource
	possibleOutputs := sets.NewString()
	for _, r := range ps.Resources {
		if _, ok := v1beta1.AllowedOutputResources[r.Type]; ok {
			possibleOutputs.Insert(r.Name)
		}
	}

	// Use that index to see if any of these are used as OutputResources.
	for _, t := range ps.Tasks {
		if t.Resources != nil {
			for _, o := range t.Resources.Outputs {
				if possibleOutputs.Has(o.Resource) {
					needStorage = true
				}
			}
		}
	}
	if !needStorage {
		return &ArtifactStorageNone{}, nil
	}

	if needsPVC(ctx) {
		pvc, err := createPVC(ctx, pr, c)
		if err != nil {
			return nil, err
		}
		return &storage.ArtifactPVC{Name: pr.Name, PersistentVolumeClaim: pvc, ShellImage: images.ShellImage}, nil
	}

	return newArtifactBucketFromConfig(ctx, images), nil
}

// CleanupArtifactStorage will delete the PipelineRun's artifact storage PVC if it exists. The PVC is created for using
// an output workspace or artifacts from one Task to another Task. No other PVCs will be impacted by this cleanup.
func CleanupArtifactStorage(ctx context.Context, pr *v1beta1.PipelineRun, c kubernetes.Interface) error {

	if needsPVC(ctx) {
		err := deletePVC(ctx, pr, c)
		if err != nil {
			return err
		}
	}
	return nil
}

// needsPVC checks if the Tekton is is configured to use a bucket for artifact storage,
// returning true if instead a PVC is needed.
func needsPVC(ctx context.Context) bool {
	bucketConfig := config.FromContextOrDefaults(ctx).ArtifactBucket
	if bucketConfig == nil {
		return true
	}
	if strings.TrimSpace(bucketConfig.Location) == "" {
		logging.FromContext(ctx).Warnf("the configmap key %q is empty", config.BucketLocationKey)
		return true
	}
	return false
}

// GetArtifactStorage returns the storage interface to enable
// consumer code to get a container step for copy to/from storage
func GetArtifactStorage(ctx context.Context, images pipeline.Images, prName string, c kubernetes.Interface) ArtifactStorageInterface {
	if needsPVC(ctx) {
		return &storage.ArtifactPVC{Name: prName, ShellImage: images.ShellImage}
	}
	return newArtifactBucketFromConfig(ctx, images)
}

// newArtifactBucketFromConfig creates a Bucket from the supplied ConfigMap
func newArtifactBucketFromConfig(ctx context.Context, images pipeline.Images) *storage.ArtifactBucket {
	c := &storage.ArtifactBucket{
		ShellImage:  images.ShellImage,
		GsutilImage: images.GsutilImage,
	}

	bucketConfig := config.FromContextOrDefaults(ctx).ArtifactBucket
	c.Location = bucketConfig.Location
	sp := resourcev1alpha1.SecretParam{}
	if bucketConfig.ServiceAccountSecretName != "" && bucketConfig.ServiceAccountSecretKey != "" {
		sp.SecretName = bucketConfig.ServiceAccountSecretName
		sp.SecretKey = bucketConfig.ServiceAccountSecretKey
		sp.FieldName = bucketConfig.ServiceAccountFieldName
		c.Secrets = append(c.Secrets, sp)
	}
	return c
}

func createPVC(ctx context.Context, pr *v1beta1.PipelineRun, c kubernetes.Interface) (*corev1.PersistentVolumeClaim, error) {
	if _, err := c.CoreV1().PersistentVolumeClaims(pr.Namespace).Get(ctx, GetPVCName(pr), metav1.GetOptions{}); err != nil {
		if errors.IsNotFound(err) {
			pvcConfig := config.FromContextOrDefaults(ctx).ArtifactPVC
			pvcSize, err := resource.ParseQuantity(pvcConfig.Size)
			if err != nil {
				return nil, err
			}

			// The storage class name on pod spec has three states. Tekton doesn't support the empty-string case.
			// - nil if we don't care
			// - "" if we explicitly want to have no class names
			// - "$name" if we want a specific name
			// https://kubernetes.io/docs/concepts/storage/persistent-volumes/#class-1
			var pvcStorageClassName *string
			if pvcConfig.StorageClassName == "" {
				pvcStorageClassName = nil
			} else {
				pvcStorageClassName = &pvcConfig.StorageClassName
			}

			pvcSpec := getPVCSpec(pr, pvcSize, pvcStorageClassName)
			pvc, err := c.CoreV1().PersistentVolumeClaims(pr.Namespace).Create(ctx, pvcSpec, metav1.CreateOptions{})
			if err != nil {
				return nil, fmt.Errorf("failed to claim Persistent Volume %q due to error: %w", pr.Name, err)
			}
			return pvc, nil
		}
		return nil, fmt.Errorf("failed to get claim Persistent Volume %q due to error: %w", pr.Name, err)
	}
	return nil, nil
}

func deletePVC(ctx context.Context, pr *v1beta1.PipelineRun, c kubernetes.Interface) error {
	if _, err := c.CoreV1().PersistentVolumeClaims(pr.Namespace).Get(ctx, GetPVCName(pr), metav1.GetOptions{}); err != nil {
		if !errors.IsNotFound(err) {
			return fmt.Errorf("failed to get Persistent Volume %q due to error: %w", GetPVCName(pr), err)
		}
	} else if err := c.CoreV1().PersistentVolumeClaims(pr.Namespace).Delete(ctx, GetPVCName(pr), metav1.DeleteOptions{}); err != nil {
		return fmt.Errorf("failed to delete Persistent Volume %q due to error: %w", pr.Name, err)
	}
	return nil
}

// getPVCSpec returns the PVC to create for a given PipelineRun
func getPVCSpec(pr *v1beta1.PipelineRun, pvcSize resource.Quantity, storageClassName *string) *corev1.PersistentVolumeClaim {
	return &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       pr.Namespace,
			Name:            GetPVCName(pr),
			OwnerReferences: []metav1.OwnerReference{*kmeta.NewControllerRef(pr)},
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
func GetPVCName(n named) string {
	return fmt.Sprintf("%s-pvc", n.GetName())
}

type named interface {
	GetName() string
}
