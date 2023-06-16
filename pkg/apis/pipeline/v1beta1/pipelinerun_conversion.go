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

var _ apis.Convertible = (*PipelineRun)(nil)

// ConvertTo implements apis.Convertible
func (pr *PipelineRun) ConvertTo(ctx context.Context, to apis.Convertible) error {
	if apis.IsInDelete(ctx) {
		return nil
	}
	switch sink := to.(type) {
	case *v1.PipelineRun:
		sink.ObjectMeta = pr.ObjectMeta
		if err := pr.Status.convertTo(ctx, &sink.Status, &sink.ObjectMeta); err != nil {
			return err
		}
		return pr.Spec.ConvertTo(ctx, &sink.Spec, &sink.ObjectMeta)
	default:
		return fmt.Errorf("unknown version, got: %T", sink)
	}
}

// ConvertTo implements apis.Convertible
func (prs PipelineRunSpec) ConvertTo(ctx context.Context, sink *v1.PipelineRunSpec, meta *metav1.ObjectMeta) error {
	if prs.PipelineRef != nil {
		sink.PipelineRef = &v1.PipelineRef{}
		prs.PipelineRef.convertTo(ctx, sink.PipelineRef)
	}
	if prs.PipelineSpec != nil {
		sink.PipelineSpec = &v1.PipelineSpec{}
		err := prs.PipelineSpec.ConvertTo(ctx, sink.PipelineSpec, meta)
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
		if err := pr.Status.convertFrom(ctx, &source.Status, &pr.ObjectMeta); err != nil {
			return err
		}
		return pr.Spec.ConvertFrom(ctx, &source.Spec, &pr.ObjectMeta)
	default:
		return fmt.Errorf("unknown version, got: %T", pr)
	}
}

