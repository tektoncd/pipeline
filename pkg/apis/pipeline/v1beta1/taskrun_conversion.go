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
	"github.com/tektoncd/pipeline/pkg/apis/version"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
)

var _ apis.Convertible = (*TaskRun)(nil)

// ConvertTo implements apis.Convertible
func (tr *TaskRun) ConvertTo(ctx context.Context, to apis.Convertible) error {
	if apis.IsInDelete(ctx) {
		return nil
	}
	switch sink := to.(type) {
	case *v1.TaskRun:
		sink.ObjectMeta = tr.ObjectMeta
		if err := serializeTaskRunResources(&sink.ObjectMeta, &tr.Spec); err != nil {
			return err
		}
		return tr.Spec.ConvertTo(ctx, &sink.Spec)
	default:
		return fmt.Errorf("unknown version, got: %T", sink)
	}
}

// ConvertTo implements apis.Convertible
func (trs *TaskRunSpec) ConvertTo(ctx context.Context, sink *v1.TaskRunSpec) error {
	if trs.Debug != nil {
		sink.Debug = &v1.TaskRunDebug{}
		trs.Debug.convertTo(ctx, sink.Debug)
	}
	sink.Params = nil
	for _, p := range trs.Params {
		new := v1.Param{}
		p.convertTo(ctx, &new)
		sink.Params = append(sink.Params, new)
	}
	sink.ServiceAccountName = trs.ServiceAccountName
	if trs.TaskRef != nil {
		sink.TaskRef = &v1.TaskRef{}
		trs.TaskRef.convertTo(ctx, sink.TaskRef)
	}
	if trs.TaskSpec != nil {
		sink.TaskSpec = &v1.TaskSpec{}
		err := trs.TaskSpec.ConvertTo(ctx, sink.TaskSpec)
		if err != nil {
			return err
		}
	}
	sink.Status = v1.TaskRunSpecStatus(trs.Status)
	sink.StatusMessage = v1.TaskRunSpecStatusMessage(trs.StatusMessage)
	sink.Timeout = trs.Timeout
	sink.PodTemplate = trs.PodTemplate
	sink.Workspaces = nil
	for _, w := range trs.Workspaces {
		new := v1.WorkspaceBinding{}
		w.convertTo(ctx, &new)
		sink.Workspaces = append(sink.Workspaces, new)
	}
	sink.StepOverrides = nil
	for _, so := range trs.StepOverrides {
		new := v1.TaskRunStepOverride{}
		so.convertTo(ctx, &new)
		sink.StepOverrides = append(sink.StepOverrides, new)
	}
	sink.SidecarOverrides = nil
	for _, so := range trs.SidecarOverrides {
		new := v1.TaskRunSidecarOverride{}
		so.convertTo(ctx, &new)
		sink.SidecarOverrides = append(sink.SidecarOverrides, new)
	}
	sink.ComputeResources = trs.ComputeResources
	return nil
}

// ConvertFrom implements apis.Convertible
func (tr *TaskRun) ConvertFrom(ctx context.Context, from apis.Convertible) error {
	if apis.IsInDelete(ctx) {
		return nil
	}
	switch source := from.(type) {
	case *v1.TaskRun:
		tr.ObjectMeta = source.ObjectMeta
		if err := deserializeTaskRunResources(&tr.ObjectMeta, &tr.Spec); err != nil {
			return err
		}
		return tr.Spec.ConvertFrom(ctx, &source.Spec)
	default:
		return fmt.Errorf("unknown version, got: %T", tr)
	}
}

// ConvertFrom implements apis.Convertible
func (trs *TaskRunSpec) ConvertFrom(ctx context.Context, source *v1.TaskRunSpec) error {
	if source.Debug != nil {
		newDebug := TaskRunDebug{}
		newDebug.convertFrom(ctx, *source.Debug)
		trs.Debug = &newDebug
	}
	trs.Params = nil
	for _, p := range source.Params {
		new := Param{}
		new.convertFrom(ctx, p)
		trs.Params = append(trs.Params, new)
	}
	trs.ServiceAccountName = source.ServiceAccountName
	if source.TaskRef != nil {
		newTaskRef := TaskRef{}
		newTaskRef.convertFrom(ctx, *source.TaskRef)
		trs.TaskRef = &newTaskRef
	}
	if source.TaskSpec != nil {
		newTaskSpec := TaskSpec{}
		err := newTaskSpec.ConvertFrom(ctx, source.TaskSpec)
		if err != nil {
			return err
		}
		trs.TaskSpec = &newTaskSpec
	}
	trs.Status = TaskRunSpecStatus(source.Status)
	trs.StatusMessage = TaskRunSpecStatusMessage(source.StatusMessage)
	trs.Timeout = source.Timeout
	trs.PodTemplate = source.PodTemplate
	trs.Workspaces = nil
	for _, w := range source.Workspaces {
		new := WorkspaceBinding{}
		new.convertFrom(ctx, w)
		trs.Workspaces = append(trs.Workspaces, new)
	}
	trs.StepOverrides = nil
	for _, so := range source.StepOverrides {
		new := TaskRunStepOverride{}
		new.convertFrom(ctx, so)
		trs.StepOverrides = append(trs.StepOverrides, new)
	}
	trs.SidecarOverrides = nil
	for _, so := range source.SidecarOverrides {
		new := TaskRunSidecarOverride{}
		new.convertFrom(ctx, so)
		trs.SidecarOverrides = append(trs.SidecarOverrides, new)
	}
	trs.ComputeResources = source.ComputeResources
	return nil
}

func (trd TaskRunDebug) convertTo(ctx context.Context, sink *v1.TaskRunDebug) {
	sink.Breakpoint = trd.Breakpoint
}

func (trd *TaskRunDebug) convertFrom(ctx context.Context, source v1.TaskRunDebug) {
	trd.Breakpoint = source.Breakpoint
}

func (trso TaskRunStepOverride) convertTo(ctx context.Context, sink *v1.TaskRunStepOverride) {
	sink.Name = trso.Name
	sink.Resources = trso.Resources
}

func (trso *TaskRunStepOverride) convertFrom(ctx context.Context, source v1.TaskRunStepOverride) {
	trso.Name = source.Name
	trso.Resources = source.Resources
}

func (trso TaskRunSidecarOverride) convertTo(ctx context.Context, sink *v1.TaskRunSidecarOverride) {
	sink.Name = trso.Name
	sink.Resources = trso.Resources
}

func (trso *TaskRunSidecarOverride) convertFrom(ctx context.Context, source v1.TaskRunSidecarOverride) {
	trso.Name = source.Name
	trso.Resources = source.Resources
}

func serializeTaskRunResources(meta *metav1.ObjectMeta, spec *TaskRunSpec) error {
	if spec.Resources == nil {
		return nil
	}
	return version.SerializeToMetadata(meta, spec.Resources, resourcesAnnotationKey)
}

func deserializeTaskRunResources(meta *metav1.ObjectMeta, spec *TaskRunSpec) error {
	resources := &TaskRunResources{}
	err := version.DeserializeFromMetadata(meta, resources, resourcesAnnotationKey)
	if err != nil {
		return err
	}
	if resources.Inputs != nil || resources.Outputs != nil {
		spec.Resources = resources
	}
	return nil
}
