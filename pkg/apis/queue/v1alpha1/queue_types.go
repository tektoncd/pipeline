/*
 *
 * Copyright 2019 The Tekton Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 * /
 */

package v1alpha1

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/kmeta"
)

type Strategy string

var (
	StrategyCancel           = Strategy("Cancel")
	StrategyGracefullyCancel = Strategy("GracefullyCancel")
	StrategyGracefullyStop   = Strategy("GracefullyStop")
	supportedStrategies      = []Strategy{StrategyCancel, StrategyGracefullyCancel, StrategyGracefullyStop}
)

// +genclient
// +genreconciler:krshapedlogic=false
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +k8s:openapi-gen=true
type Queue struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`
	// +optional
	Spec QueueSpec `json:"spec"`
	//// +optional
	//Status QueueStatus `json:"status,omitempty"`
}

var _ kmeta.OwnerRefable = (*Queue)(nil)
var _ apis.Validatable = (*Queue)(nil)
var _ apis.Defaultable = (*Queue)(nil)

type QueueSpec struct {
	// + optional
	// PipelineConcurrency limits the number of pipeline concurrency allowed by the current queue
	PipelineConcurrency int64 `json:"pipelineConcurrency,omitempty"`
	// + optional
	Strategy string `json:"strategy,omitempty"`
}

// QueueStatus are all the fields in a queue's status subresource.
type QueueStatus struct {
	QueueStatusFields `json:",inline"`
}

// QueueStatusFields are the queue-specific fields
// for the status subresource.
type QueueStatusFields struct {
	RunningPipelines    []string `json:"runningPipelines,omitempty"`
	PipelineConcurrency int64    `json:"pipelineConcurrency,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type QueueList struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Queue `json:"items"`
}

func (q *Queue) SetDefaults(ctx context.Context) {
	if q.Spec.Strategy == "" {
		q.Spec.Strategy = string(StrategyGracefullyCancel)
	}
	if q.Spec.PipelineConcurrency == 0 {
		q.Spec.PipelineConcurrency = 1
	}
}

// Validate validates a queue
func (q *Queue) Validate(ctx context.Context) *apis.FieldError {
	return validateStrategy(q.Spec.Strategy)
}

func validateStrategy(s string) *apis.FieldError {
	for _, supported := range supportedStrategies {
		if s == string(supported) {
			return nil
		}
	}
	return apis.ErrInvalidValue(fmt.Sprintf("got unsupported strategy %s", s), "strategy")
}

// GetGroupVersionKind implements kmeta.OwnerRefable
func (q *Queue) GetGroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind("queue")
}
