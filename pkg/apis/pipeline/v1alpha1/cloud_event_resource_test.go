/*
Copyright 2019 The Tekton Authors.

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

package v1alpha1_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"

	tb "github.com/tektoncd/pipeline/test/builder"
)

func TestNewCloudEventResource_Invalid(t *testing.T) {
	testcases := []struct {
		name             string
		pipelineResource *v1alpha1.PipelineResource
	}{{
		name: "create resource with no parameter",
		pipelineResource: tb.PipelineResource("cloud-event-resource-no-uri", "default", tb.PipelineResourceSpec(
			v1alpha1.PipelineResourceTypeCloudEvent,
		)),
	}, {
		name: "create resource with invalid type",
		pipelineResource: tb.PipelineResource("git-resource", "default", tb.PipelineResourceSpec(
			v1alpha1.PipelineResourceTypeGit,
			tb.PipelineResourceSpecParam("URL", "git://fake/repo"),
			tb.PipelineResourceSpecParam("Revision", "fake_rev"),
		)),
	}}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := v1alpha1.NewCloudEventResource(tc.pipelineResource)
			if err == nil {
				t.Error("Expected error creating CloudEvent resource")
			}
		})
	}
}

func TestNewCloudEventResource_Valid(t *testing.T) {
	pr := tb.PipelineResource("cloud-event-resource-uri", "default", tb.PipelineResourceSpec(
		v1alpha1.PipelineResourceTypeCloudEvent,
		tb.PipelineResourceSpecParam("TargetURI", "http://fake-sink"),
	))
	expectedCloudEventResource := &v1alpha1.CloudEventResource{
		Name:      "cloud-event-resource-uri",
		TargetURI: "http://fake-sink",
		Type:      v1alpha1.PipelineResourceTypeCloudEvent,
	}

	r, err := v1alpha1.NewCloudEventResource(pr)
	if err != nil {
		t.Fatalf("Unexpected error creating CloudEvent resource: %s", err)
	}
	if d := cmp.Diff(expectedCloudEventResource, r); d != "" {
		t.Errorf("Mismatch of CloudEvent resource: %s", d)
	}
}

func TestCloudEvent_GetReplacements(t *testing.T) {
	r := &v1alpha1.CloudEventResource{
		Name:      "cloud-event-resource",
		TargetURI: "http://fake-uri",
		Type:      v1alpha1.PipelineResourceTypeCloudEvent,
	}
	expectedReplacementMap := map[string]string{
		"name":       "cloud-event-resource",
		"type":       "cloudEvent",
		"target-uri": "http://fake-uri",
	}
	if d := cmp.Diff(r.Replacements(), expectedReplacementMap); d != "" {
		t.Errorf("CloudEvent Replacement map mismatch: %s", d)
	}
}

func TestCloudEvent_InputContainerSpec(t *testing.T) {
	r := &v1alpha1.CloudEventResource{
		Name:      "cloud-event-resource",
		TargetURI: "http://fake-uri",
		Type:      v1alpha1.PipelineResourceTypeCloudEvent,
	}
	d, e := r.GetInputTaskModifier(&v1alpha1.TaskSpec{}, "")
	if d.GetStepsToPrepend() != nil {
		t.Errorf("Did not expect a download container for CloudEventResource")
	}
	if e != nil {
		t.Errorf("Did not expect an error %s when getting a download container for CloudEventResource", e)
	}
}

func TestCloudEvent_OutputContainerSpec(t *testing.T) {
	r := &v1alpha1.CloudEventResource{
		Name:      "cloud-event-resource",
		TargetURI: "http://fake-uri",
		Type:      v1alpha1.PipelineResourceTypeCloudEvent,
	}
	d, e := r.GetOutputTaskModifier(&v1alpha1.TaskSpec{}, "")
	if d.GetStepsToAppend() != nil {
		t.Errorf("Did not expect an upload container for CloudEventResource")
	}
	if e != nil {
		t.Errorf("Did not expect an error %s when getting an upload container for CloudEventResource", e)
	}
}
