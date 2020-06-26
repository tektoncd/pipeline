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
	"strconv"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"knative.dev/pkg/apis"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	resourcev1alpha1 "github.com/tektoncd/pipeline/pkg/apis/resource/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/contexts"
	"github.com/tektoncd/pipeline/pkg/list"
	"github.com/tektoncd/pipeline/pkg/names"
	"github.com/tektoncd/pipeline/pkg/reconciler/pipeline/dag"
	"github.com/tektoncd/pipeline/pkg/reconciler/taskrun/resources"
)

const (
	// ReasonConditionCheckFailed indicates that the reason for the failure status is that the
	// condition check associated to the pipeline task evaluated to false
	ReasonConditionCheckFailed = "ConditionCheckFailed"
)

// TaskNotFoundError indicates that the resolution failed because a referenced Task couldn't be retrieved
type TaskNotFoundError struct {
	Name string
	Msg  string
}

func (e *TaskNotFoundError) Error() string {
	return fmt.Sprintf("Couldn't retrieve Task %q: %s", e.Name, e.Msg)
}

// ConditionNotFoundError is used to track failures to the
type ConditionNotFoundError struct {
	Name string
	Msg  string
}

func (e *ConditionNotFoundError) Error() string {
	return fmt.Sprintf("Couldn't retrieve Condition %q: %s", e.Name, e.Msg)
}

// ResolvedPipelineRunTask contains a Task and its associated TaskRun, if it
// exists. TaskRun can be nil to represent there being no TaskRun.
type ResolvedPipelineRunTask struct {
	TaskRunName           string
	TaskRun               *v1beta1.TaskRun
	PipelineTask          *v1beta1.PipelineTask
	ResolvedTaskResources *resources.ResolvedTaskResources
	// ConditionChecks ~~TaskRuns but for evaling conditions
	ResolvedConditionChecks TaskConditionCheckState // Could also be a TaskRun or maybe just a Pod?
}

// PipelineRunState is a slice of ResolvedPipelineRunTasks the represents the current execution
// state of the PipelineRun.
type PipelineRunState []*ResolvedPipelineRunTask

// ResolvedPipelineRun represents the DAG of a pipeline along with its runtime state
type ResolvedPipelineRun struct {

	// Dag is the graph of all the pipeline tasks in the main pipeline
	Dag *dag.Graph

	// Final is the graph of all the final tasks executed once the main pipeline is complete
	Final *dag.Graph

	// State represents the state of all pipeline tasks in the pipeline
	DagState PipelineRunState

	// FinalState represents the state of all the final tasks in the pipeline
	FinalState PipelineRunState

	// WontRun is a hash of PipelineTask names that won't be executed because they are
	// not reachable via the DAG due to the pipeline state, or had failed conditions
	WontRun map[string]struct{}
}

// IsDone returns true if a PipelineTask was successful or failed all retries
// NOTE: a PipelineTask that has been cancelled is not always treated as "done",
// the condition must be checked separately
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

// IsFailure returns true only if the taskrun itself has failed and retries are exausted
func (t ResolvedPipelineRunTask) IsFailure() bool {
	if t.TaskRun == nil {
		return false
	}
	c := t.TaskRun.Status.GetCondition(apis.ConditionSucceeded)
	retriesDone := len(t.TaskRun.Status.RetriesStatus)
	retries := t.PipelineTask.Retries
	return c.IsFalse() && retriesDone >= retries
}

// HasFailedWithRetriesLeft returns true only if the taskrun itself has failed and retries are not exausted
func (t ResolvedPipelineRunTask) HasFailedWithRetriesLeft() bool {
	if t.TaskRun == nil {
		return false
	}
	c := t.TaskRun.Status.GetCondition(apis.ConditionSucceeded)
	retriesDone := len(t.TaskRun.Status.RetriesStatus)
	retries := t.PipelineTask.Retries
	return c.IsFalse() && retriesDone < retries
}

// IsCancelled returns true only if the taskrun itself has been cancelled
func (t ResolvedPipelineRunTask) IsCancelled() bool {
	if t.TaskRun == nil {
		return false
	}

	c := t.TaskRun.Status.GetCondition(apis.ConditionSucceeded)
	if c == nil {
		return false
	}

	return c.IsFalse() && c.Reason == v1beta1.TaskRunReasonCancelled.String()
}

