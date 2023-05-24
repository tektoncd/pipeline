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

package v1

import (
	"context"
	"fmt"
	"time"

	"github.com/tektoncd/pipeline/pkg/apis/config"
	apisconfig "github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	pod "github.com/tektoncd/pipeline/pkg/apis/pipeline/pod"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/clock"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

// TaskRunSpec defines the desired state of TaskRun
type TaskRunSpec struct {
	// +optional
	Debug *TaskRunDebug `json:"debug,omitempty"`
	// +optional
	// +listType=atomic
	Params Params `json:"params,omitempty"`
	// +optional
	ServiceAccountName string `json:"serviceAccountName"`
	// no more than one of the TaskRef and TaskSpec may be specified.
	// +optional
	TaskRef *TaskRef `json:"taskRef,omitempty"`
	// +optional
	TaskSpec *TaskSpec `json:"taskSpec,omitempty"`
	// Used for cancelling a TaskRun (and maybe more later on)
	// +optional
	Status TaskRunSpecStatus `json:"status,omitempty"`
	// Status message for cancellation.
	// +optional
	StatusMessage TaskRunSpecStatusMessage `json:"statusMessage,omitempty"`
	// Retries represents how many times this TaskRun should be retried in the event of task failure.
	// +optional
	Retries int `json:"retries,omitempty"`
	// Time after which one retry attempt times out. Defaults to 1 hour.
	// Refer Go's ParseDuration documentation for expected format: https://golang.org/pkg/time/#ParseDuration
	// +optional
	Timeout *metav1.Duration `json:"timeout,omitempty"`
	// PodTemplate holds pod specific configuration
	PodTemplate *pod.PodTemplate `json:"podTemplate,omitempty"`
	// Workspaces is a list of WorkspaceBindings from volumes to workspaces.
	// +optional
	// +listType=atomic
	Workspaces []WorkspaceBinding `json:"workspaces,omitempty"`
	// Specs to apply to Steps in this TaskRun.
	// If a field is specified in both a Step and a StepSpec,
	// the value from the StepSpec will be used.
	// This field is only supported when the alpha feature gate is enabled.
	// +optional
	// +listType=atomic
	StepSpecs []TaskRunStepSpec `json:"stepSpecs,omitempty"`
	// Specs to apply to Sidecars in this TaskRun.
	// If a field is specified in both a Sidecar and a SidecarSpec,
	// the value from the SidecarSpec will be used.
	// This field is only supported when the alpha feature gate is enabled.
	// +optional
	// +listType=atomic
	SidecarSpecs []TaskRunSidecarSpec `json:"sidecarSpecs,omitempty"`
	// Compute resources to use for this TaskRun
	ComputeResources *corev1.ResourceRequirements `json:"computeResources,omitempty"`
}

// TaskRunSpecStatus defines the TaskRun spec status the user can provide
type TaskRunSpecStatus string

const (
	// TaskRunSpecStatusCancelled indicates that the user wants to cancel the task,
	// if not already cancelled or terminated
	TaskRunSpecStatusCancelled = "TaskRunCancelled"
)

// TaskRunSpecStatusMessage defines human readable status messages for the TaskRun.
type TaskRunSpecStatusMessage string

const (
	// TaskRunCancelledByPipelineMsg indicates that the PipelineRun of which this
	// TaskRun was a part of has been cancelled.
	TaskRunCancelledByPipelineMsg TaskRunSpecStatusMessage = "TaskRun cancelled as the PipelineRun it belongs to has been cancelled."
	// TaskRunCancelledByPipelineTimeoutMsg indicates that the TaskRun was cancelled because the PipelineRun running it timed out.
	TaskRunCancelledByPipelineTimeoutMsg TaskRunSpecStatusMessage = "TaskRun cancelled as the PipelineRun it belongs to has timed out."
)

// TaskRunDebug defines the breakpoint config for a particular TaskRun
type TaskRunDebug struct {
	// +optional
	// +listType=atomic
	Breakpoint []string `json:"breakpoint,omitempty"`
}

// TaskRunInputs holds the input values that this task was invoked with.
type TaskRunInputs struct {
	// +optional
	// +listType=atomic
	Params Params `json:"params,omitempty"`
}

var taskRunCondSet = apis.NewBatchConditionSet()

// TaskRunStatus defines the observed state of TaskRun
type TaskRunStatus struct {
	duckv1.Status `json:",inline"`

	// TaskRunStatusFields inlines the status fields.
	TaskRunStatusFields `json:",inline"`
}

// TaskRunReason is an enum used to store all TaskRun reason for
// the Succeeded condition that are controlled by the TaskRun itself. Failure
// reasons that emerge from underlying resources are not included here
type TaskRunReason string

const (
	// TaskRunReasonStarted is the reason set when the TaskRun has just started
	TaskRunReasonStarted TaskRunReason = "Started"
	// TaskRunReasonRunning is the reason set when the TaskRun is running
	TaskRunReasonRunning TaskRunReason = "Running"
	// TaskRunReasonSuccessful is the reason set when the TaskRun completed successfully
	TaskRunReasonSuccessful TaskRunReason = "Succeeded"
	// TaskRunReasonFailed is the reason set when the TaskRun completed with a failure
	TaskRunReasonFailed TaskRunReason = "Failed"
	// TaskRunReasonToBeRetried is the reason set when the last TaskRun execution failed, and will be retried
	TaskRunReasonToBeRetried TaskRunReason = "ToBeRetried"
	// TaskRunReasonCancelled is the reason set when the TaskRun is cancelled by the user
	TaskRunReasonCancelled TaskRunReason = "TaskRunCancelled"
	// TaskRunReasonTimedOut is the reason set when one TaskRun execution has timed out
	TaskRunReasonTimedOut TaskRunReason = "TaskRunTimeout"
	// TaskRunReasonResolvingTaskRef indicates that the TaskRun is waiting for
	// its taskRef to be asynchronously resolved.
	TaskRunReasonResolvingTaskRef = "ResolvingTaskRef"
	// TaskRunReasonImagePullFailed is the reason set when the step of a task fails due to image not being pulled
	TaskRunReasonImagePullFailed TaskRunReason = "TaskRunImagePullFailed"
	// TaskRunReasonResultLargerThanAllowedLimit is the reason set when one of the results exceeds its maximum allowed limit of 1 KB
	TaskRunReasonResultLargerThanAllowedLimit TaskRunReason = "TaskRunResultLargerThanAllowedLimit"
	// TaskRunReasonStopSidecarFailed indicates that the sidecar is not properly stopped.
	TaskRunReasonStopSidecarFailed = "TaskRunStopSidecarFailed"
)

func (t TaskRunReason) String() string {
	return string(t)
}

// GetStartedReason returns the reason set to the "Succeeded" condition when
// InitializeConditions is invoked
func (trs *TaskRunStatus) GetStartedReason() string {
	return TaskRunReasonStarted.String()
}

// GetRunningReason returns the reason set to the "Succeeded" condition when
// the TaskRun starts running. This is used indicate that the resource
// could be validated is starting to perform its job.
func (trs *TaskRunStatus) GetRunningReason() string {
	return TaskRunReasonRunning.String()
}

// MarkResourceOngoing sets the ConditionSucceeded condition to ConditionUnknown
// with the reason and message.
func (trs *TaskRunStatus) MarkResourceOngoing(reason TaskRunReason, message string) {
	taskRunCondSet.Manage(trs).SetCondition(apis.Condition{
		Type:    apis.ConditionSucceeded,
		Status:  corev1.ConditionUnknown,
		Reason:  reason.String(),
		Message: message,
	})
}

// MarkResourceFailed sets the ConditionSucceeded condition to ConditionFalse
// based on an error that occurred and a reason
func (trs *TaskRunStatus) MarkResourceFailed(reason TaskRunReason, err error) {
	taskRunCondSet.Manage(trs).SetCondition(apis.Condition{
		Type:    apis.ConditionSucceeded,
		Status:  corev1.ConditionFalse,
		Reason:  reason.String(),
		Message: err.Error(),
	})
	succeeded := trs.GetCondition(apis.ConditionSucceeded)
	trs.CompletionTime = &succeeded.LastTransitionTime.Inner
}

// TaskRunStatusFields holds the fields of TaskRun's status.  This is defined
// separately and inlined so that other types can readily consume these fields
// via duck typing.
type TaskRunStatusFields struct {
	// PodName is the name of the pod responsible for executing this task's steps.
	PodName string `json:"podName"`

	// StartTime is the time the build is actually started.
	StartTime *metav1.Time `json:"startTime,omitempty"`

	// CompletionTime is the time the build completed.
	CompletionTime *metav1.Time `json:"completionTime,omitempty"`

	// Steps describes the state of each build step container.
	// +optional
	// +listType=atomic
	Steps []StepState `json:"steps,omitempty"`

	// RetriesStatus contains the history of TaskRunStatus in case of a retry in order to keep record of failures.
	// All TaskRunStatus stored in RetriesStatus will have no date within the RetriesStatus as is redundant.
	// +optional
	// +listType=atomic
	RetriesStatus []TaskRunStatus `json:"retriesStatus,omitempty"`

	// Results are the list of results written out by the task's containers
	// +optional
	// +listType=atomic
	Results []TaskRunResult `json:"results,omitempty"`

	// The list has one entry per sidecar in the manifest. Each entry is
	// represents the imageid of the corresponding sidecar.
	// +listType=atomic
	Sidecars []SidecarState `json:"sidecars,omitempty"`

	// TaskSpec contains the Spec from the dereferenced Task definition used to instantiate this TaskRun.
	TaskSpec *TaskSpec `json:"taskSpec,omitempty"`

	// Provenance contains some key authenticated metadata about how a software artifact was built (what sources, what inputs/outputs, etc.).
	// +optional
	Provenance *Provenance `json:"provenance,omitempty"`

	// SpanContext contains tracing span context fields
	SpanContext map[string]string `json:"spanContext,omitempty"`
}

// TaskRunStepSpec is used to override the values of a Step in the corresponding Task.
type TaskRunStepSpec struct {
	// The name of the Step to override.
	Name string `json:"name"`
	// The resource requirements to apply to the Step.
	ComputeResources corev1.ResourceRequirements `json:"computeResources"`
}

// TaskRunSidecarSpec is used to override the values of a Sidecar in the corresponding Task.
type TaskRunSidecarSpec struct {
	// The name of the Sidecar to override.
	Name string `json:"name"`
	// The resource requirements to apply to the Sidecar.
	ComputeResources corev1.ResourceRequirements `json:"computeResources"`
}

// GetGroupVersionKind implements kmeta.OwnerRefable.
func (*TaskRun) GetGroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind(pipeline.TaskRunControllerName)
}

