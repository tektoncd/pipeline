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

package pipelinerun

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/pod"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/internal/affinityassistant"
	aa "github.com/tektoncd/pipeline/pkg/internal/affinityassistant"
	pipelinePod "github.com/tektoncd/pipeline/pkg/pod"
	"github.com/tektoncd/pipeline/pkg/reconciler/volumeclaim"
	"github.com/tektoncd/pipeline/pkg/workspace"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	errorutils "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"knative.dev/pkg/kmeta"
	"knative.dev/pkg/logging"
)

const (
	// ReasonCouldntCreateOrUpdateAffinityAssistantStatefulSet indicates that a PipelineRun uses workspaces with PersistentVolumeClaim
	// as a volume source and expect an Assistant StatefulSet in AffinityAssistantPerWorkspace behavior, but couldn't create a StatefulSet.
	ReasonCouldntCreateOrUpdateAffinityAssistantStatefulSet = "ReasonCouldntCreateOrUpdateAffinityAssistantStatefulSet"

	// AutoCleanupPVCAnnotation enables optional PVC cleanup in AffinityAssistantPerWorkspace mode.
	// When set to "true", PVCs created from volumeClaimTemplates are deleted on PipelineRun completion.
	// This annotation only affects volumeClaimTemplate workspaces; user-provided persistentVolumeClaim
	// workspaces are never deleted.
	AutoCleanupPVCAnnotation = "tekton.dev/auto-cleanup-pvc"
)

var (
	// Deprecated: use volumeclain.ErrPvcCreationFailed instead
	ErrPvcCreationFailed = volumeclaim.ErrPvcCreationFailed
	// Deprecated: use volumeclaim.ErrAffinityAssistantCreationFailed instead
	ErrPvcCreationFailedRetryable      = volumeclaim.ErrPvcCreationFailedRetryable
	ErrAffinityAssistantCreationFailed = errors.New("Affinity Assistant creation error")
)

