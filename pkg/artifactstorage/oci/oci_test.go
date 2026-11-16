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
	"errors"
	"net/http"
	"testing"

	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
	"github.com/tektoncd/pipeline/pkg/artifactstorage"
)

func TestOCIProvider_Name(t *testing.T) {
	p := NewOCIProvider("registry.example.com/artifacts")
	if got := p.Name(); got != "oci" {
		t.Errorf("Name() = %q, want %q", got, "oci")
	}
}

func TestOCIProvider_ImplementsInterfaces(t *testing.T) {
	p := NewOCIProvider("registry.example.com/artifacts")

	var _ artifactstorage.Provider = p
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
		name string
		err  error
		want bool
	}{
		{
			name: "transport error with 404 status code",
			err:  &transport.Error{StatusCode: http.StatusNotFound},
			want: true,
		},
		{
			name: "transport error with MANIFEST_UNKNOWN diagnostic",
			err: &transport.Error{
				StatusCode: http.StatusBadRequest,
				Errors:     []transport.Diagnostic{{Code: transport.ManifestUnknownErrorCode}},
			},
			want: true,
		},
		{
			name: "transport error with NAME_UNKNOWN diagnostic",
			err: &transport.Error{
				StatusCode: http.StatusBadRequest,
				Errors:     []transport.Diagnostic{{Code: transport.NameUnknownErrorCode}},
			},
			want: true,
		},
		{
			name: "transport error with 403 is not treated as not-found",
			err:  &transport.Error{StatusCode: http.StatusForbidden},
			want: false,
		},
		{
			name: "repo name containing 404 is not treated as not-found",
			err: &transport.Error{
				StatusCode: http.StatusForbidden,
				Errors:     []transport.Diagnostic{{Code: transport.DeniedErrorCode, Message: "artifacts404: access denied"}},
			},
			want: false,
		},
		{
			name: "non-transport error is not treated as not-found",
			err:  errors.New("connection refused"),
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isNotFound(tt.err)
			if got != tt.want {
				t.Errorf("isNotFound() = %v, want %v", got, tt.want)
			}
		})
	}
}
