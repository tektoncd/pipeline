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

package state

import (
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/pkg/reconciler/pipelinerun/resolution"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
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
type PipelineRunState []*resolution.ResolvedPipelineTask

// ToMap returns a map that maps pipeline task name to the resolved pipeline run task
func (state PipelineRunState) ToMap() map[string]*resolution.ResolvedPipelineTask {
	m := make(map[string]*resolution.ResolvedPipelineTask)
	for _, rpt := range state {
		m[rpt.PipelineTask.Name] = rpt
	}
	return m
}

// IsBeforeFirstTaskRun returns true if the PipelineRun has not yet started its first TaskRun
func (state PipelineRunState) IsBeforeFirstTaskRun() bool {
	for _, t := range state {
		if t.IsCustomTask() && t.Run != nil {
			return false
		} else if t.TaskRun != nil {
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
		if rpt.TaskRun == nil {
			if rpt.Run != nil {
				if rpt.Run.CreationTimestamp.Time.Before(adjustedStartTime.Time) {
					adjustedStartTime = &rpt.Run.CreationTimestamp
				}
			}
		} else {
			if rpt.TaskRun.CreationTimestamp.Time.Before(adjustedStartTime.Time) {
				adjustedStartTime = &rpt.TaskRun.CreationTimestamp
			}
		}
	}
	return adjustedStartTime.DeepCopy()
}

// GetTaskRunsStatus returns a map of taskrun name and the taskrun
// ignore a nil taskrun in pipelineRunState, otherwise, capture taskrun object from PipelineRun Status
// update taskrun status based on the pipelineRunState before returning it in the map
func (state PipelineRunState) GetTaskRunsStatus(pr *v1beta1.PipelineRun) map[string]*v1beta1.PipelineRunTaskRunStatus {
	status := make(map[string]*v1beta1.PipelineRunTaskRunStatus)
	for _, rpt := range state {
		if rpt.IsCustomTask() {
			continue
		}

		if rpt.TaskRun == nil {
			continue
		}

		status[rpt.TaskRunName] = rpt.GetTaskRunStatus(rpt.TaskRun, pr)
	}
	return status
}

// GetTaskRunsResults returns a map of all successfully completed TaskRuns in the state, with the pipeline task name as
// the key and the results from the corresponding TaskRun as the value. It only includes tasks which have completed successfully.
func (state PipelineRunState) GetTaskRunsResults() map[string][]v1beta1.TaskRunResult {
	results := make(map[string][]v1beta1.TaskRunResult)
	for _, rpt := range state {
		if rpt.IsCustomTask() {
			continue
		}
		if !rpt.IsSuccessful() {
			continue
		}
		if rpt.TaskRun != nil {
			results[rpt.PipelineTask.Name] = rpt.TaskRun.Status.TaskRunResults
		}
	}
	return results
}

// GetRunsStatus returns a map of run name and the run.
// Ignore a nil run in pipelineRunState, otherwise, capture run object from PipelineRun Status.
// Update run status based on the pipelineRunState before returning it in the map.
func (state PipelineRunState) GetRunsStatus(pr *v1beta1.PipelineRun) map[string]*v1beta1.PipelineRunRunStatus {
	status := map[string]*v1beta1.PipelineRunRunStatus{}
	for _, rpt := range state {
		if !rpt.IsCustomTask() {
			continue
		}

		var prrs *v1beta1.PipelineRunRunStatus
		if rpt.Run != nil {
			prrs = pr.Status.Runs[rpt.RunName]
		}

		if prrs == nil {
			prrs = &v1beta1.PipelineRunRunStatus{
				PipelineTaskName: rpt.PipelineTask.Name,
				WhenExpressions:  rpt.PipelineTask.WhenExpressions,
			}
		}

		if rpt.Run != nil {
			prrs.Status = &rpt.Run.Status
		}

		status[rpt.RunName] = prrs
	}
	return status
}

// GetRunsResults returns a map of all successfully completed Runs in the state, with the pipeline task name as the key
// and the results from the corresponding TaskRun as the value. It only includes runs which have completed successfully.
func (state PipelineRunState) GetRunsResults() map[string][]v1alpha1.RunResult {
	results := make(map[string][]v1alpha1.RunResult)
	for _, rpt := range state {
		if !rpt.IsCustomTask() {
			continue
		}
		if !rpt.IsSuccessful() {
			continue
		}
		if rpt.Run != nil {
			results[rpt.PipelineTask.Name] = rpt.Run.Status.Results
		}
	}

	return results
}

// GetChildReferences returns a slice of references, including version, kind, name, and pipeline task name, for all
// TaskRuns and Runs in the state.
func (state PipelineRunState) GetChildReferences() []v1beta1.ChildStatusReference {
	var childRefs []v1beta1.ChildStatusReference

	for _, rpt := range state {
		switch {
		case rpt.Run != nil:
			childRefs = append(childRefs, rpt.GetChildRefForRun(rpt.Run.Name))
		case rpt.TaskRun != nil:
			childRefs = append(childRefs, rpt.GetChildRefForTaskRun(rpt.TaskRun))
		case len(rpt.TaskRuns) != 0:
			for _, taskRun := range rpt.TaskRuns {
				if taskRun != nil {
					childRefs = append(childRefs, rpt.GetChildRefForTaskRun(taskRun))
				}
			}
		case len(rpt.Runs) != 0:
			for _, run := range rpt.Runs {
				if run != nil {
					childRefs = append(childRefs, rpt.GetChildRefForRun(run.Name))
				}
			}
		}
	}
	return childRefs
}

// GetNextTasks returns a list of tasks which should be executed next i.e.
// a list of tasks from candidateTasks which aren't yet indicated in state to be running and
// a list of cancelled/failed tasks from candidateTasks which haven't exhausted their retries
func (state PipelineRunState) GetNextTasks(candidateTasks sets.String) []*resolution.ResolvedPipelineTask {
	tasks := []*resolution.ResolvedPipelineTask{}
	for _, t := range state {
		if _, ok := candidateTasks[t.PipelineTask.Name]; ok {
			if t.TaskRun == nil && t.Run == nil && len(t.TaskRuns) == 0 && len(t.Runs) == 0 {
				tasks = append(tasks, t)
			}
		}
	}
	tasks = append(tasks, state.GetRetryableTasks(candidateTasks)...)
	return tasks
}

// GetRetryableTasks returns a list of pipelinetasks which should be executed next when the pipelinerun is stopping,
// i.e. a list of failed pipelinetasks from candidateTasks which haven't exhausted their retries. Note that if a
// pipelinetask is cancelled, the retries are not exhausted - they are not retryable.
func (state PipelineRunState) GetRetryableTasks(candidateTasks sets.String) []*resolution.ResolvedPipelineTask {
	var tasks []*resolution.ResolvedPipelineTask
	for _, t := range state {
		if _, ok := candidateTasks[t.PipelineTask.Name]; ok {
			var status *apis.Condition
			switch {
			case t.TaskRun != nil:
				status = t.TaskRun.Status.GetCondition(apis.ConditionSucceeded)
			case len(t.TaskRuns) != 0:
				IsDone := true
				for _, taskRun := range t.TaskRuns {
					IsDone = IsDone && taskRun.IsDone()
					c := taskRun.Status.GetCondition(apis.ConditionSucceeded)
					if c.IsFalse() {
						status = c
					}
				}
			case t.Run != nil:
				status = t.Run.Status.GetCondition(apis.ConditionSucceeded)
			case len(t.Runs) != 0:
				IsDone := true
				for _, run := range t.Runs {
					IsDone = IsDone && run.IsDone()
					c := run.Status.GetCondition(apis.ConditionSucceeded)
					if c.IsFalse() {
						status = c
					}
				}
			}
			if status.IsFalse() && !t.IsCancelled() && t.HasRemainingRetries() {
				tasks = append(tasks, t)
			}
		}
	}
	return tasks
}
