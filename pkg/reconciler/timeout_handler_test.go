package reconciler

import (
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/google/go-cmp/cmp"
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/test"
	tb "github.com/tektoncd/pipeline/test/builder"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
	corev1 "k8s.io/api/core/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	testNs     = "foo"
	simpleStep = tb.Step("simple-step", testNs, tb.Command("/mycmd"))
	simpleTask = tb.Task("test-task", testNs, tb.TaskSpec(simpleStep))
)

func setup(d test.Data, stopCh chan struct{}) (*TimeoutSet, test.Clients) {
	c, _ := test.SeedTestData(d)
	observer, _ := observer.New(zap.InfoLevel)
	return NewTimeoutHandler(zap.New(observer).Sugar(), c.Kube, c.Pipeline, stopCh), c
}

func TestTaskRunCheckTimeouts(t *testing.T) {
	taskRunTimedout := tb.TaskRun("test-taskrun-run-timedout", testNs, tb.TaskRunSpec(
		tb.TaskRunTaskRef(simpleTask.Name, tb.TaskRefAPIVersion("a1")),
		tb.TaskRunTimeout(1*time.Second),
	), tb.TaskRunStatus(tb.Condition(duckv1alpha1.Condition{
		Type:   duckv1alpha1.ConditionSucceeded,
		Status: corev1.ConditionUnknown}),
		tb.TaskRunStartTime(time.Now().Add(-10*time.Second)),
	))

	taskRunRunning := tb.TaskRun("test-taskrun-running", testNs, tb.TaskRunSpec(
		tb.TaskRunTaskRef(simpleTask.Name, tb.TaskRefAPIVersion("a1")),
		tb.TaskRunTimeout(2*time.Hour),
	), tb.TaskRunStatus(tb.Condition(duckv1alpha1.Condition{
		Type:   duckv1alpha1.ConditionSucceeded,
		Status: corev1.ConditionUnknown}),
		tb.TaskRunStartTime(time.Now()),
	))

	taskRunDone := tb.TaskRun("test-taskrun-completed", testNs, tb.TaskRunSpec(
		tb.TaskRunTaskRef(simpleTask.Name, tb.TaskRefAPIVersion("a1")),
	), tb.TaskRunStatus(tb.Condition(duckv1alpha1.Condition{
		Type:   duckv1alpha1.ConditionSucceeded,
		Status: corev1.ConditionTrue}),
	))

	taskRunCancelled := tb.TaskRun("test-taskrun-run-cancelled", testNs, tb.TaskRunSpec(
		tb.TaskRunTaskRef(simpleTask.Name),
		tb.TaskRunCancelled,
	), tb.TaskRunStatus(tb.Condition(duckv1alpha1.Condition{
		Type:   duckv1alpha1.ConditionSucceeded,
		Status: corev1.ConditionUnknown}),
	))

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
			t.Parallel()
			d := test.Data{
				TaskRuns: []*v1alpha1.TaskRun{tc.taskRun},
				Tasks:    []*v1alpha1.Task{simpleTask},
				Namespaces: []*corev1.Namespace{&corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: testNs,
					},
				}},
			}
			stopCh := make(chan struct{})
			defer close(stopCh)
			testHandler, _ := setup(d, stopCh)
			var gotCallback bool
			f := func(tr interface{}) {
				gotCallback = true
			}
			testHandler.AddtrCallBackFunc(f)
			testHandler.CheckTimeouts()

			if err := wait.PollImmediate(100*time.Millisecond, 3*time.Second, func() (bool, error) {
				if !cmp.Equal(gotCallback, tc.expectCallback) {
					return false, nil
				}
				return true, nil
			}); err != nil {
				t.Fatalf("Expected %s callback to be %t but got callback to be %t", tc.name, tc.expectCallback, gotCallback)
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
		tb.PipelineRunStatus(tb.PipelineRunStatusCondition(duckv1alpha1.Condition{
			Type:   duckv1alpha1.ConditionSucceeded,
			Status: corev1.ConditionUnknown}),
		),
	)
	prDone := tb.PipelineRun("test-pipeline-done", testNs,
		tb.PipelineRunSpec("test-pipeline"),
		tb.PipelineRunStatus(tb.PipelineRunStatusCondition(duckv1alpha1.Condition{
			Type:   duckv1alpha1.ConditionSucceeded,
			Status: corev1.ConditionTrue}),
		),
	)
	prCancelled := tb.PipelineRun("test-pipeline-cancel", testNs,
		tb.PipelineRunSpec("test-pipeline", tb.PipelineRunServiceAccount("test-sa"),
			tb.PipelineRunCancelled,
		),
		tb.PipelineRunStatus(tb.PipelineRunStatusCondition(duckv1alpha1.Condition{
			Type:   duckv1alpha1.ConditionSucceeded,
			Status: corev1.ConditionUnknown}),
		),
	)

	for _, tc := range []struct {
		name           string
		pr             *v1alpha1.PipelineRun
		expectCallback bool
	}{{
		name:           "pr-timedout",
		pr:             prTimeout,
		expectCallback: true,
	}, {
		name:           "pr-running",
		pr:             prRunning,
		expectCallback: false,
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
			t.Parallel()
			d := test.Data{
				PipelineRuns: []*v1alpha1.PipelineRun{tc.pr},
				Pipelines:    []*v1alpha1.Pipeline{simplePipeline},
				Tasks:        []*v1alpha1.Task{ts},
				Namespaces: []*corev1.Namespace{&corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: testNs,
					},
				}},
			}
			stopCh := make(chan struct{})
			var gotCallback bool
			f := func(tr interface{}) {
				gotCallback = true
			}
			testHandler, _ := setup(d, stopCh)
			testHandler.AddPrCallBackFunc(f)
			testHandler.CheckTimeouts()
			defer close(stopCh)

			if err := wait.PollImmediate(100*time.Millisecond, 5*time.Second, func() (bool, error) {
				if !cmp.Equal(gotCallback, tc.expectCallback) {
					return false, nil
				}
				return true, nil
			}); err != nil {
				t.Fatalf("Expected %s callback to be %t but got callback to be %t", tc.name, tc.expectCallback, gotCallback)
			}
		})
	}
}

// TestWithNoFunc does not set taskrun/pipelinerun function and verifies that code does not panic
func TestWithNoFunc(t *testing.T) {
	taskRunRunning := tb.TaskRun("test-taskrun-running", testNs, tb.TaskRunSpec(
		tb.TaskRunTaskRef(simpleTask.Name, tb.TaskRefAPIVersion("a1")),
		tb.TaskRunTimeout(2*time.Second),
	), tb.TaskRunStatus(tb.Condition(duckv1alpha1.Condition{
		Type:   duckv1alpha1.ConditionSucceeded,
		Status: corev1.ConditionUnknown}),
		tb.TaskRunStartTime(time.Now().Add(-10*time.Second)),
	))

	d := test.Data{
		TaskRuns: []*v1alpha1.TaskRun{taskRunRunning},
		Tasks:    []*v1alpha1.Task{simpleTask},
		Namespaces: []*corev1.Namespace{&corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: testNs,
			},
		}},
	}
	stopCh := make(chan struct{})
	testHandler, _ := setup(d, stopCh)
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
