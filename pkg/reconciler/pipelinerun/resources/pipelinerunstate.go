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

package resources

import (
	"context"
	"fmt"
	"time"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/pkg/reconciler/pipeline/dag"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/clock"
	"knative.dev/pkg/apis"
)

const (
	// PipelineTaskStateNone indicates that the execution status of a pipelineTask is unknown
	PipelineTaskStateNone = "None"
	// PipelineTaskStatusPrefix is a prefix of the param representing execution state of pipelineTask
	PipelineTaskStatusPrefix = "tasks."
	// PipelineTaskStatusSuffix is a suffix of the param representing execution state of pipelineTask
	PipelineTaskStatusSuffix = ".status"
)

// PipelineRunState is a slice of ResolvedPipelineRunTasks the represents the current execution
// state of the PipelineRun.
type PipelineRunState []*ResolvedPipelineTask

// PipelineRunFacts holds the state of all the components that make up the Pipeline graph that are used to track the
// PipelineRun state without passing all these components separately. It helps simplify our implementation for getting
// and scheduling the next tasks. It is a collection of list of ResolvedPipelineTask, graph of DAG tasks, graph of
// finally tasks, cache of skipped tasks.
type PipelineRunFacts struct {
	State           PipelineRunState
	SpecStatus      v1.PipelineRunSpecStatus
	TasksGraph      *dag.Graph
	FinalTasksGraph *dag.Graph
	TimeoutsState   PipelineRunTimeoutsState

	// SkipCache is a hash of PipelineTask names that stores whether a task will be
	// executed or not, because it's either not reachable via the DAG due to the pipeline
	// state, or because it was skipped due to when expressions.
	// We cache this data along the state, because it's expensive to compute, it requires
	// traversing potentially the whole graph; this way it can built incrementally, when
	// needed, via the `Skip` method in pipelinerunresolution.go
	// The skip data is sensitive to changes in the state. The ResetSkippedCache method
	// can be used to clean the cache and force re-computation when needed.
	SkipCache map[string]TaskSkipStatus
}

// PipelineRunTimeoutsState records information about start times and timeouts for the PipelineRun, so that the PipelineRunFacts
// can reference those values in its functions.
type PipelineRunTimeoutsState struct {
	StartTime        *time.Time
	FinallyStartTime *time.Time
	PipelineTimeout  *time.Duration
	TasksTimeout     *time.Duration
	FinallyTimeout   *time.Duration
	Clock            clock.PassiveClock
}

// pipelineRunStatusCount holds the count of successful, failed, cancelled, skipped, and incomplete tasks
type pipelineRunStatusCount struct {
	// skipped tasks count
	Skipped int
	// successful tasks count
	Succeeded int
	// failed tasks count
	Failed int
	// cancelled tasks count
	Cancelled int
	// number of tasks which are still pending, have not executed
	Incomplete int
	// count of tasks skipped due to the relevant timeout having elapsed before the task is launched
	SkippedDueToTimeout int
}

// ResetSkippedCache resets the skipped cache in the facts map
func (facts *PipelineRunFacts) ResetSkippedCache() {
	facts.SkipCache = make(map[string]TaskSkipStatus)
}

// ToMap returns a map that maps pipeline task name to the resolved pipeline run task
func (state PipelineRunState) ToMap() map[string]*ResolvedPipelineTask {
	m := make(map[string]*ResolvedPipelineTask)
	for _, rpt := range state {
		m[rpt.PipelineTask.Name] = rpt
	}
	return m
}

// IsBeforeFirstTaskRun returns true if the PipelineRun has not yet started its first TaskRun
func (state PipelineRunState) IsBeforeFirstTaskRun() bool {
	for _, t := range state {
		if len(t.CustomRuns) > 0 || len(t.TaskRuns) > 0 {
			return false
		}
	}
	return true
}

