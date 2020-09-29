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
	"fmt"
	"sync"
	"testing"
	"time"

	tb "github.com/tektoncd/pipeline/internal/builder/v1beta1"
	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	ttesting "github.com/tektoncd/pipeline/pkg/reconciler/testing"
	test "github.com/tektoncd/pipeline/test"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"knative.dev/pkg/apis"
)

var (
	allNs      = ""
	testNs     = "foo"
	simpleStep = tb.Step(testNs, tb.StepCommand("/mycmd"))
	simpleTask = tb.Task("test-task", tb.TaskSpec(simpleStep))
)

func TestTaskRunCheckTimeouts(t *testing.T) {
	taskRunTimedout := tb.TaskRun("test-taskrun-run-timedout", tb.TaskRunNamespace(testNs), tb.TaskRunSpec(
		tb.TaskRunTaskRef(simpleTask.Name, tb.TaskRefAPIVersion("a1")),
		tb.TaskRunTimeout(1*time.Second),
	), tb.TaskRunStatus(tb.StatusCondition(apis.Condition{
		Type:   apis.ConditionSucceeded,
		Status: corev1.ConditionUnknown}),
		tb.TaskRunStartTime(time.Now().Add(-10*time.Second)),
	))

	taskRunRunning := tb.TaskRun("test-taskrun-running", tb.TaskRunNamespace(testNs), tb.TaskRunSpec(
		tb.TaskRunTaskRef(simpleTask.Name, tb.TaskRefAPIVersion("a1")),
		tb.TaskRunTimeout(config.DefaultTimeoutMinutes*time.Minute),
	), tb.TaskRunStatus(tb.StatusCondition(apis.Condition{
		Type:   apis.ConditionSucceeded,
		Status: corev1.ConditionUnknown}),
		tb.TaskRunStartTime(time.Now()),
	))

	taskRunRunningNilTimeout := tb.TaskRun("test-taskrun-running-nil-timeout", tb.TaskRunNamespace(testNs), tb.TaskRunSpec(
		tb.TaskRunTaskRef(simpleTask.Name, tb.TaskRefAPIVersion("a1")),
		tb.TaskRunNilTimeout,
	), tb.TaskRunStatus(tb.StatusCondition(apis.Condition{
		Type:   apis.ConditionSucceeded,
		Status: corev1.ConditionUnknown}),
		tb.TaskRunStartTime(time.Now()),
	))

	taskRunDone := tb.TaskRun("test-taskrun-completed", tb.TaskRunNamespace(testNs), tb.TaskRunSpec(
		tb.TaskRunTaskRef(simpleTask.Name, tb.TaskRefAPIVersion("a1")),
		tb.TaskRunTimeout(config.DefaultTimeoutMinutes*time.Minute),
	), tb.TaskRunStatus(tb.StatusCondition(apis.Condition{
		Type:   apis.ConditionSucceeded,
		Status: corev1.ConditionTrue}),
	))

	taskRunCancelled := tb.TaskRun("test-taskrun-run-cancelled", tb.TaskRunNamespace(testNs), tb.TaskRunSpec(
		tb.TaskRunTaskRef(simpleTask.Name),
		tb.TaskRunCancelled,
		tb.TaskRunTimeout(1*time.Second),
	), tb.TaskRunStatus(tb.StatusCondition(apis.Condition{
		Type:   apis.ConditionSucceeded,
		Status: corev1.ConditionUnknown}),
	))

	d := test.Data{
		TaskRuns: []*v1beta1.TaskRun{taskRunTimedout, taskRunRunning, taskRunDone, taskRunCancelled, taskRunRunningNilTimeout},
		Tasks:    []*v1beta1.Task{simpleTask},
		Namespaces: []*corev1.Namespace{{
			ObjectMeta: metav1.ObjectMeta{
				Name: testNs,
			},
		}},
	}
	ctx, _ := ttesting.SetupFakeContext(t)
	c, _ := test.SeedTestData(t, ctx, d)
	stopCh := make(chan struct{})
	defer close(stopCh)
	observer, _ := observer.New(zap.InfoLevel)

	th := NewHandler(stopCh, zap.New(observer).Sugar())
	gotCallback := sync.Map{}
	f := func(n types.NamespacedName) {
		gotCallback.Store(n.Name, struct{}{})
	}

	th.SetCallbackFunc(f)
	go th.CheckTimeouts(context.Background(), testNs, c.Kube, c.Pipeline)

	for _, tc := range []struct {
		name           string
		taskRun        *v1beta1.TaskRun
		expectCallback bool
	}{{
		name:           "timedout",
		taskRun:        taskRunTimedout,
		expectCallback: true,
	}, {
		name:           "running",
		taskRun:        taskRunRunning,
		expectCallback: false,
	}, {
		name:           "running-with-nil-timeout",
		taskRun:        taskRunRunningNilTimeout,
		expectCallback: false,
	}, {
		name:           "completed",
		taskRun:        taskRunDone,
		expectCallback: false,
	}, {
		name:           "cancelled",
		taskRun:        taskRunCancelled,
		expectCallback: false,
	}} {
		t.Run(tc.name, func(t *testing.T) {
			if err := wait.PollImmediate(100*time.Millisecond, 3*time.Second, func() (bool, error) {
				if tc.expectCallback {
					if _, ok := gotCallback.Load(tc.taskRun.Name); ok {
						return true, nil
					}
					return false, nil
				}
				// not expecting callback
				if _, ok := gotCallback.Load(tc.taskRun.Name); ok {
					return false, fmt.Errorf("did not expect call back for %s why", tc.taskRun.Name)
				}
				return true, nil
			}); err != nil {
				t.Fatalf("Expected %s callback to be %t but got error: %s", tc.name, tc.expectCallback, err)
			}
		})
	}

}

