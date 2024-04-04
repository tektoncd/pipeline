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
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/pod"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/internal/computeresources/tasklevel"
	"github.com/tektoncd/pipeline/pkg/names"
	"github.com/tektoncd/pipeline/pkg/spire"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/strings/slices"
	"knative.dev/pkg/changeset"
	"knative.dev/pkg/kmeta"
)

const (
	// TektonHermeticEnvVar is the env var we set in containers to indicate they should be run hermetically
	TektonHermeticEnvVar = "TEKTON_HERMETIC"

	// ExecutionModeAnnotation is an experimental optional annotation to set the execution mode on a TaskRun
	ExecutionModeAnnotation = "experimental.tekton.dev/execution-mode"

	// ExecutionModeHermetic indicates hermetic execution mode
	ExecutionModeHermetic = "hermetic"

	// deadlineFactor is the factor we multiply the taskrun timeout with to determine the activeDeadlineSeconds of the Pod.
	// It has to be higher than the timeout (to not be killed before)
	deadlineFactor = 1.5

	// SpiffeCsiDriver is the CSI storage plugin needed for injection of SPIFFE workload api.
	SpiffeCsiDriver = "csi.spiffe.io"

	// osSelectorLabel is the label Kubernetes uses for OS-specific workloads (https://kubernetes.io/docs/reference/labels-annotations-taints/#kubernetes-io-os)
	osSelectorLabel = "kubernetes.io/os"

	// TerminationReasonTimeoutExceeded indicates a step execution timed out.
	TerminationReasonTimeoutExceeded = "TimeoutExceeded"

	// TerminationReasonSkipped indicates a step execution was skipped due to previous step failed.
	TerminationReasonSkipped = "Skipped"

	// TerminationReasonContinued indicates a step errored but was ignored since onError was set to continue.
	TerminationReasonContinued = "Continued"

	// TerminationReasonCancelled indicates a step was cancelled.
	TerminationReasonCancelled = "Cancelled"
)

// These are effectively const, but Go doesn't have such an annotation.
var (
	ReleaseAnnotation = "pipeline.tekton.dev/release"

	groupVersionKind = schema.GroupVersionKind{
		Group:   v1.SchemeGroupVersion.Group,
		Version: v1.SchemeGroupVersion.Version,
		Kind:    "TaskRun",
	}
	// These are injected into all of the source/step containers.
	implicitVolumeMounts = []corev1.VolumeMount{{
		Name:      "tekton-internal-workspace",
		MountPath: pipeline.WorkspaceDir,
	}, {
		Name:      "tekton-internal-home",
		MountPath: pipeline.HomeDir,
	}, {
		Name:      "tekton-internal-results",
		MountPath: pipeline.DefaultResultPath,
	}, {
		Name:      "tekton-internal-steps",
		MountPath: pipeline.StepsDir,
		ReadOnly:  true,
	}}
	implicitVolumes = []corev1.Volume{{
		Name:         "tekton-internal-workspace",
		VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
	}, {
		Name:         "tekton-internal-home",
		VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
	}, {
		Name:         "tekton-internal-results",
		VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
	}, {
		Name:         "tekton-internal-steps",
		VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
	}}

	// MaxActiveDeadlineSeconds is a maximum permitted value to be used for a task with no timeout
	MaxActiveDeadlineSeconds = int64(math.MaxInt32)

	// Used in security context of pod init containers
	allowPrivilegeEscalation = false
	runAsNonRoot             = true

	// The following security contexts allow init containers to run in namespaces
	// with "restricted" pod security admission
	// See https://kubernetes.io/docs/concepts/security/pod-security-standards/#restricted
	linuxSecurityContext = &corev1.SecurityContext{
		AllowPrivilegeEscalation: &allowPrivilegeEscalation,
		Capabilities: &corev1.Capabilities{
			Drop: []corev1.Capability{"ALL"},
		},
		RunAsNonRoot: &runAsNonRoot,
		SeccompProfile: &corev1.SeccompProfile{
			Type: corev1.SeccompProfileTypeRuntimeDefault,
		},
	}
	windowsSecurityContext = &corev1.SecurityContext{
		RunAsNonRoot: &runAsNonRoot,
	}
)

