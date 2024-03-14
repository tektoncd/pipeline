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

	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	clientset "github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	resolutionutil "github.com/tektoncd/pipeline/pkg/internal/resolution"
	remoteresource "github.com/tektoncd/pipeline/pkg/resolution/resource"
	"github.com/tektoncd/pipeline/pkg/trustedresources"
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
type GetStepAction func(context.Context, string) (*v1alpha1.StepAction, *v1.RefSource, error)

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

// GetStepActionsData extracts the StepActions and merges them with the inlined Step specification.
func GetStepActionsData(ctx context.Context, taskSpec v1.TaskSpec, taskRun *v1.TaskRun, tekton clientset.Interface, k8s kubernetes.Interface, requester remoteresource.Requester) ([]v1.Step, error) {
	steps := []v1.Step{}
	for _, step := range taskSpec.Steps {
		s := step.DeepCopy()
		if step.Ref != nil {
			getStepAction := GetStepActionFunc(tekton, k8s, requester, taskRun, s)
			stepAction, _, err := getStepAction(ctx, s.Ref.Name)
			if err != nil {
				return nil, err
			}
			stepActionSpec := stepAction.StepActionSpec()
			stepActionSpec.SetDefaults(ctx)

			stepFromStepAction := stepActionSpec.ToStep()
			if err := validateStepHasStepActionParameters(s.Params, stepActionSpec.Params); err != nil {
				return nil, err
			}
			stepFromStepAction = applyStepActionParameters(stepFromStepAction, &taskSpec, taskRun, s.Params, stepActionSpec.Params)

			s.Image = stepFromStepAction.Image
			s.SecurityContext = stepFromStepAction.SecurityContext
			if len(stepFromStepAction.Command) > 0 {
				s.Command = stepFromStepAction.Command
			}
			if len(stepFromStepAction.Args) > 0 {
				s.Args = stepFromStepAction.Args
			}
			if stepFromStepAction.Script != "" {
				s.Script = stepFromStepAction.Script
			}
			s.WorkingDir = stepFromStepAction.WorkingDir
			if stepFromStepAction.Env != nil {
				s.Env = stepFromStepAction.Env
			}
			if len(stepFromStepAction.VolumeMounts) > 0 {
				s.VolumeMounts = stepFromStepAction.VolumeMounts
			}
			if len(stepFromStepAction.Results) > 0 {
				s.Results = stepFromStepAction.Results
			}
			s.Params = nil
			s.Ref = nil
			steps = append(steps, *s)
		} else {
			steps = append(steps, step)
		}
	}
	return steps, nil
}
