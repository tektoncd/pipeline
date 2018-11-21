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
package resources_test

import (
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/knative/build-pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/knative/build-pipeline/pkg/reconciler/v1alpha1/pipelinerun/resources"
)

func Test_GetOutputSteps(t *testing.T) {
	sb := []v1alpha1.SourceBinding{{
		Name: "test-output",
		ResourceRef: v1alpha1.PipelineResourceRef{
			Name: "resource1",
		},
	}}
	taskOutputResources, postSteps := resources.GetOutputSteps(sb, "test-taskname")
	expectedtaskOutputResources := []v1alpha1.TaskRunResourceVersion{{
		ResourceRef: v1alpha1.PipelineResourceRef{
			Name: "resource1",
		},
		Name: "test-output",
	}}
	if d := cmp.Diff(expectedtaskOutputResources, taskOutputResources); d != "" {
		t.Errorf("error comparing task outputs")
	}
	expectedpostSteps := []v1alpha1.TaskBuildStep{{
		Name:  "resource1",
		Paths: []string{filepath.Join("/pvc", "test-taskname", "test-output")},
	}}
	if d := cmp.Diff(expectedpostSteps, postSteps); d != "" {
		t.Errorf("error comparing post steps")
	}
}
