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
	"knative.dev/pkg/apis"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	tb "github.com/tektoncd/pipeline/test/builder"
)

func TestCondition_Validate(t *testing.T) {
	c := tb.Condition("condname", "foo",
		tb.ConditionSpec(
			tb.ConditionSpecCheck("cname", "ubuntu"),
			tb.ConditionParamSpec("paramname", v1alpha1.ParamTypeString),
		))

	if err := c.Validate(context.Background()); err != nil {
		t.Errorf("Condition.Validate()  unexpected error = %v", err)
	}
}

func TestCondition_Invalidate(t *testing.T) {
	tcs := []struct {
		name          string
		cond          *v1alpha1.Condition
		expectedError apis.FieldError
	}{{
		name: "invalid meta",
		cond: tb.Condition("invalid.,name", ""),
		expectedError: apis.FieldError{
			Message: "Invalid resource name: special character . must not be present",
			Paths:   []string{"metadata.name"},
		},
	}, {
		name: "no image",
		cond: tb.Condition("cond", "foo", tb.ConditionSpec(
			tb.ConditionSpecCheck("", ""),
		)),
		expectedError: apis.FieldError{
			Message: "missing field(s)",
			Paths:   []string{"Spec.Check.Image"},
		},
	}, {
		name: "condition with script and command",
		cond: tb.Condition("cond", "foo",
			tb.ConditionSpec(
				tb.ConditionSpecCheck("cname", "image", tb.Command("exit 0")),
				tb.ConditionSpecCheckScript("echo foo"),
			)),
		expectedError: apis.FieldError{
			Message: "step 0 script cannot be used with command",
			Paths:   []string{"Spec.Check.script"},
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
