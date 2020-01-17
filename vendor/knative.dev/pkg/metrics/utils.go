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

package metrics

import "strconv"

// ResponseCodeClass converts an HTTP response code to a string representing its response code class.
// E.g., The response code class is "5xx" for response code 503.
func ResponseCodeClass(responseCode int) string {
	// Get the hundred digit of the response code and concatenate "xx".
	return strconv.Itoa(responseCode/100) + "xx"
}
