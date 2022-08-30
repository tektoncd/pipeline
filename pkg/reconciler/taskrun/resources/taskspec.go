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

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	resolutionutil "github.com/tektoncd/pipeline/pkg/internal/resolution"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GetTask is a function used to retrieve Tasks.
type GetTask func(context.Context, string) (v1beta1.TaskObject, *v1beta1.ConfigSource, error)

// GetTaskRun is a function used to retrieve TaskRuns
type GetTaskRun func(string) (*v1beta1.TaskRun, error)

// GetClusterTask is a function that will retrieve the Task from name and namespace.
type GetClusterTask func(name string) (v1beta1.TaskObject, error)

// GetTaskData will retrieve the Task metadata and Spec associated with the
// provided TaskRun. This can come from a reference Task or from the TaskRun's
// metadata and embedded TaskSpec.
func GetTaskData(ctx context.Context, taskRun *v1beta1.TaskRun, getTask GetTask) (*resolutionutil.ResolvedObjectMeta, *v1beta1.TaskSpec, error) {
	taskMeta := metav1.ObjectMeta{}
	var configSource *v1beta1.ConfigSource
	taskSpec := v1beta1.TaskSpec{}
	switch {
	case taskRun.Spec.TaskRef != nil && taskRun.Spec.TaskRef.Name != "":
		// Get related task for taskrun
		t, source, err := getTask(ctx, taskRun.Spec.TaskRef.Name)
		if err != nil {
			return nil, nil, fmt.Errorf("error when listing tasks for taskRun %s: %w", taskRun.Name, err)
		}
		taskMeta = t.TaskMetadata()
		taskSpec = t.TaskSpec()
		configSource = source
	case taskRun.Spec.TaskSpec != nil:
		taskMeta = taskRun.ObjectMeta
		taskSpec = *taskRun.Spec.TaskSpec
		// TODO: if we want to set source for embedded taskspec, set it here.
		// https://github.com/tektoncd/pipeline/issues/5522
	case taskRun.Spec.TaskRef != nil && taskRun.Spec.TaskRef.Resolver != "":
		task, source, err := getTask(ctx, taskRun.Name)
		switch {
		case err != nil:
			return nil, nil, err
		case task == nil:
			return nil, nil, errors.New("resolution of remote resource completed successfully but no task was returned")
		default:
			taskMeta = task.TaskMetadata()
			taskSpec = task.TaskSpec()
		}
		configSource = source
	default:
		return nil, nil, fmt.Errorf("taskRun %s not providing TaskRef or TaskSpec", taskRun.Name)
	}

	taskSpec.SetDefaults(ctx)
	return &resolutionutil.ResolvedObjectMeta{
		ObjectMeta:   &taskMeta,
		ConfigSource: configSource,
	}, &taskSpec, nil
}
