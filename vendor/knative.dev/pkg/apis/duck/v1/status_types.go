/*
Copyright 2019 The Knative Authors

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

package v1

import (
	"context"

	"knative.dev/pkg/apis"
	"knative.dev/pkg/apis/duck"
)

// +genduck

// Conditions is a simple wrapper around apis.Conditions to implement duck.Implementable.
type Conditions apis.Conditions

// Conditions is an Implementable "duck type".
var _ duck.Implementable = (*Conditions)(nil)

// Status shows how we expect folks to embed Conditions in
// their Status field.
// WARNING: Adding fields to this struct will add them to all Knative resources.
type Status struct {
	// ObservedGeneration is the 'Generation' of the Service that
	// was last processed by the controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Conditions the latest available observations of a resource's current state.
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	Conditions Conditions `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}

var _ apis.ConditionsAccessor = (*Status)(nil)

// GetConditions implements apis.ConditionsAccessor
func (s *Status) GetConditions() apis.Conditions {
	return apis.Conditions(s.Conditions)
}

// SetConditions implements apis.ConditionsAccessor
func (s *Status) SetConditions(c apis.Conditions) {
	s.Conditions = Conditions(c)
}

// In order for Conditions to be Implementable, KResource must be Populatable.
var _ duck.Populatable = (*KResource)(nil)

// Ensure KResource satisfies apis.Listable
var _ apis.Listable = (*KResource)(nil)

// GetFullType implements duck.Implementable
func (*Conditions) GetFullType() duck.Populatable {
	return &KResource{}
}

// GetCondition fetches the condition of the specified type.
func (s *Status) GetCondition(t apis.ConditionType) *apis.Condition {
	for _, cond := range s.Conditions {
		if cond.Type == t {
			return &cond
		}
	}
	return nil
}

// ConvertTo helps implement apis.Convertible for types embedding this Status.
//
// By default apis.ConditionReady and apis.ConditionSucceeded will be copied over to the
// sink. Other conditions types are tested against a list of predicates. If any of the predicates
// return true the condition type will be copied to the sink
func (source *Status) ConvertTo(ctx context.Context, sink *Status, predicates ...func(apis.ConditionType) bool) {
	sink.ObservedGeneration = source.ObservedGeneration

	conditions := make(apis.Conditions, 0, len(source.Conditions))
	for _, c := range source.Conditions {

		// Copy over the "happy" condition, which is the only condition that
		// we can reliably transfer.
		if c.Type == apis.ConditionReady || c.Type == apis.ConditionSucceeded {
			conditions = append(conditions, c)
			continue
		}

		for _, predicate := range predicates {
			if predicate(c.Type) {
				conditions = append(conditions, c)
				break
			}
		}
	}

	sink.SetConditions(conditions)
}
