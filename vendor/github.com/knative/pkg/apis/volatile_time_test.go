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

package apis

import (
	"encoding/json"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type testType struct {
	LastTransitionTime VolatileTime `json:"lastTransitionTime"`
}

func TestVolatileSerializationEmpty(t *testing.T) {
	tt := testType{}

	b, err := json.Marshal(tt)
	if err != nil {
		t.Errorf("Marshal() = %v", err)
	}

	if got, want := string(b), `{"lastTransitionTime":null}`; got != want {
		t.Errorf("Marshal() = %v, wanted %v", got, want)
	}

	got := testType{}
	if err := json.Unmarshal(b, &got); err != nil {
		t.Errorf("Unmarshal() = %v", err)
	}
	if got != tt {
		t.Errorf("Unmarshal() = %v, wanted %v", got, tt)
	}
}

func TestVolatileSerializationNow(t *testing.T) {
	tt := testType{
		LastTransitionTime: VolatileTime{metav1.NewTime(time.Unix(1024, 0))},
	}

	b, err := json.Marshal(tt)
	if err != nil {
		t.Errorf("Marshal() = %v", err)
	}

	if got, want := string(b), `{"lastTransitionTime":"1970-01-01T00:17:04Z"}`; got != want {
		t.Errorf("Marshal() = %v, wanted %v", got, want)
	}

	got := testType{}
	if err := json.Unmarshal(b, &got); err != nil {
		t.Errorf("Unmarshal() = %v", err)
	}
	if got != tt {
		t.Errorf("Unmarshal() = %v, wanted %v", got, tt)
	}
}

func TestVolatileTimeEquality(t *testing.T) {
	tt1 := testType{
		LastTransitionTime: VolatileTime{metav1.NewTime(time.Unix(1024, 36))},
	}
	tt2 := testType{
		LastTransitionTime: VolatileTime{metav1.NewTime(time.Unix(2048, 36))},
	}

	if !equality.Semantic.DeepEqual(tt1, tt2) {
		t.Error("equality.Semantic.DeepEqual() = false, wanted true")
	}
}
