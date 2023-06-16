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
	resolutionutil "github.com/tektoncd/pipeline/pkg/internal/resolution"
	"github.com/tektoncd/pipeline/pkg/trustedresources"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
