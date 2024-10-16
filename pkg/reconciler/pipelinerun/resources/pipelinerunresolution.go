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
	"sort"
	"strings"

	"github.com/google/cel-go/cel"
	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	pipelineErrors "github.com/tektoncd/pipeline/pkg/apis/pipeline/errors"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/pkg/reconciler/taskrun/resources"
	"github.com/tektoncd/pipeline/pkg/remote"
	"github.com/tektoncd/pipeline/pkg/substitution"
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
	SkippingReason v1.SkippingReason
}

// TaskNotFoundError indicates that the resolution failed because a referenced Task couldn't be retrieved
type TaskNotFoundError struct {
	Name string
	Msg  string
}

func (e *TaskNotFoundError) Error() string {
	return fmt.Sprintf("Couldn't retrieve Task %q: %s", e.Name, e.Msg)
}

// ResolvedPipelineTask contains a PipelineTask and its associated TaskRun(s) or CustomRuns, if they exist.
type ResolvedPipelineTask struct {
	TaskRunNames []string
	TaskRuns     []*v1.TaskRun
	// If the PipelineTask is a Custom Task, CustomRunName and CustomRun will be set.
	CustomTask     bool
	CustomRunNames []string
	CustomRuns     []*v1beta1.CustomRun
	PipelineTask   *v1.PipelineTask
	ResolvedTask   *resources.ResolvedTask
	ResultsCache   map[string][]string
	// EvaluatedCEL is used to store the results of evaluated CEL expression
	EvaluatedCEL map[string]bool
}

// EvaluateCEL evaluate the CEL expressions, and store the evaluated results in EvaluatedCEL
func (t *ResolvedPipelineTask) EvaluateCEL() error {
	if t.PipelineTask != nil {
		// Each call to this function will reset this field to prevent additional CELs.
		t.EvaluatedCEL = make(map[string]bool)
		for _, we := range t.PipelineTask.When {
			if we.CEL == "" {
				continue
			}
			_, ok := t.EvaluatedCEL[we.CEL]
			if !ok {
				// Create a program environment configured with the standard library of CEL functions and macros
				// The error is omitted because not environment declarations are passed in.
				env, _ := cel.NewEnv()
				// Parse and Check the CEL to get the Abstract Syntax Tree
				ast, iss := env.Compile(we.CEL)
				if iss.Err() != nil {
					return iss.Err()
				}
				// Generate an evaluable instance of the Ast within the environment
				prg, err := env.Program(ast)
				if err != nil {
					return err
				}
				// Evaluate the CEL expression
				out, _, err := prg.Eval(map[string]interface{}{})
				if err != nil {
					return err
				}

				b, ok := out.Value().(bool)
				if ok {
					t.EvaluatedCEL[we.CEL] = b
				} else {
					return fmt.Errorf("The CEL expression %s is not evaluated to a boolean", we.CEL)
				}
			}
		}
	}
	return nil
}

// isDone returns true only if the task is skipped, succeeded or failed
func (t ResolvedPipelineTask) isDone(facts *PipelineRunFacts) bool {
	return t.Skip(facts).IsSkipped || t.isSuccessful() || t.isFailure() || t.isValidationFailed(facts.ValidationFailedTask)
}

