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
	"fmt"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/pkg/reconciler/pipeline/dag"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"knative.dev/pkg/apis"
)

// PipelineRunState is a slice of ResolvedPipelineRunTasks the represents the current execution
// state of the PipelineRun.
type PipelineRunState []*ResolvedPipelineRunTask

// PipelineRunFacts is a collection of list of ResolvedPipelineTask, graph of DAG tasks, and graph of finally tasks
type PipelineRunFacts struct {
	State           PipelineRunState
	TasksGraph      *dag.Graph
	FinalTasksGraph *dag.Graph
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
}

// ToMap returns a map that maps pipeline task name to the resolved pipeline run task
func (state PipelineRunState) ToMap() map[string]*ResolvedPipelineRunTask {
	m := make(map[string]*ResolvedPipelineRunTask)
	for _, rprt := range state {
		m[rprt.PipelineTask.Name] = rprt
	}
	return m
}

// IsBeforeFirstTaskRun returns true if the PipelineRun has not yet started its first TaskRun
func (state PipelineRunState) IsBeforeFirstTaskRun() bool {
	for _, t := range state {
		if t.TaskRun != nil {
			return false
		}
	}
	return true
}

// GetTaskRunsStatus returns a map of taskrun name and the taskrun
// ignore a nil taskrun in pipelineRunState, otherwise, capture taskrun object from PipelineRun Status
// update taskrun status based on the pipelineRunState before returning it in the map
func (state PipelineRunState) GetTaskRunsStatus(pr *v1beta1.PipelineRun) map[string]*v1beta1.PipelineRunTaskRunStatus {
	status := make(map[string]*v1beta1.PipelineRunTaskRunStatus)
	for _, rprt := range state {
		if rprt.TaskRun == nil && rprt.ResolvedConditionChecks == nil {
			continue
		}

		var prtrs *v1beta1.PipelineRunTaskRunStatus
		if rprt.TaskRun != nil {
			prtrs = pr.Status.TaskRuns[rprt.TaskRun.Name]
		}
		if prtrs == nil {
			prtrs = &v1beta1.PipelineRunTaskRunStatus{
				PipelineTaskName: rprt.PipelineTask.Name,
				WhenExpressions:  rprt.PipelineTask.WhenExpressions,
			}
		}

		if rprt.TaskRun != nil {
			prtrs.Status = &rprt.TaskRun.Status
		}

		if len(rprt.ResolvedConditionChecks) > 0 {
			cStatus := make(map[string]*v1beta1.PipelineRunConditionCheckStatus)
			for _, c := range rprt.ResolvedConditionChecks {
				cStatus[c.ConditionCheckName] = &v1beta1.PipelineRunConditionCheckStatus{
					ConditionName: c.ConditionRegisterName,
				}
				if c.ConditionCheck != nil {
					cStatus[c.ConditionCheckName].Status = c.NewConditionCheckStatus()
				}
			}
			prtrs.ConditionChecks = cStatus
			if rprt.ResolvedConditionChecks.IsDone() && !rprt.ResolvedConditionChecks.IsSuccess() {
				if prtrs.Status == nil {
					prtrs.Status = &v1beta1.TaskRunStatus{}
				}
				prtrs.Status.SetCondition(&apis.Condition{
					Type:    apis.ConditionSucceeded,
					Status:  corev1.ConditionFalse,
					Reason:  ReasonConditionCheckFailed,
					Message: fmt.Sprintf("ConditionChecks failed for Task %s in PipelineRun %s", rprt.TaskRunName, pr.Name),
				})
			}
		}
		status[rprt.TaskRunName] = prtrs
	}
	return status
}

