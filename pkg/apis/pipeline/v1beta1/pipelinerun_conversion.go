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

var _ apis.Convertible = (*PipelineRun)(nil)

// ConvertTo implements apis.Convertible
func (pr *PipelineRun) ConvertTo(ctx context.Context, to apis.Convertible) error {
	if apis.IsInDelete(ctx) {
		return nil
	}
	switch sink := to.(type) {
	case *v1.PipelineRun:
		sink.ObjectMeta = pr.ObjectMeta
		if err := serializePipelineRunResources(&sink.ObjectMeta, &pr.Spec); err != nil {
			return err
		}
		return pr.Spec.ConvertTo(ctx, &sink.Spec)
	default:
		return fmt.Errorf("unknown version, got: %T", sink)
	}
}

// ConvertTo implements apis.Convertible
func (prs PipelineRunSpec) ConvertTo(ctx context.Context, sink *v1.PipelineRunSpec) error {
	if prs.PipelineRef != nil {
		sink.PipelineRef = &v1.PipelineRef{}
		prs.PipelineRef.convertTo(ctx, sink.PipelineRef)
	}
	if prs.PipelineSpec != nil {
		sink.PipelineSpec = &v1.PipelineSpec{}
		err := prs.PipelineSpec.ConvertTo(ctx, sink.PipelineSpec)
		if err != nil {
			return err
		}
	}
	sink.Params = nil
	for _, p := range prs.Params {
		new := v1.Param{}
		p.convertTo(ctx, &new)
		sink.Params = append(sink.Params, new)
	}
	sink.Status = v1.PipelineRunSpecStatus(prs.Status)
	if prs.Timeouts != nil {
		sink.Timeouts = &v1.TimeoutFields{}
		prs.Timeouts.convertTo(ctx, sink.Timeouts)
	}
	if prs.Timeout != nil {
		sink.Timeouts = &v1.TimeoutFields{}
		sink.Timeouts.Pipeline = prs.Timeout
	}
	sink.TaskRunTemplate = v1.PipelineTaskRunTemplate{}
	sink.TaskRunTemplate.PodTemplate = prs.PodTemplate
	sink.TaskRunTemplate.ServiceAccountName = prs.ServiceAccountName
	sink.Workspaces = nil
	for _, w := range prs.Workspaces {
		new := v1.WorkspaceBinding{}
		w.convertTo(ctx, &new)
		sink.Workspaces = append(sink.Workspaces, new)
	}
	sink.TaskRunSpecs = nil
	for _, ptrs := range prs.TaskRunSpecs {
		new := v1.PipelineTaskRunSpec{}
		ptrs.convertTo(ctx, &new)
		sink.TaskRunSpecs = append(sink.TaskRunSpecs, new)
	}
	return nil
}

// ConvertFrom implements apis.Convertible
func (pr *PipelineRun) ConvertFrom(ctx context.Context, from apis.Convertible) error {
	switch source := from.(type) {
	case *v1.PipelineRun:
		pr.ObjectMeta = source.ObjectMeta
		if err := deserializePipelineRunResources(&pr.ObjectMeta, &pr.Spec); err != nil {
			return err
		}
		return pr.Spec.ConvertFrom(ctx, &source.Spec)
	default:
		return fmt.Errorf("unknown version, got: %T", pr)
	}
}

// ConvertFrom implements apis.Convertible
func (prs *PipelineRunSpec) ConvertFrom(ctx context.Context, source *v1.PipelineRunSpec) error {
	if source.PipelineRef != nil {
		newPipelineRef := PipelineRef{}
		newPipelineRef.convertFrom(ctx, *source.PipelineRef)
		prs.PipelineRef = &newPipelineRef
	}
	if source.PipelineSpec != nil {
		newPipelineSpec := PipelineSpec{}
		err := newPipelineSpec.ConvertFrom(ctx, source.PipelineSpec)
		if err != nil {
			return err
		}
		prs.PipelineSpec = &newPipelineSpec
	}
	prs.Params = nil
	for _, p := range source.Params {
		new := Param{}
		new.convertFrom(ctx, p)
		prs.Params = append(prs.Params, new)
	}
	prs.ServiceAccountName = source.TaskRunTemplate.ServiceAccountName
	prs.Status = PipelineRunSpecStatus(source.Status)
	if source.Timeouts != nil {
		newTimeouts := &TimeoutFields{}
		newTimeouts.convertFrom(ctx, *source.Timeouts)
		prs.Timeouts = newTimeouts
	}
	prs.PodTemplate = source.TaskRunTemplate.PodTemplate
	prs.Workspaces = nil
	for _, w := range source.Workspaces {
		new := WorkspaceBinding{}
		new.convertFrom(ctx, w)
		prs.Workspaces = append(prs.Workspaces, new)
	}
	prs.TaskRunSpecs = nil
	for _, trs := range source.TaskRunSpecs {
		new := PipelineTaskRunSpec{}
		new.convertFrom(ctx, trs)
		prs.TaskRunSpecs = append(prs.TaskRunSpecs, new)
	}
	return nil
}

