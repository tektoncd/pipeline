/*
Copyright 2022 The Tekton Authors

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

package clock

import "time"

// Clock provides functions to get the current time and the duration of time elapsed since the current time
type Clock interface {
	Now() time.Time
	Since(t time.Time) time.Duration
}

// RealClock implements Clock based on the actual time
type RealClock struct{}

// Now returns the current time
func (RealClock) Now() time.Time { return time.Now() }

// Since returns the duration of time elapsed since the current time
func (RealClock) Since(t time.Time) time.Duration { return time.Since(t) }
