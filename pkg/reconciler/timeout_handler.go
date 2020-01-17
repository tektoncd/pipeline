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

package reconciler

import (
	"math"
	"math/rand"
	"sync"

	"time"

	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	clientset "github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	maxBackoffSeconds = 120
)

var (
	defaultFunc        = func(i interface{}) {}
	maxBackoffExponent = math.Ceil(math.Log2(maxBackoffSeconds))
)

// StatusKey interface to be implemented by Taskrun Pipelinerun types
type StatusKey interface {
	GetRunKey() string
}

// Backoff contains state of exponential backoff for a given StatusKey
type Backoff struct {
	// NumAttempts reflects the number of times a given StatusKey has been delayed
	NumAttempts uint
	// NextAttempt is the point in time at which this backoff expires
	NextAttempt time.Time
}

// jitterFunc is a func applied to a computed backoff duration to remove uniformity
// from its results. A jitterFunc receives the number of seconds calculated by a
// backoff algorithm and returns the "jittered" result.
type jitterFunc func(numSeconds int) (jitteredSeconds int)

// TimeoutSet contains required k8s interfaces to handle build timeouts
type TimeoutSet struct {
	logger                  *zap.SugaredLogger
	taskRunCallbackFunc     func(interface{})
	pipelineRunCallbackFunc func(interface{})
	stopCh                  <-chan struct{}
	done                    map[string]chan bool
	doneMut                 sync.Mutex
	backoffs                map[string]Backoff
	backoffsMut             sync.Mutex
}

// NewTimeoutHandler returns TimeoutSet filled structure
func NewTimeoutHandler(
	stopCh <-chan struct{},
	logger *zap.SugaredLogger,
) *TimeoutSet {
	return &TimeoutSet{
		stopCh:   stopCh,
		done:     make(map[string]chan bool),
		backoffs: make(map[string]Backoff),
		logger:   logger,
	}
}

// SetTaskRunCallbackFunc sets the callback function when timeout occurs for taskrun objects
func (t *TimeoutSet) SetTaskRunCallbackFunc(f func(interface{})) {
	t.taskRunCallbackFunc = f
}

// SetPipelineRunCallbackFunc sets the callback function when timeout occurs for pipelinerun objects
func (t *TimeoutSet) SetPipelineRunCallbackFunc(f func(interface{})) {
	t.pipelineRunCallbackFunc = f
}

// Release deletes channels and data that are specific to a StatusKey object.
func (t *TimeoutSet) Release(runObj StatusKey) {
	key := runObj.GetRunKey()
	t.doneMut.Lock()
	defer t.doneMut.Unlock()

	t.backoffsMut.Lock()
	defer t.backoffsMut.Unlock()

	if finished, ok := t.done[key]; ok {
		delete(t.done, key)
		close(finished)
	}
	delete(t.backoffs, key)
}

func (t *TimeoutSet) getOrCreateFinishedChan(runObj StatusKey) chan bool {
	var finished chan bool
	key := runObj.GetRunKey()
	t.doneMut.Lock()
	defer t.doneMut.Unlock()
	if existingFinishedChan, ok := t.done[key]; ok {
		finished = existingFinishedChan
	} else {
		finished = make(chan bool)
	}
	t.done[key] = finished
	return finished
}

// GetBackoff records the number of times it has seen a TaskRun and calculates an
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
func (t *TimeoutSet) GetBackoff(tr *v1alpha1.TaskRun) (Backoff, bool) {
	t.backoffsMut.Lock()
	defer t.backoffsMut.Unlock()
	b := t.backoffs[tr.GetRunKey()]
	if time.Now().Before(b.NextAttempt) {
		return b, true
	}
	b.NumAttempts++
	b.NextAttempt = time.Now().Add(backoffDuration(b.NumAttempts, rand.Intn))
	timeoutDeadline := tr.Status.StartTime.Time.Add(tr.Spec.Timeout.Duration)
	if timeoutDeadline.Before(b.NextAttempt) {
		b.NextAttempt = timeoutDeadline
	}
	t.backoffs[tr.GetRunKey()] = b
	return b, false
}

