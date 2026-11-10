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

package artifactstorage

import (
	"context"
	"io"
	"testing"

	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1/types"
)

type fakeProvider struct {
	name string
}

func (f *fakeProvider) Name() string { return f.name }
func (f *fakeProvider) Store(context.Context, StoreOptions, io.Reader) (*v1.ArtifactStorageRef, error) {
	return nil, nil //nolint:nilnil // test fake
}
func (f *fakeProvider) Fetch(context.Context, *v1.ArtifactStorageRef) (io.ReadCloser, error) {
	return nil, nil //nolint:nilnil // test fake
}
func (f *fakeProvider) Delete(context.Context, *v1.ArtifactStorageRef) error { return nil }
func (f *fakeProvider) Exists(context.Context, *v1.ArtifactStorageRef) (bool, error) {
	return false, nil
}

func TestRegistry_RegisterAndGet(t *testing.T) {
	r := NewRegistry()
	p := &fakeProvider{name: "test-backend"}

	if err := r.Register(p); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	got, err := r.Get("test-backend")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got.Name() != "test-backend" {
		t.Errorf("got provider name %q, want %q", got.Name(), "test-backend")
	}
}

func TestRegistry_DuplicateRegister(t *testing.T) {
	r := NewRegistry()
	p := &fakeProvider{name: "dup"}

	if err := r.Register(p); err != nil {
		t.Fatalf("first Register failed: %v", err)
	}
	if err := r.Register(p); err == nil {
		t.Fatal("expected error on duplicate Register, got nil")
	}
}

func TestRegistry_GetUnknown(t *testing.T) {
	r := NewRegistry()
	_, err := r.Get("nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown provider, got nil")
	}
}

func TestRegistry_List(t *testing.T) {
	r := NewRegistry()
	_ = r.Register(&fakeProvider{name: "a"})
	_ = r.Register(&fakeProvider{name: "b"})

	names := r.List()
	if len(names) != 2 {
		t.Fatalf("expected 2 providers, got %d", len(names))
	}

	nameSet := make(map[string]bool)
	for _, n := range names {
		nameSet[n] = true
	}
	if !nameSet["a"] || !nameSet["b"] {
		t.Errorf("expected providers [a, b], got %v", names)
	}
}