// IsStarted returns true only if the PipelineRunTask itself has a TaskRun associated
func (t ResolvedPipelineRunTask) IsStarted() bool {
	if t.TaskRun == nil {
		return false
	}

	c := t.TaskRun.Status.GetCondition(apis.ConditionSucceeded)
	if c == nil {
		return false
	}

	return true
}

// HasFailedConditions returns true if the PipelineRunTask has attached conditions that failed
func (t ResolvedPipelineRunTask) HasFailedConditions() bool {

	// Check if conditionChecks have failed, if so task is skipped
	if len(t.ResolvedConditionChecks) == 0 {
		return false
	}

	// We wait for all conditions to complete before deciding
	return t.ResolvedConditionChecks.IsDone() && !t.ResolvedConditionChecks.IsSuccess()
}

// ToMap returns a map that maps pipeline task name to the resolved pipeline run task
func (state PipelineRunState) ToMap() map[string]*ResolvedPipelineRunTask {
	m := make(map[string]*ResolvedPipelineRunTask)
	for _, rprt := range state {
		m[rprt.PipelineTask.Name] = rprt
	}
	return m
}

// HsFailedOrCancelled returns true the state includes at least one failed or one cancelled task
func (state PipelineRunState) HasFailedOrCancelled() bool {
	for _, t := range state {
		if t.IsCancelled() || t.IsFailure() {
			return true
		}
	}
	return false
}

// State returned the combined state for the PipelineRun
func (rpr ResolvedPipelineRun) State() PipelineRunState {
	return append(rpr.DagState, rpr.FinalState...)
}

// isDAGDone returns true if no task from the DAG is running and no new one can be scheduled
func (rpr ResolvedPipelineRun) isDAGDone() bool {
	for _, t := range rpr.DagState {
		if t.TaskRun == nil {
			if rpr.IsTaskSkipped(t.PipelineTask.Name) {
				continue
			}
			return false
		}
		if !t.IsDone() && !t.IsCancelled() {
			return false
		}
	}
	return true
}

// isFinallyDone returns true if all final tasks are done (it does not related to the task being late :P)
func (rpr ResolvedPipelineRun) isFinallyDone() bool {
	for _, t := range rpr.FinalState {
		if t.TaskRun == nil || !t.IsDone() {
			return false
		}
	}
	return true
}

// IsDone returns true when both DAG and Final as done
func (rpr ResolvedPipelineRun) IsDone() (isDone bool) {
	return rpr.isDAGDone() && rpr.isFinallyDone()
}

// IsBeforeFirstTaskRun returns true if the PipelineRun has not yet started its first TaskRun
func (rpr ResolvedPipelineRun) IsBeforeFirstTaskRun() bool {
	for _, t := range rpr.DagState {
		if t.TaskRun != nil {
			return false
		}
	}
	return true
}

// IsStopping returns true if the PipelineRun won't be scheduling any new Task because
// at least one task already failed or was cancelled in the specified dag, but there are
// still tasks to be run in the DAG
func (rpr ResolvedPipelineRun) IsStopping() bool {
	return rpr.DagState.HasFailedOrCancelled() && !rpr.isDAGDone()
}

// IsTaskSkipped returns true if the PipelineRun won't be scheduling the name task
func (rpr ResolvedPipelineRun) IsTaskSkipped(taskName string) bool {
	_, inWontRun := rpr.WontRun[taskName]
	return inWontRun
}

