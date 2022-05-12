package v1beta1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Step runs a subcomponent of a Task
type Step struct {

	// Name of the container specified as a DNS_LABEL.
	// Each container in a pod must have a unique name (DNS_LABEL).
	// Cannot be updated.
	Name string `json:"name" protobuf:"bytes,1,opt,name=name"`
	// Docker image name.
	// More info: https://kubernetes.io/docs/concepts/containers/images
	// This field is optional to allow higher level config management to default or override
	// container images in workload controllers like Deployments and StatefulSets.
	// +optional
	Image string `json:"image,omitempty" protobuf:"bytes,2,opt,name=image"`
	// Entrypoint array. Not executed within a shell.
	// The docker image's ENTRYPOINT is used if this is not provided.
	// Variable references $(VAR_NAME) are expanded using the container's environment. If a variable
	// cannot be resolved, the reference in the input string will be unchanged. Double $$ are reduced
	// to a single $, which allows for escaping the $(VAR_NAME) syntax: i.e. "$$(VAR_NAME)" will
	// produce the string literal "$(VAR_NAME)". Escaped references will never be expanded, regardless
	// of whether the variable exists or not. Cannot be updated.
	// More info: https://kubernetes.io/docs/tasks/inject-data-application/define-command-argument-container/#running-a-command-in-a-shell
	// +optional
	// +listType=atomic
	Command []string `json:"command,omitempty" protobuf:"bytes,3,rep,name=command"`
	// Arguments to the entrypoint.
	// The docker image's CMD is used if this is not provided.
	// Variable references $(VAR_NAME) are expanded using the container's environment. If a variable
	// cannot be resolved, the reference in the input string will be unchanged. Double $$ are reduced
	// to a single $, which allows for escaping the $(VAR_NAME) syntax: i.e. "$$(VAR_NAME)" will
	// produce the string literal "$(VAR_NAME)". Escaped references will never be expanded, regardless
	// of whether the variable exists or not. Cannot be updated.
	// More info: https://kubernetes.io/docs/tasks/inject-data-application/define-command-argument-container/#running-a-command-in-a-shell
	// +optional
	// +listType=atomic
	Args []string `json:"args,omitempty" protobuf:"bytes,4,rep,name=args"`
	// Container's working directory.
	// If not specified, the container runtime's default will be used, which
	// might be configured in the container image.
	// Cannot be updated.
	// +optional
	WorkingDir string `json:"workingDir,omitempty" protobuf:"bytes,5,opt,name=workingDir"`
	// Deprecated. This field will be removed in a future release.
	// List of ports to expose from the container. Exposing a port here gives
	// the system additional information about the network connections a
	// container uses, but is primarily informational. Not specifying a port here
	// DOES NOT prevent that port from being exposed. Any port which is
	// listening on the default "0.0.0.0" address inside a container will be
	// accessible from the network.
	// Cannot be updated.
	// +optional
	// +patchMergeKey=containerPort
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=containerPort
	// +listMapKey=protocol
	DeprecatedPorts []corev1.ContainerPort `json:"ports,omitempty" patchStrategy:"merge" patchMergeKey:"containerPort" protobuf:"bytes,6,rep,name=ports"`
	// List of sources to populate environment variables in the container.
	// The keys defined within a source must be a C_IDENTIFIER. All invalid keys
	// will be reported as an event when the container is starting. When a key exists in multiple
	// sources, the value associated with the last source will take precedence.
	// Values defined by an Env with a duplicate key will take precedence.
	// Cannot be updated.
	// +optional
	// +listType=atomic
	EnvFrom []corev1.EnvFromSource `json:"envFrom,omitempty" protobuf:"bytes,19,rep,name=envFrom"`
	// List of environment variables to set in the container.
	// Cannot be updated.
	// +optional
	// +patchMergeKey=name
	// +patchStrategy=merge
	// +listType=atomic
	Env []corev1.EnvVar `json:"env,omitempty" patchStrategy:"merge" patchMergeKey:"name" protobuf:"bytes,7,rep,name=env"`
	// Compute Resources required by this container.
	// Cannot be updated.
	// More info: https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/
	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty" protobuf:"bytes,8,opt,name=resources"`
	// Pod volumes to mount into the container's filesystem.
	// Cannot be updated.
	// +optional
	// +patchMergeKey=mountPath
	// +patchStrategy=merge
	// +listType=atomic
	VolumeMounts []corev1.VolumeMount `json:"volumeMounts,omitempty" patchStrategy:"merge" patchMergeKey:"mountPath" protobuf:"bytes,9,rep,name=volumeMounts"`
	// volumeDevices is the list of block devices to be used by the container.
	// +patchMergeKey=devicePath
	// +patchStrategy=merge
	// +optional
	// +listType=atomic
	VolumeDevices []corev1.VolumeDevice `json:"volumeDevices,omitempty" patchStrategy:"merge" patchMergeKey:"devicePath" protobuf:"bytes,21,rep,name=volumeDevices"`
	// Deprecated. This field will be removed in a future release.
	// Periodic probe of container liveness.
	// Container will be restarted if the probe fails.
	// Cannot be updated.
	// More info: https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle#container-probes
	// +optional
	DeprecatedLivenessProbe *corev1.Probe `json:"livenessProbe,omitempty" protobuf:"bytes,10,opt,name=livenessProbe"`
	// Deprecated. This field will be removed in a future release.
	// Periodic probe of container service readiness.
	// Container will be removed from service endpoints if the probe fails.
	// Cannot be updated.
	// More info: https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle#container-probes
	// +optional
	DeprecatedReadinessProbe *corev1.Probe `json:"readinessProbe,omitempty" protobuf:"bytes,11,opt,name=readinessProbe"`
	// Deprecated. This field will be removed in a future release.
	// DeprecatedStartupProbe indicates that the Pod has successfully initialized.
	// If specified, no other probes are executed until this completes successfully.
	// If this probe fails, the Pod will be restarted, just as if the livenessProbe failed.
	// This can be used to provide different probe parameters at the beginning of a Pod's lifecycle,
	// when it might take a long time to load data or warm a cache, than during steady-state operation.
	// This cannot be updated.
	// More info: https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle#container-probes
	// +optional
	DeprecatedStartupProbe *corev1.Probe `json:"startupProbe,omitempty" protobuf:"bytes,22,opt,name=startupProbe"`
	// Deprecated. This field will be removed in a future release.
	// Actions that the management system should take in response to container lifecycle events.
	// Cannot be updated.
	// +optional
	DeprecatedLifecycle *corev1.Lifecycle `json:"lifecycle,omitempty" protobuf:"bytes,12,opt,name=lifecycle"`
	// Deprecated. This field will be removed in a future release.
	// Optional: Path at which the file to which the container's termination message
	// will be written is mounted into the container's filesystem.
	// Message written is intended to be brief final status, such as an assertion failure message.
	// Will be truncated by the node if greater than 4096 bytes. The total message length across
	// all containers will be limited to 12kb.
	// Defaults to /dev/termination-log.
	// Cannot be updated.
	// +optional
	DeprecatedTerminationMessagePath string `json:"terminationMessagePath,omitempty" protobuf:"bytes,13,opt,name=terminationMessagePath"`
	// Deprecated. This field will be removed in a future release.
	// Indicate how the termination message should be populated. File will use the contents of
	// terminationMessagePath to populate the container status message on both success and failure.
	// FallbackToLogsOnError will use the last chunk of container log output if the termination
	// message file is empty and the container exited with an error.
	// The log output is limited to 2048 bytes or 80 lines, whichever is smaller.
	// Defaults to File.
	// Cannot be updated.
	// +optional
	DeprecatedTerminationMessagePolicy corev1.TerminationMessagePolicy `json:"terminationMessagePolicy,omitempty" protobuf:"bytes,20,opt,name=terminationMessagePolicy,casttype=TerminationMessagePolicy"`
	// Image pull policy.
	// One of Always, Never, IfNotPresent.
	// Defaults to Always if :latest tag is specified, or IfNotPresent otherwise.
	// Cannot be updated.
	// More info: https://kubernetes.io/docs/concepts/containers/images#updating-images
	// +optional
	ImagePullPolicy corev1.PullPolicy `json:"imagePullPolicy,omitempty" protobuf:"bytes,14,opt,name=imagePullPolicy,casttype=PullPolicy"`
	// SecurityContext defines the security options the container should be run with.
	// If set, the fields of SecurityContext override the equivalent fields of PodSecurityContext.
	// More info: https://kubernetes.io/docs/tasks/configure-pod-container/security-context/
	// +optional
	SecurityContext *corev1.SecurityContext `json:"securityContext,omitempty" protobuf:"bytes,15,opt,name=securityContext"`

	// Variables for interactive containers, these have very specialized use-cases (e.g. debugging)
	// and shouldn't be used for general purpose containers.

	// Deprecated. This field will be removed in a future release.
	// Whether this container should allocate a buffer for stdin in the container runtime. If this
	// is not set, reads from stdin in the container will always result in EOF.
	// Default is false.
	// +optional
	DeprecatedStdin bool `json:"stdin,omitempty" protobuf:"varint,16,opt,name=stdin"`
	// Deprecated. This field will be removed in a future release.
	// Whether the container runtime should close the stdin channel after it has been opened by
	// a single attach. When stdin is true the stdin stream will remain open across multiple attach
	// sessions. If stdinOnce is set to true, stdin is opened on container start, is empty until the
	// first client attaches to stdin, and then remains open and accepts data until the client disconnects,
	// at which time stdin is closed and remains closed until the container is restarted. If this
	// flag is false, a container processes that reads from stdin will never receive an EOF.
	// Default is false
	// +optional
	DeprecatedStdinOnce bool `json:"stdinOnce,omitempty" protobuf:"varint,17,opt,name=stdinOnce"`
	// Deprecated. This field will be removed in a future release.
	// Whether this container should allocate a DeprecatedTTY for itself, also requires 'stdin' to be true.
	// Default is false.
	// +optional
	DeprecatedTTY bool `json:"tty,omitempty" protobuf:"varint,18,opt,name=tty"`

	// Script is the contents of an executable file to execute.
	//
	// If Script is not empty, the Step cannot have an Command and the Args will be passed to the Script.
	// +optional
	Script string `json:"script,omitempty"`

	// Timeout is the time after which the step times out. Defaults to never.
	// Refer to Go's ParseDuration documentation for expected format: https://golang.org/pkg/time/#ParseDuration
	// +optional
	Timeout *metav1.Duration `json:"timeout,omitempty"`

	// This is an alpha field. You must set the "enable-api-fields" feature flag to "alpha"
	// for this field to be supported.
	//
	// Workspaces is a list of workspaces from the Task that this Step wants
	// exclusive access to. Adding a workspace to this list means that any
	// other Step or Sidecar that does not also request this Workspace will
	// not have access to it.
	// +optional
	// +listType=atomic
	Workspaces []WorkspaceUsage `json:"workspaces,omitempty"`

	// OnError defines the exiting behavior of a container on error
	// can be set to [ continue | stopAndFail ]
	// stopAndFail indicates exit the taskRun if the container exits with non-zero exit code
	// continue indicates continue executing the rest of the steps irrespective of the container exit code
	OnError string `json:"onError,omitempty"`
}

