package v1alpha1

import (
	"context"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/kmeta"
)

// +genclient
// +genreconciler:krshapedlogic=false
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// +k8s:openapi-gen=true
type ConcurrencyControl struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`
	// +optional
	Spec ConcurrencySpec `json:"spec"`
}

var _ kmeta.OwnerRefable = (*ConcurrencyControl)(nil)
var _ apis.Validatable = (*ConcurrencyControl)(nil)
var _ apis.Defaultable = (*ConcurrencyControl)(nil)

type ConcurrencySpec struct {
	// +optional
	// +listType=atomic
	Params []v1beta1.ParamSpec `json:"params,omitempty"`
	// +optional
	Key string `json:"key,omitempty"`
	// + optional
	Strategy string `json:"strategy,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type ConcurrencyControlList struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ConcurrencyControl `json:"items"`
}

// SetDefaults sets the defaults on the object.
func (t *ConcurrencyControl) SetDefaults(ctx context.Context) {}

// Validate validates a concurrencycontrol
func (t *ConcurrencyControl) Validate(ctx context.Context) *apis.FieldError {
	return nil
}

// GetGroupVersionKind implements kmeta.OwnerRefable
func (cc *ConcurrencyControl) GetGroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind("ConcurrencyControl")
}
