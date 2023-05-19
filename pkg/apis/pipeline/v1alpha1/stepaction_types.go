/*
Copyright 2023 The Tekton Authors
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

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/kmeta"
)

// +genclient
// +genclient:noStatus
// +genreconciler:krshapedlogic=false
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// StepAction represents a collection of sequential steps that are run as part of a
// Pipeline using a set of inputs and producing a set of outputs. Steps execute
// when StepRuns are created that provide the input parameters and resources and
// output resources the Step requires.
//
// +k8s:openapi-gen=true
type StepAction struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata"`

	// Spec holds the desired state of the Step from the client
	// +optional
	Spec StepActionSpec `json:"spec"`
}

var _ kmeta.OwnerRefable = (*StepAction)(nil)

// StepAction returns the task's spec
func (s *StepAction) StepActionSpec() StepActionSpec {
	return s.Spec
}

// StepActionMetadata returns the task's ObjectMeta
func (s *StepAction) StepActionMetadata() metav1.ObjectMeta {
	return s.ObjectMeta
}

// Copy returns a deep copy of the task
func (s *StepAction) Copy() StepActionObject {
	return s.DeepCopy()
}

// GetGroupVersionKind implements kmeta.OwnerRefable.
func (*StepAction) GetGroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind("StepAction")
}

// StepList contains a list of Step
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type StepActionList struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []StepAction `json:"items"`
}

// StepActionSpec is the actionable components of a step.
type StepActionSpec struct {
	// Name of the Step specified as a DNS_LABEL.
	// Each Step in a Task must have a unique name.
	Name string `json:"name" protobuf:"bytes,1,opt,name=name"`
	// Image reference name to run for this Step.
	// More info: https://kubernetes.io/docs/concepts/containers/images
	// +optional
	Image string `json:"image,omitempty" protobuf:"bytes,2,opt,name=image"`
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
	Command []string `json:"command,omitempty" protobuf:"bytes,3,rep,name=command"`
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
	Args []string `json:"args,omitempty" protobuf:"bytes,4,rep,name=args"`
	// Step's working directory.
	// If not specified, the container runtime's default will be used, which
	// might be configured in the container image.
	// Cannot be updated.
	// +optional
	WorkingDir string `json:"workingDir,omitempty" protobuf:"bytes,5,opt,name=workingDir"`
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
	// Script is the contents of an executable file to execute.
	//
	// If Script is not empty, the Step cannot have an Command and the Args will be passed to the Script.
	// +optional
	Script string `json:"script,omitempty"`
	// Params is a list of input parameters required to run the stepAction. Params
	// must be supplied as inputs in Tasks unless they declare a default
	// value.
	// +optional
	// +listType=atomic
	Params ParamSpecs
	// Results are values that this StepAction can output
	// +listType=atomic
	Results []StepActionResult
}

// MyStepObject is implemented by Task and ClusterTask
type StepActionObject interface {
	apis.Defaultable
	StepActionMetadata() metav1.ObjectMeta
	StepActionSpec() StepActionSpec
	Copy() StepActionObject
}
