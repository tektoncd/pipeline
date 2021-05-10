/*
Copyright 2020 The Knative Authors

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

package spoof

import "net/http"

// RequestOption enables configuration of requests
// when polling for endpoint states.
type RequestOption func(*http.Request)

// WithHeader will add the provided headers to the request.
func WithHeader(header http.Header) RequestOption {
	return func(r *http.Request) {
		if r.Header == nil {
			r.Header = header
			return
		}
		for key, values := range header {
			for _, value := range values {
				r.Header.Add(key, value)
			}
		}
	}
}
