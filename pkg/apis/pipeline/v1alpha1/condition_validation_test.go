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

package v1alpha1_test

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/test/diff"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
)

func TestCondition_Validate(t *testing.T) {
	c := v1alpha1.Condition{
		ObjectMeta: metav1.ObjectMeta{Name: "condname"},
		Spec: v1alpha1.ConditionSpec{
			Check: v1alpha1.Step{Container: corev1.Container{
				Name:  "cname",
				Image: "ubuntu",
			}},
			Params: []v1alpha1.ParamSpec{{
				Name: "paramname",
				Type: v1alpha1.ParamTypeString,
			}},
		},
	}
	if err := c.Validate(context.Background()); err != nil {
		t.Errorf("Condition.Validate()  unexpected error = %v", err)
	}
}

func TestCondition_Invalid(t *testing.T) {
	for _, tc := range []struct {
		name          string
		cond          *v1alpha1.Condition
		expectedError apis.FieldError
	}{{
		name: "invalid meta",
		cond: &v1alpha1.Condition{
			ObjectMeta: metav1.ObjectMeta{Name: "invalid.,name"},
		},
		expectedError: apis.FieldError{
			Message: "Invalid resource name: special character . must not be present",
			Paths:   []string{"metadata.name"},
		},
	}, {
		name: "no image",
		cond: &v1alpha1.Condition{
			ObjectMeta: metav1.ObjectMeta{Name: "condname"},
			Spec: v1alpha1.ConditionSpec{
				Check: v1alpha1.Step{Container: corev1.Container{
					Image: "",
				}},
			},
		},
		expectedError: apis.FieldError{
			Message: "missing field(s)",
			Paths:   []string{"Spec.Check.Image"},
		},
	}, {
		name: "condition with script and command",
		cond: &v1alpha1.Condition{
			ObjectMeta: metav1.ObjectMeta{Name: "condname"},
			Spec: v1alpha1.ConditionSpec{
				Check: v1alpha1.Step{
					Container: corev1.Container{
						Image:   "image",
						Command: []string{"exit", "0"},
					},
					Script: "echo foo",
				},
			},
		},
		expectedError: apis.FieldError{
			Message: "step 0 script cannot be used with command",
			Paths:   []string{"Spec.Check.script"},
		},
	}, {
		name: "condition with invalid check name",
		cond: &v1alpha1.Condition{
			ObjectMeta: metav1.ObjectMeta{Name: "condname"},
			Spec: v1alpha1.ConditionSpec{
				Check: v1alpha1.Step{
					Container: corev1.Container{
						Name:  "Cname",
						Image: "image",
					},
				},
			},
		},
		expectedError: apis.FieldError{
			Message: "invalid value \"Cname\"",
			Paths:   []string{"Spec.Check.name"},
			Details: "Condition check name must be a valid DNS Label, For more info refer to https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#names",
		},
	}} {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cond.Validate(context.Background())
			if err == nil {
				t.Fatalf("Expected an Error, got nothing for %v", tc)
			}
			if d := cmp.Diff(tc.expectedError, *err, cmpopts.IgnoreUnexported(apis.FieldError{})); d != "" {
				t.Errorf("Condition.Validate() errors diff %s", diff.PrintWantGot(d))
			}
		})
	}
}
