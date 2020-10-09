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
	"context"
	"math"
	"math/rand"
	"sync"

	"time"

	clientset "github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	"github.com/tektoncd/pipeline/pkg/contexts"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
)

const (
	maxBackoffSeconds = 120
)

var (
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

// Handler knows how to track channels that can be used to communicate with go routines
// which timeout, and the functions to call when that happens.
type Handler struct {
	logger *zap.SugaredLogger

	// callbackFunc is the function to call when a run has timed out.
	// This is usually set to the function that enqueues the namespaced name for reconciling.
	callbackFunc func(types.NamespacedName)
	// stopCh is used to signal to all goroutines that they should stop, e.g. because
	// the reconciler is stopping
	stopCh <-chan struct{}
	// done is a map from the name of the Run to the channel to use to indicate that the
	// Run is done (and so there is no need to wait on it any longer)
	done map[string]chan bool
	// doneMut is a mutex that protects access to done to ensure that multiple goroutines
	// don't try to update it simultaneously
	doneMut     sync.Mutex
	backoffs    map[string]Backoff
	backoffsMut sync.Mutex
}

// NewHandler returns an instance of Handler with the specified stopCh and logger, instantiated
// and ready to track go routines.
func NewHandler(
	stopCh <-chan struct{},
	logger *zap.SugaredLogger,
) *Handler {
	return &Handler{
		stopCh:   stopCh,
		done:     make(map[string]chan bool),
		backoffs: make(map[string]Backoff),
		logger:   logger,
	}
}

// SetCallbackFunc sets the callback function when timeout occurs
func (t *Handler) SetCallbackFunc(f func(types.NamespacedName)) {
	t.callbackFunc = f
}

// Release deletes channels and data that are specific to a namespacedName
func (t *Handler) Release(n types.NamespacedName) {
	t.doneMut.Lock()
	defer t.doneMut.Unlock()

	t.backoffsMut.Lock()
	defer t.backoffsMut.Unlock()

	if done, ok := t.done[n.String()]; ok {
		delete(t.done, n.String())
		close(done)
	}
	delete(t.backoffs, n.String())
}

func (t *Handler) getOrCreateDoneChan(n types.NamespacedName) chan bool {
	t.doneMut.Lock()
	defer t.doneMut.Unlock()
	var done chan bool
	var ok bool
	if done, ok = t.done[n.String()]; !ok {
		done = make(chan bool)
	}
	t.done[n.String()] = done
	return done
}

// GetBackoff records the number of times it has seen n and calculates an
// appropriate backoff deadline based on that count. Only one backoff per n
// may be active at any moment. Requests for a new backoff in the face of an
// existing one will be ignored and details of the existing backoff will be returned
// instead. Further, if a calculated backoff time is after the timeout of the runKey
// then the time of the timeout will be returned instead.
//
// Returned values are a backoff struct containing a NumAttempts field with the
// number of attempts performed for this n and a NextAttempt field
// describing the time at which the next attempt should be performed.
// Additionally a boolean is returned indicating whether a backoff for n
// is already in progress.
func (t *Handler) GetBackoff(n types.NamespacedName, startTime metav1.Time, timeout metav1.Duration) (Backoff, bool) {
	t.backoffsMut.Lock()
	defer t.backoffsMut.Unlock()
	b := t.backoffs[n.String()]
	if time.Now().Before(b.NextAttempt) {
		return b, true
	}
	b.NumAttempts++
	b.NextAttempt = time.Now().Add(backoffDuration(b.NumAttempts, rand.Intn))
	timeoutDeadline := startTime.Time.Add(timeout.Duration)
	if timeoutDeadline.Before(b.NextAttempt) {
		b.NextAttempt = timeoutDeadline
	}
	t.backoffs[n.String()] = b
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
func (t *Handler) checkPipelineRunTimeouts(ctx context.Context, namespace string, pipelineclientset clientset.Interface) {
	pipelineRuns, err := pipelineclientset.TektonV1beta1().PipelineRuns(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		t.logger.Errorf("Can't get pipelinerun list in namespace %s: %s", namespace, err)
		return
	}
	for _, pipelineRun := range pipelineRuns.Items {
		pipelineRun := pipelineRun
		pipelineRun.SetDefaults(contexts.WithUpgradeViaDefaulting(ctx))
		if pipelineRun.IsDone() || pipelineRun.IsCancelled() {
			continue
		}
		if pipelineRun.HasStarted() {
			go t.Wait(pipelineRun.GetNamespacedName(), *pipelineRun.Status.StartTime, *pipelineRun.Spec.Timeout)
		}
	}
}

// CheckTimeouts function iterates through a given namespace or all namespaces
// (if empty string) and calls corresponding taskrun/pipelinerun timeout functions
func (t *Handler) CheckTimeouts(ctx context.Context, namespace string, kubeclientset kubernetes.Interface, pipelineclientset clientset.Interface) {
	// scoped namespace
	namespaceNames := []string{namespace}
	// all namespaces
	if namespace == "" {
		namespaces, err := kubeclientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
		if err != nil {
			t.logger.Errorf("Can't get namespaces list: %s", err)
			return
		}
		namespaceNames = make([]string, len(namespaces.Items))
		for i, namespace := range namespaces.Items {
			namespaceNames[i] = namespace.GetName()
		}
	}

	for _, namespace := range namespaceNames {
		t.checkTaskRunTimeouts(ctx, namespace, pipelineclientset)
		t.checkPipelineRunTimeouts(ctx, namespace, pipelineclientset)
	}
}

// checkTaskRunTimeouts function creates goroutines to wait for pipelinerun to
// finish/timeout in a given namespace
func (t *Handler) checkTaskRunTimeouts(ctx context.Context, namespace string, pipelineclientset clientset.Interface) {
	taskruns, err := pipelineclientset.TektonV1beta1().TaskRuns(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		t.logger.Errorf("Can't get taskrun list in namespace %s: %s", namespace, err)
		return
	}
	for _, taskrun := range taskruns.Items {
		taskrun := taskrun
		taskrun.SetDefaults(contexts.WithUpgradeViaDefaulting(ctx))
		if taskrun.IsDone() || taskrun.IsCancelled() {
			continue
		}
		if taskrun.HasStarted() {
			go t.Wait(taskrun.GetNamespacedName(), *taskrun.Status.StartTime, *taskrun.Spec.Timeout)
		}
	}
}

// Wait creates a blocking function for n to wait for
// 1. Stop signal, 2. Completion 3. Or duration to elapse since startTime
// time out, which is determined by checking if the timeout has occurred since the startTime
func (t *Handler) Wait(n types.NamespacedName, startTime metav1.Time, timeout metav1.Duration) {
	if t.callbackFunc == nil {
		t.logger.Errorf("somehow the timeout handler was not initialized with a callback function")
		return
	}
	runtime := time.Since(startTime.Time)
	t.logger.Infof("About to start timeout timer for %s. started at %s, timeout is %s, running for %s", n.String(), startTime.Time, timeout, runtime)
	defer t.Release(n)
	t.setTimer(n, timeout.Duration-runtime, t.callbackFunc)
}

// SetTimer creates a blocking function for n to wait for
// 1. Stop signal, 2. TaskRun to complete or 3. a given Duration to elapse.
//
// Since the timer's duration is a parameter rather than being tied to
// the lifetime of the run object no resources are released after the timer
// fires. It is the caller's responsibility to Release() the run when
// work with it has completed.
func (t *Handler) SetTimer(n types.NamespacedName, d time.Duration) {
	if t.callbackFunc == nil {
		t.logger.Errorf("somehow the timeout handler was not initialized with a callback function")
		return
	}
	t.setTimer(n, d, t.callbackFunc)
}

func (t *Handler) setTimer(n types.NamespacedName, timeout time.Duration, callback func(types.NamespacedName)) {
	done := t.getOrCreateDoneChan(n)
	started := time.Now()
	select {
	case <-t.stopCh:
		t.logger.Infof("stopping timer for %q", n.String())
		return
	case <-done:
		t.logger.Infof("%q finished, stopping timer", n.String())
		return
	case <-time.After(timeout):
		t.logger.Infof("timer for %q has activated after %s", n.String(), time.Since(started).String())
		callback(n)
	}
}
