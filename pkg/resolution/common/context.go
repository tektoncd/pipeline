/*
Copyright 2022 The Tekton Authors

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

import "context"

// namespaceContextKey and nameContextKey must be distinct types.
// context.WithValue uses interface equality (type + value) for key lookup.
// Two zero-value empty structs of the same type are always equal in Go,
// so using a single type would cause key collision.
type namespaceContextKey struct{}
type nameContextKey struct{}

// requestNamespaceContextKey is the key stored in a context alongside
// the string namespace of a resolution request.
var requestNamespaceContextKey = namespaceContextKey{}

// InjectRequestNamespace returns a new context with a request-scoped
// namespace. This value may only be set once per request; subsequent
// calls with the same context or a derived context will be ignored.
func InjectRequestNamespace(ctx context.Context, namespace string) context.Context {
	// Once set don't allow the value to be overwritten.
	if val := ctx.Value(requestNamespaceContextKey); val != nil {
		return ctx
	}
	return context.WithValue(ctx, requestNamespaceContextKey, namespace)
}

// RequestNamespace returns the namespace of the resolution request
// currently being processed or an empty string if the request somehow
// does not originate from a namespaced location.
func RequestNamespace(ctx context.Context) string {
	if val := ctx.Value(requestNamespaceContextKey); val != nil {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

// requestNameContextKey is the key stored in a context alongside
// the string name of a resolution request.
var requestNameContextKey = nameContextKey{}

// InjectRequestName returns a new context with a request-scoped
// name. This value may only be set once per request; subsequent
// calls with the same context or a derived context will be ignored.
func InjectRequestName(ctx context.Context, name string) context.Context {
	// Once set don't allow the value to be overwritten.
	if val := ctx.Value(requestNameContextKey); val != nil {
		return ctx
	}
	return context.WithValue(ctx, requestNameContextKey, name)
}

// RequestName returns the name of the resolution request
// currently being processed or an empty string if none were registered.
func RequestName(ctx context.Context) string {
	if val := ctx.Value(requestNameContextKey); val != nil {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}