// ToK8sContainer converts the Step to a Kubernetes Container struct
func (s *Step) ToK8sContainer() *corev1.Container {
	return &corev1.Container{
		Name:                   s.Name,
		Image:                  s.Image,
		Command:                s.Command,
		Args:                   s.Args,
		WorkingDir:             s.WorkingDir,
		Ports:                  s.DeprecatedPorts,
		EnvFrom:                s.EnvFrom,
		Env:                    s.Env,
		Resources:              s.Resources,
		VolumeMounts:           s.VolumeMounts,
		VolumeDevices:          s.VolumeDevices,
		LivenessProbe:          s.DeprecatedLivenessProbe,
		ReadinessProbe:         s.DeprecatedReadinessProbe,
		StartupProbe:           s.DeprecatedStartupProbe,
		Lifecycle:              s.DeprecatedLifecycle,
		TerminationMessagePath: s.DeprecatedTerminationMessagePath,
		ImagePullPolicy:        s.ImagePullPolicy,
		SecurityContext:        s.SecurityContext,
		Stdin:                  s.DeprecatedStdin,
		StdinOnce:              s.DeprecatedStdinOnce,
		TTY:                    s.DeprecatedTTY,
	}
}

// SetContainerFields sets the fields of the Step to the values of the corresponding fields in the Container
func (s *Step) SetContainerFields(c corev1.Container) {
	s.Name = c.Name
	s.Image = c.Image
	s.Command = c.Command
	s.Args = c.Args
	s.WorkingDir = c.WorkingDir
	s.DeprecatedPorts = c.Ports
	s.EnvFrom = c.EnvFrom
	s.Env = c.Env
	s.Resources = c.Resources
	s.VolumeMounts = c.VolumeMounts
	s.VolumeDevices = c.VolumeDevices
	s.DeprecatedLivenessProbe = c.LivenessProbe
	s.DeprecatedReadinessProbe = c.ReadinessProbe
	s.DeprecatedStartupProbe = c.StartupProbe
	s.DeprecatedLifecycle = c.Lifecycle
	s.DeprecatedTerminationMessagePath = c.TerminationMessagePath
	s.ImagePullPolicy = c.ImagePullPolicy
	s.SecurityContext = c.SecurityContext
	s.DeprecatedStdin = c.Stdin
	s.DeprecatedStdinOnce = c.StdinOnce
	s.DeprecatedTTY = c.TTY
}

