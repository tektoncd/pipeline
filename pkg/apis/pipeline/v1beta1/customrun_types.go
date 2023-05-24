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

package v1beta1

import (
	"fmt"
	"time"

	apisconfig "github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	runv1beta1 "github.com/tektoncd/pipeline/pkg/apis/run/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/clock"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

// EmbeddedCustomRunSpec allows custom task definitions to be embedded
type EmbeddedCustomRunSpec struct {
	runtime.TypeMeta `json:",inline"`

	// +optional
	Metadata PipelineTaskMetadata `json:"metadata,omitempty"`

	// Spec is a specification of a custom task
	// +optional
	Spec runtime.RawExtension `json:"spec,omitempty"`
}

// CustomRunSpec defines the desired state of CustomRun
type CustomRunSpec struct {
	// +optional
	CustomRef *TaskRef `json:"customRef,omitempty"`

	// Spec is a specification of a custom task
	// +optional
	CustomSpec *EmbeddedCustomRunSpec `json:"customSpec,omitempty"`

	// +optional
	// +listType=atomic
	Params Params `json:"params,omitempty"`

	// Used for cancelling a customrun (and maybe more later on)
	// +optional
	Status CustomRunSpecStatus `json:"status,omitempty"`

	// Status message for cancellation.
	// +optional
	StatusMessage CustomRunSpecStatusMessage `json:"statusMessage,omitempty"`

	// Used for propagating retries count to custom tasks
	// +optional
	Retries int `json:"retries,omitempty"`

	// +optional
	ServiceAccountName string `json:"serviceAccountName"`

	// Time after which the custom-task times out.
	// Refer Go's ParseDuration documentation for expected format: https://golang.org/pkg/time/#ParseDuration
	// +optional
	Timeout *metav1.Duration `json:"timeout,omitempty"`

	// Workspaces is a list of WorkspaceBindings from volumes to workspaces.
	// +optional
	// +listType=atomic
	Workspaces []WorkspaceBinding `json:"workspaces,omitempty"`
}

// CustomRunSpecStatus defines the taskrun spec status the user can provide
type CustomRunSpecStatus string

const (
	// CustomRunSpecStatusCancelled indicates that the user wants to cancel the run,
	// if not already cancelled or terminated
	CustomRunSpecStatusCancelled CustomRunSpecStatus = "RunCancelled"
)

// CustomRunSpecStatusMessage defines human readable status messages for the TaskRun.
type CustomRunSpecStatusMessage string

const (
	// CustomRunCancelledByPipelineMsg indicates that the PipelineRun of which part this CustomRun was
	// has been cancelled.
	CustomRunCancelledByPipelineMsg CustomRunSpecStatusMessage = "CustomRun cancelled as the PipelineRun it belongs to has been cancelled."
	// CustomRunCancelledByPipelineTimeoutMsg indicates that the Run was cancelled because the PipelineRun running it timed out.
	CustomRunCancelledByPipelineTimeoutMsg CustomRunSpecStatusMessage = "CustomRun cancelled as the PipelineRun it belongs to has timed out."
)

// GetParam gets the Param from the CustomRunSpec with the given name
// TODO(jasonhall): Move this to a Params type so other code can use it?
func (rs CustomRunSpec) GetParam(name string) *Param {
	for _, p := range rs.Params {
		if p.Name == name {
			return &p
		}
	}
	return nil
}

// CustomRunReason is an enum used to store all Run reason for the Succeeded condition that are controlled by the CustomRun itself.
type CustomRunReason string

const (
	// CustomRunReasonStarted is the reason set when the CustomRun has just started.
	CustomRunReasonStarted CustomRunReason = "Started"
	// CustomRunReasonRunning is the reason set when the CustomRun is running.
	CustomRunReasonRunning CustomRunReason = "Running"
	// CustomRunReasonSuccessful is the reason set when the CustomRun completed successfully.
	CustomRunReasonSuccessful CustomRunReason = "Succeeded"
	// CustomRunReasonFailed is the reason set when the CustomRun completed with a failure.
	CustomRunReasonFailed CustomRunReason = "Failed"
	// CustomRunReasonCancelled must be used in the Condition Reason to indicate that a CustomRun was cancelled.
	CustomRunReasonCancelled CustomRunReason = "CustomRunCancelled"
	// CustomRunReasonTimedOut must be used in the Condition Reason to indicate that a CustomRun was timed out.
	CustomRunReasonTimedOut CustomRunReason = "CustomRunTimedOut"
	// CustomRunReasonWorkspaceNotSupported can be used in the Condition Reason to indicate that the
	// CustomRun contains a workspace which is not supported by this custom task.
	CustomRunReasonWorkspaceNotSupported CustomRunReason = "CustomRunWorkspaceNotSupported"
)

func (t CustomRunReason) String() string {
	return string(t)
}

// CustomRunStatus defines the observed state of CustomRun.
type CustomRunStatus = runv1beta1.CustomRunStatus

var customrunCondSet = apis.NewBatchConditionSet()

// GetConditionSet retrieves the condition set for this resource. Implements
// the KRShaped interface.
func (r *CustomRun) GetConditionSet() apis.ConditionSet { return customrunCondSet }

// GetStatus retrieves the status of the Parallel. Implements the KRShaped
// interface.
func (r *CustomRun) GetStatus() *duckv1.Status { return &r.Status.Status }

// CustomRunStatusFields holds the fields of CustomRun's status.  This is defined
// separately and inlined so that other types can readily consume these fields
// via duck typing.
type CustomRunStatusFields = runv1beta1.CustomRunStatusFields

// CustomRunResult used to describe the results of a task
type CustomRunResult = runv1beta1.CustomRunResult

// +genclient
// +genreconciler
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// CustomRun represents a single execution of a Custom Task.
//
// +k8s:openapi-gen=true
type CustomRun struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +optional
	Spec CustomRunSpec `json:"spec,omitempty"`
	// +optional
	Status CustomRunStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// CustomRunList contains a list of CustomRun
type CustomRunList struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CustomRun `json:"items"`
}

