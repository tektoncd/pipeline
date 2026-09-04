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
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	fakek8s "k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

func TestLatestWarningEvent(t *testing.T) {
	base := time.Date(2026, 9, 3, 1, 0, 0, 0, time.UTC)
	newTime := func(after time.Duration) metav1.MicroTime { return metav1.NewMicroTime(base.Add(after)) }
	legacyTime := func(after time.Duration) metav1.Time { return metav1.NewTime(base.Add(after)) }
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod", Namespace: "ns", UID: types.UID("uid-1")}}

	tests := []struct {
		name       string
		events     []corev1.Event
		wantReason string
		wantMsg    string
	}{
		{name: "series last observed time wins", events: []corev1.Event{
			{ObjectMeta: metav1.ObjectMeta{Name: "event-time"}, InvolvedObject: corev1.ObjectReference{UID: pod.UID}, Type: corev1.EventTypeWarning, Reason: "EventTime", Message: "older series", EventTime: newTime(20 * time.Second), Series: &corev1.EventSeries{LastObservedTime: newTime(10 * time.Second)}},
			{ObjectMeta: metav1.ObjectMeta{Name: "series"}, InvolvedObject: corev1.ObjectReference{UID: pod.UID}, Type: corev1.EventTypeWarning, Reason: "Series", Message: "latest series", EventTime: newTime(time.Second), Series: &corev1.EventSeries{LastObservedTime: newTime(30 * time.Second)}},
		}, wantReason: "Series", wantMsg: "latest series"},
		{name: "event time wins when no series time is present", events: []corev1.Event{
			{ObjectMeta: metav1.ObjectMeta{Name: "legacy"}, InvolvedObject: corev1.ObjectReference{UID: pod.UID}, Type: corev1.EventTypeWarning, Reason: "Legacy", Message: "old", LastTimestamp: legacyTime(20 * time.Second)},
			{ObjectMeta: metav1.ObjectMeta{Name: "event-time"}, InvolvedObject: corev1.ObjectReference{UID: pod.UID}, Type: corev1.EventTypeWarning, Reason: "EventTime", Message: "new", EventTime: newTime(30 * time.Second)},
		}, wantReason: "EventTime", wantMsg: "new"},
		{name: "legacy last timestamp is the final time fallback", events: []corev1.Event{
			{ObjectMeta: metav1.ObjectMeta{Name: "old"}, InvolvedObject: corev1.ObjectReference{UID: pod.UID}, Type: corev1.EventTypeWarning, Reason: "Old", Message: "old", LastTimestamp: legacyTime(time.Second)},
			{ObjectMeta: metav1.ObjectMeta{Name: "new"}, InvolvedObject: corev1.ObjectReference{UID: pod.UID}, Type: corev1.EventTypeWarning, Reason: "New", Message: "new", LastTimestamp: legacyTime(2 * time.Second)},
		}, wantReason: "New", wantMsg: "new"},
		{name: "equal timestamps use event name deterministically", events: []corev1.Event{
			{ObjectMeta: metav1.ObjectMeta{Name: "a-event"}, InvolvedObject: corev1.ObjectReference{UID: pod.UID}, Type: corev1.EventTypeWarning, Reason: "A", Message: "first", EventTime: newTime(time.Second)},
			{ObjectMeta: metav1.ObjectMeta{Name: "z-event"}, InvolvedObject: corev1.ObjectReference{UID: pod.UID}, Type: corev1.EventTypeWarning, Reason: "Z", Message: "last", EventTime: newTime(time.Second)},
		}, wantReason: "Z", wantMsg: "last"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kubeclient := fakek8s.NewSimpleClientset()
			kubeclient.PrependReactor("list", "events", func(action k8stesting.Action) (bool, runtime.Object, error) {
				listAction := action.(k8stesting.ListAction)
				if got, want := listAction.GetListRestrictions().Fields.String(), "involvedObject.uid=uid-1,type=Warning"; got != want {
					t.Errorf("field selector = %q, want %q", got, want)
				}
				return true, &corev1.EventList{Items: tt.events}, nil
			})
			reason, message := latestWarningEvent(t.Context(), kubeclient, pod)
			if reason != tt.wantReason || message != tt.wantMsg {
				t.Errorf("latestWarningEvent() = (%q, %q), want (%q, %q)", reason, message, tt.wantReason, tt.wantMsg)
			}
		})
	}
}

