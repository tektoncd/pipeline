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

	"github.com/knative/pkg/apis/duck"
)

// Subscribable is the schema for the subscribable portion of the payload
type Subscribable struct {
	// TODO(vaikas): Give me a schema!
	Field string `json:"field,omitempty"`
}

// Implementations can verify that they implement Subscribable via:
var _ = duck.VerifyType(&Topic{}, &Subscribable{})

// Subscribable is an Implementable "duck type".
var _ duck.Implementable = (*Subscribable)(nil)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Topic is a skeleton type wrapping Subscribable in the manner we expect
// resource writers defining compatible resources to embed it.  We will
// typically use this type to deserialize Subscribable ObjectReferences and
// access the Subscribable data.  This is not a real resource.
type Topic struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Status TopicStatus `json:"status"`
}

// TopicStatus shows how we expect folks to embed Subscribable in
// their Status field.
type TopicStatus struct {
	Subscribable *Subscribable `json:"subscribable,omitempty"`
}

// In order for Subscribable to be Implementable, Topic must be Populatable.
var _ duck.Populatable = (*Topic)(nil)

// GetFullType implements duck.Implementable
func (_ *Subscribable) GetFullType() duck.Populatable {
	return &Topic{}
}

// Populate implements duck.Populatable
func (t *Topic) Populate() {
	t.Status.Subscribable = &Subscribable{
		// Populate ALL fields
		Field: "this is not empty",
	}
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// TopicList is a list of Topic resources
type TopicList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Topic `json:"items"`
}
