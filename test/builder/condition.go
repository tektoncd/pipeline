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

package builder

import (
	v1alpha1 "github.com/tektoncd/pipeline/internal/builder/v1alpha1"
)

// ConditionOp is an operation which modifies a Condition struct.
// Deprecated: moved to internal/builder/v1alpha1
type ConditionOp = v1alpha1.ConditionOp

// ConditionSpecOp is an operation which modifies a ConditionSpec struct.
// Deprecated: moved to internal/builder/v1alpha1
type ConditionSpecOp = v1alpha1.ConditionSpecOp

var (

	// Condition creates a Condition with default values.
	// Any number of Condition modifiers can be passed to transform it.
	// Deprecated: moved to internal/builder/v1alpha1
	Condition = v1alpha1.Condition

	// ConditionNamespace sets the namespace on the condition
	// Deprecated: moved to internal/builder/v1alpha1
	ConditionNamespace = v1alpha1.ConditionNamespace

	// ConditionLabels sets the labels on the condition.
	// Deprecated: moved to internal/builder/v1alpha1
	ConditionLabels = v1alpha1.ConditionLabels

	// ConditionSpec creates a ConditionSpec with default values.
	// Any number of ConditionSpec modifiers can be passed to transform it.
	// Deprecated: moved to internal/builder/v1alpha1
	ConditionSpec = v1alpha1.ConditionSpec

	// ConditionSpecCheck adds a Container, with the specified name and image, to the Condition Spec Check.
	// Any number of Container modifiers can be passed to transform it.
	// Deprecated: moved to internal/builder/v1alpha1
	ConditionSpecCheck = v1alpha1.ConditionSpecCheck

	// ConditionDescription sets the description of the condition
	// Deprecated: moved to internal/builder/v1alpha1
	ConditionDescription = v1alpha1.ConditionDescription

	// ConditionSpecCheckScript adds a script to the Spec.
	// Deprecated: moved to internal/builder/v1alpha1
	ConditionSpecCheckScript = v1alpha1.ConditionSpecCheckScript

	// ConditionParamSpec adds a param, with specified name, to the Spec.
	// Any number of ParamSpec modifiers can be passed to transform it.
	// Deprecated: moved to internal/builder/v1alpha1
	ConditionParamSpec = v1alpha1.ConditionParamSpec

	// ConditionResource adds a resource with specified name, and type to the ConditionSpec.
	// Deprecated: moved to internal/builder/v1alpha1
	ConditionResource = v1alpha1.ConditionResource
)
