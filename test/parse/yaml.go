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

	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/pkg/client/clientset/versioned/scheme"
	"k8s.io/apimachinery/pkg/runtime"
)

// MustParseV1alpha1StepAction takes YAML and parses it into a *v1alpha1.StepAction
func MustParseV1alpha1StepAction(t *testing.T, yaml string) *v1alpha1.StepAction {
	t.Helper()
	var sa v1alpha1.StepAction
	yaml = `apiVersion: tekton.dev/v1alpha1
kind: StepAction
` + yaml
	mustParseYAML(t, yaml, &sa)
	return &sa
}

// MustParseV1beta1StepAction takes YAML and parses it into a *v1alpha1.StepAction
func MustParseV1beta1StepAction(t *testing.T, yaml string) *v1beta1.StepAction {
	t.Helper()
	var sa v1beta1.StepAction
	yaml = `apiVersion: tekton.dev/v1beta1
kind: StepAction
` + yaml
	mustParseYAML(t, yaml, &sa)
	return &sa
}

// MustParseV1beta1TaskRun takes YAML and parses it into a *v1beta1.TaskRun
func MustParseV1beta1TaskRun(t *testing.T, yaml string) *v1beta1.TaskRun {
	t.Helper()
	var tr v1beta1.TaskRun
	yaml = `apiVersion: tekton.dev/v1beta1
kind: TaskRun
` + yaml
	mustParseYAML(t, yaml, &tr)
	return &tr
}

// MustParseV1TaskRun takes YAML and parses it into a *v1.TaskRun
func MustParseV1TaskRun(t *testing.T, yaml string) *v1.TaskRun {
	t.Helper()
	var tr v1.TaskRun
	yaml = `apiVersion: tekton.dev/v1
kind: TaskRun
` + yaml
	mustParseYAML(t, yaml, &tr)
	return &tr
}

// MustParseV1beta1Task takes YAML and parses it into a *v1beta1.Task
func MustParseV1beta1Task(t *testing.T, yaml string) *v1beta1.Task {
	t.Helper()
	var task v1beta1.Task
	yaml = `apiVersion: tekton.dev/v1beta1
kind: Task
` + yaml
	mustParseYAML(t, yaml, &task)
	return &task
}

// MustParseV1beta1TaskAndSetDefaults takes YAML and parses it into a *v1beta1.Task and sets defaults
func MustParseV1beta1TaskAndSetDefaults(t *testing.T, yaml string) *v1beta1.Task {
	t.Helper()
	task := MustParseV1beta1Task(t, yaml)
	task.SetDefaults(t.Context())
	return task
}

// MustParseCustomRun takes YAML and parses it into a *v1beta1.CustomRun
func MustParseCustomRun(t *testing.T, yaml string) *v1beta1.CustomRun {
	t.Helper()
	var r v1beta1.CustomRun
	yaml = `apiVersion: tekton.dev/v1beta1
kind: CustomRun
` + yaml
	mustParseYAML(t, yaml, &r)
	return &r
}

// MustParseV1Task takes YAML and parses it into a *v1.Task
func MustParseV1Task(t *testing.T, yaml string) *v1.Task {
	t.Helper()
	var task v1.Task
	yaml = `apiVersion: tekton.dev/v1
kind: Task
` + yaml
	mustParseYAML(t, yaml, &task)
	return &task
}

// MustParseV1TaskAndSetDefaults takes YAML and parses it into a *v1.Task and sets defaults
func MustParseV1TaskAndSetDefaults(t *testing.T, yaml string) *v1.Task {
	t.Helper()
	task := MustParseV1Task(t, yaml)
	task.SetDefaults(t.Context())
	return task
}

// MustParseV1beta1PipelineRun takes YAML and parses it into a *v1beta1.PipelineRun
func MustParseV1beta1PipelineRun(t *testing.T, yaml string) *v1beta1.PipelineRun {
	t.Helper()
	var pr v1beta1.PipelineRun
	yaml = `apiVersion: tekton.dev/v1beta1
kind: PipelineRun
` + yaml
	mustParseYAML(t, yaml, &pr)
	return &pr
}

// MustParseV1PipelineRun takes YAML and parses it into a *v1.PipelineRun
func MustParseV1PipelineRun(t *testing.T, yaml string) *v1.PipelineRun {
	t.Helper()
	var pr v1.PipelineRun
	yaml = `apiVersion: tekton.dev/v1
kind: PipelineRun
` + yaml
	mustParseYAML(t, yaml, &pr)
	return &pr
}

// MustParseV1beta1Pipeline takes YAML and parses it into a *v1beta1.Pipeline
func MustParseV1beta1Pipeline(t *testing.T, yaml string) *v1beta1.Pipeline {
	t.Helper()
	var pipeline v1beta1.Pipeline
	yaml = `apiVersion: tekton.dev/v1beta1
kind: Pipeline
` + yaml
	mustParseYAML(t, yaml, &pipeline)
	return &pipeline
}

// MustParseV1beta1PipelineAndSetDefaults takes YAML and parses it into a *v1beta1.Pipeline and sets defaults
func MustParseV1beta1PipelineAndSetDefaults(t *testing.T, yaml string) *v1beta1.Pipeline {
	t.Helper()
	p := MustParseV1beta1Pipeline(t, yaml)
	p.SetDefaults(t.Context())
	return p
}

// MustParseV1Pipeline takes YAML and parses it into a *v1.Pipeline
func MustParseV1Pipeline(t *testing.T, yaml string) *v1.Pipeline {
	t.Helper()
	var pipeline v1.Pipeline
	yaml = `apiVersion: tekton.dev/v1
kind: Pipeline
` + yaml
	mustParseYAML(t, yaml, &pipeline)
	return &pipeline
}

// MustParseV1PipelineAndSetDefaults takes YAML and parses it into a *v1.Pipeline and sets defaults
func MustParseV1PipelineAndSetDefaults(t *testing.T, yaml string) *v1.Pipeline {
	t.Helper()
	p := MustParseV1Pipeline(t, yaml)
	p.SetDefaults(t.Context())
	return p
}

// MustParseVerificationPolicy takes YAML and parses it into a *v1alpha1.VerificationPolicy
func MustParseVerificationPolicy(t *testing.T, yaml string) *v1alpha1.VerificationPolicy {
	t.Helper()
	var v v1alpha1.VerificationPolicy
	yaml = `apiVersion: tekton.dev/v1alpha1
kind: VerificationPolicy
` + yaml
	mustParseYAML(t, yaml, &v)
	return &v
}

func mustParseYAML(t *testing.T, yaml string, i runtime.Object) {
	t.Helper()
	if _, _, err := scheme.Codecs.UniversalDeserializer().Decode([]byte(yaml), nil, i); err != nil {
		t.Fatalf("mustParseYAML (%s): %v", yaml, err)
	}
}
