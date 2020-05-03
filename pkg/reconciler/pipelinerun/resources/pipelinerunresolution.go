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

package resources

import (
	"context"
	"fmt"
	"reflect"
	"strconv"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"knative.dev/pkg/apis"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/pkg/contexts"
	"github.com/tektoncd/pipeline/pkg/list"
	"github.com/tektoncd/pipeline/pkg/names"
	"github.com/tektoncd/pipeline/pkg/reconciler/pipeline/dag"
	"github.com/tektoncd/pipeline/pkg/reconciler/taskrun/resources"
)

const (
	// ReasonRunning indicates that the reason for the inprogress status is that the TaskRun
	// is just starting to be reconciled
	ReasonRunning = "Running"

	// ReasonFailed indicates that the reason for the failure status is that one of the TaskRuns failed
	ReasonFailed = "Failed"

	// ReasonCancelled indicates that the reason for the cancelled status is that one of the TaskRuns cancelled
	ReasonCancelled = "Cancelled"

	// ReasonSucceeded indicates that the reason for the finished status is that all of the TaskRuns
	// completed successfully
	ReasonSucceeded = "Succeeded"

	// ReasonCompleted indicates that the reason for the finished status is that all of the TaskRuns
	// completed successfully but with some conditions checking failed
	ReasonCompleted = "Completed"

	// ReasonTimedOut indicates that the PipelineRun has taken longer than its configured
	// timeout
	ReasonTimedOut = "PipelineRunTimeout"

	// ReasonConditionCheckFailed indicates that the reason for the failure status is that the
	// condition check associated to the pipeline task evaluated to false
	ReasonConditionCheckFailed = "ConditionCheckFailed"
)

// ResolvedPipelineRunTask contains a Task and its associated TaskRun, if it
// exists. TaskRun can be nil to represent there being no TaskRun.
type ResolvedPipelineRunTask struct {
	TaskRunName           string
	TaskRun               *v1alpha1.TaskRun
	PipelineTask          *v1alpha1.PipelineTask
	ResolvedTaskResources *resources.ResolvedTaskResources
	// ConditionChecks ~~TaskRuns but for evaling conditions
	ResolvedConditionChecks TaskConditionCheckState // Could also be a TaskRun or maybe just a Pod?
}

// PipelineRunState is a slice of ResolvedPipelineRunTasks the represents the current execution
// state of the PipelineRun.
type PipelineRunState []*ResolvedPipelineRunTask

func (t ResolvedPipelineRunTask) IsDone() (isDone bool) {
	if t.TaskRun == nil || t.PipelineTask == nil {
		return
	}

	status := t.TaskRun.Status.GetCondition(apis.ConditionSucceeded)
	retriesDone := len(t.TaskRun.Status.RetriesStatus)
	retries := t.PipelineTask.Retries
	isDone = status.IsTrue() || status.IsFalse() && retriesDone >= retries
	return
}

// IsSuccessful returns true only if the taskrun itself has completed successfully
func (t ResolvedPipelineRunTask) IsSuccessful() bool {
	if t.TaskRun == nil {
		return false
	}
	c := t.TaskRun.Status.GetCondition(apis.ConditionSucceeded)
	if c == nil {
		return false
	}

	return c.Status == corev1.ConditionTrue
}

// IsFailure returns true only if the taskrun itself has failed
func (t ResolvedPipelineRunTask) IsFailure() bool {
	if t.TaskRun == nil {
		return false
	}
	c := t.TaskRun.Status.GetCondition(apis.ConditionSucceeded)
	retriesDone := len(t.TaskRun.Status.RetriesStatus)
	retries := t.PipelineTask.Retries
	return c.IsFalse() && retriesDone >= retries
}

