/*
Copyright 2023 The Tekton Authors

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

package common_test

import (
	"testing"

	common "github.com/tektoncd/pipeline/pkg/resolution/common"
)

func TestRequestNamespace(t *testing.T) {
	namespaceA := "foo"
	namespaceB := "bar"

	ctx := t.Context()
	ctx = common.InjectRequestNamespace(ctx, namespaceA)
	if common.RequestNamespace(ctx) != namespaceA {
		t.Fatalf("expected namespace to be stored as part of context")
	}

	ctx = common.InjectRequestNamespace(ctx, namespaceB)
	if common.RequestNamespace(ctx) != namespaceA {
		t.Fatalf("expected stored namespace to be immutable once set")
	}

	ctx = t.Context()
	if common.RequestNamespace(ctx) != "" {
		t.Fatalf("expected empty namespace returned if no value was previously injected")
	}
}

func TestRequestName(t *testing.T) {
	nameA := "foo"
	nameB := "bar"

	ctx := t.Context()
	ctx = common.InjectRequestName(ctx, nameA)
	if common.RequestName(ctx) != nameA {
		t.Fatalf("expected namespace to be stored as part of context")
	}

	ctx = common.InjectRequestNamespace(ctx, nameB)
	if common.RequestName(ctx) != nameA {
		t.Fatalf("expected stored namespace to be immutable once set")
	}

	ctx = t.Context()
	if common.RequestName(ctx) != "" {
		t.Fatalf("expected empty namespace returned if no value was previously injected")
	}
}

func TestRequestNameAfterNamespace(t *testing.T) {
	namespace := "my-namespace"
	name := "my-name"

	ctx := t.Context()
	ctx = common.InjectRequestNamespace(ctx, namespace)
	ctx = common.InjectRequestName(ctx, name)

	if got := common.RequestNamespace(ctx); got != namespace {
		t.Fatalf("expected namespace %q but got %q", namespace, got)
	}
	if got := common.RequestName(ctx); got != name {
		t.Fatalf("expected name %q but got %q", name, got)
	}
}

func TestRequestNamespaceAfterName(t *testing.T) {
	ctx := t.Context()
	ctx = common.InjectRequestName(ctx, "my-name")
	ctx = common.InjectRequestNamespace(ctx, "my-namespace")
	if common.RequestName(ctx) != "my-name" {
		t.Fatalf("expected request name 'my-name', got '%s'", common.RequestName(ctx))
	}
	if common.RequestNamespace(ctx) != "my-namespace" {
		t.Fatalf("expected request namespace 'my-namespace', got '%s'", common.RequestNamespace(ctx))
	}
}