// StepTemplate is a template for a Step
type StepTemplate struct {

	// Deprecated. This field will be removed in a future release.
	// DeprecatedName of the container specified as a DNS_LABEL.
	// Each container in a pod must have a unique name (DNS_LABEL).
	// Cannot be updated.
	DeprecatedName string `json:"name" protobuf:"bytes,1,opt,name=name"`
	// Docker image name.
	// More info: https://kubernetes.io/docs/concepts/containers/images
	// This field is optional to allow higher level config management to default or override
	// container images in workload controllers like Deployments and StatefulSets.
	// +optional
	Image string `json:"image,omitempty" protobuf:"bytes,2,opt,name=image"`
	// Entrypoint array. Not executed within a shell.
	// The docker image's ENTRYPOINT is used if this is not provided.
	// Variable references $(VAR_NAME) are expanded using the container's environment. If a variable
	// cannot be resolved, the reference in the input string will be unchanged. Double $$ are reduced
	// to a single $, which allows for escaping the $(VAR_NAME) syntax: i.e. "$$(VAR_NAME)" will
	// produce the string literal "$(VAR_NAME)". Escaped references will never be expanded, regardless
	// of whether the variable exists or not. Cannot be updated.
	// More info: https://kubernetes.io/docs/tasks/inject-data-application/define-command-argument-container/#running-a-command-in-a-shell
	// +optional
	// +listType=atomic
	Command []string `json:"command,omitempty" protobuf:"bytes,3,rep,name=command"`
	// Arguments to the entrypoint.
	// The docker image's CMD is used if this is not provided.
	// Variable references $(VAR_NAME) are expanded using the container's environment. If a variable
	// cannot be resolved, the reference in the input string will be unchanged. Double $$ are reduced
	// to a single $, which allows for escaping the $(VAR_NAME) syntax: i.e. "$$(VAR_NAME)" will
	// produce the string literal "$(VAR_NAME)". Escaped references will never be expanded, regardless
	// of whether the variable exists or not. Cannot be updated.
	// More info: https://kubernetes.io/docs/tasks/inject-data-application/define-command-argument-container/#running-a-command-in-a-shell
	// +optional
	// +listType=atomic
	Args []string `json:"args,omitempty" protobuf:"bytes,4,rep,name=args"`
	// Container's working directory.
	// If not specified, the container runtime's default will be used, which
	// might be configured in the container image.
	// Cannot be updated.
	// +optional
	WorkingDir string `json:"workingDir,omitempty" protobuf:"bytes,5,opt,name=workingDir"`
	// Deprecated. This field will be removed in a future release.
	// List of ports to expose from the container. Exposing a port here gives
	// the system additional information about the network connections a
	// container uses, but is primarily informational. Not specifying a port here
	// DOES NOT prevent that port from being exposed. Any port which is
	// listening on the default "0.0.0.0" address inside a container will be
	// accessible from the network.
	// Cannot be updated.
	// +optional
	// +patchMergeKey=containerPort
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=containerPort
	// +listMapKey=protocol
	DeprecatedPorts []corev1.ContainerPort `json:"ports,omitempty" patchStrategy:"merge" patchMergeKey:"containerPort" protobuf:"bytes,6,rep,name=ports"`
	// List of sources to populate environment variables in the container.
	// The keys defined within a source must be a C_IDENTIFIER. All invalid keys
	// will be reported as an event when the container is starting. When a key exists in multiple
	// sources, the value associated with the last source will take precedence.
	// Values defined by an Env with a duplicate key will take precedence.
	// Cannot be updated.
	// +optional
	// +listType=atomic
	EnvFrom []corev1.EnvFromSource `json:"envFrom,omitempty" protobuf:"bytes,19,rep,name=envFrom"`
	// List of environment variables to set in the container.
	// Cannot be updated.
	// +optional
	// +patchMergeKey=name
	// +patchStrategy=merge
	// +listType=atomic
	Env []corev1.EnvVar `json:"env,omitempty" patchStrategy:"merge" patchMergeKey:"name" protobuf:"bytes,7,rep,name=env"`
	// Compute Resources required by this container.
	// Cannot be updated.
	// More info: https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/
	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty" protobuf:"bytes,8,opt,name=resources"`
	// Pod volumes to mount into the container's filesystem.
	// Cannot be updated.
	// +optional
	// +patchMergeKey=mountPath
	// +patchStrategy=merge
	// +listType=atomic
	VolumeMounts []corev1.VolumeMount `json:"volumeMounts,omitempty" patchStrategy:"merge" patchMergeKey:"mountPath" protobuf:"bytes,9,rep,name=volumeMounts"`
	// volumeDevices is the list of block devices to be used by the container.
	// +patchMergeKey=devicePath
	// +patchStrategy=merge
	// +optional
	// +listType=atomic
	VolumeDevices []corev1.VolumeDevice `json:"volumeDevices,omitempty" patchStrategy:"merge" patchMergeKey:"devicePath" protobuf:"bytes,21,rep,name=volumeDevices"`
	// Deprecated. This field will be removed in a future release.
	// Periodic probe of container liveness.
	// Container will be restarted if the probe fails.
	// Cannot be updated.
	// More info: https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle#container-probes
	// +optional
	DeprecatedLivenessProbe *corev1.Probe `json:"livenessProbe,omitempty" protobuf:"bytes,10,opt,name=livenessProbe"`
	// Deprecated. This field will be removed in a future release.
	// Periodic probe of container service readiness.
	// Container will be removed from service endpoints if the probe fails.
	// Cannot be updated.
	// More info: https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle#container-probes
	// +optional
	DeprecatedReadinessProbe *corev1.Probe `json:"readinessProbe,omitempty" protobuf:"bytes,11,opt,name=readinessProbe"`
	// Deprecated. This field will be removed in a future release.
	// DeprecatedStartupProbe indicates that the Pod has successfully initialized.
	// If specified, no other probes are executed until this completes successfully.
	// If this probe fails, the Pod will be restarted, just as if the livenessProbe failed.
	// This can be used to provide different probe parameters at the beginning of a Pod's lifecycle,
	// when it might take a long time to load data or warm a cache, than during steady-state operation.
	// This cannot be updated.
	// More info: https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle#container-probes
	// +optional
	DeprecatedStartupProbe *corev1.Probe `json:"startupProbe,omitempty" protobuf:"bytes,22,opt,name=startupProbe"`
	// Deprecated. This field will be removed in a future release.
	// Actions that the management system should take in response to container lifecycle events.
	// Cannot be updated.
	// +optional
	DeprecatedLifecycle *corev1.Lifecycle `json:"lifecycle,omitempty" protobuf:"bytes,12,opt,name=lifecycle"`
	// Deprecated. This field will be removed in a future release.
	// Optional: Path at which the file to which the container's termination message
	// will be written is mounted into the container's filesystem.
	// Message written is intended to be brief final status, such as an assertion failure message.
	// Will be truncated by the node if greater than 4096 bytes. The total message length across
	// all containers will be limited to 12kb.
	// Defaults to /dev/termination-log.
	// Cannot be updated.
	// +optional
	DeprecatedTerminationMessagePath string `json:"terminationMessagePath,omitempty" protobuf:"bytes,13,opt,name=terminationMessagePath"`
	// Deprecated. This field will be removed in a future release.
	// Indicate how the termination message should be populated. File will use the contents of
	// terminationMessagePath to populate the container status message on both success and failure.
	// FallbackToLogsOnError will use the last chunk of container log output if the termination
	// message file is empty and the container exited with an error.
	// The log output is limited to 2048 bytes or 80 lines, whichever is smaller.
	// Defaults to File.
	// Cannot be updated.
	// +optional
	DeprecatedTerminationMessagePolicy corev1.TerminationMessagePolicy `json:"terminationMessagePolicy,omitempty" protobuf:"bytes,20,opt,name=terminationMessagePolicy,casttype=TerminationMessagePolicy"`
	// Image pull policy.
	// One of Always, Never, IfNotPresent.
	// Defaults to Always if :latest tag is specified, or IfNotPresent otherwise.
	// Cannot be updated.
	// More info: https://kubernetes.io/docs/concepts/containers/images#updating-images
	// +optional
	ImagePullPolicy corev1.PullPolicy `json:"imagePullPolicy,omitempty" protobuf:"bytes,14,opt,name=imagePullPolicy,casttype=PullPolicy"`
	// SecurityContext defines the security options the container should be run with.
	// If set, the fields of SecurityContext override the equivalent fields of PodSecurityContext.
	// More info: https://kubernetes.io/docs/tasks/configure-pod-container/security-context/
	// +optional
	SecurityContext *corev1.SecurityContext `json:"securityContext,omitempty" protobuf:"bytes,15,opt,name=securityContext"`

	// Variables for interactive containers, these have very specialized use-cases (e.g. debugging)
	// and shouldn't be used for general purpose containers.

	// Deprecated. This field will be removed in a future release.
	// Whether this container should allocate a buffer for stdin in the container runtime. If this
	// is not set, reads from stdin in the container will always result in EOF.
	// Default is false.
	// +optional
	DeprecatedStdin bool `json:"stdin,omitempty" protobuf:"varint,16,opt,name=stdin"`
	// Deprecated. This field will be removed in a future release.
	// Whether the container runtime should close the stdin channel after it has been opened by
	// a single attach. When stdin is true the stdin stream will remain open across multiple attach
	// sessions. If stdinOnce is set to true, stdin is opened on container start, is empty until the
	// first client attaches to stdin, and then remains open and accepts data until the client disconnects,
	// at which time stdin is closed and remains closed until the container is restarted. If this
	// flag is false, a container processes that reads from stdin will never receive an EOF.
	// Default is false
	// +optional
	DeprecatedStdinOnce bool `json:"stdinOnce,omitempty" protobuf:"varint,17,opt,name=stdinOnce"`
	// Deprecated. This field will be removed in a future release.
	// Whether this container should allocate a DeprecatedTTY for itself, also requires 'stdin' to be true.
	// Default is false.
	// +optional
	DeprecatedTTY bool `json:"tty,omitempty" protobuf:"varint,18,opt,name=tty"`
}

