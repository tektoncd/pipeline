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
	"fmt"

	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"knative.dev/pkg/apis"
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
		return t.Spec.ConvertTo(ctx, &sink.Spec)
	default:
		return fmt.Errorf("unknown version, got: %T", sink)
	}
}

// ConvertTo implements apis.Convertible
func (ts *TaskSpec) ConvertTo(ctx context.Context, sink *v1.TaskSpec) error {
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
		return t.Spec.ConvertFrom(ctx, &source.Spec)
	default:
		return fmt.Errorf("unknown version, got: %T", t)
	}
}

// ConvertFrom implements apis.Convertible
func (ts *TaskSpec) ConvertFrom(ctx context.Context, source *v1.TaskSpec) error {
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
