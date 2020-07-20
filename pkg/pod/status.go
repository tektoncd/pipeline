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

package pod

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/pkg/names"
	"github.com/tektoncd/pipeline/pkg/termination"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
)

const (
	// ReasonCouldntGetTask indicates that the reason for the failure status is that the
	// Task couldn't be found
	ReasonCouldntGetTask = "CouldntGetTask"

	// ReasonFailedResolution indicated that the reason for failure status is
	// that references within the TaskRun could not be resolved
	ReasonFailedResolution = "TaskRunResolutionFailed"

	// ReasonFailedValidation indicated that the reason for failure status is
	// that taskrun failed runtime validation
	ReasonFailedValidation = "TaskRunValidationFailed"

	// ReasonExceededResourceQuota indicates that the TaskRun failed to create a pod due to
	// a ResourceQuota in the namespace
	ReasonExceededResourceQuota = "ExceededResourceQuota"

	// ReasonExceededNodeResources indicates that the TaskRun's pod has failed to start due
	// to resource constraints on the node
	ReasonExceededNodeResources = "ExceededNodeResources"

	// ReasonCreateContainerConfigError indicates that the TaskRun failed to create a pod due to
	// config error of container
	ReasonCreateContainerConfigError = "CreateContainerConfigError"

	// ReasonPodCreationFailed indicates that the reason for the current condition
	// is that the creation of the pod backing the TaskRun failed
	ReasonPodCreationFailed = "PodCreationFailed"

	// ReasonPending indicates that the pod is in corev1.Pending, and the reason is not
	// ReasonExceededNodeResources or IsPodHitConfigError
	ReasonPending = "Pending"

	//timeFormat is RFC3339 with millisecond
	timeFormat = "2006-01-02T15:04:05.000Z07:00"
)

const oomKilled = "OOMKilled"

// SidecarsReady returns true if all of the Pod's sidecars are Ready or
// Terminated.
func SidecarsReady(podStatus corev1.PodStatus) bool {
	if podStatus.Phase != corev1.PodRunning {
		return false
	}
	for _, s := range podStatus.ContainerStatuses {
		// If the step indicates that it's a step, skip it.
		// An injected sidecar might not have the "sidecar-" prefix, so
		// we can't just look for that prefix, we need to look at any
		// non-step container.
		if IsContainerStep(s.Name) {
			continue
		}
		if s.State.Running != nil && s.Ready {
			continue
		}
		if s.State.Terminated != nil {
			continue
		}
		return false
	}
	return true
}

// MakeTaskRunStatus returns a TaskRunStatus based on the Pod's status.
func MakeTaskRunStatus(logger *zap.SugaredLogger, tr v1beta1.TaskRun, pod *corev1.Pod, taskSpec v1beta1.TaskSpec) v1beta1.TaskRunStatus {
	trs := &tr.Status
	if trs.GetCondition(apis.ConditionSucceeded) == nil || trs.GetCondition(apis.ConditionSucceeded).Status == corev1.ConditionUnknown {
		// If the taskRunStatus doesn't exist yet, it's because we just started running
		MarkStatusRunning(trs, v1beta1.TaskRunReasonRunning.String(), "Not all Steps in the Task have finished executing")
	}

	trs.PodName = pod.Name
	trs.Steps = []v1beta1.StepState{}
	trs.Sidecars = []v1beta1.SidecarState{}

	for _, s := range pod.Status.ContainerStatuses {
		if IsContainerStep(s.Name) {
			if s.State.Terminated != nil && len(s.State.Terminated.Message) != 0 {
				message, time, err := removeStartInfoFromTerminationMessage(s)
				if err != nil {
					logger.Errorf("error setting the start time of step %q in taskrun %q: %w", s.Name, tr.Name, err)
				}
				if time != nil {
					s.State.Terminated.StartedAt = *time
					s.State.Terminated.Message = message
				}
			}
			trs.Steps = append(trs.Steps, v1beta1.StepState{
				ContainerState: *s.State.DeepCopy(),
				Name:           trimStepPrefix(s.Name),
				ContainerName:  s.Name,
				ImageID:        s.ImageID,
			})
		} else if isContainerSidecar(s.Name) {
			trs.Sidecars = append(trs.Sidecars, v1beta1.SidecarState{
				ContainerState: *s.State.DeepCopy(),
				Name:           TrimSidecarPrefix(s.Name),
				ContainerName:  s.Name,
				ImageID:        s.ImageID,
			})
		}
	}

	// Complete if we did not find a step that is not complete, or the pod is in a definitely complete phase
	complete := areStepsComplete(pod) || pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodFailed

	if complete {
		updateCompletedTaskRun(trs, pod)
	} else {
		updateIncompleteTaskRun(trs, pod)
	}

	// Sort step states according to the order specified in the TaskRun spec's steps.
	trs.Steps = sortTaskRunStepOrder(trs.Steps, taskSpec.Steps)

	return *trs
}