// IsCancelled returns true only if the taskrun itself has cancelled
func (t ResolvedPipelineRunTask) IsCancelled() bool {
	if t.TaskRun == nil {
		return false
	}

	c := t.TaskRun.Status.GetCondition(apis.ConditionSucceeded)
	if c == nil {
		return false
	}

	return c.IsFalse() && c.Reason == v1alpha1.TaskRunSpecStatusCancelled
}

// ToMap returns a map that maps pipeline task name to the resolved pipeline run task
func (state PipelineRunState) ToMap() map[string]*ResolvedPipelineRunTask {
	m := make(map[string]*ResolvedPipelineRunTask)
	for _, rprt := range state {
		m[rprt.PipelineTask.Name] = rprt
	}
	return m
}

func (state PipelineRunState) IsDone() (isDone bool) {
	isDone = true
	for _, t := range state {
		if t.TaskRun == nil || t.PipelineTask == nil {
			return false
		}
		isDone = isDone && t.IsDone()
		if !isDone {
			return
		}
	}
	return
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

// GetNextTasks will return the next ResolvedPipelineRunTasks to execute, which are the ones in the
// list of candidateTasks which aren't yet indicated in state to be running.
func (state PipelineRunState) GetNextTasks(candidateTasks map[string]struct{}) []*ResolvedPipelineRunTask {
	tasks := []*ResolvedPipelineRunTask{}
	for _, t := range state {
		if _, ok := candidateTasks[t.PipelineTask.Name]; ok && t.TaskRun == nil {
			tasks = append(tasks, t)
		}
		if _, ok := candidateTasks[t.PipelineTask.Name]; ok && t.TaskRun != nil {
			status := t.TaskRun.Status.GetCondition(apis.ConditionSucceeded)
			if status != nil && status.IsFalse() {
				if !(t.TaskRun.IsCancelled() || status.Reason == v1alpha1.TaskRunSpecStatusCancelled || status.Reason == ReasonConditionCheckFailed) {
					if len(t.TaskRun.Status.RetriesStatus) < t.PipelineTask.Retries {
						tasks = append(tasks, t)
					}
				}
			}
		}
	}
	return tasks
}

// SuccessfulPipelineTaskNames returns a list of the names of all of the PipelineTasks in state
// which have successfully completed.
func (state PipelineRunState) SuccessfulPipelineTaskNames() []string {
	done := []string{}
	for _, t := range state {
		if t.TaskRun != nil {
			c := t.TaskRun.Status.GetCondition(apis.ConditionSucceeded)
			if c.IsTrue() {
				done = append(done, t.PipelineTask.Name)
			}
		}
	}
	return done
}

// GetTaskRun is a function that will retrieve the TaskRun name.
type GetTaskRun func(name string) (*v1alpha1.TaskRun, error)

// GetResourcesFromBindings will retrieve all Resources bound in PipelineRun pr and return a map
// from the declared name of the PipelineResource (which is how the PipelineResource will
// be referred to in the PipelineRun) to the PipelineResource, obtained via getResource.
func GetResourcesFromBindings(pr *v1alpha1.PipelineRun, getResource resources.GetResource) (map[string]*v1alpha1.PipelineResource, error) {
	rs := map[string]*v1alpha1.PipelineResource{}
	for _, resource := range pr.Spec.Resources {
		r, err := resources.GetResourceFromBinding(&resource, getResource)
		if err != nil {
			return rs, fmt.Errorf("error following resource reference for %s: %w", resource.Name, err)
		}
		rs[resource.Name] = r
	}
	return rs, nil
}

// ValidateResourceBindings validate that the PipelineResources declared in Pipeline p are bound in PipelineRun.
func ValidateResourceBindings(p *v1alpha1.PipelineSpec, pr *v1alpha1.PipelineRun) error {
	required := make([]string, 0, len(p.Resources))
	optional := make([]string, 0, len(p.Resources))
	for _, resource := range p.Resources {
		if resource.Optional {
			// create a list of optional resources
			optional = append(optional, resource.Name)
		} else {
			// create a list of required resources
			required = append(required, resource.Name)
		}
	}
	provided := make([]string, 0, len(pr.Spec.Resources))
	for _, resource := range pr.Spec.Resources {
		provided = append(provided, resource.Name)
	}
	// verify that the list of required resources exists in the provided resources
	missing := list.DiffLeft(required, provided)
	if len(missing) > 0 {
		return fmt.Errorf("Pipeline's declared required resources are missing from the PipelineRun: %s", missing)
	}
	// verify that the list of provided resources does not have any extra resources (outside of required and optional resources combined)
	extra := list.DiffLeft(provided, append(required, optional...))
	if len(extra) > 0 {
		return fmt.Errorf("PipelineRun's declared resources didn't match usage in Pipeline: %s", extra)
	}
	return nil
}

// ValidateWorkspaceBindings validates that the Workspaces expected by a Pipeline are provided by a PipelineRun.
func ValidateWorkspaceBindings(p *v1alpha1.PipelineSpec, pr *v1alpha1.PipelineRun) error {
	pipelineRunWorkspaces := make(map[string]v1alpha1.WorkspaceBinding)
	for _, binding := range pr.Spec.Workspaces {
		pipelineRunWorkspaces[binding.Name] = binding
	}

	for _, ws := range p.Workspaces {
		if _, ok := pipelineRunWorkspaces[ws.Name]; !ok {
			return fmt.Errorf("pipeline expects workspace with name %q be provided by pipelinerun", ws.Name)
		}
	}
	return nil
}

// TaskNotFoundError indicates that the resolution failed because a referenced Task couldn't be retrieved
type TaskNotFoundError struct {
	Name string
	Msg  string
}

func (e *TaskNotFoundError) Error() string {
	return fmt.Sprintf("Couldn't retrieve Task %q: %s", e.Name, e.Msg)
}

type ConditionNotFoundError struct {
	Name string
	Msg  string
}

func (e *ConditionNotFoundError) Error() string {
	return fmt.Sprintf("Couldn't retrieve Condition %q: %s", e.Name, e.Msg)
}

// ResolvePipelineRun retrieves all Tasks instances which are reference by tasks, getting
// instances from getTask. If it is unable to retrieve an instance of a referenced Task, it
// will return an error, otherwise it returns a list of all of the Tasks retrieved.
// It will retrieve the Resources needed for the TaskRun using the mapping of providedResources.
func ResolvePipelineRun(
	ctx context.Context,
	pipelineRun v1alpha1.PipelineRun,
	getTask resources.GetTask,
	getTaskRun resources.GetTaskRun,
	getClusterTask resources.GetClusterTask,
	getCondition GetCondition,
	tasks []v1alpha1.PipelineTask,
	providedResources map[string]*v1alpha1.PipelineResource,
) (PipelineRunState, error) {

	state := []*ResolvedPipelineRunTask{}
	for i := range tasks {
		pt := tasks[i]

		rprt := ResolvedPipelineRunTask{
			PipelineTask: &pt,
			TaskRunName:  getTaskRunName(pipelineRun.Status.TaskRuns, pt.Name, pipelineRun.Name),
		}

		// Find the Task that this PipelineTask is using
		var (
			t        v1alpha1.TaskInterface
			err      error
			spec     v1alpha1.TaskSpec
			taskName string
			kind     v1alpha1.TaskKind
		)

		if pt.TaskRef != nil {
			if pt.TaskRef.Kind == v1alpha1.ClusterTaskKind {
				t, err = getClusterTask(pt.TaskRef.Name)
			} else {
				t, err = getTask(pt.TaskRef.Name)
			}
			if err != nil {
				return nil, &TaskNotFoundError{
					Name: pt.TaskRef.Name,
					Msg:  err.Error(),
				}
			}
			spec = t.TaskSpec()
			taskName = t.TaskMetadata().Name
			kind = pt.TaskRef.Kind
		} else {
			spec = *pt.TaskSpec
		}
		spec.SetDefaults(contexts.WithUpgradeViaDefaulting(ctx))
		if err := spec.ConvertTo(ctx, &v1beta1.TaskSpec{}); err != nil {
			return nil, err
		}
		rtr, err := ResolvePipelineTaskResources(pt, &spec, taskName, kind, providedResources)
		if err != nil {
			return nil, fmt.Errorf("couldn't match referenced resources with declared resources: %w", err)
		}

		rprt.ResolvedTaskResources = rtr

		taskRun, err := getTaskRun(rprt.TaskRunName)
		if err != nil {
			if !errors.IsNotFound(err) {
				return nil, fmt.Errorf("error retrieving TaskRun %s: %w", rprt.TaskRunName, err)
			}
		}
		if taskRun != nil {
			rprt.TaskRun = taskRun
		}

		// Get all conditions that this pipelineTask will be using, if any
		if len(pt.Conditions) > 0 {
			rcc, err := resolveConditionChecks(&pt, pipelineRun.Status.TaskRuns, rprt.TaskRunName, getTaskRun, getCondition, providedResources)
			if err != nil {
				return nil, err
			}
			rprt.ResolvedConditionChecks = rcc
		}

		// Add this task to the state of the PipelineRun
		state = append(state, &rprt)
	}
	return state, nil
}

// getConditionCheckName should return a unique name for a `ConditionCheck` if one has not already been defined, and the existing one otherwise.
func getConditionCheckName(taskRunStatus map[string]*v1alpha1.PipelineRunTaskRunStatus, trName, conditionRegisterName string) string {
	trStatus, ok := taskRunStatus[trName]
	if ok && trStatus.ConditionChecks != nil {
		for k, v := range trStatus.ConditionChecks {
			// TODO(1022): Should  we allow multiple conditions of the same type?
			if conditionRegisterName == v.ConditionName {
				return k
			}
		}
	}
	return names.SimpleNameGenerator.RestrictLengthWithRandomSuffix(fmt.Sprintf("%s-%s", trName, conditionRegisterName))
}

// getTaskRunName should return a unique name for a `TaskRun` if one has not already been defined, and the existing one otherwise.
func getTaskRunName(taskRunsStatus map[string]*v1alpha1.PipelineRunTaskRunStatus, ptName, prName string) string {
	for k, v := range taskRunsStatus {
		if v.PipelineTaskName == ptName {
			return k
		}
	}

	return names.SimpleNameGenerator.RestrictLengthWithRandomSuffix(fmt.Sprintf("%s-%s", prName, ptName))
}

// GetPipelineConditionStatus will return the Condition that the PipelineRun prName should be
// updated with, based on the status of the TaskRuns in state.
func GetPipelineConditionStatus(pr *v1alpha1.PipelineRun, state PipelineRunState, logger *zap.SugaredLogger, dag *dag.Graph) *apis.Condition {
	// We have 4 different states here:
	// 1. Timed out -> Failed
	// 2. Any one TaskRun has failed - >Failed. This should change with #1020 and #1023
	// 3. All tasks are done or are skipped (i.e. condition check failed).-> Success
	// 4. A Task or Condition is running right now  or there are things left to run -> Running
	if pr.IsTimedOut() {
		return &apis.Condition{
			Type:    apis.ConditionSucceeded,
			Status:  corev1.ConditionFalse,
			Reason:  ReasonTimedOut,
			Message: fmt.Sprintf("PipelineRun %q failed to finish within %q", pr.Name, pr.Spec.Timeout.Duration.String()),
		}
	}

	// A single failed task mean we fail the pipeline
	for _, rprt := range state {
		if rprt.IsCancelled() {
			logger.Infof("TaskRun %s is cancelled, so PipelineRun %s is cancelled", rprt.TaskRunName, pr.Name)
			return &apis.Condition{
				Type:    apis.ConditionSucceeded,
				Status:  corev1.ConditionFalse,
				Reason:  ReasonCancelled,
				Message: fmt.Sprintf("TaskRun %s has cancelled", rprt.TaskRun.Name),
			}
		}

		if rprt.IsFailure() { //IsDone ensures we have crossed the retry limit
			logger.Infof("TaskRun %s has failed, so PipelineRun %s has failed, retries done: %b", rprt.TaskRunName, pr.Name, len(rprt.TaskRun.Status.RetriesStatus))
			return &apis.Condition{
				Type:    apis.ConditionSucceeded,
				Status:  corev1.ConditionFalse,
				Reason:  ReasonFailed,
				Message: fmt.Sprintf("TaskRun %s has failed", rprt.TaskRun.Name),
			}
		}
	}

	allTasks := []string{}
	successOrSkipTasks := []string{}
	skipTasks := int(0)

	// Check to see if all tasks are success or skipped
	for _, rprt := range state {
		allTasks = append(allTasks, rprt.PipelineTask.Name)
		if rprt.IsSuccessful() {
			successOrSkipTasks = append(successOrSkipTasks, rprt.PipelineTask.Name)
		}
		if isSkipped(rprt, state.ToMap(), dag) {
			skipTasks++
			successOrSkipTasks = append(successOrSkipTasks, rprt.PipelineTask.Name)
		}
	}

	if reflect.DeepEqual(allTasks, successOrSkipTasks) {
		logger.Infof("All TaskRuns have finished for PipelineRun %s so it has finished", pr.Name)
		reason := ReasonSucceeded
		if skipTasks != 0 {
			reason = ReasonCompleted
		}

		return &apis.Condition{
			Type:    apis.ConditionSucceeded,
			Status:  corev1.ConditionTrue,
			Reason:  reason,
			Message: fmt.Sprintf("Tasks Completed: %d, Skipped: %d", len(successOrSkipTasks)-skipTasks, skipTasks),
		}
	}

	// Hasn't timed out; no taskrun failed yet; and not all tasks have finished....
	// Must keep running then....
	return &apis.Condition{
		Type:    apis.ConditionSucceeded,
		Status:  corev1.ConditionUnknown,
		Reason:  ReasonRunning,
		Message: fmt.Sprintf("Tasks Completed: %d, Incomplete: %d, Skipped: %d", len(successOrSkipTasks)-skipTasks, len(allTasks)-len(successOrSkipTasks), skipTasks),
	}
}

// isSkipped returns true if a Task in a TaskRun will not be run either because
//  its Condition Checks failed or because one of the parent tasks's conditions failed
// Note that this means isSkipped returns false if a conditionCheck is in progress
func isSkipped(rprt *ResolvedPipelineRunTask, stateMap map[string]*ResolvedPipelineRunTask, d *dag.Graph) bool {
	// Taskrun not skipped if it already exists
	if rprt.TaskRun != nil {
		return false
	}

	// Check if conditionChecks have failed, if so task is skipped
	if len(rprt.ResolvedConditionChecks) > 0 {
		// isSkipped is only true iof
		if rprt.ResolvedConditionChecks.IsDone() && !rprt.ResolvedConditionChecks.IsSuccess() {
			return true
		}
	}

	// Recursively look at parent tasks to see if they have been skipped,
	// if any of the parents have been skipped, skip as well
	node := d.Nodes[rprt.PipelineTask.Name]
	for _, p := range node.Prev {
		skip := isSkipped(stateMap[p.Task.HashKey()], stateMap, d)
		if skip {
			return true
		}
	}
	return false
}

func resolveConditionChecks(pt *v1alpha1.PipelineTask, taskRunStatus map[string]*v1alpha1.PipelineRunTaskRunStatus, taskRunName string, getTaskRun resources.GetTaskRun, getCondition GetCondition, providedResources map[string]*v1alpha1.PipelineResource) ([]*ResolvedConditionCheck, error) {
	rccs := []*ResolvedConditionCheck{}
	for i := range pt.Conditions {
		ptc := pt.Conditions[i]
		cName := ptc.ConditionRef
		crName := fmt.Sprintf("%s-%s", cName, strconv.Itoa(i))
		c, err := getCondition(cName)
		if err != nil {
			return nil, &ConditionNotFoundError{
				Name: cName,
				Msg:  err.Error(),
			}
		}
		conditionCheckName := getConditionCheckName(taskRunStatus, taskRunName, crName)
		cctr, err := getTaskRun(conditionCheckName)
		if err != nil {
			if !errors.IsNotFound(err) {
				return nil, fmt.Errorf("error retrieving ConditionCheck %s for taskRun name %s : %w", conditionCheckName, taskRunName, err)
			}
		}
		conditionResources := map[string]*v1alpha1.PipelineResource{}
		for _, declared := range ptc.Resources {
			if r, ok := providedResources[declared.Resource]; ok {
				conditionResources[declared.Name] = r
			} else {
				for _, resource := range c.Spec.Resources {
					if declared.Name == resource.Name && !resource.Optional {
						return nil, fmt.Errorf("resources %s missing for condition %s in pipeline task %s", declared.Resource, cName, pt.Name)
					}
				}
			}
		}

		rcc := ResolvedConditionCheck{
			ConditionRegisterName: crName,
			Condition:             c,
			ConditionCheckName:    conditionCheckName,
			ConditionCheck:        v1alpha1.NewConditionCheck(cctr),
			PipelineTaskCondition: &ptc,
			ResolvedResources:     conditionResources,
		}

		rccs = append(rccs, &rcc)
	}
	return rccs, nil
}

// ResolvePipelineTaskResources matches PipelineResources referenced by pt inputs and outputs with the
// providedResources and returns an instance of ResolvedTaskResources.
func ResolvePipelineTaskResources(pt v1alpha1.PipelineTask, ts *v1alpha1.TaskSpec, taskName string, kind v1alpha1.TaskKind, providedResources map[string]*v1alpha1.PipelineResource) (*resources.ResolvedTaskResources, error) {
	rtr := resources.ResolvedTaskResources{
		TaskName: taskName,
		TaskSpec: ts,
		Kind:     kind,
		Inputs:   map[string]*v1alpha1.PipelineResource{},
		Outputs:  map[string]*v1alpha1.PipelineResource{},
	}
	if pt.Resources != nil {
		for _, taskInput := range pt.Resources.Inputs {
			if resource, ok := providedResources[taskInput.Resource]; ok {
				rtr.Inputs[taskInput.Name] = resource
			} else {
				if ts.Resources == nil || ts.Resources.Inputs == nil {
					return nil, fmt.Errorf("pipelineTask tried to use input resource %s not present in declared resources", taskInput.Resource)
				}
				for _, r := range ts.Resources.Inputs {
					if r.Name == taskInput.Name && !r.Optional {
						return nil, fmt.Errorf("pipelineTask tried to use input resource %s not present in declared resources", taskInput.Resource)
					}
				}
			}
		}
		for _, taskOutput := range pt.Resources.Outputs {
			if resource, ok := providedResources[taskOutput.Resource]; ok {
				rtr.Outputs[taskOutput.Name] = resource
			} else {
				if ts.Resources == nil || ts.Resources.Outputs == nil {
					return nil, fmt.Errorf("pipelineTask tried to use output resource %s not present in declared resources", taskOutput.Resource)
				}
				for _, r := range ts.Resources.Outputs {
					if r.Name == taskOutput.Name && !r.Optional {
						return nil, fmt.Errorf("pipelineTask tried to use output resource %s not present in declared resources", taskOutput.Resource)
					}
				}
			}
		}
	}
	return &rtr, nil
}