func TestTaskRunSingleNamespaceCheckTimeouts(t *testing.T) {
	taskRunTimedout := tb.TaskRun("test-taskrun-run-timedout-foo", tb.TaskRunNamespace(testNs), tb.TaskRunSpec(
		tb.TaskRunTaskRef(simpleTask.Name, tb.TaskRefAPIVersion("a1")),
		tb.TaskRunTimeout(1*time.Second),
	), tb.TaskRunStatus(tb.StatusCondition(apis.Condition{
		Type:   apis.ConditionSucceeded,
		Status: corev1.ConditionUnknown}),
		tb.TaskRunStartTime(time.Now().Add(-10*time.Second)),
	))

	taskRunTimedoutOtherNS := tb.TaskRun("test-taskrun-run-timedout-bar", tb.TaskRunNamespace("otherNS"), tb.TaskRunSpec(
		tb.TaskRunTaskRef(simpleTask.Name, tb.TaskRefAPIVersion("a1")),
		tb.TaskRunTimeout(1*time.Second),
	), tb.TaskRunStatus(tb.StatusCondition(apis.Condition{
		Type:   apis.ConditionSucceeded,
		Status: corev1.ConditionUnknown}),
		tb.TaskRunStartTime(time.Now().Add(-10*time.Second)),
	))

	d := test.Data{
		TaskRuns: []*v1beta1.TaskRun{taskRunTimedout, taskRunTimedoutOtherNS},
		Tasks:    []*v1beta1.Task{simpleTask},
		Namespaces: []*corev1.Namespace{{
			ObjectMeta: metav1.ObjectMeta{
				Name: testNs,
			},
		}},
	}
	ctx, _ := ttesting.SetupFakeContext(t)
	c, _ := test.SeedTestData(t, ctx, d)
	stopCh := make(chan struct{})
	defer close(stopCh)
	observer, _ := observer.New(zap.InfoLevel)

	th := NewHandler(stopCh, zap.New(observer).Sugar())
	gotCallback := sync.Map{}
	f := func(n types.NamespacedName) {
		gotCallback.Store(n.Name, struct{}{})
	}

	th.SetCallbackFunc(f)
	// Note that since f843899a11d5d09b29fac750f72f4a7e4882f615 CheckTimeouts is always called
	// with a namespace so there is no reason to maintain all namespaces functionality;
	// however in #2905 we should remove CheckTimeouts completely
	go th.CheckTimeouts(context.Background(), "", c.Kube, c.Pipeline)

	for _, tc := range []struct {
		name           string
		taskRun        *v1beta1.TaskRun
		expectCallback bool
	}{{
		name:           "timedout",
		taskRun:        taskRunTimedout,
		expectCallback: true,
	}, {
		name:           "timedout",
		taskRun:        taskRunTimedoutOtherNS,
		expectCallback: false,
	}} {
		t.Run(tc.name, func(t *testing.T) {
			if err := wait.PollImmediate(100*time.Millisecond, 3*time.Second, func() (bool, error) {
				if tc.expectCallback {
					if _, ok := gotCallback.Load(tc.taskRun.Name); ok {
						return true, nil
					}
					return false, nil
				}
				// not expecting callback
				if _, ok := gotCallback.Load(tc.taskRun.Name); ok {
					return false, fmt.Errorf("did not expect call back for %s why", tc.taskRun.Name)
				}
				return true, nil
			}); err != nil {
				t.Fatalf("Expected %s callback to be %t but got error: %s", tc.name, tc.expectCallback, err)
			}
		})
	}

}

