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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
)

// RunsToCompletionStatus is implemented by TaskRun.Status and PipelineRun.Status
type RunsToCompletionStatus interface {
	GetCondition(t apis.ConditionType) *apis.Condition
	InitializeConditions()
	SetCondition(newCond *apis.Condition)
}

// RunsToCompletion is implemented by TaskRun and PipelineRun
type RunsToCompletion interface {
	GetTypeMeta() *metav1.TypeMeta
	GetObjectMeta() *metav1.ObjectMeta
	GetOwnerReference() metav1.OwnerReference
	GetStatus() RunsToCompletionStatus
	IsDone() bool
	HasStarted() bool
	IsCancelled() bool
	HasTimedOut() bool
	GetRunKey() string
}
