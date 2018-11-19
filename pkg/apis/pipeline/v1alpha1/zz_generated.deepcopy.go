// +build !ignore_autogenerated

/*
Copyright 2018 The Knative Authors

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

// This file was autogenerated by deepcopy-gen. Do not edit it manually!

package v1alpha1

import (
	build_v1alpha1 "github.com/knative/build/pkg/apis/build/v1alpha1"
	duck_v1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	v1 "k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ClusterResource) DeepCopyInto(out *ClusterResource) {
	*out = *in
	if in.CAData != nil {
		in, out := &in.CAData, &out.CAData
		*out = make([]byte, len(*in))
		copy(*out, *in)
	}
	if in.Secrets != nil {
		in, out := &in.Secrets, &out.Secrets
		*out = make([]SecretParam, len(*in))
		copy(*out, *in)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ClusterResource.
func (in *ClusterResource) DeepCopy() *ClusterResource {
	if in == nil {
		return nil
	}
	out := new(ClusterResource)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *GitResource) DeepCopyInto(out *GitResource) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new GitResource.
func (in *GitResource) DeepCopy() *GitResource {
	if in == nil {
		return nil
	}
	out := new(GitResource)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ImageResource) DeepCopyInto(out *ImageResource) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ImageResource.
func (in *ImageResource) DeepCopy() *ImageResource {
	if in == nil {
		return nil
	}
	out := new(ImageResource)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Inputs) DeepCopyInto(out *Inputs) {
	*out = *in
	if in.Resources != nil {
		in, out := &in.Resources, &out.Resources
		*out = make([]TaskResource, len(*in))
		copy(*out, *in)
	}
	if in.Params != nil {
		in, out := &in.Params, &out.Params
		*out = make([]TaskParam, len(*in))
		copy(*out, *in)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Inputs.
func (in *Inputs) DeepCopy() *Inputs {
	if in == nil {
		return nil
	}
	out := new(Inputs)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Outputs) DeepCopyInto(out *Outputs) {
	*out = *in
	if in.Results != nil {
		in, out := &in.Results, &out.Results
		*out = make([]TestResult, len(*in))
		copy(*out, *in)
	}
	if in.Resources != nil {
		in, out := &in.Resources, &out.Resources
		*out = make([]TaskResource, len(*in))
		copy(*out, *in)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Outputs.
func (in *Outputs) DeepCopy() *Outputs {
	if in == nil {
		return nil
	}
	out := new(Outputs)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Param) DeepCopyInto(out *Param) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Param.
func (in *Param) DeepCopy() *Param {
	if in == nil {
		return nil
	}
	out := new(Param)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Pipeline) DeepCopyInto(out *Pipeline) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	out.Status = in.Status
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Pipeline.
func (in *Pipeline) DeepCopy() *Pipeline {
	if in == nil {
		return nil
	}
	out := new(Pipeline)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *Pipeline) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PipelineList) DeepCopyInto(out *PipelineList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	out.ListMeta = in.ListMeta
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]Pipeline, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PipelineList.
func (in *PipelineList) DeepCopy() *PipelineList {
	if in == nil {
		return nil
	}
	out := new(PipelineList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *PipelineList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PipelineParams) DeepCopyInto(out *PipelineParams) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	out.Status = in.Status
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PipelineParams.
func (in *PipelineParams) DeepCopy() *PipelineParams {
	if in == nil {
		return nil
	}
	out := new(PipelineParams)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *PipelineParams) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PipelineParamsList) DeepCopyInto(out *PipelineParamsList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	out.ListMeta = in.ListMeta
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]PipelineParams, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PipelineParamsList.
func (in *PipelineParamsList) DeepCopy() *PipelineParamsList {
	if in == nil {
		return nil
	}
	out := new(PipelineParamsList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *PipelineParamsList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PipelineParamsRef) DeepCopyInto(out *PipelineParamsRef) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PipelineParamsRef.
func (in *PipelineParamsRef) DeepCopy() *PipelineParamsRef {
	if in == nil {
		return nil
	}
	out := new(PipelineParamsRef)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PipelineParamsSpec) DeepCopyInto(out *PipelineParamsSpec) {
	*out = *in
	in.Results.DeepCopyInto(&out.Results)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PipelineParamsSpec.
func (in *PipelineParamsSpec) DeepCopy() *PipelineParamsSpec {
	if in == nil {
		return nil
	}
	out := new(PipelineParamsSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PipelineParamsStatus) DeepCopyInto(out *PipelineParamsStatus) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PipelineParamsStatus.
func (in *PipelineParamsStatus) DeepCopy() *PipelineParamsStatus {
	if in == nil {
		return nil
	}
	out := new(PipelineParamsStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PipelineRef) DeepCopyInto(out *PipelineRef) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PipelineRef.
func (in *PipelineRef) DeepCopy() *PipelineRef {
	if in == nil {
		return nil
	}
	out := new(PipelineRef)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PipelineResource) DeepCopyInto(out *PipelineResource) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	out.Status = in.Status
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PipelineResource.
func (in *PipelineResource) DeepCopy() *PipelineResource {
	if in == nil {
		return nil
	}
	out := new(PipelineResource)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *PipelineResource) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PipelineResourceList) DeepCopyInto(out *PipelineResourceList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	out.ListMeta = in.ListMeta
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]PipelineResource, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PipelineResourceList.
func (in *PipelineResourceList) DeepCopy() *PipelineResourceList {
	if in == nil {
		return nil
	}
	out := new(PipelineResourceList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *PipelineResourceList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PipelineResourceRef) DeepCopyInto(out *PipelineResourceRef) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PipelineResourceRef.
func (in *PipelineResourceRef) DeepCopy() *PipelineResourceRef {
	if in == nil {
		return nil
	}
	out := new(PipelineResourceRef)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PipelineResourceSpec) DeepCopyInto(out *PipelineResourceSpec) {
	*out = *in
	if in.Params != nil {
		in, out := &in.Params, &out.Params
		*out = make([]Param, len(*in))
		copy(*out, *in)
	}
	if in.SecretParams != nil {
		in, out := &in.SecretParams, &out.SecretParams
		*out = make([]SecretParam, len(*in))
		copy(*out, *in)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PipelineResourceSpec.
func (in *PipelineResourceSpec) DeepCopy() *PipelineResourceSpec {
	if in == nil {
		return nil
	}
	out := new(PipelineResourceSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PipelineResourceStatus) DeepCopyInto(out *PipelineResourceStatus) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PipelineResourceStatus.
func (in *PipelineResourceStatus) DeepCopy() *PipelineResourceStatus {
	if in == nil {
		return nil
	}
	out := new(PipelineResourceStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PipelineRun) DeepCopyInto(out *PipelineRun) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	out.Spec = in.Spec
	in.Status.DeepCopyInto(&out.Status)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PipelineRun.
func (in *PipelineRun) DeepCopy() *PipelineRun {
	if in == nil {
		return nil
	}
	out := new(PipelineRun)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *PipelineRun) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PipelineRunList) DeepCopyInto(out *PipelineRunList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	out.ListMeta = in.ListMeta
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]PipelineRun, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PipelineRunList.
func (in *PipelineRunList) DeepCopy() *PipelineRunList {
	if in == nil {
		return nil
	}
	out := new(PipelineRunList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *PipelineRunList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PipelineRunSpec) DeepCopyInto(out *PipelineRunSpec) {
	*out = *in
	out.PipelineRef = in.PipelineRef
	out.PipelineParamsRef = in.PipelineParamsRef
	out.PipelineTriggerRef = in.PipelineTriggerRef
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PipelineRunSpec.
func (in *PipelineRunSpec) DeepCopy() *PipelineRunSpec {
	if in == nil {
		return nil
	}
	out := new(PipelineRunSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PipelineRunStatus) DeepCopyInto(out *PipelineRunStatus) {
	*out = *in
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make(duck_v1alpha1.Conditions, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.TaskRuns != nil {
		in, out := &in.TaskRuns, &out.TaskRuns
		*out = make(map[string]TaskRunStatus, len(*in))
		for key, val := range *in {
			newVal := new(TaskRunStatus)
			val.DeepCopyInto(newVal)
			(*out)[key] = *newVal
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PipelineRunStatus.
func (in *PipelineRunStatus) DeepCopy() *PipelineRunStatus {
	if in == nil {
		return nil
	}
	out := new(PipelineRunStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PipelineSpec) DeepCopyInto(out *PipelineSpec) {
	*out = *in
	if in.Tasks != nil {
		in, out := &in.Tasks, &out.Tasks
		*out = make([]PipelineTask, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PipelineSpec.
func (in *PipelineSpec) DeepCopy() *PipelineSpec {
	if in == nil {
		return nil
	}
	out := new(PipelineSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PipelineStatus) DeepCopyInto(out *PipelineStatus) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PipelineStatus.
func (in *PipelineStatus) DeepCopy() *PipelineStatus {
	if in == nil {
		return nil
	}
	out := new(PipelineStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PipelineTask) DeepCopyInto(out *PipelineTask) {
	*out = *in
	out.TaskRef = in.TaskRef
	if in.InputSourceBindings != nil {
		in, out := &in.InputSourceBindings, &out.InputSourceBindings
		*out = make([]SourceBinding, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.OutputSourceBindings != nil {
		in, out := &in.OutputSourceBindings, &out.OutputSourceBindings
		*out = make([]SourceBinding, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.Params != nil {
		in, out := &in.Params, &out.Params
		*out = make([]Param, len(*in))
		copy(*out, *in)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PipelineTask.
func (in *PipelineTask) DeepCopy() *PipelineTask {
	if in == nil {
		return nil
	}
	out := new(PipelineTask)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PipelineTaskParam) DeepCopyInto(out *PipelineTaskParam) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PipelineTaskParam.
func (in *PipelineTaskParam) DeepCopy() *PipelineTaskParam {
	if in == nil {
		return nil
	}
	out := new(PipelineTaskParam)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PipelineTaskRun) DeepCopyInto(out *PipelineTaskRun) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PipelineTaskRun.
func (in *PipelineTaskRun) DeepCopy() *PipelineTaskRun {
	if in == nil {
		return nil
	}
	out := new(PipelineTaskRun)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PipelineTriggerRef) DeepCopyInto(out *PipelineTriggerRef) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PipelineTriggerRef.
func (in *PipelineTriggerRef) DeepCopy() *PipelineTriggerRef {
	if in == nil {
		return nil
	}
	out := new(PipelineTriggerRef)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ResultTarget) DeepCopyInto(out *ResultTarget) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ResultTarget.
func (in *ResultTarget) DeepCopy() *ResultTarget {
	if in == nil {
		return nil
	}
	out := new(ResultTarget)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Results) DeepCopyInto(out *Results) {
	*out = *in
	out.Runs = in.Runs
	out.Logs = in.Logs
	if in.Tests != nil {
		in, out := &in.Tests, &out.Tests
		if *in == nil {
			*out = nil
		} else {
			*out = new(ResultTarget)
			**out = **in
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Results.
func (in *Results) DeepCopy() *Results {
	if in == nil {
		return nil
	}
	out := new(Results)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SecretParam) DeepCopyInto(out *SecretParam) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SecretParam.
func (in *SecretParam) DeepCopy() *SecretParam {
	if in == nil {
		return nil
	}
	out := new(SecretParam)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SourceBinding) DeepCopyInto(out *SourceBinding) {
	*out = *in
	out.ResourceRef = in.ResourceRef
	if in.ProvidedBy != nil {
		in, out := &in.ProvidedBy, &out.ProvidedBy
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SourceBinding.
func (in *SourceBinding) DeepCopy() *SourceBinding {
	if in == nil {
		return nil
	}
	out := new(SourceBinding)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *StepRun) DeepCopyInto(out *StepRun) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new StepRun.
func (in *StepRun) DeepCopy() *StepRun {
	if in == nil {
		return nil
	}
	out := new(StepRun)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Task) DeepCopyInto(out *Task) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	out.Status = in.Status
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Task.
func (in *Task) DeepCopy() *Task {
	if in == nil {
		return nil
	}
	out := new(Task)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *Task) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *TaskList) DeepCopyInto(out *TaskList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	out.ListMeta = in.ListMeta
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]Task, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new TaskList.
func (in *TaskList) DeepCopy() *TaskList {
	if in == nil {
		return nil
	}
	out := new(TaskList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *TaskList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *TaskParam) DeepCopyInto(out *TaskParam) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new TaskParam.
func (in *TaskParam) DeepCopy() *TaskParam {
	if in == nil {
		return nil
	}
	out := new(TaskParam)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *TaskRef) DeepCopyInto(out *TaskRef) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new TaskRef.
func (in *TaskRef) DeepCopy() *TaskRef {
	if in == nil {
		return nil
	}
	out := new(TaskRef)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *TaskResource) DeepCopyInto(out *TaskResource) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new TaskResource.
func (in *TaskResource) DeepCopy() *TaskResource {
	if in == nil {
		return nil
	}
	out := new(TaskResource)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *TaskRun) DeepCopyInto(out *TaskRun) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new TaskRun.
func (in *TaskRun) DeepCopy() *TaskRun {
	if in == nil {
		return nil
	}
	out := new(TaskRun)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *TaskRun) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *TaskRunInputs) DeepCopyInto(out *TaskRunInputs) {
	*out = *in
	if in.Resources != nil {
		in, out := &in.Resources, &out.Resources
		*out = make([]TaskRunResourceVersion, len(*in))
		copy(*out, *in)
	}
	if in.Params != nil {
		in, out := &in.Params, &out.Params
		*out = make([]Param, len(*in))
		copy(*out, *in)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new TaskRunInputs.
func (in *TaskRunInputs) DeepCopy() *TaskRunInputs {
	if in == nil {
		return nil
	}
	out := new(TaskRunInputs)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *TaskRunList) DeepCopyInto(out *TaskRunList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	out.ListMeta = in.ListMeta
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]TaskRun, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new TaskRunList.
func (in *TaskRunList) DeepCopy() *TaskRunList {
	if in == nil {
		return nil
	}
	out := new(TaskRunList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *TaskRunList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *TaskRunOutputs) DeepCopyInto(out *TaskRunOutputs) {
	*out = *in
	if in.Resources != nil {
		in, out := &in.Resources, &out.Resources
		*out = make([]TaskRunResourceVersion, len(*in))
		copy(*out, *in)
	}
	if in.Params != nil {
		in, out := &in.Params, &out.Params
		*out = make([]Param, len(*in))
		copy(*out, *in)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new TaskRunOutputs.
func (in *TaskRunOutputs) DeepCopy() *TaskRunOutputs {
	if in == nil {
		return nil
	}
	out := new(TaskRunOutputs)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *TaskRunResourceVersion) DeepCopyInto(out *TaskRunResourceVersion) {
	*out = *in
	out.ResourceRef = in.ResourceRef
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new TaskRunResourceVersion.
func (in *TaskRunResourceVersion) DeepCopy() *TaskRunResourceVersion {
	if in == nil {
		return nil
	}
	out := new(TaskRunResourceVersion)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *TaskRunSpec) DeepCopyInto(out *TaskRunSpec) {
	*out = *in
	out.TaskRef = in.TaskRef
	out.Trigger = in.Trigger
	in.Inputs.DeepCopyInto(&out.Inputs)
	in.Outputs.DeepCopyInto(&out.Outputs)
	in.Results.DeepCopyInto(&out.Results)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new TaskRunSpec.
func (in *TaskRunSpec) DeepCopy() *TaskRunSpec {
	if in == nil {
		return nil
	}
	out := new(TaskRunSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *TaskRunStatus) DeepCopyInto(out *TaskRunStatus) {
	*out = *in
	if in.Steps != nil {
		in, out := &in.Steps, &out.Steps
		*out = make([]StepRun, len(*in))
		copy(*out, *in)
	}
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make(duck_v1alpha1.Conditions, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new TaskRunStatus.
func (in *TaskRunStatus) DeepCopy() *TaskRunStatus {
	if in == nil {
		return nil
	}
	out := new(TaskRunStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *TaskSpec) DeepCopyInto(out *TaskSpec) {
	*out = *in
	if in.Inputs != nil {
		in, out := &in.Inputs, &out.Inputs
		if *in == nil {
			*out = nil
		} else {
			*out = new(Inputs)
			(*in).DeepCopyInto(*out)
		}
	}
	if in.Outputs != nil {
		in, out := &in.Outputs, &out.Outputs
		if *in == nil {
			*out = nil
		} else {
			*out = new(Outputs)
			(*in).DeepCopyInto(*out)
		}
	}
	if in.Sources != nil {
		in, out := &in.Sources, &out.Sources
		*out = make([]build_v1alpha1.SourceSpec, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.Steps != nil {
		in, out := &in.Steps, &out.Steps
		*out = make([]v1.Container, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.Volumes != nil {
		in, out := &in.Volumes, &out.Volumes
		*out = make([]v1.Volume, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.NodeSelector != nil {
		in, out := &in.NodeSelector, &out.NodeSelector
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	if in.Timeout != nil {
		in, out := &in.Timeout, &out.Timeout
		if *in == nil {
			*out = nil
		} else {
			*out = new(meta_v1.Duration)
			**out = **in
		}
	}
	if in.Affinity != nil {
		in, out := &in.Affinity, &out.Affinity
		if *in == nil {
			*out = nil
		} else {
			*out = new(v1.Affinity)
			(*in).DeepCopyInto(*out)
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new TaskSpec.
func (in *TaskSpec) DeepCopy() *TaskSpec {
	if in == nil {
		return nil
	}
	out := new(TaskSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *TaskStatus) DeepCopyInto(out *TaskStatus) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new TaskStatus.
func (in *TaskStatus) DeepCopy() *TaskStatus {
	if in == nil {
		return nil
	}
	out := new(TaskStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *TaskTrigger) DeepCopyInto(out *TaskTrigger) {
	*out = *in
	out.TriggerRef = in.TriggerRef
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new TaskTrigger.
func (in *TaskTrigger) DeepCopy() *TaskTrigger {
	if in == nil {
		return nil
	}
	out := new(TaskTrigger)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *TaskTriggerRef) DeepCopyInto(out *TaskTriggerRef) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new TaskTriggerRef.
func (in *TaskTriggerRef) DeepCopy() *TaskTriggerRef {
	if in == nil {
		return nil
	}
	out := new(TaskTriggerRef)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *TestResult) DeepCopyInto(out *TestResult) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new TestResult.
func (in *TestResult) DeepCopy() *TestResult {
	if in == nil {
		return nil
	}
	out := new(TestResult)
	in.DeepCopyInto(out)
	return out
}
