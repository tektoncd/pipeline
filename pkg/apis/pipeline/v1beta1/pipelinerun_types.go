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
	"fmt"
	"time"

	"github.com/tektoncd/pipeline/pkg/apis/config"
	apisconfig "github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	pod "github.com/tektoncd/pipeline/pkg/apis/pipeline/pod"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/clock"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

// +genclient
// +genreconciler:krshapedlogic=false
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PipelineRun represents a single execution of a Pipeline. PipelineRuns are how
// the graph of Tasks declared in a Pipeline are executed; they specify inputs
// to Pipelines such as parameter values and capture operational aspects of the
// Tasks execution such as service account and tolerations. Creating a
// PipelineRun creates TaskRuns for Tasks in the referenced Pipeline.
//
// +k8s:openapi-gen=true
type PipelineRun struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +optional
	Spec PipelineRunSpec `json:"spec,omitempty"`
	// +optional
	Status PipelineRunStatus `json:"status,omitempty"`
}

// GetName Returns the name of the PipelineRun
func (pr *PipelineRun) GetName() string {
	return pr.ObjectMeta.GetName()
}

// GetStatusCondition returns the task run status as a ConditionAccessor
func (pr *PipelineRun) GetStatusCondition() apis.ConditionAccessor {
	return &pr.Status
}

// GetGroupVersionKind implements kmeta.OwnerRefable.
func (*PipelineRun) GetGroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind(pipeline.PipelineRunControllerName)
}

// IsDone returns true if the PipelineRun's status indicates that it is done.
func (pr *PipelineRun) IsDone() bool {
	return !pr.Status.GetCondition(apis.ConditionSucceeded).IsUnknown()
}

// HasStarted function check whether pipelinerun has valid start time set in its status
func (pr *PipelineRun) HasStarted() bool {
	return pr.Status.StartTime != nil && !pr.Status.StartTime.IsZero()
}

// IsCancelled returns true if the PipelineRun's spec status is set to Cancelled state
func (pr *PipelineRun) IsCancelled() bool {
	return pr.Spec.Status == PipelineRunSpecStatusCancelled
}

// IsGracefullyCancelled returns true if the PipelineRun's spec status is set to CancelledRunFinally state
func (pr *PipelineRun) IsGracefullyCancelled() bool {
	return pr.Spec.Status == PipelineRunSpecStatusCancelledRunFinally
}

// IsGracefullyStopped returns true if the PipelineRun's spec status is set to StoppedRunFinally state
func (pr *PipelineRun) IsGracefullyStopped() bool {
	return pr.Spec.Status == PipelineRunSpecStatusStoppedRunFinally
}

// PipelineTimeout returns the applicable timeout for the PipelineRun
func (pr *PipelineRun) PipelineTimeout(ctx context.Context) time.Duration {
	if pr.Spec.Timeout != nil {
		return pr.Spec.Timeout.Duration
	}
	if pr.Spec.Timeouts != nil && pr.Spec.Timeouts.Pipeline != nil {
		return pr.Spec.Timeouts.Pipeline.Duration
	}
	return time.Duration(config.FromContextOrDefaults(ctx).Defaults.DefaultTimeoutMinutes) * time.Minute
}

// TasksTimeout returns the tasks timeout for the PipelineRun, if set,
// or the tasks timeout computed from the Pipeline and Finally timeouts, if those are set.
func (pr *PipelineRun) TasksTimeout() *metav1.Duration {
	t := pr.Spec.Timeouts
	if t == nil {
		return nil
	}
	if t.Tasks != nil {
		return t.Tasks
	}
	if t.Pipeline != nil && t.Finally != nil {
		if t.Pipeline.Duration == apisconfig.NoTimeoutDuration || t.Finally.Duration == apisconfig.NoTimeoutDuration {
			return nil
		}
		return &metav1.Duration{Duration: (t.Pipeline.Duration - t.Finally.Duration)}
	}
	return nil
}