// SetContainerFields sets the fields of the Step to the values of the corresponding fields in the Container
func (s *StepTemplate) SetContainerFields(c corev1.Container) {
	s.DeprecatedName = c.Name
	s.Image = c.Image
	s.Command = c.Command
	s.Args = c.Args
	s.WorkingDir = c.WorkingDir
	s.DeprecatedPorts = c.Ports
	s.EnvFrom = c.EnvFrom
	s.Env = c.Env
	s.Resources = c.Resources
	s.VolumeMounts = c.VolumeMounts
	s.VolumeDevices = c.VolumeDevices
	s.DeprecatedLivenessProbe = c.LivenessProbe
	s.DeprecatedReadinessProbe = c.ReadinessProbe
	s.DeprecatedStartupProbe = c.StartupProbe
	s.DeprecatedLifecycle = c.Lifecycle
	s.DeprecatedTerminationMessagePath = c.TerminationMessagePath
	s.ImagePullPolicy = c.ImagePullPolicy
	s.SecurityContext = c.SecurityContext
	s.DeprecatedStdin = c.Stdin
	s.DeprecatedStdinOnce = c.StdinOnce
	s.DeprecatedTTY = c.TTY
}

// ToK8sContainer converts the StepTemplate to a Kubernetes Container struct
func (s *StepTemplate) ToK8sContainer() *corev1.Container {
	return &corev1.Container{
		Name:                   s.DeprecatedName,
		Image:                  s.Image,
		Command:                s.Command,
		Args:                   s.Args,
		WorkingDir:             s.WorkingDir,
		Ports:                  s.DeprecatedPorts,
		EnvFrom:                s.EnvFrom,
		Env:                    s.Env,
		Resources:              s.Resources,
		VolumeMounts:           s.VolumeMounts,
		VolumeDevices:          s.VolumeDevices,
		LivenessProbe:          s.DeprecatedLivenessProbe,
		ReadinessProbe:         s.DeprecatedReadinessProbe,
		StartupProbe:           s.DeprecatedStartupProbe,
		Lifecycle:              s.DeprecatedLifecycle,
		TerminationMessagePath: s.DeprecatedTerminationMessagePath,
		ImagePullPolicy:        s.ImagePullPolicy,
		SecurityContext:        s.SecurityContext,
		Stdin:                  s.DeprecatedStdin,
		StdinOnce:              s.DeprecatedStdinOnce,
		TTY:                    s.DeprecatedTTY,
	}
}