// AdjustStartTime adjusts potential drift in the PipelineRun's start time.
//
// The StartTime will only adjust earlier, so that the PipelineRun's StartTime
// is no later than any of its constituent TaskRuns.
//
// This drift could be due to us either failing to record the Run's start time
// previously, or our own failure to observe a prior update before reconciling
// the resource again.
func (state PipelineRunState) AdjustStartTime(unadjustedStartTime *metav1.Time) *metav1.Time {
	adjustedStartTime := unadjustedStartTime
	for _, rpt := range state {
		for _, customRun := range rpt.CustomRuns {
			creationTime := customRun.GetObjectMeta().GetCreationTimestamp()
			if creationTime.Time.Before(adjustedStartTime.Time) {
				adjustedStartTime = &creationTime
			}
		}
		for _, taskRun := range rpt.TaskRuns {
			if taskRun.CreationTimestamp.Time.Before(adjustedStartTime.Time) {
				adjustedStartTime = &taskRun.CreationTimestamp
			}
		}
	}
	return adjustedStartTime.DeepCopy()
}

// GetTaskRunsResults returns a map of all successfully completed TaskRuns in the state, with the pipeline task name as
// the key and the results from the corresponding TaskRun as the value. It only includes tasks which have completed successfully.
func (state PipelineRunState) GetTaskRunsResults() map[string][]v1.TaskRunResult {
	results := make(map[string][]v1.TaskRunResult)
	for _, rpt := range state {
		if rpt.IsCustomTask() {
			continue
		}
		if !rpt.isSuccessful() {
			continue
		}
		// Currently a Matrix cannot produce results so this is for a singular TaskRun
		if len(rpt.TaskRuns) == 1 {
			results[rpt.PipelineTask.Name] = rpt.TaskRuns[0].Status.Results
		}
	}
	return results
}

// GetRunsResults returns a map of all successfully completed Runs in the state, with the pipeline task name as the key
// and the results from the corresponding TaskRun as the value. It only includes runs which have completed successfully.
func (state PipelineRunState) GetRunsResults() map[string][]v1beta1.CustomRunResult {
	results := make(map[string][]v1beta1.CustomRunResult)
	for _, rpt := range state {
		if !rpt.IsCustomTask() {
			continue
		}
		if !rpt.isSuccessful() {
			continue
		}
		// Currently a Matrix cannot produce results so this is for a singular CustomRun
		if len(rpt.CustomRuns) == 1 {
			cr := rpt.CustomRuns[0]
			results[rpt.PipelineTask.Name] = cr.Status.Results
		}
	}

	return results
}

// GetChildReferences returns a slice of references, including version, kind, name, and pipeline task name, for all
// TaskRuns and Runs in the state.
func (state PipelineRunState) GetChildReferences() []v1.ChildStatusReference {
	var childRefs []v1.ChildStatusReference

	for _, rpt := range state {
		switch {
		case len(rpt.TaskRuns) != 0:
			for _, taskRun := range rpt.TaskRuns {
				if taskRun != nil {
					childRefs = append(childRefs, rpt.getChildRefForTaskRun(taskRun))
				}
			}
		case len(rpt.CustomRuns) != 0:
			for _, run := range rpt.CustomRuns {
				childRefs = append(childRefs, rpt.getChildRefForRun(run))
			}
		}
	}
	return childRefs
}

func (t *ResolvedPipelineTask) getChildRefForRun(customRun *v1beta1.CustomRun) v1.ChildStatusReference {
	return v1.ChildStatusReference{
		TypeMeta: runtime.TypeMeta{
			APIVersion: v1beta1.SchemeGroupVersion.String(),
			Kind:       pipeline.CustomRunControllerName,
		},
		Name:             customRun.GetObjectMeta().GetName(),
		PipelineTaskName: t.PipelineTask.Name,
		WhenExpressions:  t.PipelineTask.When,
	}
}

func (t *ResolvedPipelineTask) getChildRefForTaskRun(taskRun *v1.TaskRun) v1.ChildStatusReference {
	return v1.ChildStatusReference{
		TypeMeta: runtime.TypeMeta{
			APIVersion: v1.SchemeGroupVersion.String(),
			Kind:       pipeline.TaskRunControllerName,
		},
		Name:             taskRun.Name,
		PipelineTaskName: t.PipelineTask.Name,
		WhenExpressions:  t.PipelineTask.When,
	}
}

