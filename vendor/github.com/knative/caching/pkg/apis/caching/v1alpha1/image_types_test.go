/*
Copyright 2018 The Knative Authors

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
package v1alpha1

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/knative/pkg/apis"
)

func TestIsReady(t *testing.T) {
	cases := []struct {
		name    string
		status  ImageStatus
		isReady bool
	}{{
		name:    "empty status should not be ready",
		status:  ImageStatus{},
		isReady: false,
	}, {
		name: "Different condition type should not be ready",
		status: ImageStatus{
			Conditions: []ImageCondition{{
				Type:   "foo",
				Status: corev1.ConditionTrue,
			}},
		},
		isReady: false,
	}, {
		name: "False condition status should not be ready",
		status: ImageStatus{
			Conditions: []ImageCondition{{
				Type:   ImageConditionReady,
				Status: corev1.ConditionFalse,
			}},
		},
		isReady: false,
	}, {
		name: "Unknown condition status should not be ready",
		status: ImageStatus{
			Conditions: []ImageCondition{{
				Type:   ImageConditionReady,
				Status: corev1.ConditionUnknown,
			}},
		},
		isReady: false,
	}, {
		name: "Missing condition status should not be ready",
		status: ImageStatus{
			Conditions: []ImageCondition{{
				Type: ImageConditionReady,
			}},
		},
		isReady: false,
	}, {
		name: "True condition status should be ready",
		status: ImageStatus{
			Conditions: []ImageCondition{{
				Type:   ImageConditionReady,
				Status: corev1.ConditionTrue,
			}},
		},
		isReady: true,
	}, {
		name: "Multiple conditions with ready status should be ready",
		status: ImageStatus{
			Conditions: []ImageCondition{{
				Type:   "foo",
				Status: corev1.ConditionTrue,
			}, {
				Type:   ImageConditionReady,
				Status: corev1.ConditionTrue,
			}},
		},
		isReady: true,
	}, {
		name: "Multiple conditions with ready status false should not be ready",
		status: ImageStatus{
			Conditions: []ImageCondition{{
				Type:   "foo",
				Status: corev1.ConditionTrue,
			}, {
				Type:   ImageConditionReady,
				Status: corev1.ConditionFalse,
			}},
		},
		isReady: false,
	}}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if e, a := tc.isReady, tc.status.IsReady(); e != a {
				t.Errorf("%q expected: %v got: %v", tc.name, e, a)
			}
		})
	}
}

func TestImageConditions(t *testing.T) {
	rev := &Image{}
	foo := &ImageCondition{
		Type:   "Foo",
		Status: "True",
	}
	bar := &ImageCondition{
		Type:   "Bar",
		Status: "True",
	}

	// Add a new condition.
	rev.Status.SetCondition(foo)

	if got, want := len(rev.Status.Conditions), 1; got != want {
		t.Fatalf("Unexpected Condition length; got %d, want %d", got, want)
	}

	// Add nothing
	rev.Status.SetCondition(nil)

	if got, want := len(rev.Status.Conditions), 1; got != want {
		t.Fatalf("Unexpected Condition length; got %d, want %d", got, want)
	}

	// Add a second condition.
	rev.Status.SetCondition(bar)

	if got, want := len(rev.Status.Conditions), 2; got != want {
		t.Fatalf("Unexpected Condition length; got %d, want %d", got, want)
	}

	// Add nil condition.
	rev.Status.SetCondition(nil)

	if got, want := len(rev.Status.Conditions), 2; got != want {
		t.Fatalf("Unexpected Condition length; got %d, want %d", got, want)
	}

	// Add a condition that varies only in LastTransitionTime, and check that
	// things are not updated.
	bar = rev.Status.GetCondition("Bar")
	bar2 := bar.DeepCopy()
	bar2.LastTransitionTime = apis.VolatileTime{metav1.NewTime(time.Unix(1234, 0))}
	rev.Status.SetCondition(bar2)

	got, want := rev.Status.GetCondition("Bar"), bar
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("Unexpected traffic diff (-want +got): %v", diff)
	}
}