// FinallyTimeout returns the finally timeout for the PipelineRun, if set,
// or the finally timeout computed from the Pipeline and Tasks timeouts, if those are set.
func (pr *PipelineRun) FinallyTimeout() *metav1.Duration {
	t := pr.Spec.Timeouts
	if t == nil {
		return nil
	}
	if t.Finally != nil {
		return t.Finally
	}
	if t.Pipeline != nil && t.Tasks != nil {
		if t.Pipeline.Duration == apisconfig.NoTimeoutDuration || t.Tasks.Duration == apisconfig.NoTimeoutDuration {
			return nil
		}
		return &metav1.Duration{Duration: (t.Pipeline.Duration - t.Tasks.Duration)}
	}
	return nil
}

// IsPending returns true if the PipelineRun's spec status is set to Pending state
func (pr *PipelineRun) IsPending() bool {
	return pr.Spec.Status == PipelineRunSpecStatusPending
}

// GetNamespacedName returns a k8s namespaced name that identifies this PipelineRun
func (pr *PipelineRun) GetNamespacedName() types.NamespacedName {
	return types.NamespacedName{Namespace: pr.Namespace, Name: pr.Name}
}

// IsTimeoutConditionSet returns true when the pipelinerun has the pipelinerun timed out reason
func (pr *PipelineRun) IsTimeoutConditionSet() bool {
	condition := pr.Status.GetCondition(apis.ConditionSucceeded)
	return condition.IsFalse() && condition.Reason == PipelineRunReasonTimedOut.String()
}

// SetTimeoutCondition sets the status of the PipelineRun to timed out.
func (pr *PipelineRun) SetTimeoutCondition(ctx context.Context) {
	pr.Status.SetCondition(&apis.Condition{
		Type:    apis.ConditionSucceeded,
		Status:  corev1.ConditionFalse,
		Reason:  PipelineRunReasonTimedOut.String(),
		Message: fmt.Sprintf("PipelineRun %q failed to finish within %q", pr.Name, pr.PipelineTimeout(ctx).String()),
	})
}

// HasTimedOut returns true if a pipelinerun has exceeded its spec.Timeout based on its status.Timeout
func (pr *PipelineRun) HasTimedOut(ctx context.Context, c clock.PassiveClock) bool {
	timeout := pr.PipelineTimeout(ctx)
	startTime := pr.Status.StartTime

	if !startTime.IsZero() {
		if timeout == config.NoTimeoutDuration {
			return false
		}
		runtime := c.Since(startTime.Time)
		if runtime > timeout {
			return true
		}
	}
	return false
}

// HasTimedOutForALongTime returns true if a pipelinerun has exceeed its spec.Timeout based its status.StartTime
// by a large margin
func (pr *PipelineRun) HasTimedOutForALongTime(ctx context.Context, c clock.PassiveClock) bool {
	if !pr.HasTimedOut(ctx, c) {
		return false
	}
	timeout := pr.PipelineTimeout(ctx)
	startTime := pr.Status.StartTime
	runtime := c.Since(startTime.Time)
	// We are arbitrarily defining large margin as doubling the spec.timeout
	return runtime >= 2*timeout
}

// HaveTasksTimedOut returns true if a pipelinerun has exceeded its spec.Timeouts.Tasks
func (pr *PipelineRun) HaveTasksTimedOut(ctx context.Context, c clock.PassiveClock) bool {
	timeout := pr.TasksTimeout()
	startTime := pr.Status.StartTime

	if !startTime.IsZero() && timeout != nil {
		if timeout.Duration == config.NoTimeoutDuration {
			return false
		}
		runtime := c.Since(startTime.Time)
		if runtime > timeout.Duration {
			return true
		}
	}
	return false
}

