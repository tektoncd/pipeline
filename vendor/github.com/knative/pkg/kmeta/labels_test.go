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

package kmeta

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestMakeVersionLabels(t *testing.T) {
	tests := []struct {
		name string
		om   metav1.ObjectMeta
		s    string
	}{{
		name: "simple translation",
		om: metav1.ObjectMeta{
			UID:             "1234",
			ResourceVersion: "abcd",
		},
		s: "controller=1234,version=abcd",
	}, {
		name: "another simple translation",
		om: metav1.ObjectMeta{
			UID:             "abcd",
			ResourceVersion: "1234",
		},
		s: "controller=abcd,version=1234",
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ls := MakeVersionLabels(&test.om)
			if want, got := test.s, ls.String(); got != want {
				t.Errorf("MakeVersionLabels() = %v, wanted %v", got, want)
			}
		})
	}
}

func TestMakeVersionLabelSelector(t *testing.T) {
	tests := []struct {
		name string
		om   metav1.ObjectMeta
		s    string
	}{{
		name: "simple translation",
		om: metav1.ObjectMeta{
			UID:             "1234",
			ResourceVersion: "abcd",
		},
		s: "controller=1234,version=abcd",
	}, {
		name: "another simple translation",
		om: metav1.ObjectMeta{
			UID:             "abcd",
			ResourceVersion: "1234",
		},
		s: "controller=abcd,version=1234",
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ls := MakeVersionLabelSelector(&test.om)
			if want, got := test.s, ls.String(); got != want {
				t.Errorf("MakeVersionLabelSelector() = %v, wanted %v", got, want)
			}
		})
	}
}

func TestMakeOldVersionLabelSelector(t *testing.T) {
	tests := []struct {
		name string
		om   metav1.ObjectMeta
		s    string
	}{{
		name: "simple translation",
		om: metav1.ObjectMeta{
			UID:             "1234",
			ResourceVersion: "abcd",
		},
		s: "controller=1234,version!=abcd",
	}, {
		name: "another simple translation",
		om: metav1.ObjectMeta{
			UID:             "abcd",
			ResourceVersion: "1234",
		},
		s: "controller=abcd,version!=1234",
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ls := MakeOldVersionLabelSelector(&test.om)
			if want, got := test.s, ls.String(); got != want {
				t.Errorf("MakeOldVersionLabelSelector() = %v, wanted %v", got, want)
			}
		})
	}
}
