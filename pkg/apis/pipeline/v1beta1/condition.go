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

package v1beta1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ConditionType indicates the status of the execution of the PipelineRun.
type ConditionType string

const (
	// ConditionTypeValid indicates whether or not the created
	// Resource was found to be valid by the Controller.
	ConditionTypeValid ConditionType = "Valid"

	// ConditionTypeStarted indicates whether or not the Run
	// has started actually executing - applies only to Runs.
	ConditionTypeStarted ConditionType = "Started"

	// ConditionTypeCompleted indicates whether or not the Run
	// has finished executing - applies only to Runs.
	ConditionTypeCompleted ConditionType = "Completed"

	// ConditionTypeSucceeded indicates whether or not the Run
	// was successful - applies only to Runs.
	ConditionTypeSucceeded ConditionType = "Successful"
)

// Condition holds a Condition that the Resource has entered into after being created.
type Condition struct {
	Type ConditionType `json:"type"`

	Status corev1.ConditionStatus `json:"status"`

	LastTransitionTime metav1.Time `json:"lastTransitionTime"`

	Reason  string `json:"reason,omitempty"`
	Message string `json:"message,omitempty"`
}
