/*
Copyright 2017 The Knative Authors

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

	"encoding/json"

	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/knative/pkg/apis"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestIsTrue(t *testing.T) {
	cases := []struct {
		name      string
		condition *Condition
		truth     bool
	}{{
		name:      "empty should be false",
		condition: &Condition{},
		truth:     false,
	}, {
		name: "True should be true",
		condition: &Condition{
			Status: corev1.ConditionTrue,
		},
		truth: true,
	}, {
		name: "False should be false",
		condition: &Condition{
			Status: corev1.ConditionFalse,
		},
		truth: false,
	}, {
		name: "Unknown should be false",
		condition: &Condition{
			Status: corev1.ConditionUnknown,
		},
		truth: false,
	}, {
		name:      "Nil should be false",
		condition: nil,
		truth:     false,
	}}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if e, a := tc.truth, tc.condition.IsTrue(); e != a {
				t.Errorf("%q expected: %v got: %v", tc.name, e, a)
			}
		})
	}
}

func TestIsFalse(t *testing.T) {
	cases := []struct {
		name      string
		condition *Condition
		truth     bool
	}{{
		name:      "empty should be false",
		condition: &Condition{},
		truth:     false,
	}, {
		name: "True should be false",
		condition: &Condition{
			Status: corev1.ConditionTrue,
		},
		truth: false,
	}, {
		name: "False should be true",
		condition: &Condition{
			Status: corev1.ConditionFalse,
		},
		truth: true,
	}, {
		name: "Unknown should be false",
		condition: &Condition{
			Status: corev1.ConditionUnknown,
		},
		truth: false,
	}, {
		name:      "Nil should be false",
		condition: nil,
		truth:     false,
	}}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if e, a := tc.truth, tc.condition.IsFalse(); e != a {
				t.Errorf("%q expected: %v got: %v", tc.name, e, a)
			}
		})
	}
}

func TestIsUnknown(t *testing.T) {
	cases := []struct {
		name      string
		condition *Condition
		truth     bool
	}{{
		name:      "empty should be false",
		condition: &Condition{},
		truth:     false,
	}, {
		name: "True should be false",
		condition: &Condition{
			Status: corev1.ConditionTrue,
		},
		truth: false,
	}, {
		name: "False should be false",
		condition: &Condition{
			Status: corev1.ConditionFalse,
		},
		truth: false,
	}, {
		name: "Unknown should be true",
		condition: &Condition{
			Status: corev1.ConditionUnknown,
		},
		truth: true,
	}, {
		name:      "Nil should be true",
		condition: nil,
		truth:     true,
	}}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if e, a := tc.truth, tc.condition.IsUnknown(); e != a {
				t.Errorf("%q expected: %v got: %v", tc.name, e, a)
			}
		})
	}
}

func TestJson(t *testing.T) {
	cases := []struct {
		name      string
		raw       string
		condition *Condition
	}{{
		name:      "empty",
		raw:       `{}`,
		condition: &Condition{},
	}, {
		name: "Type",
		raw:  `{"type":"Foo"}`,
		condition: &Condition{
			Type: "Foo",
		},
	}, {
		name: "Status",
		raw:  `{"status":"True"}`,
		condition: &Condition{
			Status: corev1.ConditionTrue,
		},
	}, {
		name: "LastTransitionTime",
		raw:  `{"lastTransitionTime":"1984-02-28T18:52:00Z"}`,
		condition: &Condition{
			LastTransitionTime: apis.VolatileTime{Inner: metav1.NewTime(time.Date(1984, 02, 28, 18, 52, 00, 00, time.UTC))},
		},
	}, {
		name: "Reason",
		raw:  `{"reason":"DatTest"}`,
		condition: &Condition{
			Reason: "DatTest",
		},
	}, {
		name: "Message",
		raw:  `{"message":"this is just a test"}`,
		condition: &Condition{
			Message: "this is just a test",
		},
	}, {
		name: "all",
		raw: `{
				"type":"Foo", 
				"status":"True", 
				"lastTransitionTime":"1984-02-28T18:52:00Z",
				"reason":"DatTest",
				"message":"this is just a test"
		}`,
		condition: &Condition{
			Type:               "Foo",
			Status:             corev1.ConditionTrue,
			LastTransitionTime: apis.VolatileTime{Inner: metav1.NewTime(time.Date(1984, 02, 28, 18, 52, 00, 00, time.UTC))},
			Reason:             "DatTest",
			Message:            "this is just a test",
		},
	}}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cond := &Condition{}
			if err := json.Unmarshal([]byte(tc.raw), cond); err != nil {
				t.Errorf("%q unexpected error from json.Unmarshal: %v", tc.name, err)
			}
			if diff := cmp.Diff(tc.condition, cond); diff != "" {
				t.Errorf("%q unexpected diff (-want +got): %s", tc.name, diff)
			}

		})
	}
}
