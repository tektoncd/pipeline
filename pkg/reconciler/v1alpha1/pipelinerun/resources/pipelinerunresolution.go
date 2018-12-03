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
	"github.com/knative/build-pipeline/pkg/reconciler/v1alpha1/taskrun/resources"
)

const (
	// ReasonRunning indicates that the reason for the inprogress status is that the TaskRun
	// is just starting to be reconciled
	ReasonRunning = "Running"

	// ReasonFailed indicates that the reason for the failure status is that one of the TaskRuns failed
	ReasonFailed = "Failed"

	// ReasonSucceeded indicates that the reason for the finished status is that all of the TaskRuns
	// completed successfully
	ReasonSucceeded = "Succeeded"
)

// GetNextTask returns the next Task for which a TaskRun should be created,
// or nil if no TaskRun should be created.
func GetNextTask(prName string, state []*ResolvedPipelineRunTask, logger *zap.SugaredLogger) *ResolvedPipelineRunTask {
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
		} else if canTaskRun(prtr.PipelineTask, state) {
			logger.Infof("TaskRun %s should be started for PipelineRun %s", prtr.TaskRunName, prName)
			return prtr
		}
	}
	logger.Infof("No TaskRuns to start for PipelineRun %s", prName)
	return nil
}

func canTaskRun(pt *v1alpha1.PipelineTask, state []*ResolvedPipelineRunTask) bool {
	// Check if Task can run now. Go through all the input constraints
	for _, rd := range pt.ResourceDependencies {
		if len(rd.ProvidedBy) > 0 {
			for _, constrainingTaskName := range rd.ProvidedBy {
				for _, prtr := range state {
					// the constraining task must have a successful task run to allow this task to run
					if prtr.PipelineTask.Name == constrainingTaskName {
						if prtr.TaskRun == nil {
							return false
						}
						c := prtr.TaskRun.Status.GetCondition(duckv1alpha1.ConditionSucceeded)
						if c == nil {
							return false
						}
						switch c.Status {
						case corev1.ConditionFalse:
							return false
						case corev1.ConditionUnknown:
							return false
						}
					}
				}
			}
		}
	}
	return true
}

// ResolvedPipelineRunTask contains a Task and its associated TaskRun, if it
// exists. TaskRun can be nil to represent there being no TaskRun.
type ResolvedPipelineRunTask struct {
	TaskRunName           string
	TaskRun               *v1alpha1.TaskRun
	PipelineTask          *v1alpha1.PipelineTask
	ResolvedTaskResources *resources.ResolvedTaskResources
}

// GetTaskRun is a function that will retrieve the TaskRun name.
type GetTaskRun func(name string) (*v1alpha1.TaskRun, error)

func getPipelineRunTask(pipelineTaskName string, pr *v1alpha1.PipelineRun) *v1alpha1.PipelineTaskResource {
	for _, ptr := range pr.Spec.PipelineTaskResources {
		if ptr.Name == pipelineTaskName {
			return &ptr
		}
	}
	// It's not an error to not find corresponding resources, because a Task may not need resources.
	// Validation should occur later once resolution is done.
	return nil
}

// ResolvePipelineRun retrieves all Tasks instances which the pipeline p references, getting
// instances from getTask. If it is unable to retrieve an instance of a referenced Task, it
// will return an error, otherwise it returns a list of all of the Tasks retrieved.
// It will retrieve the Resources needed for the TaskRun as well using getResource.
func ResolvePipelineRun(getTask resources.GetTask, getResource resources.GetResource, p *v1alpha1.Pipeline, pr *v1alpha1.PipelineRun) ([]*ResolvedPipelineRunTask, error) {
	state := []*ResolvedPipelineRunTask{}
	for i := range p.Spec.Tasks {
		pt := p.Spec.Tasks[i]

		rprt := ResolvedPipelineRunTask{
			PipelineTask: &pt,
			TaskRunName:  getTaskRunName(pr.Name, &pt),
		}

		// Find the Task that this task in the Pipeline is using
		t, err := getTask(pt.TaskRef.Name)
		if err != nil {
			// If the Task can't be found, it means the PipelineRun is invalid. Return the same error
			// type so it can be used by the caller.
			return nil, err
		}

		// Get all the resources that this task will be using
		ptr := getPipelineRunTask(pt.Name, pr)
		rtr, err := resources.ResolveTaskResources(&t.Spec, t.Name, ptr.Inputs, ptr.Outputs, getResource)
		if err != nil {
			return nil, fmt.Errorf("couldn't resolve task resources for task %q: %v", t.Name, err)
		}
		rprt.ResolvedTaskResources = rtr

		// Add this task to the state of the PipelineRun
		state = append(state, &rprt)
	}
	return state, nil
}