// getNextTasks returns a list of tasks which should be executed next i.e.
// a list of tasks from candidateTasks which aren't yet indicated in state to be running and
// a list of cancelled/failed tasks from candidateTasks which haven't exhausted their retries
func (state PipelineRunState) getNextTasks(candidateTasks sets.String) []*ResolvedPipelineTask {
	tasks := []*ResolvedPipelineTask{}
	for _, t := range state {
		if _, ok := candidateTasks[t.PipelineTask.Name]; ok {
			if len(t.TaskRuns) == 0 && len(t.CustomRuns) == 0 {
				tasks = append(tasks, t)
			}
		}
	}
	return tasks
}

// IsStopping returns true if the PipelineRun won't be scheduling any new Task because
// at least one task already failed or was cancelled in the specified dag
func (facts *PipelineRunFacts) IsStopping() bool {
	for _, t := range facts.State {
		if facts.isDAGTask(t.PipelineTask.Name) {
			if t.isFailure() {
				return true
			}
		}
	}
	return false
}

// IsRunning returns true if the PipelineRun is still running tasks in the specified dag
func (facts *PipelineRunFacts) IsRunning() bool {
	for _, t := range facts.State {
		if facts.isDAGTask(t.PipelineTask.Name) {
			if t.IsRunning() {
				return true
			}
		}
	}
	return false
}

// IsCancelled returns true if the PipelineRun was cancelled
func (facts *PipelineRunFacts) IsCancelled() bool {
	return facts.SpecStatus == v1.PipelineRunSpecStatusCancelled
}

// IsGracefullyCancelled returns true if the PipelineRun was gracefully cancelled
func (facts *PipelineRunFacts) IsGracefullyCancelled() bool {
	return facts.SpecStatus == v1.PipelineRunSpecStatusCancelledRunFinally
}

// IsGracefullyStopped returns true if the PipelineRun was gracefully stopped
func (facts *PipelineRunFacts) IsGracefullyStopped() bool {
	return facts.SpecStatus == v1.PipelineRunSpecStatusStoppedRunFinally
}

// DAGExecutionQueue returns a list of DAG tasks which needs to be scheduled next
func (facts *PipelineRunFacts) DAGExecutionQueue() (PipelineRunState, error) {
	var tasks PipelineRunState
	// when pipelinerun is cancelled or gracefully cancelled, do not schedule any new tasks,
	// and only wait for all running tasks to complete (without exhausting retries).
	if facts.IsCancelled() || facts.IsGracefullyCancelled() {
		return tasks, nil
	}
	// candidateTasks is initialized to DAG root nodes to start pipeline execution
	// candidateTasks is derived based on successfully finished tasks and/or skipped tasks
	candidateTasks, err := dag.GetCandidateTasks(facts.TasksGraph, facts.completedOrSkippedDAGTasks()...)
	if err != nil {
		return tasks, err
	}
	if !facts.IsStopping() && !facts.IsGracefullyStopped() {
		tasks = facts.State.getNextTasks(candidateTasks)
	}
	return tasks, nil
}

// GetFinalTaskNames returns a list of all final task names
func (facts *PipelineRunFacts) GetFinalTaskNames() sets.String {
	names := sets.NewString()
	// return list of tasks with all final tasks
	for _, t := range facts.State {
		if facts.isFinalTask(t.PipelineTask.Name) {
			names.Insert(t.PipelineTask.Name)
		}
	}
	return names
}

// GetTaskNames returns a list of all non-final task names
func (facts *PipelineRunFacts) GetTaskNames() sets.String {
	names := sets.NewString()
	// return list of tasks with all final tasks
	for _, t := range facts.State {
		if !facts.isFinalTask(t.PipelineTask.Name) {
			names.Insert(t.PipelineTask.Name)
		}
	}
	return names
}

