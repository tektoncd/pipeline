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
	builder "github.com/tektoncd/pipeline/test/builder/v1alpha1"
)

// ConditionOp is an operation which modifies a Condition struct.
type ConditionOp = builder.ConditionOp

// ConditionSpecOp is an operation which modifies a ConditionSpec struct.
type ConditionSpecOp = builder.ConditionSpecOp

var (
	// Condition creates a Condition with default values.
	// Any number of Condition modifiers can be passed to transform it.
	Condition = builder.Condition

	// ConditionSpec creates a ConditionSpec with default values.
	// Any number of ConditionSpec modifiers can be passed to transform it.
	ConditionSpec = builder.ConditionSpec

	// ConditionSpecCheck adds a Container, with the specified name and image, to the Condition Spec Check.
	// Any number of Container modifiers can be passed to transform it.
	ConditionSpecCheck = builder.ConditionSpecCheck

	// ConditionParamSpec adds a param, with specified name, to the Spec.
	// Any number of ParamSpec modifiers can be passed to transform it.
	ConditionParamSpec = builder.ConditionParamSpec

	// ConditionResource adds a resource with specified name, and type to the ConditionSpec.
	ConditionResource = builder.ConditionResource
)