// getNextTasks returns a list of tasks which should be executed next i.e.
// a list of tasks from candidateTasks which aren't yet indicated in state to be running and
// a list of cancelled/failed tasks from candidateTasks which haven't exhausted their retries
func (state PipelineRunState) getNextTasks(candidateTasks sets.String) []*ResolvedPipelineRunTask {
	tasks := []*ResolvedPipelineRunTask{}
	for _, t := range state {
		if _, ok := candidateTasks[t.PipelineTask.Name]; ok && t.TaskRun == nil {
			tasks = append(tasks, t)
		}
		if _, ok := candidateTasks[t.PipelineTask.Name]; ok && t.TaskRun != nil {
			status := t.TaskRun.Status.GetCondition(apis.ConditionSucceeded)
			if status != nil && status.IsFalse() {
				if !(t.TaskRun.IsCancelled() || status.Reason == v1beta1.TaskRunReasonCancelled.String() || status.Reason == ReasonConditionCheckFailed) {
					if len(t.TaskRun.Status.RetriesStatus) < t.PipelineTask.Retries {
						tasks = append(tasks, t)
					}
				}
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
			if t.IsCancelled() {
				return true
			}
			if t.IsFailure() {
				return true
			}
		}
	}
	return false
}

// DAGExecutionQueue returns a list of DAG tasks which needs to be scheduled next
func (facts *PipelineRunFacts) DAGExecutionQueue() (PipelineRunState, error) {
	tasks := PipelineRunState{}
	// when pipeline run is stopping, do not schedule any new task and only
	// wait for all running tasks to complete and report their status
	if !facts.IsStopping() {
		// candidateTasks is initialized to DAG root nodes to start pipeline execution
		// candidateTasks is derived based on successfully finished tasks and/or skipped tasks
		candidateTasks, err := dag.GetSchedulable(facts.TasksGraph, facts.successfulOrSkippedDAGTasks()...)
		if err != nil {
			return tasks, err
		}
		tasks = facts.State.getNextTasks(candidateTasks)
	}
	return tasks, nil
}

// GetFinalTasks returns a list of final tasks without any taskRun associated with it
// GetFinalTasks returns final tasks only when all DAG tasks have finished executing successfully or skipped or
// any one DAG task resulted in failure
func (facts *PipelineRunFacts) GetFinalTasks() PipelineRunState {
	tasks := PipelineRunState{}
	finalCandidates := sets.NewString()
	// check either pipeline has finished executing all DAG pipelineTasks
	// or any one of the DAG pipelineTask has failed
	if facts.checkDAGTasksDone() {
		// return list of tasks with all final tasks
		for _, t := range facts.State {
			if facts.isFinalTask(t.PipelineTask.Name) && !t.IsSuccessful() {
				finalCandidates.Insert(t.PipelineTask.Name)
			}
		}
		tasks = facts.State.getNextTasks(finalCandidates)
	}
	return tasks
}

// GetPipelineConditionStatus will return the Condition that the PipelineRun prName should be
// updated with, based on the status of the TaskRuns in state.
func (facts *PipelineRunFacts) GetPipelineConditionStatus(pr *v1beta1.PipelineRun, logger *zap.SugaredLogger) *apis.Condition {
	// We have 4 different states here:
	// 1. Timed out -> Failed
	// 2. All tasks are done and at least one has failed or has been cancelled -> Failed
	// 3. All tasks are done or are skipped (i.e. condition check failed).-> Success
	// 4. A Task or Condition is running right now or there are things left to run -> Running
	if pr.IsTimedOut() {
		return &apis.Condition{
			Type:    apis.ConditionSucceeded,
			Status:  corev1.ConditionFalse,
			Reason:  v1beta1.PipelineRunReasonTimedOut.String(),
			Message: fmt.Sprintf("PipelineRun %q failed to finish within %q", pr.Name, pr.Spec.Timeout.Duration.String()),
		}
	}

	// report the count in PipelineRun Status
	// get the count of successful tasks, failed tasks, cancelled tasks, skipped task, and incomplete tasks
	s := facts.getPipelineTasksCount()
	// completed task is a collection of successful, failed, cancelled tasks (skipped tasks are reported separately)
	cmTasks := s.Succeeded + s.Failed + s.Cancelled

	// The completion reason is set from the TaskRun completion reason
	// by default, set it to ReasonRunning
	reason := v1beta1.PipelineRunReasonRunning.String()

	// check if the pipeline is finished executing all tasks i.e. no incomplete tasks
	if s.Incomplete == 0 {
		status := corev1.ConditionTrue
		reason := v1beta1.PipelineRunReasonSuccessful.String()
		// Set reason to ReasonCompleted - At least one is skipped
		if s.Skipped > 0 {
			reason = v1beta1.PipelineRunReasonCompleted.String()
		}
		// Set reason to ReasonFailed - At least one failed
		if s.Failed > 0 {
			reason = v1beta1.PipelineRunReasonFailed.String()
			status = corev1.ConditionFalse
			// Set reason to ReasonCancelled - At least one is cancelled and no failure yet
		} else if s.Cancelled > 0 {
			reason = v1beta1.PipelineRunReasonCancelled.String()
			status = corev1.ConditionFalse
		}
		logger.Infof("All TaskRuns have finished for PipelineRun %s so it has finished", pr.Name)
		return &apis.Condition{
			Type:   apis.ConditionSucceeded,
			Status: status,
			Reason: reason,
			Message: fmt.Sprintf("Tasks Completed: %d (Failed: %d, Cancelled %d), Skipped: %d",
				cmTasks, s.Failed, s.Cancelled, s.Skipped),
		}
	}

	// Hasn't timed out; not all tasks have finished.... Must keep running then....
	// transition pipeline into stopping state when one of the tasks(dag/final) cancelled or one of the dag tasks failed
	// for a pipeline with final tasks, single dag task failure does not transition to interim stopping state
	// pipeline stays in running state until all final tasks are done before transitioning to failed state
	if s.Cancelled > 0 || (s.Failed > 0 && facts.checkFinalTasksDone()) {
		reason = v1beta1.PipelineRunReasonStopping.String()
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
func (facts *PipelineRunFacts) GetSkippedTasks() []v1beta1.SkippedTask {
	var skipped []v1beta1.SkippedTask
	for _, rprt := range facts.State {
		if rprt.Skip(facts) {
			skippedTask := v1beta1.SkippedTask{
				Name:            rprt.PipelineTask.Name,
				WhenExpressions: rprt.PipelineTask.WhenExpressions,
			}
			skipped = append(skipped, skippedTask)
		}
	}
	return skipped
}

// successfulOrSkippedTasks returns a list of the names of all of the PipelineTasks in state
// which have successfully completed or skipped
func (facts *PipelineRunFacts) successfulOrSkippedDAGTasks() []string {
	tasks := []string{}
	for _, t := range facts.State {
		if facts.isDAGTask(t.PipelineTask.Name) {
			if t.IsSuccessful() || t.Skip(facts) {
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
			if !t.IsDone(facts) {
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
		Skipped:    0,
		Succeeded:  0,
		Failed:     0,
		Cancelled:  0,
		Incomplete: 0,
	}
	for _, t := range facts.State {
		switch {
		// increment success counter since the task is successful
		case t.IsSuccessful():
			s.Succeeded++
		// increment failure counter since the task has failed
		case t.IsFailure():
			s.Failed++
		// increment cancelled counter since the task is cancelled
		case t.IsCancelled():
			s.Cancelled++
		// increment skip counter since the task is skipped
		case t.Skip(facts):
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
