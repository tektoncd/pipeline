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
	"k8s.io/apimachinery/pkg/runtime"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/apis/duck"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Runnable is a duck type that holds the common fields that are present
// within types that may be Run.
//
// +k8s:openapi-gen=true
type Runnable struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata"`

	// Spec holds the desired state of the Runnable from the client
	// +optional
	Spec RunnableSpec `json:"spec"`
}

// Verify that Runnable implements the appropriate interfaces.
var (
	_ duck.Implementable = (*Runnable)(nil)
	_ duck.Populatable   = (*Runnable)(nil)
	_ apis.Listable      = (*Runnable)(nil)
)

// RunnableSpec defines the partial schema of Runnable resources' spec.
type RunnableSpec struct {
	// Params is a list of input parameters required to run the task. Params
	// must be supplied as inputs in TaskRuns unless they declare a default
	// value.
	// +optional
	Params []ParamSpec `json:"params,omitempty"`

	// Results are values that this Task can output
	Results []TaskResult `json:"results,omitempty"`
}

// GetFullType implements duck.Implementable
func (*Runnable) GetFullType() duck.Populatable {
	return &Runnable{}
}

// Populate implements duck.Populatable
func (t *Runnable) Populate() {
	// Populate data in every field we care about surviving a
	// roundtrip through JSON.
	t.Spec = RunnableSpec{
		Params: []ParamSpec{{
			Name:        "this-is-a-name",
			Type:        ParamTypeString,
			Description: "Something about the parameter",
			Default: &ArrayOrString{
				Type:      ParamTypeString,
				StringVal: "default value",
			},
		}, {
			Name:        "this-is-another-name",
			Type:        ParamTypeArray,
			Description: "Something about the other parameter",
			Default: &ArrayOrString{
				Type:     ParamTypeArray,
				ArrayVal: []string{"default value"},
			},
		}},
		Results: []TaskResult{{
			Name:        "the-result-name",
			Description: "A description of the result",
		}},
	}
}

// GetListType implements apis.Listable
func (*Runnable) GetListType() runtime.Object {
	return &RunnableList{}
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RunnableList is a list of Runnable resources
type RunnableList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Runnable `json:"items"`
}
