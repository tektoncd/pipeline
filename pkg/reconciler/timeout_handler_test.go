package reconciler

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/knative/pkg/apis"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/test"
	tb "github.com/tektoncd/pipeline/test/builder"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

var (
	testNs     = "foo"
	simpleStep = tb.Step("simple-step", testNs, tb.Command("/mycmd"))
	simpleTask = tb.Task("test-task", testNs, tb.TaskSpec(simpleStep))
)

func TestTaskRunCheckTimeouts(t *testing.T) {
	taskRunTimedout := tb.TaskRun("test-taskrun-run-timedout", testNs, tb.TaskRunSpec(
		tb.TaskRunTaskRef(simpleTask.Name, tb.TaskRefAPIVersion("a1")),
		tb.TaskRunTimeout(1*time.Second),
	), tb.TaskRunStatus(tb.Condition(apis.Condition{
		Type:   apis.ConditionSucceeded,
		Status: corev1.ConditionUnknown}),
		tb.TaskRunStartTime(time.Now().Add(-10*time.Second)),
	))

	taskRunRunning := tb.TaskRun("test-taskrun-running", testNs, tb.TaskRunSpec(
		tb.TaskRunTaskRef(simpleTask.Name, tb.TaskRefAPIVersion("a1")),
	), tb.TaskRunStatus(tb.Condition(apis.Condition{
		Type:   apis.ConditionSucceeded,
		Status: corev1.ConditionUnknown}),
		tb.TaskRunStartTime(time.Now()),
	))

	taskRunDone := tb.TaskRun("test-taskrun-completed", testNs, tb.TaskRunSpec(
		tb.TaskRunTaskRef(simpleTask.Name, tb.TaskRefAPIVersion("a1")),
	), tb.TaskRunStatus(tb.Condition(apis.Condition{
		Type:   apis.ConditionSucceeded,
		Status: corev1.ConditionTrue}),
	))

	taskRunCancelled := tb.TaskRun("test-taskrun-run-cancelled", testNs, tb.TaskRunSpec(
		tb.TaskRunTaskRef(simpleTask.Name),
		tb.TaskRunCancelled,
	), tb.TaskRunStatus(tb.Condition(apis.Condition{
		Type:   apis.ConditionSucceeded,
		Status: corev1.ConditionUnknown}),
	))

	d := test.Data{
		TaskRuns: []*v1alpha1.TaskRun{taskRunTimedout, taskRunRunning, taskRunDone, taskRunCancelled},
		Tasks:    []*v1alpha1.Task{simpleTask},
		Namespaces: []*corev1.Namespace{{
			ObjectMeta: metav1.ObjectMeta{
				Name: testNs,
			},
		}},
	}
	stopCh := make(chan struct{})
	defer close(stopCh)
	c, _ := test.SeedTestData(t, d)
	observer, _ := observer.New(zap.InfoLevel)
	th := NewTimeoutHandler(c.Kube, c.Pipeline, stopCh, zap.New(observer).Sugar())
	gotCallback := sync.Map{}
	f := func(tr interface{}) {
		trNew := tr.(*v1alpha1.TaskRun)
		gotCallback.Store(trNew.Name, struct{}{})
	}

	th.SetTaskRunCallbackFunc(f)
	th.CheckTimeouts()

	for _, tc := range []struct {
		name           string
		taskRun        *v1alpha1.TaskRun
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

func TestPipelinRunCheckTimeouts(t *testing.T) {
	simplePipeline := tb.Pipeline("test-pipeline", testNs, tb.PipelineSpec(
		tb.PipelineTask("hello-world-1", "hello-world"),
	))
	prTimeout := tb.PipelineRun("test-pipeline-run-with-timeout", testNs,
		tb.PipelineRunSpec("test-pipeline",
			tb.PipelineRunServiceAccount("test-sa"),
			tb.PipelineRunTimeout(&metav1.Duration{Duration: 1 * time.Second}),
		),
		tb.PipelineRunStatus(
			tb.PipelineRunStartTime(time.Now().AddDate(0, 0, -1))),
	)
	ts := tb.Task("hello-world", testNs)

	prRunning := tb.PipelineRun("test-pipeline-running", testNs,
		tb.PipelineRunSpec("test-pipeline"),
		tb.PipelineRunStatus(tb.PipelineRunStatusCondition(apis.Condition{
			Type:   apis.ConditionSucceeded,
			Status: corev1.ConditionUnknown}),
			tb.PipelineRunStartTime(time.Now()),
		),
	)
	prDone := tb.PipelineRun("test-pipeline-done", testNs,
		tb.PipelineRunSpec("test-pipeline"),
		tb.PipelineRunStatus(tb.PipelineRunStatusCondition(apis.Condition{
			Type:   apis.ConditionSucceeded,
			Status: corev1.ConditionTrue}),
		),
	)
	prCancelled := tb.PipelineRun("test-pipeline-cancel", testNs,
		tb.PipelineRunSpec("test-pipeline", tb.PipelineRunServiceAccount("test-sa"),
			tb.PipelineRunCancelled,
		),
		tb.PipelineRunStatus(tb.PipelineRunStatusCondition(apis.Condition{
			Type:   apis.ConditionSucceeded,
			Status: corev1.ConditionUnknown}),
		),
	)
	d := test.Data{
		PipelineRuns: []*v1alpha1.PipelineRun{prTimeout, prRunning, prDone, prCancelled},
		Pipelines:    []*v1alpha1.Pipeline{simplePipeline},
		Tasks:        []*v1alpha1.Task{ts},
		Namespaces: []*corev1.Namespace{{
			ObjectMeta: metav1.ObjectMeta{
				Name: testNs,
			},
		}},
	}
	c, _ := test.SeedTestData(t, d)
	stopCh := make(chan struct{})
	observer, _ := observer.New(zap.InfoLevel)
	th := NewTimeoutHandler(c.Kube, c.Pipeline, stopCh, zap.New(observer).Sugar())
	defer close(stopCh)

	gotCallback := sync.Map{}
	f := func(pr interface{}) {
		prNew := pr.(*v1alpha1.PipelineRun)
		gotCallback.Store(prNew.Name, struct{}{})
	}

	th.SetPipelineRunCallbackFunc(f)
	th.CheckTimeouts()
	for _, tc := range []struct {
		name           string
		pr             *v1alpha1.PipelineRun
		expectCallback bool
	}{{
		name:           "pr-running",
		pr:             prRunning,
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
	taskRunRunning := tb.TaskRun("test-taskrun-running", testNs, tb.TaskRunSpec(
		tb.TaskRunTaskRef(simpleTask.Name, tb.TaskRefAPIVersion("a1")),
		tb.TaskRunTimeout(2*time.Second),
	), tb.TaskRunStatus(tb.Condition(apis.Condition{
		Type:   apis.ConditionSucceeded,
		Status: corev1.ConditionUnknown}),
		tb.TaskRunStartTime(time.Now().Add(-10*time.Second)),
	))

	d := test.Data{
		TaskRuns: []*v1alpha1.TaskRun{taskRunRunning},
		Tasks:    []*v1alpha1.Task{simpleTask},
		Namespaces: []*corev1.Namespace{{
			ObjectMeta: metav1.ObjectMeta{
				Name: testNs,
			},
		}},
	}
	stopCh := make(chan struct{})
	c, _ := test.SeedTestData(t, d)
	observer, _ := observer.New(zap.InfoLevel)
	testHandler := NewTimeoutHandler(c.Kube, c.Pipeline, stopCh, zap.New(observer).Sugar())
	defer func() {
		// this delay will ensure there is no race condition between stopCh/ timeout channel getting triggered
		time.Sleep(10 * time.Millisecond)
		close(stopCh)
		if r := recover(); r != nil {
			t.Fatal("Expected CheckTimeouts function not to panic")
		}
	}()
	testHandler.CheckTimeouts()

}
