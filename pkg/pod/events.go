/*
Copyright 2026 The Tekton Authors

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
	"context"
	"fmt"
	"strings"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const maxEventMessageLength = 1024

// latestWarningEvent returns the reason and message of the most recent Warning
// event for the given Pod. It performs a single, field-selected List scoped to
// the Pod's UID. If no Warning events exist or the lookup fails, it returns
// empty strings.
func latestWarningEvent(ctx context.Context, logger *zap.SugaredLogger, kubeclient kubernetes.Interface, pod *corev1.Pod) (reason, message string) {
	if pod.UID == "" {
		return "", ""
	}
	events, err := kubeclient.CoreV1().Events(pod.Namespace).List(ctx, metav1.ListOptions{
		FieldSelector: fmt.Sprintf("involvedObject.uid=%s,type=Warning", pod.UID),
	})
	if err != nil {
		logger.Debugw("Failed to list pod warning events", "pod", pod.Name, "namespace", pod.Namespace, "error", err)
		return "", ""
	}
	if len(events.Items) == 0 {
		return "", ""
	}

	latest := events.Items[0]
	for i := range events.Items[1:] {
		ev := &events.Items[i+1]
		if ev.LastTimestamp.After(latest.LastTimestamp.Time) {
			latest = *ev
		}
	}

	msg := latest.Message
	if len(msg) > maxEventMessageLength {
		msg = msg[:maxEventMessageLength] + "..."
	}
	return latest.Reason, msg
}

// isGenericPending returns true when the Pod is stuck in a pending state that
// carries no actionable detail — the container waiting reason is
// "ContainerCreating" with an empty message, or getWaitingMessage fell through
// to one of its generic fallbacks.
func isGenericPending(pod *corev1.Pod) bool {
	for _, s := range pod.Status.ContainerStatuses {
		if s.State.Waiting != nil && s.State.Waiting.Reason == "ContainerCreating" && s.State.Waiting.Message == "" {
			return true
		}
	}
	for _, s := range pod.Status.InitContainerStatuses {
		if s.State.Waiting != nil && s.State.Waiting.Reason == "ContainerCreating" && s.State.Waiting.Message == "" {
			return true
		}
	}
	// If no container statuses exist yet the pod hasn't been scheduled so we
	// can't determine anything useful.
	if len(pod.Status.ContainerStatuses) == 0 && len(pod.Status.InitContainerStatuses) == 0 {
		return false
	}
	// This matches the final fallback path in getWaitingMessage: no container
	// has a waiting message, all pod conditions are True, and pod.Status.Message
	// is empty, so getWaitingMessage would return the bare "Pending" string
	// with no actionable detail.
	if noContainerHasWaitingMessage(pod) && allConditionsTrue(pod) && pod.Status.Message == "" {
		return true
	}
	return false
}

func noContainerHasWaitingMessage(pod *corev1.Pod) bool {
	for _, s := range pod.Status.ContainerStatuses {
		if s.State.Waiting != nil && s.State.Waiting.Message != "" {
			return false
		}
	}
	return true
}

func allConditionsTrue(pod *corev1.Pod) bool {
	for _, c := range pod.Status.Conditions {
		if c.Status != corev1.ConditionTrue {
			return false
		}
	}
	return true
}

// formatEventMessage formats an event reason and message into a string
// suitable for a TaskRun status condition message.
func formatEventMessage(reason, message string) string {
	if strings.TrimSpace(message) == "" {
		return reason
	}
	return fmt.Sprintf("%s: %s", reason, message)
}
