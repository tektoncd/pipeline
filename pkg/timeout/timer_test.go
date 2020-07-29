/*
Copyright 2020 The Tekton Authors

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

package timeout

import (
	"testing"
	"time"

	tb "github.com/tektoncd/pipeline/internal/builder/v1beta1"
	rtesting "github.com/tektoncd/pipeline/pkg/reconciler/testing"
)

func TestSetTimer_TimesOut(t *testing.T) {
	timer := Timer{
		stopCh: make(chan struct{}),
		done:   make(map[string]chan bool),
		logger: rtesting.TestLogger(t),
	}
	callbackCh := make(chan struct{})
	c := func(interface{}) { close(callbackCh) }

	go timer.SetTimer(tb.TaskRun("foo"), time.Millisecond, c)
	select {
	case <-callbackCh:
		// This case is hit when the callback is called due to a successful timeout
	case <-time.After(timerFailDeadline):
		t.Errorf("timer did not execucte callback within expected time")
	}
}

func waitForSuccess(t *testing.T, successCh, callbackCh chan struct{}) {
	t.Helper()
	select {
	case <-successCh:
		// The successful case is that signaling done causes the timer to stop and return
	case <-callbackCh:
		t.Errorf("the callback was called, even though it should only be called after 500 hours!")
	case <-time.After(timerFailDeadline):
		t.Errorf("timer did not execucte callback within expected time")
	}
}

func TestSetTimer_Done(t *testing.T) {
	tr := tb.TaskRun("foo")
	doneCh := make(chan bool)
	timer := Timer{
		stopCh: make(chan struct{}),
		done: map[string]chan bool{
			tr.GetRunKey(): doneCh,
		},
		logger: rtesting.TestLogger(t),
	}

	callbackCh := make(chan struct{})
	c := func(interface{}) { close(callbackCh) }

	successCh := make(chan struct{})
	go func() {
		timer.SetTimer(tr, 500*time.Hour, c)
		// Indicate that SetTimer returned, as expected
		close(successCh)
	}()
	close(doneCh)
	waitForSuccess(t, successCh, callbackCh)
}

func TestSetTimer_Stop(t *testing.T) {
	stopCh := make(chan struct{})
	timer := Timer{
		stopCh: stopCh,
		done:   make(map[string]chan bool),
		logger: rtesting.TestLogger(t),
	}

	callbackCh := make(chan struct{})
	c := func(interface{}) { close(callbackCh) }

	successCh := make(chan struct{})
	go func() {
		timer.SetTimer(tb.TaskRun("foo"), 500*time.Hour, c)
		// Indicate that SetTimer returned, as expected
		close(successCh)
	}()
	close(stopCh)
	waitForSuccess(t, successCh, callbackCh)
}
