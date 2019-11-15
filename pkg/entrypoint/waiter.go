/*
Copyright 2019 The Tekton Authors

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

package entrypoint

import (
	"fmt"
	"os"
	"time"
)

// realWaiter actually waits for files, by polling.
type RealWaiter struct{}

var _ Waiter = (*RealWaiter)(nil)

var waitPollingInterval = time.Second

// Wait watches a file and returns when either a) the file exists and, if
// the expectContent argument is true, the file has non-zero size or b) there
// is an error polling the file.
//
// If the passed-in file is an empty string then this function returns
// immediately.
//
// If a file of the same name with a ".err" extension exists then this Wait
// will end with a skipError.
func (*RealWaiter) Wait(file string, expectContent bool) error {
	if file == "" {
		return nil
	}
	for ; ; time.Sleep(waitPollingInterval) {
		if info, err := os.Stat(file); err == nil {
			if !expectContent || info.Size() > 0 {
				return nil
			}
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("waiting for %q: %w", file, err)
		}
		if _, err := os.Stat(file + ".err"); err == nil {
			return SkipError("error file present, bail and skip the step")
		}
	}
}

type SkipError string

func (e SkipError) Error() string {
	return string(e)
}
