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
	"errors"
	"fmt"

	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/pkg/reconciler/taskrun/resources"
	"github.com/tektoncd/pipeline/pkg/remote"
	"github.com/tektoncd/pipeline/pkg/trustedresources"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/kmeta"
)

const (
	// ReasonConditionCheckFailed indicates that the reason for the failure status is that the
	// condition check associated to the pipeline task evaluated to false
	ReasonConditionCheckFailed = "ConditionCheckFailed"
)

// TaskSkipStatus stores whether a task was skipped and why
type TaskSkipStatus struct {
	IsSkipped      bool
	SkippingReason v1beta1.SkippingReason
}

// TaskNotFoundError indicates that the resolution failed because a referenced Task couldn't be retrieved
type TaskNotFoundError struct {
	Name string
	Msg  string
}

func (e *TaskNotFoundError) Error() string {
	return fmt.Sprintf("Couldn't retrieve Task %q: %s", e.Name, e.Msg)
}

// ResolvedPipelineTask contains a PipelineTask and its associated TaskRun(s) or RunObjects, if they exist.
type ResolvedPipelineTask struct {
	TaskRunName  string
	TaskRun      *v1beta1.TaskRun
	TaskRunNames []string
	TaskRuns     []*v1beta1.TaskRun
	// If the PipelineTask is a Custom Task, RunObjectName and RunObject will be set.
	CustomTask     bool
	RunObjectName  string
	RunObject      v1beta1.RunObject
	RunObjectNames []string
	RunObjects     []v1beta1.RunObject
	PipelineTask   *v1beta1.PipelineTask
	ResolvedTask   *resources.ResolvedTask
}

// isDone returns true only if the task is skipped, succeeded or failed
func (t ResolvedPipelineTask) isDone(facts *PipelineRunFacts) bool {
	return t.Skip(facts).IsSkipped || t.isSuccessful() || t.isFailure()
}

// IsRunning returns true only if the task is neither succeeded, cancelled nor failed
func (t ResolvedPipelineTask) IsRunning() bool {
	switch {
	case t.IsCustomTask() && t.PipelineTask.IsMatrixed():
		if len(t.RunObjects) == 0 {
			return false
		}
	case t.IsCustomTask():
		if t.RunObject == nil {
			return false
		}
	case t.PipelineTask.IsMatrixed():
		if len(t.TaskRuns) == 0 {
			return false
		}
	default:
		if t.TaskRun == nil {
			return false
		}
	}
	return !t.isSuccessful() && !t.isFailure()
}

// IsCustomTask returns true if the PipelineTask references a Custom Task.
func (t ResolvedPipelineTask) IsCustomTask() bool {
	return t.CustomTask
}

// isSuccessful returns true only if the run has completed successfully
// If the PipelineTask has a Matrix, isSuccessful returns true if all runs have completed successfully
func (t ResolvedPipelineTask) isSuccessful() bool {
	switch {
	case t.IsCustomTask() && t.PipelineTask.IsMatrixed():
		if len(t.RunObjects) == 0 {
			return false
		}
		for _, run := range t.RunObjects {
			if !run.IsSuccessful() {
				return false
			}
		}
		return true
	case t.IsCustomTask():
		return t.RunObject != nil && t.RunObject.IsSuccessful()
	case t.PipelineTask.IsMatrixed():
		if len(t.TaskRuns) == 0 {
			return false
		}
		for _, taskRun := range t.TaskRuns {
			if !taskRun.IsSuccessful() {
				return false
			}
		}
		return true
	default:
		return t.TaskRun.IsSuccessful()
	}
}

