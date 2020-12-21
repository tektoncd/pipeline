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
	"testing"

	"github.com/google/go-cmp/cmp"
	tbv1alpha1 "github.com/tektoncd/pipeline/internal/builder/v1alpha1"
	tb "github.com/tektoncd/pipeline/internal/builder/v1beta1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	resourcev1alpha1 "github.com/tektoncd/pipeline/pkg/apis/resource/v1alpha1"
	"github.com/tektoncd/pipeline/test/diff"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
	duckv1beta1 "knative.dev/pkg/apis/duck/v1beta1"
)

var c = tbv1alpha1.Condition("conditionname")

var notStartedState = TaskConditionCheckState{{
	ConditionCheckName: "foo",
	Condition:          c,
}}

var runningState = TaskConditionCheckState{{
	ConditionCheckName: "foo",
	Condition:          c,
	ConditionCheck: &v1beta1.ConditionCheck{
		ObjectMeta: metav1.ObjectMeta{
			Name: "running-condition-check",
		},
	},
}}

var successState = TaskConditionCheckState{{
	ConditionCheckName: "foo",
	Condition:          c,
	ConditionCheck: &v1beta1.ConditionCheck{
		ObjectMeta: metav1.ObjectMeta{
			Name: "successful-condition-check",
		},
		Spec: v1beta1.TaskRunSpec{},
		Status: v1beta1.TaskRunStatus{
			Status: duckv1beta1.Status{
				Conditions: []apis.Condition{{
					Type:   apis.ConditionSucceeded,
					Status: corev1.ConditionTrue,
				}},
			},
		},
	},
}}

var failedState = TaskConditionCheckState{{
	ConditionCheckName: "foo",
	Condition:          c,
	ConditionCheck: &v1beta1.ConditionCheck{
		ObjectMeta: metav1.ObjectMeta{
			Name: "failed-condition-check",
		},
		Spec: v1beta1.TaskRunSpec{},
		Status: v1beta1.TaskRunStatus{
			Status: duckv1beta1.Status{
				Conditions: []apis.Condition{{
					Type:   apis.ConditionSucceeded,
					Status: corev1.ConditionFalse,
				}},
			},
		},
	},
}}

func TestTaskConditionCheckState_HasStarted(t *testing.T) {
	tcs := []struct {
		name  string
		state TaskConditionCheckState
		want  bool
	}{{
		name:  "no-condition-checks",
		state: notStartedState,
		want:  false,
	}, {
		name:  "running-condition-check",
		state: runningState,
		want:  true,
	}, {
		name:  "successful-condition-check",
		state: successState,
		want:  true,
	}, {
		name:  "failed-condition-check",
		state: failedState,
		want:  true,
	}}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.state.HasStarted()
			if got != tc.want {
				t.Errorf("Expected HasStarted to be %v but got %v for %s", tc.want, got, tc.name)
			}
		})
	}
}

func TestTaskConditionCheckState_IsComplete(t *testing.T) {
	tcs := []struct {
		name  string
		state TaskConditionCheckState
		want  bool
	}{{
		name:  "no-condition-checks",
		state: notStartedState,
		want:  false,
	}, {
		name:  "running-condition-check",
		state: runningState,
		want:  false,
	}, {
		name:  "successful-condition-check",
		state: successState,
		want:  true,
	}, {
		name:  "failed-condition-check",
		state: failedState,
		want:  true,
	}}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.state.IsDone()
			if got != tc.want {
				t.Errorf("Expected IsComplete to be %v but got %v for %s", tc.want, got, tc.name)
			}
		})
	}
}

func TestTaskConditionCheckState_IsSuccess(t *testing.T) {
	tcs := []struct {
		name  string
		state TaskConditionCheckState
		want  bool
	}{{
		name:  "no-condition-checks",
		state: notStartedState,
		want:  false,
	}, {
		name:  "running-condition-check",
		state: runningState,
		want:  false,
	}, {
		name:  "successful-condition-check",
		state: successState,
		want:  true,
	}, {
		name:  "failed-condition-check",
		state: failedState,
		want:  false,
	}}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.state.IsSuccess()
			if got != tc.want {
				t.Errorf("Expected IsSuccess to be %v but got %v for %s", tc.want, got, tc.name)
			}
		})
	}
}

