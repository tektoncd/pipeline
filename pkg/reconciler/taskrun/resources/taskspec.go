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
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	clientset "github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	resolutionutil "github.com/tektoncd/pipeline/pkg/internal/resolution"
	"github.com/tektoncd/pipeline/pkg/pod"
	remoteresource "github.com/tektoncd/pipeline/pkg/remoteresolution/resource"
	"github.com/tektoncd/pipeline/pkg/trustedresources"
	"golang.org/x/sync/errgroup"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// ResolvedTask contains the data that is needed to execute
// the TaskRun.
type ResolvedTask struct {
	TaskName string
	Kind     v1.TaskKind
	TaskSpec *v1.TaskSpec
	// VerificationResult is the result from trusted resources if the feature is enabled.
	VerificationResult *trustedresources.VerificationResult
}

// GetStepAction is a function used to retrieve StepActions.
type GetStepAction func(context.Context, string) (*v1beta1.StepAction, *v1.RefSource, error)

// GetTask is a function used to retrieve Tasks.
// VerificationResult is the result from trusted resources if the feature is enabled.
type GetTask func(context.Context, string) (*v1.Task, *v1.RefSource, *trustedresources.VerificationResult, error)

// GetTaskRun is a function used to retrieve TaskRuns
type GetTaskRun func(string) (*v1.TaskRun, error)

// GetTaskData will retrieve the Task metadata and Spec associated with the
// provided TaskRun. This can come from a reference Task or from the TaskRun's
// metadata and embedded TaskSpec.
func GetTaskData(ctx context.Context, taskRun *v1.TaskRun, getTask GetTask) (*resolutionutil.ResolvedObjectMeta, *v1.TaskSpec, error) {
	taskMeta := metav1.ObjectMeta{}
	taskSpec := v1.TaskSpec{}
	var refSource *v1.RefSource
	var verificationResult *trustedresources.VerificationResult
	switch {
	case taskRun.Spec.TaskRef != nil && taskRun.Spec.TaskRef.Name != "":
		// Get related task for taskrun
		t, source, vr, err := getTask(ctx, taskRun.Spec.TaskRef.Name)
		if err != nil {
			return nil, nil, fmt.Errorf("error when listing tasks for taskRun %s: %w", taskRun.Name, err)
		}
		taskMeta = t.ObjectMeta
		taskSpec = t.Spec
		refSource = source
		verificationResult = vr
	case taskRun.Spec.TaskSpec != nil:
		taskMeta = taskRun.ObjectMeta
		taskSpec = *taskRun.Spec.TaskSpec
		// TODO: if we want to set RefSource for embedded taskspec, set it here.
		// https://github.com/tektoncd/pipeline/issues/5522
	case taskRun.Spec.TaskRef != nil && taskRun.Spec.TaskRef.Resolver != "":
		task, source, vr, err := getTask(ctx, taskRun.Name)
		switch {
		case err != nil:
			return nil, nil, err
		case task == nil:
			return nil, nil, errors.New("resolution of remote resource completed successfully but no task was returned")
		default:
			taskMeta = task.ObjectMeta
			taskSpec = task.Spec
		}
		refSource = source
		verificationResult = vr
	default:
		return nil, nil, fmt.Errorf("taskRun %s not providing TaskRef or TaskSpec", taskRun.Name)
	}

	taskSpec.SetDefaults(ctx)
	return &resolutionutil.ResolvedObjectMeta{
		ObjectMeta:         &taskMeta,
		RefSource:          refSource,
		VerificationResult: verificationResult,
	}, &taskSpec, nil
}

// stepRefResolution holds the outcome of resolving a step referencing a StepAction.
type stepRefResolution struct {
	resolvedStep *v1.Step
	source       *v1.RefSource
}

// hasStepRefs provides a fast check to see if any steps in a TaskSpec contain a reference to a StepAction.
func hasStepRefs(taskSpec *v1.TaskSpec) bool {
	for _, step := range taskSpec.Steps {
		if step.Ref != nil {
			return true
		}
	}
	return false
}