// isFailure returns true only if the run has failed and will not be retried.
// If the PipelineTask has a Matrix, isFailure returns true if any run has failed (no remaining retries)
// and all other runs are done.
func (t ResolvedPipelineTask) isFailure() bool {
	if t.isCancelledForTimeOut() {
		return true
	}
	if t.isCancelled() {
		return true
	}
	if t.isSuccessful() {
		return false
	}
	var c *apis.Condition
	var isDone bool
	switch {
	case t.IsCustomTask() && t.PipelineTask.IsMatrixed():
		if len(t.RunObjects) == 0 {
			return false
		}
		isDone = true
		atLeastOneFailed := false
		for _, run := range t.RunObjects {
			isDone = isDone && run.IsDone()
			runFailed := run.GetStatusCondition().GetCondition(apis.ConditionSucceeded).IsFalse()
			atLeastOneFailed = atLeastOneFailed || runFailed
		}
		return atLeastOneFailed && isDone
	case t.IsCustomTask():
		if t.RunObject == nil {
			return false
		}
		c = t.RunObject.GetStatusCondition().GetCondition(apis.ConditionSucceeded)
		isDone = t.RunObject.IsDone()
		return isDone && c.IsFalse()
	case t.PipelineTask.IsMatrixed():
		if len(t.TaskRuns) == 0 {
			return false
		}
		isDone = true
		atLeastOneFailed := false
		for _, taskRun := range t.TaskRuns {
			isDone = isDone && taskRun.IsDone()
			taskRunFailed := taskRun.Status.GetCondition(apis.ConditionSucceeded).IsFalse()
			atLeastOneFailed = atLeastOneFailed || taskRunFailed
		}
		return atLeastOneFailed && isDone
	default:
		if t.TaskRun == nil {
			return false
		}
		c = t.TaskRun.Status.GetCondition(apis.ConditionSucceeded)
		isDone = t.TaskRun.IsDone()
		return isDone && c.IsFalse()
	}
}

// isCancelledForTimeOut returns true only if the run is cancelled due to PipelineRun-controlled timeout
// If the PipelineTask has a Matrix, isCancelled returns true if any run is cancelled due to PipelineRun-controlled timeout and all other runs are done.
func (t ResolvedPipelineTask) isCancelledForTimeOut() bool {
	switch {
	case t.IsCustomTask() && t.PipelineTask.IsMatrixed():
		if len(t.RunObjects) == 0 {
			return false
		}
		isDone := true
		atLeastOneCancelled := false
		for _, run := range t.RunObjects {
			isDone = isDone && run.IsDone()
			c := run.GetStatusCondition().GetCondition(apis.ConditionSucceeded)
			runCancelled := c.IsFalse() &&
				c.Reason == v1beta1.CustomRunReasonCancelled.String() &&
				isCustomRunCancelledByPipelineRunTimeout(run)
			atLeastOneCancelled = atLeastOneCancelled || runCancelled
		}
		return atLeastOneCancelled && isDone
	case t.IsCustomTask():
		if t.RunObject == nil {
			return false
		}
		c := t.RunObject.GetStatusCondition().GetCondition(apis.ConditionSucceeded)
		return c != nil && c.IsFalse() &&
			c.Reason == v1beta1.CustomRunReasonCancelled.String() &&
			isCustomRunCancelledByPipelineRunTimeout(t.RunObject)
	case t.PipelineTask.IsMatrixed():
		if len(t.TaskRuns) == 0 {
			return false
		}
		isDone := true
		atLeastOneCancelled := false
		for _, taskRun := range t.TaskRuns {
			isDone = isDone && taskRun.IsDone()
			c := taskRun.Status.GetCondition(apis.ConditionSucceeded)
			taskRunCancelled := c.IsFalse() &&
				c.Reason == v1beta1.TaskRunReasonCancelled.String() &&
				taskRun.Spec.StatusMessage == v1beta1.TaskRunCancelledByPipelineTimeoutMsg
			atLeastOneCancelled = atLeastOneCancelled || taskRunCancelled
		}
		return atLeastOneCancelled && isDone
	default:
		if t.TaskRun == nil {
			return false
		}
		c := t.TaskRun.Status.GetCondition(apis.ConditionSucceeded)
		return c != nil && c.IsFalse() &&
			c.Reason == v1beta1.TaskRunReasonCancelled.String() &&
			t.TaskRun.Spec.StatusMessage == v1beta1.TaskRunCancelledByPipelineTimeoutMsg
	}
}

