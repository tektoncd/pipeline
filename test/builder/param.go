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

// ParamSpecOp is an operation which modify a ParamSpec struct.
type ParamSpecOp = v1alpha1.ParamSpecOp

var (
	// ArrayOrString creates an ArrayOrString of type ParamTypeString or ParamTypeArray, based on
	// how many inputs are given (>1 input will create an array, not string).
	// Deprecated: moved to internal/builder/v1alpha1
	ArrayOrString = v1alpha1.ArrayOrString

	// ParamSpecDescription sets the description of a ParamSpec.
	// Deprecated: moved to internal/builder/v1alpha1
	ParamSpecDescription = v1alpha1.ParamSpecDescription

	// ParamSpecDefault sets the default value of a ParamSpec.
	// Deprecated: moved to internal/builder/v1alpha1
	ParamSpecDefault = v1alpha1.ParamSpecDefault
)