// resolveStepRef resolves a step referecing a StepAction by fetching the remote StepAction, merging it with the Step's specification, and returning the resolved step.
func resolveStepRef(ctx context.Context, taskSpec v1.TaskSpec, taskRun *v1.TaskRun, tekton clientset.Interface, k8s kubernetes.Interface, requester remoteresource.Requester, step *v1.Step) (*v1.Step, *v1.RefSource, error) {
	resolvedStep := step.DeepCopy()

	getStepAction := GetStepActionFunc(tekton, k8s, requester, taskRun, taskSpec, resolvedStep)
	stepAction, source, err := getStepAction(ctx, resolvedStep.Ref.Name)
	if err != nil {
		return nil, nil, err
	}

	stepActionSpec := stepAction.StepActionSpec()
	stepActionSpec.SetDefaults(ctx)

	stepFromStepAction := stepActionSpec.ToStep()
	if err := validateStepHasStepActionParameters(resolvedStep.Params, stepActionSpec.Params); err != nil {
		return nil, nil, err
	}

	stepFromStepAction, err = applyStepActionParameters(stepFromStepAction, &taskSpec, taskRun, resolvedStep.Params, stepActionSpec.Params)
	if err != nil {
		return nil, nil, err
	}

	// Merge fields from the resolved StepAction into the step
	resolvedStep.Image = stepFromStepAction.Image
	resolvedStep.SecurityContext = stepFromStepAction.SecurityContext
	if len(stepFromStepAction.Command) > 0 {
		resolvedStep.Command = stepFromStepAction.Command
	}
	if len(stepFromStepAction.Args) > 0 {
		resolvedStep.Args = stepFromStepAction.Args
	}
	if stepFromStepAction.Script != "" {
		resolvedStep.Script = stepFromStepAction.Script
	}
	resolvedStep.WorkingDir = stepFromStepAction.WorkingDir
	if stepFromStepAction.Env != nil {
		resolvedStep.Env = stepFromStepAction.Env
	}
	if len(stepFromStepAction.VolumeMounts) > 0 {
		resolvedStep.VolumeMounts = stepFromStepAction.VolumeMounts
	}
	if len(stepFromStepAction.Results) > 0 {
		resolvedStep.Results = stepFromStepAction.Results
	}

	// Finalize by clearing Ref and Params, as they have been resolved
	resolvedStep.Ref = nil
	resolvedStep.Params = nil

	return resolvedStep, source, nil
}

// updateTaskRunProvenance update the TaskRun's status with source provenance information for a given step
func updateTaskRunProvenance(taskRun *v1.TaskRun, stepName string, stepIndex int, source *v1.RefSource, stepStatusIndex map[string]int) {
	var provenance *v1.Provenance

	// The StepState already exists. Update it in place
	if index, found := stepStatusIndex[stepName]; found {
		if taskRun.Status.Steps[index].Provenance == nil {
			taskRun.Status.Steps[index].Provenance = &v1.Provenance{}
		}
		taskRun.Status.Steps[index].Provenance.RefSource = source
		return
	}

	provenance = &v1.Provenance{RefSource: source}

	// No existing StepState found. Create and append a new one
	newState := v1.StepState{
		Name:       pod.TrimStepPrefix(pod.StepName(stepName, stepIndex)),
		Provenance: provenance,
	}
	taskRun.Status.Steps = append(taskRun.Status.Steps, newState)
}

// GetStepActionsData extracts the StepActions and merges them with the inlined Step specification.
func GetStepActionsData(ctx context.Context, taskSpec v1.TaskSpec, taskRun *v1.TaskRun, tekton clientset.Interface, k8s kubernetes.Interface, requester remoteresource.Requester) ([]v1.Step, error) {
	steps := make([]v1.Step, len(taskSpec.Steps))

	// Init step states and known step states indexes lookup map
	if taskRun.Status.Steps == nil {
		taskRun.Status.Steps = []v1.StepState{}
	}
	stepStatusIndex := make(map[string]int, len(taskRun.Status.Steps))
	for i, stepState := range taskRun.Status.Steps {
		stepStatusIndex[stepState.Name] = i
	}

	// If there are no step-ref to resolve, return immediately with nil provenance
	if !hasStepRefs(&taskSpec) {
		for i, step := range taskSpec.Steps {
			steps[i] = step
			updateTaskRunProvenance(taskRun, step.Name, i, nil, stepStatusIndex) // create StepState with nil provenance
		}
		return steps, nil
	}

	// Phase 1: Concurrently resolve all StepActions
	stepRefConcurrencyLimit := config.FromContextOrDefaults(ctx).Defaults.DefaultStepRefConcurrencyLimit
	g, ctx := errgroup.WithContext(ctx)
	// This limit prevents overwhelming the API server or remote git servers
	g.SetLimit(stepRefConcurrencyLimit)

	stepRefResolutions := make([]*stepRefResolution, len(taskSpec.Steps))
	for i, step := range taskSpec.Steps {
		if step.Ref == nil { // Only process steps with a Ref
			continue
		}

		g.Go(func() error {
			resolvedStep, source, err := resolveStepRef(ctx, taskSpec, taskRun, tekton, k8s, requester, &step)
			if err != nil {
				return fmt.Errorf("failed to resolve step ref for step %q (index %d): %w", step.Name, i, err)
			}
			stepRefResolutions[i] = &stepRefResolution{resolvedStep: resolvedStep, source: source}
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}

	// Phase 2: Sequentially merge results into the final step list and update status
	for i, step := range taskSpec.Steps {
		if step.Ref == nil {
			steps[i] = step
			updateTaskRunProvenance(taskRun, step.Name, i, nil, stepStatusIndex) // create StepState for inline step with nil provenance
			continue
		}

		stepRefResolution := stepRefResolutions[i]
		steps[i] = *stepRefResolution.resolvedStep
		updateTaskRunProvenance(taskRun, stepRefResolution.resolvedStep.Name, i, stepRefResolution.source, stepStatusIndex)
	}

	return steps, nil
}