// Builder exposes options to configure Pod construction from TaskSpecs/Runs.
type Builder struct {
	Images          pipeline.Images
	KubeClient      kubernetes.Interface
	EntrypointCache EntrypointCache
}

// Transformer is a function that will transform a Pod. This can be used to mutate
// a Pod generated by Tekton after it got generated.
type Transformer func(*corev1.Pod) (*corev1.Pod, error)

// Build creates a Pod using the configuration options set on b and the TaskRun
// and TaskSpec provided in its arguments. An error is returned if there are
// any problems during the conversion.
func (b *Builder) Build(ctx context.Context, taskRun *v1.TaskRun, taskSpec v1.TaskSpec, transformers ...Transformer) (*corev1.Pod, error) {
	var (
		scriptsInit                                       *corev1.Container
		initContainers, stepContainers, sidecarContainers []corev1.Container
		volumes                                           []corev1.Volume
	)
	volumeMounts := []corev1.VolumeMount{binROMount}
	implicitEnvVars := []corev1.EnvVar{}
	featureFlags := config.FromContextOrDefaults(ctx).FeatureFlags
	defaultForbiddenEnv := config.FromContextOrDefaults(ctx).Defaults.DefaultForbiddenEnv
	alphaAPIEnabled := featureFlags.EnableAPIFields == config.AlphaAPIFields
	sidecarLogsResultsEnabled := config.FromContextOrDefaults(ctx).FeatureFlags.ResultExtractionMethod == config.ResultExtractionMethodSidecarLogs
	enableKeepPodOnCancel := featureFlags.EnableKeepPodOnCancel
	setSecurityContext := config.FromContextOrDefaults(ctx).FeatureFlags.SetSecurityContext

	// Add our implicit volumes first, so they can be overridden by the user if they prefer.
	volumes = append(volumes, implicitVolumes...)
	volumeMounts = append(volumeMounts, implicitVolumeMounts...)

	// Create Volumes and VolumeMounts for any credentials found in annotated
	// Secrets, along with any arguments needed by Step entrypoints to process
	// those secrets.
	commonExtraEntrypointArgs := []string{}
	// Entrypoint arg to enable or disable spire
	if config.IsSpireEnabled(ctx) {
		commonExtraEntrypointArgs = append(commonExtraEntrypointArgs, "-enable_spire")
	}
	credEntrypointArgs, credVolumes, credVolumeMounts, err := credsInit(ctx, taskRun.Spec.ServiceAccountName, taskRun.Namespace, b.KubeClient)
	if err != nil {
		return nil, err
	}
	commonExtraEntrypointArgs = append(commonExtraEntrypointArgs, credEntrypointArgs...)
	volumes = append(volumes, credVolumes...)
	volumeMounts = append(volumeMounts, credVolumeMounts...)

	// Merge step template with steps.
	// TODO(#1605): Move MergeSteps to pkg/pod
	steps, err := v1.MergeStepsWithStepTemplate(taskSpec.StepTemplate, taskSpec.Steps)
	if err != nil {
		return nil, err
	}
	steps, err = v1.MergeStepsWithSpecs(steps, taskRun.Spec.StepSpecs)
	if err != nil {
		return nil, err
	}
	if taskRun.Spec.ComputeResources != nil {
		tasklevel.ApplyTaskLevelComputeResources(steps, taskRun.Spec.ComputeResources)
	}
	windows := usesWindows(taskRun)
	if sidecarLogsResultsEnabled && taskSpec.Results != nil {
		// create a results sidecar
		resultsSidecar, err := createResultsSidecar(taskSpec, b.Images.SidecarLogResultsImage, setSecurityContext, windows)
		if err != nil {
			return nil, err
		}
		taskSpec.Sidecars = append(taskSpec.Sidecars, resultsSidecar)
		commonExtraEntrypointArgs = append(commonExtraEntrypointArgs, "-result_from", config.ResultExtractionMethodSidecarLogs)
	}
	sidecars, err := v1.MergeSidecarsWithSpecs(taskSpec.Sidecars, taskRun.Spec.SidecarSpecs)
	if err != nil {
		return nil, err
	}

	initContainers = []corev1.Container{
		entrypointInitContainer(b.Images.EntrypointImage, steps, setSecurityContext, windows),
	}

	// Convert any steps with Script to command+args.
	// If any are found, append an init container to initialize scripts.
	if alphaAPIEnabled {
		scriptsInit, stepContainers, sidecarContainers = convertScripts(b.Images.ShellImage, b.Images.ShellImageWin, steps, sidecars, taskRun.Spec.Debug, setSecurityContext)
	} else {
		scriptsInit, stepContainers, sidecarContainers = convertScripts(b.Images.ShellImage, "", steps, sidecars, nil, setSecurityContext)
	}

	if scriptsInit != nil {
		initContainers = append(initContainers, *scriptsInit)
		volumes = append(volumes, scriptsVolume)
	}
	if alphaAPIEnabled && taskRun.Spec.Debug != nil {
		volumes = append(volumes, debugScriptsVolume, debugInfoVolume)
	}
	// Initialize any workingDirs under /workspace.
	if workingDirInit := workingDirInit(b.Images.WorkingDirInitImage, stepContainers, setSecurityContext, windows); workingDirInit != nil {
		initContainers = append(initContainers, *workingDirInit)
	}

	// By default, use an empty pod template and take the one defined in the task run spec if any
	podTemplate := pod.Template{}

	if taskRun.Spec.PodTemplate != nil {
		podTemplate = *taskRun.Spec.PodTemplate
	}

	// Resolve entrypoint for any steps that don't specify command.
	stepContainers, err = resolveEntrypoints(ctx, b.EntrypointCache, taskRun.Namespace, taskRun.Spec.ServiceAccountName, podTemplate.ImagePullSecrets, stepContainers)
	if err != nil {
		return nil, err
	}

	readyImmediately := isPodReadyImmediately(*featureFlags, taskSpec.Sidecars)

	if alphaAPIEnabled {
		stepContainers, err = orderContainers(ctx, commonExtraEntrypointArgs, stepContainers, &taskSpec, taskRun.Spec.Debug, !readyImmediately, enableKeepPodOnCancel)
	} else {
		stepContainers, err = orderContainers(ctx, commonExtraEntrypointArgs, stepContainers, &taskSpec, nil, !readyImmediately, enableKeepPodOnCancel)
	}
	if err != nil {
		return nil, err
	}
	volumes = append(volumes, binVolume)
	if !readyImmediately || enableKeepPodOnCancel {
		downwardVolumeDup := downwardVolume.DeepCopy()
		if enableKeepPodOnCancel {
			downwardVolumeDup.VolumeSource.DownwardAPI.Items = append(downwardVolumeDup.VolumeSource.DownwardAPI.Items, downwardCancelVolumeItem)
		}
		volumes = append(volumes, *downwardVolumeDup)
	}

	// Order of precedence for envs
	// implicit env vars
	// Superceded by step env vars
	// Superceded by config-default default-pod-template envs
	// Superceded by podTemplate envs
	if len(implicitEnvVars) > 0 {
		for i, s := range stepContainers {
			env := append(implicitEnvVars, s.Env...) //nolint:gocritic
			stepContainers[i].Env = env
		}
	}
	filteredEnvs := []corev1.EnvVar{}
	for _, e := range podTemplate.Env {
		if !slices.Contains(defaultForbiddenEnv, e.Name) {
			filteredEnvs = append(filteredEnvs, e)
		}
	}
	if len(podTemplate.Env) > 0 {
		for i, s := range stepContainers {
			env := append(s.Env, filteredEnvs...) //nolint:gocritic
			stepContainers[i].Env = env
		}
	}
	// Add env var if hermetic execution was requested & if the alpha API is enabled
	if taskRun.Annotations[ExecutionModeAnnotation] == ExecutionModeHermetic && alphaAPIEnabled {
		for i, s := range stepContainers {
			// Add it at the end so it overrides
			env := append(s.Env, corev1.EnvVar{Name: TektonHermeticEnvVar, Value: "1"}) //nolint:gocritic
			stepContainers[i].Env = env
		}
	}

	// Add implicit volume mounts to each step, unless the step specifies
	// its own volume mount at that path.
	for i, s := range stepContainers {
		// Mount /tekton/creds with a fresh volume for each Step. It needs to
		// be world-writeable and empty so creds can be initialized in there. Cant
		// guarantee what UID container runs with. If legacy credential helper (creds-init)
		// is disabled via feature flag then these can be nil since we don't want to mount
		// the automatic credential volume.
		v, vm := getCredsInitVolume(ctx, i)
		if v != nil && vm != nil {
			volumes = append(volumes, *v)
			s.VolumeMounts = append(s.VolumeMounts, *vm)
		}

		// Add /tekton/run state volumes.
		// Each step should only mount their own volume as RW,
		// all other steps should be mounted RO.
		volumes = append(volumes, runVolume(i))
		for j := 0; j < len(stepContainers); j++ {
			s.VolumeMounts = append(s.VolumeMounts, runMount(j, i != j))
		}

		requestedVolumeMounts := map[string]bool{}
		for _, vm := range s.VolumeMounts {
			requestedVolumeMounts[filepath.Clean(vm.MountPath)] = true
		}
		var toAdd []corev1.VolumeMount
		for _, imp := range volumeMounts {
			if !requestedVolumeMounts[filepath.Clean(imp.MountPath)] {
				toAdd = append(toAdd, imp)
			}
		}
		vms := append(s.VolumeMounts, toAdd...) //nolint:gocritic
		stepContainers[i].VolumeMounts = vms
	}

	if sidecarLogsResultsEnabled && taskSpec.Results != nil {
		// Mount implicit volumes onto sidecarContainers
		// so that they can access /tekton/results and /tekton/run.
		for i, s := range sidecarContainers {
			for j := 0; j < len(stepContainers); j++ {
				s.VolumeMounts = append(s.VolumeMounts, runMount(j, true))
			}
			requestedVolumeMounts := map[string]bool{}
			for _, vm := range s.VolumeMounts {
				requestedVolumeMounts[filepath.Clean(vm.MountPath)] = true
			}
			var toAdd []corev1.VolumeMount
			for _, imp := range volumeMounts {
				if !requestedVolumeMounts[filepath.Clean(imp.MountPath)] {
					toAdd = append(toAdd, imp)
				}
			}
			vms := append(s.VolumeMounts, toAdd...) //nolint:gocritic
			sidecarContainers[i].VolumeMounts = vms
		}
	}

	// This loop:
	// - sets container name to add "step-" prefix or "step-unnamed-#" if not specified.
	// TODO(#1605): Remove this loop and make each transformation in
	// isolation.
	for i, s := range stepContainers {
		stepContainers[i].Name = names.SimpleNameGenerator.RestrictLength(StepName(s.Name, i))
	}

	// Add podTemplate Volumes to the explicitly declared use volumes
	volumes = append(volumes, taskSpec.Volumes...)
	volumes = append(volumes, podTemplate.Volumes...)

	if err := v1.ValidateVolumes(volumes); err != nil {
		return nil, err
	}

	readonly := true
	if config.IsSpireEnabled(ctx) {
		// add SPIRE's CSI volume to the explicitly declared use volumes
		volumes = append(volumes, corev1.Volume{
			Name: spire.WorkloadAPI,
			VolumeSource: corev1.VolumeSource{
				CSI: &corev1.CSIVolumeSource{
					Driver:   SpiffeCsiDriver,
					ReadOnly: &readonly,
				},
			},
		})

		// mount SPIRE's CSI volume to each Step Container
		for i := range stepContainers {
			c := &stepContainers[i]
			c.VolumeMounts = append(c.VolumeMounts, corev1.VolumeMount{
				Name:      spire.WorkloadAPI,
				MountPath: spire.VolumeMountPath,
				ReadOnly:  readonly,
			})
		}
		for i := range initContainers {
			// mount SPIRE's CSI volume to each Init Container
			c := &initContainers[i]
			c.VolumeMounts = append(c.VolumeMounts, corev1.VolumeMount{
				Name:      spire.WorkloadAPI,
				MountPath: spire.VolumeMountPath,
				ReadOnly:  readonly,
			})
		}
	}

	mergedPodContainers := stepContainers

	// Merge sidecar containers with step containers.
	for _, sc := range sidecarContainers {
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

	podAnnotations := kmeta.CopyMap(taskRun.Annotations)
	podAnnotations[ReleaseAnnotation] = changeset.Get()

	if readyImmediately {
		podAnnotations[readyAnnotation] = readyAnnotationValue
	}

	// calculate the activeDeadlineSeconds based on the specified timeout (uses default timeout if it's not specified)
	activeDeadlineSeconds := int64(taskRun.GetTimeout(ctx).Seconds() * deadlineFactor)
	// set activeDeadlineSeconds to the max. allowed value i.e. max int32 when timeout is explicitly set to 0
	if taskRun.GetTimeout(ctx) == config.NoTimeoutDuration {
		activeDeadlineSeconds = MaxActiveDeadlineSeconds
	}

	podNameSuffix := "-pod"
	if taskRunRetries := len(taskRun.Status.RetriesStatus); taskRunRetries > 0 {
		podNameSuffix = fmt.Sprintf("%s-retry%d", podNameSuffix, taskRunRetries)
	}
	newPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			// We execute the build's pod in the same namespace as where the build was
			// created so that it can access colocated resources.
			Namespace: taskRun.Namespace,
			// Generate a unique name based on the build's name.
			// The name is univocally generated so that in case of
			// stale informer cache, we never create duplicate Pods
			Name: kmeta.ChildName(taskRun.Name, podNameSuffix),
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
			SchedulerName:                podTemplate.SchedulerName,
			HostNetwork:                  podTemplate.HostNetwork,
			DNSPolicy:                    dnsPolicy,
			DNSConfig:                    podTemplate.DNSConfig,
			EnableServiceLinks:           podTemplate.EnableServiceLinks,
			PriorityClassName:            priorityClassName,
			ImagePullSecrets:             podTemplate.ImagePullSecrets,
			HostAliases:                  podTemplate.HostAliases,
			TopologySpreadConstraints:    podTemplate.TopologySpreadConstraints,
			ActiveDeadlineSeconds:        &activeDeadlineSeconds, // Set ActiveDeadlineSeconds to mark the pod as "terminating" (like a Job)
		},
	}

	for _, f := range transformers {
		newPod, err = f(newPod)
		if err != nil {
			return newPod, err
		}
	}

	// update init container and containers resource requirements
	// resource limits values are taken from a config map
	configDefaults := config.FromContextOrDefaults(ctx).Defaults
	updateResourceRequirements(configDefaults.DefaultContainerResourceRequirements, newPod)

	return newPod, nil
}

