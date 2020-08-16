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

package v1alpha1

import (
	"fmt"
	"time"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	v1beta1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

var (
	runGroupVersionKind = schema.GroupVersionKind{
		Group:   SchemeGroupVersion.Group,
		Version: SchemeGroupVersion.Version,
		Kind:    pipeline.RunControllerName,
	}
)

// RunSpec defines the desired state of Run
type RunSpec struct {
	// +optional
	Ref *TaskRef `json:"ref,omitempty"`

	// +optional
	Params []v1beta1.Param `json:"params,omitempty"`

	// TODO(https://github.com/tektoncd/community/pull/128)
	// - cancellation
	// - timeout
	// - inline task spec
	// - workspaces ?
}

// TODO(jasonhall): Move this to a Params type so other code can use it?
func (rs RunSpec) GetParam(name string) *v1beta1.Param {
	for _, p := range rs.Params {
		if p.Name == name {
			return &p
		}
	}
	return nil
}

type RunStatus struct {
	duckv1.Status `json:",inline"`

	// RunStatusFields inlines the status fields.
	RunStatusFields `json:",inline"`
}

var runCondSet = apis.NewBatchConditionSet()

// GetCondition returns the Condition matching the given type.
func (r *RunStatus) GetCondition(t apis.ConditionType) *apis.Condition {
	return runCondSet.Manage(r).GetCondition(t)
}

// InitializeConditions will set all conditions in runCondSet to unknown for the PipelineRun
// and set the started time to the current time
func (r *RunStatus) InitializeConditions() {
	started := false
	if r.StartTime.IsZero() {
		r.StartTime = &metav1.Time{Time: time.Now()}
		started = true
	}
	conditionManager := runCondSet.Manage(r)
	conditionManager.InitializeConditions()
	// Ensure the started reason is set for the "Succeeded" condition
	if started {
		initialCondition := conditionManager.GetCondition(apis.ConditionSucceeded)
		initialCondition.Reason = "Started"
		conditionManager.SetCondition(*initialCondition)
	}
}

// SetCondition sets the condition, unsetting previous conditions with the same
// type as necessary.
func (r *RunStatus) SetCondition(newCond *apis.Condition) {
	if newCond != nil {
		runCondSet.Manage(r).SetCondition(*newCond)
	}
}

// GetConditionSet retrieves the condition set for this resource. Implements
// the KRShaped interface.
func (r *Run) GetConditionSet() apis.ConditionSet { return runCondSet }

// GetStatus retrieves the status of the Parallel. Implements the KRShaped
// interface.
func (r *Run) GetStatus() *duckv1.Status { return &r.Status.Status }

// RunStatusFields holds the fields of Run's status.  This is defined
// separately and inlined so that other types can readily consume these fields
// via duck typing.
type RunStatusFields struct {
	// StartTime is the time the build is actually started.
	// +optional
	StartTime *metav1.Time `json:"startTime,omitempty"`

	// CompletionTime is the time the build completed.
	// +optional
	CompletionTime *metav1.Time `json:"completionTime,omitempty"`

	// Results reports any output result values to be consumed by later
	// tasks in a pipeline.
	// +optional
	Results []v1beta1.TaskRunResult `json:"results,omitempty"`

	// ExtraFields holds arbitrary fields provided by the custom task
	// controller.
	ExtraFields runtime.RawExtension `json:"extraFields,omitempty"`
}

// +genclient
// +genreconciler
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Run represents a single execution of a Custom Task.
//
// +k8s:openapi-gen=true
type Run struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +optional
	Spec RunSpec `json:"spec,omitempty"`
	// +optional
	Status RunStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RunList contains a list of Run
type RunList struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Run `json:"items"`
}

// GetOwnerReference gets the task run as owner reference for any related objects
func (r *Run) GetOwnerReference() metav1.OwnerReference {
	return *metav1.NewControllerRef(r, runGroupVersionKind)
}

// HasPipelineRunOwnerReference returns true of Run has
// owner reference of type PipelineRun
func (r *Run) HasPipelineRunOwnerReference() bool {
	for _, ref := range r.GetOwnerReferences() {
		if ref.Kind == pipeline.PipelineRunControllerName {
			return true
		}
	}
	return false
}

// IsDone returns true if the Run's status indicates that it is done.
func (r *Run) IsDone() bool {
	return !r.Status.GetCondition(apis.ConditionSucceeded).IsUnknown()
}

// HasStarted function check whether taskrun has valid start time set in its status
func (r *Run) HasStarted() bool {
	return r.Status.StartTime != nil && !r.Status.StartTime.IsZero()
}

// IsSuccessful returns true if the Run's status indicates that it is done.
func (r *Run) IsSuccessful() bool {
	return r.Status.GetCondition(apis.ConditionSucceeded).IsTrue()
}

// GetRunKey return the taskrun key for timeout handler map
func (r *Run) GetRunKey() string {
	// The address of the pointer is a threadsafe unique identifier for the taskrun
	return fmt.Sprintf("%s/%p", "Run", r)
}
