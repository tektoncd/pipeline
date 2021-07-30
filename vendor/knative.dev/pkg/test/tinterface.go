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

// Defines an interface of commonality between testing.T and logging.TLogger
// Allows most library functions to be shared
// Simplifies coexistence with TLogger

package test

// T is an interface mimicking *testing.T.
// Deprecated: Do not use this. Define your own interface.
type T interface {
	Name() string
	Helper()
	SkipNow()
	Cleanup(func())
	Log(args ...interface{})
	Error(args ...interface{})
}

// TLegacy is an interface mimicking *testing.T.
// Deprecated: Do not use this. Define your own interface.
type TLegacy interface {
	T
	Logf(fmt string, args ...interface{}) // It gets passed to things in logstream
	Fatal(args ...interface{})
}
