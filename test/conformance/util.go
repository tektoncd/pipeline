//go:build conformance
// +build conformance

/*
Copyright 2024 The Tekton Authors
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

package conformance_test

import (
	"fmt"

	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
)

// checkTaskRunConditionSucceeded checks the TaskRun Succeeded Condition;
// expectedSucceeded is a corev1.ConditionStatus(string), which is either "True" or "False"
// expectedReason is string, the expected Condition.Reason
func checkTaskRunConditionSucceeded(trStatus v1.TaskRunStatus, expectedSucceededStatus string, expectedReason string) error {
	hasSucceededConditionType := false

	for _, cond := range trStatus.Conditions {
		if cond.Type == "Succeeded" {
			if string(cond.Status) != expectedSucceededStatus {
				return fmt.Errorf("Expect vendor service to populate Condition %s but got: %s", expectedSucceededStatus, cond.Status)
			}
			if cond.Reason != expectedReason {
				return fmt.Errorf("Expect vendor service to populate Condition Reason %s but got: %s", expectedReason, cond.Reason)
			}

			hasSucceededConditionType = true
		}
	}

	if !hasSucceededConditionType {
		return fmt.Errorf("Expect vendor service to populate Succeeded Condition but not apparent in TaskRunStatus")
	}

	return nil
}

// checkPipelineRunConditionSucceeded checks the PipelineRun Succeeded Condition;
// expectedSucceeded is a corev1.ConditionStatus(string), which is either "True" or "False"
// expectedReason is string, the expected Condition.Reason
func checkPipelineRunConditionSucceeded(prStatus v1.PipelineRunStatus, expectedSucceededStatus string, expectedReason string) error {
	hasSucceededConditionType := false

	for _, cond := range prStatus.Conditions {
		if cond.Type == "Succeeded" {
			if string(cond.Status) != expectedSucceededStatus {
				return fmt.Errorf("Expect vendor service to populate Condition %s but got: %s", expectedSucceededStatus, cond.Status)
			}
			if cond.Reason != expectedReason {
				return fmt.Errorf("Expect vendor service to populate Condition Reason %s but got: %s", expectedReason, cond.Reason)
			}

			hasSucceededConditionType = true
		}
	}

	if !hasSucceededConditionType {
		return fmt.Errorf("Expect vendor service to populate Succeeded Condition but not apparent in PipelineRunStatus")
	}

	return nil
}
