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
	"encoding/hex"
	"encoding/json"
	"fmt"

	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"go.uber.org/zap"
	"gomodules.xyz/jsonpatch/v2"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	clientset "k8s.io/client-go/kubernetes"
)

const (
	// ReasonCouldntCreateWorkspacePVC indicates that a Pipeline expects a workspace from a
	// volumeClaimTemplate but couldn't create a claim.
	ReasonCouldntCreateWorkspacePVC = "CouldntCreateWorkspacePVC"
)

// PvcHandler is used to create PVCs for workspaces
type PvcHandler interface {
	CreatePVCFromVolumeClaimTemplate(ctx context.Context, wb v1.WorkspaceBinding, ownerReference metav1.OwnerReference, namespace string) error
	PurgeFinalizerAndDeletePVCForWorkspace(ctx context.Context, pvcName, namespace string) error
}

type defaultPVCHandler struct {
	clientset clientset.Interface
	logger    *zap.SugaredLogger
}

// NewPVCHandler returns a new defaultPVCHandler
func NewPVCHandler(clientset clientset.Interface, logger *zap.SugaredLogger) PvcHandler {
	return &defaultPVCHandler{clientset, logger}
}

// CreatePVCFromVolumeClaimTemplate checks if a PVC named <claim-name>-<workspace-name>-<owner-name> exists;
// where claim-name is provided by the user in the volumeClaimTemplate, and owner-name is the name of the
// resource with the volumeClaimTemplate declared, a PipelineRun or TaskRun. If the PVC did not exist, a new PVC
// with that name is created with the provided OwnerReference.
func (c *defaultPVCHandler) CreatePVCFromVolumeClaimTemplate(ctx context.Context, wb v1.WorkspaceBinding, ownerReference metav1.OwnerReference, namespace string) error {
	claim := c.getPVCFromVolumeClaimTemplate(wb, ownerReference, namespace)
	if claim == nil {
		return nil
	}

	_, err := c.clientset.CoreV1().PersistentVolumeClaims(claim.Namespace).Get(ctx, claim.Name, metav1.GetOptions{})
	switch {
	case apierrors.IsNotFound(err):
		_, err := c.clientset.CoreV1().PersistentVolumeClaims(claim.Namespace).Create(ctx, claim, metav1.CreateOptions{})
		if err != nil {
			if apierrors.IsAlreadyExists(err) {
				c.logger.Infof("Tried to create PersistentVolumeClaim %s in namespace %s, but it already exists",
					claim.Name, claim.Namespace)
			} else {
				return fmt.Errorf("failed to create PVC %s: %w", claim.Name, err)
			}
		} else {
			c.logger.Infof("Created PersistentVolumeClaim %s in namespace %s", claim.Name, claim.Namespace)
		}
	case err != nil:
		return fmt.Errorf("failed to retrieve PVC %s: %w", claim.Name, err)
	}

	return nil
}

// PurgeFinalizerAndDeletePVCForWorkspace deletes pvcs and then purges the `kubernetes.io/pvc-protection` finalizer protection.
// Purging the `kubernetes.io/pvc-protection` finalizer allows the pvc to be deleted even when it is referenced by a taskrun pod.
// See mode details in https://kubernetes.io/docs/concepts/storage/persistent-volumes/#storage-object-in-use-protection.
func (c *defaultPVCHandler) PurgeFinalizerAndDeletePVCForWorkspace(ctx context.Context, pvcName, namespace string) error {
	p, err := c.clientset.CoreV1().PersistentVolumeClaims(namespace).Get(ctx, pvcName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get the PVC %s: %w", pvcName, err)
	}

	// get the list of existing finalizers and drop `pvc-protection` if exists
	var finalizers []string
	for _, f := range p.ObjectMeta.Finalizers {
		if f == "kubernetes.io/pvc-protection" {
			continue
		}
		finalizers = append(finalizers, f)
	}

	// prepare data to remove pvc-protection from the list of finalizers
	removeFinalizerBytes, err := json.Marshal([]jsonpatch.JsonPatchOperation{{
		Path:      "/metadata/finalizers",
		Operation: "replace",
		Value:     finalizers,
	}})
	if err != nil {
		return fmt.Errorf("failed to marshal jsonpatch: %w", err)
	}

	// delete PVC
	err = c.clientset.CoreV1().PersistentVolumeClaims(namespace).Delete(ctx, pvcName, metav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("failed to delete the PVC %s: %w", pvcName, err)
	}

	// remove finalizer
	_, err = c.clientset.CoreV1().PersistentVolumeClaims(namespace).Patch(ctx, pvcName, types.JSONPatchType, removeFinalizerBytes, metav1.PatchOptions{})
	if err != nil {
		return fmt.Errorf("failed to patch the PVC %s: %w", pvcName, err)
	}

	return nil
}

// getPVCFromVolumeClaimTemplate returns a PersistentVolumeClaim based on given workspaceBinding (using VolumeClaimTemplate), ownerReference and namespace
func (c *defaultPVCHandler) getPVCFromVolumeClaimTemplate(workspaceBinding v1.WorkspaceBinding, ownerReference metav1.OwnerReference, namespace string) *corev1.PersistentVolumeClaim {
	if workspaceBinding.VolumeClaimTemplate == nil {
		c.logger.Infof("workspace binding %v does not contain VolumeClaimTemplate, skipping creating PVC", workspaceBinding.Name)
		return nil
	}

	claim := workspaceBinding.VolumeClaimTemplate.DeepCopy()
	claim.Name = GeneratePVCNameFromWorkspaceBinding(workspaceBinding.VolumeClaimTemplate.Name, workspaceBinding, ownerReference)
	claim.Namespace = namespace
	claim.OwnerReferences = []metav1.OwnerReference{ownerReference}

	return claim
}

// GeneratePVCNameFromWorkspaceBinding gets the name of PersistentVolumeClaim for a Workspace and PipelineRun or TaskRun. claim
// must be a PersistentVolumeClaim from a volumeClaimTemplate. The returned name must be consistent given the same
// workspaceBinding name and ownerReference UID - because it is first used for creating a PVC and later,
// possibly several TaskRuns to lookup the PVC to mount.
// We use ownerReference UID over ownerReference name to distinguish runs with the same name.
// If the given volumeClaimTemplate name is empty, the prefix "pvc" will be applied to the PersistentVolumeClaim name.
// See function `getPersistentVolumeClaimNameWithAffinityAssistant` when the PersistentVolumeClaim is created by Affinity Assistant StatefulSet.
func GeneratePVCNameFromWorkspaceBinding(claimName string, wb v1.WorkspaceBinding, owner metav1.OwnerReference) string {
	if claimName == "" {
		return fmt.Sprintf("%s-%s", "pvc", getPersistentVolumeClaimIdentity(wb.Name, string(owner.UID)))
	}
	return fmt.Sprintf("%s-%s", claimName, getPersistentVolumeClaimIdentity(wb.Name, string(owner.UID)))
}

func getPersistentVolumeClaimIdentity(workspaceName, ownerName string) string {
	hashBytes := sha256.Sum256([]byte(workspaceName + ownerName))
	hashString := hex.EncodeToString(hashBytes[:])
	return hashString[:10]
}