// GetStatusCondition returns the task run status as a ConditionAccessor
func (r *CustomRun) GetStatusCondition() apis.ConditionAccessor {
	return &r.Status
}

// GetGroupVersionKind implements kmeta.OwnerRefable.
func (*CustomRun) GetGroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind(pipeline.CustomRunControllerName)
}

// HasPipelineRunOwnerReference returns true of CustomRun has
// owner reference of type PipelineRun
func (r *CustomRun) HasPipelineRunOwnerReference() bool {
	for _, ref := range r.GetOwnerReferences() {
		if ref.Kind == pipeline.PipelineRunControllerName {
			return true
		}
	}
	return false
}

// IsCancelled returns true if the CustomRun's spec status is set to Cancelled state
func (r *CustomRun) IsCancelled() bool {
	return r.Spec.Status == CustomRunSpecStatusCancelled
}

// IsDone returns true if the CustomRun's status indicates that it is done.
func (r *CustomRun) IsDone() bool {
	return !r.Status.GetCondition(apis.ConditionSucceeded).IsUnknown()
}

// HasStarted function check whether taskrun has valid start time set in its status
func (r *CustomRun) HasStarted() bool {
	return r.Status.StartTime != nil && !r.Status.StartTime.IsZero()
}

// IsSuccessful returns true if the CustomRun's status indicates that it has succeeded.
func (r *CustomRun) IsSuccessful() bool {
	return r != nil && r.Status.GetCondition(apis.ConditionSucceeded).IsTrue()
}

// IsFailure returns true if the CustomRun's status indicates that it has failed.
func (r *CustomRun) IsFailure() bool {
	return r != nil && r.Status.GetCondition(apis.ConditionSucceeded).IsFalse()
}

// GetCustomRunKey return the customrun's key for timeout handler map
func (r *CustomRun) GetCustomRunKey() string {
	// The address of the pointer is a threadsafe unique identifier for the customrun
	return fmt.Sprintf("%s/%p", "CustomRun", r)
}

// HasTimedOut returns true if the CustomRun's running time is beyond the allowed timeout
func (r *CustomRun) HasTimedOut(c clock.PassiveClock) bool {
	if r.Status.StartTime == nil || r.Status.StartTime.IsZero() {
		return false
	}
	timeout := r.GetTimeout()
	// If timeout is set to 0 or defaulted to 0, there is no timeout.
	if timeout == apisconfig.NoTimeoutDuration {
		return false
	}
	runtime := c.Since(r.Status.StartTime.Time)
	return runtime > timeout
}

// GetTimeout returns the timeout for this customrun, or the default if not configured
func (r *CustomRun) GetTimeout() time.Duration {
	// Use the platform default if no timeout is set
	if r.Spec.Timeout == nil {
		return apisconfig.DefaultTimeoutMinutes * time.Minute
	}
	return r.Spec.Timeout.Duration
}

// GetRetryCount returns the number of times this CustomRun has already been retried
func (r *CustomRun) GetRetryCount() int {
	return len(r.Status.RetriesStatus)
}
