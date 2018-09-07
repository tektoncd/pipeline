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
package convert

import (
	"flag"
	"fmt"
	"reflect"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"

	v1alpha1 "github.com/knative/build/pkg/apis/build/v1alpha1"
	"github.com/knative/build/pkg/builder/validation"
	"github.com/knative/build/pkg/credentials"
	"github.com/knative/build/pkg/credentials/dockercreds"
	"github.com/knative/build/pkg/credentials/gitcreds"
)

// These are effectively const, but Go doesn't have such an annotation.
var (
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
		MountPath: "/workspace",
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
	// A benign placeholder for when a container is required, but none was specified.
	nopContainer = corev1.Container{
		Name:    "nop",
		Image:   "busybox",
		Command: []string{"/bin/echo"},
		Args:    []string{"Nothing to push"},
	}
)

func validateVolumes(vs []corev1.Volume) error {
	seen := make(map[string]interface{})
	for i, v := range vs {
		if _, ok := seen[v.Name]; ok {
			return validation.NewError("DuplicateVolume", "saw Volume %q defined multiple times", v.Name)
		}
		seen[v.Name] = i
	}
	return nil
}

const (
	// Prefixes to add to the name of the init containers.
	// IMPORTANT: Changing these values without changing fluentd collection configuration
	// will break log collection for init containers.
	initContainerPrefix        = "build-step-"
	unnamedInitContainerPrefix = "build-step-unnamed-"
	// A label with the following is added to the pod to identify the pods belonging to a build.
	buildNameLabelKey = "build-name"
	// Name of the credential initialization container.
	credsInit = "credential-initializer"
	// Names for source containers.
	gitSource    = "git-source"
	gcsSource    = "gcs-source"
	customSource = "custom-source"
)

var (
	// The container used to initialize credentials before the build runs.
	credsImage = flag.String("creds-image", "override-with-creds:latest",
		"The container image for preparing our Build's credentials.")
	// The container with Git that we use to implement the Git source step.
	gitImage = flag.String("git-image", "override-with-git:latest",
		"The container image containing our Git binary.")
	gcsFetcherImage = flag.String("gcs-fetcher-image", "gcr.io/cloud-builders/gcs-fetcher:latest",
		"The container image containing our GCS fetcher binary.")
)

var (
	// Used to reverse the mapping from source to containers based on the
	// name given to the first step.  This is fragile, but predominantly for testing.
	containerToSourceMap = map[string]func(corev1.Container) (*v1alpha1.SourceSpec, error){
		gitSource:    containerToGit,
		gcsSource:    containerToGCS,
		customSource: containerToCustom,
	}
)

// TODO(mattmoor): Should we move this somewhere common, because of the flag?
func gitToContainer(git *v1alpha1.GitSourceSpec) (*corev1.Container, error) {
	if git.Url == "" {
		return nil, validation.NewError("MissingUrl", "git sources are expected to specify a Url, got: %v", git)
	}
	if git.Revision == "" {
		return nil, validation.NewError("MissingRevision", "git sources are expected to specify a Revision, got: %v", git)
	}
	return &corev1.Container{
		Name:  gitSource,
		Image: *gitImage,
		Args: []string{
			"-url", git.Url,
			"-revision", git.Revision,
		},
	}, nil
}

func containerToGit(git corev1.Container) (*v1alpha1.SourceSpec, error) {
	if git.Image != *gitImage {
		return nil, fmt.Errorf("Unrecognized git source image: %v", git.Image)
	}
	if len(git.Args) < 3 {
		return nil, fmt.Errorf("Unexpectedly few arguments to git source container: %v", git.Args)
	}
	// Now undo what we did above
	return &v1alpha1.SourceSpec{
		Git: &v1alpha1.GitSourceSpec{
			Url:      git.Args[1],
			Revision: git.Args[3],
		},
	}, nil
}

func gcsToContainer(gcs *v1alpha1.GCSSourceSpec) (*corev1.Container, error) {
	if gcs.Location == "" {
		return nil, validation.NewError("MissingLocation", "gcs sources are expected to specify a Location, got: %v", gcs)
	}
	return &corev1.Container{
		Name:  gcsSource,
		Image: *gcsFetcherImage,
		Args:  []string{"--type", string(gcs.Type), "--location", gcs.Location},
	}, nil
}