// updates init containers and containers resource requirements of a pod base of config_defaults configmap.
func updateResourceRequirements(resourceRequirementsMap map[string]corev1.ResourceRequirements, pod *corev1.Pod) {
	if len(resourceRequirementsMap) == 0 {
		return
	}

	// collect all the available container names from the resource requirement map
	// some of the container names: place-scripts, prepare, working-dir-initializer
	// some of the container names with prefix: prefix-scripts, prefix-sidecar-scripts
	containerNames := []string{}
	containerNamesWithPrefix := []string{}
	for containerName := range resourceRequirementsMap {
		// skip the default key
		if containerName == config.ResourceRequirementDefaultContainerKey {
			continue
		}

		if strings.HasPrefix(containerName, "prefix-") {
			containerNamesWithPrefix = append(containerNamesWithPrefix, containerName)
		} else {
			containerNames = append(containerNames, containerName)
		}
	}

	// update the containers resource requirements which does not have resource requirements
	for _, containerName := range containerNames {
		resourceRequirements := resourceRequirementsMap[containerName]
		if resourceRequirements.Size() == 0 {
			continue
		}

		// update init containers
		for index := range pod.Spec.InitContainers {
			targetContainer := pod.Spec.InitContainers[index]
			if containerName == targetContainer.Name && targetContainer.Resources.Size() == 0 {
				pod.Spec.InitContainers[index].Resources = resourceRequirements
			}
		}
		// update containers
		for index := range pod.Spec.Containers {
			targetContainer := pod.Spec.Containers[index]
			if containerName == targetContainer.Name && targetContainer.Resources.Size() == 0 {
				pod.Spec.Containers[index].Resources = resourceRequirements
			}
		}
	}

	// update the containers resource requirements which does not have resource requirements with the mentioned prefix
	for _, containerPrefix := range containerNamesWithPrefix {
		resourceRequirements := resourceRequirementsMap[containerPrefix]
		if resourceRequirements.Size() == 0 {
			continue
		}

		// get actual container name, remove "prefix-" string and append "-" at the end
		// append '-' in the container prefix
		containerPrefix = strings.Replace(containerPrefix, "prefix-", "", 1)
		containerPrefix += "-"

		// update init containers
		for index := range pod.Spec.InitContainers {
			targetContainer := pod.Spec.InitContainers[index]
			if strings.HasPrefix(targetContainer.Name, containerPrefix) && targetContainer.Resources.Size() == 0 {
				pod.Spec.InitContainers[index].Resources = resourceRequirements
			}
		}
		// update containers
		for index := range pod.Spec.Containers {
			targetContainer := pod.Spec.Containers[index]
			if strings.HasPrefix(targetContainer.Name, containerPrefix) && targetContainer.Resources.Size() == 0 {
				pod.Spec.Containers[index].Resources = resourceRequirements
			}
		}
	}

	// reset of the containers resource requirements which has empty resource requirements
	if resourceRequirements, found := resourceRequirementsMap[config.ResourceRequirementDefaultContainerKey]; found && resourceRequirements.Size() != 0 {
		// update init containers
		for index := range pod.Spec.InitContainers {
			if pod.Spec.InitContainers[index].Resources.Size() == 0 {
				pod.Spec.InitContainers[index].Resources = resourceRequirements
			}
		}
		// update containers
		for index := range pod.Spec.Containers {
			if pod.Spec.Containers[index].Resources.Size() == 0 {
				pod.Spec.Containers[index].Resources = resourceRequirements
			}
		}
	}
}

