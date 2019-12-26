package main

import (
	"fmt"
	"os"
	"time"

	"github.com/tektoncd/pipeline/pkg/entrypoint"
)

// realWaiter actually waits for files, by polling.
type realWaiter struct{}

var _ entrypoint.Waiter = (*realWaiter)(nil)

// Wait watches a file and returns when either a) the file exists and, if
// the expectContent argument is true, the file has non-zero size or b) there
// is an error polling the file.
//
// If the passed-in file is an empty string then this function returns
// immediately.
//
// If a file of the same name with a ".err" extension exists then this Wait
// will end with a skipError.
func (*realWaiter) Wait(file string, expectContent bool) error {
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
			return skipError("error file present, bail and skip the step")
		}
	}
}

type skipError string

func (e skipError) Error() string {
	return string(e)
}