// createOrUpdateAffinityAssistantsAndPVCs creates Affinity Assistant StatefulSets and PVCs based on AffinityAssistantBehavior.
// This is done to achieve Node Affinity for taskruns in a pipelinerun, and make it possible for the taskruns to execute parallel while sharing volume.
// If the AffinityAssistantBehavior is AffinityAssistantPerWorkspace, it creates an Affinity Assistant for
// every taskrun in the pipelinerun that use the same PVC based volume.
// If the AffinityAssistantBehavior is AffinityAssistantPerPipelineRun or AffinityAssistantPerPipelineRunWithIsolation,
// it creates one Affinity Assistant for the pipelinerun.
func (c *Reconciler) createOrUpdateAffinityAssistantsAndPVCs(ctx context.Context, pr *v1.PipelineRun, aaBehavior aa.AffinityAssistantBehavior) error {
	var unschedulableNodes sets.Set[string] = nil

	var claimTemplates []corev1.PersistentVolumeClaim
	var claimNames []string
	claimNameToWorkspaceName := map[string]string{}
	claimTemplateToWorkspace := map[*corev1.PersistentVolumeClaim]v1.WorkspaceBinding{}

	for _, w := range pr.Spec.Workspaces {
		if w.PersistentVolumeClaim == nil && w.VolumeClaimTemplate == nil {
			continue
		}
		if w.PersistentVolumeClaim != nil {
			claim := w.PersistentVolumeClaim
			claimNames = append(claimNames, claim.ClaimName)
			claimNameToWorkspaceName[claim.ClaimName] = w.Name
		} else if w.VolumeClaimTemplate != nil {
			claimTemplate := w.VolumeClaimTemplate.DeepCopy()
			claimTemplate.Name = volumeclaim.GeneratePVCNameFromWorkspaceBinding(w.VolumeClaimTemplate.Name, w, *kmeta.NewControllerRef(pr))
			claimTemplates = append(claimTemplates, *claimTemplate)
			claimTemplateToWorkspace[claimTemplate] = w
		}
	}
	switch aaBehavior {
	case aa.AffinityAssistantPerWorkspace:
		for claimName, workspaceName := range claimNameToWorkspaceName {
			aaName := GetAffinityAssistantName(workspaceName, pr.Name)
			if err := c.createOrUpdateAffinityAssistant(ctx, aaName, pr, nil, []string{claimName}, unschedulableNodes); err != nil {
				return fmt.Errorf("%w: %v", ErrAffinityAssistantCreationFailed, err)
			}
		}
		for claimTemplate, workspace := range claimTemplateToWorkspace {
			// To support PVC auto deletion at pipelinerun deletion time, the OwnerReference of the PVCs should be set to the owning pipelinerun instead of the StatefulSets,
			// so we create PVCs from PipelineRuns' VolumeClaimTemplate and pass the PVCs to the Affinity Assistant StatefulSet for volume scheduling.
			if err := c.pvcHandler.CreatePVCFromVolumeClaimTemplate(ctx, workspace, *kmeta.NewControllerRef(pr), pr.Namespace); err != nil {
				return err
			}
			aaName := GetAffinityAssistantName(workspace.Name, pr.Name)
			if err := c.createOrUpdateAffinityAssistant(ctx, aaName, pr, nil, []string{claimTemplate.Name}, unschedulableNodes); err != nil {
				return fmt.Errorf("%w: %v", ErrAffinityAssistantCreationFailed, err)
			}
		}
	case aa.AffinityAssistantPerPipelineRun, aa.AffinityAssistantPerPipelineRunWithIsolation:
		aaName := GetAffinityAssistantName("", pr.Name)
		// The PVCs are created via StatefulSet's VolumeClaimTemplate for volume scheduling
		// in AffinityAssistantPerPipelineRun or AffinityAssistantPerPipelineRunWithIsolation modes.
		// This is because PVCs from pipelinerun's VolumeClaimTemplate are enforced to be deleted at pipelinerun completion time in these modes,
		// and there is no requirement of the PVC OwnerReference.
		if err := c.createOrUpdateAffinityAssistant(ctx, aaName, pr, claimTemplates, claimNames, unschedulableNodes); err != nil {
			return fmt.Errorf("%w: %v", ErrAffinityAssistantCreationFailed, err)
		}
	case aa.AffinityAssistantDisabled:
		for _, workspace := range claimTemplateToWorkspace {
			if err := c.pvcHandler.CreatePVCFromVolumeClaimTemplate(ctx, workspace, *kmeta.NewControllerRef(pr), pr.Namespace); err != nil {
				return err
			}
		}
	}

	return nil
}