// HasFinallyTimedOut returns true if a pipelinerun has exceeded its spec.Timeouts.Finally, based on status.FinallyStartTime
func (pr *PipelineRun) HasFinallyTimedOut(ctx context.Context, c clock.PassiveClock) bool {
	timeout := pr.FinallyTimeout()
	startTime := pr.Status.FinallyStartTime

	if startTime != nil && !startTime.IsZero() && timeout != nil {
		if timeout.Duration == config.NoTimeoutDuration {
			return false
		}
		runtime := c.Since(startTime.Time)
		if runtime > timeout.Duration {
			return true
		}
	}
	return false
}

// HasVolumeClaimTemplate returns true if PipelineRun contains volumeClaimTemplates that is
// used for creating PersistentVolumeClaims with an OwnerReference for each run
func (pr *PipelineRun) HasVolumeClaimTemplate() bool {
	for _, ws := range pr.Spec.Workspaces {
		if ws.VolumeClaimTemplate != nil {
			return true
		}
	}
	return false
}

// PipelineRunSpec defines the desired state of PipelineRun
type PipelineRunSpec struct {
	// +optional
	PipelineRef *PipelineRef `json:"pipelineRef,omitempty"`
	// +optional
	PipelineSpec *PipelineSpec `json:"pipelineSpec,omitempty"`
	// Resources is a list of bindings specifying which actual instances of
	// PipelineResources to use for the resources the Pipeline has declared
	// it needs.
	//
	// Deprecated: Unused, preserved only for backwards compatibility
	// +listType=atomic
	Resources []PipelineResourceBinding `json:"resources,omitempty"`
	// Params is a list of parameter names and values.
	// +listType=atomic
	Params Params `json:"params,omitempty"`
	// +optional
	ServiceAccountName string `json:"serviceAccountName,omitempty"`

	// Used for cancelling a pipelinerun (and maybe more later on)
	// +optional
	Status PipelineRunSpecStatus `json:"status,omitempty"`
	// Time after which the Pipeline times out.
	// Currently three keys are accepted in the map
	// pipeline, tasks and finally
	// with Timeouts.pipeline >= Timeouts.tasks + Timeouts.finally
	// +optional
	Timeouts *TimeoutFields `json:"timeouts,omitempty"`

	// Timeout is the Time after which the Pipeline times out.
	// Defaults to never.
	// Refer to Go's ParseDuration documentation for expected format: https://golang.org/pkg/time/#ParseDuration
	//
	// Deprecated: use pipelineRunSpec.Timeouts.Pipeline instead
	//
	// +optional
	Timeout *metav1.Duration `json:"timeout,omitempty"`
	// PodTemplate holds pod specific configuration
	PodTemplate *pod.PodTemplate `json:"podTemplate,omitempty"`
	// Workspaces holds a set of workspace bindings that must match names
	// with those declared in the pipeline.
	// +optional
	// +listType=atomic
	Workspaces []WorkspaceBinding `json:"workspaces,omitempty"`
	// TaskRunSpecs holds a set of runtime specs
	// +optional
	// +listType=atomic
	TaskRunSpecs []PipelineTaskRunSpec `json:"taskRunSpecs,omitempty"`
}

// TimeoutFields allows granular specification of pipeline, task, and finally timeouts
type TimeoutFields struct {
	// Pipeline sets the maximum allowed duration for execution of the entire pipeline. The sum of individual timeouts for tasks and finally must not exceed this value.
	Pipeline *metav1.Duration `json:"pipeline,omitempty"`
	// Tasks sets the maximum allowed duration of this pipeline's tasks
	Tasks *metav1.Duration `json:"tasks,omitempty"`
	// Finally sets the maximum allowed duration of this pipeline's finally
	Finally *metav1.Duration `json:"finally,omitempty"`
}

// PipelineRunSpecStatus defines the pipelinerun spec status the user can provide
type PipelineRunSpecStatus string

