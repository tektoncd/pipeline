/*
Copyright 2018 The Knative Authors.

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
	"reflect"
	"sort"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"knative.dev/pkg/apis"
	"knative.dev/pkg/kmeta"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Image is a Knative abstraction that encapsulates the interface by which Knative
// components express a desire to have a particular image cached.
type Image struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec holds the desired state of the Image (from the client).
	// +optional
	Spec ImageSpec `json:"spec,omitempty"`

	// Status communicates the observed state of the Image (from the controller).
	// +optional
	Status ImageStatus `json:"status,omitempty"`
}

// Check that Image can be validated and defaulted.
var _ apis.Validatable = (*Image)(nil)
var _ apis.Defaultable = (*Image)(nil)
var _ kmeta.OwnerRefable = (*Image)(nil)

// ImageSpec holds the desired state of the Image (from the client).
type ImageSpec struct {

	// Image is the name of the container image url to cache across the cluster.
	Image string `json:"image"`

	// ServiceAccountName is the name of the Kubernetes ServiceAccount as which the Pods
	// will run this container.  This is potentially used to authenticate the image pull
	// if the service account has attached pull secrets.  For more information:
	// https://kubernetes.io/docs/tasks/configure-pod-container/configure-service-account/#add-imagepullsecrets-to-a-service-account
	// +optional
	ServiceAccountName string `json:"serviceAccountName,omitempty"`

	// ImagePullSecrets contains the names of the Kubernetes Secrets containing login
	// information used by the Pods which will run this container.
	// +optional
	ImagePullSecrets []corev1.LocalObjectReference `json:"imagePullSecrets,omitempty"`
}

// ImageConditionType is used to communicate the status of the reconciliation process.
type ImageConditionType string

const (
	// ImageConditionReady is set when the revision is starting to materialize
	// runtime resources, and becomes true when those resources are ready.
	ImageConditionReady ImageConditionType = "Ready"
)

// ImageCondition defines a readiness condition for a Image.
// See: https://github.com/kubernetes/community/blob/master/contributors/devel/api-conventions.md#typical-status-properties
type ImageCondition struct {
	Type ImageConditionType `json:"type" description:"type of Image condition"`

	Status corev1.ConditionStatus `json:"status" description:"status of the condition, one of True, False, Unknown"`

	// +optional
	// We use VolatileTime in place of metav1.Time to exclude this from creating equality.Semantic
	// differences (all other things held constant).
	LastTransitionTime apis.VolatileTime `json:"lastTransitionTime,omitempty" description:"last time the condition transit from one status to another"`

	// +optional
	Reason string `json:"reason,omitempty" description:"one-word CamelCase reason for the condition's last transition"`

	// +optional
	Message string `json:"message,omitempty" description:"human-readable message indicating details about last transition"`
}

// ImageStatus communicates the observed state of the Image (from the controller).
type ImageStatus struct {
	// Conditions communicates information about ongoing/complete
	// reconciliation processes that bring the "spec" inline with the observed
	// state of the world.
	// +optional
	Conditions []ImageCondition `json:"conditions,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ImageList is a list of Image resources
type ImageList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Image `json:"items"`
}

// IsReady looks at the conditions and if the Status has a condition
// ImageConditionReady returns true if ConditionStatus is True
func (rs *ImageStatus) IsReady() bool {
	if c := rs.GetCondition(ImageConditionReady); c != nil {
		return c.Status == corev1.ConditionTrue
	}
	return false
}

func (rs *ImageStatus) GetCondition(t ImageConditionType) *ImageCondition {
	for _, cond := range rs.Conditions {
		if cond.Type == t {
			return &cond
		}
	}
	return nil
}

func (rs *ImageStatus) SetCondition(new *ImageCondition) {
	if new == nil {
		return
	}

	t := new.Type
	var conditions []ImageCondition
	for _, cond := range rs.Conditions {
		if cond.Type != t {
			conditions = append(conditions, cond)
		} else {
			// If we'd only update the LastTransitionTime, then return.
			new.LastTransitionTime = cond.LastTransitionTime
			if reflect.DeepEqual(new, &cond) {
				return
			}
		}
	}
	new.LastTransitionTime = apis.VolatileTime{metav1.NewTime(time.Now())}
	conditions = append(conditions, *new)
	// Deterministically order the conditions
	sort.Slice(conditions, func(i, j int) bool { return conditions[i].Type < conditions[j].Type })
	rs.Conditions = conditions
}

func (rs *ImageStatus) InitializeConditions() {
	for _, cond := range []ImageConditionType{
		ImageConditionReady,
	} {
		if rc := rs.GetCondition(cond); rc == nil {
			rs.SetCondition(&ImageCondition{
				Type:   cond,
				Status: corev1.ConditionUnknown,
			})
		}
	}
}

func (i *Image) GetGroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind("Image")
}
