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

const (
	cloudEventsAnnotationKey     = "tekton.dev/v1beta1CloudEvents"
	resourcesResultAnnotationKey = "tekton.dev/v1beta1ResourcesResult"
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
		if err := serializeTaskRunCloudEvents(&sink.ObjectMeta, &tr.Status); err != nil {
			return err
		}
		if err := tr.Status.ConvertTo(ctx, &sink.Status, &sink.ObjectMeta); err != nil {
			return err
		}
		return tr.Spec.ConvertTo(ctx, &sink.Spec, &sink.ObjectMeta)
	default:
		return fmt.Errorf("unknown version, got: %T", sink)
	}
}

// ConvertTo implements apis.Convertible
func (trs *TaskRunSpec) ConvertTo(ctx context.Context, sink *v1.TaskRunSpec, meta *metav1.ObjectMeta) error {
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
		err := trs.TaskSpec.ConvertTo(ctx, sink.TaskSpec, meta, meta.Name)
		if err != nil {
			return err
		}
	}
	sink.Status = v1.TaskRunSpecStatus(trs.Status)
	sink.StatusMessage = v1.TaskRunSpecStatusMessage(trs.StatusMessage)
	sink.Retries = trs.Retries
	sink.Timeout = trs.Timeout
	sink.PodTemplate = trs.PodTemplate
	sink.Workspaces = nil
	for _, w := range trs.Workspaces {
		new := v1.WorkspaceBinding{}
		w.convertTo(ctx, &new)
		sink.Workspaces = append(sink.Workspaces, new)
	}
	sink.StepSpecs = nil
	for _, so := range trs.StepOverrides {
		new := v1.TaskRunStepSpec{}
		so.convertTo(ctx, &new)
		sink.StepSpecs = append(sink.StepSpecs, new)
	}
	sink.SidecarSpecs = nil
	for _, so := range trs.SidecarOverrides {
		new := v1.TaskRunSidecarSpec{}
		so.convertTo(ctx, &new)
		sink.SidecarSpecs = append(sink.SidecarSpecs, new)
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
		if err := deserializeTaskRunCloudEvents(&tr.ObjectMeta, &tr.Status); err != nil {
			return err
		}
		if err := tr.Status.ConvertFrom(ctx, source.Status, &tr.ObjectMeta); err != nil {
			return err
		}
		return tr.Spec.ConvertFrom(ctx, &source.Spec, &tr.ObjectMeta)
	default:
		return fmt.Errorf("unknown version, got: %T", tr)
	}
}