// isCancelled returns true only if the run is cancelled
// If the PipelineTask has a Matrix, isCancelled returns true if any run is cancelled and all other runs are done.
func (t ResolvedPipelineTask) isCancelled() bool {
	switch {
	case t.IsCustomTask() && t.PipelineTask.IsMatrixed():
		if len(t.RunObjects) == 0 {
			return false
		}
		isDone := true
		atLeastOneCancelled := false
		for _, run := range t.RunObjects {
			isDone = isDone && run.IsDone()
			c := run.GetStatusCondition().GetCondition(apis.ConditionSucceeded)
			runCancelled := c.IsFalse() && c.Reason == v1beta1.CustomRunReasonCancelled.String()
			atLeastOneCancelled = atLeastOneCancelled || runCancelled
		}
		return atLeastOneCancelled && isDone
	case t.IsCustomTask():
		if t.RunObject == nil {
			return false
		}
		c := t.RunObject.GetStatusCondition().GetCondition(apis.ConditionSucceeded)
		return c != nil && c.IsFalse() && c.Reason == v1beta1.CustomRunReasonCancelled.String()
	case t.PipelineTask.IsMatrixed():
		if len(t.TaskRuns) == 0 {
			return false
		}
		isDone := true
		atLeastOneCancelled := false
		for _, taskRun := range t.TaskRuns {
			isDone = isDone && taskRun.IsDone()
			c := taskRun.Status.GetCondition(apis.ConditionSucceeded)
			taskRunCancelled := c.IsFalse() && c.Reason == v1beta1.TaskRunReasonCancelled.String()
			atLeastOneCancelled = atLeastOneCancelled || taskRunCancelled
		}
		return atLeastOneCancelled && isDone
	default:
		if t.TaskRun == nil {
			return false
		}
		c := t.TaskRun.Status.GetCondition(apis.ConditionSucceeded)
		return c != nil && c.IsFalse() && c.Reason == v1beta1.TaskRunReasonCancelled.String()
	}
}

// isScheduled returns true when the PipelineRunTask itself has a TaskRun
// or Run associated.
func (t ResolvedPipelineTask) isScheduled() bool {
	if t.IsCustomTask() {
		return t.RunObject != nil
	}
	return t.TaskRun != nil
}

// isStarted returns true only if the PipelineRunTask itself has a TaskRun or
// Run associated that has a Succeeded-type condition.
func (t ResolvedPipelineTask) isStarted() bool {
	if t.IsCustomTask() {
		return t.RunObject != nil && t.RunObject.GetStatusCondition().GetCondition(apis.ConditionSucceeded) != nil
	}
	return t.TaskRun != nil && t.TaskRun.Status.GetCondition(apis.ConditionSucceeded) != nil
}

// isConditionStatusFalse returns true when a task has succeeded condition with status set to false
// it includes task failed after retries are exhausted, cancelled tasks, and time outs
func (t ResolvedPipelineTask) isConditionStatusFalse() bool {
	if t.isStarted() {
		if t.IsCustomTask() {
			return t.RunObject.GetStatusCondition().GetCondition(apis.ConditionSucceeded).IsFalse()
		}
		return t.TaskRun.Status.GetCondition(apis.ConditionSucceeded).IsFalse()
	}
	return false
}

// haveAnyRunsFailed returns true when any of the taskRuns/customRuns have succeeded condition with status set to false
func (t ResolvedPipelineTask) haveAnyRunsFailed() bool {
	if t.IsCustomTask() {
		return t.haveAnyCustomRunsFailed()
	}
	return t.haveAnyTaskRunsFailed()
}

// haveAnyTaskRunsFailed returns true when any of the taskRuns have succeeded condition with status set to false
func (t ResolvedPipelineTask) haveAnyTaskRunsFailed() bool {
	if len(t.TaskRuns) > 0 || t.PipelineTask.IsMatrixed() {
		for _, taskRun := range t.TaskRuns {
			if taskRun.IsFailure() {
				return true
			}
		}
	}
	return t.TaskRun.IsFailure()
}

// haveAnyCustomRunsFailed returns true when a CustomRun has succeeded condition with status set to false
func (t ResolvedPipelineTask) haveAnyCustomRunsFailed() bool {
	if len(t.RunObjects) > 0 || t.PipelineTask.IsMatrixed() {
		for _, customRun := range t.RunObjects {
			if customRun.IsFailure() {
				return true
			}
		}
	}
	return t.RunObject != nil && t.RunObject.IsFailure()
}

func (t *ResolvedPipelineTask) checkParentsDone(facts *PipelineRunFacts) bool {
	if facts.isFinalTask(t.PipelineTask.Name) {
		return true
	}
	stateMap := facts.State.ToMap()
	node := facts.TasksGraph.Nodes[t.PipelineTask.Name]
	for _, p := range node.Prev {
		if !stateMap[p.Key].isDone(facts) {
			return false
		}
	}
	return true
}

