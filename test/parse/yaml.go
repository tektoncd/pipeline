/*
 Copyright 2021 The Tekton Authors
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

package parse

import (
	"testing"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	resourcev1alpha1 "github.com/tektoncd/pipeline/pkg/apis/resource/v1alpha1"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/pkg/client/clientset/versioned/scheme"
	"k8s.io/apimachinery/pkg/runtime"
)

// MustParseV1beta1TaskRun takes YAML and parses it into a *v1beta1.TaskRun
func MustParseV1beta1TaskRun(t *testing.T, yaml string) *v1beta1.TaskRun {
	var tr v1beta1.TaskRun
	yaml = `apiVersion: tekton.dev/v1beta1
kind: TaskRun
` + yaml
	mustParseYAML(t, yaml, &tr)
	return &tr
}

// MustParseRun takes YAML and parses it into a *v1alpha1.Run
func MustParseRun(t *testing.T, yaml string) *v1alpha1.Run {
	var r v1alpha1.Run
	yaml = `apiVersion: tekton.dev/v1alpha1
kind: Run
` + yaml
	mustParseYAML(t, yaml, &r)
	return &r
}

// MustParseV1beta1Task takes YAML and parses it into a *v1beta1.Task
func MustParseV1beta1Task(t *testing.T, yaml string) *v1beta1.Task {
	var task v1beta1.Task
	yaml = `apiVersion: tekton.dev/v1beta1
kind: Task
` + yaml
	mustParseYAML(t, yaml, &task)
	return &task
}

// MustParseClusterTask takes YAML and parses it into a *v1beta1.ClusterTask
func MustParseClusterTask(t *testing.T, yaml string) *v1beta1.ClusterTask {
	var clusterTask v1beta1.ClusterTask
	yaml = `apiVersion: tekton.dev/v1beta1
kind: ClusterTask
` + yaml
	mustParseYAML(t, yaml, &clusterTask)
	return &clusterTask
}

// MustParseV1beta1PipelineRun takes YAML and parses it into a *v1beta1.PipelineRun
func MustParseV1beta1PipelineRun(t *testing.T, yaml string) *v1beta1.PipelineRun {
	var pr v1beta1.PipelineRun
	yaml = `apiVersion: tekton.dev/v1beta1
kind: PipelineRun
` + yaml
	mustParseYAML(t, yaml, &pr)
	return &pr
}

// MustParseV1beta1Pipeline takes YAML and parses it into a *v1beta1.Pipeline
func MustParseV1beta1Pipeline(t *testing.T, yaml string) *v1beta1.Pipeline {
	var pipeline v1beta1.Pipeline
	yaml = `apiVersion: tekton.dev/v1beta1
kind: Pipeline
` + yaml
	mustParseYAML(t, yaml, &pipeline)
	return &pipeline
}

// MustParsePipelineResource takes YAML and parses it into a *resourcev1alpha1.PipelineResource
func MustParsePipelineResource(t *testing.T, yaml string) *resourcev1alpha1.PipelineResource {
	var resource resourcev1alpha1.PipelineResource
	yaml = `apiVersion: tekton.dev/v1alpha1
kind: PipelineResource
` + yaml
	mustParseYAML(t, yaml, &resource)
	return &resource
}

func mustParseYAML(t *testing.T, yaml string, i runtime.Object) {
	if _, _, err := scheme.Codecs.UniversalDeserializer().Decode([]byte(yaml), nil, i); err != nil {
		t.Fatalf("mustParseYAML (%s): %v", yaml, err)
	}
}
