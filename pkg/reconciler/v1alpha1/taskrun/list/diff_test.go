/*
Copyright 2018 The Knative Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either extress or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package list

import (
	"reflect"
	"testing"
)

func TestIsSame_same(t *testing.T) {
	required := []string{"elsa", "anna", "olaf", "kristoff"}
	provided := []string{"elsa", "anna", "olaf", "kristoff"}
	err := IsSame(required, provided)
	if err != nil {
		t.Errorf("Didn't expect error when everything required has been provided")
	}
}

func TestIsSame_missing(t *testing.T) {
	required := []string{"elsa", "anna", "olaf", "kristoff"}
	provided := []string{"elsa", "anna", "olaf"}
	err := IsSame(required, provided)
	if err == nil {
		t.Errorf("Expected error since `kristoff` should be missing")
	}
}

func TestIsSame_extra(t *testing.T) {
	required := []string{"elsa", "anna", "olaf"}
	provided := []string{"elsa", "anna", "olaf", "kristoff"}
	err := IsSame(required, provided)
	if err == nil {
		t.Errorf("Expected error since `kristoff` should be extra")
	}
}

func TestDiffLeft_same(t *testing.T) {
	left := []string{"elsa", "anna", "olaf", "kristoff"}
	right := []string{"elsa", "anna", "olaf", "kristoff"}
	extraLeft := DiffLeft(left, right)

	if !reflect.DeepEqual(extraLeft, []string{}) {
		t.Errorf("Didn't expect extra strings in left list but got %v", extraLeft)
	}
}

func TestDiffLeft_extraLeft(t *testing.T) {
	left := []string{"elsa", "anna", "olaf", "kristoff", "hans"}
	right := []string{"elsa", "anna", "olaf", "kristoff"}
	extraLeft := DiffLeft(left, right)

	if !reflect.DeepEqual(extraLeft, []string{"hans"}) {
		t.Errorf("Should have identified extra string in left list but got %v", extraLeft)
	}
}

func TestDiffLeft_extraRight(t *testing.T) {
	left := []string{"elsa", "anna", "olaf", "kristoff"}
	right := []string{"elsa", "anna", "olaf", "kristoff", "hans"}
	extraLeft := DiffLeft(left, right)

	if !reflect.DeepEqual(extraLeft, []string{}) {
		t.Errorf("Shouldn't have noticed extra item in right list but got %v", extraLeft)
	}
}