// GetFinalTasks returns a list of final tasks which needs to be executed next
// GetFinalTasks returns final tasks only when all DAG tasks have finished executing or have been skipped
func (facts *PipelineRunFacts) GetFinalTasks() PipelineRunState {
	tasks := PipelineRunState{}
	finalCandidates := sets.NewString()
	// check either pipeline has finished executing all DAG pipelineTasks,
	// where "finished executing" means succeeded, failed, or skipped.
	if facts.checkDAGTasksDone() {
		// return list of tasks with all final tasks
		for _, t := range facts.State {
			if facts.isFinalTask(t.PipelineTask.Name) {
				finalCandidates.Insert(t.PipelineTask.Name)
			}
		}
		tasks = facts.State.getNextTasks(finalCandidates)
	}
	return tasks
}

// GetPipelineConditionStatus will return the Condition that the PipelineRun prName should be
// updated with, based on the status of the TaskRuns in state.
func (facts *PipelineRunFacts) GetPipelineConditionStatus(ctx context.Context, pr *v1.PipelineRun, logger *zap.SugaredLogger, c clock.PassiveClock) *apis.Condition {
	// We have 4 different states here:
	// 1. Timed out -> Failed
	// 2. All tasks are done and at least one has failed or has been cancelled -> Failed
	// 3. All tasks are done or are skipped (i.e. condition check failed).-> Success
	// 4. A Task or Condition is running right now or there are things left to run -> Running
	if pr.HasTimedOut(ctx, c) {
		return &apis.Condition{
			Type:    apis.ConditionSucceeded,
			Status:  corev1.ConditionFalse,
			Reason:  v1.PipelineRunReasonTimedOut.String(),
			Message: fmt.Sprintf("PipelineRun %q failed to finish within %q", pr.Name, pr.PipelineTimeout(ctx).String()),
		}
	}

	// report the count in PipelineRun Status
	// get the count of successful tasks, failed tasks, cancelled tasks, skipped task, and incomplete tasks
	s := facts.getPipelineTasksCount()
	// completed task is a collection of successful, failed, cancelled tasks (skipped tasks are reported separately)
	cmTasks := s.Succeeded + s.Failed + s.Cancelled

	// The completion reason is set from the TaskRun completion reason
	// by default, set it to ReasonRunning
	reason := v1.PipelineRunReasonRunning.String()

	// check if the pipeline is finished executing all tasks i.e. no incomplete tasks
	if s.Incomplete == 0 {
		status := corev1.ConditionTrue
		reason := v1.PipelineRunReasonSuccessful.String()
		message := fmt.Sprintf("Tasks Completed: %d (Failed: %d, Cancelled %d), Skipped: %d",
			cmTasks, s.Failed, s.Cancelled, s.Skipped)
		// Set reason to ReasonCompleted - At least one is skipped
		if s.Skipped > 0 {
			reason = v1.PipelineRunReasonCompleted.String()
		}

		switch {
		case s.Failed > 0 || s.SkippedDueToTimeout > 0:
			// Set reason to ReasonFailed - At least one failed
			reason = v1.PipelineRunReasonFailed.String()
			status = corev1.ConditionFalse
		case pr.IsGracefullyCancelled() || pr.IsGracefullyStopped():
			// Set reason to ReasonCancelled - Cancellation requested
			reason = v1.PipelineRunReasonCancelled.String()
			status = corev1.ConditionFalse
			message = fmt.Sprintf("PipelineRun %q was cancelled", pr.Name)
		case s.Cancelled > 0:
			// Set reason to ReasonCancelled - At least one is cancelled and no failure yet
			reason = v1.PipelineRunReasonCancelled.String()
			status = corev1.ConditionFalse
		}
		logger.Infof("All TaskRuns have finished for PipelineRun %s so it has finished", pr.Name)
		return &apis.Condition{
			Type:    apis.ConditionSucceeded,
			Status:  status,
			Reason:  reason,
			Message: message,
		}
	}

	// Hasn't timed out; not all tasks have finished.... Must keep running then....
	switch {
	case pr.IsGracefullyCancelled():
		// Transition pipeline into running finally state, when graceful cancel is in progress
		reason = v1.PipelineRunReasonCancelledRunningFinally.String()
	case pr.IsGracefullyStopped():
		// Transition pipeline into running finally state, when graceful stop is in progress
		reason = v1.PipelineRunReasonStoppedRunningFinally.String()
	case s.Cancelled > 0 || (s.Failed > 0 && facts.checkFinalTasksDone()):
		// Transition pipeline into stopping state when one of the tasks(dag/final) cancelled or one of the dag tasks failed
		// for a pipeline with final tasks, single dag task failure does not transition to interim stopping state
		// pipeline stays in running state until all final tasks are done before transitioning to failed state
		reason = v1.PipelineRunReasonStopping.String()
	}

	// return the status
	return &apis.Condition{
		Type:   apis.ConditionSucceeded,
		Status: corev1.ConditionUnknown,
		Reason: reason,
		Message: fmt.Sprintf("Tasks Completed: %d (Failed: %d, Cancelled %d), Incomplete: %d, Skipped: %d",
			cmTasks, s.Failed, s.Cancelled, s.Incomplete, s.Skipped),
	}
}