// IsRunning returns true only if the task is neither succeeded, cancelled nor failed
func (t ResolvedPipelineTask) IsRunning() bool {
	if t.IsCustomTask() && len(t.CustomRuns) == 0 {
		return false
	}
	if !t.IsCustomTask() && len(t.TaskRuns) == 0 {
		return false
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
	if t.IsCustomTask() {
		if len(t.CustomRuns) == 0 {
			return false
		}
		for _, run := range t.CustomRuns {
			if !run.IsSuccessful() {
				return false
			}
		}
		return true
	}
	if len(t.TaskRuns) == 0 {
		return false
	}
	for _, taskRun := range t.TaskRuns {
		if !taskRun.IsSuccessful() {
			return false
		}
	}
	return true
}

// isFailure returns true only if the run has failed (if it has ConditionSucceeded = False).
// If the PipelineTask has a Matrix, isFailure returns true if any run has failed and all other runs are done.
func (t ResolvedPipelineTask) isFailure() bool {
	var isDone bool
	if t.IsCustomTask() {
		if len(t.CustomRuns) == 0 {
			return false
		}
		isDone = true
		for _, run := range t.CustomRuns {
			isDone = isDone && run.IsDone()
		}
		return t.haveAnyRunsFailed() && isDone
	}
	if len(t.TaskRuns) == 0 {
		return false
	}
	isDone = true
	for _, taskRun := range t.TaskRuns {
		isDone = isDone && taskRun.IsDone()
	}
	return t.haveAnyTaskRunsFailed() && isDone
}

// isValidationFailed return true if the task is failed at the validation step
func (t ResolvedPipelineTask) isValidationFailed(ftasks []*ResolvedPipelineTask) bool {
	for _, ftask := range ftasks {
		if ftask.ResolvedTask == t.ResolvedTask {
			return true
		}
	}
	return false
}

// isCancelledForTimeOut returns true only if the run is cancelled due to PipelineRun-controlled timeout
// If the PipelineTask has a Matrix, isCancelled returns true if any run is cancelled due to PipelineRun-controlled timeout and all other runs are done.
func (t ResolvedPipelineTask) isCancelledForTimeOut() bool {
	if t.IsCustomTask() {
		if len(t.CustomRuns) == 0 {
			return false
		}
		isDone := true
		atLeastOneCancelled := false
		for _, run := range t.CustomRuns {
			isDone = isDone && run.IsDone()
			c := run.GetStatusCondition().GetCondition(apis.ConditionSucceeded)
			runCancelled := c.IsFalse() &&
				c.Reason == v1beta1.CustomRunReasonCancelled.String() &&
				isCustomRunCancelledByPipelineRunTimeout(run)
			atLeastOneCancelled = atLeastOneCancelled || runCancelled
		}
		return atLeastOneCancelled && isDone
	}
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
			taskRun.Spec.StatusMessage == v1.TaskRunCancelledByPipelineTimeoutMsg
		atLeastOneCancelled = atLeastOneCancelled || taskRunCancelled
	}
	return atLeastOneCancelled && isDone
}

// isCancelled returns true only if the run is cancelled
// If the PipelineTask has a Matrix, isCancelled returns true if any run is cancelled and all other runs are done.
func (t ResolvedPipelineTask) isCancelled() bool {
	if t.IsCustomTask() {
		if len(t.CustomRuns) == 0 {
			return false
		}
		isDone := true
		atLeastOneCancelled := false
		for _, run := range t.CustomRuns {
			isDone = isDone && run.IsDone()
			c := run.GetStatusCondition().GetCondition(apis.ConditionSucceeded)
			runCancelled := c.IsFalse() && c.Reason == v1beta1.CustomRunReasonCancelled.String()
			atLeastOneCancelled = atLeastOneCancelled || runCancelled
		}
		return atLeastOneCancelled && isDone
	}
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
}

// isScheduled returns true when the PipelineRunTask itself has any TaskRuns/CustomRuns
// or a singular TaskRun/CustomRun associated.
func (t ResolvedPipelineTask) isScheduled() bool {
	if t.IsCustomTask() {
		return len(t.CustomRuns) > 0
	}
	return len(t.TaskRuns) > 0
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
	for _, taskRun := range t.TaskRuns {
		if taskRun.IsFailure() {
			return true
		}
	}
	return false
}