func (t *ResolvedPipelineTask) skip(facts *PipelineRunFacts) TaskSkipStatus {
	var skippingReason v1beta1.SkippingReason

	switch {
	case facts.isFinalTask(t.PipelineTask.Name) || t.isScheduled():
		skippingReason = v1beta1.None
	case facts.IsStopping():
		skippingReason = v1beta1.StoppingSkip
	case facts.IsGracefullyCancelled():
		skippingReason = v1beta1.GracefullyCancelledSkip
	case facts.IsGracefullyStopped():
		skippingReason = v1beta1.GracefullyStoppedSkip
	case t.skipBecauseParentTaskWasSkipped(facts):
		skippingReason = v1beta1.ParentTasksSkip
	case t.skipBecauseResultReferencesAreMissing(facts):
		skippingReason = v1beta1.MissingResultsSkip
	case t.skipBecauseWhenExpressionsEvaluatedToFalse(facts):
		skippingReason = v1beta1.WhenExpressionsSkip
	case t.skipBecausePipelineRunPipelineTimeoutReached(facts):
		skippingReason = v1beta1.PipelineTimedOutSkip
	case t.skipBecausePipelineRunTasksTimeoutReached(facts):
		skippingReason = v1beta1.TasksTimedOutSkip
	case t.skipBecauseEmptyArrayInMatrixParams():
		skippingReason = v1beta1.EmptyArrayInMatrixParams
	default:
		skippingReason = v1beta1.None
	}

	return TaskSkipStatus{
		IsSkipped:      skippingReason != v1beta1.None,
		SkippingReason: skippingReason,
	}
}

// Skip returns true if a PipelineTask will not be run because
// (1) its When Expressions evaluated to false
// (2) its Condition Checks failed
// (3) its parent task was skipped
// (4) Pipeline is in stopping state (one of the PipelineTasks failed)
// (5) Pipeline is gracefully cancelled or stopped
func (t *ResolvedPipelineTask) Skip(facts *PipelineRunFacts) TaskSkipStatus {
	if facts.SkipCache == nil {
		facts.SkipCache = make(map[string]TaskSkipStatus)
	}
	if _, cached := facts.SkipCache[t.PipelineTask.Name]; !cached {
		facts.SkipCache[t.PipelineTask.Name] = t.skip(facts)
	}
	return facts.SkipCache[t.PipelineTask.Name]
}

// skipBecauseWhenExpressionsEvaluatedToFalse confirms that the when expressions have completed evaluating, and
// it returns true if any of the when expressions evaluate to false
func (t *ResolvedPipelineTask) skipBecauseWhenExpressionsEvaluatedToFalse(facts *PipelineRunFacts) bool {
	if t.checkParentsDone(facts) {
		if !t.PipelineTask.WhenExpressions.AllowsExecution() {
			return true
		}
	}
	return false
}

// skipBecauseParentTaskWasSkipped loops through the parent tasks and checks if the parent task skipped:
//
//	if yes, is it because of when expressions?
//	    if yes, it ignores this parent skip and continue evaluating other parent tasks
//	    if no, it returns true to skip the current task because this parent task was skipped
//	if no, it continues checking the other parent tasks
func (t *ResolvedPipelineTask) skipBecauseParentTaskWasSkipped(facts *PipelineRunFacts) bool {
	stateMap := facts.State.ToMap()
	node := facts.TasksGraph.Nodes[t.PipelineTask.Name]
	for _, p := range node.Prev {
		parentTask := stateMap[p.Key]
		if parentSkipStatus := parentTask.Skip(facts); parentSkipStatus.IsSkipped {
			// if the parent task was skipped due to its `when` expressions,
			// then we should ignore that and continue evaluating if we should skip because of other parent tasks
			if parentSkipStatus.SkippingReason == v1beta1.WhenExpressionsSkip {
				continue
			}
			return true
		}
	}
	return false
}

// skipBecauseResultReferencesAreMissing checks if the task references results that cannot be resolved, which is a
// reason for skipping the task, and applies result references if found
func (t *ResolvedPipelineTask) skipBecauseResultReferencesAreMissing(facts *PipelineRunFacts) bool {
	if t.checkParentsDone(facts) && t.hasResultReferences() {
		resolvedResultRefs, pt, err := ResolveResultRefs(facts.State, PipelineRunState{t})
		rpt := facts.State.ToMap()[pt]
		if rpt != nil {
			if err != nil && (t.IsFinalTask(facts) || rpt.Skip(facts).SkippingReason == v1beta1.WhenExpressionsSkip) {
				return true
			}
		}
		ApplyTaskResults(PipelineRunState{t}, resolvedResultRefs)
		facts.ResetSkippedCache()
	}
	return false
}