// GetNextTasks returns a list of tasks which should be executed next i.e.
// a list of tasks from candidateTasks which aren't yet indicated in state to be running and
// a list of failed tasks from candidateTasks which haven't exhausted their retries
func (rpr ResolvedPipelineRun) GetNextTasks() ([]*ResolvedPipelineRunTask, error) {

	candidateTasks, err := dag.GetSchedulable(rpr.Dag, rpr.successfulOrSkipped()...)
	if err != nil {
		return nil, err
	}

	tasks := []*ResolvedPipelineRunTask{}
	for _, t := range rpr.DagState {
		if _, ok := candidateTasks[t.PipelineTask.Name]; !ok {
			continue
		}
		if t.TaskRun == nil {
			tasks = append(tasks, t)
		} else {
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
	return tasks, nil
}

// successfulOrSkipped returns a list of the names of all of the PipelineTasks in State
// which have successfully completed or skipped
func (rpr ResolvedPipelineRun) successfulOrSkipped() []string {
	tasks := []string{}
	for _, t := range rpr.DagState {
		if t.IsSuccessful() || rpr.IsTaskSkipped(t.PipelineTask.Name) {
			tasks = append(tasks, t.PipelineTask.Name)
		}
	}
	return tasks
}

// GetFinalTasks returns a list of final tasks without any taskRun associated with it
// GetFinalTasks returns final tasks only when all DAG tasks have finished executing successfully or skipped or
// any one DAG task resulted in failure
func (rpr ResolvedPipelineRun) GetFinalTasks() []*ResolvedPipelineRunTask {
	tasks := []*ResolvedPipelineRunTask{}
	// If the main DAG is still running, return nothing
	if !rpr.isDAGDone() {
		return tasks
	}
	// return list of tasks with all final tasks
	for _, t := range rpr.FinalState {
		// Final states cannot be skipped, so it's enough to check is a Task is not started
		// and not done in case of failure
		if t.TaskRun == nil {
			tasks = append(tasks, t)
		}
	}
	return tasks
}

// GetTaskRun is a function that will retrieve the TaskRun name.
type GetTaskRun func(name string) (*v1beta1.TaskRun, error)

// GetResourcesFromBindings will retrieve all Resources bound in PipelineRun pr and return a map
// from the declared name of the PipelineResource (which is how the PipelineResource will
// be referred to in the PipelineRun) to the PipelineResource, obtained via getResource.
func GetResourcesFromBindings(pr *v1beta1.PipelineRun, getResource resources.GetResource) (map[string]*resourcev1alpha1.PipelineResource, error) {
	rs := map[string]*resourcev1alpha1.PipelineResource{}
	for _, resource := range pr.Spec.Resources {
		r, err := resources.GetResourceFromBinding(resource, getResource)
		if err != nil {
			return rs, fmt.Errorf("error following resource reference for %s: %w", resource.Name, err)
		}
		rs[resource.Name] = r
	}
	return rs, nil
}

// ValidateResourceBindings validate that the PipelineResources declared in Pipeline p are bound in PipelineRun.
func ValidateResourceBindings(p *v1beta1.PipelineSpec, pr *v1beta1.PipelineRun) error {
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
func ValidateWorkspaceBindings(p *v1beta1.PipelineSpec, pr *v1beta1.PipelineRun) error {
	pipelineRunWorkspaces := make(map[string]v1beta1.WorkspaceBinding)
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

// ValidateServiceaccountMapping validates that the ServiceAccountNames defined by a PipelineRun are not correct.
func ValidateServiceaccountMapping(p *v1beta1.PipelineSpec, pr *v1beta1.PipelineRun) error {
	pipelineTasks := make(map[string]string)
	for _, task := range p.Tasks {
		pipelineTasks[task.Name] = task.Name
	}

	for _, name := range pr.Spec.ServiceAccountNames {
		if _, ok := pipelineTasks[name.TaskName]; !ok {
			return fmt.Errorf("PipelineRun's ServiceAccountNames defined wrong taskName: %q, not existed in Pipeline", name.TaskName)
		}
	}
	return nil
}

// ResolvePipelineRun retrieves all Tasks instances which are reference by tasks, getting
// instances from getTask. If it is unable to retrieve an instance of a referenced Task, it
// will return an error, otherwise it returns a list of all of the Tasks retrieved.
// It will retrieve the Resources needed for the TaskRun using the mapping of providedResources.
func ResolvePipelineRun(
	ctx context.Context,
	pipelineSpec v1beta1.PipelineSpec,
	pipelineRun v1beta1.PipelineRun,
	getTask resources.GetTask,
	getTaskRun resources.GetTaskRun,
	getClusterTask resources.GetClusterTask,
	getCondition GetCondition,
	providedResources map[string]*resourcev1alpha1.PipelineResource,
) (*ResolvedPipelineRun, error) {

	// We always validate the pipeline spec before resolving it, so errors are not
	// expected here.
	d, err := dag.Build(v1beta1.PipelineTaskList(pipelineSpec.Tasks))
	if err != nil {
		return nil, err
	}
	final, err := dag.Build(v1beta1.PipelineTaskList(pipelineSpec.Finally))
	if err != nil {
		return nil, err
	}

	state, err := resolvePipelineRun(ctx, pipelineSpec.Tasks, pipelineRun, getTask, getTaskRun, getClusterTask,
		getCondition, providedResources)
	if err != nil {
		return nil, err
	}

	finalState, err := resolvePipelineRun(ctx, pipelineSpec.Finally, pipelineRun, getTask, getTaskRun, getClusterTask,
		getCondition, providedResources)
	if err != nil {
		return nil, err
	}

	return &ResolvedPipelineRun{
		Dag:        d,
		Final:      final,
		DagState:   state,
		FinalState: finalState,
		WontRun:    skippedTasks(state, d),
	}, nil
}

func resolvePipelineRun(
	ctx context.Context,
	tasks []v1beta1.PipelineTask,
	pipelineRun v1beta1.PipelineRun,
	getTask resources.GetTask,
	getTaskRun resources.GetTaskRun,
	getClusterTask resources.GetClusterTask,
	getCondition GetCondition,
	providedResources map[string]*resourcev1alpha1.PipelineResource,
) (PipelineRunState, error) {
	state := []*ResolvedPipelineRunTask{}
	for i := range tasks {
		pt := tasks[i]

		rprt := ResolvedPipelineRunTask{
			PipelineTask: &pt,
			TaskRunName:  GetTaskRunName(pipelineRun.Status.TaskRuns, pt.Name, pipelineRun.Name),
		}

		// Find the Task that this PipelineTask is using
		var (
			t        v1beta1.TaskInterface
			err      error
			spec     v1beta1.TaskSpec
			taskName string
			kind     v1beta1.TaskKind
		)

		if pt.TaskRef != nil {
			if pt.TaskRef.Kind == v1beta1.ClusterTaskKind {
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

// skippedTasks returns returns a list of tasks that won't be executed
// When the state includes failed or cancelled tasks, all tasks that are not running, complete
// or cancelled are returned. When the pipeline is running normally, all tasks that have failed
// conditions or that depend on tasks with failed conditions are returned.
func skippedTasks(state PipelineRunState, d *dag.Graph) map[string]struct{} {
	skipped := map[string]struct{}{}
	stateMap := state.ToMap()
	failedOrCancelled := state.HasFailedOrCancelled()

	for _, task := range state {
		if task.IsDone() || task.IsCancelled() {
			continue
		}
		// We have left tasks that are not running. They may be waiting on a dependency, waiting on a
		// condition or going to retry after a failure
		if failedOrCancelled {
			if !task.IsStarted() || task.HasFailedWithRetriesLeft() {
				skipped[task.PipelineTask.Name] = struct{}{}
				continue
			}
		}
		// If a task has failed conditions is skipped
		if isSkipped(task, stateMap, d) {
			skipped[task.PipelineTask.Name] = struct{}{}
			continue
		}
	}
	return skipped
}

// getConditionCheckName should return a unique name for a `ConditionCheck` if one has not already been defined, and the existing one otherwise.
func getConditionCheckName(taskRunStatus map[string]*v1beta1.PipelineRunTaskRunStatus, trName, conditionRegisterName string) string {
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

// GetTaskRunName should return a unique name for a `TaskRun` if one has not already been defined, and the existing one otherwise.
func GetTaskRunName(taskRunsStatus map[string]*v1beta1.PipelineRunTaskRunStatus, ptName, prName string) string {
	for k, v := range taskRunsStatus {
		if v.PipelineTaskName == ptName {
			return k
		}
	}

	return names.SimpleNameGenerator.RestrictLengthWithRandomSuffix(fmt.Sprintf("%s-%s", prName, ptName))
}

// GetPipelineConditionStatus will return the Condition that the PipelineRun prName should be
// updated with, based on the status of the TaskRuns in state.
func GetPipelineConditionStatus(pr v1beta1.PipelineRun, rpr ResolvedPipelineRun, logger *zap.SugaredLogger) *apis.Condition {
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

	skippedTasks := len(rpr.WontRun)
	failedTasks := int(0)
	cancelledTasks := int(0)
	succeededTasks := int(0)
	reason := v1beta1.PipelineRunReasonSuccessful.String()
	state := rpr.State()

	// Calculate the number of cancelled and failed tasks
	//
	// The completion reason is also calculated here, but it will only be used
	// if all tasks are completed.
	//
	// The pipeline run completion reason is set from the taskrun completion reason
	// according to the following logic:
	//
	// - All successful: ReasonSucceeded
	// - Some successful, some skipped: ReasonCompleted
	// - Some cancelled, none failed: ReasonCancelled
	// - At least one failed: ReasonFailed
	for _, rprt := range state {
		switch {
		case rprt.IsSuccessful():
			succeededTasks++
		case rpr.IsTaskSkipped(rprt.PipelineTask.Name):
			// At least one is skipped and no failure yet, mark as completed
			if reason == v1beta1.PipelineRunReasonSuccessful.String() {
				reason = v1beta1.PipelineRunReasonCompleted.String()
			}
		case rprt.IsCancelled():
			cancelledTasks++
			if reason != v1beta1.PipelineRunReasonFailed.String() {
				reason = v1beta1.PipelineRunReasonCancelled.String()
			}
		case rprt.IsFailure():
			failedTasks++
			reason = v1beta1.PipelineRunReasonFailed.String()
		}
	}

	if rpr.IsDone() {
		status := corev1.ConditionTrue
		if failedTasks > 0 || cancelledTasks > 0 {
			status = corev1.ConditionFalse
		}
		logger.Infof("All TaskRuns have finished for PipelineRun %s so it has finished", pr.Name)
		return &apis.Condition{
			Type:   apis.ConditionSucceeded,
			Status: status,
			Reason: reason,
			Message: fmt.Sprintf("Tasks Completed: %d (Failed: %d, Cancelled %d), Skipped: %d",
				len(state)-skippedTasks, failedTasks, cancelledTasks, skippedTasks),
		}
	}

	// Hasn't timed out; not all tasks have finished.... Must keep running then....
	if rpr.IsStopping() {
		reason = v1beta1.PipelineRunReasonStopping.String()
	} else {
		reason = v1beta1.PipelineRunReasonRunning.String()
	}
	completedTasks := succeededTasks + failedTasks + cancelledTasks
	return &apis.Condition{
		Type:   apis.ConditionSucceeded,
		Status: corev1.ConditionUnknown,
		Reason: reason,
		Message: fmt.Sprintf("Tasks Completed: %d (Failed: %d, Cancelled %d), Incomplete: %d, Skipped: %d",
			completedTasks, failedTasks, cancelledTasks, len(state)-completedTasks, skippedTasks),
	}
}

// isSkipped returns true if a Task in a TaskRun will not be run either because
// its Condition Checks failed or because one of the parent tasks' conditions failed
// Note that this means isSkipped returns false if a conditionCheck is in progress
func isSkipped(rprt *ResolvedPipelineRunTask, stateMap map[string]*ResolvedPipelineRunTask, d *dag.Graph) bool {
	// Check if conditionChecks have failed, if so task is skipped
	if rprt.HasFailedConditions() {
		return true
	}

	// Recursively look at parent tasks to see if they have been skipped,
	// if any of the parents have been skipped, skip as well
	// continue if the task does not belong to the specified Graph
	if node, ok := d.Nodes[rprt.PipelineTask.Name]; ok {
		for _, p := range node.Prev {
			skip := isSkipped(stateMap[p.Task.HashKey()], stateMap, d)
			if skip {
				return true
			}
		}
	}
	return false
}

func resolveConditionChecks(pt *v1beta1.PipelineTask, taskRunStatus map[string]*v1beta1.PipelineRunTaskRunStatus, taskRunName string, getTaskRun resources.GetTaskRun, getCondition GetCondition, providedResources map[string]*resourcev1alpha1.PipelineResource) ([]*ResolvedConditionCheck, error) {
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
		conditionResources := map[string]*resourcev1alpha1.PipelineResource{}
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
			ConditionCheck:        v1beta1.NewConditionCheck(cctr),
			PipelineTaskCondition: &ptc,
			ResolvedResources:     conditionResources,
		}

		rccs = append(rccs, &rcc)
	}
	return rccs, nil
}

// ResolvePipelineTaskResources matches PipelineResources referenced by pt inputs and outputs with the
// providedResources and returns an instance of ResolvedTaskResources.
func ResolvePipelineTaskResources(pt v1beta1.PipelineTask, ts *v1beta1.TaskSpec, taskName string, kind v1beta1.TaskKind, providedResources map[string]*resourcev1alpha1.PipelineResource) (*resources.ResolvedTaskResources, error) {
	rtr := resources.ResolvedTaskResources{
		TaskName: taskName,
		TaskSpec: ts,
		Kind:     kind,
		Inputs:   map[string]*resourcev1alpha1.PipelineResource{},
		Outputs:  map[string]*resourcev1alpha1.PipelineResource{},
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
