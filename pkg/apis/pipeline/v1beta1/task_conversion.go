/*
Copyright 2020 The Tekton Authors

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

package v1beta1

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/tektoncd/pipeline/pkg/apis/version"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"knative.dev/pkg/apis"
)

// TaskDeprecationsAnnotationKey is the annotation key for all deprecated fields of (a) Task(s) that belong(s) to an object.
// For example: a v1beta1.Pipeline contains two tasks
//
// spec:
//
//	 tasks:
//	 - name: task-1
//	   stepTemplate:
//		 name: deprecated-name-field # deprecated field
//	 - name: task-2
//	   steps:
//	   - tty: true # deprecated field
//
// The annotation would be:
//
//	"tekton.dev/v1beta1.task-deprecations": `{
//	   "task1":{
//	      "deprecatedStepTemplates":{
//	         "name":"deprecated-name-field"
//	       },
//	    },
//	   "task-2":{
//	      "deprecatedSteps":[{"tty":true}],
//	    },
//	}`
const (
	TaskDeprecationsAnnotationKey = "tekton.dev/v1beta1.task-deprecations"
	resourcesAnnotationKey        = "tekton.dev/v1beta1Resources"
)

var _ apis.Convertible = (*Task)(nil)

// ConvertTo implements apis.Convertible
func (t *Task) ConvertTo(ctx context.Context, to apis.Convertible) error {
	if apis.IsInDelete(ctx) {
		return nil
	}
	switch sink := to.(type) {
	case *v1.Task:
		sink.ObjectMeta = t.ObjectMeta
		if err := serializeResources(&sink.ObjectMeta, &t.Spec); err != nil {
			return err
		}
		return t.Spec.ConvertTo(ctx, &sink.Spec, &sink.ObjectMeta, t.Name)
	default:
		return fmt.Errorf("unknown version, got: %T", sink)
	}
}

// ConvertTo implements apis.Convertible
func (ts *TaskSpec) ConvertTo(ctx context.Context, sink *v1.TaskSpec, meta *metav1.ObjectMeta, taskName string) error {
	if err := serializeTaskDeprecations(meta, ts, taskName); err != nil {
		return err
	}
	sink.Steps = nil
	for _, s := range ts.Steps {
		new := v1.Step{}
		s.convertTo(ctx, &new)
		sink.Steps = append(sink.Steps, new)
	}
	sink.Volumes = ts.Volumes
	if ts.StepTemplate != nil {
		new := v1.StepTemplate{}
		ts.StepTemplate.convertTo(ctx, &new)
		sink.StepTemplate = &new
	}
	sink.Sidecars = nil
	for _, s := range ts.Sidecars {
		new := v1.Sidecar{}
		s.convertTo(ctx, &new)
		sink.Sidecars = append(sink.Sidecars, new)
	}
	sink.Workspaces = nil
	for _, w := range ts.Workspaces {
		new := v1.WorkspaceDeclaration{}
		w.convertTo(ctx, &new)
		sink.Workspaces = append(sink.Workspaces, new)
	}
	sink.Results = nil
	for _, r := range ts.Results {
		new := v1.TaskResult{}
		r.convertTo(ctx, &new)
		sink.Results = append(sink.Results, new)
	}
	sink.Params = nil
	for _, p := range ts.Params {
		new := v1.ParamSpec{}
		p.convertTo(ctx, &new)
		sink.Params = append(sink.Params, new)
	}
	sink.DisplayName = ts.DisplayName
	sink.Description = ts.Description
	return nil
}

// ConvertFrom implements apis.Convertible
func (t *Task) ConvertFrom(ctx context.Context, from apis.Convertible) error {
	if apis.IsInDelete(ctx) {
		return nil
	}
	switch source := from.(type) {
	case *v1.Task:
		t.ObjectMeta = source.ObjectMeta
		if err := deserializeResources(&t.ObjectMeta, &t.Spec); err != nil {
			return err
		}
		return t.Spec.ConvertFrom(ctx, &source.Spec, &t.ObjectMeta, t.Name)
	default:
		return fmt.Errorf("unknown version, got: %T", t)
	}
}

// ConvertFrom implements apis.Convertible
func (ts *TaskSpec) ConvertFrom(ctx context.Context, source *v1.TaskSpec, meta *metav1.ObjectMeta, taskName string) error {
	ts.Steps = nil
	for _, s := range source.Steps {
		new := Step{}
		new.convertFrom(ctx, s)
		ts.Steps = append(ts.Steps, new)
	}
	ts.Volumes = source.Volumes
	if source.StepTemplate != nil {
		new := StepTemplate{}
		new.convertFrom(ctx, source.StepTemplate)
		ts.StepTemplate = &new
	}
	if err := deserializeTaskDeprecations(meta, ts, taskName); err != nil {
		return err
	}
	ts.Sidecars = nil
	for _, s := range source.Sidecars {
		new := Sidecar{}
		new.convertFrom(ctx, s)
		ts.Sidecars = append(ts.Sidecars, new)
	}
	ts.Workspaces = nil
	for _, w := range source.Workspaces {
		new := WorkspaceDeclaration{}
		new.convertFrom(ctx, w)
		ts.Workspaces = append(ts.Workspaces, new)
	}
	ts.Results = nil
	for _, r := range source.Results {
		new := TaskResult{}
		new.convertFrom(ctx, r)
		ts.Results = append(ts.Results, new)
	}
	ts.Params = nil
	for _, p := range source.Params {
		new := ParamSpec{}
		new.convertFrom(ctx, p)
		ts.Params = append(ts.Params, new)
	}
	ts.DisplayName = source.DisplayName
	ts.Description = source.Description
	return nil
}

// taskDeprecation contains deprecated fields of a Task
// +k8s:openapi-gen=false
type taskDeprecation struct {
	// DeprecatedSteps contains Steps of a Task that with deprecated fields defined.
	// +listType=atomic
	DeprecatedSteps []Step `json:"deprecatedSteps,omitempty"`
	// DeprecatedStepTemplate contains stepTemplate of a Task that with deprecated fields defined.
	DeprecatedStepTemplate *StepTemplate `json:"deprecatedStepTemplate,omitempty"`
}

// taskDeprecations contains deprecated fields of Tasks that belong to the same Pipeline or PipelineRun
// the key is Task name
// +k8s:openapi-gen=false
type taskDeprecations map[string]taskDeprecation

// serializeTaskDeprecations appends the current Task's deprecation info to annotation of the object.
// The object could be Task, TaskRun, Pipeline or PipelineRun
func serializeTaskDeprecations(meta *metav1.ObjectMeta, spec *TaskSpec, taskName string) error {
	var taskDeprecation *taskDeprecation
	if spec.HasDeprecatedFields() {
		taskDeprecation = retrieveTaskDeprecation(spec)
	}
	existingDeprecations := taskDeprecations{}
	if str, ok := meta.Annotations[TaskDeprecationsAnnotationKey]; ok {
		if err := json.Unmarshal([]byte(str), &existingDeprecations); err != nil {
			return fmt.Errorf("error serializing key %s from metadata: %w", TaskDeprecationsAnnotationKey, err)
		}
	}
	if taskDeprecation != nil {
		existingDeprecations[taskName] = *taskDeprecation
		return version.SerializeToMetadata(meta, existingDeprecations, TaskDeprecationsAnnotationKey)
	}
	return nil
}

// deserializeTaskDeprecations retrieves deprecation info of the Task from object annotation.
// The object could be Task, TaskRun, Pipeline or PipelineRun.
func deserializeTaskDeprecations(meta *metav1.ObjectMeta, spec *TaskSpec, taskName string) error {
	existingDeprecations := taskDeprecations{}
	if meta == nil || meta.Annotations == nil {
		return nil
	}
	if str, ok := meta.Annotations[TaskDeprecationsAnnotationKey]; ok {
		if err := json.Unmarshal([]byte(str), &existingDeprecations); err != nil {
			return fmt.Errorf("error deserializing key %s from metadata: %w", TaskDeprecationsAnnotationKey, err)
		}
	}
	if td, ok := existingDeprecations[taskName]; ok {
		if len(spec.Steps) != len(td.DeprecatedSteps) {
			return errors.New("length of deserialized steps mismatch the length of target steps")
		}
		for i := range len(spec.Steps) {
			spec.Steps[i].DeprecatedPorts = td.DeprecatedSteps[i].DeprecatedPorts
			spec.Steps[i].DeprecatedLivenessProbe = td.DeprecatedSteps[i].DeprecatedLivenessProbe
			spec.Steps[i].DeprecatedReadinessProbe = td.DeprecatedSteps[i].DeprecatedReadinessProbe
			spec.Steps[i].DeprecatedStartupProbe = td.DeprecatedSteps[i].DeprecatedStartupProbe
			spec.Steps[i].DeprecatedLifecycle = td.DeprecatedSteps[i].DeprecatedLifecycle
			spec.Steps[i].DeprecatedTerminationMessagePath = td.DeprecatedSteps[i].DeprecatedTerminationMessagePath
			spec.Steps[i].DeprecatedTerminationMessagePolicy = td.DeprecatedSteps[i].DeprecatedTerminationMessagePolicy
			spec.Steps[i].DeprecatedStdin = td.DeprecatedSteps[i].DeprecatedStdin
			spec.Steps[i].DeprecatedStdinOnce = td.DeprecatedSteps[i].DeprecatedStdinOnce
			spec.Steps[i].DeprecatedTTY = td.DeprecatedSteps[i].DeprecatedTTY
		}
		if td.DeprecatedStepTemplate != nil {
			if spec.StepTemplate == nil {
				spec.StepTemplate = &StepTemplate{}
			}
			spec.StepTemplate.DeprecatedName = td.DeprecatedStepTemplate.DeprecatedName
			spec.StepTemplate.DeprecatedPorts = td.DeprecatedStepTemplate.DeprecatedPorts
			spec.StepTemplate.DeprecatedLivenessProbe = td.DeprecatedStepTemplate.DeprecatedLivenessProbe
			spec.StepTemplate.DeprecatedReadinessProbe = td.DeprecatedStepTemplate.DeprecatedReadinessProbe
			spec.StepTemplate.DeprecatedStartupProbe = td.DeprecatedStepTemplate.DeprecatedStartupProbe
			spec.StepTemplate.DeprecatedLifecycle = td.DeprecatedStepTemplate.DeprecatedLifecycle
			spec.StepTemplate.DeprecatedTerminationMessagePath = td.DeprecatedStepTemplate.DeprecatedTerminationMessagePath
			spec.StepTemplate.DeprecatedTerminationMessagePolicy = td.DeprecatedStepTemplate.DeprecatedTerminationMessagePolicy
			spec.StepTemplate.DeprecatedStdin = td.DeprecatedStepTemplate.DeprecatedStdin
			spec.StepTemplate.DeprecatedStdinOnce = td.DeprecatedStepTemplate.DeprecatedStdinOnce
			spec.StepTemplate.DeprecatedTTY = td.DeprecatedStepTemplate.DeprecatedTTY
		}
		delete(existingDeprecations, taskName)
		if len(existingDeprecations) == 0 {
			delete(meta.Annotations, TaskDeprecationsAnnotationKey)
		} else {
			updatedDeprecations, err := json.Marshal(existingDeprecations)
			if err != nil {
				return err
			}
			meta.Annotations[TaskDeprecationsAnnotationKey] = string(updatedDeprecations)
		}
		if len(meta.Annotations) == 0 {
			meta.Annotations = nil
		}
	}
	return nil
}

func retrieveTaskDeprecation(spec *TaskSpec) *taskDeprecation {
	if !spec.HasDeprecatedFields() {
		return nil
	}
	ds := []Step{}
	for _, s := range spec.Steps {
		ds = append(ds, Step{
			DeprecatedPorts:                    s.DeprecatedPorts,
			DeprecatedLivenessProbe:            s.DeprecatedLivenessProbe,
			DeprecatedReadinessProbe:           s.DeprecatedReadinessProbe,
			DeprecatedStartupProbe:             s.DeprecatedStartupProbe,
			DeprecatedLifecycle:                s.DeprecatedLifecycle,
			DeprecatedTerminationMessagePath:   s.DeprecatedTerminationMessagePath,
			DeprecatedTerminationMessagePolicy: s.DeprecatedTerminationMessagePolicy,
			DeprecatedStdin:                    s.DeprecatedStdin,
			DeprecatedStdinOnce:                s.DeprecatedStdinOnce,
			DeprecatedTTY:                      s.DeprecatedTTY,
		})
	}
	var dst *StepTemplate
	if spec.StepTemplate != nil {
		dst = &StepTemplate{
			DeprecatedName:                     spec.StepTemplate.DeprecatedName,
			DeprecatedPorts:                    spec.StepTemplate.DeprecatedPorts,
			DeprecatedLivenessProbe:            spec.StepTemplate.DeprecatedLivenessProbe,
			DeprecatedReadinessProbe:           spec.StepTemplate.DeprecatedReadinessProbe,
			DeprecatedStartupProbe:             spec.StepTemplate.DeprecatedStartupProbe,
			DeprecatedLifecycle:                spec.StepTemplate.DeprecatedLifecycle,
			DeprecatedTerminationMessagePath:   spec.StepTemplate.DeprecatedTerminationMessagePath,
			DeprecatedTerminationMessagePolicy: spec.StepTemplate.DeprecatedTerminationMessagePolicy,
			DeprecatedStdin:                    spec.StepTemplate.DeprecatedStdin,
			DeprecatedStdinOnce:                spec.StepTemplate.DeprecatedStdinOnce,
			DeprecatedTTY:                      spec.StepTemplate.DeprecatedTTY,
		}
	}
	return &taskDeprecation{
		DeprecatedSteps:        ds,
		DeprecatedStepTemplate: dst,
	}
}

func serializeResources(meta *metav1.ObjectMeta, spec *TaskSpec) error {
	if spec.Resources == nil {
		return nil
	}
	return version.SerializeToMetadata(meta, spec.Resources, resourcesAnnotationKey)
}

func deserializeResources(meta *metav1.ObjectMeta, spec *TaskSpec) error {
	resources := &TaskResources{}
	err := version.DeserializeFromMetadata(meta, resources, resourcesAnnotationKey)
	if err != nil {
		return err
	}
	if resources.Inputs != nil || resources.Outputs != nil {
		spec.Resources = resources
	}
	return nil
}
