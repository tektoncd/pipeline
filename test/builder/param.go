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

import "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"

// ArrayOrString creates an ArrayOrString of type ParamTypeString or ParamTypeArray, based on
// how many inputs are given (>1 input will create an array, not string).
func ArrayOrString(value string, additionalValues ...string) *v1alpha1.ArrayOrString {
	if len(additionalValues) > 0 {
		additionalValues = append([]string{value}, additionalValues...)
		return &v1alpha1.ArrayOrString{
			Type:     v1alpha1.ParamTypeArray,
			ArrayVal: additionalValues,
		}
	}
	return &v1alpha1.ArrayOrString{
		Type:      v1alpha1.ParamTypeString,
		StringVal: value,
	}
}
