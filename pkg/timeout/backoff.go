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
	"math"
	"math/rand"
	"sync"
	"time"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"go.uber.org/zap"
)

// Backoff can be used to start timers used to perform exponential backoffs with jitter.
type Backoff struct {
	logger *zap.SugaredLogger

	// attempts is a map from the name of the item being backed off to the Attemps object
	// containing its current state
	attempts map[string]Attempts
	// attemptsMut is used to protect access to attempts to ensure that multiple goroutines
	// don't try to update it simultaneously
	attemptsMut sync.Mutex
	// timeoutCallback is the function to call when a timeout has occurred.
	timeoutCallback func(interface{})
	// timer is used to start timers in separate goroutines
	timer *Timer
	// j is the function that will be called to jitter the backoff intervals.
	j jitterFunc
	// now is the function that will be used to get the current time.
	now nowFunc
}

// Attempts contains state of exponential backoff for a given StatusKey
type Attempts struct {
	// NumAttempts reflects the number of times a given StatusKey has been delayed
	NumAttempts uint
	// NextAttempt is the point in time at which this backoff expires
	NextAttempt time.Time
}

// jitterFunc is a func applied to a computed backoff duration to remove uniformity
// from its results. A jitterFunc receives the number of seconds calculated by a
// backoff algorithm and returns the "jittered" result.
type jitterFunc func(numSeconds int) (jitteredSeconds int)

// nowFunc is a function that is used to get the current time
type nowFunc func() time.Time

// NewBackoff returns an instance of Backoff with the specified stopCh and logger, instantiated
// and ready to track go routines.
func NewBackoff(
	stopCh <-chan struct{},
	logger *zap.SugaredLogger,
) *Backoff {
	return &Backoff{
		timer:    NewTimer(stopCh, logger),
		attempts: make(map[string]Attempts),
		j:        rand.Intn,
		now:      time.Now,
		logger:   logger,
	}
}

// Release will remove keys tracking the specified runObj.
func (b *Backoff) Release(runObj StatusKey) {
	b.attemptsMut.Lock()
	defer b.attemptsMut.Unlock()
	delete(b.attempts, runObj.GetRunKey())
}

// SetTimeoutCallback will set the function to be called when a timeout has occurred.
func (b *Backoff) SetTimeoutCallback(f func(interface{})) {
	b.timeoutCallback = f
}

// Get records the number of times it has seen a TaskRun and calculates an
// appropriate backoff deadline based on that count. Only one backoff per TaskRun
// may be active at any moment. Requests for a new backoff in the face of an
// existing one will be ignored and details of the existing backoff will be returned
// instead. Further, if a calculated backoff time is after the timeout of the TaskRun
// then the time of the timeout will be returned instead.
//
// Returned values are a backoff struct containing a NumAttempts field with the
// number of attempts performed for this TaskRun and a NextAttempt field
// describing the time at which the next attempt should be performed.
// Additionally a boolean is returned indicating whether a backoff for the
// TaskRun is already in progress.
func (b *Backoff) Get(tr *v1beta1.TaskRun) (a Attempts, inProgress bool) {
	b.attemptsMut.Lock()
	defer b.attemptsMut.Unlock()
	a = b.attempts[tr.GetRunKey()]
	if b.now().Before(a.NextAttempt) {
		inProgress = true
		return
	}
	a.NumAttempts++
	a.NextAttempt = b.now().Add(GetExponentialBackoffWithJitter(a.NumAttempts, b.j))
	duration := timeoutFromSpec((tr.Spec.Timeout))
	timeoutDeadline := tr.Status.StartTime.Time.Add(duration)
	if timeoutDeadline.Before(a.NextAttempt) {
		a.NextAttempt = timeoutDeadline
	}
	b.attempts[tr.GetRunKey()] = a
	return
}

// GetExponentialBackoffWithJitter will return a number which is 2 to the power
// of count, but with a jittered value obtained via jf.
func GetExponentialBackoffWithJitter(count uint, jf jitterFunc) time.Duration {
	exp := float64(count)
	if exp > maxBackoffExponent {
		exp = maxBackoffExponent
	}
	seconds := int(math.Exp2(exp))
	jittered := 1 + jf(seconds)
	if jittered > maxBackoffSeconds {
		jittered = maxBackoffSeconds
	}
	return time.Duration(jittered) * time.Second
}

// SetTimer creates a blocking function to wait for
// 1. Stop signal, 2. completion or 3. a given Duration to elapse.
func (b *Backoff) SetTimer(runObj StatusKey, d time.Duration) {
	if b.timeoutCallback == nil {
		b.logger.Errorf("attempted to set a timer for %q but no callback has been assigned", runObj)
		return
	}
	b.logger.Infof("About to start backoff timer for %s. backing off for %s", runObj.GetRunKey(), d)
	defer b.Release(runObj)
	b.timer.SetTimer(runObj, d, b.timeoutCallback)
}
