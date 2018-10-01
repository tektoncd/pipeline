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

// Channelable is the schema for the channelable portion of the spec
// section of the resource.
type Channelable struct {
	// TODO: What is actually required here for Channel spec.
	// This is the list of subscriptions for this channel.
	Subscribers []ChannelSubscriberSpec `json:"subscribers,omitempty"`
}

// ChannelSubscriberSpec defines a single subscriber to a Channel.
// CallableDomain is the endpoint for the call
// SinkableDomain is the endpoint for the result
// One of them must be present
type ChannelSubscriberSpec struct {
	// +optional
	CallableDomain string `json:"callableDomain,omitempty"`
	// +optional
	SinkableDomain string `json:"sinkableDomain,omitempty"`
}


// Channelable is an Implementable "duck type".
var _ duck.Implementable = (*Channelable)(nil)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Channel is a skeleton type wrapping Channelable in the manner we expect
// resource writers defining compatible resources to embed it.  We will
// typically use this type to deserialize Channelable ObjectReferences and
// access the Channelable data.  This is not a real resource.
type Channel struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// ChannelSpec is the part where Channelable object is
	// configured as to be compatible with Channelable contract.
	Spec ChannelSpec `json:"spec"`
}

// ChannelSpec shows how we expect folks to embed Channelable in
// their Spec field.
type ChannelSpec struct {
	Channelable *Channelable `json:"channelable,omitempty"`
}

// In order for Channelable to be Implementable, Channel must be Populatable.
var _ duck.Populatable = (*Channel)(nil)

// Ensure Channel satisfies apis.Listable
var _ apis.Listable = (*Channel)(nil)

// GetFullType implements duck.Implementable
func (_ *Channelable) GetFullType() duck.Populatable {
	return &Channel{}
}

// Populate implements duck.Populatable
func (t *Channel) Populate() {
	t.Spec.Channelable = &Channelable{
		// Populate ALL fields
		Subscribers: []ChannelSubscriberSpec{{"call1", "sink2"}, {"call2", "sink2"}},
	}
}

// GetListType implements apis.Listable
func (r *Channel) GetListType() runtime.Object {
	return &ChannelList{}
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ChannelList is a list of Channel resources
type ChannelList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Channel `json:"items"`
}