// removeStartInfoFromTerminationMessage searches for a result called "StartedAt" in the JSON-formatted
// termination message of a step and returns the values to use for sets State.Terminated if it's
// found. The "StartedAt" result is also removed from the list of results in the container status.
func removeStartInfoFromTerminationMessage(s corev1.ContainerStatus) (string, *metav1.Time, error) {
	r, err := termination.ParseMessage(s.State.Terminated.Message)
	if err != nil {
		return "", nil, fmt.Errorf("termination message could not be parsed as JSON: %w", err)
	}
	for index, result := range r {
		if result.Key == "StartedAt" {
			t, err := time.Parse(timeFormat, result.Value)
			if err != nil {
				return "", nil, fmt.Errorf("could not parse time value %q in StartedAt field: %w", result.Value, err)
			}
			message := ""
			startedAt := metav1.NewTime(t)
			// remove the entry for the starting time
			r = append(r[:index], r[index+1:]...)
			if len(r) == 0 {
				message = ""
			} else if bytes, err := json.Marshal(r); err != nil {
				return "", nil, fmt.Errorf("error marshalling remaining results back into termination message: %w", err)
			} else {
				message = string(bytes)
			}
			return message, &startedAt, nil
		}
	}
	return "", nil, nil
}

func updateCompletedTaskRun(trs *v1beta1.TaskRunStatus, pod *corev1.Pod) {
	if DidTaskRunFail(pod) {
		msg := getFailureMessage(pod)
		MarkStatusFailure(trs, msg)
	} else {
		MarkStatusSuccess(trs)
	}

	// update tr completed time
	trs.CompletionTime = &metav1.Time{Time: time.Now()}
}

func updateIncompleteTaskRun(trs *v1beta1.TaskRunStatus, pod *corev1.Pod) {
	switch pod.Status.Phase {
	case corev1.PodRunning:
		MarkStatusRunning(trs, v1beta1.TaskRunReasonRunning.String(), "Not all Steps in the Task have finished executing")
	case corev1.PodPending:
		var reason, msg string
		switch {
		case IsPodExceedingNodeResources(pod):
			reason = ReasonExceededNodeResources
			msg = "TaskRun Pod exceeded available resources"
		case IsPodHitConfigError(pod):
			reason = ReasonCreateContainerConfigError
			msg = getWaitingMessage(pod)
		default:
			reason = ReasonPending
			msg = getWaitingMessage(pod)
		}
		MarkStatusRunning(trs, reason, msg)
	}
}

// DidTaskRunFail check the status of pod to decide if related taskrun is failed
func DidTaskRunFail(pod *corev1.Pod) bool {
	f := pod.Status.Phase == corev1.PodFailed
	for _, s := range pod.Status.ContainerStatuses {
		if IsContainerStep(s.Name) {
			if s.State.Terminated != nil {
				f = f || s.State.Terminated.ExitCode != 0 || isOOMKilled(s)
			}
		}
	}
	return f
}