// createOrUpdateAffinityAssistant creates an Affinity Assistant Statefulset with the provided affinityAssistantName and pipelinerun information.
// The VolumeClaimTemplates and Volumes of StatefulSet reference the resolved claimTemplates and claims respectively.
// It maintains a set of unschedulableNodes to detect and recreate Affinity Assistant in case of the node is cordoned to avoid pipelinerun deadlock.
func (c *Reconciler) createOrUpdateAffinityAssistant(ctx context.Context, affinityAssistantName string, pr *v1.PipelineRun, claimTemplates []corev1.PersistentVolumeClaim, claimNames []string, unschedulableNodes sets.Set[string]) []error {
	logger := logging.FromContext(ctx)
	cfg := config.FromContextOrDefaults(ctx)

	var errs []error
	a, err := c.KubeClientSet.AppsV1().StatefulSets(pr.Namespace).Get(ctx, affinityAssistantName, metav1.GetOptions{})
	switch {
	// check whether the affinity assistant (StatefulSet) exists or not, create one if it does not exist
	case apierrors.IsNotFound(err):
		aaBehavior, err := aa.GetAffinityAssistantBehavior(ctx)
		if err != nil {
			return []error{err}
		}

		securityContextConfig := pipelinePod.SecurityContextConfig{
			SetSecurityContext:        cfg.FeatureFlags.SetSecurityContext,
			SetReadOnlyRootFilesystem: cfg.FeatureFlags.SetSecurityContextReadOnlyRootFilesystem,
		}

		containerConfig := aa.ContainerConfig{
			Image:                 c.Images.NopImage,
			SecurityContextConfig: securityContextConfig,
		}

		affinityAssistantStatefulSet := affinityAssistantStatefulSet(aaBehavior, affinityAssistantName, pr, claimTemplates, claimNames, containerConfig, cfg.Defaults.DefaultAAPodTemplate)
		_, err = c.KubeClientSet.AppsV1().StatefulSets(pr.Namespace).Create(ctx, affinityAssistantStatefulSet, metav1.CreateOptions{})
		if err != nil {
			errs = append(errs, fmt.Errorf("failed to create StatefulSet %s: %w", affinityAssistantName, err))
		}
		if err == nil {
			logger.Infof("Created StatefulSet %s in namespace %s", affinityAssistantName, pr.Namespace)
		}
	// check whether the affinity assistant (StatefulSet) exists and the affinity assistant pod is created
	// this check requires the StatefulSet to have the readyReplicas set to 1 to allow for any delay between the StatefulSet creation
	// and the necessary pod creation, the delay can be caused by any dependency on PVCs and PVs creation
	// this case addresses issues specified in https://github.com/tektoncd/pipeline/issues/6586
	case err == nil && a != nil && a.Status.ReadyReplicas == 1:
		if unschedulableNodes == nil {
			ns, err := c.KubeClientSet.CoreV1().Nodes().List(ctx, metav1.ListOptions{
				FieldSelector: "spec.unschedulable=true",
			})
			if err != nil {
				errs = append(errs, fmt.Errorf("could not get the list of nodes, err: %w", err))
			}
			unschedulableNodes = sets.Set[string]{}
			// maintain the list of nodes which are unschedulable
			for _, n := range ns.Items {
				unschedulableNodes.Insert(n.Name)
			}
		}
		if unschedulableNodes.Len() > 0 {
			// get the pod created for a given StatefulSet, pod is assigned ordinal of 0 with the replicas set to 1
			p, err := c.KubeClientSet.CoreV1().Pods(pr.Namespace).Get(ctx, a.Name+"-0", metav1.GetOptions{})
			// ignore instead of failing if the affinity assistant pod was not found
			if err != nil && !apierrors.IsNotFound(err) {
				errs = append(errs, fmt.Errorf("could not get the affinity assistant pod for StatefulSet %s: %w", a.Name, err))
			}
			// check the node which hosts the affinity assistant pod if it is unschedulable or cordoned
			if p != nil && unschedulableNodes.Has(p.Spec.NodeName) {
				// if the node is unschedulable, delete the affinity assistant pod such that a StatefulSet can recreate the same pod on a different node
				err = c.KubeClientSet.CoreV1().Pods(p.Namespace).Delete(ctx, p.Name, metav1.DeleteOptions{})
				if err != nil {
					errs = append(errs, fmt.Errorf("error deleting affinity assistant pod %s in ns %s: %w", p.Name, p.Namespace, err))
				}
			}
		}
	case err != nil:
		errs = append(errs, fmt.Errorf("failed to retrieve StatefulSet %s: %w", affinityAssistantName, err))
	}

	return errs
}

