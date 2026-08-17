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

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	fakek8s "k8s.io/client-go/kubernetes/fake"
)

func TestLatestWarningEvent(t *testing.T) {
	now := metav1.Now()
	earlier := metav1.NewTime(now.Add(-5 * time.Minute))

	tests := []struct {
		name       string
		pod        *corev1.Pod
		events     []corev1.Event
		wantReason string
		wantMsg    string
	}{
		{
			name: "returns most recent warning event",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pod", Namespace: "ns", UID: types.UID("uid-1"),
				},
			},
			events: []corev1.Event{
				{
					ObjectMeta:     metav1.ObjectMeta{Name: "old-evt", Namespace: "ns"},
					InvolvedObject: corev1.ObjectReference{UID: "uid-1"},
					Type:           "Warning",
					Reason:         "FailedScheduling",
					Message:        "old scheduling error",
					LastTimestamp:  earlier,
				},
				{
					ObjectMeta:     metav1.ObjectMeta{Name: "new-evt", Namespace: "ns"},
					InvolvedObject: corev1.ObjectReference{UID: "uid-1"},
					Type:           "Warning",
					Reason:         "FailedMount",
					Message:        `secret "my-secret" not found`,
					LastTimestamp:  now,
				},
			},
			wantReason: "FailedMount",
			wantMsg:    `secret "my-secret" not found`,
		},
		{
			name: "returns empty when no events exist",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pod", Namespace: "ns", UID: types.UID("uid-2"),
				},
			},
			events:     nil,
			wantReason: "",
			wantMsg:    "",
		},
		{
			name: "returns empty when pod has no UID",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pod", Namespace: "ns",
				},
			},
			events:     nil,
			wantReason: "",
			wantMsg:    "",
		},
		{
			name: "truncates long messages",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pod", Namespace: "ns", UID: types.UID("uid-3"),
				},
			},
			events: []corev1.Event{{
				ObjectMeta:     metav1.ObjectMeta{Name: "long-evt", Namespace: "ns"},
				InvolvedObject: corev1.ObjectReference{UID: "uid-3"},
				Type:           "Warning",
				Reason:         "FailedMount",
				Message:        strings.Repeat("x", 2000),
				LastTimestamp:  now,
			}},
			wantReason: "FailedMount",
			wantMsg:    strings.Repeat("x", maxEventMessageLength-len(truncationSuffix)) + truncationSuffix,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kubeclient := fakek8s.NewSimpleClientset()
			for i := range tt.events {
				_, _ = kubeclient.CoreV1().Events(tt.events[i].Namespace).Create(t.Context(), &tt.events[i], metav1.CreateOptions{})
			}

			reason, msg := latestWarningEvent(t.Context(), kubeclient, tt.pod)
			if reason != tt.wantReason {
				t.Errorf("reason = %q, want %q", reason, tt.wantReason)
			}
			if msg != tt.wantMsg {
				t.Errorf("message = %q, want %q", msg, tt.wantMsg)
			}
			if len(msg) > maxEventMessageLength {
				t.Errorf("message length = %d, want at most %d", len(msg), maxEventMessageLength)
			}
		})
	}
}

func TestIsGenericPending(t *testing.T) {
	tests := []struct {
		name string
		pod  *corev1.Pod
		want bool
	}{
		{
			name: "ContainerCreating with empty message is generic",
			pod: &corev1.Pod{
				Status: corev1.PodStatus{
					ContainerStatuses: []corev1.ContainerStatus{{
						State: corev1.ContainerState{
							Waiting: &corev1.ContainerStateWaiting{
								Reason: "ContainerCreating",
							},
						},
					}},
				},
			},
			want: true,
		},
		{
			name: "ContainerCreating with a message is not generic",
			pod: &corev1.Pod{
				Status: corev1.PodStatus{
					ContainerStatuses: []corev1.ContainerStatus{{
						State: corev1.ContainerState{
							Waiting: &corev1.ContainerStateWaiting{
								Reason:  "ContainerCreating",
								Message: "something useful",
							},
						},
					}},
				},
			},
			want: false,
		},
		{
			name: "containers present but not waiting, all conditions true, no message is generic",
			pod: &corev1.Pod{
				Status: corev1.PodStatus{
					ContainerStatuses: []corev1.ContainerStatus{{
						Name:  "step-foo",
						State: corev1.ContainerState{},
					}},
					Conditions: []corev1.PodCondition{{
						Type:   corev1.PodScheduled,
						Status: corev1.ConditionTrue,
					}},
				},
			},
			want: true,
		},
		{
			name: "unschedulable pod is generic",
			pod: &corev1.Pod{
				Status: corev1.PodStatus{
					Conditions: []corev1.PodCondition{{
						Type:   corev1.PodScheduled,
						Status: corev1.ConditionFalse,
						Reason: "Unschedulable",
					}},
				},
			},
			want: true,
		},
		{
			name: "pod with no container statuses yet is generic",
			pod:  &corev1.Pod{},
			want: true,
		},
		{
			name: "ImagePullBackOff is not generic",
			pod: &corev1.Pod{
				Status: corev1.PodStatus{
					ContainerStatuses: []corev1.ContainerStatus{{
						State: corev1.ContainerState{
							Waiting: &corev1.ContainerStateWaiting{
								Reason:  "ImagePullBackOff",
								Message: "pull failed",
							},
						},
					}},
				},
			},
			want: false,
		},
		{
			name: "init container ContainerCreating with empty message is generic",
			pod: &corev1.Pod{
				Status: corev1.PodStatus{
					InitContainerStatuses: []corev1.ContainerStatus{{
						State: corev1.ContainerState{
							Waiting: &corev1.ContainerStateWaiting{
								Reason: "ContainerCreating",
							},
						},
					}},
				},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isGenericPending(tt.pod)
			if got != tt.want {
				t.Errorf("isGenericPending() = %v, want %v", got, tt.want)
			}
		})
	}
}