const (
	// PipelineRunSpecStatusCancelled indicates that the user wants to cancel the task,
	// if not already cancelled or terminated
	PipelineRunSpecStatusCancelled = "Cancelled"

	// PipelineRunSpecStatusCancelledRunFinally indicates that the user wants to cancel the pipeline run,
	// if not already cancelled or terminated, but ensure finally is run normally
	PipelineRunSpecStatusCancelledRunFinally = "CancelledRunFinally"

	// PipelineRunSpecStatusStoppedRunFinally indicates that the user wants to stop the pipeline run,
	// wait for already running tasks to be completed and run finally
	// if not already cancelled or terminated
	PipelineRunSpecStatusStoppedRunFinally = "StoppedRunFinally"

	// PipelineRunSpecStatusPending indicates that the user wants to postpone starting a PipelineRun
	// until some condition is met
	PipelineRunSpecStatusPending = "PipelineRunPending"
)

// PipelineRunStatus defines the observed state of PipelineRun
type PipelineRunStatus struct {
	duckv1.Status `json:",inline"`

	// PipelineRunStatusFields inlines the status fields.
	PipelineRunStatusFields `json:",inline"`
}

// PipelineRunReason represents a reason for the pipeline run "Succeeded" condition
type PipelineRunReason string

const (
	// PipelineRunReasonStarted is the reason set when the PipelineRun has just started
	PipelineRunReasonStarted PipelineRunReason = "Started"
	// PipelineRunReasonRunning is the reason set when the PipelineRun is running
	PipelineRunReasonRunning PipelineRunReason = "Running"
	// PipelineRunReasonSuccessful is the reason set when the PipelineRun completed successfully
	PipelineRunReasonSuccessful PipelineRunReason = "Succeeded"
	// PipelineRunReasonCompleted is the reason set when the PipelineRun completed successfully with one or more skipped Tasks
	PipelineRunReasonCompleted PipelineRunReason = "Completed"
	// PipelineRunReasonFailed is the reason set when the PipelineRun completed with a failure
	PipelineRunReasonFailed PipelineRunReason = "Failed"
	// PipelineRunReasonCancelled is the reason set when the PipelineRun cancelled by the user
	// This reason may be found with a corev1.ConditionFalse status, if the cancellation was processed successfully
	// This reason may be found with a corev1.ConditionUnknown status, if the cancellation is being processed or failed
	PipelineRunReasonCancelled PipelineRunReason = "Cancelled"
	// PipelineRunReasonPending is the reason set when the PipelineRun is in the pending state
	PipelineRunReasonPending PipelineRunReason = "PipelineRunPending"
	// PipelineRunReasonTimedOut is the reason set when the PipelineRun has timed out
	PipelineRunReasonTimedOut PipelineRunReason = "PipelineRunTimeout"
	// PipelineRunReasonStopping indicates that no new Tasks will be scheduled by the controller, and the
	// pipeline will stop once all running tasks complete their work
	PipelineRunReasonStopping PipelineRunReason = "PipelineRunStopping"
	// PipelineRunReasonCancelledRunningFinally indicates that pipeline has been gracefully cancelled
	// and no new Tasks will be scheduled by the controller, but final tasks are now running
	PipelineRunReasonCancelledRunningFinally PipelineRunReason = "CancelledRunningFinally"
	// PipelineRunReasonStoppedRunningFinally indicates that pipeline has been gracefully stopped
	// and no new Tasks will be scheduled by the controller, but final tasks are now running
	PipelineRunReasonStoppedRunningFinally PipelineRunReason = "StoppedRunningFinally"
)

func (t PipelineRunReason) String() string {
	return string(t)
}

var pipelineRunCondSet = apis.NewBatchConditionSet()

// GetCondition returns the Condition matching the given type.
func (pr *PipelineRunStatus) GetCondition(t apis.ConditionType) *apis.Condition {
	return pipelineRunCondSet.Manage(pr).GetCondition(t)
}

