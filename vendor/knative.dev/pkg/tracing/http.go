/*
Copyright 2019 The Knative Authors

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

package tracing

import (
	"net/http"

	"go.opencensus.io/plugin/ochttp"
	"go.opencensus.io/trace"
	"k8s.io/apimachinery/pkg/util/sets"
)

var (
	// neverSample forcibly turns off sampling.
	neverSample = trace.StartOptions{
		Sampler: trace.NeverSample(),
	}
	// underlyingSampling uses the underlying sampling configuration (normally via the ConfigMap
	// config-tracing).
	underlyingSampling = trace.StartOptions{}

	// HTTPSpanMiddleware is an http.Handler middleware to create spans for the HTTP endpoint.
	HTTPSpanMiddleware = HTTPSpanIgnoringPaths()
)

// HTTPSpanIgnoringPaths is an http.Handler middleware to create spans for the HTTP
// endpoint, not sampling any request whose path is in pathsToIgnore.
func HTTPSpanIgnoringPaths(pathsToIgnore ...string) func(http.Handler) http.Handler {
	pathsToIgnoreSet := sets.NewString(pathsToIgnore...)
	return func(next http.Handler) http.Handler {
		return &ochttp.Handler{
			Handler: next,
			GetStartOptions: func(r *http.Request) trace.StartOptions {
				if pathsToIgnoreSet.Has(r.URL.Path) {
					return neverSample
				}
				return underlyingSampling
			},
		}
	}
}
