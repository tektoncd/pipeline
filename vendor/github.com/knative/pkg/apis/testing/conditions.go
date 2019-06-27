/*
Copyright 2019 The Knative Authors
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

package testing

import (
	"fmt"
	"testing"

	"github.com/knative/pkg/apis"
	duckv1b1 "github.com/knative/pkg/apis/duck/v1beta1"
	corev1 "k8s.io/api/core/v1"
)

// CheckCondition checks if condition `c` on `cc` has value `cs`.
func CheckCondition(s *duckv1b1.Status, c apis.ConditionType, cs corev1.ConditionStatus) error {
	cond := s.GetCondition(c)
	if cond == nil {
		return fmt.Errorf("condition %v is nil", c)
	}
	if cond.Status != cs {
		return fmt.Errorf("condition(%v) = %v, wanted: %v", c, cond, cs)
	}
	return nil
}

// CheckConditionOngoing checks if the condition is in state `Unknown`.
func CheckConditionOngoing(s *duckv1b1.Status, c apis.ConditionType, t *testing.T) {
	t.Helper()
	if err := CheckCondition(s, c, corev1.ConditionUnknown); err != nil {
		t.Error(err)
	}
}

// CheckConditionFailed checks if the condition is in state `False`.
func CheckConditionFailed(s *duckv1b1.Status, c apis.ConditionType, t *testing.T) {
	t.Helper()
	if err := CheckCondition(s, c, corev1.ConditionFalse); err != nil {
		t.Error(err)
	}
}

// CheckConditionSucceeded checks if the condition is in state `True`.
func CheckConditionSucceeded(s *duckv1b1.Status, c apis.ConditionType, t *testing.T) {
	t.Helper()
	if err := CheckCondition(s, c, corev1.ConditionTrue); err != nil {
		t.Error(err)
	}
}
