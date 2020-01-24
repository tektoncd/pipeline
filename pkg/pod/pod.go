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

package pod

import (
	"fmt"
	"path/filepath"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/names"
	"github.com/tektoncd/pipeline/pkg/version"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
)

const (
	homeDir = "/tekton/home"

	taskRunLabelKey = pipeline.GroupName + pipeline.TaskRunLabelKey
)

// These are effectively const, but Go doesn't have such an annotation.
var (
	ReleaseAnnotation      = "tekton.dev/release"
	ReleaseAnnotationValue = version.PipelineVersion

	groupVersionKind = schema.GroupVersionKind{
		Group:   v1alpha1.SchemeGroupVersion.Group,
		Version: v1alpha1.SchemeGroupVersion.Version,
		Kind:    "TaskRun",
	}
	// These are injected into all of the source/step containers.
	implicitEnvVars = []corev1.EnvVar{{
		Name:  "HOME",
		Value: homeDir,
	}}
	implicitVolumeMounts = []corev1.VolumeMount{{
		Name:      "tekton-internal-workspace",
		MountPath: pipeline.WorkspaceDir,
	}, {
		Name:      "tekton-internal-home",
		MountPath: homeDir,
	}}
	implicitVolumes = []corev1.Volume{{
		Name:         "tekton-internal-workspace",
		VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
	}, {
		Name:         "tekton-internal-home",
		VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
	}}
)