// makeLabels constructs the labels we will propagate from TaskRuns to Pods.
func makeLabels(s *v1.TaskRun) map[string]string {
	labels := make(map[string]string, len(s.ObjectMeta.Labels)+1)
	// NB: Set this *before* passing through TaskRun labels. If the TaskRun
	// has a managed-by label, it should override this default.

	// Copy through the TaskRun's labels to the underlying Pod's.
	for k, v := range s.ObjectMeta.Labels {
		labels[k] = v
	}

	// NB: Set this *after* passing through TaskRun Labels. If the TaskRun
	// specifies this label, it should be overridden by this value.
	labels[pipeline.TaskRunLabelKey] = s.Name
	return labels
}

// isPodReadyImmediately returns a bool indicating whether the
// controller should consider the Pod "Ready" as soon as it's deployed.
// This will add the `Ready` annotation when creating the Pod,
// and prevent the first step from waiting for the annotation to appear before starting.
func isPodReadyImmediately(featureFlags config.FeatureFlags, sidecars []v1.Sidecar) bool {
	// If the TaskRun has sidecars, we must wait for them
	if len(sidecars) > 0 || featureFlags.RunningInEnvWithInjectedSidecars {
		if featureFlags.AwaitSidecarReadiness {
			return false
		}
		log.Printf("warning: not waiting for sidecars before starting first Step of Task pod")
	}
	return true
}