// GetSkippedTasks constructs a list of SkippedTask struct to be included in the PipelineRun Status
func (facts *PipelineRunFacts) GetSkippedTasks() []v1.SkippedTask {
	var skipped []v1.SkippedTask
	for _, rpt := range facts.State {
		if rpt.Skip(facts).IsSkipped {
			skippedTask := v1.SkippedTask{
				Name:            rpt.PipelineTask.Name,
				Reason:          rpt.Skip(facts).SkippingReason,
				WhenExpressions: rpt.PipelineTask.When,
			}
			skipped = append(skipped, skippedTask)
		}
		if rpt.IsFinallySkipped(facts).IsSkipped {
			skippedTask := v1.SkippedTask{
				Name:   rpt.PipelineTask.Name,
				Reason: rpt.IsFinallySkipped(facts).SkippingReason,
			}
			// include the when expressions only when the finally task was skipped because
			// its when expressions evaluated to false (not because results variables were missing)
			if rpt.IsFinallySkipped(facts).SkippingReason == v1.WhenExpressionsSkip {
				skippedTask.WhenExpressions = rpt.PipelineTask.When
			}
			skipped = append(skipped, skippedTask)
		}
	}
	return skipped
}

// GetPipelineTaskStatus returns the status of a PipelineTask depending on its taskRun
// the checks are implemented such that the finally tasks are requesting status of the dag tasks
func (facts *PipelineRunFacts) GetPipelineTaskStatus() map[string]string {
	// construct a map of tasks.<pipelineTask>.status and its state
	tStatus := make(map[string]string)
	for _, t := range facts.State {
		if facts.isDAGTask(t.PipelineTask.Name) {
			var s string
			switch {
			// execution status is Succeeded when a task has succeeded condition with status set to true
			case t.isSuccessful():
				s = v1.TaskRunReasonSuccessful.String()
			// execution status is Failed when a task has succeeded condition with status set to false
			case t.haveAnyRunsFailed():
				s = v1.TaskRunReasonFailed.String()
			default:
				// None includes skipped as well
				s = PipelineTaskStateNone
			}
			tStatus[PipelineTaskStatusPrefix+t.PipelineTask.Name+PipelineTaskStatusSuffix] = s
		}
	}
	// initialize aggregate status of all dag tasks to None
	aggregateStatus := PipelineTaskStateNone
	if facts.checkDAGTasksDone() {
		// all dag tasks are done, change the aggregate status to succeeded
		// will reset it to failed/skipped if needed
		aggregateStatus = v1.PipelineRunReasonSuccessful.String()
		for _, t := range facts.State {
			if facts.isDAGTask(t.PipelineTask.Name) {
				// if any of the dag task failed, change the aggregate status to failed and return
				if !t.IsCustomTask() && t.haveAnyTaskRunsFailed() || t.IsCustomTask() && t.haveAnyCustomRunsFailed() {
					aggregateStatus = v1beta1.PipelineRunReasonFailed.String()
					break
				}
				// if any of the dag task skipped, change the aggregate status to completed
				// but continue checking for any other failure
				if t.Skip(facts).IsSkipped {
					aggregateStatus = v1.PipelineRunReasonCompleted.String()
				}
			}
		}
	}
	tStatus[v1.PipelineTasksAggregateStatus] = aggregateStatus
	return tStatus
}