func TestLatestWarningEventDoesNotListWithoutPodUID(t *testing.T) {
	kubeclient := fakek8s.NewSimpleClientset()
	reason, message := latestWarningEvent(t.Context(), kubeclient, &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod", Namespace: "ns"}})
	if reason != "" || message != "" {
		t.Errorf("latestWarningEvent() = (%q, %q), want empty result", reason, message)
	}
	if got := len(kubeclient.Actions()); got != 0 {
		t.Errorf("event list calls = %d, want 0", got)
	}
}

func TestLatestWarningEventDefensivelyFiltersResults(t *testing.T) {
	const podUID = types.UID("uid-1")
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod", Namespace: "ns", UID: podUID}}
	kubeclient := fakek8s.NewSimpleClientset()
	kubeclient.PrependReactor("list", "events", func(action k8stesting.Action) (bool, runtime.Object, error) {
		return true, &corev1.EventList{Items: []corev1.Event{
			{ObjectMeta: metav1.ObjectMeta{Name: "other-pod"}, InvolvedObject: corev1.ObjectReference{UID: "other-uid"}, Type: corev1.EventTypeWarning, Reason: "WrongPod", Message: "ignore"},
			{ObjectMeta: metav1.ObjectMeta{Name: "normal"}, InvolvedObject: corev1.ObjectReference{UID: podUID}, Type: corev1.EventTypeNormal, Reason: "Normal", Message: "ignore"},
			{ObjectMeta: metav1.ObjectMeta{Name: "warning"}, InvolvedObject: corev1.ObjectReference{UID: podUID}, Type: corev1.EventTypeWarning, Reason: "FailedMount", Message: "secret missing"},
		}}, nil
	})

	reason, message := latestWarningEvent(t.Context(), kubeclient, pod)
	if reason != "FailedMount" || message != "secret missing" {
		t.Errorf("latestWarningEvent() = (%q, %q), want (%q, %q)", reason, message, "FailedMount", "secret missing")
	}
}

func TestIsGenericPendingRequiresNoUsefulPodDiagnosis(t *testing.T) {
	waiting := func(reason, message string) corev1.ContainerStatus {
		return corev1.ContainerStatus{State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: reason, Message: message}}}
	}
	for _, tt := range []struct {
		name string
		pod  *corev1.Pod
		want bool
	}{
		{name: "bare ContainerCreating is generic", pod: &corev1.Pod{Status: corev1.PodStatus{ContainerStatuses: []corev1.ContainerStatus{waiting(containerWaitingReasonCreating, "")}}}, want: true},
		{name: "useful regular container prevents fallback", pod: &corev1.Pod{Status: corev1.PodStatus{ContainerStatuses: []corev1.ContainerStatus{waiting(containerWaitingReasonCreating, ""), waiting("ImagePullBackOff", "image unavailable")}}}, want: false},
		{name: "useful init container prevents fallback", pod: &corev1.Pod{Status: corev1.PodStatus{ContainerStatuses: []corev1.ContainerStatus{waiting(containerWaitingReasonCreating, "")}, InitContainerStatuses: []corev1.ContainerStatus{waiting("ErrImagePull", "image unavailable")}}}, want: false},
		{name: "pod message prevents fallback", pod: &corev1.Pod{Status: corev1.PodStatus{ContainerStatuses: []corev1.ContainerStatus{waiting(containerWaitingReasonCreating, "")}, Message: "scheduling failed"}}, want: false},
		{name: "meaningful pod condition prevents fallback", pod: &corev1.Pod{Status: corev1.PodStatus{ContainerStatuses: []corev1.ContainerStatus{waiting(containerWaitingReasonCreating, "")}, Conditions: []corev1.PodCondition{{Type: corev1.PodScheduled, Status: corev1.ConditionFalse, Reason: "Unschedulable", Message: "no matching nodes"}}}}, want: false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := isGenericPending(tt.pod); got != tt.want {
				t.Errorf("isGenericPending() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLatestWarningEventKeepsUTF8AndBoundsPersistedDiagnostic(t *testing.T) {
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod", Namespace: "ns", UID: "uid-1"}}
	kubeclient := fakek8s.NewSimpleClientset()
	kubeclient.PrependReactor("list", "events", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, &corev1.EventList{Items: []corev1.Event{{InvolvedObject: corev1.ObjectReference{UID: pod.UID}, Type: corev1.EventTypeWarning, Reason: "FailedMount", Message: strings.Repeat("界", maxEventMessageLength)}}}, nil
	})
	reason, message := latestWarningEvent(t.Context(), kubeclient, pod)
	diagnostic := "Last observed Pod warning: " + reason + ": " + message
	if !utf8.ValidString(diagnostic) {
		t.Error("persisted diagnostic is not valid UTF-8")
	}
	if len(diagnostic) > maxEventMessageLength {
		t.Errorf("persisted diagnostic length = %d, want at most %d", len(diagnostic), maxEventMessageLength)
	}
}
