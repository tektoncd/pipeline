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
	"fmt"

	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/pod"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
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
	// as a volume source and expect an Assistant StatefulSet, but couldn't create a StatefulSet.
	ReasonCouldntCreateOrUpdateAffinityAssistantStatefulSet = "CouldntCreateOrUpdateAffinityAssistantstatefulSet"

	featureFlagDisableAffinityAssistantKey = "disable-affinity-assistant"
)

// createOrUpdateAffinityAssistants creates an Affinity Assistant StatefulSet for every workspace in the PipelineRun that
// use a PersistentVolumeClaim volume. This is done to achieve Node Affinity for all TaskRuns that
// share the workspace volume and make it possible for the tasks to execute parallel while sharing volume.
func (c *Reconciler) createOrUpdateAffinityAssistants(ctx context.Context, wb []v1.WorkspaceBinding, pr *v1.PipelineRun, namespace string) error {
	logger := logging.FromContext(ctx)
	cfg := config.FromContextOrDefaults(ctx)

	var errs []error
	var unschedulableNodes sets.Set[string] = nil
	for _, w := range wb {
		if w.PersistentVolumeClaim == nil && w.VolumeClaimTemplate == nil {
			continue
		}

		var claimTemplates []corev1.PersistentVolumeClaim
		var claims []corev1.PersistentVolumeClaimVolumeSource
		if w.PersistentVolumeClaim != nil {
			claims = append(claims, *w.PersistentVolumeClaim.DeepCopy())
		} else if w.VolumeClaimTemplate != nil {
			claimTemplate := w.VolumeClaimTemplate.DeepCopy()
			// PVCs Will be created by Affinity Assistant StatefulSet and will follow the naming format `<claimTemplate.Name>-<AffinityAssistantName>-0`
			claimTemplate.Name = volumeclaim.GetPVCNameWithoutAffinityAssistant(w.VolumeClaimTemplate.Name, w, *kmeta.NewControllerRef(pr))
			claimTemplates = append(claimTemplates, *claimTemplate)
		}

		affinityAssistantName := getAffinityAssistantName(w.Name, pr.Name)
		a, err := c.KubeClientSet.AppsV1().StatefulSets(namespace).Get(ctx, affinityAssistantName, metav1.GetOptions{})
		switch {
		// check whether the affinity assistant (StatefulSet) exists or not, create one if it does not exist
		case apierrors.IsNotFound(err):
			affinityAssistantStatefulSet := affinityAssistantStatefulSet(affinityAssistantName, pr, claimTemplates, claims, c.Images.NopImage, cfg.Defaults.DefaultAAPodTemplate)
			_, err := c.KubeClientSet.AppsV1().StatefulSets(namespace).Create(ctx, affinityAssistantStatefulSet, metav1.CreateOptions{})
			if err != nil {
				errs = append(errs, fmt.Errorf("failed to create StatefulSet %s: %w", affinityAssistantName, err))
			}
			if err == nil {
				logger.Infof("Created StatefulSet %s in namespace %s", affinityAssistantName, namespace)
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
	}
	return errorutils.NewAggregate(errs)
}

func (c *Reconciler) cleanupAffinityAssistants(ctx context.Context, pr *v1.PipelineRun) error {
	// omit cleanup if the feature is disabled
	if c.isAffinityAssistantDisabled(ctx) {
		return nil
	}

	var errs []error
	for _, w := range pr.Spec.Workspaces {
		if w.PersistentVolumeClaim != nil || w.VolumeClaimTemplate != nil {
			affinityAssistantStsName := getAffinityAssistantName(w.Name, pr.Name)
			if err := c.KubeClientSet.AppsV1().StatefulSets(pr.Namespace).Delete(ctx, affinityAssistantStsName, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
				errs = append(errs, fmt.Errorf("failed to delete StatefulSet %s: %w", affinityAssistantStsName, err))
			}

			// cleanup PVCs created by Affinity Assistants
			if w.VolumeClaimTemplate != nil {
				pvcName := getPersistentVolumeClaimNameWithAffinityAssistant(w.Name, pr.Name, w, *kmeta.NewControllerRef(pr))
				if err := c.KubeClientSet.CoreV1().PersistentVolumeClaims(pr.Namespace).Delete(ctx, pvcName, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
					errs = append(errs, fmt.Errorf("failed to delete PersistentVolumeClaim %s: %w", pvcName, err))
				}
			}
		}
	}
	return errorutils.NewAggregate(errs)
}

// getPersistentVolumeClaimNameWithAffinityAssistant returns the PersistentVolumeClaim name that is
// created by the Affinity Assistant StatefulSet VolumeClaimTemplate when Affinity Assistant is enabled.
// The PVCs created by StatefulSet VolumeClaimTemplates follow the format `<pvcName>-<affinityAssistantName>-0`
func getPersistentVolumeClaimNameWithAffinityAssistant(pipelineWorkspaceName, prName string, wb v1.WorkspaceBinding, owner metav1.OwnerReference) string {
	pvcName := volumeclaim.GetPVCNameWithoutAffinityAssistant(wb.VolumeClaimTemplate.Name, wb, owner)
	affinityAssistantName := getAffinityAssistantName(pipelineWorkspaceName, prName)
	return fmt.Sprintf("%s-%s-0", pvcName, affinityAssistantName)
}

func getAffinityAssistantName(pipelineWorkspaceName string, pipelineRunName string) string {
	hashBytes := sha256.Sum256([]byte(pipelineWorkspaceName + pipelineRunName))
	hashString := fmt.Sprintf("%x", hashBytes)
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

// affinityAssistantStatefulSet returns an Affinity Assistant as a StatefulSet with the given AffinityAssistantTemplate applied to the StatefulSet PodTemplateSpec.
// The VolumeClaimTemplates and Volume of StatefulSet reference the PipelineRun WorkspaceBinding VolumeClaimTempalte and the PVCs respectively.
// The PVs created by the StatefulSet are scheduled to the same availability zone which avoids PV scheduling conflict.
func affinityAssistantStatefulSet(name string, pr *v1.PipelineRun, claimTemplates []corev1.PersistentVolumeClaim, claims []corev1.PersistentVolumeClaimVolumeSource, affinityAssistantImage string, defaultAATpl *pod.AffinityAssistantTemplate) *appsv1.StatefulSet {
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

	containers := []corev1.Container{{
		Name:  "affinity-assistant",
		Image: affinityAssistantImage,
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
		VolumeMounts: mounts,
	}}

	var volumes []corev1.Volume
	for i, claim := range claims {
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
				PersistentVolumeClaim: claim.DeepCopy(),
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

					Tolerations:      tpl.Tolerations,
					NodeSelector:     tpl.NodeSelector,
					ImagePullSecrets: tpl.ImagePullSecrets,

					Affinity: getAssistantAffinityMergedWithPodTemplateAffinity(pr),
					Volumes:  volumes,
				},
			},
		},
	}
}