// ConvertFrom implements apis.Convertible
func (prs *PipelineRunSpec) ConvertFrom(ctx context.Context, source *v1.PipelineRunSpec, meta *metav1.ObjectMeta) error {
	if source.PipelineRef != nil {
		newPipelineRef := PipelineRef{}
		newPipelineRef.convertFrom(ctx, *source.PipelineRef)
		prs.PipelineRef = &newPipelineRef
	}
	if source.PipelineSpec != nil {
		newPipelineSpec := PipelineSpec{}
		err := newPipelineSpec.ConvertFrom(ctx, source.PipelineSpec, meta)
		if err != nil {
			return err
		}
		prs.PipelineSpec = &newPipelineSpec
	}
	prs.Params = nil
	for _, p := range source.Params {
		new := Param{}
		new.ConvertFrom(ctx, p)
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
		new.ConvertFrom(ctx, w)
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

func (prs *PipelineRunStatus) convertTo(ctx context.Context, sink *v1.PipelineRunStatus, meta *metav1.ObjectMeta) error {
	sink.Status = prs.Status
	sink.StartTime = prs.StartTime
	sink.CompletionTime = prs.CompletionTime
	sink.Results = nil
	for _, pr := range prs.PipelineResults {
		new := v1.PipelineRunResult{}
		pr.convertTo(ctx, &new)
		sink.Results = append(sink.Results, new)
	}
	if prs.PipelineSpec != nil {
		sink.PipelineSpec = &v1.PipelineSpec{}
		err := prs.PipelineSpec.ConvertTo(ctx, sink.PipelineSpec, meta)
		if err != nil {
			return err
		}
	}
	sink.SkippedTasks = nil
	for _, st := range prs.SkippedTasks {
		new := v1.SkippedTask{}
		st.convertTo(ctx, &new)
		sink.SkippedTasks = append(sink.SkippedTasks, new)
	}
	sink.ChildReferences = nil
	for _, cr := range prs.ChildReferences {
		new := v1.ChildStatusReference{}
		cr.convertTo(ctx, &new)
		sink.ChildReferences = append(sink.ChildReferences, new)
	}
	sink.FinallyStartTime = prs.FinallyStartTime
	if prs.Provenance != nil {
		new := v1.Provenance{}
		prs.Provenance.convertTo(ctx, &new)
		sink.Provenance = &new
	}
	return nil
}

func (prs *PipelineRunStatus) convertFrom(ctx context.Context, source *v1.PipelineRunStatus, meta *metav1.ObjectMeta) error {
	prs.Status = source.Status
	prs.StartTime = source.StartTime
	prs.CompletionTime = source.CompletionTime
	prs.PipelineResults = nil
	for _, pr := range source.Results {
		new := PipelineRunResult{}
		new.convertFrom(ctx, pr)
		prs.PipelineResults = append(prs.PipelineResults, new)
	}
	if source.PipelineSpec != nil {
		newPipelineSpec := PipelineSpec{}
		err := newPipelineSpec.ConvertFrom(ctx, source.PipelineSpec, meta)
		if err != nil {
			return err
		}
		prs.PipelineSpec = &newPipelineSpec
	}
	prs.SkippedTasks = nil
	for _, st := range source.SkippedTasks {
		new := SkippedTask{}
		new.convertFrom(ctx, st)
		prs.SkippedTasks = append(prs.SkippedTasks, new)
	}
	prs.ChildReferences = nil
	for _, cr := range source.ChildReferences {
		new := ChildStatusReference{}
		new.convertFrom(ctx, cr)
		prs.ChildReferences = append(prs.ChildReferences, new)
	}

	prs.FinallyStartTime = source.FinallyStartTime
	if source.Provenance != nil {
		new := Provenance{}
		new.convertFrom(ctx, *source.Provenance)
		prs.Provenance = &new
	}
	return nil
}

func (prr PipelineRunResult) convertTo(ctx context.Context, sink *v1.PipelineRunResult) {
	sink.Name = prr.Name
	newValue := v1.ParamValue{}
	prr.Value.convertTo(ctx, &newValue)
	sink.Value = newValue
}

func (prr *PipelineRunResult) convertFrom(ctx context.Context, source v1.PipelineRunResult) {
	prr.Name = source.Name
	newValue := ParamValue{}
	newValue.convertFrom(ctx, source.Value)
	prr.Value = newValue
}

func (st SkippedTask) convertTo(ctx context.Context, sink *v1.SkippedTask) {
	sink.Name = st.Name
	sink.Reason = v1.SkippingReason(st.Reason)
	sink.WhenExpressions = nil
	for _, we := range st.WhenExpressions {
		new := v1.WhenExpression{}
		we.convertTo(ctx, &new)
		sink.WhenExpressions = append(sink.WhenExpressions, new)
	}
}

func (st *SkippedTask) convertFrom(ctx context.Context, source v1.SkippedTask) {
	st.Name = source.Name
	st.Reason = SkippingReason(source.Reason)
	st.WhenExpressions = nil
	for _, we := range source.WhenExpressions {
		new := WhenExpression{}
		new.convertFrom(ctx, we)
		st.WhenExpressions = append(st.WhenExpressions, new)
	}
}

func (csr ChildStatusReference) convertTo(ctx context.Context, sink *v1.ChildStatusReference) {
	sink.TypeMeta = csr.TypeMeta
	sink.Name = csr.Name
	sink.PipelineTaskName = csr.PipelineTaskName
	sink.WhenExpressions = nil
	for _, we := range csr.WhenExpressions {
		new := v1.WhenExpression{}
		we.convertTo(ctx, &new)
		sink.WhenExpressions = append(sink.WhenExpressions, new)
	}
}

func (csr *ChildStatusReference) convertFrom(ctx context.Context, source v1.ChildStatusReference) {
	csr.TypeMeta = source.TypeMeta
	csr.Name = source.Name
	csr.PipelineTaskName = source.PipelineTaskName
	csr.WhenExpressions = nil
	for _, we := range source.WhenExpressions {
		new := WhenExpression{}
		new.convertFrom(ctx, we)
		csr.WhenExpressions = append(csr.WhenExpressions, new)
	}
}