// ConvertFrom implements apis.Convertible
func (trs *TaskRunSpec) ConvertFrom(ctx context.Context, source *v1.TaskRunSpec, meta *metav1.ObjectMeta) error {
	if source.Debug != nil {
		newDebug := TaskRunDebug{}
		newDebug.convertFrom(ctx, *source.Debug)
		trs.Debug = &newDebug
	}
	trs.Params = nil
	for _, p := range source.Params {
		new := Param{}
		new.ConvertFrom(ctx, p)
		trs.Params = append(trs.Params, new)
	}
	trs.ServiceAccountName = source.ServiceAccountName
	if source.TaskRef != nil {
		newTaskRef := TaskRef{}
		newTaskRef.ConvertFrom(ctx, *source.TaskRef)
		trs.TaskRef = &newTaskRef
	}
	if source.TaskSpec != nil {
		newTaskSpec := TaskSpec{}
		err := newTaskSpec.ConvertFrom(ctx, source.TaskSpec, meta, meta.Name)
		if err != nil {
			return err
		}
		trs.TaskSpec = &newTaskSpec
	}
	trs.Status = TaskRunSpecStatus(source.Status)
	trs.StatusMessage = TaskRunSpecStatusMessage(source.StatusMessage)
	trs.Retries = source.Retries
	trs.Timeout = source.Timeout
	trs.PodTemplate = source.PodTemplate
	trs.Workspaces = nil
	for _, w := range source.Workspaces {
		new := WorkspaceBinding{}
		new.ConvertFrom(ctx, w)
		trs.Workspaces = append(trs.Workspaces, new)
	}
	trs.StepOverrides = nil
	for _, so := range source.StepSpecs {
		new := TaskRunStepOverride{}
		new.convertFrom(ctx, so)
		trs.StepOverrides = append(trs.StepOverrides, new)
	}
	trs.SidecarOverrides = nil
	for _, so := range source.SidecarSpecs {
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

func (trso TaskRunStepOverride) convertTo(ctx context.Context, sink *v1.TaskRunStepSpec) {
	sink.Name = trso.Name
	sink.ComputeResources = trso.Resources
}

func (trso *TaskRunStepOverride) convertFrom(ctx context.Context, source v1.TaskRunStepSpec) {
	trso.Name = source.Name
	trso.Resources = source.ComputeResources
}

func (trso TaskRunSidecarOverride) convertTo(ctx context.Context, sink *v1.TaskRunSidecarSpec) {
	sink.Name = trso.Name
	sink.ComputeResources = trso.Resources
}

func (trso *TaskRunSidecarOverride) convertFrom(ctx context.Context, source v1.TaskRunSidecarSpec) {
	trso.Name = source.Name
	trso.Resources = source.ComputeResources
}

// ConvertTo implements apis.Convertible
func (trs *TaskRunStatus) ConvertTo(ctx context.Context, sink *v1.TaskRunStatus, meta *metav1.ObjectMeta) error {
	sink.Status = trs.Status
	sink.PodName = trs.PodName
	sink.StartTime = trs.StartTime
	sink.CompletionTime = trs.CompletionTime
	sink.Steps = nil
	for _, ss := range trs.Steps {
		new := v1.StepState{}
		ss.convertTo(ctx, &new)
		sink.Steps = append(sink.Steps, new)
	}
	sink.RetriesStatus = nil
	for _, rr := range trs.RetriesStatus {
		new := v1.TaskRunStatus{}
		err := rr.ConvertTo(ctx, &new, meta)
		if err != nil {
			return err
		}
		sink.RetriesStatus = append(sink.RetriesStatus, new)
	}
	sink.Results = nil
	for _, trr := range trs.TaskRunResults {
		new := v1.TaskRunResult{}
		trr.convertTo(ctx, &new)
		sink.Results = append(sink.Results, new)
	}
	sink.Sidecars = nil
	for _, sc := range trs.Sidecars {
		new := v1.SidecarState{}
		sc.convertTo(ctx, &new)
		sink.Sidecars = append(sink.Sidecars, new)
	}

	if trs.TaskSpec != nil {
		sink.TaskSpec = &v1.TaskSpec{}
		err := trs.TaskSpec.ConvertTo(ctx, sink.TaskSpec, meta, meta.Name)
		if err != nil {
			return err
		}
	}
	if trs.Provenance != nil {
		new := v1.Provenance{}
		trs.Provenance.convertTo(ctx, &new)
		sink.Provenance = &new
	}
	return nil
}

// ConvertFrom implements apis.Convertible
func (trs *TaskRunStatus) ConvertFrom(ctx context.Context, source v1.TaskRunStatus, meta *metav1.ObjectMeta) error {
	trs.Status = source.Status
	trs.PodName = source.PodName
	trs.StartTime = source.StartTime
	trs.CompletionTime = source.CompletionTime
	trs.Steps = nil
	for _, ss := range source.Steps {
		new := StepState{}
		new.convertFrom(ctx, ss)
		trs.Steps = append(trs.Steps, new)
	}
	trs.RetriesStatus = nil
	for _, rr := range source.RetriesStatus {
		new := TaskRunStatus{}
		err := new.ConvertFrom(ctx, rr, meta)
		if err != nil {
			return err
		}
		trs.RetriesStatus = append(trs.RetriesStatus, new)
	}
	trs.TaskRunResults = nil
	for _, trr := range source.Results {
		new := TaskRunResult{}
		new.convertFrom(ctx, trr)
		trs.TaskRunResults = append(trs.TaskRunResults, new)
	}
	trs.Sidecars = nil
	for _, sc := range source.Sidecars {
		new := SidecarState{}
		new.convertFrom(ctx, sc)
		trs.Sidecars = append(trs.Sidecars, new)
	}

	if source.TaskSpec != nil {
		trs.TaskSpec = &TaskSpec{}
		err := trs.TaskSpec.ConvertFrom(ctx, source.TaskSpec, meta, meta.Name)
		if err != nil {
			return err
		}
	}
	if source.Provenance != nil {
		new := Provenance{}
		new.convertFrom(ctx, *source.Provenance)
		trs.Provenance = &new
	}
	return nil
}

func (ss StepState) convertTo(ctx context.Context, sink *v1.StepState) {
	sink.ContainerState = ss.ContainerState
	sink.Name = ss.Name
	sink.Container = ss.ContainerName
	sink.ImageID = ss.ImageID
}

func (ss *StepState) convertFrom(ctx context.Context, source v1.StepState) {
	ss.ContainerState = source.ContainerState
	ss.Name = source.Name
	ss.ContainerName = source.Container
	ss.ImageID = source.ImageID
}

func (trr TaskRunResult) convertTo(ctx context.Context, sink *v1.TaskRunResult) {
	sink.Name = trr.Name
	sink.Type = v1.ResultsType(trr.Type)
	newValue := v1.ParamValue{}
	trr.Value.convertTo(ctx, &newValue)
	sink.Value = newValue
}

func (trr *TaskRunResult) convertFrom(ctx context.Context, source v1.TaskRunResult) {
	trr.Name = source.Name
	trr.Type = ResultsType(source.Type)
	newValue := ParamValue{}
	newValue.convertFrom(ctx, source.Value)
	trr.Value = newValue
}

func (ss SidecarState) convertTo(ctx context.Context, sink *v1.SidecarState) {
	sink.ContainerState = ss.ContainerState
	sink.Name = ss.Name
	sink.Container = ss.ContainerName
	sink.ImageID = ss.ImageID
}

func (ss *SidecarState) convertFrom(ctx context.Context, source v1.SidecarState) {
	ss.ContainerState = source.ContainerState
	ss.Name = source.Name
	ss.ContainerName = source.Container
	ss.ImageID = source.ImageID
}

func serializeTaskRunCloudEvents(meta *metav1.ObjectMeta, status *TaskRunStatus) error {
	if status.CloudEvents == nil {
		return nil
	}
	return version.SerializeToMetadata(meta, status.CloudEvents, cloudEventsAnnotationKey)
}

func deserializeTaskRunCloudEvents(meta *metav1.ObjectMeta, status *TaskRunStatus) error {
	cloudEvents := []CloudEventDelivery{}
	err := version.DeserializeFromMetadata(meta, &cloudEvents, cloudEventsAnnotationKey)
	if err != nil {
		return err
	}
	if len(cloudEvents) != 0 {
		status.CloudEvents = cloudEvents
	}
	return nil
}
