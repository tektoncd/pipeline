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
	"fmt"
	"time"

	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"knative.dev/pkg/apis"
	duckv1beta1 "knative.dev/pkg/apis/duck/v1beta1"
)

var (
	groupVersionKind = schema.GroupVersionKind{
		Group:   SchemeGroupVersion.Group,
		Version: SchemeGroupVersion.Version,
		Kind:    pipeline.PipelineRunControllerName,
	}
)

// +genclient
// +genreconciler
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

func (pr *PipelineRun) GetName() string {
	return pr.ObjectMeta.GetName()
}

// GetTaskRunRef for pipelinerun
func (pr *PipelineRun) GetTaskRunRef() corev1.ObjectReference {
	return corev1.ObjectReference{
		APIVersion: SchemeGroupVersion.String(),
		Kind:       "TaskRun",
		Namespace:  pr.Namespace,
		Name:       pr.Name,
	}
}

// GetStatusCondition returns the task run status as a ConditionAccessor
func (pr *PipelineRun) GetStatusCondition() apis.ConditionAccessor {
	return &pr.Status
}

// GetOwnerReference gets the pipeline run as owner reference for any related objects
func (pr *PipelineRun) GetOwnerReference() metav1.OwnerReference {
	return *metav1.NewControllerRef(pr, groupVersionKind)
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

// GetRunKey return the pipelinerun key for timeout handler map
func (pr *PipelineRun) GetRunKey() string {
	// The address of the pointer is a threadsafe unique identifier for the pipelinerun
	return fmt.Sprintf("%s/%p", pipeline.PipelineRunControllerName, pr)
}

// IsTimedOut returns true if a pipelinerun has exceeded its spec.Timeout based on its status.Timeout
func (pr *PipelineRun) IsTimedOut() bool {
	return pr.HasTimedOut()
}

// HasTimedOut returns true if a pipelinerun has exceeded its spec.Timeout based on its status.Timeout
func (pr *PipelineRun) HasTimedOut() bool {
	pipelineTimeout := pr.Spec.Timeout
	startTime := pr.Status.StartTime

	if !startTime.IsZero() && pipelineTimeout != nil {
		timeout := pipelineTimeout.Duration
		if timeout == config.NoTimeoutDuration {
			return false
		}
		runtime := time.Since(startTime.Time)
		if runtime > timeout {
			return true
		}
	}
	return false
}

// GetServiceAccountName returns the service account name for a given
// PipelineTask if configured, otherwise it returns the PipelineRun's serviceAccountName.
func (pr *PipelineRun) GetServiceAccountName(pipelineTaskName string) string {
	serviceAccountName := pr.Spec.ServiceAccountName
	for _, sa := range pr.Spec.ServiceAccountNames {
		if sa.TaskName == pipelineTaskName {
			serviceAccountName = sa.ServiceAccountName
		}
	}
	return serviceAccountName
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
	Resources []PipelineResourceBinding `json:"resources,omitempty"`
	// Params is a list of parameter names and values.
	Params []Param `json:"params,omitempty"`
	// +optional
	ServiceAccountName string `json:"serviceAccountName,omitempty"`
	// +optional
	ServiceAccountNames []PipelineRunSpecServiceAccountName `json:"serviceAccountNames,omitempty"`
	// Used for cancelling a pipelinerun (and maybe more later on)
	// +optional
	Status PipelineRunSpecStatus `json:"status,omitempty"`
	// Time after which the Pipeline times out. Defaults to never.
	// Refer to Go's ParseDuration documentation for expected format: https://golang.org/pkg/time/#ParseDuration
	// +optional
	Timeout *metav1.Duration `json:"timeout,omitempty"`
	// PodTemplate holds pod specific configuration
	PodTemplate *PodTemplate `json:"podTemplate,omitempty"`
	// Workspaces holds a set of workspace bindings that must match names
	// with those declared in the pipeline.
	// +optional
	Workspaces []WorkspaceBinding `json:"workspaces,omitempty"`
	// TaskRunSpecs holds a set of runtime specs
	// +optional
	TaskRunSpecs []PipelineTaskRunSpec `json:"taskRunSpecs,omitempty"`
}

// PipelineRunSpecStatus defines the pipelinerun spec status the user can provide
type PipelineRunSpecStatus string

const (
	// PipelineRunSpecStatusCancelled indicates that the user wants to cancel the task,
	// if not already cancelled or terminated
	PipelineRunSpecStatusCancelled = "PipelineRunCancelled"
)

// PipelineRef can be used to refer to a specific instance of a Pipeline.
// Copied from CrossVersionObjectReference: https://github.com/kubernetes/kubernetes/blob/169df7434155cbbc22f1532cba8e0a9588e29ad8/pkg/apis/autoscaling/types.go#L64
type PipelineRef struct {
	// Name of the referent; More info: http://kubernetes.io/docs/user-guide/identifiers#names
	Name string `json:"name,omitempty"`
	// API version of the referent
	// +optional
	APIVersion string `json:"apiVersion,omitempty"`
}

// PipelineRunStatus defines the observed state of PipelineRun
type PipelineRunStatus struct {
	duckv1beta1.Status `json:",inline"`

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
	// PipelineRunReasonTimedOut is the reason set when the PipelineRun has timed out
	PipelineRunReasonTimedOut PipelineRunReason = "PipelineRunTimeout"
	// ReasonStopping indicates that no new Tasks will be scheduled by the controller, and the
	// pipeline will stop once all running tasks complete their work
	PipelineRunReasonStopping PipelineRunReason = "PipelineRunStopping"
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
func (pr *PipelineRunStatus) InitializeConditions() {
	started := false
	if pr.TaskRuns == nil {
		pr.TaskRuns = make(map[string]*PipelineRunTaskRunStatus)
	}
	if pr.StartTime.IsZero() {
		pr.StartTime = &metav1.Time{Time: time.Now()}
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

// MarkResourceNotConvertible adds a Warning-severity condition to the resource noting
// that it cannot be converted to a higher version.
func (pr *PipelineRunStatus) MarkResourceNotConvertible(err *CannotConvertError) {
	pipelineRunCondSet.Manage(pr).SetCondition(apis.Condition{
		Type:     ConditionTypeConvertible,
		Status:   corev1.ConditionFalse,
		Severity: apis.ConditionSeverityWarning,
		Reason:   err.Field,
		Message:  err.Message,
	})
}

// PipelineRunStatusFields holds the fields of PipelineRunStatus' status.
// This is defined separately and inlined so that other types can readily
// consume these fields via duck typing.
type PipelineRunStatusFields struct {
	// StartTime is the time the PipelineRun is actually started.
	// +optional
	StartTime *metav1.Time `json:"startTime,omitempty"`

	// CompletionTime is the time the PipelineRun completed.
	// +optional
	CompletionTime *metav1.Time `json:"completionTime,omitempty"`

	// map of PipelineRunTaskRunStatus with the taskRun name as the key
	// +optional
	TaskRuns map[string]*PipelineRunTaskRunStatus `json:"taskRuns,omitempty"`

	// PipelineResults are the list of results written out by the pipeline task's containers
	// +optional
	PipelineResults []PipelineRunResult `json:"pipelineResults,omitempty"`

	// PipelineRunSpec contains the exact spec used to instantiate the run
	PipelineSpec *PipelineSpec `json:"pipelineSpec,omitempty"`
}

// PipelineRunResult used to describe the results of a pipeline
type PipelineRunResult struct {
	// Name is the result's name as declared by the Pipeline
	Name string `json:"name"`

	// Value is the result returned from the execution of this PipelineRun
	Value string `json:"value"`
}

// PipelineRunTaskRunStatus contains the name of the PipelineTask for this TaskRun and the TaskRun's Status
type PipelineRunTaskRunStatus struct {
	// PipelineTaskName is the name of the PipelineTask.
	PipelineTaskName string `json:"pipelineTaskName,omitempty"`
	// Status is the TaskRunStatus for the corresponding TaskRun
	// +optional
	Status *TaskRunStatus `json:"status,omitempty"`
	// ConditionChecks maps the name of a condition check to its Status
	// +optional
	ConditionChecks map[string]*PipelineRunConditionCheckStatus `json:"conditionChecks,omitempty"`
}

// PipelineRunConditionCheckStatus returns the condition check status
type PipelineRunConditionCheckStatus struct {
	// ConditionName is the name of the Condition
	ConditionName string `json:"conditionName,omitempty"`
	// Status is the ConditionCheckStatus for the corresponding ConditionCheck
	// +optional
	Status *ConditionCheckStatus `json:"status,omitempty"`
}

// PipelineRunSpecServiceAccountName can be used to configure specific
// ServiceAccountName for a concrete Task
type PipelineRunSpecServiceAccountName struct {
	TaskName           string `json:"taskName,omitempty"`
	ServiceAccountName string `json:"serviceAccountName,omitempty"`
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
	PipelineTaskName       string       `json:"pipelineTaskName,omitempty"`
	TaskServiceAccountName string       `json:"taskServiceAccountName,omitempty"`
	TaskPodTemplate        *PodTemplate `json:"taskPodTemplate,omitempty"`
}

// GetTaskRunSpecs returns the task specific spec for a given
// PipelineTask if configured, otherwise it returns the PipelineRun's default.
func (pr *PipelineRun) GetTaskRunSpecs(pipelineTaskName string) (string, *PodTemplate) {
	serviceAccountName := pr.GetServiceAccountName(pipelineTaskName)
	taskPodTemplate := pr.Spec.PodTemplate
	for _, task := range pr.Spec.TaskRunSpecs {
		if task.PipelineTaskName == pipelineTaskName {
			taskPodTemplate = task.TaskPodTemplate
			serviceAccountName = task.TaskServiceAccountName
		}
	}
	return serviceAccountName, taskPodTemplate
}
