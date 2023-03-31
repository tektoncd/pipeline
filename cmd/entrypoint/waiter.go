/*
Copyright 2023 The Tekton Authors

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

package main

import (
	"fmt"
	"os"
	"time"

	"github.com/tektoncd/pipeline/pkg/entrypoint"
)

// realWaiter actually waits for files, by polling.
type realWaiter struct {
	waitPollingInterval time.Duration
	breakpointOnFailure bool
}

var _ entrypoint.Waiter = (*realWaiter)(nil)

// setWaitPollingInterval sets the pollingInterval that will be used by the wait function
func (rw *realWaiter) setWaitPollingInterval(pollingInterval time.Duration) *realWaiter {
	rw.waitPollingInterval = pollingInterval
	return rw
}

// Wait watches a file and returns when either a) the file exists and, if
// the expectContent argument is true, the file has non-zero size or b) there
// is an error polling the file.
//
// If the passed-in file is an empty string then this function returns
// immediately.
//
// If a file of the same name with a ".err" extension exists then this Wait
// will end with a skipError.
func (rw *realWaiter) Wait(file string, expectContent bool, breakpointOnFailure bool) error {
	if file == "" {
		return nil
	}
	for ; ; time.Sleep(rw.waitPollingInterval) {
		if info, err := os.Stat(file); err == nil {
			if !expectContent || info.Size() > 0 {
				return nil
			}
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("waiting for %q: %w", file, err)
		}
		// When a .err file is read by this step, it means that a previous step has failed
		// We wouldn't want this step to stop executing because the previous step failed during debug
		// That is counterproductive to debugging
		// Hence we disable skipError here so that the other steps in the failed taskRun can continue
		// executing if breakpointOnFailure is enabled for the taskRun
		// TLDR: Do not return skipError when breakpointOnFailure is enabled as it breaks execution of the TaskRun
		if _, err := os.Stat(file + ".err"); err == nil {
			if breakpointOnFailure {
				return nil
			}
			return skipError("error file present, bail and skip the step")
		}
	}
}

type skipError string

func (e skipError) Error() string {
	return string(e)
}
