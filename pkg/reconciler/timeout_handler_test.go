package reconciler

import (
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/knative/build-pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/knative/build-pipeline/test"
	tb "github.com/knative/build-pipeline/test/builder"
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/wait"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	testNs                   = "foo"
	ignoreLastTransitionTime = cmpopts.IgnoreTypes(duckv1alpha1.Condition{}.LastTransitionTime.Inner.Time)
	simpleStep               = tb.Step("simple-step", testNs, tb.Command("/mycmd"))
	simpleTask               = tb.Task("test-task", testNs, tb.TaskSpec(simpleStep))
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
		tb.PodName("test-taskrun-run-not-timedout-pod-9999"),
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

	taskRunNoCondition := tb.TaskRun("test-taskrun-run-no-cond", testNs, tb.TaskRunSpec(
		tb.TaskRunTaskRef(simpleTask.Name),
	))

	d := test.Data{
		TaskRuns: []*v1alpha1.TaskRun{taskRunTimedout, taskRunRunning, taskRunDone, taskRunCancelled, taskRunNoCondition},
		Tasks:    []*v1alpha1.Task{simpleTask},
		Namespaces: []*corev1.Namespace{&corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: testNs,
			},
		}},
	}
	stopCh := make(chan struct{})
	testHandler, c := setup(d, stopCh)
	testHandler.CheckTimeouts()
	defer close(stopCh)

	for _, tc := range []struct {
		name       string
		taskRun    *v1alpha1.TaskRun
		wantStatus *duckv1alpha1.Condition
	}{{
		name:    "timedout",
		taskRun: taskRunTimedout,
		wantStatus: &duckv1alpha1.Condition{
			Type:    duckv1alpha1.ConditionSucceeded,
			Status:  corev1.ConditionFalse,
			Reason:  "Timeout",
			Message: `TaskRun "test-taskrun-run-timedout" failed to finish within "1s"`,
		},
	}, {
		name:    "running",
		taskRun: taskRunRunning,
		wantStatus: &duckv1alpha1.Condition{
			Type:   duckv1alpha1.ConditionSucceeded,
			Status: corev1.ConditionUnknown,
		},
	}, {
		name:    "completed",
		taskRun: taskRunDone,
		wantStatus: &duckv1alpha1.Condition{
			Type:   duckv1alpha1.ConditionSucceeded,
			Status: corev1.ConditionTrue,
		},
	}, {
		name:    "cancelled",
		taskRun: taskRunCancelled,
		wantStatus: &duckv1alpha1.Condition{
			Type:   duckv1alpha1.ConditionSucceeded,
			Status: corev1.ConditionUnknown,
		},
	}, {
		name:       "no-condition",
		taskRun:    taskRunNoCondition,
		wantStatus: nil,
	}} {
		t.Run(tc.name, func(t *testing.T) {
			if err := wait.PollImmediate(100*time.Millisecond, 5*time.Second, func() (bool, error) {
				reconciledRun, err := c.Pipeline.TektonV1alpha1().TaskRuns(testNs).Get(tc.taskRun.Name, metav1.GetOptions{})
				if err != nil {
					return false, err
				}
				cond := reconciledRun.Status.GetCondition(duckv1alpha1.ConditionSucceeded)
				d := cmp.Diff(cond, tc.wantStatus, ignoreLastTransitionTime)
				if d != "" {
					return false, nil
				}
				return true, nil
			}); err != nil {
				t.Fatalf("Expected Taskrun to be timed out, but got error %v", err)
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
	prNoCondition := tb.PipelineRun("test-pipeline-no-cond", testNs,
		tb.PipelineRunSpec("test-pipeline", tb.PipelineRunServiceAccount("test-sa")),
	)

	prWithTaskrunStatus := tb.PipelineRun("test-pipeline-taskrun", testNs,
		tb.PipelineRunSpec("test-pipeline"),
		tb.PipelineRunStatus(tb.PipelineRunStatusCondition(duckv1alpha1.Condition{
			Type:   duckv1alpha1.ConditionSucceeded,
			Status: corev1.ConditionUnknown}),
		),
	)
	taskRunsStatus := make(map[string]*v1alpha1.PipelineRunTaskRunStatus)
	taskRunsStatus["test-pipeline-taskrun-test-1"] = &v1alpha1.PipelineRunTaskRunStatus{
		PipelineTaskName: "unit-test-1",
		Status: &v1alpha1.TaskRunStatus{
			Steps: []v1alpha1.StepState{{
				ContainerState: corev1.ContainerState{
					Terminated: &corev1.ContainerStateTerminated{ExitCode: 0},
				},
			}},
			Conditions: []duckv1alpha1.Condition{{
				Type: duckv1alpha1.ConditionSucceeded,
			}},
		},
	}
	prWithTaskrunStatus.Status.TaskRuns = taskRunsStatus

	d := test.Data{
		PipelineRuns: []*v1alpha1.PipelineRun{prTimeout, prRunning, prDone, prCancelled, prNoCondition, prWithTaskrunStatus},
		Pipelines:    []*v1alpha1.Pipeline{simplePipeline},
		Tasks:        []*v1alpha1.Task{ts},
		Namespaces: []*corev1.Namespace{&corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: testNs,
			},
		}},
	}
	stopCh := make(chan struct{})
	testHandler, c := setup(d, stopCh)
	testHandler.CheckTimeouts()
	defer close(stopCh)

	for _, tc := range []struct {
		name       string
		pr         *v1alpha1.PipelineRun
		wantStatus *duckv1alpha1.Condition
	}{{
		name: "pr-timedout",
		pr:   prTimeout,
		wantStatus: &duckv1alpha1.Condition{
			Type:    duckv1alpha1.ConditionSucceeded,
			Status:  corev1.ConditionFalse,
			Reason:  "Timeout",
			Message: `PipelineRun "test-pipeline-run-with-timeout" failed to finish within "1s"`,
		},
	}, {
		name: "pr-running",
		pr:   prRunning,
		wantStatus: &duckv1alpha1.Condition{
			Type:   duckv1alpha1.ConditionSucceeded,
			Status: corev1.ConditionUnknown,
		},
	}, {
		name: "pr-completed",
		pr:   prDone,
		wantStatus: &duckv1alpha1.Condition{
			Type:   duckv1alpha1.ConditionSucceeded,
			Status: corev1.ConditionTrue,
		},
	}, {
		name: "pr-cancel",
		pr:   prCancelled,
		wantStatus: &duckv1alpha1.Condition{
			Type:   duckv1alpha1.ConditionSucceeded,
			Status: corev1.ConditionUnknown,
		},
	}, {
		name:       "pr-no-condition",
		pr:         prNoCondition,
		wantStatus: nil,
	}, {
		name: "pr-with-taskrunStatus",
		pr:   prWithTaskrunStatus,
		wantStatus: &duckv1alpha1.Condition{
			Type:   duckv1alpha1.ConditionSucceeded,
			Status: corev1.ConditionUnknown,
		},
	}} {
		t.Run(tc.name, func(t *testing.T) {
			if err := wait.PollImmediate(100*time.Millisecond, 5*time.Second, func() (bool, error) {
				reconciledRun, err := c.Pipeline.TektonV1alpha1().PipelineRuns(testNs).Get(tc.pr.Name, metav1.GetOptions{})
				if err != nil {
					return false, err
				}
				cond := reconciledRun.Status.GetCondition(duckv1alpha1.ConditionSucceeded)
				if d := cmp.Diff(cond, tc.wantStatus, ignoreLastTransitionTime); d != "" {
					return false, nil
				}
				return true, nil
			}); err != nil {
				t.Fatalf("Expected Taskrun to be timed out, but got error %v", err)
			}
		})
	}
}

func TestTaskRunMultipleReconciler(t *testing.T) {
	taskRunRunning := tb.TaskRun("test-taskrun-running", testNs, tb.TaskRunSpec(
		tb.TaskRunTaskRef(simpleTask.Name, tb.TaskRefAPIVersion("a1")),
		tb.TaskRunTimeout(2*time.Hour),
	), tb.TaskRunStatus(tb.Condition(duckv1alpha1.Condition{
		Type:   duckv1alpha1.ConditionSucceeded,
		Status: corev1.ConditionUnknown}),
		tb.TaskRunStartTime(time.Now()),
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
	expectStatus := &duckv1alpha1.Condition{
		Type:   duckv1alpha1.ConditionSucceeded,
		Status: corev1.ConditionUnknown,
	}

	stopCh := make(chan struct{})
	testHandler, c := setup(d, stopCh)
	testHandler.CheckTimeouts()
	defer close(stopCh)

	go testHandler.WaitTaskRun(taskRunRunning)
	if err := wait.PollImmediate(100*time.Millisecond, 5*time.Second, func() (bool, error) {
		reconciledRun, err := c.Pipeline.TektonV1alpha1().TaskRuns(testNs).Get(taskRunRunning.Name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		cond := reconciledRun.Status.GetCondition(duckv1alpha1.ConditionSucceeded)
		d := cmp.Diff(cond, expectStatus, ignoreLastTransitionTime)
		if d != "" {
			return false, nil
		}
		return true, nil
	}); err != nil {
		t.Fatalf("Expected Taskrun to have status %#v , but got error %v", expectStatus, err)
	}

	go testHandler.WaitTaskRun(taskRunRunning)
	if err := wait.PollImmediate(100*time.Millisecond, 5*time.Second, func() (bool, error) {
		reconciledRun, err := c.Pipeline.TektonV1alpha1().TaskRuns(testNs).Get(taskRunRunning.Name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		cond := reconciledRun.Status.GetCondition(duckv1alpha1.ConditionSucceeded)
		d := cmp.Diff(cond, expectStatus, ignoreLastTransitionTime)
		if d != "" {
			return false, nil
		}
		return true, nil
	}); err != nil {
		t.Fatalf("Expected Taskrun to have status %#v , but got error %v", expectStatus, err)
	}
}

func TestPipelineRunMultipleReconciler(t *testing.T) {
	simplePipeline := tb.Pipeline("test-pipeline", testNs, tb.PipelineSpec(
		tb.PipelineTask("hello-world-1", "hello-world"),
	))

	ts := tb.Task("hello-world", testNs)
	prRunning := tb.PipelineRun("test-pipeline-running", testNs,
		tb.PipelineRunSpec("test-pipeline"),
		tb.PipelineRunStatus(tb.PipelineRunStatusCondition(duckv1alpha1.Condition{
			Type:   duckv1alpha1.ConditionSucceeded,
			Status: corev1.ConditionUnknown}),
		),
	)
	d := test.Data{
		PipelineRuns: []*v1alpha1.PipelineRun{prRunning},
		Pipelines:    []*v1alpha1.Pipeline{simplePipeline},
		Tasks:        []*v1alpha1.Task{ts},
		Namespaces: []*corev1.Namespace{&corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: testNs,
			},
		}},
	}
	expectStatus := &duckv1alpha1.Condition{
		Type:   duckv1alpha1.ConditionSucceeded,
		Status: corev1.ConditionUnknown,
	}

	stopCh := make(chan struct{})
	testHandler, c := setup(d, stopCh)
	testHandler.CheckTimeouts()
	defer close(stopCh)

	go testHandler.WaitPipelineRun(prRunning)
	if err := wait.PollImmediate(100*time.Millisecond, 5*time.Second, func() (bool, error) {
		reconciledRun, err := c.Pipeline.TektonV1alpha1().PipelineRuns(testNs).Get(prRunning.Name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		cond := reconciledRun.Status.GetCondition(duckv1alpha1.ConditionSucceeded)
		d := cmp.Diff(cond, expectStatus, ignoreLastTransitionTime)
		if d != "" {
			return false, nil
		}
		return true, nil
	}); err != nil {
		t.Fatalf("Expected Pipeline to have status %#v , but got error %v", expectStatus, err)
	}

	go testHandler.WaitPipelineRun(prRunning)
	if err := wait.PollImmediate(100*time.Millisecond, 5*time.Second, func() (bool, error) {
		reconciledRun, err := c.Pipeline.TektonV1alpha1().PipelineRuns(testNs).Get(prRunning.Name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		cond := reconciledRun.Status.GetCondition(duckv1alpha1.ConditionSucceeded)
		d := cmp.Diff(cond, expectStatus, ignoreLastTransitionTime)
		if d != "" {
			return false, nil
		}
		return true, nil
	}); err != nil {
		t.Fatalf("Expected Taskrun to be timed out, but got error %v", err)
	}
}

func TestPipelineRunWithTaskRunStatus(t *testing.T) {
	simplePipeline := tb.Pipeline("test-pipeline", testNs, tb.PipelineSpec(
		tb.PipelineTask("hello-world-1", "hello-world"),
	))

	ts := tb.Task("hello-world", testNs)

	prTimedoutWithTaskrunStatus := tb.PipelineRun("test-pipeline-pr", testNs,
		tb.PipelineRunSpec("test-pipeline",
			tb.PipelineRunTimeout(&metav1.Duration{Duration: 1 * time.Second})),
		tb.PipelineRunStatus(tb.PipelineRunStatusCondition(duckv1alpha1.Condition{
			Type:   duckv1alpha1.ConditionSucceeded,
			Status: corev1.ConditionUnknown}),
			tb.PipelineRunStartTime(time.Now().Add(-15*time.Second)),
		),
	)
	taskRunRunning := tb.TaskRun("test-taskrun-running", testNs, tb.TaskRunSpec(
		tb.TaskRunTaskRef(simpleTask.Name, tb.TaskRefAPIVersion("a1")),
	), tb.TaskRunStatus(tb.Condition(duckv1alpha1.Condition{
		Type:   duckv1alpha1.ConditionSucceeded,
		Status: corev1.ConditionUnknown}),
		tb.TaskRunStartTime(time.Now().Add(-15*time.Second)),
		tb.PodName("test-pod"),
	))

	taskRunsStatus := make(map[string]*v1alpha1.PipelineRunTaskRunStatus)
	taskRunsStatus[taskRunRunning.Name] = &v1alpha1.PipelineRunTaskRunStatus{
		PipelineTaskName: "unit-test-1",
		Status:           &taskRunRunning.Status,
	}
	prTimedoutWithTaskrunStatus.Status.TaskRuns = taskRunsStatus

	d := test.Data{
		PipelineRuns: []*v1alpha1.PipelineRun{prTimedoutWithTaskrunStatus},
		Pipelines:    []*v1alpha1.Pipeline{simplePipeline},
		Tasks:        []*v1alpha1.Task{ts},
		TaskRuns:     []*v1alpha1.TaskRun{taskRunRunning},
		Namespaces: []*corev1.Namespace{&corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: testNs,
			},
		}},
		Pods: []*corev1.Pod{&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-pod",
				Namespace: testNs,
			},
		}},
	}
	stopCh := make(chan struct{})
	testHandler, c := setup(d, stopCh)
	testHandler.CheckTimeouts()
	defer close(stopCh)
	expectPrStatus := &duckv1alpha1.Condition{
		Type:    duckv1alpha1.ConditionSucceeded,
		Status:  corev1.ConditionFalse,
		Reason:  "Timeout",
		Message: fmt.Sprintf(`PipelineRun %q failed to finish within "1s"`, prTimedoutWithTaskrunStatus.Name),
	}
	if err := wait.PollImmediate(100*time.Millisecond, 5*time.Second, func() (bool, error) {
		reconciledRun, err := c.Pipeline.TektonV1alpha1().PipelineRuns(testNs).Get(prTimedoutWithTaskrunStatus.Name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		cond := reconciledRun.Status.GetCondition(duckv1alpha1.ConditionSucceeded)
		d := cmp.Diff(cond, expectPrStatus, ignoreLastTransitionTime)
		if d != "" {
			return false, nil
		}
		return true, nil
	}); err != nil {
		t.Fatalf("Expected Taskrun to be timed out, but got error %v", err)
	}
	expectTrStatus := &duckv1alpha1.Condition{
		Type:    duckv1alpha1.ConditionSucceeded,
		Status:  corev1.ConditionFalse,
		Reason:  "Timeout",
		Message: fmt.Sprintf(`TaskRun %q failed to finish within "1s"`, taskRunRunning.Name),
	}

	tr, err := c.Pipeline.TektonV1alpha1().TaskRuns(testNs).Get(taskRunRunning.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Expected taskrun to be timed out %s", err)
	}
	cond := tr.Status.GetCondition(duckv1alpha1.ConditionSucceeded)
	if d := cmp.Diff(cond, expectTrStatus, ignoreLastTransitionTime); d != "" {
		t.Fatalf("Expected Taskrun to have status %#v, but got error %v", expectTrStatus, cond)
	}
	if _, err := c.Kube.CoreV1().Pods(testNs).Get("test-pod", metav1.GetOptions{}); err != nil && !errors.IsNotFound(err) {
		t.Fatalf("Expected pod to be deleted but got error %v", err)
	}
}