// skipBecausePipelineRunPipelineTimeoutReached returns true if the task shouldn't be launched because the elapsed time since
// the PipelineRun started is greater than the PipelineRun's pipeline timeout
func (t *ResolvedPipelineTask) skipBecausePipelineRunPipelineTimeoutReached(facts *PipelineRunFacts) bool {
	if t.checkParentsDone(facts) {
		if facts.TimeoutsState.PipelineTimeout != nil && *facts.TimeoutsState.PipelineTimeout != config.NoTimeoutDuration && facts.TimeoutsState.StartTime != nil {
			// If the elapsed time since the PipelineRun's start time is greater than the PipelineRun's Pipeline timeout, skip.
			return facts.TimeoutsState.Clock.Since(*facts.TimeoutsState.StartTime) > *facts.TimeoutsState.PipelineTimeout
		}
	}

	return false
}

// skipBecausePipelineRunTasksTimeoutReached returns true if the task shouldn't be launched because the elapsed time since
// the PipelineRun started is greater than the PipelineRun's tasks timeout
func (t *ResolvedPipelineTask) skipBecausePipelineRunTasksTimeoutReached(facts *PipelineRunFacts) bool {
	if t.checkParentsDone(facts) && !t.IsFinalTask(facts) {
		if facts.TimeoutsState.TasksTimeout != nil && *facts.TimeoutsState.TasksTimeout != config.NoTimeoutDuration && facts.TimeoutsState.StartTime != nil {
			// If the elapsed time since the PipelineRun's start time is greater than the PipelineRun's Tasks timeout, skip.
			return facts.TimeoutsState.Clock.Since(*facts.TimeoutsState.StartTime) > *facts.TimeoutsState.TasksTimeout
		}
	}

	return false
}

// skipBecausePipelineRunFinallyTimeoutReached returns true if the task shouldn't be launched because the elapsed time since
// finally tasks started being executed is greater than the PipelineRun's finally timeout
func (t *ResolvedPipelineTask) skipBecausePipelineRunFinallyTimeoutReached(facts *PipelineRunFacts) bool {
	if t.checkParentsDone(facts) && t.IsFinalTask(facts) {
		if facts.TimeoutsState.FinallyTimeout != nil && *facts.TimeoutsState.FinallyTimeout != config.NoTimeoutDuration && facts.TimeoutsState.FinallyStartTime != nil {
			// If the elapsed time since the PipelineRun's finally start time is greater than the PipelineRun's finally timeout, skip.
			return facts.TimeoutsState.Clock.Since(*facts.TimeoutsState.FinallyStartTime) > *facts.TimeoutsState.FinallyTimeout
		}
	}

	return false
}

// skipBecauseEmptyArrayInMatrixParams returns true if the matrix parameters contain an empty array
func (t *ResolvedPipelineTask) skipBecauseEmptyArrayInMatrixParams() bool {
	if t.PipelineTask.IsMatrixed() {
		for _, ps := range t.PipelineTask.Matrix.Params {
			if len(ps.Value.ArrayVal) == 0 {
				return true
			}
		}
	}

	return false
}

// IsFinalTask returns true if a task is a finally task
func (t *ResolvedPipelineTask) IsFinalTask(facts *PipelineRunFacts) bool {
	return facts.isFinalTask(t.PipelineTask.Name)
}

// IsFinallySkipped returns true if a finally task is not executed and skipped due to task result validation failure
func (t *ResolvedPipelineTask) IsFinallySkipped(facts *PipelineRunFacts) TaskSkipStatus {
	var skippingReason v1beta1.SkippingReason

	switch {
	case t.isScheduled():
		skippingReason = v1beta1.None
	case facts.checkDAGTasksDone() && facts.isFinalTask(t.PipelineTask.Name):
		switch {
		case t.skipBecauseResultReferencesAreMissing(facts):
			skippingReason = v1beta1.MissingResultsSkip
		case t.skipBecauseWhenExpressionsEvaluatedToFalse(facts):
			skippingReason = v1beta1.WhenExpressionsSkip
		case t.skipBecausePipelineRunPipelineTimeoutReached(facts):
			skippingReason = v1beta1.PipelineTimedOutSkip
		case t.skipBecausePipelineRunFinallyTimeoutReached(facts):
			skippingReason = v1beta1.FinallyTimedOutSkip
		case t.skipBecauseEmptyArrayInMatrixParams():
			skippingReason = v1beta1.EmptyArrayInMatrixParams
		default:
			skippingReason = v1beta1.None
		}
	default:
		skippingReason = v1beta1.None
	}

	return TaskSkipStatus{
		IsSkipped:      skippingReason != v1beta1.None,
		SkippingReason: skippingReason,
	}
}