// InitializeConditions will set all conditions in pipelineRunCondSet to unknown for the PipelineRun
// and set the started time to the current time
func (pr *PipelineRunStatus) InitializeConditions(c clock.PassiveClock) {
	started := false
	if pr.StartTime.IsZero() {
		pr.StartTime = &metav1.Time{Time: c.Now()}
		started = true
	}
	conditionManager := pipelineRunCondSet.Manage(pr)
	conditionManager.InitializeConditions()
	// Ensure the started reason is set for the "Succeeded" condition
	if started {
		initialCondition := conditionManager.GetCondition(apis.ConditionSucceeded)
		initialCondition.Reason = PipelineRunReasonStarted.String()
		conditionManager.SetCondition(*initialCondition)
	}
}

// SetCondition sets the condition, unsetting previous conditions with the same
// type as necessary.
func (pr *PipelineRunStatus) SetCondition(newCond *apis.Condition) {
	if newCond != nil {
		pipelineRunCondSet.Manage(pr).SetCondition(*newCond)
	}
}

// MarkSucceeded changes the Succeeded condition to True with the provided reason and message.
func (pr *PipelineRunStatus) MarkSucceeded(reason, messageFormat string, messageA ...interface{}) {
	pipelineRunCondSet.Manage(pr).MarkTrueWithReason(apis.ConditionSucceeded, reason, messageFormat, messageA...)
	succeeded := pr.GetCondition(apis.ConditionSucceeded)
	pr.CompletionTime = &succeeded.LastTransitionTime.Inner
}

// MarkFailed changes the Succeeded condition to False with the provided reason and message.
func (pr *PipelineRunStatus) MarkFailed(reason, messageFormat string, messageA ...interface{}) {
	pipelineRunCondSet.Manage(pr).MarkFalse(apis.ConditionSucceeded, reason, messageFormat, messageA...)
	succeeded := pr.GetCondition(apis.ConditionSucceeded)
	pr.CompletionTime = &succeeded.LastTransitionTime.Inner
}

// MarkRunning changes the Succeeded condition to Unknown with the provided reason and message.
func (pr *PipelineRunStatus) MarkRunning(reason, messageFormat string, messageA ...interface{}) {
	pipelineRunCondSet.Manage(pr).MarkUnknown(apis.ConditionSucceeded, reason, messageFormat, messageA...)
}

// ChildStatusReference is used to point to the statuses of individual TaskRuns and Runs within this PipelineRun.
type ChildStatusReference struct {
	runtime.TypeMeta `json:",inline"`
	// Name is the name of the TaskRun or Run this is referencing.
	Name string `json:"name,omitempty"`
	// PipelineTaskName is the name of the PipelineTask this is referencing.
	PipelineTaskName string `json:"pipelineTaskName,omitempty"`

	// WhenExpressions is the list of checks guarding the execution of the PipelineTask
	// +optional
	// +listType=atomic
	WhenExpressions []WhenExpression `json:"whenExpressions,omitempty"`
}