func runMount(i int, ro bool) corev1.VolumeMount {
	return corev1.VolumeMount{
		Name:      fmt.Sprintf("%s-%d", runVolumeName, i),
		MountPath: filepath.Join(RunDir, strconv.Itoa(i)),
		ReadOnly:  ro,
	}
}

func runVolume(i int) corev1.Volume {
	return corev1.Volume{
		Name:         fmt.Sprintf("%s-%d", runVolumeName, i),
		VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
	}
}

// entrypointInitContainer generates a few init containers based of a set of command (in images), volumes to run, and whether the pod will run on a windows node
// This should effectively merge multiple command and volumes together.
// If setSecurityContext is true, the init container will include a security context
// allowing it to run in namespaces with restriced pod security admission.
func entrypointInitContainer(image string, steps []v1.Step, setSecurityContext, windows bool) corev1.Container {
	// Invoke the entrypoint binary in "cp mode" to copy itself
	// into the correct location for later steps and initialize steps folder
	command := []string{"/ko-app/entrypoint", "init", "/ko-app/entrypoint", entrypointBinary}
	for i, s := range steps {
		command = append(command, StepName(s.Name, i))
	}
	volumeMounts := []corev1.VolumeMount{binMount, internalStepsMount}
	securityContext := linuxSecurityContext
	if windows {
		securityContext = windowsSecurityContext
	}

	// Rewrite steps with entrypoint binary. Append the entrypoint init
	// container to place the entrypoint binary. Also add timeout flags
	// to entrypoint binary.
	prepareInitContainer := corev1.Container{
		Name:  "prepare",
		Image: image,
		// Rewrite default WorkingDir from "/home/nonroot" to "/"
		// as suggested at https://github.com/GoogleContainerTools/distroless/issues/718
		// to avoid permission errors with nonroot users not equal to `65532`
		WorkingDir:   "/",
		Command:      command,
		VolumeMounts: volumeMounts,
	}
	if setSecurityContext {
		prepareInitContainer.SecurityContext = securityContext
	}
	return prepareInitContainer
}

