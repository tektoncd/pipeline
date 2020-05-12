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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
	duckv1beta1 "knative.dev/pkg/apis/duck/v1beta1"
)

// ConditionCheck represents a single evaluation of a Condition step.
type ConditionCheck TaskRun

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

// ConditionCheckStatus defines the observed state of ConditionCheck
type ConditionCheckStatus struct {
	duckv1beta1.Status `json:",inline"`

	// ConditionCheckStatusFields inlines the status fields.
	ConditionCheckStatusFields `json:",inline"`
}

// ConditionCheckStatusFields holds the fields of ConfigurationCheck's status.
// This is defined separately and inlined so that other types can readily
// consume these fields via duck typing.
type ConditionCheckStatusFields struct {
	// PodName is the name of the pod responsible for executing this condition check.
	PodName string `json:"podName"`

	// StartTime is the time the check is actually started.
	// +optional
	StartTime *metav1.Time `json:"startTime,omitempty"`

	// CompletionTime is the time the check pod completed.
	// +optional
	CompletionTime *metav1.Time `json:"completionTime,omitempty"`

	// Check describes the state of the check container.
	// +optional
	Check corev1.ContainerState `json:"check,omitempty"`
}