// PipelineRunStatusFields holds the fields of PipelineRunStatus' status.
// This is defined separately and inlined so that other types can readily
// consume these fields via duck typing.
type PipelineRunStatusFields struct {
	// StartTime is the time the PipelineRun is actually started.
	StartTime *metav1.Time `json:"startTime,omitempty"`

	// CompletionTime is the time the PipelineRun completed.
	CompletionTime *metav1.Time `json:"completionTime,omitempty"`

	// TaskRuns is a map of PipelineRunTaskRunStatus with the taskRun name as the key.
	//
	// Deprecated: use ChildReferences instead. As of v0.45.0, this field is no
	// longer populated and is only included for backwards compatibility with
	// older server versions.
	// +optional
	TaskRuns map[string]*PipelineRunTaskRunStatus `json:"taskRuns,omitempty"`

	// Runs is a map of PipelineRunRunStatus with the run name as the key
	//
	// Deprecated: use ChildReferences instead. As of v0.45.0, this field is no
	// longer populated and is only included for backwards compatibility with
	// older server versions.
	// +optional
	Runs map[string]*PipelineRunRunStatus `json:"runs,omitempty"`

	// PipelineResults are the list of results written out by the pipeline task's containers
	// +optional
	// +listType=atomic
	PipelineResults []PipelineRunResult `json:"pipelineResults,omitempty"`

	// PipelineRunSpec contains the exact spec used to instantiate the run
	PipelineSpec *PipelineSpec `json:"pipelineSpec,omitempty"`

	// list of tasks that were skipped due to when expressions evaluating to false
	// +optional
	// +listType=atomic
	SkippedTasks []SkippedTask `json:"skippedTasks,omitempty"`

	// list of TaskRun and Run names, PipelineTask names, and API versions/kinds for children of this PipelineRun.
	// +optional
	// +listType=atomic
	ChildReferences []ChildStatusReference `json:"childReferences,omitempty"`

	// FinallyStartTime is when all non-finally tasks have been completed and only finally tasks are being executed.
	// +optional
	FinallyStartTime *metav1.Time `json:"finallyStartTime,omitempty"`

	// Provenance contains some key authenticated metadata about how a software artifact was built (what sources, what inputs/outputs, etc.).
	// +optional
	Provenance *Provenance `json:"provenance,omitempty"`

	// SpanContext contains tracing span context fields
	SpanContext map[string]string `json:"spanContext,omitempty"`
}

// SkippedTask is used to describe the Tasks that were skipped due to their When Expressions
// evaluating to False. This is a struct because we are looking into including more details
// about the When Expressions that caused this Task to be skipped.
type SkippedTask struct {
	// Name is the Pipeline Task name
	Name string `json:"name"`
	// Reason is the cause of the PipelineTask being skipped.
	Reason SkippingReason `json:"reason"`
	// WhenExpressions is the list of checks guarding the execution of the PipelineTask
	// +optional
	// +listType=atomic
	WhenExpressions []WhenExpression `json:"whenExpressions,omitempty"`
}

// SkippingReason explains why a PipelineTask was skipped.
type SkippingReason string

const (
	// WhenExpressionsSkip means the task was skipped due to at least one of its when expressions evaluating to false
	WhenExpressionsSkip SkippingReason = "When Expressions evaluated to false"
	// ParentTasksSkip means the task was skipped because its parent was skipped
	ParentTasksSkip SkippingReason = "Parent Tasks were skipped"
	// StoppingSkip means the task was skipped because the pipeline run is stopping
	StoppingSkip SkippingReason = "PipelineRun was stopping"
	// GracefullyCancelledSkip means the task was skipped because the pipeline run has been gracefully cancelled
	GracefullyCancelledSkip SkippingReason = "PipelineRun was gracefully cancelled"
	// GracefullyStoppedSkip means the task was skipped because the pipeline run has been gracefully stopped
	GracefullyStoppedSkip SkippingReason = "PipelineRun was gracefully stopped"
	// MissingResultsSkip means the task was skipped because it's missing necessary results
	MissingResultsSkip SkippingReason = "Results were missing"
	// PipelineTimedOutSkip means the task was skipped because the PipelineRun has passed its overall timeout.
	PipelineTimedOutSkip SkippingReason = "PipelineRun timeout has been reached"
	// TasksTimedOutSkip means the task was skipped because the PipelineRun has passed its Timeouts.Tasks.
	TasksTimedOutSkip SkippingReason = "PipelineRun Tasks timeout has been reached"
	// FinallyTimedOutSkip means the task was skipped because the PipelineRun has passed its Timeouts.Finally.
	FinallyTimedOutSkip SkippingReason = "PipelineRun Finally timeout has been reached"
	// EmptyArrayInMatrixParams means the task was skipped because Matrix parameters contain empty array.
	EmptyArrayInMatrixParams SkippingReason = "Matrix Parameters have an empty array"
	// None means the task was not skipped
	None SkippingReason = "None"
)