func (tf TimeoutFields) convertTo(ctx context.Context, sink *v1.TimeoutFields) {
	sink.Pipeline = tf.Pipeline
	sink.Tasks = tf.Tasks
	sink.Finally = tf.Finally
}

func (tf *TimeoutFields) convertFrom(ctx context.Context, source v1.TimeoutFields) {
	tf.Pipeline = source.Pipeline
	tf.Tasks = source.Tasks
	tf.Finally = source.Finally
}

func (ptrs PipelineTaskRunSpec) convertTo(ctx context.Context, sink *v1.PipelineTaskRunSpec) {
	sink.PipelineTaskName = ptrs.PipelineTaskName
	sink.ServiceAccountName = ptrs.TaskServiceAccountName
	sink.PodTemplate = ptrs.TaskPodTemplate
	sink.StepSpecs = nil
	for _, so := range ptrs.StepOverrides {
		new := v1.TaskRunStepSpec{}
		so.convertTo(ctx, &new)
		sink.StepSpecs = append(sink.StepSpecs, new)
	}
	sink.SidecarSpecs = nil
	for _, so := range ptrs.SidecarOverrides {
		new := v1.TaskRunSidecarSpec{}
		so.convertTo(ctx, &new)
		sink.SidecarSpecs = append(sink.SidecarSpecs, new)
	}
	if ptrs.Metadata != nil {
		sink.Metadata = &v1.PipelineTaskMetadata{}
		ptrs.Metadata.convertTo(ctx, sink.Metadata)
	}
	sink.ComputeResources = ptrs.ComputeResources
}

func (ptrs *PipelineTaskRunSpec) convertFrom(ctx context.Context, source v1.PipelineTaskRunSpec) {
	ptrs.PipelineTaskName = source.PipelineTaskName
	ptrs.TaskServiceAccountName = source.ServiceAccountName
	ptrs.TaskPodTemplate = source.PodTemplate
	ptrs.StepOverrides = nil
	for _, so := range source.StepSpecs {
		new := TaskRunStepOverride{}
		new.convertFrom(ctx, so)
		ptrs.StepOverrides = append(ptrs.StepOverrides, new)
	}
	ptrs.SidecarOverrides = nil
	for _, so := range source.SidecarSpecs {
		new := TaskRunSidecarOverride{}
		new.convertFrom(ctx, so)
		ptrs.SidecarOverrides = append(ptrs.SidecarOverrides, new)
	}
	if source.Metadata != nil {
		newMetadata := PipelineTaskMetadata{}
		newMetadata.convertFrom(ctx, *source.Metadata)
		ptrs.Metadata = &newMetadata
	}
	ptrs.ComputeResources = source.ComputeResources
}

func serializePipelineRunResources(meta *metav1.ObjectMeta, spec *PipelineRunSpec) error {
	if spec.Resources == nil {
		return nil
	}
	return version.SerializeToMetadata(meta, spec.Resources, resourcesAnnotationKey)
}

func deserializePipelineRunResources(meta *metav1.ObjectMeta, spec *PipelineRunSpec) error {
	resources := []PipelineResourceBinding{}
	err := version.DeserializeFromMetadata(meta, &resources, resourcesAnnotationKey)
	if err != nil {
		return err
	}
	if len(resources) != 0 {
		spec.Resources = resources
	}
	return nil
}
