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

// MustParseTaskRun takes YAML and parses it into a *v1beta1.TaskRun
func MustParseTaskRun(t *testing.T, yaml string) *v1beta1.TaskRun {
	var tr v1beta1.TaskRun
	yaml = `apiVersion: tekton.dev/v1beta1
kind: TaskRun
` + yaml
	mustParseYAML(t, yaml, &tr)
	return &tr
}

// MustParseAlphaTaskRun takes YAML and parses it into a *v1alpha1.TaskRun
func MustParseAlphaTaskRun(t *testing.T, yaml string) *v1alpha1.TaskRun {
	var tr v1alpha1.TaskRun
	yaml = `apiVersion: tekton.dev/v1alpha1
kind: TaskRun
` + yaml
	mustParseYAML(t, yaml, &tr)
	return &tr
}

// MustParseTask takes YAML and parses it into a *v1beta1.Task
func MustParseTask(t *testing.T, yaml string) *v1beta1.Task {
	var task v1beta1.Task
	yaml = `apiVersion: tekton.dev/v1beta1
kind: Task
` + yaml
	mustParseYAML(t, yaml, &task)
	return &task
}

// MustParseAlphaTask takes YAML and parses it into a *v1alpha1.Task
func MustParseAlphaTask(t *testing.T, yaml string) *v1alpha1.Task {
	var task v1alpha1.Task
	yaml = `apiVersion: tekton.dev/v1alpha1
kind: Task
` + yaml
	mustParseYAML(t, yaml, &task)
	return &task
}

// MustParsePipelineRun takes YAML and parses it into a *v1beta1.PipelineRun
func MustParsePipelineRun(t *testing.T, yaml string) *v1beta1.PipelineRun {
	var pr v1beta1.PipelineRun
	yaml = `apiVersion: tekton.dev/v1beta1
kind: PipelineRun
` + yaml
	mustParseYAML(t, yaml, &pr)
	return &pr
}

// MustParseAlphaPipelineRun takes YAML and parses it into a *v1alpha1.PipelineRun
func MustParseAlphaPipelineRun(t *testing.T, yaml string) *v1alpha1.PipelineRun {
	var pr v1alpha1.PipelineRun
	yaml = `apiVersion: tekton.dev/v1alpha1
kind: PipelineRun
` + yaml
	mustParseYAML(t, yaml, &pr)
	return &pr
}

// MustParsePipeline takes YAML and parses it into a *v1beta1.Pipeline
func MustParsePipeline(t *testing.T, yaml string) *v1beta1.Pipeline {
	var pipeline v1beta1.Pipeline
	yaml = `apiVersion: tekton.dev/v1beta1
kind: Pipeline
` + yaml
	mustParseYAML(t, yaml, &pipeline)
	return &pipeline
}

// MustParseAlphaPipeline takes YAML and parses it into a *v1alpha1.Pipeline
func MustParseAlphaPipeline(t *testing.T, yaml string) *v1alpha1.Pipeline {
	var pipeline v1alpha1.Pipeline
	yaml = `apiVersion: tekton.dev/v1alpha1
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

// MustParseCondition takes YAML and parses it into a *v1alpha1.Condition
func MustParseCondition(t *testing.T, yaml string) *v1alpha1.Condition {
	var cond v1alpha1.Condition
	yaml = `apiVersion: tekton.dev/v1alpha1
kind: Condition
` + yaml
	mustParseYAML(t, yaml, &cond)
	return &cond
}

func mustParseYAML(t *testing.T, yaml string, i runtime.Object) {
	if _, _, err := scheme.Codecs.UniversalDeserializer().Decode([]byte(yaml), nil, i); err != nil {
		t.Fatalf("mustParseYAML (%s): %v", yaml, err)
	}
}