// createResultsSidecar creates a sidecar that will run the sidecarlogresults binary,
// based on the spec of the Task, the image that should run in the results sidecar,
// whether it will run on a windows node, and whether the sidecar should include a security context
// that will allow it to run in namespaces with "restricted" pod security admission.
// It will also provide arguments to the binary that allow it to surface the step results.
func createResultsSidecar(taskSpec v1.TaskSpec, image string, setSecurityContext, windows bool) (v1.Sidecar, error) {
	names := make([]string, 0, len(taskSpec.Results))
	for _, r := range taskSpec.Results {
		names = append(names, r.Name)
	}
	resultsStr := strings.Join(names, ",")
	command := []string{"/ko-app/sidecarlogresults", "-results-dir", pipeline.DefaultResultPath, "-result-names", resultsStr}

	// create a map of container Name to step results
	stepResults := map[string][]string{}
	for i, s := range taskSpec.Steps {
		if len(s.Results) > 0 {
			stepName := StepName(s.Name, i)
			stepResults[stepName] = make([]string, 0, len(s.Results))
			for _, r := range s.Results {
				stepResults[stepName] = append(stepResults[stepName], r.Name)
			}
		}
	}

	stepResultsBytes, err := json.Marshal(stepResults)
	if err != nil {
		return v1.Sidecar{}, err
	}
	if len(stepResultsBytes) > 0 {
		command = append(command, "-step-results", string(stepResultsBytes))
	}
	sidecar := v1.Sidecar{
		Name:    pipeline.ReservedResultsSidecarName,
		Image:   image,
		Command: command,
	}
	securityContext := linuxSecurityContext
	if windows {
		securityContext = windowsSecurityContext
	}
	if setSecurityContext {
		sidecar.SecurityContext = securityContext
	}
	return sidecar, nil
}

// usesWindows returns true if the TaskRun will run on a windows node,
// based on its node selector.
// See https://kubernetes.io/docs/concepts/windows/user-guide/ for more info.
func usesWindows(tr *v1.TaskRun) bool {
	if tr.Spec.PodTemplate == nil || tr.Spec.PodTemplate.NodeSelector == nil {
		return false
	}
	osSelector := tr.Spec.PodTemplate.NodeSelector[osSelectorLabel]
	return osSelector == "windows"
}