func backoffDuration(count uint, jf jitterFunc) time.Duration {
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

// checkPipelineRunTimeouts function creates goroutines to wait for pipelinerun to
// finish/timeout in a given namespace
func (t *TimeoutSet) checkPipelineRunTimeouts(namespace string, pipelineclientset clientset.Interface) {
	pipelineRuns, err := pipelineclientset.TektonV1alpha1().PipelineRuns(namespace).List(metav1.ListOptions{})
	if err != nil {
		t.logger.Errorf("Can't get pipelinerun list in namespace %s: %s", namespace, err)
		return
	}
	for _, pipelineRun := range pipelineRuns.Items {
		pipelineRun := pipelineRun
		if pipelineRun.IsDone() || pipelineRun.IsCancelled() {
			continue
		}
		if pipelineRun.HasStarted() {
			go t.WaitPipelineRun(&pipelineRun, pipelineRun.Status.StartTime)
		}
	}
}

// CheckTimeouts function iterates through all namespaces and calls corresponding
// taskrun/pipelinerun timeout functions
func (t *TimeoutSet) CheckTimeouts(kubeclientset kubernetes.Interface, pipelineclientset clientset.Interface) {
	namespaces, err := kubeclientset.CoreV1().Namespaces().List(metav1.ListOptions{})
	if err != nil {
		t.logger.Errorf("Can't get namespaces list: %s", err)
		return
	}
	for _, namespace := range namespaces.Items {
		t.checkTaskRunTimeouts(namespace.GetName(), pipelineclientset)
		t.checkPipelineRunTimeouts(namespace.GetName(), pipelineclientset)
	}
}

// checkTaskRunTimeouts function creates goroutines to wait for pipelinerun to
// finish/timeout in a given namespace
func (t *TimeoutSet) checkTaskRunTimeouts(namespace string, pipelineclientset clientset.Interface) {
	taskruns, err := pipelineclientset.TektonV1alpha1().TaskRuns(namespace).List(metav1.ListOptions{})
	if err != nil {
		t.logger.Errorf("Can't get taskrun list in namespace %s: %s", namespace, err)
		return
	}
	for _, taskrun := range taskruns.Items {
		taskrun := taskrun
		if taskrun.IsDone() || taskrun.IsCancelled() {
			continue
		}
		if taskrun.HasStarted() {
			go t.WaitTaskRun(&taskrun, taskrun.Status.StartTime)
		}
	}
}

// WaitTaskRun function creates a blocking function for taskrun to wait for
// 1. Stop signal, 2. TaskRun to complete or 3. Taskrun to time out, which is
// determined by checking if the tr's timeout has occurred since the startTime
func (t *TimeoutSet) WaitTaskRun(tr *v1alpha1.TaskRun, startTime *metav1.Time) {
	var timeout time.Duration
	if tr.Spec.Timeout == nil {
		timeout = config.DefaultTimeoutMinutes * time.Minute
	} else {
		timeout = tr.Spec.Timeout.Duration
	}
	t.waitRun(tr, timeout, startTime, t.taskRunCallbackFunc)
}

// WaitPipelineRun function creates a blocking function for pipelinerun to wait for
// 1. Stop signal, 2. pipelinerun to complete or 3. pipelinerun to time out which is
// determined by checking if the tr's timeout has occurred since the startTime
func (t *TimeoutSet) WaitPipelineRun(pr *v1alpha1.PipelineRun, startTime *metav1.Time) {
	var timeout time.Duration
	if pr.Spec.Timeout == nil {
		timeout = config.DefaultTimeoutMinutes * time.Minute
	} else {
		timeout = pr.Spec.Timeout.Duration
	}
	t.waitRun(pr, timeout, startTime, t.pipelineRunCallbackFunc)
}

func (t *TimeoutSet) waitRun(runObj StatusKey, timeout time.Duration, startTime *metav1.Time, callback func(interface{})) {
	if startTime == nil {
		t.logger.Errorf("startTime must be specified in order for a timeout to be calculated accurately for %s", runObj.GetRunKey())
		return
	}
	if callback == nil {
		callback = defaultFunc
	}
	runtime := time.Since(startTime.Time)
	t.logger.Infof("About to start timeout timer for %s. started at %s, timeout is %s, running for %s", runObj.GetRunKey(), startTime.Time, timeout, runtime)
	defer t.Release(runObj)
	t.setTimer(runObj, timeout-runtime, callback)
}

// SetTaskRunTimer creates a blocking function for taskrun to wait for
// 1. Stop signal, 2. TaskRun to complete or 3. a given Duration to elapse.
//
// Since the timer's duration is a parameter rather than being tied to
// the lifetime of the TaskRun no resources are released after the timer
// fires. It is the caller's responsibility to Release() the TaskRun when
// work with it has completed.
func (t *TimeoutSet) SetTaskRunTimer(tr *v1alpha1.TaskRun, d time.Duration) {
	callback := t.taskRunCallbackFunc
	if callback == nil {
		t.logger.Errorf("attempted to set a timer for %q but no task run callback has been assigned", tr.Name)
		return
	}
	t.setTimer(tr, d, callback)
}

func (t *TimeoutSet) setTimer(runObj StatusKey, timeout time.Duration, callback func(interface{})) {
	finished := t.getOrCreateFinishedChan(runObj)
	started := time.Now()
	select {
	case <-t.stopCh:
		t.logger.Infof("stopping timer for %q", runObj.GetRunKey())
		return
	case <-finished:
		t.logger.Infof("%q finished, stopping timer", runObj.GetRunKey())
		return
	case <-time.After(timeout):
		t.logger.Infof("timer for %q has activated after %s", runObj.GetRunKey(), time.Since(started).String())
		callback(runObj)
	}
}
