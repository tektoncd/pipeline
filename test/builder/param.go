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

// ParamSpecOp is an operation which modify a ParamSpec struct.
type ParamSpecOp = builder.ParamSpecOp

var (
	// ArrayOrString creates an ArrayOrString of type ParamTypeString or ParamTypeArray, based on
	// how many inputs are given (>1 input will create an array, not string).
	ArrayOrString = builder.ArrayOrString

	// ParamSpecDescription sets the description of a ParamSpec.
	ParamSpecDescription = builder.ParamSpecDescription

	// ParamSpecDefault sets the default value of a ParamSpec.
	ParamSpecDefault = builder.ParamSpecDefault
)