// cleanupAffinityAssistantsAndPVCs deletes Affinity Assistant StatefulSets and PVCs created from VolumeClaimTemplates
func (c *Reconciler) cleanupAffinityAssistantsAndPVCs(ctx context.Context, pr *v1.PipelineRun) error {
	aaBehavior, err := aa.GetAffinityAssistantBehavior(ctx)
	if err != nil {
		return err
	}

	var errs []error
	switch aaBehavior {
	case aa.AffinityAssistantPerWorkspace:
		// Check if auto-cleanup annotation is enabled
		autoCleanup := pr.Annotations != nil && pr.Annotations[AutoCleanupPVCAnnotation] == "true"

		for _, w := range pr.Spec.Workspaces {
			if w.PersistentVolumeClaim != nil || w.VolumeClaimTemplate != nil {
				affinityAssistantName := GetAffinityAssistantName(w.Name, pr.Name)
				if err := c.KubeClientSet.AppsV1().StatefulSets(pr.Namespace).Delete(ctx, affinityAssistantName, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
					errs = append(errs, fmt.Errorf("failed to delete StatefulSet %s: %w", affinityAssistantName, err))
				}
			}

			// Delete PVCs from volumeClaimTemplate if auto-cleanup is enabled.
			// User-provided persistentVolumeClaim workspaces are never deleted.
			if w.VolumeClaimTemplate != nil && autoCleanup {
				pvcName := volumeclaim.GeneratePVCNameFromWorkspaceBinding(w.VolumeClaimTemplate.Name, w, *kmeta.NewControllerRef(pr))
				if err := c.pvcHandler.PurgeFinalizerAndDeletePVCForWorkspace(ctx, pvcName, pr.Namespace); err != nil {
					errs = append(errs, err)
				}
			}
		}
	case aa.AffinityAssistantPerPipelineRun, aa.AffinityAssistantPerPipelineRunWithIsolation:
		affinityAssistantName := GetAffinityAssistantName("", pr.Name)
		if err := c.KubeClientSet.AppsV1().StatefulSets(pr.Namespace).Delete(ctx, affinityAssistantName, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
			errs = append(errs, fmt.Errorf("failed to delete StatefulSet %s: %w", affinityAssistantName, err))
		}

		// cleanup PVCs created by Affinity Assistants
		for _, w := range pr.Spec.Workspaces {
			if w.VolumeClaimTemplate != nil {
				pvcName := getPersistentVolumeClaimNameWithAffinityAssistant("", pr.Name, w, *kmeta.NewControllerRef(pr))
				if err := c.pvcHandler.PurgeFinalizerAndDeletePVCForWorkspace(ctx, pvcName, pr.Namespace); err != nil {
					errs = append(errs, err)
				}
			}
		}
	case aa.AffinityAssistantDisabled:
		return nil
	}

	return errorutils.NewAggregate(errs)
}

// getPersistentVolumeClaimNameWithAffinityAssistant returns the PersistentVolumeClaim name that is
// created by the Affinity Assistant StatefulSet VolumeClaimTemplate when Affinity Assistant is enabled.
// The PVCs created by StatefulSet VolumeClaimTemplates follow the format `<pvcName>-<affinityAssistantName>-0`
func getPersistentVolumeClaimNameWithAffinityAssistant(pipelineWorkspaceName, prName string, wb v1.WorkspaceBinding, owner metav1.OwnerReference) string {
	pvcName := volumeclaim.GeneratePVCNameFromWorkspaceBinding(wb.VolumeClaimTemplate.Name, wb, owner)
	affinityAssistantName := GetAffinityAssistantName(pipelineWorkspaceName, prName)
	return fmt.Sprintf("%s-%s-0", pvcName, affinityAssistantName)
}

// getAffinityAssistantAnnotationVal generates and returns the value for `pipeline.tekton.dev/affinity-assistant` annotation
// based on aaBehavior, pipelinePVCWorkspaceName and prName
func getAffinityAssistantAnnotationVal(aaBehavior affinityassistant.AffinityAssistantBehavior, pipelinePVCWorkspaceName string, prName string) string {
	switch aaBehavior {
	case affinityassistant.AffinityAssistantPerWorkspace:
		if pipelinePVCWorkspaceName != "" {
			return GetAffinityAssistantName(pipelinePVCWorkspaceName, prName)
		}
	case affinityassistant.AffinityAssistantPerPipelineRun, affinityassistant.AffinityAssistantPerPipelineRunWithIsolation:
		return GetAffinityAssistantName("", prName)

	case affinityassistant.AffinityAssistantDisabled:
	}

	return ""
}

// GetAffinityAssistantName returns the Affinity Assistant name based on pipelineWorkspaceName and pipelineRunName
func GetAffinityAssistantName(pipelineWorkspaceName string, pipelineRunName string) string {
	hashBytes := sha256.Sum256([]byte(pipelineWorkspaceName + pipelineRunName))
	hashString := hex.EncodeToString(hashBytes[:])
	return fmt.Sprintf("%s-%s", workspace.ComponentNameAffinityAssistant, hashString[:10])
}

