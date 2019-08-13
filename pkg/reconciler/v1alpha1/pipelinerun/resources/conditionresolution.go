/*
 *
 * Copyright 2019 The Tekton Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package resources

import (
	"fmt"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

const (
	// unnamedCheckNamePrefix is the prefix added to the name of a condition's
	// spec.Check.Image if the name is missing
	unnamedCheckNamePrefix = "condition-check-"
)

// GetCondition is a function used to retrieve PipelineConditions.
type GetCondition func(string) (*v1alpha1.Condition, error)

// ResolvedConditionCheck contains a Condition and its associated ConditionCheck, if it
// exists. ConditionCheck can be nil to represent there being no ConditionCheck (i.e the condition
// has not been evaluated).
type ResolvedConditionCheck struct {
	ConditionCheckName    string
	Condition             *v1alpha1.Condition
	ConditionCheck        *v1alpha1.ConditionCheck
	PipelineTaskCondition *v1alpha1.PipelineTaskCondition
}

// TaskConditionCheckState is a slice of ResolvedConditionCheck the represents the current execution
// state of Conditions for a Task in a pipeline run.
type TaskConditionCheckState []*ResolvedConditionCheck

// HasStarted returns true if the conditionChecks for a given object have been created
func (state TaskConditionCheckState) HasStarted() bool {
	hasStarted := true
	for _, j := range state {
		if j.ConditionCheck == nil {
			hasStarted = false
		}
	}
	return hasStarted
}

// IsComplete returns true if the status for all conditionChecks for a task indicate that they are done
func (state TaskConditionCheckState) IsDone() bool {
	if !state.HasStarted() {
		return false
	}
	isDone := true
	for _, rcc := range state {
		isDone = isDone && rcc.ConditionCheck.IsDone()
	}
	return isDone
}

// IsComplete returns true if the status for all conditionChecks for a task indicate they have
// completed successfully
func (state TaskConditionCheckState) IsSuccess() bool {
	if !state.IsDone() {
		return false
	}
	isSuccess := true
	for _, rcc := range state {
		isSuccess = isSuccess && rcc.ConditionCheck.IsSuccessful()
	}
	return isSuccess
}

// ConditionToTaskSpec creates a TaskSpec from a given Condition
func (rcc *ResolvedConditionCheck) ConditionToTaskSpec() *v1alpha1.TaskSpec {
	if rcc.Condition.Spec.Check.Name == "" {
		rcc.Condition.Spec.Check.Name = unnamedCheckNamePrefix + rcc.Condition.Name
	}

	t := &v1alpha1.TaskSpec{
		Steps: []v1alpha1.Step{{Container: rcc.Condition.Spec.Check}},
	}

	if len(rcc.Condition.Spec.Params) > 0 {
		t.Inputs = &v1alpha1.Inputs{
			Params: rcc.Condition.Spec.Params,
		}
		// convert param strings of type ${params.x} to ${inputs.params.x}
		// in order to apply taskrun substitution
		convertParamTemplates(&t.Steps[0], rcc.Condition.Spec.Params)
	}
	return t
}

// Replaces all instances of ${params.x} in the container to ${inputs.params.x} for each param name
func convertParamTemplates(step *v1alpha1.Step, params []v1alpha1.ParamSpec) {
	replacements := make(map[string]string)
	for _, p := range params {
		replacements[fmt.Sprintf("params.%s", p.Name)] = fmt.Sprintf("${inputs.params.%s}", p.Name)
		v1alpha1.ApplyStepReplacements(step, replacements, map[string][]string{})
	}
}

// NewConditionCheck status creates a ConditionCheckStatus from a ConditionCheck
func (rcc *ResolvedConditionCheck) NewConditionCheckStatus() *v1alpha1.ConditionCheckStatus {
	var checkStep corev1.ContainerState
	trs := rcc.ConditionCheck.Status
	for _, s := range trs.Steps {
		if s.Name == rcc.Condition.Spec.Check.Name {
			checkStep = s.ContainerState
			break
		}
	}

	return &v1alpha1.ConditionCheckStatus{
		Status:         trs.Status,
		PodName:        trs.PodName,
		StartTime:      trs.StartTime,
		CompletionTime: trs.CompletionTime,
		Check:          checkStep,
	}
}