func TestResolvedConditionCheck_ConditionToTaskSpec(t *testing.T) {
	tcs := []struct {
		name              string
		cond              *v1alpha1.Condition
		resolvedResources map[string]*resourcev1alpha1.PipelineResource
		want              v1beta1.TaskSpec
	}{{
		name: "user-provided-container-name",
		cond: tbv1alpha1.Condition("name", tbv1alpha1.ConditionSpec(
			tbv1alpha1.ConditionSpecCheck("foo", "ubuntu"),
		)),
		want: v1beta1.TaskSpec{
			Steps: []v1beta1.Step{{Container: corev1.Container{
				Name:  "foo",
				Image: "ubuntu",
			}}},
		},
	}, {
		name: "default-container-name",
		cond: tbv1alpha1.Condition("bar", tbv1alpha1.ConditionSpec(
			tbv1alpha1.ConditionSpecCheck("", "ubuntu"),
		)),
		want: v1beta1.TaskSpec{
			Steps: []v1beta1.Step{{Container: corev1.Container{
				Name:  "condition-check-bar",
				Image: "ubuntu",
			}}},
		},
	}, {
		name: "default-container-name",
		cond: tbv1alpha1.Condition("very-very-very-very-very-very-very-very-very-very-very-long-name", tbv1alpha1.ConditionSpec(
			tbv1alpha1.ConditionSpecCheck("", "ubuntu"),
		)),
		want: v1beta1.TaskSpec{
			Steps: []v1beta1.Step{{Container: corev1.Container{
				// Shortened via: names.SimpleNameGenerator.RestrictLength
				Name:  "condition-check-very-very-very-very-very-very-very-very-very-ve",
				Image: "ubuntu",
			}}},
		},
	}, {
		name: "with-input-params",
		cond: tbv1alpha1.Condition("bar", tbv1alpha1.ConditionSpec(
			tbv1alpha1.ConditionSpecCheck("$(params.name)", "$(params.img)",
				tb.WorkingDir("$(params.not.replaced)")),
			tbv1alpha1.ConditionParamSpec("name", v1beta1.ParamTypeString),
			tbv1alpha1.ConditionParamSpec("img", v1beta1.ParamTypeString),
		)),
		want: v1beta1.TaskSpec{
			Steps: []v1beta1.Step{{Container: corev1.Container{
				Name:       "$(inputs.params.name)",
				Image:      "$(inputs.params.img)",
				WorkingDir: "$(params.not.replaced)",
			}}},
			Params: []v1beta1.ParamSpec{{
				Name: "name",
				Type: "string",
			}, {
				Name: "img",
				Type: "string",
			}},
		},
	}, {
		name: "with-resources",
		cond: tbv1alpha1.Condition("bar", tbv1alpha1.ConditionSpec(
			tbv1alpha1.ConditionSpecCheck("name", "ubuntu",
				tb.Args("$(resources.git-resource.revision)")),
			tbv1alpha1.ConditionResource("git-resource", resourcev1alpha1.PipelineResourceTypeGit),
		)),
		resolvedResources: map[string]*resourcev1alpha1.PipelineResource{
			"git-resource": tb.PipelineResource("git-resource",
				tb.PipelineResourceSpec(resourcev1alpha1.PipelineResourceTypeGit,
					tb.PipelineResourceSpecParam("revision", "master"),
				)),
		},
		want: v1beta1.TaskSpec{
			Steps: []v1beta1.Step{{Container: corev1.Container{
				Name:  "name",
				Image: "ubuntu",
				Args:  []string{"master"},
			}}},
			Resources: &v1beta1.TaskResources{
				Inputs: []v1beta1.TaskResource{{
					ResourceDeclaration: v1beta1.ResourceDeclaration{
						Name: "git-resource",
						Type: "git",
					}}},
			},
		},
	}}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			rcc := &ResolvedConditionCheck{Condition: tc.cond, ResolvedResources: tc.resolvedResources}
			got, err := rcc.ConditionToTaskSpec()
			if err != nil {
				t.Errorf("Unexpected error when converting condition to task spec: %v", err)
			}

			if d := cmp.Diff(&tc.want, got); d != "" {
				t.Errorf("TaskSpec generated from Condition is unexpected %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestResolvedConditionCheck_ToTaskResourceBindings(t *testing.T) {
	rcc := ResolvedConditionCheck{
		ResolvedResources: map[string]*resourcev1alpha1.PipelineResource{
			"git-resource": tb.PipelineResource("some-repo"),
		},
	}

	expected := []v1beta1.TaskResourceBinding{{
		PipelineResourceBinding: v1beta1.PipelineResourceBinding{
			Name: "git-resource",
		},
	}}

	if d := cmp.Diff(expected, rcc.ToTaskResourceBindings()); d != "" {
		t.Errorf("Did not get expected task resource binding from condition %s", diff.PrintWantGot(d))
	}
}
