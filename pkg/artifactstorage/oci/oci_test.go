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

package oci

import (
	"fmt"
	"testing"

	"github.com/tektoncd/pipeline/pkg/artifactstorage"
)

func TestOCIProvider_Name(t *testing.T) {
	p := NewOCIProvider("registry.example.com/artifacts")
	if got := p.Name(); got != "oci" {
		t.Errorf("Name() = %q, want %q", got, "oci")
	}
}

func TestOCIProvider_ContentAddressed(t *testing.T) {
	p := NewOCIProvider("registry.example.com/artifacts")
	if !p.ContentAddressed() {
		t.Error("ContentAddressed() = false, want true")
	}
}

func TestOCIProvider_ImplementsInterfaces(t *testing.T) {
	p := NewOCIProvider("registry.example.com/artifacts")

	var _ artifactstorage.Provider = p
	var _ artifactstorage.ContentAddressedProvider = p
}

func TestNewOCIProvider_WithOptions(t *testing.T) {
	p := NewOCIProvider("registry.example.com/artifacts",
		WithInsecure(true),
	)
	if !p.insecure {
		t.Error("expected insecure to be true")
	}
	if p.repository != "registry.example.com/artifacts" {
		t.Errorf("repository = %q, want %q", p.repository, "registry.example.com/artifacts")
	}
}

func TestIsNotFound(t *testing.T) {
	tests := []struct {
		msg  string
		want bool
	}{
		{"NOT_FOUND: manifest not found", true},
		{"MANIFEST_UNKNOWN", true},
		{"unexpected status: 404 Not Found", true},
		{"connection refused", false},
	}
	for _, tt := range tests {
		got := isNotFound(fmt.Errorf("%s", tt.msg))
		if got != tt.want {
			t.Errorf("isNotFound(%q) = %v, want %v", tt.msg, got, tt.want)
		}
	}
}
