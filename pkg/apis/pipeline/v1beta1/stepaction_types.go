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

package v1beta1

import (
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/internal/checksum"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
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

// StepAction represents the actionable components of Step.
// The Step can only reference it from the cluster or using remote resolution.
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

// StepAction returns the step action's spec
func (s *StepAction) StepActionSpec() StepActionSpec {
	return s.Spec
}

// StepActionMetadata returns the step action's ObjectMeta
func (s *StepAction) StepActionMetadata() metav1.ObjectMeta {
	return s.ObjectMeta
}

// Copy returns a deep copy of the stepaction
func (s *StepAction) Copy() StepActionObject {
	return s.DeepCopy()
}

// GetGroupVersionKind implements kmeta.OwnerRefable.
func (*StepAction) GetGroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind("StepAction")
}

// Checksum computes the sha256 checksum of the stepaction object.
// Prior to computing the checksum, it performs some preprocessing on the
// metadata of the object where it removes system provided annotations.
// Only the name, namespace, generateName, user-provided labels and annotations
// and the taskSpec are included for the checksum computation.
func (s *StepAction) Checksum() ([]byte, error) {
	objectMeta := checksum.PrepareObjectMeta(s)
	preprocessedStepaction := StepAction{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "tekton.dev/v1beta1",
			Kind:       "StepAction",
		},
		ObjectMeta: objectMeta,
		Spec:       s.Spec,
	}
	sha256Checksum, err := checksum.ComputeSha256Checksum(preprocessedStepaction)
	if err != nil {
		return nil, err
	}
	return sha256Checksum, nil
}

// StepActionList contains a list of StepActions
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type StepActionList struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []StepAction `json:"items"`
}

// StepActionSpec contains the actionable components of a step.
type StepActionSpec struct {
	// Description is a user-facing description of the stepaction that may be
	// used to populate a UI.
	// +optional
	Description string `json:"description,omitempty"`
	// Image reference name to run for this StepAction.
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
	// List of environment variables to set in the container.
	// Cannot be updated.
	// +optional
	// +patchMergeKey=name
	// +patchStrategy=merge
	// +listType=atomic
	Env []corev1.EnvVar `json:"env,omitempty" patchMergeKey:"name" patchStrategy:"merge" protobuf:"bytes,7,rep,name=env"`
	// Script is the contents of an executable file to execute.
	//
	// If Script is not empty, the Step cannot have an Command and the Args will be passed to the Script.
	// +optional
	Script string `json:"script,omitempty"`
	// Step's working directory.
	// If not specified, the container runtime's default will be used, which
	// might be configured in the container image.
	// Cannot be updated.
	// +optional
	WorkingDir string `json:"workingDir,omitempty" protobuf:"bytes,5,opt,name=workingDir"`
	// Params is a list of input parameters required to run the stepAction.
	// Params must be supplied as inputs in Steps unless they declare a defaultvalue.
	// +optional
	// +listType=atomic
	Params v1.ParamSpecs `json:"params,omitempty"`
	// Results are values that this StepAction can output
	// +optional
	// +listType=atomic
	Results []v1.StepResult `json:"results,omitempty"`
	// SecurityContext defines the security options the Step should be run with.
	// If set, the fields of SecurityContext override the equivalent fields of PodSecurityContext.
	// More info: https://kubernetes.io/docs/tasks/configure-pod-container/security-context/
	// The value set in StepAction will take precedence over the value from Task.
	// +optional
	SecurityContext *corev1.SecurityContext `json:"securityContext,omitempty" protobuf:"bytes,15,opt,name=securityContext"`
	// Volumes to mount into the Step's filesystem.
	// Cannot be updated.
	// +optional
	// +patchMergeKey=mountPath
	// +patchStrategy=merge
	// +listType=atomic
	VolumeMounts []corev1.VolumeMount `json:"volumeMounts,omitempty" patchMergeKey:"mountPath" patchStrategy:"merge" protobuf:"bytes,9,rep,name=volumeMounts"`
}

// ToStep converts the StepActionSpec to a Step struct
func (ss *StepActionSpec) ToStep() *v1.Step {
	return &v1.Step{
		Image:           ss.Image,
		Command:         ss.Command,
		Args:            ss.Args,
		WorkingDir:      ss.WorkingDir,
		Script:          ss.Script,
		Env:             ss.Env,
		VolumeMounts:    ss.VolumeMounts,
		SecurityContext: ss.SecurityContext,
		Results:         ss.Results,
	}
}

// StepActionObject is implemented by StepAction
type StepActionObject interface {
	apis.Defaultable
	StepActionMetadata() metav1.ObjectMeta
	StepActionSpec() StepActionSpec
	Copy() StepActionObject
}
