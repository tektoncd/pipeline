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
	"testing"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
)

func TestInputResourcePath(t *testing.T) {
	tcs := []struct {
		name     string
		resource v1alpha1.ResourceDeclaration
		expected string
	}{{
		name: "default_path",
		resource: v1alpha1.ResourceDeclaration{
			Name: "foo",
		},
		expected: "/workspace/foo",
	}, {
		name: "with target path",
		resource: v1alpha1.ResourceDeclaration{
			Name:       "foo",
			TargetPath: "bar",
		},
		expected: "/workspace/bar",
	}}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			if actual := v1alpha1.InputResourcePath(tc.resource); actual != tc.expected {
				t.Errorf("Unexpected input resource path: %s", actual)
			}
		})
	}
}

func TestOutputResourcePath(t *testing.T) {
	tcs := []struct {
		name     string
		resource v1alpha1.ResourceDeclaration
		expected string
	}{{
		name: "default_path",
		resource: v1alpha1.ResourceDeclaration{
			Name: "foo",
		},
		expected: "/workspace/output/foo",
	}, {
		name: "with target path",
		resource: v1alpha1.ResourceDeclaration{
			Name:       "foo",
			TargetPath: "bar",
		},
		expected: "/workspace/bar",
	}}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			if actual := v1alpha1.OutputResourcePath(tc.resource); actual != tc.expected {
				t.Errorf("Unexpected output resource path: %s", actual)
			}
		})
	}
}
