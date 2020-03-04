/*
Copyright 2019-2020 The Tekton Authors

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
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
)

// +genclient
// +genclient:noStatus
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Condition declares a step that is used to gate the execution of a Task in a Pipeline.
// A condition execution (ConditionCheck) evaluates to either true or false
// +k8s:openapi-gen=true
type Condition struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata"`

	// Spec holds the desired state of the Condition from the client
	// +optional
	Spec ConditionSpec `json:"spec"`
}

// ConditionCheckStatus defines the observed state of ConditionCheck
type ConditionCheckStatus = v1beta1.ConditionCheckStatus

// ConditionCheckStatusFields holds the fields of ConfigurationCheck's status.
// This is defined separately and inlined so that other types can readily
// consume these fields via duck typing.
type ConditionCheckStatusFields = v1beta1.ConditionCheckStatusFields

// ConditionSpec defines the desired state of the Condition
type ConditionSpec struct {
	// Check declares container whose exit code determines where a condition is true or false
	Check Step `json:"check,omitempty"`

	// Description is a user-facing description of the condition that may be
	// used to populate a UI.
	// +optional
	Description string `json:"description,omitempty"`

	// Params is an optional set of parameters which must be supplied by the user when a Condition
	// is evaluated
	// +optional
	Params []ParamSpec `json:"params,omitempty"`

	// Resources is a list of the ConditionResources required to run the condition.
	// +optional
	Resources []ResourceDeclaration `json:"resources,omitempty"`
}

// ConditionCheck represents a single evaluation of a Condition step.
type ConditionCheck TaskRun

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ConditionList contains a list of Conditions
type ConditionList struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Condition `json:"items"`
}

func NewConditionCheck(tr *TaskRun) *ConditionCheck {
	if tr == nil {
		return nil
	}

	cc := ConditionCheck(*tr)
	return &cc
}

// IsDone returns true if the ConditionCheck's status indicates that it is done.
func (cc *ConditionCheck) IsDone() bool {
	return !cc.Status.GetCondition(apis.ConditionSucceeded).IsUnknown()
}

// IsSuccessful returns true if the ConditionCheck's status indicates that it is done.
func (cc *ConditionCheck) IsSuccessful() bool {
	return cc.Status.GetCondition(apis.ConditionSucceeded).IsTrue()
}