// ResolveTaskRuns will go through all tasks in state and check if there are existing TaskRuns
// for each of them by calling getTaskRun.
func ResolveTaskRuns(getTaskRun GetTaskRun, state []*ResolvedPipelineRunTask) error {
	for _, rprt := range state {
		// Check if we have already started a TaskRun for this task
		taskRun, err := getTaskRun(rprt.TaskRunName)
		if err != nil {
			// If the TaskRun isn't found, it just means it hasn't been run yet
			if !errors.IsNotFound(err) {
				return fmt.Errorf("error retrieving TaskRun %s: %s", rprt.TaskRunName, err)
			}
		} else {
			rprt.TaskRun = taskRun
		}
	}
	return nil
}

// getTaskRunName should return a uniquie name for a `TaskRun`.
func getTaskRunName(prName string, pt *v1alpha1.PipelineTask) string {
	return fmt.Sprintf("%s-%s", prName, pt.Name)
}

// GetPipelineConditionStatus will return the Condition that the PipelineRun prName should be
// updated with, based on the status of the TaskRuns in state.
func GetPipelineConditionStatus(prName string, state []*ResolvedPipelineRunTask, logger *zap.SugaredLogger) *duckv1alpha1.Condition {
	allFinished := true
	for _, rprt := range state {
		if rprt.TaskRun == nil {
			logger.Infof("TaskRun %s doesn't have a Status, so PipelineRun %s isn't finished", rprt.TaskRunName, prName)
			allFinished = false
			break
		}
		c := rprt.TaskRun.Status.GetCondition(duckv1alpha1.ConditionSucceeded)
		if c == nil {
			logger.Infof("TaskRun %s doens't have a condition, so PipelineRun %s isn't finished", rprt.TaskRunName, prName)
			allFinished = false
			break
		}
		// If any TaskRuns have failed, we should halt execution and consider the run failed
		if c.Status == corev1.ConditionFalse {
			logger.Infof("TaskRun %s has failed, so PipelineRun %s has failed", rprt.TaskRunName, prName)
			return &duckv1alpha1.Condition{
				Type:    duckv1alpha1.ConditionSucceeded,
				Status:  corev1.ConditionFalse,
				Reason:  ReasonFailed,
				Message: fmt.Sprintf("TaskRun %s has failed", rprt.TaskRun.Name),
			}
		}
		if c.Status != corev1.ConditionTrue {
			logger.Infof("TaskRun %s is still running so PipelineRun %s is still running", rprt.TaskRunName, prName)
			allFinished = false
		}
	}
	if !allFinished {
		logger.Infof("PipelineRun %s still has running TaskRuns so it isn't yet done", prName)
		return &duckv1alpha1.Condition{
			Type:    duckv1alpha1.ConditionSucceeded,
			Status:  corev1.ConditionUnknown,
			Reason:  ReasonRunning,
			Message: "Not all Tasks in the Pipeline have finished executing",
		}
	}
	logger.Infof("All TaskRuns have finished for PipelineRun %s so it has finished", prName)
	return &duckv1alpha1.Condition{
		Type:    duckv1alpha1.ConditionSucceeded,
		Status:  corev1.ConditionTrue,
		Reason:  ReasonSucceeded,
		Message: "All Tasks have completed executing",
	}
}
