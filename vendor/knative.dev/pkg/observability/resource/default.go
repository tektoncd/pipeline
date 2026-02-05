/*
Copyright 2025 The Knative Authors

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

package resource

import (
	"os"

	"knative.dev/pkg/changeset"
	"knative.dev/pkg/observability/semconv"
	"knative.dev/pkg/system"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/resource"
)

const otelServiceNameKey = "OTEL_SERVICE_NAME"

// Default returns a default OpenTelemetry Resource enriched
// with common Knative/Kubernetes attributes.
//
// It will populate:
// - Namespace using system.Namespace
// - PodName using system.PodName
// - ServiceVersion with changeset.Get
func Default(serviceName string) *resource.Resource {
	// If the OTEL_SERVICE_NAME is set then let this override
	// our own serviceName
	if name := os.Getenv(otelServiceNameKey); name != "" {
		serviceName = name
	}

	attrs := []attribute.KeyValue{
		semconv.K8SNamespaceName(system.Namespace()),
		semconv.ServiceVersion(changeset.Get()),
		semconv.ServiceName(serviceName),
	}

	if pn := system.PodName(); pn != "" {
		attrs = append(attrs, semconv.K8SPodName(pn))
	}

	// Ignore the error because it complains about semconv
	// schema version differences
	resource, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			attrs...,
		),
	)
	if err != nil {
		panic(err)
	}
	return resource
}
