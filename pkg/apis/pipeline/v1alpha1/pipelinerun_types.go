/*
Copyright 2018 The Knative Authors.

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
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	"github.com/knative/pkg/webhook"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Assert that TaskRun implements the GenericCRD interface.
var _ webhook.GenericCRD = (*TaskRun)(nil)

// PipelineRunSpec defines the desired state of PipelineRun
type PipelineRunSpec struct {
	PipelineRef           PipelineRef            `json:"pipelineRef"`
	PipelineParamsRef     PipelineParamsRef      `json:"pipelineParamsRef"`
	PipelineTriggerRef    PipelineTriggerRef     `json:"triggerRef"`
	PipelineTaskResources []PipelineTaskResource `json:"resources"`
	Generation            int64                  `json:"generation,omitempty"`
}

// PipelineTaskResource maps Task inputs and outputs to existing PipelineResources by their names.
type PipelineTaskResource struct {
	// Name is the name of the `PipelineTask` for which these PipelineResources are being provided.
	Name string `json:"name"`

	// Inputs is a list containing mapping from the input Resources which the Task has declared it needs
	// and the corresponding Resource instance in the system which should be used.
	Inputs []TaskResourceBinding `json:"inputs"`

	// Outputs is a list containing mapping from the output Resources which the Task has declared it needs
	// and the corresponding Resource instance in the system which should be used.
	Outputs []TaskResourceBinding `json:"outputs"`
}

// TaskResourceBinding is used to bind a PipelineResource to a PipelineResource required for a Task as an input or an output.
type TaskResourceBinding struct {
	// Name is the name of the Task's input that this Resource should be used for.
	Name string `json:"name"`
	// The Resource that should be provided to the Task for the Resource it requires.
	ResourceRef PipelineResourceRef `json:"resourceRef"`
}

// PipelineResourceRef can be used to refer to a specific instance of a Resource
type PipelineResourceRef struct {
	// Name of the referent; More info: http://kubernetes.io/docs/user-guide/identifiers#names
	Name string `json:"name"`
	// API version of the referent
	// +optional
	APIVersion string `json:"apiVersion,omitempty"`
}

// PipelineRef can be used to refer to a specific instance of a Pipeline.
// Copied from CrossVersionObjectReference: https://github.com/kubernetes/kubernetes/blob/169df7434155cbbc22f1532cba8e0a9588e29ad8/pkg/apis/autoscaling/types.go#L64
type PipelineRef struct {
	// Name of the referent; More info: http://kubernetes.io/docs/user-guide/identifiers#names
	Name string `json:"name"`
	// API version of the referent
	// +optional
	APIVersion string `json:"apiVersion,omitempty"`
}

// PipelineParamsRef can be used to refer to a specific instance of a Pipeline.
// Copied from CrossVersionObjectReference: https://github.com/kubernetes/kubernetes/blob/169df7434155cbbc22f1532cba8e0a9588e29ad8/pkg/apis/autoscaling/types.go#L64
type PipelineParamsRef struct {
	// Name of the referent; More info: http://kubernetes.io/docs/user-guide/identifiers#names
	Name string `json:"name"`
	// API version of the referent
	// +optional
	APIVersion string `json:"apiVersion,omitempty"`
}

// PipelineTriggerType indicates the mechanism by which this PipelineRun was created.
type PipelineTriggerType string

const (
	// PipelineTriggerTypeManual indicates that this PipelineRun was invoked manually by a user.
	PipelineTriggerTypeManual PipelineTriggerType = "manual"
)

// PipelineTriggerRef describes what triggered this Pipeline to run. It could be triggered manually,
// or it could have been some kind of external event (not yet designed).
type PipelineTriggerRef struct {
	Type PipelineTriggerType `json:"type"`
	// +optional
	Name string `json:"name,omitempty"`
}

// PipelineRunStatus defines the observed state of PipelineRun
type PipelineRunStatus struct {
	Conditions duckv1alpha1.Conditions `json:"conditions"`
	// map of TaskRun Status with the taskRun name as the key
	//+optional
	TaskRuns map[string]TaskRunStatus `json:"taskRuns,omitempty"`
}

var pipelineRunCondSet = duckv1alpha1.NewBatchConditionSet()

// GetCondition returns the Condition matching the given type.
func (pr *PipelineRunStatus) GetCondition(t duckv1alpha1.ConditionType) *duckv1alpha1.Condition {
	return pipelineRunCondSet.Manage(pr).GetCondition(t)
}

// InitializeConditions will set all conditions in pipelineRunCondSet to unknown for the PipelineRun
func (pr *PipelineRunStatus) InitializeConditions() {
	if pr.TaskRuns == nil {
		pr.TaskRuns = make(map[string]TaskRunStatus)
	}
	pipelineRunCondSet.Manage(pr).InitializeConditions()
}

// SetCondition sets the condition, unsetting previous conditions with the same
// type as necessary.
func (pr *PipelineRunStatus) SetCondition(newCond *duckv1alpha1.Condition) {
	if newCond != nil {
		pipelineRunCondSet.Manage(pr).SetCondition(*newCond)
	}
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PipelineRun is the Schema for the pipelineruns API
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

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PipelineRunList contains a list of PipelineRun
type PipelineRunList struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PipelineRun `json:"items"`
}

// PipelineTaskRun reports the results of running a step in the Task. Each
// task has the potential to succeed or fail (based on the exit code)
// and produces logs.
type PipelineTaskRun struct {
	Name string `json:"name"`
}

// GetTaskRunRef for pipelinerun
func (pr *PipelineRun) GetTaskRunRef() corev1.ObjectReference {
	return corev1.ObjectReference{
		APIVersion: "build-pipeline.knative.dev/v1alpha1",
		Kind:       "TaskRun",
		Namespace:  pr.Namespace,
		Name:       pr.Name,
	}
}

// SetDefaults for pipelinerun
func (pr *PipelineRun) SetDefaults() {}
