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
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/knative/pkg/apis"
)

func TestValidate(t *testing.T) {
	results := Results{
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
		results *Results
		want    *apis.FieldError
	}{
		{
			name: "invalid task type result",
			results: &Results{
				URL:  "http://www.google.com",
				Type: "wrongtype",
			},
			want: apis.ErrInvalidValue("wrongtype", "spec.results.Type"),
		},
		{
			name: "invalid task type results missing url",
			results: &Results{
				Type: ResultTargetTypeGCS,
				URL:  "",
			},
			want: apis.ErrMissingField("spec.results.URL"),
		},
		{
			name: "invalid task type results bad url",
			results: &Results{
				Type: ResultTargetTypeGCS,
				URL:  "badurl",
			},
			want: apis.ErrInvalidValue("badurl", "spec.results.URL"),
		},
		{
			name: "invalid task type results type",
			results: &Results{
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