func containerToGCS(source corev1.Container) (*v1alpha1.SourceSpec, error) {
	var sourceType, location string
	for i, a := range source.Args {
		if a == "--type" && i < len(source.Args) {
			sourceType = source.Args[i+1]
		}
		if a == "--location" && i < len(source.Args) {
			location = source.Args[i+1]
		}
	}
	return &v1alpha1.SourceSpec{
		GCS: &v1alpha1.GCSSourceSpec{
			Type:     v1alpha1.GCSSourceType(sourceType),
			Location: location,
		},
	}, nil
}

func customToContainer(source *corev1.Container) (*corev1.Container, error) {
	if source.Name != "" {
		return nil, validation.NewError("OmitName", "custom source containers are expected to omit Name, got: %v", source.Name)
	}
	custom := source.DeepCopy()
	custom.Name = customSource
	return custom, nil
}

func containerToCustom(custom corev1.Container) (*v1alpha1.SourceSpec, error) {
	c := custom.DeepCopy()
	c.Name = ""
	return &v1alpha1.SourceSpec{Custom: c}, nil
}

func sourceToContainer(source *v1alpha1.SourceSpec) (*corev1.Container, error) {
	switch {
	case source == nil:
		return nil, nil
	case source.Git != nil:
		return gitToContainer(source.Git)
	case source.GCS != nil:
		return gcsToContainer(source.GCS)
	case source.Custom != nil:
		return customToContainer(source.Custom)
	default:
		return nil, validation.NewError("UnrecognizedSource", "saw SourceSpec with no supported contents: %v", source)
	}
}

func makeCredentialInitializer(build *v1alpha1.Build, kubeclient kubernetes.Interface) (*corev1.Container, []corev1.Volume, error) {
	serviceAccountName := build.Spec.ServiceAccountName
	if serviceAccountName == "" {
		serviceAccountName = "default"
	}

	sa, err := kubeclient.CoreV1().ServiceAccounts(build.Namespace).Get(serviceAccountName, metav1.GetOptions{})
	if err != nil {
		return nil, nil, err
	}

	builders := []credentials.Builder{dockercreds.NewBuilder(), gitcreds.NewBuilder()}

	// Collect the volume declarations, there mounts into the cred-init container, and the arguments to it.
	volumes := []corev1.Volume{}
	volumeMounts := []corev1.VolumeMount{}
	args := []string{}
	for _, secretEntry := range sa.Secrets {
		secret, err := kubeclient.CoreV1().Secrets(build.Namespace).Get(secretEntry.Name, metav1.GetOptions{})
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
			name := fmt.Sprintf("secret-volume-%s", secret.Name)
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
		Name:         credsInit,
		Image:        *credsImage,
		Args:         args,
		VolumeMounts: volumeMounts,
	}, volumes, nil
}

func FromCRD(build *v1alpha1.Build, kubeclient kubernetes.Interface) (*corev1.Pod, error) {
	build = build.DeepCopy()

	cred, secrets, err := makeCredentialInitializer(build, kubeclient)
	if err != nil {
		return nil, err
	}
	setupContainers := []corev1.Container{*cred}
	extraVolumes := append(implicitVolumes, secrets...)

	if build.Spec.Source != nil {
		scm, err := sourceToContainer(build.Spec.Source)
		if err != nil {
			return nil, err
		}
		setupContainers = append(setupContainers, *scm)
	}

	// Add the implicit volume mounts to each step container.
	var initContainers []corev1.Container
	for i, step := range append(setupContainers, build.Spec.Steps...) {
		if step.WorkingDir == "" {
			step.WorkingDir = "/workspace"
		}
		if step.Name == "" {
			step.Name = fmt.Sprintf("%v%d", unnamedInitContainerPrefix, i)
		} else {
			step.Name = fmt.Sprintf("%v%v", initContainerPrefix, step.Name)
		}
		step.Env = append(implicitEnvVars, step.Env...)
		// TODO(mattmoor): Check that volumeMounts match volumes.
		step.VolumeMounts = append(step.VolumeMounts, implicitVolumeMounts...)
		initContainers = append(initContainers, step)
	}
	// Add our implicit volumes and any volumes needed for secrets to the explicitly
	// declared user volumes.
	volumes := append(build.Spec.Volumes, extraVolumes...)
	if err := validateVolumes(volumes); err != nil {
		return nil, err
	}

	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			// We execute the build's pod in the same namespace as where the build was
			// created so that it can access colocated resources.
			Namespace: build.Namespace,
			// Ensure our Pod gets a unique name.
			GenerateName: fmt.Sprintf("%s-", build.Name),
			// If our parent Build is deleted, then we should be as well.
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(build, schema.GroupVersionKind{
					Group:   v1alpha1.SchemeGroupVersion.Group,
					Version: v1alpha1.SchemeGroupVersion.Version,
					Kind:    "Build",
				}),
			},
			Annotations: map[string]string{
				"sidecar.istio.io/inject": "false",
			},
			Labels: map[string]string{
				buildNameLabelKey: build.Name,
			},
		},
		Spec: corev1.PodSpec{
			// If the build fails, don't restart it.
			RestartPolicy:      corev1.RestartPolicyNever,
			InitContainers:     initContainers,
			Containers:         []corev1.Container{nopContainer},
			ServiceAccountName: build.Spec.ServiceAccountName,
			Volumes:            volumes,
		},
	}, nil
}

