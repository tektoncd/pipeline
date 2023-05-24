/*
Copyright 2020 The Tekton Authors

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

package v1beta1_test

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	apisconfig "github.com/tektoncd/pipeline/pkg/apis/config"
	v1beta1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/pkg/client/clientset/versioned/scheme"
	"github.com/tektoncd/pipeline/test/diff"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

func TestGetParams(t *testing.T) {
	for _, c := range []struct {
		desc string
		spec v1beta1.CustomRunSpec
		name string
		want *v1beta1.Param
	}{{
		desc: "no params",
		spec: v1beta1.CustomRunSpec{},
		name: "anything",
		want: nil,
	}, {
		desc: "found",
		spec: v1beta1.CustomRunSpec{
			Params: v1beta1.Params{{
				Name:  "first",
				Value: *v1beta1.NewStructuredValues("blah"),
			}, {
				Name:  "foo",
				Value: *v1beta1.NewStructuredValues("bar"),
			}},
		},
		name: "foo",
		want: &v1beta1.Param{
			Name:  "foo",
			Value: *v1beta1.NewStructuredValues("bar"),
		},
	}, {
		desc: "not found",
		spec: v1beta1.CustomRunSpec{
			Params: v1beta1.Params{{
				Name:  "first",
				Value: *v1beta1.NewStructuredValues("blah"),
			}, {
				Name:  "foo",
				Value: *v1beta1.NewStructuredValues("bar"),
			}},
		},
		name: "bar",
		want: nil,
	}, {
		// This shouldn't happen since it's invalid, but just in
		// case, GetParams just returns the first param it finds with
		// the specified name.
		desc: "multiple with same name",
		spec: v1beta1.CustomRunSpec{
			Params: v1beta1.Params{{
				Name:  "first",
				Value: *v1beta1.NewStructuredValues("blah"),
			}, {
				Name:  "foo",
				Value: *v1beta1.NewStructuredValues("bar"),
			}, {
				Name:  "foo",
				Value: *v1beta1.NewStructuredValues("second bar"),
			}},
		},
		name: "foo",
		want: &v1beta1.Param{
			Name:  "foo",
			Value: *v1beta1.NewStructuredValues("bar"),
		},
	}} {
		t.Run(c.desc, func(t *testing.T) {
			got := c.spec.GetParam(c.name)
			if d := cmp.Diff(c.want, got); d != "" {
				t.Fatalf("Diff(-want,+got): %v", d)
			}
		})
	}
}

func TestRunHasStarted(t *testing.T) {
	params := []struct {
		name          string
		runStatus     v1beta1.CustomRunStatus
		expectedValue bool
	}{{
		name:          "runWithNoStartTime",
		runStatus:     v1beta1.CustomRunStatus{},
		expectedValue: false,
	}, {
		name: "runWithStartTime",
		runStatus: v1beta1.CustomRunStatus{
			CustomRunStatusFields: v1beta1.CustomRunStatusFields{
				StartTime: &metav1.Time{Time: now},
			},
		},
		expectedValue: true,
	}, {
		name: "runWithZeroStartTime",
		runStatus: v1beta1.CustomRunStatus{
			CustomRunStatusFields: v1beta1.CustomRunStatusFields{
				StartTime: &metav1.Time{},
			},
		},
		expectedValue: false,
	}}
	for _, tc := range params {
		t.Run(tc.name, func(t *testing.T) {
			run := v1beta1.CustomRun{}
			run.Status = tc.runStatus
			if run.HasStarted() != tc.expectedValue {
				t.Fatalf("Expected run HasStarted() to return %t but got %t", tc.expectedValue, run.HasStarted())
			}
		})
	}
}

func TestRunIsDone(t *testing.T) {
	customRun := v1beta1.CustomRun{
		Status: v1beta1.CustomRunStatus{
			Status: duckv1.Status{
				Conditions: duckv1.Conditions{{
					Type:   apis.ConditionSucceeded,
					Status: corev1.ConditionFalse,
				}},
			},
		},
	}

	if !customRun.IsDone() {
		t.Fatal("Expected run status to be done")
	}
}

func TestCustomRunIsSuccessful(t *testing.T) {
	tcs := []struct {
		name      string
		customRun *v1beta1.CustomRun
		want      bool
	}{{
		name: "nil taskrun",
		want: false,
	}, {
		name: "still running",
		customRun: &v1beta1.CustomRun{Status: v1beta1.CustomRunStatus{Status: duckv1.Status{Conditions: []apis.Condition{{
			Type:   apis.ConditionSucceeded,
			Status: corev1.ConditionUnknown,
		}}}}},
		want: false,
	}, {
		name: "succeeded",
		customRun: &v1beta1.CustomRun{Status: v1beta1.CustomRunStatus{Status: duckv1.Status{Conditions: []apis.Condition{{
			Type:   apis.ConditionSucceeded,
			Status: corev1.ConditionTrue,
		}}}}},
		want: true,
	}, {
		name: "failed",
		customRun: &v1beta1.CustomRun{Status: v1beta1.CustomRunStatus{Status: duckv1.Status{Conditions: []apis.Condition{{
			Type:   apis.ConditionSucceeded,
			Status: corev1.ConditionFalse,
		}}}}},
		want: false,
	}}
	for _, tc := range tcs {
		got := tc.customRun.IsSuccessful()
		if tc.want != got {
			t.Errorf("wanted isSuccessful to be %t but was %t", tc.want, got)
		}
	}
}

func TestCustomRunIsFailure(t *testing.T) {
	tcs := []struct {
		name    string
		taskRun *v1beta1.TaskRun
		want    bool
	}{{
		name: "nil taskrun",
		want: false,
	}, {
		name: "still running",
		taskRun: &v1beta1.TaskRun{Status: v1beta1.TaskRunStatus{Status: duckv1.Status{Conditions: []apis.Condition{{
			Type:   apis.ConditionSucceeded,
			Status: corev1.ConditionUnknown,
		}}}}},
		want: false,
	}, {
		name: "succeeded",
		taskRun: &v1beta1.TaskRun{Status: v1beta1.TaskRunStatus{Status: duckv1.Status{Conditions: []apis.Condition{{
			Type:   apis.ConditionSucceeded,
			Status: corev1.ConditionTrue,
		}}}}},
		want: false,
	}, {
		name: "failed",
		taskRun: &v1beta1.TaskRun{Status: v1beta1.TaskRunStatus{Status: duckv1.Status{Conditions: []apis.Condition{{
			Type:   apis.ConditionSucceeded,
			Status: corev1.ConditionFalse,
		}}}}},
		want: true,
	}}
	for _, tc := range tcs {
		got := tc.taskRun.IsFailure()
		if tc.want != got {
			t.Errorf("wanted isFailure to be %t but was %t", tc.want, got)
		}
	}
}

func TestRunIsCancelled(t *testing.T) {
	customRun := v1beta1.CustomRun{
		Spec: v1beta1.CustomRunSpec{
			Status: v1beta1.CustomRunSpecStatusCancelled,
		},
	}
	if !customRun.IsCancelled() {
		t.Fatal("Expected run status to be cancelled")
	}
	expected := ""
	if string(customRun.Spec.StatusMessage) != expected {
		t.Fatalf("Expected StatusMessage is %s but got %s", expected, customRun.Spec.StatusMessage)
	}
}

func TestRunIsCancelledWithMessage(t *testing.T) {
	expectedStatusMessage := "test message"
	customRun := v1beta1.CustomRun{
		Spec: v1beta1.CustomRunSpec{
			Status:        v1beta1.CustomRunSpecStatusCancelled,
			StatusMessage: v1beta1.CustomRunSpecStatusMessage(expectedStatusMessage),
		},
	}
	if !customRun.IsCancelled() {
		t.Fatal("Expected run status to be cancelled")
	}

	if string(customRun.Spec.StatusMessage) != expectedStatusMessage {
		t.Fatalf("Expected StatusMessage is %s but got %s", expectedStatusMessage, customRun.Spec.StatusMessage)
	}
}

// TestCustomRunStatus tests that extraFields in a CustomRunStatus can be parsed
// from YAML.
func TestCustomRunStatus(t *testing.T) {
	in := `apiVersion: tekton.dev/v1beta1
kind: CustomRun
metadata:
  name: run
spec:
  retries: 3
  customRef:
    apiVersion: example.dev/v0
    kind: Example
status:
  conditions:
  - type: "Succeeded"
    status: "True"
  results:
  - name: foo
    value: bar
  extraFields:
    simple: 'hello'
    complex:
      hello: ['w', 'o', 'r', 'l', 'd']
`
	var r v1beta1.CustomRun
	if _, _, err := scheme.Codecs.UniversalDeserializer().Decode([]byte(in), nil, &r); err != nil {
		t.Fatalf("Decode YAML: %v", err)
	}

	want := &v1beta1.CustomRun{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "tekton.dev/v1beta1",
			Kind:       "CustomRun",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "run",
		},
		Spec: v1beta1.CustomRunSpec{
			Retries: 3,
			CustomRef: &v1beta1.TaskRef{
				APIVersion: "example.dev/v0",
				Kind:       "Example",
			},
		},
		Status: v1beta1.CustomRunStatus{
			Status: duckv1.Status{
				Conditions: duckv1.Conditions{{
					Type:   apis.ConditionSucceeded,
					Status: corev1.ConditionTrue,
				}},
			},
			CustomRunStatusFields: v1beta1.CustomRunStatusFields{
				// Results are parsed correctly.
				Results: []v1beta1.CustomRunResult{{
					Name:  "foo",
					Value: "bar",
				}},
				// Any extra fields are simply stored as JSON bytes.
				ExtraFields: runtime.RawExtension{
					Raw: []byte(`{"complex":{"hello":["w","o","r","l","d"]},"simple":"hello"}`),
				},
			},
		},
	}
	if d := cmp.Diff(want, &r); d != "" {
		t.Fatalf("Diff(-want,+got): %s", d)
	}
}

func TestEncodeDecodeExtraFields(t *testing.T) {
	type Mystatus struct {
		S string
		I int
	}
	status := Mystatus{S: "one", I: 1}
	r := &v1beta1.CustomRunStatus{}
	if err := r.EncodeExtraFields(&status); err != nil {
		t.Fatalf("EncodeExtraFields failed: %s", err)
	}
	newStatus := Mystatus{}
	if err := r.DecodeExtraFields(&newStatus); err != nil {
		t.Fatalf("DecodeExtraFields failed: %s", err)
	}
	if d := cmp.Diff(status, newStatus); d != "" {
		t.Fatalf("Diff(-want,+got): %s", d)
	}
}

func TestRunGetTimeOut(t *testing.T) {
	testCases := []struct {
		name          string
		customRun     v1beta1.CustomRun
		expectedValue time.Duration
	}{{
		name:          "runWithNoTimeout",
		customRun:     v1beta1.CustomRun{},
		expectedValue: apisconfig.DefaultTimeoutMinutes * time.Minute,
	}, {
		name: "runWithTimeout",
		customRun: v1beta1.CustomRun{
			TypeMeta: metav1.TypeMeta{Kind: "kind", APIVersion: "apiVersion"},
			Spec:     v1beta1.CustomRunSpec{Timeout: &metav1.Duration{Duration: 10 * time.Second}},
		},
		expectedValue: 10 * time.Second,
	}}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.customRun.GetTimeout()
			if d := cmp.Diff(result, tc.expectedValue); d != "" {
				t.Fatalf(diff.PrintWantGot(d))
			}
		})
	}
}

func TestRunHasTimedOut(t *testing.T) {
	testCases := []struct {
		name          string
		customRun     v1beta1.CustomRun
		expectedValue bool
	}{{
		name:          "runWithNoStartTimeNoTimeout",
		customRun:     v1beta1.CustomRun{},
		expectedValue: false,
	}, {
		name: "runWithStartTimeNoTimeout",
		customRun: v1beta1.CustomRun{
			TypeMeta: metav1.TypeMeta{Kind: "kind", APIVersion: "apiVersion"},
			Status: v1beta1.CustomRunStatus{
				CustomRunStatusFields: v1beta1.CustomRunStatusFields{
					StartTime: &metav1.Time{Time: now},
				},
			}},
		expectedValue: false,
	}, {
		name: "runWithStartTimeNoTimeout2",
		customRun: v1beta1.CustomRun{
			TypeMeta: metav1.TypeMeta{Kind: "kind", APIVersion: "apiVersion"},
			Status: v1beta1.CustomRunStatus{
				CustomRunStatusFields: v1beta1.CustomRunStatusFields{
					StartTime: &metav1.Time{Time: now.Add(-1 * (apisconfig.DefaultTimeoutMinutes + 1) * time.Minute)},
				},
			}},
		expectedValue: true,
	}, {
		name: "runWithStartTimeAndTimeout",
		customRun: v1beta1.CustomRun{
			TypeMeta: metav1.TypeMeta{Kind: "kind", APIVersion: "apiVersion"},
			Spec:     v1beta1.CustomRunSpec{Timeout: &metav1.Duration{Duration: 10 * time.Second}},
			Status: v1beta1.CustomRunStatus{CustomRunStatusFields: v1beta1.CustomRunStatusFields{
				StartTime: &metav1.Time{Time: now.Add(-1 * (apisconfig.DefaultTimeoutMinutes + 1) * time.Minute)},
			}}},
		expectedValue: true,
	}, {
		name: "runWithNoStartTimeAndTimeout",
		customRun: v1beta1.CustomRun{
			TypeMeta: metav1.TypeMeta{Kind: "kind", APIVersion: "apiVersion"},
			Spec:     v1beta1.CustomRunSpec{Timeout: &metav1.Duration{Duration: time.Second}},
		},
		expectedValue: false,
	}, {
		name: "runWithStartTimeAndTimeout2",
		customRun: v1beta1.CustomRun{
			TypeMeta: metav1.TypeMeta{Kind: "kind", APIVersion: "apiVersion"},
			Spec:     v1beta1.CustomRunSpec{Timeout: &metav1.Duration{Duration: 10 * time.Second}},
			Status: v1beta1.CustomRunStatus{CustomRunStatusFields: v1beta1.CustomRunStatusFields{
				StartTime: &metav1.Time{Time: now},
			}}},
		expectedValue: false,
	}}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.customRun.HasTimedOut(testClock)
			if d := cmp.Diff(result, tc.expectedValue); d != "" {
				t.Fatalf(diff.PrintWantGot(d))
			}
		})
	}
}

func TestCustomRun_GetRetryCount(t *testing.T) {
	testCases := []struct {
		name      string
		customRun *v1beta1.CustomRun
		count     int
	}{
		{
			name: "no retries",
			customRun: &v1beta1.CustomRun{
				Status: v1beta1.CustomRunStatus{
					CustomRunStatusFields: v1beta1.CustomRunStatusFields{},
				},
			},
			count: 0,
		}, {
			name: "one retry",
			customRun: &v1beta1.CustomRun{
				Status: v1beta1.CustomRunStatus{
					CustomRunStatusFields: v1beta1.CustomRunStatusFields{
						RetriesStatus: []v1beta1.CustomRunStatus{{}},
					},
				},
			},
			count: 1,
		}, {
			name: "two retries",
			customRun: &v1beta1.CustomRun{
				Status: v1beta1.CustomRunStatus{
					CustomRunStatusFields: v1beta1.CustomRunStatusFields{
						RetriesStatus: []v1beta1.CustomRunStatus{{}, {}},
					},
				},
			},
			count: 2,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			seenCount := tc.customRun.GetRetryCount()
			if seenCount != tc.count {
				t.Errorf("expected %d retries, but got %d", tc.count, seenCount)
			}
		})
	}
}
