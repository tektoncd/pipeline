/*
Copyright 2026 The Tekton Authors

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

package v1beta1_test

import (
	"testing"

	v1beta1 "github.com/tektoncd/pipeline/pkg/apis/resolution/v1beta1"
)

func TestIsResolved(t *testing.T) {
	tests := []struct {
		name string
		data string
		want bool
	}{
		{
			name: "empty data is not resolved",
			data: "",
			want: false,
		},
		{
			name: "populated data is resolved",
			data: "some data",
			want: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rr := &v1beta1.ResolutionRequest{
				Status: v1beta1.ResolutionRequestStatus{
					ResolutionRequestStatusFields: v1beta1.ResolutionRequestStatusFields{
						Data: tc.data,
					},
				},
			}
			if got := rr.IsResolved(); got != tc.want {
				t.Errorf("IsResolved() = %v, want %v", got, tc.want)
			}
		})
	}
}
