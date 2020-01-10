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
package termination

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	v1alpha1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestParseMessages(t *testing.T) {
	t1, t2 := time.Now(), time.Now().Add(time.Hour)
	t1s, t2s := t1.Format(time.RFC3339Nano), t2.Format(time.RFC3339Nano)
	for _, c := range []struct {
		desc   string
		status corev1.PodStatus
		want   *TerminationDetails
	}{{
		desc: "valid single message",
		status: corev1.PodStatus{
			ContainerStatuses: []corev1.ContainerStatus{{
				State: corev1.ContainerState{
					Terminated: &corev1.ContainerStateTerminated{
						Message: `[{"key":"StartedAt","value":"` + t1s + `"},{"key":"foo","value":"bar"},{"key":"bar","value":"baz"}]`,
					},
				},
			}},
		},
		want: &TerminationDetails{
			StartTimes: []metav1.Time{
				metav1.NewTime(t1),
			},
			ResourceResults: []v1alpha1.PipelineResourceResult{{
				Key:   "bar",
				Value: "baz",
			}, { // sorted by key.
				Key:   "foo",
				Value: "bar",
			}},
		},
	}, {
		desc: "two messages, last value wins",
		status: corev1.PodStatus{
			ContainerStatuses: []corev1.ContainerStatus{{
				State: corev1.ContainerState{
					Terminated: &corev1.ContainerStateTerminated{
						Message: `[{"key":"StartedAt","value":"` + t1s + `"},{"key":"foo","value":"bar"},{"key":"bar","value":"baz"}]`,
					},
				},
			}, {
				State: corev1.ContainerState{
					Terminated: &corev1.ContainerStateTerminated{
						Message: `[{"key":"StartedAt","value":"` + t2s + `"},{"key":"foo","value":"newbar"},{"key":"bar","value":"newbaz"}]`,
					},
				},
			}},
		},
		want: &TerminationDetails{
			StartTimes: []metav1.Time{
				metav1.NewTime(t1),
				metav1.NewTime(t2),
			},
			ResourceResults: []v1alpha1.PipelineResourceResult{{
				Key:   "bar",
				Value: "newbaz",
			}, { // sorted by key.
				Key:   "foo",
				Value: "newbar",
			}},
		},
	}, {
		desc: "empty message",
		status: corev1.PodStatus{
			ContainerStatuses: []corev1.ContainerStatus{{
				State: corev1.ContainerState{
					Terminated: &corev1.ContainerStateTerminated{
						Message: "",
					},
				},
			}},
		},
		want: &TerminationDetails{},
	}, {
		desc: "duplicate keys same status",
		status: corev1.PodStatus{
			ContainerStatuses: []corev1.ContainerStatus{{
				State: corev1.ContainerState{
					Terminated: &corev1.ContainerStateTerminated{
						Message: `[
						{"key":"foo","value":"first"},
						{"key":"foo","value":"middle"},
						{"key":"foo","value":"last"}]`,
					},
				},
			}},
		},
		want: &TerminationDetails{
			ResourceResults: []v1alpha1.PipelineResourceResult{{
				Key:   "foo",
				Value: "last",
			}},
		},
	}, {
		desc: "sorted by key",
		status: corev1.PodStatus{
			ContainerStatuses: []corev1.ContainerStatus{{
				State: corev1.ContainerState{
					Terminated: &corev1.ContainerStateTerminated{
						Message: `[
						{"key":"zzz","value":"last"},
						{"key":"ddd","value":"middle"},
						{"key":"aaa","value":"first"}]`,
					},
				},
			}},
		},
		want: &TerminationDetails{
			ResourceResults: []v1alpha1.PipelineResourceResult{{
				Key:   "aaa",
				Value: "first",
			}, {
				Key:   "ddd",
				Value: "middle",
			}, {
				Key:   "zzz",
				Value: "last",
			}},
		},
	}} {
		t.Run(c.desc, func(t *testing.T) {
			got, err := ParseMessages(c.status)
			if err != nil {
				t.Fatalf("ParseMessages: %v", err)
			}
			if d := cmp.Diff(c.want, got); d != "" {
				t.Fatalf("ParseMessages(-want,+got): %s", d)
			}
		})
	}
}

func TestParseMessages_Invalid(t *testing.T) {
	for _, c := range []struct {
		desc   string
		status corev1.PodStatus
	}{{
		desc: "invalid json",
		status: corev1.PodStatus{
			ContainerStatuses: []corev1.ContainerStatus{{
				State: corev1.ContainerState{
					Terminated: &corev1.ContainerStateTerminated{
						Message: "INVALID NOT JSON",
					},
				},
			}},
		},
	}, {
		desc: "invalid start time",
		status: corev1.PodStatus{
			ContainerStatuses: []corev1.ContainerStatus{{
				State: corev1.ContainerState{
					Terminated: &corev1.ContainerStateTerminated{
						Message: `[{"key":"StartedAt":"INVALID NOT TIME"}]`,
					},
				},
			}},
		},
	}} {
		t.Run(c.desc, func(t *testing.T) {
			if got, err := ParseMessages(c.status); err == nil {
				t.Errorf("Expected error, got nil error and result: %v", got)
			}
		})
	}
}
