/*
Copyright 2017 The Knative Authors

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

package testing

import (
	"context"
	"fmt"

	"github.com/knative/pkg/apis"
	"github.com/knative/pkg/kmp"

	authenticationv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Resource is a simple resource that's compatible with our webhook
type Resource struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ResourceSpec `json:"spec,omitempty"`
}

const (
	// CreatorAnnotation is the annotation that denotes the user that created the resource.
	CreatorAnnotation = "testing.knative.dev/creator"
	// UpdaterAnnotation is the annotation that denotes the user that last updated the resource.
	UpdaterAnnotation = "testing.knative.dev/updater"
)

// Check that Resource may be validated and defaulted.
var _ apis.Validatable = (*Resource)(nil)
var _ apis.Defaultable = (*Resource)(nil)
var _ apis.Immutable = (*Resource)(nil)
var _ apis.Listable = (*Resource)(nil)

// ResourceSpec represents test resource spec.
type ResourceSpec struct {
	FieldWithDefault               string `json:"fieldWithDefault,omitempty"`
	FieldWithContextDefault        string `json:"fieldWithContextDefault,omitempty"`
	FieldWithValidation            string `json:"fieldWithValidation,omitempty"`
	FieldThatsImmutable            string `json:"fieldThatsImmutable,omitempty"`
	FieldThatsImmutableWithDefault string `json:"fieldThatsImmutableWithDefault,omitempty"`
}

// SetDefaults sets the defaults on the object.
func (c *Resource) SetDefaults(ctx context.Context) {
	c.Spec.SetDefaults(ctx)

	if apis.IsInUpdate(ctx) {
		old := apis.GetBaseline(ctx).(*Resource)
		c.AnnotateUserInfo(ctx, old, apis.GetUserInfo(ctx))
	} else {
		c.AnnotateUserInfo(ctx, nil, apis.GetUserInfo(ctx))
	}
}

// AnnotateUserInfo satisfies the Annotatable interface.
func (c *Resource) AnnotateUserInfo(ctx context.Context, prev *Resource, ui *authenticationv1.UserInfo) {
	a := c.ObjectMeta.GetAnnotations()
	if a == nil {
		a = map[string]string{}
	}
	userName := ui.Username

	// If previous is nil (i.e. this is `Create` operation),
	// then we set both fields.
	// Otherwise copy creator from the previous state.
	if prev == nil {
		a[CreatorAnnotation] = userName
	} else {
		// No spec update ==> bail out.
		if ok, _ := kmp.SafeEqual(prev.Spec, c.Spec); ok {
			if prev.ObjectMeta.GetAnnotations() != nil {
				a[CreatorAnnotation] = prev.ObjectMeta.GetAnnotations()[CreatorAnnotation]
				userName = prev.ObjectMeta.GetAnnotations()[UpdaterAnnotation]
			}
		} else {
			if prev.ObjectMeta.GetAnnotations() != nil {
				a[CreatorAnnotation] = prev.ObjectMeta.GetAnnotations()[CreatorAnnotation]
			}
		}
	}
	// Regardless of `old` set the updater.
	a[UpdaterAnnotation] = userName
	c.ObjectMeta.SetAnnotations(a)
}

func (c *Resource) Validate(ctx context.Context) *apis.FieldError {
	err := c.Spec.Validate(ctx).ViaField("spec")

	if apis.IsInUpdate(ctx) {
		original := apis.GetBaseline(ctx).(*Resource)
		err = err.Also(c.CheckImmutableFields(ctx, original))
	}
	return err
}

type onContextKey struct{}

// WithValue returns a WithContext for attaching an OnContext with the given value.
func WithValue(ctx context.Context, val string) context.Context {
	return context.WithValue(ctx, onContextKey{}, &OnContext{Value: val})
}

// OnContext is a struct for holding a value attached to a context.
type OnContext struct {
	Value string
}

// SetDefaults sets the defaults on the spec.
func (cs *ResourceSpec) SetDefaults(ctx context.Context) {
	if cs.FieldWithDefault == "" {
		cs.FieldWithDefault = "I'm a default."
	}
	if cs.FieldWithContextDefault == "" {
		oc, ok := ctx.Value(onContextKey{}).(*OnContext)
		if ok {
			cs.FieldWithContextDefault = oc.Value
		}
	}
	if cs.FieldThatsImmutableWithDefault == "" {
		cs.FieldThatsImmutableWithDefault = "this is another default value"
	}
}

func (cs *ResourceSpec) Validate(ctx context.Context) *apis.FieldError {
	if cs.FieldWithValidation != "magic value" {
		return apis.ErrInvalidValue(cs.FieldWithValidation, "fieldWithValidation")
	}
	return nil
}

func (current *Resource) CheckImmutableFields(ctx context.Context, og apis.Immutable) *apis.FieldError {
	original, ok := og.(*Resource)
	if !ok {
		return &apis.FieldError{Message: "The provided original was not a Resource"}
	}

	if original.Spec.FieldThatsImmutable != current.Spec.FieldThatsImmutable {
		return &apis.FieldError{
			Message: "Immutable field changed",
			Paths:   []string{"spec.fieldThatsImmutable"},
			Details: fmt.Sprintf("got: %v, want: %v", current.Spec.FieldThatsImmutable,
				original.Spec.FieldThatsImmutable),
		}
	}

	if original.Spec.FieldThatsImmutableWithDefault != current.Spec.FieldThatsImmutableWithDefault {
		return &apis.FieldError{
			Message: "Immutable field changed",
			Paths:   []string{"spec.fieldThatsImmutableWithDefault"},
			Details: fmt.Sprintf("got: %v, want: %v", current.Spec.FieldThatsImmutableWithDefault,
				original.Spec.FieldThatsImmutableWithDefault),
		}
	}
	return nil
}

// GetListType implements apis.Listable
func (r *Resource) GetListType() runtime.Object {
	return &ResourceList{}
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ResourceList is a list of Resource resources
type ResourceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Resource `json:"items"`
}