func getStatefulSetLabels(pr *v1.PipelineRun, affinityAssistantName string) map[string]string {
	// Propagate labels from PipelineRun to StatefulSet.
	labels := make(map[string]string, len(pr.ObjectMeta.Labels)+1)
	for key, val := range pr.ObjectMeta.Labels {
		labels[key] = val
	}
	labels[pipeline.PipelineRunLabelKey] = pr.Name

	// LabelInstance is used to configure PodAffinity for all TaskRuns belonging to this Affinity Assistant
	// LabelComponent is used to configure PodAntiAffinity to other Affinity Assistants
	labels[workspace.LabelInstance] = affinityAssistantName
	labels[workspace.LabelComponent] = workspace.ComponentNameAffinityAssistant
	return labels
}

// affinityAssistantStatefulSet returns an Affinity Assistant as a StatefulSet based on the AffinityAssistantBehavior
// with the given AffinityAssistantTemplate applied to the StatefulSet PodTemplateSpec.
// The VolumeClaimTemplates and Volume of StatefulSet reference the PipelineRun WorkspaceBinding VolumeClaimTempalte and the PVCs respectively.
// The PVs created by the StatefulSet are scheduled to the same availability zone which avoids PV scheduling conflict.
func affinityAssistantStatefulSet(aaBehavior aa.AffinityAssistantBehavior, name string, pr *v1.PipelineRun, claimTemplates []corev1.PersistentVolumeClaim, claimNames []string, containerConfig aa.ContainerConfig, defaultAATpl *pod.AffinityAssistantTemplate) *appsv1.StatefulSet {
	// We want a singleton pod
	replicas := int32(1)

	tpl := &pod.AffinityAssistantTemplate{}
	// merge pod template from spec and default if any of them are defined
	if pr.Spec.TaskRunTemplate.PodTemplate != nil || defaultAATpl != nil {
		tpl = pod.MergeAAPodTemplateWithDefault(pr.Spec.TaskRunTemplate.PodTemplate.ToAffinityAssistantTemplate(), defaultAATpl)
	}

	var mounts []corev1.VolumeMount
	for _, claimTemplate := range claimTemplates {
		mounts = append(mounts, corev1.VolumeMount{Name: claimTemplate.Name, MountPath: claimTemplate.Name})
	}

	securityContext := &corev1.SecurityContext{}
	if containerConfig.SecurityContextConfig.SetSecurityContext {
		isWindows := tpl.NodeSelector[pipelinePod.OsSelectorLabel] == "windows"
		securityContext = containerConfig.SecurityContextConfig.GetSecurityContext(isWindows)
	}

	var priorityClassName string
	if tpl.PriorityClassName != nil {
		priorityClassName = *tpl.PriorityClassName
	}

	// Determine ServiceAccountName with 3-tier priority:
	// 1. Explicit override from AffinityAssistantTemplate
	// 2. Inherit from PipelineRun's TaskRunTemplate (default behavior for OpenShift SCC compatibility)
	// 3. Empty string (Kubernetes defaults to "default" ServiceAccount)
	serviceAccountName := ""
	if tpl.ServiceAccountName != "" {
		// Explicit override in AffinityAssistantTemplate
		serviceAccountName = tpl.ServiceAccountName
	} else if pr.Spec.TaskRunTemplate.ServiceAccountName != "" {
		// Inherit from PipelineRun's TaskRunTemplate
		serviceAccountName = pr.Spec.TaskRunTemplate.ServiceAccountName
	}

	containers := []corev1.Container{{
		Name:  "affinity-assistant",
		Image: containerConfig.Image,
		Args:  []string{"tekton_run_indefinitely"},

		// Set requests == limits to get QoS class _Guaranteed_.
		// See https://kubernetes.io/docs/tasks/configure-pod-container/quality-service-pod/#create-a-pod-that-gets-assigned-a-qos-class-of-guaranteed
		// Affinity Assistant pod is a placeholder; request minimal resources
		Resources: corev1.ResourceRequirements{
			Limits: corev1.ResourceList{
				"cpu":    resource.MustParse("50m"),
				"memory": resource.MustParse("100Mi"),
			},
			Requests: corev1.ResourceList{
				"cpu":    resource.MustParse("50m"),
				"memory": resource.MustParse("100Mi"),
			},
		},
		VolumeMounts:    mounts,
		SecurityContext: securityContext,
	}}

	var volumes []corev1.Volume
	for i, claimName := range claimNames {
		volumes = append(volumes, corev1.Volume{
			Name: fmt.Sprintf("workspace-%d", i),
			VolumeSource: corev1.VolumeSource{
				// A Pod mounting a PersistentVolumeClaim that has a StorageClass with
				// volumeBindingMode: Immediate
				// the PV is allocated on a Node first, and then the pod need to be
				// scheduled to that node.
				// To support those PVCs, the Affinity Assistant must also mount the
				// same PersistentVolumeClaim - to be sure that the Affinity Assistant
				// pod is scheduled to the same Availability Zone as the PV, when using
				// a regional cluster. This is called VolumeScheduling.
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: claimName},
			},
		})
	}

	return &appsv1.StatefulSet{
		TypeMeta: metav1.TypeMeta{
			Kind:       "StatefulSet",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:            name,
			Labels:          getStatefulSetLabels(pr, name),
			OwnerReferences: []metav1.OwnerReference{*kmeta.NewControllerRef(pr)},
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: getStatefulSetLabels(pr, name),
			},
			// by setting VolumeClaimTemplates from StatefulSet, all the PVs are scheduled to the same Availability Zone as the StatefulSet
			VolumeClaimTemplates: claimTemplates,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: getStatefulSetLabels(pr, name),
				},
				Spec: corev1.PodSpec{
					Containers: containers,

					Tolerations:        tpl.Tolerations,
					NodeSelector:       tpl.NodeSelector,
					ImagePullSecrets:   tpl.ImagePullSecrets,
					SecurityContext:    tpl.SecurityContext,
					PriorityClassName:  priorityClassName,
					ServiceAccountName: serviceAccountName,

					Affinity: getAssistantAffinityMergedWithPodTemplateAffinity(pr, aaBehavior),
					Volumes:  volumes,
				},
			},
		},
	}
}

