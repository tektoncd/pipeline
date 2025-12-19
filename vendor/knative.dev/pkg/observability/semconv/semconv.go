/*
Copyright 2022 The Knative Authors

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

package semconv

import (
	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
)

const (
	SchemaURL = semconv.SchemaURL

	ServiceNameKey = semconv.ServiceNameKey

	ServerAddressKey = semconv.ServerAddressKey
	ServerPortKey    = semconv.ServerPortKey

	HTTPRequestMethodKey      = semconv.HTTPRequestMethodKey
	HTTPResponseStatusCodeKey = semconv.HTTPResponseStatusCodeKey

	URLSchemeKey   = semconv.URLSchemeKey
	URLTemplateKey = semconv.URLTemplateKey

	K8SNamespaceNameKey = semconv.K8SNamespaceNameKey
	K8SPodNameKey       = semconv.K8SPodNameKey
	K8SContainerNameKey = semconv.K8SContainerNameKey
)

func K8SContainerName(val string) attribute.KeyValue {
	return semconv.ContainerName(val)
}

func K8SPodName(val string) attribute.KeyValue {
	return semconv.K8SPodName(val)
}

func K8SNamespaceName(val string) attribute.KeyValue {
	return semconv.K8SNamespaceName(val)
}

func ServiceVersion(val string) attribute.KeyValue {
	return semconv.ServiceVersion(val)
}

func ServiceName(val string) attribute.KeyValue {
	return semconv.ServiceName(val)
}

func ServiceInstanceID(val string) attribute.KeyValue {
	return semconv.ServiceInstanceID(val)
}

func HTTPResponseStatusCode(val int) attribute.KeyValue {
	return semconv.HTTPResponseStatusCode(val)
}
