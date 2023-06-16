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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"knative.dev/pkg/apis"
)

var _ apis.Convertible = (*Pipeline)(nil)

// ConvertTo implements apis.Convertible
func (p *Pipeline) ConvertTo(ctx context.Context, to apis.Convertible) error {
	if apis.IsInDelete(ctx) {
		return nil
	}
	switch sink := to.(type) {
	case *v1.Pipeline:
		sink.ObjectMeta = p.ObjectMeta
		return p.Spec.ConvertTo(ctx, &sink.Spec, &sink.ObjectMeta)
	default:
		return fmt.Errorf("unknown version, got: %T", sink)
	}
}

// ConvertTo implements apis.Convertible
func (ps *PipelineSpec) ConvertTo(ctx context.Context, sink *v1.PipelineSpec, meta *metav1.ObjectMeta) error {
	sink.DisplayName = ps.DisplayName
	sink.Description = ps.Description
	sink.Tasks = nil
	for _, t := range ps.Tasks {
		new := v1.PipelineTask{}
		err := t.convertTo(ctx, &new, meta)
		if err != nil {
			return err
		}
		sink.Tasks = append(sink.Tasks, new)
	}
	sink.Params = nil
	for _, p := range ps.Params {
		new := v1.ParamSpec{}
		p.convertTo(ctx, &new)
		sink.Params = append(sink.Params, new)
	}
	sink.Workspaces = nil
	for _, w := range ps.Workspaces {
		new := v1.PipelineWorkspaceDeclaration{}
		w.convertTo(ctx, &new)
		sink.Workspaces = append(sink.Workspaces, new)
	}
	sink.Results = nil
	for _, r := range ps.Results {
		new := v1.PipelineResult{}
		r.convertTo(ctx, &new)
		sink.Results = append(sink.Results, new)
	}
	sink.Finally = nil
	for _, f := range ps.Finally {
		new := v1.PipelineTask{}
		err := f.convertTo(ctx, &new, meta)
		if err != nil {
			return err
		}
		sink.Finally = append(sink.Finally, new)
	}
	return nil
}

// ConvertFrom implements apis.Convertible
func (p *Pipeline) ConvertFrom(ctx context.Context, from apis.Convertible) error {
	switch source := from.(type) {
	case *v1.Pipeline:
		p.ObjectMeta = source.ObjectMeta
		return p.Spec.ConvertFrom(ctx, &source.Spec, &p.ObjectMeta)
	default:
		return fmt.Errorf("unknown version, got: %T", p)
	}
}

// ConvertFrom implements apis.Convertible
func (ps *PipelineSpec) ConvertFrom(ctx context.Context, source *v1.PipelineSpec, meta *metav1.ObjectMeta) error {
	ps.DisplayName = source.DisplayName
	ps.Description = source.Description
	ps.Tasks = nil
	for _, t := range source.Tasks {
		new := PipelineTask{}
		err := new.convertFrom(ctx, t, meta)
		if err != nil {
			return err
		}
		ps.Tasks = append(ps.Tasks, new)
	}
	ps.Params = nil
	for _, p := range source.Params {
		new := ParamSpec{}
		new.convertFrom(ctx, p)
		ps.Params = append(ps.Params, new)
	}
	ps.Workspaces = nil
	for _, w := range source.Workspaces {
		new := PipelineWorkspaceDeclaration{}
		new.convertFrom(ctx, w)
		ps.Workspaces = append(ps.Workspaces, new)
	}
	ps.Results = nil
	for _, r := range source.Results {
		new := PipelineResult{}
		new.convertFrom(ctx, r)
		ps.Results = append(ps.Results, new)
	}
	ps.Finally = nil
	for _, f := range source.Finally {
		new := PipelineTask{}
		err := new.convertFrom(ctx, f, meta)
		if err != nil {
			return err
		}
		ps.Finally = append(ps.Finally, new)
	}
	return nil
}

func (pt PipelineTask) convertTo(ctx context.Context, sink *v1.PipelineTask, meta *metav1.ObjectMeta) error {
	sink.Name = pt.Name
	sink.DisplayName = pt.DisplayName
	sink.Description = pt.Description
	if pt.TaskRef != nil {
		sink.TaskRef = &v1.TaskRef{}
		pt.TaskRef.convertTo(ctx, sink.TaskRef)
	}
	if pt.TaskSpec != nil {
		sink.TaskSpec = &v1.EmbeddedTask{}
		err := pt.TaskSpec.convertTo(ctx, sink.TaskSpec, meta, pt.Name)
		if err != nil {
			return err
		}
	}
	sink.When = nil
	for _, we := range pt.WhenExpressions {
		new := v1.WhenExpression{}
		we.convertTo(ctx, &new)
		sink.When = append(sink.When, new)
	}
	sink.Retries = pt.Retries
	sink.RunAfter = pt.RunAfter
	sink.Params = nil
	for _, p := range pt.Params {
		new := v1.Param{}
		p.convertTo(ctx, &new)
		sink.Params = append(sink.Params, new)
	}
	sink.Matrix = nil
	if pt.IsMatrixed() {
		new := v1.Matrix{}
		pt.Matrix.convertTo(ctx, &new)
		sink.Matrix = &new
	}
	sink.Workspaces = nil
	for _, w := range pt.Workspaces {
		new := v1.WorkspacePipelineTaskBinding{}
		w.convertTo(ctx, &new)
		sink.Workspaces = append(sink.Workspaces, new)
	}

	sink.Timeout = pt.Timeout
	return nil
}

