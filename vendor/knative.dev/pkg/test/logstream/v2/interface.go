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

package logstream

type (
	// Canceler is the type of a function returned when a logstream is
	// started to be deferred so that the logstream can be stopped when
	// the test is complete.
	Canceler func()

	// Callback is invoked after pod logs are transformed
	Callback func(string, ...interface{})

	// Source allows you to create streams for a given resource name
	Source interface {
		// Start a log stream for the given resource name and invoke
		// the callback with the processed log
		StartStream(name string, l Callback) (Canceler, error)
	}
)
