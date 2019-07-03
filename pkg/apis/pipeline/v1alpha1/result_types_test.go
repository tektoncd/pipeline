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
	"github.com/knative/pkg/apis"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
)

func TestValidate(t *testing.T) {
	results := v1alpha1.Results{
		URL:  "http://google.com",
		Type: "gcs",
	}
	err := results.Validate(context.Background(), "somepath")
	if err != nil {
		t.Fatalf("Did not expect error when validating valid Results but got %s", err)
	}
}

func TestValidate_Invalid(t *testing.T) {
	tests := []struct {
		name    string
		results *v1alpha1.Results
		want    *apis.FieldError
	}{
		{
			name: "invalid task type result",
			results: &v1alpha1.Results{
				URL:  "http://www.google.com",
				Type: "wrongtype",
			},
			want: apis.ErrInvalidValue("wrongtype", "spec.results.Type"),
		},
		{
			name: "invalid task type results missing url",
			results: &v1alpha1.Results{
				Type: v1alpha1.ResultTargetTypeGCS,
				URL:  "",
			},
			want: apis.ErrMissingField("spec.results.URL"),
		},
		{
			name: "invalid task type results bad url",
			results: &v1alpha1.Results{
				Type: v1alpha1.ResultTargetTypeGCS,
				URL:  "badurl",
			},
			want: apis.ErrInvalidValue("badurl", "spec.results.URL"),
		},
		{
			name: "invalid task type results type",
			results: &v1alpha1.Results{
				Type: "badtype",
				URL:  "http://www.google.com",
			},
			want: apis.ErrInvalidValue("badtype", "spec.results.Type"),
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			path := "spec.results"
			err := tc.results.Validate(context.Background(), path)
			if d := cmp.Diff(err.Error(), tc.want.Error()); d != "" {
				t.Errorf("Results.Validate/%s (-want, +got) = %v", tc.name, d)
			}
		})
	}
}