func areStepsComplete(pod *corev1.Pod) bool {
	stepsComplete := len(pod.Status.ContainerStatuses) > 0 && pod.Status.Phase == corev1.PodRunning
	for _, s := range pod.Status.ContainerStatuses {
		if IsContainerStep(s.Name) {
			if s.State.Terminated == nil {
				stepsComplete = false
			}
		}
	}
	return stepsComplete
}

//SortContainerStatuses sort ContainerStatuses based on "FinishedAt"
func SortContainerStatuses(podInstance *corev1.Pod) {
	sort.Slice(podInstance.Status.ContainerStatuses, func(i, j int) bool {
		var ifinish, istart, jfinish, jstart time.Time
		if term := podInstance.Status.ContainerStatuses[i].State.Terminated; term != nil {
			ifinish = term.FinishedAt.Time
			istart = term.StartedAt.Time
		}
		if term := podInstance.Status.ContainerStatuses[j].State.Terminated; term != nil {
			jfinish = term.FinishedAt.Time
			jstart = term.StartedAt.Time
		}

		if ifinish.Equal(jfinish) {
			return istart.Before(jstart)
		}
		return ifinish.Before(jfinish)
	})

}

func getFailureMessage(pod *corev1.Pod) string {
	SortContainerStatuses(pod)
	// First, try to surface an error about the actual build step that failed.
	for _, status := range pod.Status.ContainerStatuses {
		term := status.State.Terminated
		if term != nil && term.ExitCode != 0 {
			// Newline required at end to prevent yaml parser from breaking the log help text at 80 chars
			return fmt.Sprintf("%q exited with code %d (image: %q); for logs run: kubectl -n %s logs %s -c %s\n",
				status.Name, term.ExitCode, status.ImageID,
				pod.Namespace, pod.Name, status.Name)
		}
	}
	// Next, return the Pod's status message if it has one.
	if pod.Status.Message != "" {
		return pod.Status.Message
	}

	for _, s := range pod.Status.ContainerStatuses {
		if IsContainerStep(s.Name) {
			if s.State.Terminated != nil {
				if isOOMKilled(s) {
					return oomKilled
				}
			}
		}
	}

	// Lastly fall back on a generic error message.
	return "build failed for unspecified reasons."
}

// IsPodExceedingNodeResources returns true if the Pod's status indicates there
// are insufficient resources to schedule the Pod.
func IsPodExceedingNodeResources(pod *corev1.Pod) bool {
	for _, podStatus := range pod.Status.Conditions {
		if podStatus.Reason == corev1.PodReasonUnschedulable && strings.Contains(podStatus.Message, "Insufficient") {
			return true
		}
	}
	return false
}

// IsPodHitConfigError returns true if the Pod's status undicates there are config error raised
func IsPodHitConfigError(pod *corev1.Pod) bool {
	for _, containerStatus := range pod.Status.ContainerStatuses {
		if containerStatus.State.Waiting != nil && containerStatus.State.Waiting.Reason == "CreateContainerConfigError" {
			return true
		}
	}
	return false
}

func getWaitingMessage(pod *corev1.Pod) string {
	// First, try to surface reason for pending/unknown about the actual build step.
	for _, status := range pod.Status.ContainerStatuses {
		wait := status.State.Waiting
		if wait != nil && wait.Message != "" {
			return fmt.Sprintf("build step %q is pending with reason %q",
				status.Name, wait.Message)
		}
	}
	// Try to surface underlying reason by inspecting pod's recent status if condition is not true
	for i, podStatus := range pod.Status.Conditions {
		if podStatus.Status != corev1.ConditionTrue {
			return fmt.Sprintf("pod status %q:%q; message: %q",
				pod.Status.Conditions[i].Type,
				pod.Status.Conditions[i].Status,
				pod.Status.Conditions[i].Message)
		}
	}
	// Next, return the Pod's status message if it has one.
	if pod.Status.Message != "" {
		return pod.Status.Message
	}

	// Lastly fall back on a generic pending message.
	return "Pending"
}

