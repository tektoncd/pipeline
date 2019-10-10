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

package gke

import (
	"errors"
	"fmt"
	"time"

	container "google.golang.org/api/container/v1beta1"
)

// These are arbitrary numbers determined based on past experience
var (
	creationTimeout = 20 * time.Minute
	deletionTimeout = 10 * time.Minute
)

const (
	pendingStatus = "PENDING"
	runningStatus = "RUNNING"
	doneStatus    = "DONE"
)

// Wait depends on unique opName(operation ID created by cloud), and waits until
// it's done
func Wait(gsc SDKOperations, project, region, zone, opName string, wait time.Duration) error {
	var op *container.Operation
	var err error

	timeout := time.After(wait)
	tick := time.Tick(500 * time.Millisecond)
	for {
		select {
		// Got a timeout! fail with a timeout error
		case <-timeout:
			return errors.New("timed out waiting")
		case <-tick:
			// Retry 3 times in case of weird network error, or rate limiting
			for r, w := 0, 50*time.Microsecond; r < 3; r, w = r+1, w*2 {
				op, err = gsc.GetOperation(project, region, zone, opName)
				if err == nil {
					if op.Status == doneStatus {
						return nil
					} else if op.Status == pendingStatus || op.Status == runningStatus {
						// Valid operation, no need to retry
						break
					} else {
						// Have seen intermittent error state and fixed itself,
						// let it retry to avoid too much flakiness
						err = fmt.Errorf("unexpected operation status: %q", op.Status)
					}
				}
				time.Sleep(w)
			}
			// If err still persist after retries, exit
			if err != nil {
				return err
			}
		}
	}
}