func TestPipelinRunCheckTimeouts(t *testing.T) {
	simplePipeline := tb.Pipeline("test-pipeline", tb.PipelineNamespace(testNs), tb.PipelineSpec(
		tb.PipelineTask("hello-world-1", "hello-world"),
	))
	prTimeout := tb.PipelineRun("test-pipeline-run-with-timeout", tb.PipelineRunNamespace(testNs),
		tb.PipelineRunSpec("test-pipeline",
			tb.PipelineRunServiceAccountName("test-sa"),
			tb.PipelineRunTimeout(1*time.Second),
		),
		tb.PipelineRunStatus(
			tb.PipelineRunStartTime(time.Now().AddDate(0, 0, -1))),
	)
	ts := tb.Task("hello-world")

	prRunning := tb.PipelineRun("test-pipeline-running", tb.PipelineRunNamespace(testNs),
		tb.PipelineRunSpec("test-pipeline",
			tb.PipelineRunTimeout(config.DefaultTimeoutMinutes*time.Minute),
		),
		tb.PipelineRunStatus(tb.PipelineRunStatusCondition(apis.Condition{
			Type:   apis.ConditionSucceeded,
			Status: corev1.ConditionUnknown}),
			tb.PipelineRunStartTime(time.Now()),
		),
	)
	prRunningNilTimeout := tb.PipelineRun("test-pipeline-running-nil-timeout", tb.PipelineRunNamespace(testNs),
		tb.PipelineRunSpec("test-pipeline",
			tb.PipelineRunNilTimeout,
		),
		tb.PipelineRunStatus(tb.PipelineRunStatusCondition(apis.Condition{
			Type:   apis.ConditionSucceeded,
			Status: corev1.ConditionUnknown}),
			tb.PipelineRunStartTime(time.Now()),
		),
	)
	prDone := tb.PipelineRun("test-pipeline-done", tb.PipelineRunNamespace(testNs),
		tb.PipelineRunSpec("test-pipeline",
			tb.PipelineRunTimeout(config.DefaultTimeoutMinutes*time.Minute),
		),
		tb.PipelineRunStatus(tb.PipelineRunStatusCondition(apis.Condition{
			Type:   apis.ConditionSucceeded,
			Status: corev1.ConditionTrue}),
		),
	)
	prCancelled := tb.PipelineRun("test-pipeline-cancel", tb.PipelineRunNamespace(testNs),
		tb.PipelineRunSpec("test-pipeline", tb.PipelineRunServiceAccountName("test-sa"),
			tb.PipelineRunCancelled,
			tb.PipelineRunTimeout(config.DefaultTimeoutMinutes*time.Minute),
		),
		tb.PipelineRunStatus(tb.PipelineRunStatusCondition(apis.Condition{
			Type:   apis.ConditionSucceeded,
			Status: corev1.ConditionUnknown}),
		),
	)
	d := test.Data{
		PipelineRuns: []*v1beta1.PipelineRun{prTimeout, prRunning, prDone, prCancelled, prRunningNilTimeout},
		Pipelines:    []*v1beta1.Pipeline{simplePipeline},
		Tasks:        []*v1beta1.Task{ts},
		Namespaces: []*corev1.Namespace{{
			ObjectMeta: metav1.ObjectMeta{
				Name: testNs,
			},
		}},
	}

	ctx, _ := ttesting.SetupFakeContext(t)
	c, _ := test.SeedTestData(t, ctx, d)
	stopCh := make(chan struct{})
	defer close(stopCh)
	observer, _ := observer.New(zap.InfoLevel)
	th := NewHandler(stopCh, zap.New(observer).Sugar())

	gotCallback := sync.Map{}
	f := func(n types.NamespacedName) {
		gotCallback.Store(n.Name, struct{}{})
	}

	th.SetCallbackFunc(f)
	go th.CheckTimeouts(context.Background(), allNs, c.Kube, c.Pipeline)
	for _, tc := range []struct {
		name           string
		pr             *v1beta1.PipelineRun
		expectCallback bool
	}{{
		name:           "pr-running",
		pr:             prRunning,
		expectCallback: false,
	}, {
		name:           "pr-running-nil-timeout",
		pr:             prRunningNilTimeout,
		expectCallback: false,
	}, {
		name:           "pr-timedout",
		pr:             prTimeout,
		expectCallback: true,
	}, {
		name:           "pr-completed",
		pr:             prDone,
		expectCallback: false,
	}, {
		name:           "pr-cancel",
		pr:             prCancelled,
		expectCallback: false,
	}} {
		t.Run(tc.name, func(t *testing.T) {
			if err := wait.PollImmediate(1*time.Second, 5*time.Second, func() (bool, error) {
				if tc.expectCallback {
					if _, ok := gotCallback.Load(tc.pr.Name); ok {
						return true, nil
					}
					return false, nil
				}
				// not expecting callback
				if _, ok := gotCallback.Load(tc.pr.Name); ok {
					return false, fmt.Errorf("did not expect call back for %s why", tc.pr.Name)
				}
				return true, nil
			}); err != nil {
				t.Fatalf("Expected %s callback to be %t but got error: %s", tc.name, tc.expectCallback, err)
			}
		})
	}
}

