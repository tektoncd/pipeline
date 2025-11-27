/*
Copyright 2025 The Tekton Authors

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

package cache

import "time"

// realClock implements Clock using the actual system time.
type realClock struct{}

func (realClock) Now() time.Time {
	return time.Now()
}

// fakeClock implements Clock with a controllable time for testing.
type fakeClock struct {
	now time.Time
}

// Now returns the current time of the fake clock.
func (f *fakeClock) Now() time.Time {
	return f.now
}

// Advance moves the fake clock forward by the given duration.
func (f *fakeClock) Advance(d time.Duration) {
	f.now = f.now.Add(d)
}
