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
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
)

// ParamSpecOp is an operation which modify a ParamSpec struct.
type ParamSpecOp func(*v1alpha1.ParamSpec)

// ParamSpecDescription sets the description of a ParamSpec.
func ParamSpecDescription(desc string) ParamSpecOp {
	return func(ps *v1alpha1.ParamSpec) {
		ps.Description = desc
	}
}

// ParamSpecDefault sets the default value of a ParamSpec.
func ParamSpecDefault(value string, additionalValues ...string) ParamSpecOp {
	arrayOrString := v1beta1.NewArrayOrString(value, additionalValues...)
	return func(ps *v1alpha1.ParamSpec) {
		ps.Default = arrayOrString
	}
}
