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

package common

import (
	"context"
	"testing"
)

func TestRequestNamespace(t *testing.T) {
	namespaceA := "foo"
	namespaceB := "bar"

	ctx := context.Background()
	ctx = InjectRequestNamespace(ctx, namespaceA)
	if RequestNamespace(ctx) != namespaceA {
		t.Fatalf("expected namespace to be stored as part of context")
	}

	ctx = InjectRequestNamespace(ctx, namespaceB)
	if RequestNamespace(ctx) != namespaceA {
		t.Fatalf("expected stored namespace to be immutable once set")
	}

	ctx = context.Background()
	if RequestNamespace(ctx) != "" {
		t.Fatalf("expected empty namespace returned if no value was previously injected")
	}
}

func TestRequestName(t *testing.T) {
	nameA := "foo"
	nameB := "bar"

	ctx := context.Background()
	ctx = InjectRequestName(ctx, nameA)
	if RequestName(ctx) != nameA {
		t.Fatalf("expected namespace to be stored as part of context")
	}

	ctx = InjectRequestNamespace(ctx, nameB)
	if RequestName(ctx) != nameA {
		t.Fatalf("expected stored namespace to be immutable once set")
	}

	ctx = context.Background()
	if RequestName(ctx) != "" {
		t.Fatalf("expected empty namespace returned if no value was previously injected")
	}
}