// isAffinityAssistantDisabled returns a bool indicating whether an Affinity Assistant should
// be created for each PipelineRun that use workspaces with PersistentVolumeClaims
// as volume source. The default behaviour is to enable the Affinity Assistant to
// provide Node Affinity for TaskRuns that share a PVC workspace.
func (c *Reconciler) isAffinityAssistantDisabled(ctx context.Context) bool {
	cfg := config.FromContextOrDefaults(ctx)
	return cfg.FeatureFlags.DisableAffinityAssistant
}

// getAssistantAffinityMergedWithPodTemplateAffinity return the affinity that merged with PipelineRun PodTemplate affinity.
func getAssistantAffinityMergedWithPodTemplateAffinity(pr *v1.PipelineRun) *corev1.Affinity {
	// use podAntiAffinity to repel other affinity assistants
	repelOtherAffinityAssistantsPodAffinityTerm := corev1.WeightedPodAffinityTerm{
		Weight: 100,
		PodAffinityTerm: corev1.PodAffinityTerm{
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					workspace.LabelComponent: workspace.ComponentNameAffinityAssistant,
				},
			},
			TopologyKey: "kubernetes.io/hostname",
		},
	}

	affinityAssistantsAffinity := &corev1.Affinity{}
	if pr.Spec.TaskRunTemplate.PodTemplate != nil && pr.Spec.TaskRunTemplate.PodTemplate.Affinity != nil {
		affinityAssistantsAffinity = pr.Spec.TaskRunTemplate.PodTemplate.Affinity
	}
	if affinityAssistantsAffinity.PodAntiAffinity == nil {
		affinityAssistantsAffinity.PodAntiAffinity = &corev1.PodAntiAffinity{}
	}
	affinityAssistantsAffinity.PodAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution =
		append(affinityAssistantsAffinity.PodAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution,
			repelOtherAffinityAssistantsPodAffinityTerm)

	return affinityAssistantsAffinity
}
