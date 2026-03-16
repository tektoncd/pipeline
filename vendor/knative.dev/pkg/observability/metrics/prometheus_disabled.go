//go:build noprometheus

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

package metrics

import (
	"context"

	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
)

func buildPrometheus(ctx context.Context, cfg Config) (sdkmetric.Reader, shutdownFunc, error) {
	panic("prometheus exporter is not supported")
}