func (pt *PipelineTask) convertFrom(ctx context.Context, source v1.PipelineTask, meta *metav1.ObjectMeta) error {
	pt.Name = source.Name
	pt.DisplayName = source.DisplayName
	pt.Description = source.Description
	if source.TaskRef != nil {
		newTaskRef := TaskRef{}
		newTaskRef.ConvertFrom(ctx, *source.TaskRef)
		pt.TaskRef = &newTaskRef
	}
	if source.TaskSpec != nil {
		newTaskSpec := EmbeddedTask{}
		err := newTaskSpec.convertFrom(ctx, *source.TaskSpec, meta, pt.Name)
		pt.TaskSpec = &newTaskSpec
		if err != nil {
			return err
		}
	}
	pt.WhenExpressions = nil
	for _, we := range source.When {
		new := WhenExpression{}
		new.convertFrom(ctx, we)
		pt.WhenExpressions = append(pt.WhenExpressions, new)
	}
	pt.Retries = source.Retries
	pt.RunAfter = source.RunAfter
	pt.Params = nil
	for _, p := range source.Params {
		new := Param{}
		new.ConvertFrom(ctx, p)
		pt.Params = append(pt.Params, new)
	}
	pt.Matrix = nil
	if source.IsMatrixed() {
		new := Matrix{}
		new.convertFrom(ctx, *source.Matrix)
		pt.Matrix = &new
	}
	pt.Workspaces = nil
	for _, w := range source.Workspaces {
		new := WorkspacePipelineTaskBinding{}
		new.convertFrom(ctx, w)
		pt.Workspaces = append(pt.Workspaces, new)
	}

	pt.Timeout = source.Timeout
	return nil
}

func (et EmbeddedTask) convertTo(ctx context.Context, sink *v1.EmbeddedTask, meta *metav1.ObjectMeta, taskName string) error {
	sink.TypeMeta = et.TypeMeta
	sink.Spec = et.Spec
	sink.Metadata = v1.PipelineTaskMetadata(et.Metadata)
	sink.TaskSpec = v1.TaskSpec{}
	return et.TaskSpec.ConvertTo(ctx, &sink.TaskSpec, meta, taskName)
}

func (et *EmbeddedTask) convertFrom(ctx context.Context, source v1.EmbeddedTask, meta *metav1.ObjectMeta, taskName string) error {
	et.TypeMeta = source.TypeMeta
	et.Spec = source.Spec
	et.Metadata = PipelineTaskMetadata(source.Metadata)
	et.TaskSpec = TaskSpec{}
	return et.TaskSpec.ConvertFrom(ctx, &source.TaskSpec, meta, taskName)
}

func (we WhenExpression) convertTo(ctx context.Context, sink *v1.WhenExpression) {
	sink.Input = we.Input
	sink.Operator = we.Operator
	sink.Values = we.Values
}

func (we *WhenExpression) convertFrom(ctx context.Context, source v1.WhenExpression) {
	we.Input = source.Input
	we.Operator = source.Operator
	we.Values = source.Values
}

func (m *Matrix) convertTo(ctx context.Context, sink *v1.Matrix) {
	for _, param := range m.Params {
		new := v1.Param{}
		param.convertTo(ctx, &new)
		sink.Params = append(sink.Params, new)
	}
	for i, include := range m.Include {
		sink.Include = append(sink.Include, v1.IncludeParams{Name: include.Name})
		for _, param := range include.Params {
			newIncludeParam := v1.Param{}
			param.convertTo(ctx, &newIncludeParam)
			sink.Include[i].Params = append(sink.Include[i].Params, newIncludeParam)
		}
	}
}

func (m *Matrix) convertFrom(ctx context.Context, source v1.Matrix) {
	for _, param := range source.Params {
		new := Param{}
		new.ConvertFrom(ctx, param)
		m.Params = append(m.Params, new)
	}

	for i, include := range source.Include {
		m.Include = append(m.Include, IncludeParams{Name: include.Name})
		for _, p := range include.Params {
			new := Param{}
			new.ConvertFrom(ctx, p)
			m.Include[i].Params = append(m.Include[i].Params, new)
		}
	}
}

func (pr PipelineResult) convertTo(ctx context.Context, sink *v1.PipelineResult) {
	sink.Name = pr.Name
	sink.Type = v1.ResultsType(pr.Type)
	sink.Description = pr.Description
	newValue := v1.ParamValue{}
	pr.Value.convertTo(ctx, &newValue)
	sink.Value = newValue
}

func (pr *PipelineResult) convertFrom(ctx context.Context, source v1.PipelineResult) {
	pr.Name = source.Name
	pr.Type = ResultsType(source.Type)
	pr.Description = source.Description
	newValue := ParamValue{}
	newValue.convertFrom(ctx, source.Value)
	pr.Value = newValue
}

func (ptm PipelineTaskMetadata) convertTo(ctx context.Context, sink *v1.PipelineTaskMetadata) {
	sink.Labels = ptm.Labels
	sink.Annotations = ptm.Annotations
}

func (ptm *PipelineTaskMetadata) convertFrom(ctx context.Context, source v1.PipelineTaskMetadata) {
	ptm.Labels = source.Labels
	ptm.Annotations = source.Labels
}
