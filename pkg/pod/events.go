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
	"slices"
	"strings"
	"unicode/utf8"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"knative.dev/pkg/logging"
)

const (
	maxEventMessageLength          = 1024
	truncationSuffix               = "..."
	containerWaitingReasonCreating = "ContainerCreating"
	podInitializingReason          = "PodInitializing"
	lastObservedPodWarningPrefix   = "Last observed Pod warning: "
)

type eventLookupContextKey struct{}

// WithPodEventLookup controls whether MakeTaskRunStatus may look up a Pod
// Warning Event. Callers that schedule a delayed retry use it to avoid polling
// Events on unrelated reconciliations.
func WithPodEventLookup(ctx context.Context, enabled bool) context.Context {
	return context.WithValue(ctx, eventLookupContextKey{}, enabled)
}

func podEventLookupEnabled(ctx context.Context) bool {
	enabled, ok := ctx.Value(eventLookupContextKey{}).(bool)
	return !ok || enabled
}

// IsGenericPending reports whether the Pod has no useful current diagnosis and
// is therefore eligible for a Warning Event fallback.
func IsGenericPending(pod *corev1.Pod) bool {
	return !hasUsefulPodDiagnostic(pod)
}

// IsLastObservedPodWarning reports whether message is a controller-written
// Event fallback diagnostic.
func IsLastObservedPodWarning(message string) bool {
	return strings.HasPrefix(message, lastObservedPodWarningPrefix)
}

// latestWarningEvent returns the reason and message of the most recent Warning
// event for the given Pod. If no Warning events exist or the lookup fails, it
// returns empty strings.
func latestWarningEvent(ctx context.Context, kubeclient kubernetes.Interface, pod *corev1.Pod) (reason, message string) {
	if pod.UID == "" {
		return "", ""
	}
	events, err := kubeclient.CoreV1().Events(pod.Namespace).List(ctx, metav1.ListOptions{
		FieldSelector: fmt.Sprintf("involvedObject.uid=%s,type=Warning", pod.UID),
	})
	if err != nil {
		logging.FromContext(ctx).Debugw("Failed to list pod warning events", "pod", pod.Name, "namespace", pod.Namespace, "error", err)
		return "", ""
	}
	matching := make([]corev1.Event, 0, len(events.Items))
	for _, event := range events.Items {
		if event.InvolvedObject.UID == pod.UID && event.Type == corev1.EventTypeWarning {
			matching = append(matching, event)
		}
	}
	if len(matching) == 0 {
		return "", ""
	}

	latest := slices.MaxFunc(matching, func(a, b corev1.Event) int {
		at, bt := eventRecency(a), eventRecency(b)
		if comparison := at.Compare(bt.Time); comparison != 0 {
			return comparison
		}
		return strings.Compare(a.Name, b.Name)
	})

	return truncateWarningParts(latest.Reason, latest.Message)
}

func eventRecency(event corev1.Event) metav1.Time {
	if event.Series != nil && !event.Series.LastObservedTime.IsZero() {
		return metav1.NewTime(event.Series.LastObservedTime.Time)
	}
	if !event.EventTime.IsZero() {
		return metav1.NewTime(event.EventTime.Time)
	}
	return event.LastTimestamp
}

func truncateWarningParts(reason, message string) (string, string) {
	reason = strings.ToValidUTF8(reason, "")
	message = strings.ToValidUTF8(message, "")
	fixedLength := len(lastObservedPodWarningPrefix) + len(reason)
	if fixedLength > maxEventMessageLength {
		return truncateUTF8(reason, maxEventMessageLength-len(lastObservedPodWarningPrefix)-len(truncationSuffix)) + truncationSuffix, ""
	}
	if message == "" {
		return reason, ""
	}
	available := maxEventMessageLength - fixedLength - len(": ")
	if available <= len(truncationSuffix) {
		return reason, ""
	}
	if len(message) > available {
		message = truncateUTF8(message, available-len(truncationSuffix)) + truncationSuffix
	}
	return reason, message
}

func truncateUTF8(value string, maxBytes int) string {
	if len(value) <= maxBytes {
		return value
	}
	end := maxBytes
	for end > 0 && !utf8.RuneStart(value[end]) {
		end--
	}
	return value[:end]
}

// isGenericPending returns true when every available Pod-derived diagnosis is
// empty or a known neutral placeholder.
func isGenericPending(pod *corev1.Pod) bool {
	return IsGenericPending(pod)
}

func hasUsefulPodDiagnostic(pod *corev1.Pod) bool {
	if pod.Status.Message != "" {
		return true
	}
	for _, status := range pod.Status.ContainerStatuses {
		if status.State.Waiting != nil && usefulState(status.State.Waiting.Reason, status.State.Waiting.Message) {
			return true
		}
		if status.State.Terminated != nil && usefulState(status.State.Terminated.Reason, status.State.Terminated.Message) {
			return true
		}
	}
	for _, status := range pod.Status.InitContainerStatuses {
		if status.State.Waiting != nil && usefulState(status.State.Waiting.Reason, status.State.Waiting.Message) {
			return true
		}
		if status.State.Terminated != nil && usefulState(status.State.Terminated.Reason, status.State.Terminated.Message) {
			return true
		}
	}
	for _, condition := range pod.Status.Conditions {
		if condition.Message != "" || usefulReason(condition.Reason) {
			return true
		}
	}
	return false
}

func usefulState(reason, message string) bool {
	return message != "" || usefulReason(reason)
}

func usefulReason(reason string) bool {
	return reason != "" && reason != string(corev1.PodPending) && reason != containerWaitingReasonCreating && reason != podInitializingReason
}
