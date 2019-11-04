package pod

import (
	"fmt"
	"strings"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/reconciler/taskrun/resources"
	"github.com/tektoncd/pipeline/pkg/status"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
)

const (
	readyAnnotationKey   = "tekton.dev/ready"
	readyAnnotationValue = "READY"
)

// UpdateTaskRunStatusFromPodStatus updates the TaskRunStatus corresponding to
// the given Pod's PodStatus.
//
// It returns any Pod annotations that should be applied to the input Pod.
// This is done to signal to the entrypoint binary that runs step containers in
// order that the TaskRun's sidecars are ready.
func UpdateTaskRunStatusFromPod(pod *corev1.Pod, trs *v1alpha1.TaskRunStatus) map[string]string {
	trs.PodName = pod.Name
	trs.StartTime = pod.Status.StartTime

	trs.Steps = nil
	for _, s := range pod.Status.ContainerStatuses {
		if resources.IsContainerStep(s.Name) {
			trs.Steps = append(trs.Steps, v1alpha1.StepState{
				ContainerState: *s.State.DeepCopy(),
				Name:           resources.TrimContainerNamePrefix(s.Name),
				ContainerName:  s.Name,
				ImageID:        s.ImageID,
			})
		}
	}

	var podAnnotations map[string]string
	sidecarsCount, readyOrTerminatedSidecarsCount := countSidecars(pod)
	if pod.Status.Phase == corev1.PodRunning &&
		readyOrTerminatedSidecarsCount == sidecarsCount {
		podAnnotations = map[string]string{readyAnnotationKey: readyAnnotationValue}
	}

	trs.SetCondition(&apis.Condition{
		Type:    apis.ConditionSucceeded,
		Status:  corev1.ConditionUnknown,
		Reason:  status.ReasonRunning,
		Message: "Not all Steps in the Task have finished executing",
	})

	switch pod.Status.Phase {
	case corev1.PodFailed:
		msg := getFailureMessage(pod)
		trs.SetCondition(&apis.Condition{
			Type:    apis.ConditionSucceeded,
			Status:  corev1.ConditionFalse,
			Reason:  status.ReasonFailed,
			Message: msg,
		})
		trs.CompletionTime = getLastFinishTime(pod.Status)
	case corev1.PodSucceeded:
		trs.SetCondition(&apis.Condition{
			Type:    apis.ConditionSucceeded,
			Status:  corev1.ConditionTrue,
			Reason:  status.ReasonSucceeded,
			Message: "All Steps have completed executing",
		})
		trs.CompletionTime = getLastFinishTime(pod.Status)
	case corev1.PodRunning:
		trs.SetCondition(&apis.Condition{
			Type:    apis.ConditionSucceeded,
			Status:  corev1.ConditionUnknown,
			Reason:  status.ReasonRunning,
			Message: "Not all Steps in the Task have finished executing",
		})
	case corev1.PodPending:
		var reason, msg string
		if isPodExceedingNodeResources(pod) {
			reason = status.ReasonExceededNodeResources
			msg = "TaskRun exceeded available resources"
		} else {
			reason = "Pending"
			msg = getWaitingMessage(pod)
		}
		trs.SetCondition(&apis.Condition{
			Type:    apis.ConditionSucceeded,
			Status:  corev1.ConditionUnknown,
			Reason:  reason,
			Message: msg,
		})
	}
	return podAnnotations
}

func getLastFinishTime(podStatus corev1.PodStatus) *metav1.Time {
	// Iterate container statuses backwards until one (typically the last
	// one) reports the time it finished.
	for i := len(podStatus.ContainerStatuses) - 1; i >= 0; i-- {
		this := podStatus.ContainerStatuses[i]
		if !resources.IsContainerStep(this.Name) {
			continue
		}
		if this.State.Terminated != nil {
			return &this.State.Terminated.FinishedAt
		}
	}
	// This shouldn't happen, since the Pod is terminated and should report
	// at least one terminated container (typically the last one), but just
	// in case reporting zero is better than panicking.
	return &metav1.Time{}
}

func countSidecars(pod *corev1.Pod) (total int, readyOrTerminated int) {
	for _, s := range pod.Status.ContainerStatuses {
		if !resources.IsContainerStep(s.Name) {
			if s.State.Running != nil && s.Ready {
				readyOrTerminated++
			} else if s.State.Terminated != nil {
				readyOrTerminated++
			}
			total++
		}
	}
	return total, readyOrTerminated
}

func getFailureMessage(pod *corev1.Pod) string {
	// First, try to surface an error about the actual build step that failed.
	for _, status := range pod.Status.ContainerStatuses {
		term := status.State.Terminated
		if term != nil && term.ExitCode != 0 {
			return fmt.Sprintf("%q exited with code %d (image: %q); for logs run: kubectl -n %s logs %s -c %s",
				status.Name, term.ExitCode, status.ImageID,
				pod.Namespace, pod.Name, status.Name)
		}
	}
	// Next, return the Pod's status message if it has one.
	if pod.Status.Message != "" {
		return pod.Status.Message
	}
	// Lastly fall back on a generic error message.
	return "build failed for unspecified reasons."
}

func isPodExceedingNodeResources(pod *corev1.Pod) bool {
	for _, podStatus := range pod.Status.Conditions {
		if podStatus.Reason == corev1.PodReasonUnschedulable && strings.Contains(podStatus.Message, "Insufficient") {
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
	return status.ReasonPending
}