// GetRun is a function that will retrieve a Run by name.
type GetRun func(name string) (v1beta1.RunObject, error)

// ValidateWorkspaceBindings validates that the Workspaces expected by a Pipeline are provided by a PipelineRun.
func ValidateWorkspaceBindings(p *v1beta1.PipelineSpec, pr *v1beta1.PipelineRun) error {
	pipelineRunWorkspaces := make(map[string]v1beta1.WorkspaceBinding)
	for _, binding := range pr.Spec.Workspaces {
		pipelineRunWorkspaces[binding.Name] = binding
	}

	for _, ws := range p.Workspaces {
		if ws.Optional {
			continue
		}
		if _, ok := pipelineRunWorkspaces[ws.Name]; !ok {
			return fmt.Errorf("pipeline requires workspace with name %q be provided by pipelinerun", ws.Name)
		}
	}
	return nil
}

// ValidateTaskRunSpecs that the TaskRunSpecs defined by a PipelineRun are correct.
func ValidateTaskRunSpecs(p *v1beta1.PipelineSpec, pr *v1beta1.PipelineRun) error {
	pipelineTasks := make(map[string]string)
	for _, task := range p.Tasks {
		pipelineTasks[task.Name] = task.Name
	}

	for _, task := range p.Finally {
		pipelineTasks[task.Name] = task.Name
	}

	for _, taskrunSpec := range pr.Spec.TaskRunSpecs {
		if _, ok := pipelineTasks[taskrunSpec.PipelineTaskName]; !ok {
			return fmt.Errorf("PipelineRun's taskrunSpecs defined wrong taskName: %q, does not exist in Pipeline", taskrunSpec.PipelineTaskName)
		}
	}
	return nil
}

// ResolvePipelineTask retrieves a single Task's instance using the getTask to fetch
// the spec. If it is unable to retrieve an instance of a referenced Task, it  will return
// an error, otherwise it returns a list of all the Tasks retrieved.  It will retrieve
// the Resources needed for the TaskRuns or RunObjects using the mapping of providedResources.
func ResolvePipelineTask(
	ctx context.Context,
	pipelineRun v1beta1.PipelineRun,
	getTask resources.GetTask,
	getTaskRun resources.GetTaskRun,
	getRun GetRun,
	pipelineTask v1beta1.PipelineTask,
) (*ResolvedPipelineTask, error) {
	rpt := ResolvedPipelineTask{
		PipelineTask: &pipelineTask,
	}
	rpt.CustomTask = rpt.PipelineTask.TaskRef.IsCustomTask() || rpt.PipelineTask.TaskSpec.IsCustomTask()
	switch {
	case rpt.IsCustomTask() && rpt.PipelineTask.IsMatrixed():
		rpt.RunObjectNames = getNamesOfRuns(pipelineRun.Status.ChildReferences, pipelineTask.Name, pipelineRun.Name, pipelineTask.Matrix.CountCombinations())
		for _, runName := range rpt.RunObjectNames {
			run, err := getRun(runName)
			if err != nil && !kerrors.IsNotFound(err) {
				return nil, fmt.Errorf("error retrieving RunObject %s: %w", runName, err)
			}
			if run != nil {
				rpt.RunObjects = append(rpt.RunObjects, run)
			}
		}
	case rpt.IsCustomTask():
		rpt.RunObjectName = getRunName(pipelineRun.Status.ChildReferences, pipelineTask.Name, pipelineRun.Name)
		run, err := getRun(rpt.RunObjectName)
		if err != nil && !kerrors.IsNotFound(err) {
			return nil, fmt.Errorf("error retrieving RunObject %s: %w", rpt.RunObjectName, err)
		}
		if run != nil {
			rpt.RunObject = run
		}
	case rpt.PipelineTask.IsMatrixed():
		rpt.TaskRunNames = GetNamesOfTaskRuns(pipelineRun.Status.ChildReferences, pipelineTask.Name, pipelineRun.Name, pipelineTask.Matrix.CountCombinations())
		for _, taskRunName := range rpt.TaskRunNames {
			if err := rpt.resolvePipelineRunTaskWithTaskRun(ctx, taskRunName, getTask, getTaskRun, pipelineTask); err != nil {
				return nil, err
			}
		}
	default:
		rpt.TaskRunName = GetTaskRunName(pipelineRun.Status.ChildReferences, pipelineTask.Name, pipelineRun.Name)
		if err := rpt.resolvePipelineRunTaskWithTaskRun(ctx, rpt.TaskRunName, getTask, getTaskRun, pipelineTask); err != nil {
			return nil, err
		}
	}
	return &rpt, nil
}