// haveAnyCustomRunsFailed returns true when a CustomRun has succeeded condition with status set to false
func (t ResolvedPipelineTask) haveAnyCustomRunsFailed() bool {
	for _, customRun := range t.CustomRuns {
		if customRun.IsFailure() {
			return true
		}
	}
	return false
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
	var skippingReason v1.SkippingReason

	switch {
	case facts.isFinalTask(t.PipelineTask.Name) || t.isScheduled():
		skippingReason = v1.None
	case facts.IsStopping():
		skippingReason = v1.StoppingSkip
	case facts.IsGracefullyCancelled():
		skippingReason = v1.GracefullyCancelledSkip
	case facts.IsGracefullyStopped():
		skippingReason = v1.GracefullyStoppedSkip
	case t.skipBecauseParentTaskWasSkipped(facts):
		skippingReason = v1.ParentTasksSkip
	case t.skipBecauseResultReferencesAreMissing(facts):
		skippingReason = v1.MissingResultsSkip
	case t.skipBecauseWhenExpressionsEvaluatedToFalse(facts):
		skippingReason = v1.WhenExpressionsSkip
	case t.skipBecausePipelineRunPipelineTimeoutReached(facts):
		skippingReason = v1.PipelineTimedOutSkip
	case t.skipBecausePipelineRunTasksTimeoutReached(facts):
		skippingReason = v1.TasksTimedOutSkip
	case t.skipBecauseEmptyArrayInMatrixParams():
		skippingReason = v1.EmptyArrayInMatrixParams
	default:
		skippingReason = v1.None
	}

	return TaskSkipStatus{
		IsSkipped:      skippingReason != v1.None,
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
		if !t.PipelineTask.When.AllowsExecution(t.EvaluatedCEL) {
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
			if parentSkipStatus.SkippingReason == v1.WhenExpressionsSkip {
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
			if err != nil &&
				(t.PipelineTask.OnError == v1.PipelineTaskContinue ||
					(t.IsFinalTask(facts) || rpt.Skip(facts).SkippingReason == v1.WhenExpressionsSkip)) {
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
			if ps.Value.Type == v1.ParamTypeArray && len(ps.Value.ArrayVal) == 0 {
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
	var skippingReason v1.SkippingReason

	switch {
	case t.isScheduled():
		skippingReason = v1.None
	case facts.checkDAGTasksDone() && facts.isFinalTask(t.PipelineTask.Name):
		switch {
		case t.skipBecauseResultReferencesAreMissing(facts):
			skippingReason = v1.MissingResultsSkip
		case t.skipBecauseWhenExpressionsEvaluatedToFalse(facts):
			skippingReason = v1.WhenExpressionsSkip
		case t.skipBecausePipelineRunPipelineTimeoutReached(facts):
			skippingReason = v1.PipelineTimedOutSkip
		case t.skipBecausePipelineRunFinallyTimeoutReached(facts):
			skippingReason = v1.FinallyTimedOutSkip
		case t.skipBecauseEmptyArrayInMatrixParams():
			skippingReason = v1.EmptyArrayInMatrixParams
		default:
			skippingReason = v1.None
		}
	default:
		skippingReason = v1.None
	}

	return TaskSkipStatus{
		IsSkipped:      skippingReason != v1.None,
		SkippingReason: skippingReason,
	}
}

// GetRun is a function that will retrieve a CustomRun by name.
type GetRun func(name string) (*v1beta1.CustomRun, error)

// ValidateWorkspaceBindings validates that the Workspaces expected by a Pipeline are provided by a PipelineRun.
func ValidateWorkspaceBindings(p *v1.PipelineSpec, pr *v1.PipelineRun) error {
	pipelineRunWorkspaces := make(map[string]v1.WorkspaceBinding)
	for _, binding := range pr.Spec.Workspaces {
		pipelineRunWorkspaces[binding.Name] = binding
	}

	for _, ws := range p.Workspaces {
		if ws.Optional {
			continue
		}
		if _, ok := pipelineRunWorkspaces[ws.Name]; !ok {
			return pipelineErrors.WrapUserError(fmt.Errorf("pipeline requires workspace with name %q be provided by pipelinerun", ws.Name))
		}
	}
	return nil
}

// ValidateTaskRunSpecs that the TaskRunSpecs defined by a PipelineRun are correct.
func ValidateTaskRunSpecs(p *v1.PipelineSpec, pr *v1.PipelineRun) error {
	pipelineTasks := make(map[string]string)
	for _, task := range p.Tasks {
		pipelineTasks[task.Name] = task.Name
	}

	for _, task := range p.Finally {
		pipelineTasks[task.Name] = task.Name
	}

	for _, taskrunSpec := range pr.Spec.TaskRunSpecs {
		if _, ok := pipelineTasks[taskrunSpec.PipelineTaskName]; !ok {
			return pipelineErrors.WrapUserError(fmt.Errorf("pipelineRun's taskrunSpecs defined wrong taskName: %q, does not exist in Pipeline", taskrunSpec.PipelineTaskName))
		}
	}
	return nil
}

// ResolvePipelineTask returns a new ResolvedPipelineTask representing any TaskRuns or CustomRuns
// associated with this Pipeline Task, if they exist.
//
// If the Pipeline Task is a Task, it retrieves any TaskRuns, plus the Task spec, and updates the ResolvedPipelineTask
// with this information. It also sets the ResolvedPipelineTask's TaskRunName(s) with the names of TaskRuns
// that should be or already have been created.
//
// If the Pipeline Task is a Custom Task, it retrieves any CustomRuns and updates the ResolvedPipelineTask with this information.
// It also sets the ResolvedPipelineTask's RunName(s) with the names of CustomRuns that should be or already have been created.
func ResolvePipelineTask(
	ctx context.Context,
	pipelineRun v1.PipelineRun,
	getTask resources.GetTask,
	getTaskRun resources.GetTaskRun,
	getRun GetRun,
	pipelineTask v1.PipelineTask,
	pst PipelineRunState,
) (*ResolvedPipelineTask, error) {
	rpt := ResolvedPipelineTask{
		PipelineTask: &pipelineTask,
	}
	rpt.CustomTask = rpt.PipelineTask.TaskRef.IsCustomTask() || rpt.PipelineTask.TaskSpec.IsCustomTask()
	numCombinations := 1
	// We want to resolve all of the result references and ignore any errors at this point since there could be
	// instances where result references are missing here, but will be later skipped and resolved in
	// skipBecauseResultReferencesAreMissing. The final validation is handled in CheckMissingResultReferences.
	resolvedResultRefs, _, _ := ResolveResultRefs(pst, PipelineRunState{&rpt})
	if err := validateArrayResultsIndex(resolvedResultRefs); err != nil {
		return nil, err
	}

	ApplyTaskResults(PipelineRunState{&rpt}, resolvedResultRefs)

	if rpt.PipelineTask.IsMatrixed() {
		numCombinations = rpt.PipelineTask.Matrix.CountCombinations()
	}
	if rpt.IsCustomTask() {
		rpt.CustomRunNames = getNamesOfCustomRuns(pipelineRun.Status.ChildReferences, pipelineTask.Name, pipelineRun.Name, numCombinations)
		for _, runName := range rpt.CustomRunNames {
			run, err := getRun(runName)
			if err != nil && !kerrors.IsNotFound(err) {
				return nil, fmt.Errorf("error retrieving CustomRun %s: %w", runName, err)
			}
			if run != nil {
				rpt.CustomRuns = append(rpt.CustomRuns, run)
			}
		}
	} else {
		rpt.TaskRunNames = GetNamesOfTaskRuns(pipelineRun.Status.ChildReferences, pipelineTask.Name, pipelineRun.Name, numCombinations)
		for _, taskRunName := range rpt.TaskRunNames {
			if err := rpt.setTaskRunsAndResolvedTask(ctx, taskRunName, getTask, getTaskRun, pipelineTask); err != nil {
				return nil, err
			}
		}
	}
	return &rpt, nil
}

// setTaskRunsAndResolvedTask fetches the named TaskRun using the input function getTaskRun,
// and the resolved Task spec of the Pipeline Task using the input function getTask.
// It updates the ResolvedPipelineTask with the ResolvedTask and a pointer to the fetched TaskRun.
func (t *ResolvedPipelineTask) setTaskRunsAndResolvedTask(
	ctx context.Context,
	taskRunName string,
	getTask resources.GetTask,
	getTaskRun resources.GetTaskRun,
	pipelineTask v1.PipelineTask,
) error {
	taskRun, err := getTaskRun(taskRunName)
	if err != nil {
		if !kerrors.IsNotFound(err) {
			return fmt.Errorf("error retrieving TaskRun %s: %w", taskRunName, err)
		}
	}
	if taskRun != nil {
		t.TaskRuns = append(t.TaskRuns, taskRun)
	}

	rt, err := resolveTask(ctx, taskRun, getTask, pipelineTask)
	if err != nil {
		return err
	}
	t.ResolvedTask = rt
	return nil
}

// resolveTask fetches the Task spec for the PipelineTask and sets its default values.
// It returns a ResolvedTask with the defaulted spec, name, and kind (namespaced Task or Cluster Task) of the Task.
// Returns an error if the Task could not be found because resolution was in progress or any other reason.
func resolveTask(
	ctx context.Context,
	taskRun *v1.TaskRun,
	getTask resources.GetTask,
	pipelineTask v1.PipelineTask,
) (*resources.ResolvedTask, error) {
	rt := &resources.ResolvedTask{}
	switch {
	case pipelineTask.TaskRef != nil:
		// If the TaskRun has already a stored TaskSpec in its status, use it as source of truth
		if taskRun != nil && taskRun.Status.TaskSpec != nil {
			rt.TaskSpec = taskRun.Status.TaskSpec
			rt.TaskName = pipelineTask.TaskRef.Name
		} else {
			// Following minimum status principle (TEP-0100), no need to propagate the RefSource about PipelineTask up to PipelineRun status.
			// Instead, the child TaskRun's status will be the place recording the RefSource of individual task.
			t, _, vr, err := getTask(ctx, pipelineTask.TaskRef.Name)
			switch {
			case errors.Is(err, remote.ErrRequestInProgress):
				return rt, err
			case err != nil:
				return rt, &TaskNotFoundError{
					Name: pipelineTask.TaskRef.Name,
					Msg:  err.Error(),
				}
			default:
				spec := t.Spec
				rt.TaskSpec = &spec
				rt.TaskName = t.Name
				rt.VerificationResult = vr
			}
		}
		rt.Kind = pipelineTask.TaskRef.Kind
	case pipelineTask.TaskSpec != nil:
		rt.TaskSpec = &pipelineTask.TaskSpec.TaskSpec
	default:
		// If the alpha feature is enabled, and the user has configured pipelineSpec or pipelineRef, it will enter here.
		// Currently, the controller is not yet adapted, and to avoid a panic, an error message is provided here.
		// TODO: Adjust the logic here once the feature is supported in the future.
		return nil, fmt.Errorf("Currently, Task %q does not support PipelineRef or PipelineSpec, please use TaskRef or TaskSpec instead", pipelineTask.Name)
	}
	rt.TaskSpec.SetDefaults(ctx)
	return rt, nil
}

// GetTaskRunName should return a unique name for a `TaskRun` if one has not already been defined, and the existing one otherwise.
func GetTaskRunName(childRefs []v1.ChildStatusReference, ptName, prName string) string {
	for _, cr := range childRefs {
		if cr.Kind == pipeline.TaskRunControllerName && cr.PipelineTaskName == ptName {
			return cr.Name
		}
	}
	return kmeta.ChildName(prName, "-"+ptName)
}

// GetNamesOfTaskRuns should return unique names for `TaskRuns` if one has not already been defined, and the existing one otherwise.
func GetNamesOfTaskRuns(childRefs []v1.ChildStatusReference, ptName, prName string, numberOfTaskRuns int) []string {
	if taskRunNames := getTaskRunNamesFromChildRefs(childRefs, ptName); taskRunNames != nil {
		return taskRunNames
	}
	return getNewRunNames(ptName, prName, numberOfTaskRuns)
}

// getTaskRunNamesFromChildRefs returns the names of TaskRuns defined in childRefs that are associated with the named Pipeline Task.
func getTaskRunNamesFromChildRefs(childRefs []v1.ChildStatusReference, ptName string) []string {
	var taskRunNames []string
	for _, cr := range childRefs {
		if cr.Kind == pipeline.TaskRunControllerName && cr.PipelineTaskName == ptName {
			taskRunNames = append(taskRunNames, cr.Name)
		}
	}
	return taskRunNames
}

func getNewRunNames(ptName, prName string, numberOfRuns int) []string {
	var taskRunNames []string
	// If it is a singular TaskRun/CustomRun, we only append the ptName
	if numberOfRuns == 1 {
		taskRunName := kmeta.ChildName(prName, "-"+ptName)
		return append(taskRunNames, taskRunName)
	}
	// For a matrix we append i to then end of the fanned out TaskRuns "matrixed-pr-taskrun-0"
	for i := 0; i < numberOfRuns; i++ {
		taskRunName := kmeta.ChildName(prName, fmt.Sprintf("-%s-%d", ptName, i))
		// check if the taskRun name ends with a matrix instance count
		if !strings.HasSuffix(taskRunName, fmt.Sprintf("-%d", i)) {
			taskRunName = kmeta.ChildName(prName, "-"+ptName)
			// kmeta.ChildName limits the size of a name to max of 63 characters based on k8s guidelines
			// truncate the name such that "-<matrix-id>" can be appended to the taskRun name
			longest := 63 - len(fmt.Sprintf("-%d", numberOfRuns))
			taskRunName = taskRunName[0:longest]
			taskRunName = fmt.Sprintf("%s-%d", taskRunName, i)
		}
		taskRunNames = append(taskRunNames, taskRunName)
	}
	return taskRunNames
}

// getCustomRunName should return a unique name for a `Run` if one has not already
// been defined, and the existing one otherwise.
func getCustomRunName(childRefs []v1.ChildStatusReference, ptName, prName string) string {
	for _, cr := range childRefs {
		if cr.PipelineTaskName == ptName {
			if cr.Kind == pipeline.CustomRunControllerName {
				return cr.Name
			}
		}
	}

	return kmeta.ChildName(prName, "-"+ptName)
}

// getNamesOfCustomRuns should return a unique names for `CustomRuns` if they have not already been defined,
// and the existing ones otherwise.
func getNamesOfCustomRuns(childRefs []v1.ChildStatusReference, ptName, prName string, numberOfRuns int) []string {
	if customRunNames := getRunNamesFromChildRefs(childRefs, ptName); customRunNames != nil {
		return customRunNames
	}
	return getNewRunNames(ptName, prName, numberOfRuns)
}

// getRunNamesFromChildRefs returns the names of CustomRuns defined in childRefs that are associated with the named Pipeline Task.
func getRunNamesFromChildRefs(childRefs []v1.ChildStatusReference, ptName string) []string {
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

func (t *ResolvedPipelineTask) hasResultReferences() bool {
	var matrixParams v1.Params
	if t.PipelineTask.IsMatrixed() {
		matrixParams = t.PipelineTask.Params
	}
	for _, param := range append(t.PipelineTask.Params, matrixParams...) {
		if ps, ok := param.GetVarSubstitutionExpressions(); ok {
			if v1.LooksLikeContainsResultRefs(ps) {
				return true
			}
		}
	}
	for _, we := range t.PipelineTask.When {
		if ps, ok := we.GetVarSubstitutionExpressions(); ok {
			if v1.LooksLikeContainsResultRefs(ps) {
				return true
			}
		}
	}
	return false
}

func isCustomRunCancelledByPipelineRunTimeout(cr *v1beta1.CustomRun) bool {
	return cr.Spec.StatusMessage == v1beta1.CustomRunCancelledByPipelineTimeoutMsg
}

// CheckMissingResultReferences returns an error if it is missing any result references.
// Missing result references can occur if task fails to produce a result but has
// OnError: continue (ie TestMissingResultWhenStepErrorIsIgnored)
func CheckMissingResultReferences(pipelineRunState PipelineRunState, target *ResolvedPipelineTask) error {
	for _, resultRef := range v1.PipelineTaskResultRefs(target.PipelineTask) {
		referencedPipelineTask, ok := pipelineRunState.ToMap()[resultRef.PipelineTask]
		if !ok {
			return fmt.Errorf("Result reference error: Could not find ref \"%s\" in internal pipelineRunState", resultRef.PipelineTask)
		}
		if referencedPipelineTask.IsCustomTask() {
			if len(referencedPipelineTask.CustomRuns) == 0 {
				return fmt.Errorf("Result reference error: Internal result ref \"%s\" has zero-length CustomRuns", resultRef.PipelineTask)
			}
			customRun := referencedPipelineTask.CustomRuns[0]
			_, err := findRunResultForParam(customRun, resultRef)
			if err != nil {
				return err
			}
		} else {
			if len(referencedPipelineTask.TaskRuns) == 0 {
				return fmt.Errorf("Result reference error: Internal result ref \"%s\" has zero-length TaskRuns", resultRef.PipelineTask)
			}
			taskRun := referencedPipelineTask.TaskRuns[0]
			_, err := findTaskResultForParam(taskRun, resultRef)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// createResultsCacheMatrixedTaskRuns creates a cache of results that have been fanned out from a
// referenced matrixed PipelintTask so that you can easily access these results in subsequent Pipeline Tasks
func createResultsCacheMatrixedTaskRuns(rpt *ResolvedPipelineTask) (resultsCache map[string][]string) {
	if len(rpt.ResultsCache) == 0 {
		resultsCache = make(map[string][]string)
	}
	// Sort the taskRuns by name to ensure the order is deterministic
	sort.Slice(rpt.TaskRuns, func(i, j int) bool {
		return rpt.TaskRuns[i].Name < rpt.TaskRuns[j].Name
	})
	for _, taskRun := range rpt.TaskRuns {
		results := taskRun.Status.Results
		for _, result := range results {
			resultsCache[result.Name] = append(resultsCache[result.Name], result.Value.StringVal)
		}
	}
	return resultsCache
}

// ValidateParamEnumSubset finds the referenced pipeline-level params in the resolved pipelineTask.
// It then validates if the referenced pipeline-level param enums are subsets of the resolved pipelineTask-level param enums
func ValidateParamEnumSubset(pipelineTaskParams []v1.Param, pipelineParamSpecs []v1.ParamSpec, rt *resources.ResolvedTask) error {
	for _, p := range pipelineTaskParams {
		// calculate referenced param enums
		res, present, errString := substitution.ExtractVariablesFromString(p.Value.StringVal, "params")
		if errString != "" {
			return fmt.Errorf("unexpected error in ExtractVariablesFromString: %s", errString)
		}

		// if multiple params are extracted, that means the task-level param is a compounded value, skip subset validation
		if !present || len(res) > 1 {
			continue
		}

		// resolve pipeline-level and pipelineTask-level enums
		paramName := substitution.TrimArrayIndex(res[0])
		pipelineParam := getParamFromName(paramName, pipelineParamSpecs)
		resolvedTaskParam := getParamFromName(p.Name, rt.TaskSpec.Params)

		// param enum is only supported for string param type,
		// we only validate the enum subset requirement for string typed param.
		// If there is no task-level enum (allowing any value), any pipeline-level enum is allowed
		if pipelineParam.Type != v1.ParamTypeString || len(resolvedTaskParam.Enum) == 0 {
			return nil
		}

		// if pipelin-level enum is empty (allowing any value) but task-level enum is not, it is not a "subset"
		if len(pipelineParam.Enum) == 0 && len(resolvedTaskParam.Enum) > 0 {
			return fmt.Errorf("pipeline param \"%s\" has no enum, but referenced in \"%s\" task has enums: %v", pipelineParam.Name, rt.TaskName, resolvedTaskParam.Enum)
		}

		// validate if pipeline-level enum is a subset of pipelineTask-level enum
		if isValid := isSubset(pipelineParam.Enum, resolvedTaskParam.Enum); !isValid {
			return fmt.Errorf("pipeline param \"%s\" enum: %v is not a subset of the referenced in \"%s\" task param enum: %v", pipelineParam.Name, pipelineParam.Enum, rt.TaskName, resolvedTaskParam.Enum)
		}
	}

	return nil
}

func isSubset(pipelineEnum, taskEnum []string) bool {
	pipelineEnumMap := make(map[string]bool)
	TaskEnumMap := make(map[string]bool)
	for _, e := range pipelineEnum {
		pipelineEnumMap[e] = true
	}
	for _, e := range taskEnum {
		TaskEnumMap[e] = true
	}

	for e := range pipelineEnumMap {
		if !TaskEnumMap[e] {
			return false
		}
	}

	return true
}

func getParamFromName(name string, pss v1.ParamSpecs) v1.ParamSpec {
	for _, ps := range pss {
		if ps.Name == name {
			return ps
		}
	}
	return v1.ParamSpec{}
}