// GetStatusCondition returns the task run status as a ConditionAccessor
func (tr *TaskRun) GetStatusCondition() apis.ConditionAccessor {
	return &tr.Status
}

// GetCondition returns the Condition matching the given type.
func (trs *TaskRunStatus) GetCondition(t apis.ConditionType) *apis.Condition {
	return taskRunCondSet.Manage(trs).GetCondition(t)
}

// InitializeConditions will set all conditions in taskRunCondSet to unknown for the TaskRun
// and set the started time to the current time
func (trs *TaskRunStatus) InitializeConditions() {
	started := false
	if trs.StartTime.IsZero() {
		trs.StartTime = &metav1.Time{Time: time.Now()}
		started = true
	}
	conditionManager := taskRunCondSet.Manage(trs)
	conditionManager.InitializeConditions()
	// Ensure the started reason is set for the "Succeeded" condition
	if started {
		initialCondition := conditionManager.GetCondition(apis.ConditionSucceeded)
		initialCondition.Reason = TaskRunReasonStarted.String()
		conditionManager.SetCondition(*initialCondition)
	}
}

// SetCondition sets the condition, unsetting previous conditions with the same
// type as necessary.
func (trs *TaskRunStatus) SetCondition(newCond *apis.Condition) {
	if newCond != nil {
		taskRunCondSet.Manage(trs).SetCondition(*newCond)
	}
}