func (t *ResolvedPipelineTask) resolvePipelineRunTaskWithTaskRun(
	ctx context.Context,
	taskRunName string,
	getTask resources.GetTask,
	getTaskRun resources.GetTaskRun,
	pipelineTask v1beta1.PipelineTask,
) error {
	taskRun, err := getTaskRun(taskRunName)
	if err != nil {
		if !kerrors.IsNotFound(err) {
			return fmt.Errorf("error retrieving TaskRun %s: %w", taskRunName, err)
		}
	}
	if taskRun != nil {
		if t.PipelineTask.IsMatrixed() {
			t.TaskRuns = append(t.TaskRuns, taskRun)
		} else {
			t.TaskRun = taskRun
		}
	}

	return t.resolveTaskResources(ctx, getTask, pipelineTask, taskRun)
}

func (t *ResolvedPipelineTask) resolveTaskResources(
	ctx context.Context,
	getTask resources.GetTask,
	pipelineTask v1beta1.PipelineTask,
	taskRun *v1beta1.TaskRun,
) error {
	spec, taskName, kind, err := resolveTask(ctx, taskRun, getTask, pipelineTask)
	if err != nil {
		return err
	}

	spec.SetDefaults(ctx)
	rtr, err := resolvePipelineTaskResources(pipelineTask, &spec, taskName, kind)
	if err != nil {
		return fmt.Errorf("couldn't match referenced resources with declared resources: %w", err)
	}
	t.ResolvedTask = rtr

	return nil
}

func resolveTask(
	ctx context.Context,
	taskRun *v1beta1.TaskRun,
	getTask resources.GetTask,
	pipelineTask v1beta1.PipelineTask,
) (v1beta1.TaskSpec, string, v1beta1.TaskKind, error) {
	var (
		t        *v1beta1.Task
		err      error
		spec     v1beta1.TaskSpec
		taskName string
		kind     v1beta1.TaskKind
	)

	if pipelineTask.TaskRef != nil {
		// If the TaskRun has already a stored TaskSpec in its status, use it as source of truth
		if taskRun != nil && taskRun.Status.TaskSpec != nil {
			spec = *taskRun.Status.TaskSpec
			taskName = pipelineTask.TaskRef.Name
		} else {
			// Following minimum status principle (TEP-0100), no need to propagate the RefSource about PipelineTask up to PipelineRun status.
			// Instead, the child TaskRun's status will be the place recording the RefSource of individual task.
			t, _, err = getTask(ctx, pipelineTask.TaskRef.Name)
			switch {
			case errors.Is(err, remote.ErrRequestInProgress):
				return v1beta1.TaskSpec{}, "", "", err
			case errors.Is(err, trustedresources.ErrResourceVerificationFailed):
				return v1beta1.TaskSpec{}, "", "", err
			case err != nil:
				return v1beta1.TaskSpec{}, "", "", &TaskNotFoundError{
					Name: pipelineTask.TaskRef.Name,
					Msg:  err.Error(),
				}
			default:
				spec = t.TaskSpec()
				taskName = t.TaskMetadata().Name
			}
		}
		kind = pipelineTask.TaskRef.Kind
	} else {
		spec = pipelineTask.TaskSpec.TaskSpec
	}
	return spec, taskName, kind, err
}