// MarkStatusRunning sets taskrun status to running
func MarkStatusRunning(trs *v1beta1.TaskRunStatus, reason, message string) {
	trs.SetCondition(&apis.Condition{
		Type:    apis.ConditionSucceeded,
		Status:  corev1.ConditionUnknown,
		Reason:  reason,
		Message: message,
	})
}

// MarkStatusFailure sets taskrun status to failure
func MarkStatusFailure(trs *v1beta1.TaskRunStatus, message string) {
	trs.SetCondition(&apis.Condition{
		Type:    apis.ConditionSucceeded,
		Status:  corev1.ConditionFalse,
		Reason:  v1beta1.TaskRunReasonFailed.String(),
		Message: message,
	})
}

// MarkStatusSuccess sets taskrun status to success
func MarkStatusSuccess(trs *v1beta1.TaskRunStatus) {
	trs.SetCondition(&apis.Condition{
		Type:    apis.ConditionSucceeded,
		Status:  corev1.ConditionTrue,
		Reason:  v1beta1.TaskRunReasonSuccessful.String(),
		Message: "All Steps have completed executing",
	})
}

// sortTaskRunStepOrder sorts the StepStates in the same order as the original
// TaskSpec steps.
func sortTaskRunStepOrder(taskRunSteps []v1beta1.StepState, taskSpecSteps []v1beta1.Step) []v1beta1.StepState {
	trt := &stepStateSorter{
		taskRunSteps: taskRunSteps,
	}
	trt.mapForSort = trt.constructTaskStepsSorter(taskSpecSteps)
	sort.Sort(trt)
	return trt.taskRunSteps
}

// stepStateSorter implements a sorting mechanism to align the order of the steps in TaskRun
// with the spec steps in Task.
type stepStateSorter struct {
	taskRunSteps []v1beta1.StepState
	mapForSort   map[string]int
}

// constructTaskStepsSorter constructs a map matching the names of
// the steps to their indices for a task.
func (trt *stepStateSorter) constructTaskStepsSorter(taskSpecSteps []v1beta1.Step) map[string]int {
	sorter := make(map[string]int)
	for index, step := range taskSpecSteps {
		stepName := step.Name
		if stepName == "" {
			stepName = names.SimpleNameGenerator.RestrictLength(fmt.Sprintf("unnamed-%d", index))
		}
		sorter[stepName] = index
	}
	return sorter
}

// changeIndex sorts the steps of the task run, based on the
// order of the steps in the task. Instead of changing the element with the one next to it,
// we directly swap it with the desired index.
func (trt *stepStateSorter) changeIndex(index int) {
	// Check if the current index is equal to the desired index. If they are equal, do not swap; if they
	// are not equal, swap index j with the desired index.
	desiredIndex, exist := trt.mapForSort[trt.taskRunSteps[index].Name]
	if exist && index != desiredIndex {
		trt.taskRunSteps[desiredIndex], trt.taskRunSteps[index] = trt.taskRunSteps[index], trt.taskRunSteps[desiredIndex]
	}
}

func (trt *stepStateSorter) Len() int { return len(trt.taskRunSteps) }

func (trt *stepStateSorter) Swap(i, j int) {
	trt.changeIndex(j)
	// The index j is unable to reach the last index.
	// When i reaches the end of the array, we need to check whether the last one needs a swap.
	if i == trt.Len()-1 {
		trt.changeIndex(i)
	}
}

func (trt *stepStateSorter) Less(i, j int) bool {
	// Since the logic is complicated, we move it into the Swap function to decide whether
	// and how to change the index. We set it to true here in order to iterate all the
	// elements of the array in the Swap function.
	return true
}

func isOOMKilled(s corev1.ContainerStatus) bool {
	return s.State.Terminated.Reason == oomKilled
}
