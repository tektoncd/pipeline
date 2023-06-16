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

package volumeclaim

import (
	"context"
	"crypto/sha256"
	"fmt"

	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	errorutils "k8s.io/apimachinery/pkg/util/errors"
	clientset "k8s.io/client-go/kubernetes"
)

const (
	// ReasonCouldntCreateWorkspacePVC indicates that a Pipeline expects a workspace from a
	// volumeClaimTemplate but couldn't create a claim.
	ReasonCouldntCreateWorkspacePVC = "CouldntCreateWorkspacePVC"
)

// PvcHandler is used to create PVCs for workspaces
type PvcHandler interface {
	CreatePVCsForWorkspacesWithoutAffinityAssistant(ctx context.Context, wb []v1.WorkspaceBinding, ownerReference metav1.OwnerReference, namespace string) error
}

type defaultPVCHandler struct {
	clientset clientset.Interface
	logger    *zap.SugaredLogger
}

// NewPVCHandler returns a new defaultPVCHandler
func NewPVCHandler(clientset clientset.Interface, logger *zap.SugaredLogger) PvcHandler {
	return &defaultPVCHandler{clientset, logger}
}

// CreatePVCsForWorkspacesWithoutAffinityAssistant checks if a PVC named <claim-name>-<workspace-name>-<owner-name> exists;
// where claim-name is provided by the user in the volumeClaimTemplate, and owner-name is the name of the
// resource with the volumeClaimTemplate declared, a PipelineRun or TaskRun. If the PVC did not exist, a new PVC
// with that name is created with the provided OwnerReference.
// This function is only called when Affinity Assistant is disabled.
// When Affinity Assistant is enabled, the PersistentVolumeClaims will be created by the Affinity Assistant StatefulSet VolumeClaimTemplate instead.
func (c *defaultPVCHandler) CreatePVCsForWorkspacesWithoutAffinityAssistant(ctx context.Context, wb []v1.WorkspaceBinding, ownerReference metav1.OwnerReference, namespace string) error {
	var errs []error
	for _, claim := range getPVCsWithoutAffinityAssistant(wb, ownerReference, namespace) {
		_, err := c.clientset.CoreV1().PersistentVolumeClaims(claim.Namespace).Get(ctx, claim.Name, metav1.GetOptions{})
		switch {
		case apierrors.IsNotFound(err):
			_, err := c.clientset.CoreV1().PersistentVolumeClaims(claim.Namespace).Create(ctx, claim, metav1.CreateOptions{})
			if err != nil && !apierrors.IsAlreadyExists(err) {
				errs = append(errs, fmt.Errorf("failed to create PVC %s: %w", claim.Name, err))
			}

			if apierrors.IsAlreadyExists(err) {
				c.logger.Infof("Tried to create PersistentVolumeClaim %s in namespace %s, but it already exists",
					claim.Name, claim.Namespace)
			}

			if err == nil {
				c.logger.Infof("Created PersistentVolumeClaim %s in namespace %s", claim.Name, claim.Namespace)
			}
		case err != nil:
			errs = append(errs, fmt.Errorf("failed to retrieve PVC %s: %w", claim.Name, err))
		}
	}
	return errorutils.NewAggregate(errs)
}

func getPVCsWithoutAffinityAssistant(workspaceBindings []v1.WorkspaceBinding, ownerReference metav1.OwnerReference, namespace string) map[string]*corev1.PersistentVolumeClaim {
	claims := make(map[string]*corev1.PersistentVolumeClaim)
	for _, workspaceBinding := range workspaceBindings {
		if workspaceBinding.VolumeClaimTemplate == nil {
			continue
		}

		claim := workspaceBinding.VolumeClaimTemplate.DeepCopy()
		claim.Name = GetPVCNameWithoutAffinityAssistant(workspaceBinding.VolumeClaimTemplate.Name, workspaceBinding, ownerReference)
		claim.Namespace = namespace
		claim.OwnerReferences = []metav1.OwnerReference{ownerReference}
		claims[workspaceBinding.Name] = claim
	}
	return claims
}

// GetPVCNameWithoutAffinityAssistant gets the name of PersistentVolumeClaim for a Workspace and PipelineRun or TaskRun. claim
// must be a PersistentVolumeClaim from a volumeClaimTemplate. The returned name must be consistent given the same
// workspaceBinding name and ownerReference UID - because it is first used for creating a PVC and later,
// possibly several TaskRuns to lookup the PVC to mount.
// We use ownerReference UID over ownerReference name to distinguish runs with the same name.
// If the given volumeClaimTemplate name is empty, the prefix "pvc" will be applied to the PersistentVolumeClaim name.
// See function `getPersistentVolumeClaimNameWithAffinityAssistant` when the PersistentVolumeClaim is created by Affinity Assistant StatefulSet.
func GetPVCNameWithoutAffinityAssistant(claimName string, wb v1.WorkspaceBinding, owner metav1.OwnerReference) string {
	if claimName == "" {
		return fmt.Sprintf("%s-%s", "pvc", getPersistentVolumeClaimIdentity(wb.Name, string(owner.UID)))
	}
	return fmt.Sprintf("%s-%s", claimName, getPersistentVolumeClaimIdentity(wb.Name, string(owner.UID)))
}

func getPersistentVolumeClaimIdentity(workspaceName, ownerName string) string {
	hashBytes := sha256.Sum256([]byte(workspaceName + ownerName))
	hashString := fmt.Sprintf("%x", hashBytes)
	return hashString[:10]
}