func isImplicitEnvVar(ev corev1.EnvVar) bool {
	for _, iev := range implicitEnvVars {
		if ev.Name == iev.Name {
			return true
		}
	}
	return false
}

func filterImplicitEnvVars(evs []corev1.EnvVar) []corev1.EnvVar {
	var envs []corev1.EnvVar
	for _, ev := range evs {
		if isImplicitEnvVar(ev) {
			continue
		}
		envs = append(envs, ev)
	}
	return envs
}

func isImplicitVolumeMount(vm corev1.VolumeMount) bool {
	for _, ivm := range implicitVolumeMounts {
		if vm.Name == ivm.Name {
			return true
		}
	}
	return false
}

func filterImplicitVolumeMounts(vms []corev1.VolumeMount) []corev1.VolumeMount {
	var volumes []corev1.VolumeMount
	for _, vm := range vms {
		if isImplicitVolumeMount(vm) {
			continue
		}
		volumes = append(volumes, vm)
	}
	return volumes
}

func isImplicitVolume(v corev1.Volume) bool {
	for _, iv := range implicitVolumes {
		if v.Name == iv.Name {
			return true
		}
	}
	if strings.HasPrefix(v.Name, "secret-volume-") {
		return true
	}
	return false
}

func filterImplicitVolumes(vs []corev1.Volume) []corev1.Volume {
	var volumes []corev1.Volume
	for _, v := range vs {
		if isImplicitVolume(v) {
			continue
		}
		volumes = append(volumes, v)
	}
	return volumes
}

func ToCRD(pod *corev1.Pod) (*v1alpha1.Build, error) {
	podSpec := pod.Spec.DeepCopy()

	for _, c := range podSpec.Containers {
		if !reflect.DeepEqual(c, nopContainer) {
			return nil, fmt.Errorf("unrecognized container spec, got: %v", podSpec.Containers)
		}
	}

	var steps []corev1.Container
	for _, step := range podSpec.InitContainers {
		if step.WorkingDir == "/workspace" {
			step.WorkingDir = ""
		}
		step.Env = filterImplicitEnvVars(step.Env)
		step.VolumeMounts = filterImplicitVolumeMounts(step.VolumeMounts)
		// Strip the init container prefix that is added automatically.
		if strings.HasPrefix(step.Name, unnamedInitContainerPrefix) {
			step.Name = ""
		} else {
			step.Name = strings.TrimPrefix(step.Name, initContainerPrefix)
		}
		steps = append(steps, step)
	}
	volumes := filterImplicitVolumes(podSpec.Volumes)

	// Strip the credential initializer.
	if steps[0].Name == credsInit {
		steps = steps[1:]
	}

	var scm *v1alpha1.SourceSpec
	if conv, ok := containerToSourceMap[steps[0].Name]; ok {
		src, err := conv(steps[0])
		if err != nil {
			return nil, err
		}
		// The first init container is actually a source step.  Convert
		// it to our source spec and pop it off the list of steps.
		scm = src
		steps = steps[1:]
	}

	return &v1alpha1.Build{
		// TODO(mattmoor): What should we do for ObjectMeta stuff?
		Spec: v1alpha1.BuildSpec{
			Source:             scm,
			Steps:              steps,
			ServiceAccountName: podSpec.ServiceAccountName,
			Volumes:            volumes,
		},
	}, nil
}
