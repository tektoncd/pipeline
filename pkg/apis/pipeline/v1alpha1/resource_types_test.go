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
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestAttributesFromType(t *testing.T) {
	tests := []struct {
		name         string
		resourceType PipelineResourceType
		expectedErr  error
	}{
		{
			name:         "git resource type",
			resourceType: PipelineResourceTypeGit,
			expectedErr:  nil,
		},
		{
			name:         "storage resource type",
			resourceType: PipelineResourceTypeStorage,
			expectedErr:  nil,
		},
		{
			name:         "image resource type",
			resourceType: PipelineResourceTypeImage,
			expectedErr:  nil,
		},
		{
			name:         "cluster resource type",
			resourceType: PipelineResourceTypeCluster,
			expectedErr:  nil,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := AttributesFromType(tc.resourceType)
			if d := cmp.Diff(err, tc.expectedErr); d != "" {
				t.Errorf("AttributesFromType error did not match expected error %s", d)
			}
		})
	}
}