// getAssistantAffinityMergedWithPodTemplateAffinity return the affinity that merged with PipelineRun PodTemplate affinity.
func getAssistantAffinityMergedWithPodTemplateAffinity(pr *v1.PipelineRun, aaBehavior aa.AffinityAssistantBehavior) *corev1.Affinity {
	affinityAssistantsAffinity := &corev1.Affinity{}
	if pr.Spec.TaskRunTemplate.PodTemplate != nil && pr.Spec.TaskRunTemplate.PodTemplate.Affinity != nil {
		affinityAssistantsAffinity = pr.Spec.TaskRunTemplate.PodTemplate.Affinity
	}
	if affinityAssistantsAffinity.PodAntiAffinity == nil {
		affinityAssistantsAffinity.PodAntiAffinity = &corev1.PodAntiAffinity{}
	}

	repelOtherAffinityAssistantsPodAffinityTerm := corev1.PodAffinityTerm{
		LabelSelector: &metav1.LabelSelector{
			MatchLabels: map[string]string{
				workspace.LabelComponent: workspace.ComponentNameAffinityAssistant,
			},
		},
		TopologyKey: "kubernetes.io/hostname",
	}

	if aaBehavior == aa.AffinityAssistantPerPipelineRunWithIsolation {
		// use RequiredDuringSchedulingIgnoredDuringExecution term to enforce only one pipelinerun can run in a node at a time
		affinityAssistantsAffinity.PodAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution = append(affinityAssistantsAffinity.PodAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution,
			repelOtherAffinityAssistantsPodAffinityTerm)
	} else {
		preferredRepelOtherAffinityAssistantsPodAffinityTerm := corev1.WeightedPodAffinityTerm{
			Weight:          100,
			PodAffinityTerm: repelOtherAffinityAssistantsPodAffinityTerm,
		}
		// use RequiredDuringSchedulingIgnoredDuringExecution term to schedule pipelineruns to different nodes when possible
		affinityAssistantsAffinity.PodAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution = append(affinityAssistantsAffinity.PodAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution,
			preferredRepelOtherAffinityAssistantsPodAffinityTerm)
	}

	return affinityAssistantsAffinity
}
