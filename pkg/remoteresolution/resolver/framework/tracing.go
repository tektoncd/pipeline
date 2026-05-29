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

package framework

import (
	"context"
	"net/url"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const (
	// TracerName is the name of the tracer used by the resolver framework.
	TracerName = "tekton.dev/resolution/resolver"

	spanNameResolutionRequestReconcile = "ResolutionRequest:Reconcile"
	spanNameResolutionRequestValidate  = "ResolutionRequest:Validate"
	spanNameResolutionRequestResolve   = "ResolutionRequest:Resolve"
	spanNameResolverResolve            = "Resolver:Resolve"

	attributeResolverType               = "resolver.type"
	attributeResolutionRequestName      = "resolutionrequest.name"
	attributeResolutionRequestNamespace = "resolutionrequest.namespace"
)

type tracerProviderKey struct{}

func withTracerProvider(ctx context.Context, tracerProvider trace.TracerProvider) context.Context {
	return context.WithValue(ctx, tracerProviderKey{}, tracerProvider)
}

func tracerProviderFromContext(ctx context.Context) trace.TracerProvider {
	if tracerProvider, ok := ctx.Value(tracerProviderKey{}).(trace.TracerProvider); ok && tracerProvider != nil {
		return tracerProvider
	}
	return trace.NewNoopTracerProvider()
}

func StartResolverSpan(ctx context.Context, resolverType string) (context.Context, trace.Span) {
	ctx, span := tracerProviderFromContext(ctx).Tracer(TracerName).Start(ctx, spanNameResolverResolve)
	span.SetAttributes(attribute.String(attributeResolverType, resolverType))
	return ctx, span
}

func RecordSpanError(span trace.Span, err error) {
	if err == nil {
		return
	}
	span.RecordError(err)
	span.SetStatus(codes.Error, err.Error())
}

func SanitizedURLAttribute(key, rawURL string) attribute.KeyValue {
	return attribute.String(key, sanitizeURLForTracing(rawURL))
}

func sanitizeURLForTracing(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	u.User = nil
	u.RawQuery = ""
	u.Fragment = ""
	return u.String()
}
