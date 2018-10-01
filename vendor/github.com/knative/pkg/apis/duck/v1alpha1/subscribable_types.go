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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/knative/pkg/apis"
	"github.com/knative/pkg/apis/duck"
)

// Subscribable is the schema for the subscribable portion of the payload.
// It is a reference to actual object that implements Channelable duck
// type.
type Subscribable struct {
	// Channelable is a reference to the actual resource
	// that provides the ability to perform Subscription capabilities.
	// This may point to object itself (for example Channel) or to another
	// object providing the actual capabilities..
	Channelable corev1.ObjectReference `json:"channelable,omitempty"`
}


// Subscribable is an Implementable "duck type".
var _ duck.Implementable = (*Subscribable)(nil)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Subscription is a skeleton type wrapping the notion that this object
// can be subscribed to. SubscriptionStatus provides the reference
// (in a form of Subscribable) to the object that you can actually create
// a subscription to.
// We will typically use this type to deserialize Subscription objects
// to access the Subscripion data.  This is not a real resource.
type Subscription struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// SubscriptionStatus is the part of the Status where a Subscribable
	// object points to the underlying Channelable object that fullfills
	// the SubscribableSpec contract. Note that this can be a self-link
	// for example for concrete Channel implementations.
	Status SubscriptionStatus `json:"status"`
}

// SubscriptionStatus shows how we expect folks to embed Subscribable in
// their Status field.
type SubscriptionStatus struct {
	Subscribable *Subscribable `json:"subscribable,omitempty"`
}

// In order for Subscribable to be Implementable, Subscribable must be Populatable.
var _ duck.Populatable = (*Subscription)(nil)

// Ensure Subscription satisfies apis.Listable
var _ apis.Listable = (*Subscription)(nil)

// GetFullType implements duck.Implementable
func (_ *Subscribable) GetFullType() duck.Populatable {
	return &Subscription{}
}

// Populate implements duck.Populatable
func (t *Subscription) Populate() {
	t.Status.Subscribable = &Subscribable{
		// Populate ALL fields
		Channelable: corev1.ObjectReference{
			Name:       "placeholdername",
			APIVersion: "apiversionhere",
			Kind:       "ChannelKindHere",
		},
	}
}

// GetListType implements apis.Listable
func (r *Subscription) GetListType() runtime.Object {
	return &SubscriptionList{}
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// SubscribableList is a list of Subscribable resources
type SubscriptionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Subscription `json:"items"`
}
