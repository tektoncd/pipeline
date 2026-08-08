/*
Copyright 2025 The Tekton Authors

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
package platforms

import (
	"testing"
)

func TestNewPlatform(t *testing.T) {
	p := NewPlatform()
	if p.OS == "" {
		t.Errorf("Expected OS to be set, got empty string")
	}
	if p.Architecture == "" {
		t.Errorf("Expected Arch to be set, got empty string")
	}
}

func TestPlatformFormat(t *testing.T) {
	p := &Platform{
		OS:           "linux",
		Architecture: "amd64",
		Variant:      "v8",
	}
	expected := "linux/amd64/v8"
	if got := p.Format(); got != expected {
		t.Errorf("Expected %s, got %s", expected, got)
	}
}

func TestIsArmArch(t *testing.T) {
	tests := []struct {
		arch     string
		expected bool
	}{
		{"arm", true},
		{"arm64", true},
		{"amd64", false},
		{"ppc64le", false},
	}

	for _, tt := range tests {
		if got := isArmArch(tt.arch); got != tt.expected {
			t.Errorf("isArmArch(%s) = %v, expected %v", tt.arch, got, tt.expected)
		}
	}
}

func TestGetCPUVariant(t *testing.T) {
	variant, err := getCPUVariant()
	if err != nil && variant != "" {
		t.Errorf("Expected error but got variant: %s", variant)
	}
}
