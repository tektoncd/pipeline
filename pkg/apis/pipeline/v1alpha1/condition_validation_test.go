/*
 *
 * Copyright 2019 The Tekton Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 * /
 */

package v1alpha1

import (
	"context"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/knative/pkg/apis"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"
)
func TestCondition_Validate(t *testing.T) {
	c := Condition{
		ObjectMeta: metav1.ObjectMeta{
			Name: "taskname",
		},
		Spec: ConditionSpec{
			Check: corev1.Container{
				Name:  "foo",
				Image: "bar",
			},
			Params: []ParamSpec{
				{
					Name: "expected",
				},
			},
		},
	}
	if err := c.Validate(context.Background()); err != nil {
		t.Errorf("Condition.Validate()  unexpected error = %v", err)
	}
}

func TestCondition_Invalidate(t *testing.T) {
	tcs := []struct{
		name string
		cond Condition
		expectedError apis.FieldError
	}{{
		name: "invalid meta",
		cond: Condition{
			ObjectMeta: metav1.ObjectMeta{
				Name : "invalid.,name",
			},
		},
		expectedError: apis.FieldError{
			Message: "Invalid resource name: special character . must not be present",
			Paths: []string{"metadata.name"},
		},
	},{
		name: "no image",
		cond:Condition{
			ObjectMeta: metav1.ObjectMeta{Name: "cond"},
			Spec: ConditionSpec{
				Check: corev1.Container{},
			},
		},
		expectedError: apis.FieldError{
			Message: "missing field(s)",
			Paths: []string{"Spec.Check.Image"},
		},
	}}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cond.Validate(context.Background())
			if err == nil {
				t.Fatalf("Expected an Error, got nothing for %v", tc)
			}
			if d := cmp.Diff(tc.expectedError, *err, cmpopts.IgnoreUnexported(apis.FieldError{})); d != "" {
				t.Errorf("Condition.Validate() errors diff -want, +got: %v", d)
			}
		})
	}
}