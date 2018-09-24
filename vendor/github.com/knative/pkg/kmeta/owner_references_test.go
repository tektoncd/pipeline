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

	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type Frobber struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
}

func (f *Frobber) GetGroupVersionKind() schema.GroupVersionKind {
	return schema.GroupVersionKind{
		Group:   "example.knative.dev",
		Version: "v1alpha1",
		Kind:    "Frobber",
	}
}

func TestNewControllerRef(t *testing.T) {
	f := &Frobber{
		metav1.TypeMeta{},
		metav1.ObjectMeta{
			Name: "foo",
			UID:  "42",
		},
	}

	blockOwnerDeletion := true
	isController := true
	want := &metav1.OwnerReference{
		APIVersion:         "example.knative.dev/v1alpha1",
		Kind:               "Frobber",
		Name:               "foo",
		UID:                "42",
		BlockOwnerDeletion: &blockOwnerDeletion,
		Controller:         &isController,
	}

	got := NewControllerRef(f)
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("Unexpected OwnerReference (-want +got): %v", diff)
	}
}
