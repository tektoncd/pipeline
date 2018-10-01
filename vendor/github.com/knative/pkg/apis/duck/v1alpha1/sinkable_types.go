/*
Copyright 2018 The Knative Authors

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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/knative/pkg/apis"
	"github.com/knative/pkg/apis/duck"
)

// Sinkable is very similar concept as Targetable. However, at the
// transport level they have different contracts and hence Sinkable
// and Targetable are two distinct resources.

// Sinkable is the schema for the sinkable portion of the payload
type Sinkable struct {
	DomainInternal string `json:"domainInternal,omitempty"`
}


// Sinkable is an Implementable "duck type".
var _ duck.Implementable = (*Sinkable)(nil)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Sink is a skeleton type wrapping Sinkable in the manner we expect
// resource writers defining compatible resources to embed it.  We will
// typically use this type to deserialize Sinkable ObjectReferences and
// access the Sinkable data.  This is not a real resource.
type Sink struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Status SinkStatus `json:"status"`
}

// SinkStatus shows how we expect folks to embed Sinkable in
// their Status field.
type SinkStatus struct {
	Sinkable *Sinkable `json:"sinkable,omitempty"`
}

// In order for Sinkable to be Implementable, Sink must be Populatable.
var _ duck.Populatable = (*Sink)(nil)

// Ensure Sink satisfies apis.Listable
var _ apis.Listable = (*Sink)(nil)

// GetFullType implements duck.Implementable
func (_ *Sinkable) GetFullType() duck.Populatable {
	return &Sink{}
}

// Populate implements duck.Populatable
func (t *Sink) Populate() {
	t.Status = SinkStatus{
		&Sinkable{
			// Populate ALL fields
			DomainInternal: "this is not empty",
		},
	}
}

// GetListType implements apis.Listable
func (r *Sink) GetListType() runtime.Object {
	return &SinkList{}
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// SinkList is a list of Sink resources
type SinkList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Sink `json:"items"`
}
