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
	"context"
	"testing"

	common "github.com/tektoncd/pipeline/pkg/resolution/common"
)

func TestRequestNamespace(t *testing.T) {
	namespaceA := "foo"
	namespaceB := "bar"

	ctx := context.Background()
	ctx = common.InjectRequestNamespace(ctx, namespaceA)
	if common.RequestNamespace(ctx) != namespaceA {
		t.Fatalf("expected namespace to be stored as part of context")
	}

	ctx = common.InjectRequestNamespace(ctx, namespaceB)
	if common.RequestNamespace(ctx) != namespaceA {
		t.Fatalf("expected stored namespace to be immutable once set")
	}

	ctx = context.Background()
	if common.RequestNamespace(ctx) != "" {
		t.Fatalf("expected empty namespace returned if no value was previously injected")
	}
}

func TestRequestName(t *testing.T) {
	nameA := "foo"
	nameB := "bar"

	ctx := context.Background()
	ctx = common.InjectRequestName(ctx, nameA)
	if common.RequestName(ctx) != nameA {
		t.Fatalf("expected namespace to be stored as part of context")
	}

	ctx = common.InjectRequestNamespace(ctx, nameB)
	if common.RequestName(ctx) != nameA {
		t.Fatalf("expected stored namespace to be immutable once set")
	}

	ctx = context.Background()
	if common.RequestName(ctx) != "" {
		t.Fatalf("expected empty namespace returned if no value was previously injected")
	}
}