// MakePod converts TaskRun and TaskSpec objects to a Pod which implements the taskrun specified
// by the supplied CRD.
func MakePod(images pipeline.Images, taskRun *v1alpha1.TaskRun, taskSpec v1alpha1.TaskSpec, kubeclient kubernetes.Interface, entrypointCache EntrypointCache) (*corev1.Pod, error) {
	var initContainers []corev1.Container
	var volumes []corev1.Volume

	// Add our implicit volumes first, so they can be overridden by the user if they prefer.
	volumes = append(volumes, implicitVolumes...)

	// Inititalize any credentials found in annotated Secrets.
	if credsInitContainer, secretsVolumes, err := credsInit(images.CredsImage, taskRun.Spec.ServiceAccountName, taskRun.Namespace, kubeclient, implicitVolumeMounts, implicitEnvVars); err != nil {
		return nil, err
	} else if credsInitContainer != nil {
		initContainers = append(initContainers, *credsInitContainer)
		volumes = append(volumes, secretsVolumes...)
	}

	// Merge step template with steps.
	// TODO(#1605): Move MergeSteps to pkg/pod
	steps, err := v1alpha1.MergeStepsWithStepTemplate(taskSpec.StepTemplate, taskSpec.Steps)
	if err != nil {
		return nil, err
	}

	// Convert any steps with Script to command+args.
	// If any are found, append an init container to initialize scripts.
	scriptsInit, stepContainers := convertScripts(images.ShellImage, steps)
	if scriptsInit != nil {
		initContainers = append(initContainers, *scriptsInit)
		volumes = append(volumes, scriptsVolume)
	}

	// Initialize any workingDirs under /workspace.
	if workingDirInit := workingDirInit(images.ShellImage, stepContainers); workingDirInit != nil {
		initContainers = append(initContainers, *workingDirInit)
	}

	// Resolve entrypoint for any steps that don't specify command.
	stepContainers, err = resolveEntrypoints(entrypointCache, taskRun.Namespace, taskRun.Spec.ServiceAccountName, stepContainers)
	if err != nil {
		return nil, err
	}

	// Rewrite steps with entrypoint binary. Append the entrypoint init
	// container to place the entrypoint binary.
	entrypointInit, stepContainers, err := orderContainers(images.EntrypointImage, stepContainers, taskSpec.Results)
	if err != nil {
		return nil, err
	}
	initContainers = append(initContainers, entrypointInit)
	volumes = append(volumes, toolsVolume, downwardVolume)

	// Zero out non-max resource requests.
	stepContainers = resolveResourceRequests(stepContainers)

	// Add implicit env vars.
	// They're prepended to the list, so that if the user specified any
	// themselves their value takes precedence.
	for i, s := range stepContainers {
		env := append(implicitEnvVars, s.Env...)
		stepContainers[i].Env = env
	}

	// Add implicit volume mounts to each step, unless the step specifies
	// its own volume mount at that path.
	for i, s := range stepContainers {
		requestedVolumeMounts := map[string]bool{}
		for _, vm := range s.VolumeMounts {
			requestedVolumeMounts[filepath.Clean(vm.MountPath)] = true
		}
		var toAdd []corev1.VolumeMount
		for _, imp := range implicitVolumeMounts {
			if !requestedVolumeMounts[filepath.Clean(imp.MountPath)] {
				toAdd = append(toAdd, imp)
			}
		}
		vms := append(s.VolumeMounts, toAdd...)
		stepContainers[i].VolumeMounts = vms
	}

	// This loop:
	// - defaults workingDir to /workspace
	// - sets container name to add "step-" prefix or "step-unnamed-#" if not specified.
	// TODO(#1605): Remove this loop and make each transformation in
	// isolation.
	for i, s := range stepContainers {
		if s.WorkingDir == "" {
			stepContainers[i].WorkingDir = pipeline.WorkspaceDir
		}
		if s.Name == "" {
			stepContainers[i].Name = names.SimpleNameGenerator.RestrictLength(fmt.Sprintf("%sunnamed-%d", stepPrefix, i))
		} else {
			stepContainers[i].Name = names.SimpleNameGenerator.RestrictLength(fmt.Sprintf("%s%s", stepPrefix, s.Name))
		}
	}

	// By default, use an empty pod template and take the one defined in the task run spec if any
	podTemplate := v1alpha1.PodTemplate{}

	if taskRun.Spec.PodTemplate != nil {
		podTemplate = *taskRun.Spec.PodTemplate
	}

	// Add podTemplate Volumes to the explicitly declared use volumes
	volumes = append(volumes, taskSpec.Volumes...)
	volumes = append(volumes, podTemplate.Volumes...)

	if err := v1alpha1.ValidateVolumes(volumes); err != nil {
		return nil, err
	}

	// Merge sidecar containers with step containers.
	mergedPodContainers := stepContainers
	for _, sc := range taskSpec.Sidecars {
		sc.Name = names.SimpleNameGenerator.RestrictLength(fmt.Sprintf("%v%v", sidecarPrefix, sc.Name))
		mergedPodContainers = append(mergedPodContainers, sc)
	}

	var dnsPolicy corev1.DNSPolicy
	if podTemplate.DNSPolicy != nil {
		dnsPolicy = *podTemplate.DNSPolicy
	}

	var priorityClassName string
	if podTemplate.PriorityClassName != nil {
		priorityClassName = *podTemplate.PriorityClassName
	}

	podAnnotations := taskRun.Annotations
	podAnnotations[ReleaseAnnotation] = ReleaseAnnotationValue

	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			// We execute the build's pod in the same namespace as where the build was
			// created so that it can access colocated resources.
			Namespace: taskRun.Namespace,
			// Generate a unique name based on the build's name.
			// Add a unique suffix to avoid confusion when a build
			// is deleted and re-created with the same name.
			// We don't use RestrictLengthWithRandomSuffix here because k8s fakes don't support it.
			Name: names.SimpleNameGenerator.RestrictLengthWithRandomSuffix(fmt.Sprintf("%s-pod", taskRun.Name)),
			// If our parent TaskRun is deleted, then we should be as well.
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(taskRun, groupVersionKind),
			},
			Annotations: podAnnotations,
			Labels:      makeLabels(taskRun),
		},
		Spec: corev1.PodSpec{
			RestartPolicy:                corev1.RestartPolicyNever,
			InitContainers:               initContainers,
			Containers:                   mergedPodContainers,
			ServiceAccountName:           taskRun.Spec.ServiceAccountName,
			Volumes:                      volumes,
			NodeSelector:                 podTemplate.NodeSelector,
			Tolerations:                  podTemplate.Tolerations,
			Affinity:                     podTemplate.Affinity,
			SecurityContext:              podTemplate.SecurityContext,
			RuntimeClassName:             podTemplate.RuntimeClassName,
			AutomountServiceAccountToken: podTemplate.AutomountServiceAccountToken,
			DNSPolicy:                    dnsPolicy,
			DNSConfig:                    podTemplate.DNSConfig,
			EnableServiceLinks:           podTemplate.EnableServiceLinks,
			PriorityClassName:            priorityClassName,
		},
	}, nil
}

// makeLabels constructs the labels we will propagate from TaskRuns to Pods.
func makeLabels(s *v1alpha1.TaskRun) map[string]string {
	labels := make(map[string]string, len(s.ObjectMeta.Labels)+1)
	// NB: Set this *before* passing through TaskRun labels. If the TaskRun
	// has a managed-by label, it should override this default.

	// Copy through the TaskRun's labels to the underlying Pod's.
	for k, v := range s.ObjectMeta.Labels {
		labels[k] = v
	}

	// NB: Set this *after* passing through TaskRun Labels. If the TaskRun
	// specifies this label, it should be overridden by this value.
	labels[taskRunLabelKey] = s.Name
	return labels
}