// PipelineRunResult used to describe the results of a pipeline
type PipelineRunResult struct {
	// Name is the result's name as declared by the Pipeline
	Name string `json:"name"`

	// Value is the result returned from the execution of this PipelineRun
	Value ResultValue `json:"value"`
}

// PipelineRunTaskRunStatus contains the name of the PipelineTask for this TaskRun and the TaskRun's Status
type PipelineRunTaskRunStatus struct {
	// PipelineTaskName is the name of the PipelineTask.
	PipelineTaskName string `json:"pipelineTaskName,omitempty"`
	// Status is the TaskRunStatus for the corresponding TaskRun
	// +optional
	Status *TaskRunStatus `json:"status,omitempty"`
	// WhenExpressions is the list of checks guarding the execution of the PipelineTask
	// +optional
	// +listType=atomic
	WhenExpressions []WhenExpression `json:"whenExpressions,omitempty"`
}

// PipelineRunRunStatus contains the name of the PipelineTask for this CustomRun or Run and the CustomRun or Run's Status
type PipelineRunRunStatus struct {
	// PipelineTaskName is the name of the PipelineTask.
	PipelineTaskName string `json:"pipelineTaskName,omitempty"`
	// Status is the CustomRunStatus for the corresponding CustomRun or Run
	// +optional
	Status *CustomRunStatus `json:"status,omitempty"`
	// WhenExpressions is the list of checks guarding the execution of the PipelineTask
	// +optional
	// +listType=atomic
	WhenExpressions []WhenExpression `json:"whenExpressions,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PipelineRunList contains a list of PipelineRun
type PipelineRunList struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PipelineRun `json:"items,omitempty"`
}

// PipelineTaskRun reports the results of running a step in the Task. Each
// task has the potential to succeed or fail (based on the exit code)
// and produces logs.
type PipelineTaskRun struct {
	Name string `json:"name,omitempty"`
}

// PipelineTaskRunSpec  can be used to configure specific
// specs for a concrete Task
type PipelineTaskRunSpec struct {
	PipelineTaskName       string           `json:"pipelineTaskName,omitempty"`
	TaskServiceAccountName string           `json:"taskServiceAccountName,omitempty"`
	TaskPodTemplate        *pod.PodTemplate `json:"taskPodTemplate,omitempty"`
	// +listType=atomic
	StepOverrides []TaskRunStepOverride `json:"stepOverrides,omitempty"`
	// +listType=atomic
	SidecarOverrides []TaskRunSidecarOverride `json:"sidecarOverrides,omitempty"`

	// +optional
	Metadata *PipelineTaskMetadata `json:"metadata,omitempty"`

	// Compute resources to use for this TaskRun
	ComputeResources *corev1.ResourceRequirements `json:"computeResources,omitempty"`
}

// GetTaskRunSpec returns the task specific spec for a given
// PipelineTask if configured, otherwise it returns the PipelineRun's default.
func (pr *PipelineRun) GetTaskRunSpec(pipelineTaskName string) PipelineTaskRunSpec {
	s := PipelineTaskRunSpec{
		PipelineTaskName:       pipelineTaskName,
		TaskServiceAccountName: pr.Spec.ServiceAccountName,
		TaskPodTemplate:        pr.Spec.PodTemplate,
	}
	for _, task := range pr.Spec.TaskRunSpecs {
		if task.PipelineTaskName == pipelineTaskName {
			// merge podTemplates specified in pipelineRun.spec.taskRunSpecs[].podTemplate and pipelineRun.spec.podTemplate
			// with taskRunSpecs taking higher precedence
			s.TaskPodTemplate = pod.MergePodTemplateWithDefault(task.TaskPodTemplate, s.TaskPodTemplate)
			if task.TaskServiceAccountName != "" {
				s.TaskServiceAccountName = task.TaskServiceAccountName
			}
			s.StepOverrides = task.StepOverrides
			s.SidecarOverrides = task.SidecarOverrides
			s.Metadata = task.Metadata
			s.ComputeResources = task.ComputeResources
		}
	}
	return s
}
