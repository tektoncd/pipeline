/*
Copyright 2022 The Tekton Authors

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

package version_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/pkg/apis/version"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type testStruct struct {
	Field string
}

func TestSerializationRoundTrip(t *testing.T) {
	meta := metav1.ObjectMeta{}
	source := testStruct{Field: "foo"}
	key := "my-key"
	err := version.SerializeToMetadata(&meta, source, key)
	if err != nil {
		t.Fatalf("Serialization error: %s", err)
	}

	sink := testStruct{}
	err = version.DeserializeFromMetadata(&meta, &sink, key)
	if err != nil {
		t.Fatalf("Deserialization error: %s", err)
	}

	_, ok := meta.Annotations[key]
	if ok {
		t.Errorf("Expected key %s not to be present in annotations but it was", key)
	}

	if d := cmp.Diff(source, sink); d != "" {
		t.Errorf("Unexpected diff after serialization/deserialization round trip: %s", d)
	}
}

func TestSliceSerializationRoundTrip(t *testing.T) {
	meta := metav1.ObjectMeta{}
	source := []testStruct{{Field: "foo"}, {Field: "bar"}}
	key := "my-key"
	err := version.SerializeToMetadata(&meta, source, key)
	if err != nil {
		t.Fatalf("Serialization error: %s", err)
	}

	sink := []testStruct{}
	err = version.DeserializeFromMetadata(&meta, &sink, key)
	if err != nil {
		t.Fatalf("Deserialization error: %s", err)
	}

	_, ok := meta.Annotations[key]
	if ok {
		t.Errorf("Expected key %s not to be present in annotations but it was", key)
	}

	if d := cmp.Diff(source, sink); d != "" {
		t.Errorf("Unexpected diff after serialization/deserialization round trip: %s", d)
	}
}
