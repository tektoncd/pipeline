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

package v1alpha1_test

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	"knative.dev/pkg/apis"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	tb "github.com/tektoncd/pipeline/test/builder"
)

func TestConditionCheck_IsDone(t *testing.T) {
	tr := tb.TaskRun("", "", tb.TaskRunStatus(tb.StatusCondition(
		apis.Condition{
			Type:   apis.ConditionSucceeded,
			Status: corev1.ConditionFalse,
		},
	)))

	cc := v1alpha1.ConditionCheck(*tr)
	if !cc.IsDone() {
		t.Fatal("Expected conditionCheck status to be done")
	}
}

func TestConditionCheck_IsSuccessful(t *testing.T) {
	tr := tb.TaskRun("", "", tb.TaskRunStatus(tb.StatusCondition(
		apis.Condition{
			Type:   apis.ConditionSucceeded,
			Status: corev1.ConditionTrue,
		},
	)))

	cc := v1alpha1.ConditionCheck(*tr)
	if !cc.IsSuccessful() {
		t.Fatal("Expected conditionCheck status to be done")
	}
}
