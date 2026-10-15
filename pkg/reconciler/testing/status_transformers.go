package testing

import (
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	v1beta1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	corev1 "k8s.io/api/core/v1"
	apis "knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

// Status transformer helpers for testing
func MakeScheduled(taskRun v1.TaskRun, newTaskRun func(v1.TaskRun) *v1.TaskRun) *v1.TaskRun {
	tr := newTaskRun(taskRun)
	tr.Status = v1.TaskRunStatus{}
	return tr
}

func MakeStarted(taskRun v1.TaskRun, newTaskRun func(v1.TaskRun) *v1.TaskRun) *v1.TaskRun {
	tr := newTaskRun(taskRun)
	tr.Status.Conditions[0].Status = corev1.ConditionUnknown
	return tr
}

func MakeSucceeded(taskRun v1.TaskRun, newTaskRun func(v1.TaskRun) *v1.TaskRun) *v1.TaskRun {
	tr := newTaskRun(taskRun)
	tr.Status.Conditions[0].Status = corev1.ConditionTrue
	tr.Status.Conditions[0].Reason = "Succeeded"
	return tr
}

func MakeFailed(taskRun v1.TaskRun, newTaskRun func(v1.TaskRun) *v1.TaskRun) *v1.TaskRun {
	tr := newTaskRun(taskRun)
	tr.Status.Conditions[0].Status = corev1.ConditionFalse
	tr.Status.Conditions[0].Reason = "Failed"
	return tr
}

func MakeToBeRetried(taskRun v1.TaskRun, newTaskRun func(v1.TaskRun) *v1.TaskRun) *v1.TaskRun {
	tr := newTaskRun(taskRun)
	tr.Status.Conditions[0].Status = corev1.ConditionUnknown
	tr.Status.Conditions[0].Reason = v1.TaskRunReasonToBeRetried.String()
	return tr
}

func MakeRetried(taskRun v1.TaskRun, newTaskRun func(v1.TaskRun) *v1.TaskRun, withRetries func(*v1.TaskRun) *v1.TaskRun) *v1.TaskRun {
	tr := newTaskRun(taskRun)
	return withRetries(tr)
}

func WithRetries(taskRun *v1.TaskRun) *v1.TaskRun {
	taskRun.Status.RetriesStatus = []v1.TaskRunStatus{{
		Status: duckv1.Status{
			Conditions: []apis.Condition{{
				Type:   apis.ConditionSucceeded,
				Status: corev1.ConditionFalse,
			}},
		},
	}}
	return taskRun
}

func WithCancelled(taskRun *v1.TaskRun) *v1.TaskRun {
	taskRun.Status.Conditions[0].Reason = v1.TaskRunSpecStatusCancelled
	return taskRun
}

func WithCancelledForTimeout(taskRun *v1.TaskRun) *v1.TaskRun {
	taskRun.Spec.StatusMessage = v1.TaskRunCancelledByPipelineTimeoutMsg
	taskRun.Status.Conditions[0].Reason = v1.TaskRunSpecStatusCancelled
	return taskRun
}

func WithCancelledBySpec(taskRun *v1.TaskRun) *v1.TaskRun {
	taskRun.Spec.Status = v1.TaskRunSpecStatusCancelled
	return taskRun
}

// CustomRun status helpers
func MakeCustomRunStarted(customRun v1beta1.CustomRun, newCustomRun func(v1beta1.CustomRun) *v1beta1.CustomRun) *v1beta1.CustomRun {
	run := newCustomRun(customRun)
	run.Status.Conditions[0].Status = corev1.ConditionUnknown
	return run
}

func MakeCustomRunSucceeded(customRun v1beta1.CustomRun, newCustomRun func(v1beta1.CustomRun) *v1beta1.CustomRun) *v1beta1.CustomRun {
	run := newCustomRun(customRun)
	run.Status.Conditions[0].Status = corev1.ConditionTrue
	run.Status.Conditions[0].Reason = "Succeeded"
	return run
}

func MakeCustomRunFailed(customRun v1beta1.CustomRun, newCustomRun func(v1beta1.CustomRun) *v1beta1.CustomRun) *v1beta1.CustomRun {
	run := newCustomRun(customRun)
	run.Status.Conditions[0].Status = corev1.ConditionFalse
	run.Status.Conditions[0].Reason = "Failed"
	return run
}

func WithCustomRunRetries(customRun *v1beta1.CustomRun) *v1beta1.CustomRun {
	customRun.Status.RetriesStatus = []v1beta1.CustomRunStatus{{
		Status: duckv1.Status{
			Conditions: []apis.Condition{{
				Type:   apis.ConditionSucceeded,
				Status: corev1.ConditionFalse,
			}},
		},
	}}
	return customRun
}

func WithCustomRunCancelled(customRun *v1beta1.CustomRun) *v1beta1.CustomRun {
	customRun.Status.Conditions[0].Reason = v1beta1.CustomRunReasonCancelled.String()
	return customRun
}

func WithCustomRunCancelledForTimeout(customRun *v1beta1.CustomRun) *v1beta1.CustomRun {
	customRun.Spec.StatusMessage = v1beta1.CustomRunCancelledByPipelineTimeoutMsg
	customRun.Status.Conditions[0].Reason = v1beta1.CustomRunReasonCancelled.String()
	return customRun
}

func WithCustomRunCancelledBySpec(customRun *v1beta1.CustomRun) *v1beta1.CustomRun {
	customRun.Spec.Status = v1beta1.CustomRunSpecStatusCancelled
	return customRun
}
