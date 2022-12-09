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
	"encoding/json"
	"time"

	"github.com/tektoncd/pipeline/pkg/apis/run/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

// This package contains common definitions needed by v1beta1.CustomRun and v1beta1.PipelineRun.

// +k8s:deepcopy-gen=true

// CustomRunStatus defines the observed state of CustomRun
type CustomRunStatus struct {
	duckv1.Status `json:",inline"`

	// CustomRunStatusFields inlines the status fields.
	CustomRunStatusFields `json:",inline"`
}

// +k8s:deepcopy-gen=true

// CustomRunStatusFields holds the fields of CustomRun's status.  This is defined
// separately and inlined so that other types can readily consume these fields
// via duck typing.
type CustomRunStatusFields struct {
	// StartTime is the time the build is actually started.
	// +optional
	StartTime *metav1.Time `json:"startTime,omitempty"`

	// CompletionTime is the time the build completed.
	// +optional
	CompletionTime *metav1.Time `json:"completionTime,omitempty"`

	// Results reports any output result values to be consumed by later
	// tasks in a pipeline.
	// +optional
	Results []CustomRunResult `json:"results,omitempty"`

	// RetriesStatus contains the history of CustomRunStatus, in case of a retry.
	// +optional
	RetriesStatus []CustomRunStatus `json:"retriesStatus,omitempty"`

	// ExtraFields holds arbitrary fields provided by the custom task
	// controller.
	ExtraFields runtime.RawExtension `json:"extraFields,omitempty"`
}

// CustomRunResult used to describe the results of a task
type CustomRunResult struct {
	// Name the given name
	Name string `json:"name"`
	// Value the given value of the result
	Value string `json:"value"`
}

var customRunCondSet = apis.NewBatchConditionSet()

// GetCondition returns the Condition matching the given type.
func (r *CustomRunStatus) GetCondition(t apis.ConditionType) *apis.Condition {
	return customRunCondSet.Manage(r).GetCondition(t)
}

// InitializeConditions will set all conditions in customRunCondSet to unknown
// and set the started time to the current time
func (r *CustomRunStatus) InitializeConditions() {
	started := false
	if r.StartTime.IsZero() {
		r.StartTime = &metav1.Time{Time: time.Now()}
		started = true
	}
	conditionManager := customRunCondSet.Manage(r)
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
func (r *CustomRunStatus) SetCondition(newCond *apis.Condition) {
	if newCond != nil {
		customRunCondSet.Manage(r).SetCondition(*newCond)
	}
}

// MarkCustomRunSucceeded changes the Succeeded condition to True with the provided reason and message.
func (r *CustomRunStatus) MarkCustomRunSucceeded(reason, messageFormat string, messageA ...interface{}) {
	customRunCondSet.Manage(r).MarkTrueWithReason(apis.ConditionSucceeded, reason, messageFormat, messageA...)
	succeeded := r.GetCondition(apis.ConditionSucceeded)
	r.CompletionTime = &succeeded.LastTransitionTime.Inner
}

// MarkCustomRunFailed changes the Succeeded condition to False with the provided reason and message.
func (r *CustomRunStatus) MarkCustomRunFailed(reason, messageFormat string, messageA ...interface{}) {
	customRunCondSet.Manage(r).MarkFalse(apis.ConditionSucceeded, reason, messageFormat, messageA...)
	succeeded := r.GetCondition(apis.ConditionSucceeded)
	r.CompletionTime = &succeeded.LastTransitionTime.Inner
}

// MarkCustomRunRunning changes the Succeeded condition to Unknown with the provided reason and message.
func (r *CustomRunStatus) MarkCustomRunRunning(reason, messageFormat string, messageA ...interface{}) {
	customRunCondSet.Manage(r).MarkUnknown(apis.ConditionSucceeded, reason, messageFormat, messageA...)
}

// DecodeExtraFields deserializes the extra fields in the CustomRun status.
func (r *CustomRunStatus) DecodeExtraFields(into interface{}) error {
	if len(r.ExtraFields.Raw) == 0 {
		return nil
	}
	return json.Unmarshal(r.ExtraFields.Raw, into)
}

// EncodeExtraFields serializes the extra fields in the CustomRun status.
func (r *CustomRunStatus) EncodeExtraFields(from interface{}) error {
	data, err := json.Marshal(from)
	if err != nil {
		return err
	}
	r.ExtraFields = runtime.RawExtension{
		Raw: data,
	}
	return nil
}

// FromRunStatus converts a v1alpha1.RunStatus into a corresponding v1beta1.CustomRunStatus
func FromRunStatus(orig v1alpha1.RunStatus) CustomRunStatus {
	crs := CustomRunStatus{
		Status: orig.Status,
		CustomRunStatusFields: CustomRunStatusFields{
			StartTime:      orig.StartTime,
			CompletionTime: orig.CompletionTime,
			ExtraFields:    orig.ExtraFields,
		},
	}

	for _, origRes := range orig.Results {
		crs.Results = append(crs.Results, CustomRunResult{
			Name:  origRes.Name,
			Value: origRes.Value,
		})
	}

	for _, origRetryStatus := range orig.RetriesStatus {
		crs.RetriesStatus = append(crs.RetriesStatus, FromRunStatus(origRetryStatus))
	}

	return crs
}