// TestWithNoFunc does not set taskrun/pipelinerun function and verifies that code does not panic
func TestWithNoFunc(t *testing.T) {
	taskRunRunning := tb.TaskRun("test-taskrun-running", tb.TaskRunNamespace(testNs), tb.TaskRunSpec(
		tb.TaskRunTaskRef(simpleTask.Name, tb.TaskRefAPIVersion("a1")),
		tb.TaskRunTimeout(2*time.Second),
	), tb.TaskRunStatus(tb.StatusCondition(apis.Condition{
		Type:   apis.ConditionSucceeded,
		Status: corev1.ConditionUnknown}),
		tb.TaskRunStartTime(time.Now().Add(-10*time.Second)),
	))

	d := test.Data{
		TaskRuns: []*v1beta1.TaskRun{taskRunRunning},
		Tasks:    []*v1beta1.Task{simpleTask},
		Namespaces: []*corev1.Namespace{{
			ObjectMeta: metav1.ObjectMeta{
				Name: testNs,
			},
		}},
	}
	ctx, _ := ttesting.SetupFakeContext(t)
	c, _ := test.SeedTestData(t, ctx, d)
	stopCh := make(chan struct{})
	observer, _ := observer.New(zap.InfoLevel)
	testHandler := NewHandler(stopCh, zap.New(observer).Sugar())
	defer func() {
		// this delay will ensure there is no race condition between stopCh/ timeout channel getting triggered
		time.Sleep(10 * time.Millisecond)
		close(stopCh)
		if r := recover(); r != nil {
			t.Fatal("Expected CheckTimeouts function not to panic")
		}
	}()
	go testHandler.CheckTimeouts(context.Background(), allNs, c.Kube, c.Pipeline)

}

// TestSetTaskRunTimer checks that the SetTaskRunTimer method correctly calls the TaskRun
// callback after a set amount of time.
func TestSetTaskRunTimer(t *testing.T) {
	taskRun := tb.TaskRun("test-taskrun-arbitrary-timer", tb.TaskRunNamespace(testNs), tb.TaskRunSpec(
		tb.TaskRunTaskRef(simpleTask.Name, tb.TaskRefAPIVersion("a1")),
		tb.TaskRunTimeout(50*time.Millisecond),
	), tb.TaskRunStatus(tb.StatusCondition(apis.Condition{
		Type:   apis.ConditionSucceeded,
		Status: corev1.ConditionUnknown}),
		tb.TaskRunStartTime(time.Now().Add(-10*time.Second)),
	))

	stopCh := make(chan struct{})
	observer, _ := observer.New(zap.InfoLevel)
	testHandler := NewHandler(stopCh, zap.New(observer).Sugar())
	timerDuration := 50 * time.Millisecond
	timerFailDeadline := 100 * time.Millisecond
	doneCh := make(chan struct{})
	f := func(_ types.NamespacedName) {
		close(doneCh)
	}
	testHandler.SetCallbackFunc(f)
	go testHandler.SetTimer(taskRun.GetNamespacedName(), timerDuration)
	select {
	case <-doneCh:
		// The task run timer executed before the failure deadline
	case <-time.After(timerFailDeadline):
		t.Errorf("timer did not execute task run callback func within expected time")
	}
}

// TestBackoffDuration asserts that the backoffDuration func returns Durations
// within the timeout handler's bounds.
func TestBackoffDuration(t *testing.T) {
	testcases := []struct {
		description      string
		inputCount       uint
		jitterFunc       func(int) int
		expectedDuration time.Duration
	}{
		{
			description:      "an input count that is too large is rounded to the maximum allowed exponent",
			inputCount:       uint(maxBackoffExponent + 1),
			jitterFunc:       func(in int) int { return in },
			expectedDuration: maxBackoffSeconds * time.Second,
		},
		{
			description:      "a jittered number of seconds that is above the maximum allowed is constrained",
			inputCount:       1,
			jitterFunc:       func(in int) int { return maxBackoffSeconds + 1 },
			expectedDuration: maxBackoffSeconds * time.Second,
		},
	}
	for _, tc := range testcases {
		t.Run(tc.description, func(t *testing.T) {
			result := backoffDuration(tc.inputCount, tc.jitterFunc)
			if result != tc.expectedDuration {
				t.Errorf("expected %q received %q", tc.expectedDuration.String(), result.String())
			}
		})
	}
}