// StepState reports the results of running a step in a Task.
type StepState struct {
	corev1.ContainerState `json:",inline"`
	Name                  string `json:"name,omitempty"`
	Container             string `json:"container,omitempty"`
	ImageID               string `json:"imageID,omitempty"`
}

// SidecarState reports the results of running a sidecar in a Task.
type SidecarState struct {
	corev1.ContainerState `json:",inline"`
	Name                  string `json:"name,omitempty"`
	Container             string `json:"container,omitempty"`
	ImageID               string `json:"imageID,omitempty"`
}

// +genclient
// +genreconciler:krshapedlogic=false
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// TaskRun represents a single execution of a Task. TaskRuns are how the steps
// specified in a Task are executed; they specify the parameters and resources
// used to run the steps in a Task.
//
// +k8s:openapi-gen=true
type TaskRun struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +optional
	Spec TaskRunSpec `json:"spec,omitempty"`
	// +optional
	Status TaskRunStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// TaskRunList contains a list of TaskRun
type TaskRunList struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TaskRun `json:"items"`
}

// GetPipelineRunPVCName for TaskRun gets pipelinerun
func (tr *TaskRun) GetPipelineRunPVCName() string {
	if tr == nil {
		return ""
	}
	for _, ref := range tr.GetOwnerReferences() {
		if ref.Kind == pipeline.PipelineRunControllerName {
			return fmt.Sprintf("%s-pvc", ref.Name)
		}
	}
	return ""
}