// GetTaskRunName should return a unique name for a `TaskRun` if one has not already been defined, and the existing one otherwise.
func GetTaskRunName(childRefs []v1beta1.ChildStatusReference, ptName, prName string) string {
	for _, cr := range childRefs {
		if cr.Kind == pipeline.TaskRunControllerName && cr.PipelineTaskName == ptName {
			return cr.Name
		}
	}
	return kmeta.ChildName(prName, fmt.Sprintf("-%s", ptName))
}

// GetNamesOfTaskRuns should return unique names for `TaskRuns` if one has not already been defined, and the existing one otherwise.
func GetNamesOfTaskRuns(childRefs []v1beta1.ChildStatusReference, ptName, prName string, combinationCount int) []string {
	if taskRunNames := getTaskRunNamesFromChildRefs(childRefs, ptName); taskRunNames != nil {
		return taskRunNames
	}
	return getNewTaskRunNames(ptName, prName, combinationCount)
}

func getTaskRunNamesFromChildRefs(childRefs []v1beta1.ChildStatusReference, ptName string) []string {
	var taskRunNames []string
	for _, cr := range childRefs {
		if cr.Kind == pipeline.TaskRunControllerName && cr.PipelineTaskName == ptName {
			taskRunNames = append(taskRunNames, cr.Name)
		}
	}
	return taskRunNames
}

func getNewTaskRunNames(ptName, prName string, combinationCount int) []string {
	var taskRunNames []string
	for i := 0; i < combinationCount; i++ {
		taskRunName := kmeta.ChildName(prName, fmt.Sprintf("-%s-%d", ptName, i))
		taskRunNames = append(taskRunNames, taskRunName)
	}
	return taskRunNames
}

// getRunName should return a unique name for a `Run` if one has not already
// been defined, and the existing one otherwise.
func getRunName(childRefs []v1beta1.ChildStatusReference, ptName, prName string) string {
	for _, cr := range childRefs {
		if cr.PipelineTaskName == ptName {
			if cr.Kind == pipeline.CustomRunControllerName {
				return cr.Name
			}
		}
	}

	return kmeta.ChildName(prName, fmt.Sprintf("-%s", ptName))
}

// getNamesOfRuns should return a unique names for `RunObjects` if they have not already been defined,
// and the existing ones otherwise.
func getNamesOfRuns(childRefs []v1beta1.ChildStatusReference, ptName, prName string, combinationCount int) []string {
	if runNames := getRunNamesFromChildRefs(childRefs, ptName); runNames != nil {
		return runNames
	}
	return getNewTaskRunNames(ptName, prName, combinationCount)
}

func getRunNamesFromChildRefs(childRefs []v1beta1.ChildStatusReference, ptName string) []string {
	var runNames []string
	for _, cr := range childRefs {
		if cr.PipelineTaskName == ptName {
			if cr.Kind == pipeline.CustomRunControllerName {
				runNames = append(runNames, cr.Name)
			}
		}
	}
	return runNames
}

// resolvePipelineTaskResources matches PipelineResources referenced by pt inputs and outputs with the
// providedResources and returns an instance of ResolvedTask.
func resolvePipelineTaskResources(_ v1beta1.PipelineTask, ts *v1beta1.TaskSpec, taskName string, kind v1beta1.TaskKind) (*resources.ResolvedTask, error) {
	rtr := resources.ResolvedTask{
		TaskName: taskName,
		TaskSpec: ts,
		Kind:     kind,
	}
	return &rtr, nil
}

func (t *ResolvedPipelineTask) hasResultReferences() bool {
	var matrixParams v1beta1.Params
	if t.PipelineTask.IsMatrixed() {
		matrixParams = t.PipelineTask.Params
	}
	for _, param := range append(t.PipelineTask.Params, matrixParams...) {
		if ps, ok := v1beta1.GetVarSubstitutionExpressionsForParam(param); ok {
			if v1beta1.LooksLikeContainsResultRefs(ps) {
				return true
			}
		}
	}
	for _, we := range t.PipelineTask.WhenExpressions {
		if ps, ok := we.GetVarSubstitutionExpressions(); ok {
			if v1beta1.LooksLikeContainsResultRefs(ps) {
				return true
			}
		}
	}
	return false
}

func isCustomRunCancelledByPipelineRunTimeout(ro v1beta1.RunObject) bool {
	cr := ro.(*v1beta1.CustomRun)
	return cr.Spec.StatusMessage == v1beta1.CustomRunCancelledByPipelineTimeoutMsg
}
