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
