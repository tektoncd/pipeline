/*
Copyright 2019 The Tekton Authors

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
	"context"
	"time"

	"github.com/tektoncd/pipeline/pkg/apis/config"
	apisconfig "github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	pod "github.com/tektoncd/pipeline/pkg/apis/pipeline/pod"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
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
	// Deprecated: Unused, preserved only for backwards compatibility
	// +optional
	Resources *TaskRunResources `json:"resources,omitempty"`
	// +optional
	ServiceAccountName string `json:"serviceAccountName"`
	// no more than one of the TaskRef and TaskSpec may be specified.
	// +optional
	TaskRef *TaskRef `json:"taskRef,omitempty"`
	// Specifying PipelineSpec can be disabled by setting
	// `disable-inline-spec` feature flag..
	// +optional
	TaskSpec *TaskSpec `json:"taskSpec,omitempty"`
	// Used for cancelling a TaskRun (and maybe more later on)
	// +optional
	Status TaskRunSpecStatus `json:"status,omitempty"`
	// Status message for cancellation.
	// +optional
	StatusMessage TaskRunSpecStatusMessage `json:"statusMessage,omitempty"`
	// Retries represents how many times this TaskRun should be retried in the event of Task failure.
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
	// Overrides to apply to Steps in this TaskRun.
	// If a field is specified in both a Step and a StepOverride,
	// the value from the StepOverride will be used.
	// This field is only supported when the alpha feature gate is enabled.
	// +optional
	// +listType=atomic
	StepOverrides []TaskRunStepOverride `json:"stepOverrides,omitempty"`
	// Overrides to apply to Sidecars in this TaskRun.
	// If a field is specified in both a Sidecar and a SidecarOverride,
	// the value from the SidecarOverride will be used.
	// This field is only supported when the alpha feature gate is enabled.
	// +optional
	// +listType=atomic
	SidecarOverrides []TaskRunSidecarOverride `json:"sidecarOverrides,omitempty"`
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

const (
	// EnabledOnFailureBreakpoint is the value for TaskRunDebug.Breakpoints.OnFailure that means the breakpoint onFailure is enabled
	EnabledOnFailureBreakpoint = "enabled"
)

// TaskRunDebug defines the breakpoint config for a particular TaskRun
type TaskRunDebug struct {
	// +optional
	Breakpoints *TaskBreakpoints `json:"breakpoints,omitempty"`
}

// TaskBreakpoints defines the breakpoint config for a particular Task
type TaskBreakpoints struct {
	// if enabled, pause TaskRun on failure of a step
	// failed step will not exit
	// +optional
	OnFailure string `json:"onFailure,omitempty"`
	// +optional
	// +listType=atomic
	BeforeSteps []string `json:"beforeSteps,omitempty"`
}

// NeedsDebugOnFailure return true if the TaskRun is configured to debug on failure
func (trd *TaskRunDebug) NeedsDebugOnFailure() bool {
	if trd.Breakpoints == nil {
		return false
	}
	return trd.Breakpoints.OnFailure == EnabledOnFailureBreakpoint
}

// NeedsDebugBeforeStep return true if the step is configured to debug before execution
func (trd *TaskRunDebug) NeedsDebugBeforeStep(stepName string) bool {
	if trd.Breakpoints == nil {
		return false
	}
	beforeStepSets := sets.NewString(trd.Breakpoints.BeforeSteps...)
	return beforeStepSets.Has(stepName)
}

// StepNeedsDebug return true if the step is configured to debug
func (trd *TaskRunDebug) StepNeedsDebug(stepName string) bool {
	return trd.NeedsDebugOnFailure() || trd.NeedsDebugBeforeStep(stepName)
}

// HaveBeforeSteps return true if have any before steps
func (trd *TaskRunDebug) HaveBeforeSteps() bool {
	return trd.Breakpoints != nil && len(trd.Breakpoints.BeforeSteps) > 0
}

// NeedsDebug return true if defined onfailure or have any before, after steps
func (trd *TaskRunDebug) NeedsDebug() bool {
	return trd.NeedsDebugOnFailure() || trd.HaveBeforeSteps()
}

var taskRunCondSet = apis.NewBatchConditionSet()

// TaskRunStatus defines the observed state of TaskRun
type TaskRunStatus struct {
	duckv1.Status `json:",inline"`

	// TaskRunStatusFields inlines the status fields.
	TaskRunStatusFields `json:",inline"`
}

// TaskRunConditionType is an enum used to store TaskRun custom
// conditions such as one used in spire results verification
type TaskRunConditionType string

const (
	// TaskRunConditionResultsVerified is a Condition Type that indicates that the results were verified by spire
	TaskRunConditionResultsVerified TaskRunConditionType = "SignedResultsVerified"
)

func (t TaskRunConditionType) String() string {
	return string(t)
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
	// TaskRunReasonResultsVerified is the reason set when the TaskRun results are verified by spire
	TaskRunReasonResultsVerified TaskRunReason = "TaskRunResultsVerified"
	// TaskRunReasonsResultsVerificationFailed is the reason set when the TaskRun results are failed to verify by spire
	TaskRunReasonsResultsVerificationFailed TaskRunReason = "TaskRunResultsVerificationFailed"
	// AwaitingTaskRunResults is the reason set when waiting upon `TaskRun` results and signatures to verify
	AwaitingTaskRunResults TaskRunReason = "AwaitingTaskRunResults"
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

	// CloudEvents describe the state of each cloud event requested via a
	// CloudEventResource.
	//
	// Deprecated: Removed in v0.44.0.
	//
	// +optional
	// +listType=atomic
	CloudEvents []CloudEventDelivery `json:"cloudEvents,omitempty"`

	// RetriesStatus contains the history of TaskRunStatus in case of a retry in order to keep record of failures.
	// All TaskRunStatus stored in RetriesStatus will have no date within the RetriesStatus as is redundant.
	// +optional
	// +listType=atomic
	RetriesStatus []TaskRunStatus `json:"retriesStatus,omitempty"`

	// Results from Resources built during the TaskRun.
	// This is tomb-stoned along with the removal of pipelineResources
	// Deprecated: this field is not populated and is preserved only for backwards compatibility
	// +optional
	// +listType=atomic
	ResourcesResult []PipelineResourceResult `json:"resourcesResult,omitempty"`

	// TaskRunResults are the list of results written out by the task's containers
	// +optional
	// +listType=atomic
	TaskRunResults []TaskRunResult `json:"taskResults,omitempty"`

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

// TaskRunStepOverride is used to override the values of a Step in the corresponding Task.
type TaskRunStepOverride struct {
	// The name of the Step to override.
	Name string `json:"name"`
	// The resource requirements to apply to the Step.
	Resources corev1.ResourceRequirements `json:"resources"`
}

// TaskRunSidecarOverride is used to override the values of a Sidecar in the corresponding Task.
type TaskRunSidecarOverride struct {
	// The name of the Sidecar to override.
	Name string `json:"name"`
	// The resource requirements to apply to the Sidecar.
	Resources corev1.ResourceRequirements `json:"resources"`
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
	Name                  string                `json:"name,omitempty"`
	ContainerName         string                `json:"container,omitempty"`
	ImageID               string                `json:"imageID,omitempty"`
	Results               []TaskRunStepResult   `json:"results,omitempty"`
	Provenance            *Provenance           `json:"provenance,omitempty"`
	Inputs                []TaskRunStepArtifact `json:"inputs,omitempty"`
	Outputs               []TaskRunStepArtifact `json:"outputs,omitempty"`
}

// SidecarState reports the results of running a sidecar in a Task.
type SidecarState struct {
	corev1.ContainerState `json:",inline"`
	Name                  string `json:"name,omitempty"`
	ContainerName         string `json:"container,omitempty"`
	ImageID               string `json:"imageID,omitempty"`
}

// CloudEventDelivery is the target of a cloud event along with the state of
// delivery.
type CloudEventDelivery struct {
	// Target points to an addressable
	Target string                  `json:"target,omitempty"`
	Status CloudEventDeliveryState `json:"status,omitempty"`
}

// CloudEventCondition is a string that represents the condition of the event.
type CloudEventCondition string

const (
	// CloudEventConditionUnknown means that the condition for the event to be
	// triggered was not met yet, or we don't know the state yet.
	CloudEventConditionUnknown CloudEventCondition = "Unknown"
	// CloudEventConditionSent means that the event was sent successfully
	CloudEventConditionSent CloudEventCondition = "Sent"
	// CloudEventConditionFailed means that there was one or more attempts to
	// send the event, and none was successful so far.
	CloudEventConditionFailed CloudEventCondition = "Failed"
)

// CloudEventDeliveryState reports the state of a cloud event to be sent.
type CloudEventDeliveryState struct {
	// Current status
	Condition CloudEventCondition `json:"condition,omitempty"`
	// SentAt is the time at which the last attempt to send the event was made
	// +optional
	SentAt *metav1.Time `json:"sentAt,omitempty"`
	// Error is the text of error (if any)
	Error string `json:"message"`
	// RetryCount is the number of attempts of sending the cloud event
	RetryCount int32 `json:"retryCount"`
}

// +genclient
// +genreconciler:krshapedlogic=false
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +k8s:openapi-gen=true

// TaskRun represents a single execution of a Task. TaskRuns are how the steps
// specified in a Task are executed; they specify the parameters and resources
// used to run the steps in a Task.
//
// Deprecated: Please use v1.TaskRun instead.
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
			return ref.Name + "-pvc"
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

// IsTaskRunResultVerified returns true if the TaskRun's results have been validated by spire.
func (tr *TaskRun) IsTaskRunResultVerified() bool {
	return tr.Status.GetCondition(apis.ConditionType(TaskRunConditionResultsVerified.String())).IsTrue()
}

// IsTaskRunResultDone returns true if the TaskRun's results are available for verification
func (tr *TaskRun) IsTaskRunResultDone() bool {
	return !tr.Status.GetCondition(apis.ConditionType(TaskRunConditionResultsVerified.String())).IsUnknown()
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
