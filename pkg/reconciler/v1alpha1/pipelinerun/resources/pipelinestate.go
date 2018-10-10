/*
Copyright 2018 The Knative Authors

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

	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"

	"github.com/knative/build-pipeline/pkg/apis/pipeline/v1alpha1"
)

// GetNextTask returns the first Task in pipelineTaskRuns that does
// not have a corresponding TaskRun and can run.
func GetNextTask(pipelineTaskRuns []*PipelineRunTaskRun) *PipelineRunTaskRun {
	for _, prtr := range pipelineTaskRuns {
		if prtr.TaskRun != nil {
			switch s := prtr.TaskRun.Status.GetCondition(duckv1alpha1.ConditionSucceeded); s.Status {
			// if any of the TaskRuns failed, there is no new TaskRun to start
			case corev1.ConditionFalse:
				return nil
			// if the current TaskRun is currently running, don't start another one
			case corev1.ConditionUnknown:
				return nil
			}
			// otherwise the TaskRun has finished successfully, so we should move on
		} else if canTaskRun(prtr.PipelineTask) {
			return prtr
		}
	}
	return nil
}

func canTaskRun(pt *v1alpha1.PipelineTask) bool {
	// Check if Task can run now. Go through all the input constraints
	return true
}

// PipelineRunTaskRun contains a Task and its associated TaskRun, if it
// exists. TaskRun can be nil to represent there being no TaskRun.
type PipelineRunTaskRun struct {
	Task         *v1alpha1.Task
	PipelineTask *v1alpha1.PipelineTask
	TaskRunName  string
	TaskRun      *v1alpha1.TaskRun
}

// GetTask is a function that will retrieve the Task name from namespace.
type GetTask func(namespace, name string) (*v1alpha1.Task, error)

// GetTaskRun is a function that will retrieve the TaskRun name from namespace.
type GetTaskRun func(namespace, name string) (*v1alpha1.TaskRun, error)

// GetPipelineState retrieves all Tasks instances which the pipeline p references, getting
// instances from getTask. It will also check if there is a corresponding TaskRun for the
// Task using getTaskRun (the name is built from pipelineRunName). If it is unable to
// retrieve an instance of a referenced Task, it will return an error, otherwise it
// returns a list of all of the Tasks retrieved, and their TaskRuns if applicable.
func GetPipelineState(getTask GetTask, getTaskRun GetTaskRun, p *v1alpha1.Pipeline, pipelineRunName string) ([]*PipelineRunTaskRun, error) {
	state := []*PipelineRunTaskRun{}
	for i := range p.Spec.Tasks {
		pt := p.Spec.Tasks[i]
		t, err := getTask(p.Namespace, pt.TaskRef.Name)
		if err != nil {
			return nil, fmt.Errorf("failed to get tasks for Pipeline %q: Error getting task %q : %s",
				fmt.Sprintf("%s/%s", p.Namespace, p.Name),
				fmt.Sprintf("%s/%s", p.Namespace, pt.TaskRef.Name), err)
		}
		prtr := PipelineRunTaskRun{
			Task:         t,
			PipelineTask: &pt,
		}
		prtr.TaskRunName = getTaskRunName(pipelineRunName, &pt)
		taskRun, err := getTaskRun(p.Namespace, prtr.TaskRunName)
		if err != nil {
			// If the TaskRun isn't found, it just means it hasn't been run yet
			if !errors.IsNotFound(err) {
				return nil, fmt.Errorf("error retrieving TaskRun %s for Task %s: %s", prtr.TaskRunName, t.Name, err)
			}
		} else {
			prtr.TaskRun = taskRun
		}
		state = append(state, &prtr)
	}
	return state, nil
}

// getTaskRunName should return a uniquie name for a `TaskRun`.
func getTaskRunName(prName string, pt *v1alpha1.PipelineTask) string {
	return fmt.Sprintf("%s-%s", prName, pt.Name)
}
