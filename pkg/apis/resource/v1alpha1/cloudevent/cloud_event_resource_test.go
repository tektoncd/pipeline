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

package cloudevent_test

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	resourcev1alpha1 "github.com/tektoncd/pipeline/pkg/apis/resource/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/apis/resource/v1alpha1/cloudevent"
	"github.com/tektoncd/pipeline/test/diff"
)

func TestNewResource_Invalid(t *testing.T) {
	testcases := []struct {
		name             string
		pipelineResource *resourcev1alpha1.PipelineResource
	}{{
		name: "create resource with no parameter",
		pipelineResource: &resourcev1alpha1.PipelineResource{
			ObjectMeta: metav1.ObjectMeta{
				Name: "cloud-event-resource-no-uri",
			},
			Spec: resourcev1alpha1.PipelineResourceSpec{
				Type: resourcev1alpha1.PipelineResourceTypeCloudEvent,
			},
		},
	}, {
		name: "create resource with invalid type",
		pipelineResource: &resourcev1alpha1.PipelineResource{
			ObjectMeta: metav1.ObjectMeta{
				Name: "git-resource",
			},
			Spec: resourcev1alpha1.PipelineResourceSpec{
				Type: resourcev1alpha1.PipelineResourceTypeGit,
				Params: []resourcev1alpha1.ResourceParam{
					{
						Name:  "URL",
						Value: "git://fake/repo",
					},
					{
						Name:  "Revision",
						Value: "fake_rev",
					},
				},
			},
		},
	}}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := cloudevent.NewResource("test-resource", tc.pipelineResource)
			if err == nil {
				t.Error("Expected error creating CloudEvent resource")
			}
		})
	}
}

func TestNewResource_Valid(t *testing.T) {
	pr := &resourcev1alpha1.PipelineResource{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cloud-event-resource-uri",
		},
		Spec: resourcev1alpha1.PipelineResourceSpec{
			Type: resourcev1alpha1.PipelineResourceTypeCloudEvent,
			Params: []resourcev1alpha1.ResourceParam{{
				Name:  "TargetURI",
				Value: "http://fake-sink",
			}},
		},
	}
	expectedResource := &cloudevent.Resource{
		Name:      "test-resource",
		TargetURI: "http://fake-sink",
		Type:      resourcev1alpha1.PipelineResourceTypeCloudEvent,
	}

	r, err := cloudevent.NewResource("test-resource", pr)
	if err != nil {
		t.Fatalf("Unexpected error creating CloudEvent resource: %s", err)
	}
	if d := cmp.Diff(expectedResource, r); d != "" {
		t.Errorf("Mismatch of CloudEvent resource %s", diff.PrintWantGot(d))
	}
}

func TestCloudEvent_GetReplacements(t *testing.T) {
	r := &cloudevent.Resource{
		Name:      "cloud-event-resource",
		TargetURI: "http://fake-uri",
		Type:      resourcev1alpha1.PipelineResourceTypeCloudEvent,
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
	r := &cloudevent.Resource{
		Name:      "cloud-event-resource",
		TargetURI: "http://fake-uri",
		Type:      resourcev1alpha1.PipelineResourceTypeCloudEvent,
	}
	d, e := r.GetInputTaskModifier(&v1beta1.TaskSpec{}, "")
	if d.GetStepsToPrepend() != nil {
		t.Errorf("Did not expect a download container for Resource")
	}
	if e != nil {
		t.Errorf("Did not expect an error %s when getting a download container for Resource", e)
	}
}

func TestCloudEvent_OutputContainerSpec(t *testing.T) {
	r := &cloudevent.Resource{
		Name:      "cloud-event-resource",
		TargetURI: "http://fake-uri",
		Type:      resourcev1alpha1.PipelineResourceTypeCloudEvent,
	}
	d, e := r.GetOutputTaskModifier(&v1beta1.TaskSpec{}, "")
	if d.GetStepsToAppend() != nil {
		t.Errorf("Did not expect an upload container for Resource")
	}
	if e != nil {
		t.Errorf("Did not expect an error %s when getting an upload container for Resource", e)
	}
}
