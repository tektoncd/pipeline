/*
Copyright 2018 The Knative Authors

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

// Package resources provides methods to convert a Build CRD to a k8s Pod
// resource.
package resources

import (
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"path/filepath"
	"sort"
	"strings"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/credentials"
	"github.com/tektoncd/pipeline/pkg/credentials/dockercreds"
	"github.com/tektoncd/pipeline/pkg/credentials/gitcreds"
	"github.com/tektoncd/pipeline/pkg/merge"
	"github.com/tektoncd/pipeline/pkg/names"
	"github.com/tektoncd/pipeline/pkg/reconciler/v1alpha1/taskrun/entrypoint"
)

const workspaceDir = "/workspace"

// These are effectively const, but Go doesn't have such an annotation.
var (
	groupVersionKind = schema.GroupVersionKind{
		Group:   v1alpha1.SchemeGroupVersion.Group,
		Version: v1alpha1.SchemeGroupVersion.Version,
		Kind:    "TaskRun",
	}
	emptyVolumeSource = corev1.VolumeSource{
		EmptyDir: &corev1.EmptyDirVolumeSource{},
	}
	// These are injected into all of the source/step containers.
	implicitEnvVars = []corev1.EnvVar{{
		Name:  "HOME",
		Value: "/builder/home",
	}}
	implicitVolumeMounts = []corev1.VolumeMount{{
		Name:      "workspace",
		MountPath: workspaceDir,
	}, {
		Name:      "home",
		MountPath: "/builder/home",
	}}
	implicitVolumes = []corev1.Volume{{
		Name:         "workspace",
		VolumeSource: emptyVolumeSource,
	}, {
		Name:         "home",
		VolumeSource: emptyVolumeSource,
	}}

	zeroQty = resource.MustParse("0")

	// Random byte reader used for pod name generation.
	// var for testing.
	randReader = rand.Reader
)

const (
	// Prefixes to add to the name of the init containers.
	// IMPORTANT: Changing these values without changing fluentd collection configuration
	// will break log collection for init containers.
	containerPrefix            = "build-step-"
	unnamedInitContainerPrefix = "build-step-unnamed-"
	// Name of the credential initialization container.
	credsInit = "credential-initializer"
	// Name of the working dir initialization container.
	workingDirInit = "working-dir-initializer"
)

var (
	// The container used to initialize credentials before the build runs.
	credsImage = flag.String("creds-image", "override-with-creds:latest",
		"The container image for preparing our Build's credentials.")
	// The container that just prints build successful.
	nopImage = flag.String("nop-image", "override-with-nop:latest",
		"The container image run at the end of the build to log build success")
)

func makeCredentialInitializer(serviceAccountName, namespace string, kubeclient kubernetes.Interface) (*corev1.Container, []corev1.Volume, error) {
	if serviceAccountName == "" {
		serviceAccountName = "default"
	}

	sa, err := kubeclient.CoreV1().ServiceAccounts(namespace).Get(serviceAccountName, metav1.GetOptions{})
	if err != nil {
		return nil, nil, err
	}

	builders := []credentials.Builder{dockercreds.NewBuilder(), gitcreds.NewBuilder()}

	// Collect the volume declarations, there mounts into the cred-init container, and the arguments to it.
	volumes := []corev1.Volume{}
	volumeMounts := implicitVolumeMounts
	args := []string{}
	for _, secretEntry := range sa.Secrets {
		secret, err := kubeclient.CoreV1().Secrets(namespace).Get(secretEntry.Name, metav1.GetOptions{})
		if err != nil {
			return nil, nil, err
		}

		matched := false
		for _, b := range builders {
			if sa := b.MatchingAnnotations(secret); len(sa) > 0 {
				matched = true
				args = append(args, sa...)
			}
		}

		if matched {
			name := names.SimpleNameGenerator.RestrictLengthWithRandomSuffix(fmt.Sprintf("secret-volume-%s", secret.Name))
			volumeMounts = append(volumeMounts, corev1.VolumeMount{
				Name:      name,
				MountPath: credentials.VolumeName(secret.Name),
			})
			volumes = append(volumes, corev1.Volume{
				Name: name,
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: secret.Name,
					},
				},
			})
		}
	}

	return &corev1.Container{
		Name:         names.SimpleNameGenerator.RestrictLengthWithRandomSuffix(containerPrefix + credsInit),
		Image:        *credsImage,
		Command:      []string{"/ko-app/creds-init"},
		Args:         args,
		VolumeMounts: volumeMounts,
		Env:          implicitEnvVars,
		WorkingDir:   workspaceDir,
	}, volumes, nil
}

func makeWorkingDirScript(workingDirs map[string]bool) string {
	script := ""
	var orderedDirs []string

	for wd := range workingDirs {
		if wd != "" {
			orderedDirs = append(orderedDirs, wd)
		}
	}
	sort.Strings(orderedDirs)

	for _, wd := range orderedDirs {
		p := filepath.Clean(wd)
		if rel, err := filepath.Rel(workspaceDir, p); err == nil && !strings.HasPrefix(rel, ".") {
			if script == "" {
				script = fmt.Sprintf("mkdir -p %s", p)
			} else {
				script = fmt.Sprintf("%s %s", script, p)
			}
		}
	}

	return script
}

func makeWorkingDirInitializer(steps []corev1.Container) *corev1.Container {
	workingDirs := make(map[string]bool)
	for _, step := range steps {
		workingDirs[step.WorkingDir] = true
	}

	if script := makeWorkingDirScript(workingDirs); script != "" {
		return &corev1.Container{
			Name:         names.SimpleNameGenerator.RestrictLengthWithRandomSuffix(containerPrefix + workingDirInit),
			Image:        *v1alpha1.BashNoopImage,
			Command:      []string{"/ko-app/bash"},
			Args:         []string{"-args", script},
			VolumeMounts: implicitVolumeMounts,
			Env:          implicitEnvVars,
			WorkingDir:   workspaceDir,
		}
	}

	return nil
}

// GetPod returns the Pod for the given pod name
type GetPod func(string, metav1.GetOptions) (*corev1.Pod, error)

// TryGetPod fetches the TaskRun's pod, returning nil if it has not been created or it does not exist.
func TryGetPod(taskRunStatus v1alpha1.TaskRunStatus, gp GetPod) (*corev1.Pod, error) {
	if taskRunStatus.PodName == "" {
		return nil, nil
	}

	pod, err := gp(taskRunStatus.PodName, metav1.GetOptions{})
	if err == nil || errors.IsNotFound(err) {
		return pod, nil
	}

	return nil, err
}

// MakePod converts TaskRun and TaskSpec objects to a Pod which implements the taskrun specified
// by the supplied CRD.
func MakePod(taskRun *v1alpha1.TaskRun, taskSpec v1alpha1.TaskSpec, kubeclient kubernetes.Interface, cache *entrypoint.Cache, logger *zap.SugaredLogger) (*corev1.Pod, error) {
	cred, secrets, err := makeCredentialInitializer(taskRun.Spec.ServiceAccount, taskRun.Namespace, kubeclient)
	if err != nil {
		return nil, err
	}
	initContainers := []corev1.Container{*cred}
	podContainers := []corev1.Container{}

	if workingDir := makeWorkingDirInitializer(taskSpec.Steps); workingDir != nil {
		initContainers = append(initContainers, *workingDir)
	}

	maxIndicesByResource := findMaxResourceRequest(taskSpec.Steps, corev1.ResourceCPU, corev1.ResourceMemory, corev1.ResourceEphemeralStorage)

	for i := range taskSpec.Steps {
		step := &taskSpec.Steps[i]
		step.Env = append(implicitEnvVars, step.Env...)
		// TODO(mattmoor): Check that volumeMounts match volumes.

		// Add implicit volume mounts, unless the user has requested
		// their own volume mount at that path.
		requestedVolumeMounts := map[string]bool{}
		for _, vm := range step.VolumeMounts {
			requestedVolumeMounts[filepath.Clean(vm.MountPath)] = true
		}
		for _, imp := range implicitVolumeMounts {
			if !requestedVolumeMounts[filepath.Clean(imp.MountPath)] {
				step.VolumeMounts = append(step.VolumeMounts, imp)
			}
		}

		if step.WorkingDir == "" {
			step.WorkingDir = workspaceDir
		}
		if step.Name == "" {
			step.Name = fmt.Sprintf("%v%d", unnamedInitContainerPrefix, i)
		} else {
			step.Name = names.SimpleNameGenerator.RestrictLength(fmt.Sprintf("%v%v", containerPrefix, step.Name))
		}
		// use the step name to add the entrypoint biary as an init container
		if step.Name == names.SimpleNameGenerator.RestrictLength(fmt.Sprintf("%v%v", containerPrefix, entrypoint.InitContainerName)) {
			initContainers = append(initContainers, *step)
		} else {
			zeroNonMaxResourceRequests(step, i, maxIndicesByResource)
			podContainers = append(podContainers, *step)
		}
	}
	// Add our implicit volumes and any volumes needed for secrets to the explicitly
	// declared user volumes.
	volumes := append(taskSpec.Volumes, implicitVolumes...)
	volumes = append(volumes, secrets...)
	if err := v1alpha1.ValidateVolumes(volumes); err != nil {
		return nil, err
	}

	// Generate a short random hex string.
	b, err := ioutil.ReadAll(io.LimitReader(randReader, 3))
	if err != nil {
		return nil, err
	}
	gibberish := hex.EncodeToString(b)

	nopContainer := &corev1.Container{Name: "nop", Image: *nopImage, Command: []string{"/ko-app/nop"}}
	if err := entrypoint.RedirectStep(cache, len(podContainers), nopContainer, kubeclient, taskRun, logger); err != nil {
		return nil, err
	}
	podContainers = append(podContainers, *nopContainer)

	mergedInitContainers, err := merge.CombineStepsWithContainerTemplate(taskSpec.ContainerTemplate, initContainers)
	if err != nil {
		return nil, err
	}
	mergedPodContainers, err := merge.CombineStepsWithContainerTemplate(taskSpec.ContainerTemplate, podContainers)
	if err != nil {
		return nil, err
	}

	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			// We execute the build's pod in the same namespace as where the build was
			// created so that it can access colocated resources.
			Namespace: taskRun.Namespace,
			// Generate a unique name based on the build's name.
			// Add a unique suffix to avoid confusion when a build
			// is deleted and re-created with the same name.
			// We don't use RestrictLengthWithRandomSuffix here because k8s fakes don't support it.
			Name: fmt.Sprintf("%s-pod-%s", taskRun.Name, gibberish),
			// If our parent TaskRun is deleted, then we should be as well.
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(taskRun, groupVersionKind),
			},
			Annotations: makeAnnotations(taskRun),
			Labels:      makeLabels(taskRun),
		},
		Spec: corev1.PodSpec{
			RestartPolicy:      corev1.RestartPolicyNever,
			InitContainers:     mergedInitContainers,
			Containers:         mergedPodContainers,
			ServiceAccountName: taskRun.Spec.ServiceAccount,
			Volumes:            volumes,
			NodeSelector:       taskRun.Spec.NodeSelector,
			Tolerations:        taskRun.Spec.Tolerations,
			Affinity:           taskRun.Spec.Affinity,
		},
	}, nil
}

// makeLabels constructs the labels we will propagate from TaskRuns to Pods.
func makeLabels(s *v1alpha1.TaskRun) map[string]string {
	labels := make(map[string]string, len(s.ObjectMeta.Labels)+1)
	for k, v := range s.ObjectMeta.Labels {
		labels[k] = v
	}
	labels[pipeline.GroupName+pipeline.TaskRunLabelKey] = s.Name
	return labels
}

// makeAnnotations constructs the annotations we will propagate from TaskRuns to Pods.
func makeAnnotations(s *v1alpha1.TaskRun) map[string]string {
	annotations := make(map[string]string, len(s.ObjectMeta.Annotations)+1)
	for k, v := range s.ObjectMeta.Annotations {
		annotations[k] = v
	}
	annotations["sidecar.istio.io/inject"] = "false"
	return annotations
}

// zeroNonMaxResourceRequests zeroes out the container's cpu, memory, or
// ephemeral storage resource requests if the container does not have the
// largest request out of all containers in the pod. This is done because Tekton
// overwrites each container's entrypoint to make containers effectively execute
// one at a time, so we want pods to only request the maximum resources needed
// at any single point in time. If no container has an explicit resource
// request, all requests are set to 0.
func zeroNonMaxResourceRequests(container *corev1.Container, containerIndex int, maxIndicesByResource map[corev1.ResourceName]int) {
	if container.Resources.Requests == nil {
		container.Resources.Requests = corev1.ResourceList{}
	}
	for name, maxIdx := range maxIndicesByResource {
		if maxIdx != containerIndex {
			container.Resources.Requests[name] = zeroQty
		}
	}
}

// findMaxResourceRequest returns the index of the container with the maximum
// request for the given resource from among the given set of containers.
func findMaxResourceRequest(containers []corev1.Container, resourceNames ...corev1.ResourceName) map[corev1.ResourceName]int {
	maxIdxs := make(map[corev1.ResourceName]int, len(resourceNames))
	maxReqs := make(map[corev1.ResourceName]resource.Quantity, len(resourceNames))
	for _, name := range resourceNames {
		maxIdxs[name] = -1
		maxReqs[name] = zeroQty
	}
	for i, c := range containers {
		for _, name := range resourceNames {
			maxReq := maxReqs[name]
			req, exists := c.Resources.Requests[name]
			if exists && req.Cmp(maxReq) > 0 {
				maxIdxs[name] = i
				maxReqs[name] = req
			}
		}
	}
	return maxIdxs
}

// TrimContainerNamePrefix trim the container name prefix to get the corresponding step name
func TrimContainerNamePrefix(containerName string) string {
	return strings.TrimPrefix(containerName, containerPrefix)
}
