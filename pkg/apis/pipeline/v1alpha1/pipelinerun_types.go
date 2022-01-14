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
	"fmt"

	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/pkg/clock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"knative.dev/pkg/apis"
)

// +genclient
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

// GetName returns the PipelineRun's name
func (pr *PipelineRun) GetName() string {
	return pr.ObjectMeta.GetName()
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
	// TaskRunSpecs holds a set of task specific specs
	// +optional
	TaskRunSpecs []PipelineTaskRunSpec `json:"taskRunSpecs,omitempty"`
}

// PipelineRunSpecStatus defines the pipelinerun spec status the user can provide
type PipelineRunSpecStatus = v1beta1.PipelineRunSpecStatus

const (
	// PipelineRunSpecStatusCancelled indicates that the user wants to cancel the task,
	// if not already cancelled or terminated
	PipelineRunSpecStatusCancelled = v1beta1.PipelineRunSpecStatusCancelledDeprecated
)

// PipelineResourceRef can be used to refer to a specific instance of a Resource
type PipelineResourceRef = v1beta1.PipelineResourceRef

// PipelineRef can be used to refer to a specific instance of a Pipeline.
// Copied from CrossVersionObjectReference: https://github.com/kubernetes/kubernetes/blob/169df7434155cbbc22f1532cba8e0a9588e29ad8/pkg/apis/autoscaling/types.go#L64
type PipelineRef = v1beta1.PipelineRef

// PipelineRunStatus defines the observed state of PipelineRun
type PipelineRunStatus = v1beta1.PipelineRunStatus

// PipelineRunStatusFields holds the fields of PipelineRunStatus' status.
// This is defined separately and inlined so that other types can readily
// consume these fields via duck typing.
type PipelineRunStatusFields = v1beta1.PipelineRunStatusFields

// PipelineRunTaskRunStatus contains the name of the PipelineTask for this TaskRun and the TaskRun's Status
type PipelineRunTaskRunStatus = v1beta1.PipelineRunTaskRunStatus

// PipelineRunSpecServiceAccountName can be used to configure specific
// ServiceAccountName for a concrete Task
type PipelineRunSpecServiceAccountName = v1beta1.PipelineRunSpecServiceAccountName

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
type PipelineTaskRun = v1beta1.PipelineTaskRun

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

// GetRunKey return the pipelinerun key for timeout handler map
func (pr *PipelineRun) GetRunKey() string {
	// The address of the pointer is a threadsafe unique identifier for the pipelinerun
	return fmt.Sprintf("%s/%p", pipeline.PipelineRunControllerName, pr)
}

// IsTimedOut returns true if a pipelinerun has exceeded its spec.Timeout based on its status.Timeout
func (pr *PipelineRun) IsTimedOut(c clock.Clock) bool {
	pipelineTimeout := pr.Spec.Timeout
	startTime := pr.Status.StartTime

	if !startTime.IsZero() && pipelineTimeout != nil {
		timeout := pipelineTimeout.Duration
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

// PipelineTaskRunSpec holds task specific specs
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