// HasPipelineRunOwnerReference returns true of TaskRun has
// owner reference of type PipelineRun
func (tr *TaskRun) HasPipelineRunOwnerReference() bool {
	for _, ref := range tr.GetOwnerReferences() {
		if ref.Kind == pipeline.PipelineRunControllerName {
			return true
		}
	}
	return false
}

// IsDone returns true if the TaskRun's status indicates that it is done.
func (tr *TaskRun) IsDone() bool {
	return !tr.Status.GetCondition(apis.ConditionSucceeded).IsUnknown()
}

// HasStarted function check whether TaskRun has valid start time set in its status
func (tr *TaskRun) HasStarted() bool {
	return tr.Status.StartTime != nil && !tr.Status.StartTime.IsZero()
}

// IsSuccessful returns true if the TaskRun's status indicates that it has succeeded.
func (tr *TaskRun) IsSuccessful() bool {
	return tr != nil && tr.Status.GetCondition(apis.ConditionSucceeded).IsTrue()
}

// IsFailure returns true if the TaskRun's status indicates that it has failed.
func (tr *TaskRun) IsFailure() bool {
	return tr != nil && tr.Status.GetCondition(apis.ConditionSucceeded).IsFalse()
}

// IsCancelled returns true if the TaskRun's spec status is set to Cancelled state
func (tr *TaskRun) IsCancelled() bool {
	return tr.Spec.Status == TaskRunSpecStatusCancelled
}

// IsRetriable returns true if the TaskRun's Retries is not exhausted.
func (tr *TaskRun) IsRetriable() bool {
	return len(tr.Status.RetriesStatus) < tr.Spec.Retries
}

// HasTimedOut returns true if the TaskRun runtime is beyond the allowed timeout
func (tr *TaskRun) HasTimedOut(ctx context.Context, c clock.PassiveClock) bool {
	if tr.Status.StartTime.IsZero() {
		return false
	}
	timeout := tr.GetTimeout(ctx)
	// If timeout is set to 0 or defaulted to 0, there is no timeout.
	if timeout == apisconfig.NoTimeoutDuration {
		return false
	}
	runtime := c.Since(tr.Status.StartTime.Time)
	return runtime > timeout
}

// GetTimeout returns the timeout for the TaskRun, or the default if not specified
func (tr *TaskRun) GetTimeout(ctx context.Context) time.Duration {
	// Use the platform default is no timeout is set
	if tr.Spec.Timeout == nil {
		defaultTimeout := time.Duration(config.FromContextOrDefaults(ctx).Defaults.DefaultTimeoutMinutes)
		return defaultTimeout * time.Minute //nolint:durationcheck
	}
	return tr.Spec.Timeout.Duration
}

// GetNamespacedName returns a k8s namespaced name that identifies this TaskRun
func (tr *TaskRun) GetNamespacedName() types.NamespacedName {
	return types.NamespacedName{Namespace: tr.Namespace, Name: tr.Name}
}

// HasVolumeClaimTemplate returns true if TaskRun contains volumeClaimTemplates that is
// used for creating PersistentVolumeClaims with an OwnerReference for each run
func (tr *TaskRun) HasVolumeClaimTemplate() bool {
	for _, ws := range tr.Spec.Workspaces {
		if ws.VolumeClaimTemplate != nil {
			return true
		}
	}
	return false
}