// Sidecar has nearly the same data structure as Step but does not have the ability to timeout.
type Sidecar struct {

	// Name of the container specified as a DNS_LABEL.
	// Each container in a pod must have a unique name (DNS_LABEL).
	// Cannot be updated.
	Name string `json:"name" protobuf:"bytes,1,opt,name=name"`
	// Docker image name.
	// More info: https://kubernetes.io/docs/concepts/containers/images
	// This field is optional to allow higher level config management to default or override
	// container images in workload controllers like Deployments and StatefulSets.
	// +optional
	Image string `json:"image,omitempty" protobuf:"bytes,2,opt,name=image"`
	// Entrypoint array. Not executed within a shell.
	// The docker image's ENTRYPOINT is used if this is not provided.
	// Variable references $(VAR_NAME) are expanded using the container's environment. If a variable
	// cannot be resolved, the reference in the input string will be unchanged. Double $$ are reduced
	// to a single $, which allows for escaping the $(VAR_NAME) syntax: i.e. "$$(VAR_NAME)" will
	// produce the string literal "$(VAR_NAME)". Escaped references will never be expanded, regardless
	// of whether the variable exists or not. Cannot be updated.
	// More info: https://kubernetes.io/docs/tasks/inject-data-application/define-command-argument-container/#running-a-command-in-a-shell
	// +optional
	// +listType=atomic
	Command []string `json:"command,omitempty" protobuf:"bytes,3,rep,name=command"`
	// Arguments to the entrypoint.
	// The docker image's CMD is used if this is not provided.
	// Variable references $(VAR_NAME) are expanded using the container's environment. If a variable
	// cannot be resolved, the reference in the input string will be unchanged. Double $$ are reduced
	// to a single $, which allows for escaping the $(VAR_NAME) syntax: i.e. "$$(VAR_NAME)" will
	// produce the string literal "$(VAR_NAME)". Escaped references will never be expanded, regardless
	// of whether the variable exists or not. Cannot be updated.
	// More info: https://kubernetes.io/docs/tasks/inject-data-application/define-command-argument-container/#running-a-command-in-a-shell
	// +optional
	// +listType=atomic
	Args []string `json:"args,omitempty" protobuf:"bytes,4,rep,name=args"`
	// Container's working directory.
	// If not specified, the container runtime's default will be used, which
	// might be configured in the container image.
	// Cannot be updated.
	// +optional
	WorkingDir string `json:"workingDir,omitempty" protobuf:"bytes,5,opt,name=workingDir"`
	// List of ports to expose from the container. Exposing a port here gives
	// the system additional information about the network connections a
	// container uses, but is primarily informational. Not specifying a port here
	// DOES NOT prevent that port from being exposed. Any port which is
	// listening on the default "0.0.0.0" address inside a container will be
	// accessible from the network.
	// Cannot be updated.
	// +optional
	// +patchMergeKey=containerPort
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=containerPort
	// +listMapKey=protocol
	Ports []corev1.ContainerPort `json:"ports,omitempty" patchStrategy:"merge" patchMergeKey:"containerPort" protobuf:"bytes,6,rep,name=ports"`
	// List of sources to populate environment variables in the container.
	// The keys defined within a source must be a C_IDENTIFIER. All invalid keys
	// will be reported as an event when the container is starting. When a key exists in multiple
	// sources, the value associated with the last source will take precedence.
	// Values defined by an Env with a duplicate key will take precedence.
	// Cannot be updated.
	// +optional
	// +listType=atomic
	EnvFrom []corev1.EnvFromSource `json:"envFrom,omitempty" protobuf:"bytes,19,rep,name=envFrom"`
	// List of environment variables to set in the container.
	// Cannot be updated.
	// +optional
	// +patchMergeKey=name
	// +patchStrategy=merge
	// +listType=atomic
	Env []corev1.EnvVar `json:"env,omitempty" patchStrategy:"merge" patchMergeKey:"name" protobuf:"bytes,7,rep,name=env"`
	// Compute Resources required by this container.
	// Cannot be updated.
	// More info: https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/
	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty" protobuf:"bytes,8,opt,name=resources"`
	// Pod volumes to mount into the container's filesystem.
	// Cannot be updated.
	// +optional
	// +patchMergeKey=mountPath
	// +patchStrategy=merge
	// +listType=atomic
	VolumeMounts []corev1.VolumeMount `json:"volumeMounts,omitempty" patchStrategy:"merge" patchMergeKey:"mountPath" protobuf:"bytes,9,rep,name=volumeMounts"`
	// volumeDevices is the list of block devices to be used by the container.
	// +patchMergeKey=devicePath
	// +patchStrategy=merge
	// +optional
	// +listType=atomic
	VolumeDevices []corev1.VolumeDevice `json:"volumeDevices,omitempty" patchStrategy:"merge" patchMergeKey:"devicePath" protobuf:"bytes,21,rep,name=volumeDevices"`
	// Periodic probe of container liveness.
	// Container will be restarted if the probe fails.
	// Cannot be updated.
	// More info: https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle#container-probes
	// +optional
	LivenessProbe *corev1.Probe `json:"livenessProbe,omitempty" protobuf:"bytes,10,opt,name=livenessProbe"`
	// Periodic probe of container service readiness.
	// Container will be removed from service endpoints if the probe fails.
	// Cannot be updated.
	// More info: https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle#container-probes
	// +optional
	ReadinessProbe *corev1.Probe `json:"readinessProbe,omitempty" protobuf:"bytes,11,opt,name=readinessProbe"`
	// StartupProbe indicates that the Pod has successfully initialized.
	// If specified, no other probes are executed until this completes successfully.
	// If this probe fails, the Pod will be restarted, just as if the livenessProbe failed.
	// This can be used to provide different probe parameters at the beginning of a Pod's lifecycle,
	// when it might take a long time to load data or warm a cache, than during steady-state operation.
	// This cannot be updated.
	// More info: https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle#container-probes
	// +optional
	StartupProbe *corev1.Probe `json:"startupProbe,omitempty" protobuf:"bytes,22,opt,name=startupProbe"`
	// Actions that the management system should take in response to container lifecycle events.
	// Cannot be updated.
	// +optional
	Lifecycle *corev1.Lifecycle `json:"lifecycle,omitempty" protobuf:"bytes,12,opt,name=lifecycle"`
	// Optional: Path at which the file to which the container's termination message
	// will be written is mounted into the container's filesystem.
	// Message written is intended to be brief final status, such as an assertion failure message.
	// Will be truncated by the node if greater than 4096 bytes. The total message length across
	// all containers will be limited to 12kb.
	// Defaults to /dev/termination-log.
	// Cannot be updated.
	// +optional
	TerminationMessagePath string `json:"terminationMessagePath,omitempty" protobuf:"bytes,13,opt,name=terminationMessagePath"`
	// Indicate how the termination message should be populated. File will use the contents of
	// terminationMessagePath to populate the container status message on both success and failure.
	// FallbackToLogsOnError will use the last chunk of container log output if the termination
	// message file is empty and the container exited with an error.
	// The log output is limited to 2048 bytes or 80 lines, whichever is smaller.
	// Defaults to File.
	// Cannot be updated.
	// +optional
	TerminationMessagePolicy corev1.TerminationMessagePolicy `json:"terminationMessagePolicy,omitempty" protobuf:"bytes,20,opt,name=terminationMessagePolicy,casttype=TerminationMessagePolicy"`
	// Image pull policy.
	// One of Always, Never, IfNotPresent.
	// Defaults to Always if :latest tag is specified, or IfNotPresent otherwise.
	// Cannot be updated.
	// More info: https://kubernetes.io/docs/concepts/containers/images#updating-images
	// +optional
	ImagePullPolicy corev1.PullPolicy `json:"imagePullPolicy,omitempty" protobuf:"bytes,14,opt,name=imagePullPolicy,casttype=PullPolicy"`
	// SecurityContext defines the security options the container should be run with.
	// If set, the fields of SecurityContext override the equivalent fields of PodSecurityContext.
	// More info: https://kubernetes.io/docs/tasks/configure-pod-container/security-context/
	// +optional
	SecurityContext *corev1.SecurityContext `json:"securityContext,omitempty" protobuf:"bytes,15,opt,name=securityContext"`

	// Variables for interactive containers, these have very specialized use-cases (e.g. debugging)
	// and shouldn't be used for general purpose containers.

	// Whether this container should allocate a buffer for stdin in the container runtime. If this
	// is not set, reads from stdin in the container will always result in EOF.
	// Default is false.
	// +optional
	Stdin bool `json:"stdin,omitempty" protobuf:"varint,16,opt,name=stdin"`
	// Whether the container runtime should close the stdin channel after it has been opened by
	// a single attach. When stdin is true the stdin stream will remain open across multiple attach
	// sessions. If stdinOnce is set to true, stdin is opened on container start, is empty until the
	// first client attaches to stdin, and then remains open and accepts data until the client disconnects,
	// at which time stdin is closed and remains closed until the container is restarted. If this
	// flag is false, a container processes that reads from stdin will never receive an EOF.
	// Default is false
	// +optional
	StdinOnce bool `json:"stdinOnce,omitempty" protobuf:"varint,17,opt,name=stdinOnce"`
	// Whether this container should allocate a TTY for itself, also requires 'stdin' to be true.
	// Default is false.
	// +optional
	TTY bool `json:"tty,omitempty" protobuf:"varint,18,opt,name=tty"`

	// Script is the contents of an executable file to execute.
	//
	// If Script is not empty, the Step cannot have an Command or Args.
	// +optional
	Script string `json:"script,omitempty"`

	// This is an alpha field. You must set the "enable-api-fields" feature flag to "alpha"
	// for this field to be supported.
	//
	// Workspaces is a list of workspaces from the Task that this Sidecar wants
	// exclusive access to. Adding a workspace to this list means that any
	// other Step or Sidecar that does not also request this Workspace will
	// not have access to it.
	// +optional
	// +listType=atomic
	Workspaces []WorkspaceUsage `json:"workspaces,omitempty"`
}

