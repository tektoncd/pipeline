/*
Copyright 2022 The Tekton Authors

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
package pipeline

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Step runs a subcomponent of a Task
type Step struct {
	// Name of the Step specified as a DNS_LABEL.
	// Each Step in a Task must have a unique name.
	Name string
	// Docker image name.
	// More info: https://kubernetes.io/docs/concepts/containers/images
	// +optional
	Image string
	// Entrypoint array. Not executed within a shell.
	// The image's ENTRYPOINT is used if this is not provided.
	// Variable references $(VAR_NAME) are expanded using the container's environment. If a variable
	// cannot be resolved, the reference in the input string will be unchanged. Double $$ are reduced
	// to a single $, which allows for escaping the $(VAR_NAME) syntax: i.e. "$$(VAR_NAME)" will
	// produce the string literal "$(VAR_NAME)". Escaped references will never be expanded, regardless
	// of whether the variable exists or not. Cannot be updated.
	// More info: https://kubernetes.io/docs/tasks/inject-data-application/define-command-argument-container/#running-a-command-in-a-shell
	// +optional
	// +listType=atomic
	Command []string
	// Arguments to the entrypoint.
	// The image's CMD is used if this is not provided.
	// Variable references $(VAR_NAME) are expanded using the container's environment. If a variable
	// cannot be resolved, the reference in the input string will be unchanged. Double $$ are reduced
	// to a single $, which allows for escaping the $(VAR_NAME) syntax: i.e. "$$(VAR_NAME)" will
	// produce the string literal "$(VAR_NAME)". Escaped references will never be expanded, regardless
	// of whether the variable exists or not. Cannot be updated.
	// More info: https://kubernetes.io/docs/tasks/inject-data-application/define-command-argument-container/#running-a-command-in-a-shell
	// +optional
	// +listType=atomic
	Args []string
	// Step's working directory.
	// If not specified, the container runtime's default will be used, which
	// might be configured in the container image.
	// Cannot be updated.
	// +optional
	WorkingDir string
	// List of sources to populate environment variables in the Step.
	// The keys defined within a source must be a C_IDENTIFIER. All invalid keys
	// will be reported as an event when the Step is starting. When a key exists in multiple
	// sources, the value associated with the last source will take precedence.
	// Values defined by an Env with a duplicate key will take precedence.
	// Cannot be updated.
	// +optional
	// +listType=atomic
	EnvFrom []corev1.EnvFromSource
	// List of environment variables to set in the Step.
	// Cannot be updated.
	// +optional
	// +patchMergeKey=name
	// +patchStrategy=merge
	// +listType=atomic
	Env []corev1.EnvVar
	// ComputeResources required by this Step.
	// Cannot be updated.
	// More info: https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/
	// +optional
	ComputeResources corev1.ResourceRequirements
	// Volumes to mount into the Step's filesystem.
	// Cannot be updated.
	// +optional
	// +patchMergeKey=mountPath
	// +patchStrategy=merge
	// +listType=atomic
	VolumeMounts []corev1.VolumeMount
	// volumeDevices is the list of block devices to be used by the Step.
	// +patchMergeKey=devicePath
	// +patchStrategy=merge
	// +optional
	// +listType=atomic
	VolumeDevices []corev1.VolumeDevice
	// Image pull policy.
	// One of Always, Never, IfNotPresent.
	// Defaults to Always if :latest tag is specified, or IfNotPresent otherwise.
	// Cannot be updated.
	// More info: https://kubernetes.io/docs/concepts/containers/images#updating-images
	// +optional
	ImagePullPolicy corev1.PullPolicy
	// SecurityContext defines the security options the Step should be run with.
	// If set, the fields of SecurityContext override the equivalent fields of PodSecurityContext.
	// More info: https://kubernetes.io/docs/tasks/configure-pod-container/security-context/
	// +optional
	SecurityContext *corev1.SecurityContext

	// Script is the contents of an executable file to execute.
	//
	// If Script is not empty, the Step cannot have an Command and the Args will be passed to the Script.
	// +optional
	Script string

	// Timeout is the time after which the step times out. Defaults to never.
	// Refer to Go's ParseDuration documentation for expected format: https://golang.org/pkg/time/#ParseDuration
	// +optional
	Timeout *metav1.Duration

	// This is an alpha field. You must set the "enable-api-fields" feature flag to "alpha"
	// for this field to be supported.
	//
	// Workspaces is a list of workspaces from the Task that this Step wants
	// exclusive access to. Adding a workspace to this list means that any
	// other Step or Sidecar that does not also request this Workspace will
	// not have access to it.
	// +optional
	// +listType=atomic
	Workspaces []WorkspaceUsage

	// OnError defines the exiting behavior of a container on error
	// can be set to [ continue | stopAndFail ]
	OnError OnErrorType
	// Stores configuration for the stdout stream of the step.
	// +optional
	StdoutConfig *StepOutputConfig
	// Stores configuration for the stderr stream of the step.
	// +optional
	StderrConfig *StepOutputConfig
}

// OnErrorType defines a list of supported exiting behavior of a container on error
type OnErrorType string

const (
	// StopAndFail indicates exit the taskRun if the container exits with non-zero exit code
	StopAndFail OnErrorType = "stopAndFail"
	// Continue indicates continue executing the rest of the steps irrespective of the container exit code
	Continue OnErrorType = "continue"
)

// StepOutputConfig stores configuration for a step output stream.
type StepOutputConfig struct {
	// Path to duplicate stdout stream to on container's local filesystem.
	// +optional
	Path string
}

// StepTemplate is a template for a Step
type StepTemplate struct {
	// Image reference name.
	// More info: https://kubernetes.io/docs/concepts/containers/images
	// This field is optional to allow higher level config management to default or override
	// container images in workload controllers like Deployments and StatefulSets.
	// +optional
	Image string
	// Entrypoint array. Not executed within a shell.
	// The image's ENTRYPOINT is used if this is not provided.
	// Variable references $(VAR_NAME) are expanded using the Step's environment. If a variable
	// cannot be resolved, the reference in the input string will be unchanged. Double $$ are reduced
	// to a single $, which allows for escaping the $(VAR_NAME) syntax: i.e. "$$(VAR_NAME)" will
	// produce the string literal "$(VAR_NAME)". Escaped references will never be expanded, regardless
	// of whether the variable exists or not. Cannot be updated.
	// More info: https://kubernetes.io/docs/tasks/inject-data-application/define-command-argument-container/#running-a-command-in-a-shell
	// +optional
	// +listType=atomic
	Command []string
	// Arguments to the entrypoint.
	// The image's CMD is used if this is not provided.
	// Variable references $(VAR_NAME) are expanded using the Step's environment. If a variable
	// cannot be resolved, the reference in the input string will be unchanged. Double $$ are reduced
	// to a single $, which allows for escaping the $(VAR_NAME) syntax: i.e. "$$(VAR_NAME)" will
	// produce the string literal "$(VAR_NAME)". Escaped references will never be expanded, regardless
	// of whether the variable exists or not. Cannot be updated.
	// More info: https://kubernetes.io/docs/tasks/inject-data-application/define-command-argument-container/#running-a-command-in-a-shell
	// +optional
	// +listType=atomic
	Args []string
	// Step's working directory.
	// If not specified, the container runtime's default will be used, which
	// might be configured in the container image.
	// Cannot be updated.
	// +optional
	WorkingDir string
	// List of sources to populate environment variables in the Step.
	// The keys defined within a source must be a C_IDENTIFIER. All invalid keys
	// will be reported as an event when the Step is starting. When a key exists in multiple
	// sources, the value associated with the last source will take precedence.
	// Values defined by an Env with a duplicate key will take precedence.
	// Cannot be updated.
	// +optional
	// +listType=atomic
	EnvFrom []corev1.EnvFromSource
	// List of environment variables to set in the Step.
	// Cannot be updated.
	// +optional
	// +patchMergeKey=name
	// +patchStrategy=merge
	// +listType=atomic
	Env []corev1.EnvVar
	// ComputeResources required by this Step.
	// Cannot be updated.
	// More info: https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/
	// +optional
	ComputeResources corev1.ResourceRequirements
	// Volumes to mount into the Step's filesystem.
	// Cannot be updated.
	// +optional
	// +patchMergeKey=mountPath
	// +patchStrategy=merge
	// +listType=atomic
	VolumeMounts []corev1.VolumeMount
	// volumeDevices is the list of block devices to be used by the Step.
	// +patchMergeKey=devicePath
	// +patchStrategy=merge
	// +optional
	// +listType=atomic
	VolumeDevices []corev1.VolumeDevice
	// Image pull policy.
	// One of Always, Never, IfNotPresent.
	// Defaults to Always if :latest tag is specified, or IfNotPresent otherwise.
	// Cannot be updated.
	// More info: https://kubernetes.io/docs/concepts/containers/images#updating-images
	// +optional
	ImagePullPolicy corev1.PullPolicy
	// SecurityContext defines the security options the Step should be run with.
	// If set, the fields of SecurityContext override the equivalent fields of PodSecurityContext.
	// More info: https://kubernetes.io/docs/tasks/configure-pod-container/security-context/
	// +optional
	SecurityContext *corev1.SecurityContext
}

// Sidecar has nearly the same data structure as Step but does not have the ability to timeout.
type Sidecar struct {
	// Name of the Sidecar specified as a DNS_LABEL.
	// Each Sidecar in a Task must have a unique name (DNS_LABEL).
	// Cannot be updated.
	Name string
	// Image reference name.
	// More info: https://kubernetes.io/docs/concepts/containers/images
	// This field is optional to allow higher level config management to default or override
	// container images in workload controllers like Deployments and StatefulSets.
	// +optional
	Image string
	// Entrypoint array. Not executed within a shell.
	// The image's ENTRYPOINT is used if this is not provided.
	// Variable references $(VAR_NAME) are expanded using the Sidecar's environment. If a variable
	// cannot be resolved, the reference in the input string will be unchanged. Double $$ are reduced
	// to a single $, which allows for escaping the $(VAR_NAME) syntax: i.e. "$$(VAR_NAME)" will
	// produce the string literal "$(VAR_NAME)". Escaped references will never be expanded, regardless
	// of whether the variable exists or not. Cannot be updated.
	// More info: https://kubernetes.io/docs/tasks/inject-data-application/define-command-argument-container/#running-a-command-in-a-shell
	// +optional
	// +listType=atomic
	Command []string
	// Arguments to the entrypoint.
	// The image's CMD is used if this is not provided.
	// Variable references $(VAR_NAME) are expanded using the Sidecar's environment. If a variable
	// cannot be resolved, the reference in the input string will be unchanged. Double $$ are reduced
	// to a single $, which allows for escaping the $(VAR_NAME) syntax: i.e. "$$(VAR_NAME)" will
	// produce the string literal "$(VAR_NAME)". Escaped references will never be expanded, regardless
	// of whether the variable exists or not. Cannot be updated.
	// More info: https://kubernetes.io/docs/tasks/inject-data-application/define-command-argument-container/#running-a-command-in-a-shell
	// +optional
	// +listType=atomic
	Args []string
	// Sidecar's working directory.
	// If not specified, the container runtime's default will be used, which
	// might be configured in the container image.
	// Cannot be updated.
	// +optional
	WorkingDir string
	// List of ports to expose from the Sidecar. Exposing a port here gives
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
	Ports []corev1.ContainerPort
	// List of sources to populate environment variables in the Sidecar.
	// The keys defined within a source must be a C_IDENTIFIER. All invalid keys
	// will be reported as an event when the container is starting. When a key exists in multiple
	// sources, the value associated with the last source will take precedence.
	// Values defined by an Env with a duplicate key will take precedence.
	// Cannot be updated.
	// +optional
	// +listType=atomic
	EnvFrom []corev1.EnvFromSource
	// List of environment variables to set in the Sidecar.
	// Cannot be updated.
	// +optional
	// +patchMergeKey=name
	// +patchStrategy=merge
	// +listType=atomic
	Env []corev1.EnvVar
	// ComputeResources required by this Sidecar.
	// Cannot be updated.
	// More info: https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/
	// +optional
	ComputeResources corev1.ResourceRequirements
	// Volumes to mount into the Sidecar's filesystem.
	// Cannot be updated.
	// +optional
	// +patchMergeKey=mountPath
	// +patchStrategy=merge
	// +listType=atomic
	VolumeMounts []corev1.VolumeMount
	// volumeDevices is the list of block devices to be used by the Sidecar.
	// +patchMergeKey=devicePath
	// +patchStrategy=merge
	// +optional
	// +listType=atomic
	VolumeDevices []corev1.VolumeDevice
	// Periodic probe of Sidecar liveness.
	// Container will be restarted if the probe fails.
	// Cannot be updated.
	// More info: https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle#container-probes
	// +optional
	LivenessProbe *corev1.Probe
	// Periodic probe of Sidecar service readiness.
	// Container will be removed from service endpoints if the probe fails.
	// Cannot be updated.
	// More info: https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle#container-probes
	// +optional
	ReadinessProbe *corev1.Probe
	// StartupProbe indicates that the Pod the Sidecar is running in has successfully initialized.
	// If specified, no other probes are executed until this completes successfully.
	// If this probe fails, the Pod will be restarted, just as if the livenessProbe failed.
	// This can be used to provide different probe parameters at the beginning of a Pod's lifecycle,
	// when it might take a long time to load data or warm a cache, than during steady-state operation.
	// This cannot be updated.
	// More info: https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle#container-probes
	// +optional
	StartupProbe *corev1.Probe
	// Actions that the management system should take in response to Sidecar lifecycle events.
	// Cannot be updated.
	// +optional
	Lifecycle *corev1.Lifecycle
	// Optional: Path at which the file to which the Sidecar's termination message
	// will be written is mounted into the Sidecar's filesystem.
	// Message written is intended to be brief final status, such as an assertion failure message.
	// Will be truncated by the node if greater than 4096 bytes. The total message length across
	// all containers will be limited to 12kb.
	// Defaults to /dev/termination-log.
	// Cannot be updated.
	// +optional
	TerminationMessagePath string
	// Indicate how the termination message should be populated. File will use the contents of
	// terminationMessagePath to populate the Sidecar status message on both success and failure.
	// FallbackToLogsOnError will use the last chunk of Sidecar log output if the termination
	// message file is empty and the Sidecar exited with an error.
	// The log output is limited to 2048 bytes or 80 lines, whichever is smaller.
	// Defaults to File.
	// Cannot be updated.
	// +optional
	TerminationMessagePolicy corev1.TerminationMessagePolicy
	// Image pull policy.
	// One of Always, Never, IfNotPresent.
	// Defaults to Always if :latest tag is specified, or IfNotPresent otherwise.
	// Cannot be updated.
	// More info: https://kubernetes.io/docs/concepts/containers/images#updating-images
	// +optional
	ImagePullPolicy corev1.PullPolicy
	// SecurityContext defines the security options the Sidecar should be run with.
	// If set, the fields of SecurityContext override the equivalent fields of PodSecurityContext.
	// More info: https://kubernetes.io/docs/tasks/configure-pod-container/security-context/
	// +optional
	SecurityContext *corev1.SecurityContext

	// Variables for interactive containers, these have very specialized use-cases (e.g. debugging)
	// and shouldn't be used for general purpose containers.

	// Whether this Sidecar should allocate a buffer for stdin in the container runtime. If this
	// is not set, reads from stdin in the Sidecar will always result in EOF.
	// Default is false.
	// +optional
	Stdin bool
	// Whether the container runtime should close the stdin channel after it has been opened by
	// a single attach. When stdin is true the stdin stream will remain open across multiple attach
	// sessions. If stdinOnce is set to true, stdin is opened on Sidecar start, is empty until the
	// first client attaches to stdin, and then remains open and accepts data until the client disconnects,
	// at which time stdin is closed and remains closed until the Sidecar is restarted. If this
	// flag is false, a container processes that reads from stdin will never receive an EOF.
	// Default is false
	// +optional
	StdinOnce bool
	// Whether this Sidecar should allocate a TTY for itself, also requires 'stdin' to be true.
	// Default is false.
	// +optional
	TTY bool

	// Script is the contents of an executable file to execute.
	//
	// If Script is not empty, the Step cannot have an Command or Args.
	// +optional
	Script string

	// This is an alpha field. You must set the "enable-api-fields" feature flag to "alpha"
	// for this field to be supported.
	//
	// Workspaces is a list of workspaces from the Task that this Sidecar wants
	// exclusive access to. Adding a workspace to this list means that any
	// other Step or Sidecar that does not also request this Workspace will
	// not have access to it.
	// +optional
	// +listType=atomic
	Workspaces []WorkspaceUsage
}
