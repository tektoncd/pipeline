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

package logstream

import (
	"os"
	"sync"

	"knative.dev/pkg/system"
	"knative.dev/pkg/test"
)

// Canceler is the type of a function returned when a logstream is started to be
// deferred so that the logstream can be stopped when the test is complete.
type Canceler func()

// Start begins streaming the logs from system components with a `key:` matching
// `test.ObjectNameForTest(t)` to `t.Log`.  It returns a Canceler, which must
// be called before the test completes.
func Start(t test.TLegacy) Canceler {
	// Do this lazily to make import ordering less important.
	once.Do(func() {
		if ns := os.Getenv(system.NamespaceEnvKey); ns != "" {
			// If SYSTEM_NAMESPACE is set, then start the stream.
			stream = &kubelogs{namespace: ns}
		} else {
			// Otherwise set up a null stream.
			stream = &null{}
		}
	})

	return stream.Start(t)
}

type streamer interface {
	Start(t test.TLegacy) Canceler
}

var (
	stream streamer
	once   sync.Once
)
