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
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"

	"github.com/knative/build-pipeline/pkg/apis/pipeline/v1alpha1"
)

// GetNextTask returns the next Task for which a TaskRun should be created,
// or nil if no TaskRun should be created.
func GetNextTask(prName string, state []*PipelineRunTaskRun, logger *zap.SugaredLogger) *PipelineRunTaskRun {
	for _, prtr := range state {
		if prtr.TaskRun != nil {
			c := prtr.TaskRun.Status.GetCondition(duckv1alpha1.ConditionSucceeded)
			if c == nil {
				logger.Infof("TaskRun %s doesn't have a condition so it is just starting and we shouldn't start more for PipelineRun %s", prtr.TaskRunName, prName)
				return nil
			}
			switch c.Status {
			case corev1.ConditionFalse:
				logger.Infof("TaskRun %s has failed; we don't need to run PipelineRun %s", prtr.TaskRunName, prName)
				return nil
			case corev1.ConditionUnknown:
				logger.Infof("TaskRun %s is still running so we shouldn't start more for PipelineRun %s", prtr.TaskRunName, prName)
				return nil
			}
		} else if canTaskRun(prtr.PipelineTask) {
			logger.Infof("TaskRun %s should be started for PipelineRun %s", prtr.TaskRunName, prName)
			return prtr
		}
	}
	logger.Infof("No TaskRuns to start for PipelineRun %s", prName)
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
			// If the Task can't be found, it means the PipelineRun is invalid. Return the same error
			// type so it can be used by the caller.
			return nil, err
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

// GetPipelineConditionStatus will return the Condition that the PipelineRun prName should be
// updated with, based on the status of the TaskRuns in state.
func GetPipelineConditionStatus(prName string, state []*PipelineRunTaskRun, logger *zap.SugaredLogger) *duckv1alpha1.Condition {
	allFinished := true
	for _, prtr := range state {
		if prtr.TaskRun == nil {
			logger.Infof("TaskRun %s doesn't have a Status, so PipelineRun %s isn't finished", prtr.TaskRunName, prName)
			allFinished = false
			break
		}
		c := prtr.TaskRun.Status.GetCondition(duckv1alpha1.ConditionSucceeded)
		if c == nil {
			logger.Infof("TaskRun %s doens't have a condition, so PipelineRun %s isn't finished", prtr.TaskRunName, prName)
			allFinished = false
			break
		}
		// If any TaskRuns have failed, we should halt execution and consider the run failed
		if c.Status == corev1.ConditionFalse {
			logger.Infof("TaskRun %s has failed, so PipelineRun %s has failed", prtr.TaskRunName, prName)
			return &duckv1alpha1.Condition{
				Type:    duckv1alpha1.ConditionSucceeded,
				Status:  corev1.ConditionFalse,
				Reason:  "Failed",
				Message: fmt.Sprintf("TaskRun %s for Task %s has failed", prtr.TaskRun.Name, prtr.Task.Name),
			}
		}
		if c.Status != corev1.ConditionTrue {
			logger.Infof("TaskRun %s is still running so PipelineRun %s is still running", prtr.TaskRunName, prName)
			allFinished = false
		}
	}
	if !allFinished {
		logger.Infof("PipelineRun %s still has running TaskRuns so it isn't yet done", prName)
		return &duckv1alpha1.Condition{
			Type:    duckv1alpha1.ConditionSucceeded,
			Status:  corev1.ConditionUnknown,
			Reason:  "Running",
			Message: "Not all Tasks in the Pipeline have finished executing",
		}
	}
	logger.Infof("All TaskRuns have finished for PipelineRun %s so it has finished", prName)
	return &duckv1alpha1.Condition{
		Type:    duckv1alpha1.ConditionSucceeded,
		Status:  corev1.ConditionTrue,
		Reason:  "Finished",
		Message: "All Tasks have completed executing",
	}
}