// ToK8sContainer converts the Sidecar to a Kubernetes Container struct
func (s *Sidecar) ToK8sContainer() *corev1.Container {
	return &corev1.Container{
		Name:                   s.Name,
		Image:                  s.Image,
		Command:                s.Command,
		Args:                   s.Args,
		WorkingDir:             s.WorkingDir,
		Ports:                  s.Ports,
		EnvFrom:                s.EnvFrom,
		Env:                    s.Env,
		Resources:              s.Resources,
		VolumeMounts:           s.VolumeMounts,
		VolumeDevices:          s.VolumeDevices,
		LivenessProbe:          s.LivenessProbe,
		ReadinessProbe:         s.ReadinessProbe,
		StartupProbe:           s.StartupProbe,
		Lifecycle:              s.Lifecycle,
		TerminationMessagePath: s.TerminationMessagePath,
		ImagePullPolicy:        s.ImagePullPolicy,
		SecurityContext:        s.SecurityContext,
		Stdin:                  s.Stdin,
		StdinOnce:              s.StdinOnce,
		TTY:                    s.TTY,
	}
}

// SetContainerFields sets the fields of the Sidecar to the values of the corresponding fields in the Container
func (s *Sidecar) SetContainerFields(c corev1.Container) {
	s.Name = c.Name
	s.Image = c.Image
	s.Command = c.Command
	s.Args = c.Args
	s.WorkingDir = c.WorkingDir
	s.Ports = c.Ports
	s.EnvFrom = c.EnvFrom
	s.Env = c.Env
	s.Resources = c.Resources
	s.VolumeMounts = c.VolumeMounts
	s.VolumeDevices = c.VolumeDevices
	s.LivenessProbe = c.LivenessProbe
	s.ReadinessProbe = c.ReadinessProbe
	s.StartupProbe = c.StartupProbe
	s.Lifecycle = c.Lifecycle
	s.TerminationMessagePath = c.TerminationMessagePath
	s.ImagePullPolicy = c.ImagePullPolicy
	s.SecurityContext = c.SecurityContext
	s.Stdin = c.Stdin
	s.StdinOnce = c.StdinOnce
	s.TTY = c.TTY
}