// completedOrSkippedTasks returns a list of the names of all of the PipelineTasks in state
// which have completed or skipped
func (facts *PipelineRunFacts) completedOrSkippedDAGTasks() []string {
	tasks := []string{}
	for _, t := range facts.State {
		if facts.isDAGTask(t.PipelineTask.Name) {
			if t.isDone(facts) {
				tasks = append(tasks, t.PipelineTask.Name)
			}
		}
	}
	return tasks
}

// checkTasksDone returns true if all tasks from the specified graph are finished executing
// a task is considered done if it has failed/succeeded/skipped
func (facts *PipelineRunFacts) checkTasksDone(d *dag.Graph) bool {
	for _, t := range facts.State {
		if isTaskInGraph(t.PipelineTask.Name, d) {
			if !t.isDone(facts) {
				return false
			}
		}
	}
	return true
}

// check if all DAG tasks done executing (succeeded, failed, or skipped)
func (facts *PipelineRunFacts) checkDAGTasksDone() bool {
	return facts.checkTasksDone(facts.TasksGraph)
}

// check if all finally tasks done executing (succeeded or failed)
func (facts *PipelineRunFacts) checkFinalTasksDone() bool {
	return facts.checkTasksDone(facts.FinalTasksGraph)
}

// getPipelineTasksCount returns the count of successful tasks, failed tasks, cancelled tasks, skipped task, and incomplete tasks
func (facts *PipelineRunFacts) getPipelineTasksCount() pipelineRunStatusCount {
	s := pipelineRunStatusCount{
		Skipped:             0,
		Succeeded:           0,
		Failed:              0,
		Cancelled:           0,
		Incomplete:          0,
		SkippedDueToTimeout: 0,
	}
	for _, t := range facts.State {
		switch {
		// increment success counter since the task is successful
		case t.isSuccessful():
			s.Succeeded++
		// increment failure counter since the task is cancelled due to a timeout
		case t.isCancelledForTimeOut():
			s.Failed++
		// increment cancelled counter since the task is cancelled
		case t.isCancelled():
			s.Cancelled++
		// increment failure counter since the task has failed
		case t.isFailure():
			s.Failed++
		// increment skipped and skipped due to timeout counters since the task was skipped due to the pipeline, tasks, or finally timeout being reached before the task was launched
		case t.Skip(facts).SkippingReason == v1.PipelineTimedOutSkip ||
			t.Skip(facts).SkippingReason == v1.TasksTimedOutSkip ||
			t.IsFinallySkipped(facts).SkippingReason == v1.FinallyTimedOutSkip:
			s.Skipped++
			s.SkippedDueToTimeout++
		// increment skip counter since the task is skipped
		case t.Skip(facts).IsSkipped:
			s.Skipped++
		// checking if any finally tasks were referring to invalid/missing task results
		case t.IsFinallySkipped(facts).IsSkipped:
			s.Skipped++
		// increment incomplete counter since the task is pending and not executed yet
		default:
			s.Incomplete++
		}
	}
	return s
}

// check if a specified pipelineTask is defined under tasks(DAG) section
func (facts *PipelineRunFacts) isDAGTask(pipelineTaskName string) bool {
	if _, ok := facts.TasksGraph.Nodes[pipelineTaskName]; ok {
		return true
	}
	return false
}

// check if a specified pipelineTask is defined under finally section
func (facts *PipelineRunFacts) isFinalTask(pipelineTaskName string) bool {
	if _, ok := facts.FinalTasksGraph.Nodes[pipelineTaskName]; ok {
		return true
	}
	return false
}

// Check if a PipelineTask belongs to the specified Graph
func isTaskInGraph(pipelineTaskName string, d *dag.Graph) bool {
	if _, ok := d.Nodes[pipelineTaskName]; ok {
		return true
	}
	return false
}
