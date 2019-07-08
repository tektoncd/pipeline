/*
Copyright 2019 The Tekton Authors
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

package builder

import (
	"time"

	"github.com/knative/pkg/apis"
	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/reconciler/v1alpha1/taskrun/resources"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TaskOp is an operation which modify a Task struct.
type TaskOp func(*v1alpha1.Task)

// ClusterTaskOp is an operation which modify a ClusterTask struct.
type ClusterTaskOp func(*v1alpha1.ClusterTask)

// TaskSpeOp is an operation which modify a TaskSpec struct.
type TaskSpecOp func(*v1alpha1.TaskSpec)

// InputsOp is an operation which modify an Inputs struct.
type InputsOp func(*v1alpha1.Inputs)

// OutputsOp is an operation which modify an Outputs struct.
type OutputsOp func(*v1alpha1.Outputs)

// TaskParamOp is an operation which modify a ParamSpec struct.
type TaskParamOp func(*v1alpha1.ParamSpec)

// TaskRunOp is an operation which modify a TaskRun struct.
type TaskRunOp func(*v1alpha1.TaskRun)

// TaskRunSpecOp is an operation which modify a TaskRunSpec struct.
type TaskRunSpecOp func(*v1alpha1.TaskRunSpec)

// TaskResourceOp is an operation which modify a TaskResource struct.
type TaskResourceOp func(*v1alpha1.TaskResource)

// TaskResourceBindingOp is an operation which modify a TaskResourceBinding struct.
type TaskResourceBindingOp func(*v1alpha1.TaskResourceBinding)

// TaskRunStatusOp is an operation which modify a TaskRunStatus struct.
type TaskRunStatusOp func(*v1alpha1.TaskRunStatus)

// TaskRefOp is an operation which modify a TaskRef struct.
type TaskRefOp func(*v1alpha1.TaskRef)

// TaskRunInputsOp is an operation which modify a TaskRunInputs struct.
type TaskRunInputsOp func(*v1alpha1.TaskRunInputs)

// TaskRunOutputsOp is an operation which modify a TaskRunOutputs struct.
type TaskRunOutputsOp func(*v1alpha1.TaskRunOutputs)

// ResolvedTaskResourcesOp is an operation which modify a ResolvedTaskResources struct.
type ResolvedTaskResourcesOp func(*resources.ResolvedTaskResources)

// StepStateOp is an operation which modify a StepStep struct.
type StepStateOp func(*v1alpha1.StepState)

// VolumeOp is an operation which modify a Volume struct.
type VolumeOp func(*corev1.Volume)

var (
	trueB = true
)

// Task creates a Task with default values.
// Any number of Task modifier can be passed to transform it.
func Task(name, namespace string, ops ...TaskOp) *v1alpha1.Task {
	t := &v1alpha1.Task{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
	}

	for _, op := range ops {
		op(t)
	}

	return t
}

// ClusterTask creates a ClusterTask with default values.
// Any number of ClusterTask modifier can be passed to transform it.
func ClusterTask(name string, ops ...ClusterTaskOp) *v1alpha1.ClusterTask {
	t := &v1alpha1.ClusterTask{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}

	for _, op := range ops {
		op(t)
	}

	return t
}

// ClusterTaskSpec sets the specified spec of the cluster task.
// Any number of TaskSpec modifier can be passed to create it.
func ClusterTaskSpec(ops ...TaskSpecOp) ClusterTaskOp {
	return func(t *v1alpha1.ClusterTask) {
		spec := &t.Spec
		for _, op := range ops {
			op(spec)
		}
		t.Spec = *spec
	}
}

// TaskSpec sets the specified spec of the task.
// Any number of TaskSpec modifier can be passed to create/modify it.
func TaskSpec(ops ...TaskSpecOp) TaskOp {
	return func(t *v1alpha1.Task) {
		spec := &t.Spec
		for _, op := range ops {
			op(spec)
		}
		t.Spec = *spec
	}
}

// Step adds a step with the specified name and image to the TaskSpec.
// Any number of Container modifier can be passed to transform it.
func Step(name, image string, ops ...ContainerOp) TaskSpecOp {
	return func(spec *v1alpha1.TaskSpec) {
		if spec.Steps == nil {
			spec.Steps = []corev1.Container{}
		}
		step := &corev1.Container{
			Name:  name,
			Image: image,
		}
		for _, op := range ops {
			op(step)
		}
		spec.Steps = append(spec.Steps, *step)
	}
}

// TaskStepTemplate adds a base container for all steps in the task.
func TaskStepTemplate(ops ...ContainerOp) TaskSpecOp {
	return func(spec *v1alpha1.TaskSpec) {
		base := &corev1.Container{}

		for _, op := range ops {
			op(base)
		}
		spec.StepTemplate = base
	}
}

// TaskContainerTemplate adds the deprecated (#977) base container for
// all steps in the task. ContainerTemplate is now StepTemplate.
func TaskContainerTemplate(ops ...ContainerOp) TaskSpecOp {
	return func(spec *v1alpha1.TaskSpec) {
		base := &corev1.Container{}

		for _, op := range ops {
			op(base)
		}
		spec.ContainerTemplate = base
	}
}

// TaskVolume adds a volume with specified name to the TaskSpec.
// Any number of Volume modifier can be passed to transform it.
func TaskVolume(name string, ops ...VolumeOp) TaskSpecOp {
	return func(spec *v1alpha1.TaskSpec) {
		v := &corev1.Volume{Name: name}
		for _, op := range ops {
			op(v)
		}
		spec.Volumes = append(spec.Volumes, *v)
	}
}

// VolumeSource sets the VolumeSource to the Volume.
func VolumeSource(s corev1.VolumeSource) VolumeOp {
	return func(v *corev1.Volume) {
		v.VolumeSource = s
	}
}

// TaskInputs sets inputs to the TaskSpec.
// Any number of Inputs modifier can be passed to transform it.
func TaskInputs(ops ...InputsOp) TaskSpecOp {
	return func(spec *v1alpha1.TaskSpec) {
		if spec.Inputs == nil {
			spec.Inputs = &v1alpha1.Inputs{}
		}
		for _, op := range ops {
			op(spec.Inputs)
		}
	}
}

// TaskOutputs sets inputs to the TaskSpec.
// Any number of Outputs modifier can be passed to transform it.
func TaskOutputs(ops ...OutputsOp) TaskSpecOp {
	return func(spec *v1alpha1.TaskSpec) {
		if spec.Outputs == nil {
			spec.Outputs = &v1alpha1.Outputs{}
		}
		for _, op := range ops {
			op(spec.Outputs)
		}
	}
}

// InputsResource adds a resource, with specified name and type, to the Inputs.
// Any number of TaskResource modifier can be passed to transform it.
func InputsResource(name string, resourceType v1alpha1.PipelineResourceType, ops ...TaskResourceOp) InputsOp {
	return func(i *v1alpha1.Inputs) {
		r := &v1alpha1.TaskResource{Name: name, Type: resourceType}
		for _, op := range ops {
			op(r)
		}
		i.Resources = append(i.Resources, *r)
	}
}

func ResourceTargetPath(path string) TaskResourceOp {
	return func(r *v1alpha1.TaskResource) {
		r.TargetPath = path
	}
}

// OutputsResource adds a resource, with specified name and type, to the Outputs.
func OutputsResource(name string, resourceType v1alpha1.PipelineResourceType) OutputsOp {
	return func(o *v1alpha1.Outputs) {
		o.Resources = append(o.Resources, v1alpha1.TaskResource{Name: name, Type: resourceType})
	}
}

// InputsParam adds a param, with specified name, to the Inputs.
// Any number of ParamSpec modifier can be passed to transform it.
func InputsParam(name string, ops ...TaskParamOp) InputsOp {
	return func(i *v1alpha1.Inputs) {
		tp := &v1alpha1.ParamSpec{Name: name}
		for _, op := range ops {
			op(tp)
		}
		i.Params = append(i.Params, *tp)
	}
}

// ParamDescripiton sets the description to the ParamSpec.
func ParamDescription(desc string) TaskParamOp {
	return func(tp *v1alpha1.ParamSpec) {
		tp.Description = desc
	}
}

// ParamDefault sets the default value to the ParamSpec.
func ParamDefault(value string) TaskParamOp {
	return func(tp *v1alpha1.ParamSpec) {
		tp.Default = value
	}
}

// TaskRun creates a TaskRun with default values.
// Any number of TaskRun modifier can be passed to transform it.
func TaskRun(name, namespace string, ops ...TaskRunOp) *v1alpha1.TaskRun {
	tr := &v1alpha1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   namespace,
			Name:        name,
			Annotations: map[string]string{},
		},
	}

	for _, op := range ops {
		op(tr)
	}

	return tr
}

// TaskRunStatus sets the TaskRunStatus to tshe TaskRun
func TaskRunStatus(ops ...TaskRunStatusOp) TaskRunOp {
	return func(tr *v1alpha1.TaskRun) {
		status := &tr.Status
		for _, op := range ops {
			op(status)
		}
		tr.Status = *status
	}
}

// PodName sets the Pod name to the TaskRunStatus.
func PodName(name string) TaskRunStatusOp {
	return func(s *v1alpha1.TaskRunStatus) {
		s.PodName = name
	}
}

// Condition adds a Condition to the TaskRunStatus.
func Condition(condition apis.Condition) TaskRunStatusOp {
	return func(s *v1alpha1.TaskRunStatus) {
		s.Conditions = append(s.Conditions, condition)
	}
}

func Retry(retry v1alpha1.TaskRunStatus) TaskRunStatusOp {
	return func(s *v1alpha1.TaskRunStatus) {
		s.RetriesStatus = append(s.RetriesStatus, retry)
	}
}

// StepState adds a StepState to the TaskRunStatus.
func StepState(ops ...StepStateOp) TaskRunStatusOp {
	return func(s *v1alpha1.TaskRunStatus) {
		state := &v1alpha1.StepState{}
		for _, op := range ops {
			op(state)
		}
		s.Steps = append(s.Steps, *state)
	}
}

// TaskRunStartTime sets the start time to the TaskRunStatus.
func TaskRunStartTime(startTime time.Time) TaskRunStatusOp {
	return func(s *v1alpha1.TaskRunStatus) {
		s.StartTime = &metav1.Time{Time: startTime}
	}
}

// TaskRunTimeout sets the timeout duration to the TaskRunSpec.
func TaskRunTimeout(d time.Duration) TaskRunSpecOp {
	return func(spec *v1alpha1.TaskRunSpec) {
		spec.Timeout = &metav1.Duration{Duration: d}
	}
}

// TaskRunNodeSelector sets the NodeSelector to the PipelineSpec.
func TaskRunNodeSelector(values map[string]string) TaskRunSpecOp {
	return func(spec *v1alpha1.TaskRunSpec) {
		spec.NodeSelector = values
	}
}

// TaskRunTolerations sets the Tolerations to the PipelineSpec.
func TaskRunTolerations(values []corev1.Toleration) TaskRunSpecOp {
	return func(spec *v1alpha1.TaskRunSpec) {
		spec.Tolerations = values
	}
}

// TaskRunAffinity sets the Affinity to the PipelineSpec.
func TaskRunAffinity(affinity *corev1.Affinity) TaskRunSpecOp {
	return func(spec *v1alpha1.TaskRunSpec) {
		spec.Affinity = affinity
	}
}

// StateTerminated set Terminated to the StepState.
func StateTerminated(exitcode int) StepStateOp {
	return func(s *v1alpha1.StepState) {
		s.ContainerState = corev1.ContainerState{
			Terminated: &corev1.ContainerStateTerminated{ExitCode: int32(exitcode)},
		}
	}
}

// TaskRunOwnerReference sets the OwnerReference, with specified kind and name, to the TaskRun.
func TaskRunOwnerReference(kind, name string, ops ...OwnerReferenceOp) TaskRunOp {
	return func(tr *v1alpha1.TaskRun) {
		o := &metav1.OwnerReference{
			Kind: kind,
			Name: name,
		}
		for _, op := range ops {
			op(o)
		}
		tr.ObjectMeta.OwnerReferences = append(tr.ObjectMeta.OwnerReferences, *o)
	}
}

func Controller(o *metav1.OwnerReference) {
	o.Controller = &trueB
}

func BlockOwnerDeletion(o *metav1.OwnerReference) {
	o.BlockOwnerDeletion = &trueB
}

func TaskRunLabel(key, value string) TaskRunOp {
	return func(tr *v1alpha1.TaskRun) {
		if tr.ObjectMeta.Labels == nil {
			tr.ObjectMeta.Labels = map[string]string{}
		}
		tr.ObjectMeta.Labels[key] = value
	}
}

func TaskRunAnnotation(key, value string) TaskRunOp {
	return func(tr *v1alpha1.TaskRun) {
		if tr.ObjectMeta.Annotations == nil {
			tr.ObjectMeta.Annotations = map[string]string{}
		}
		tr.ObjectMeta.Annotations[key] = value
	}
}

// TaskRunSpec sets the specified spec of the TaskRun.
// Any number of TaskRunSpec modifier can be passed to transform it.
func TaskRunSpec(ops ...TaskRunSpecOp) TaskRunOp {
	return func(tr *v1alpha1.TaskRun) {
		spec := &tr.Spec
		// Set a default timeout
		spec.Timeout = &metav1.Duration{Duration: config.DefaultTimeoutMinutes * time.Minute}
		for _, op := range ops {
			op(spec)
		}
		tr.Spec = *spec
	}
}

// TaskRunCancelled sets the status to cancel to the TaskRunSpec.
func TaskRunCancelled(spec *v1alpha1.TaskRunSpec) {
	spec.Status = v1alpha1.TaskRunSpecStatusCancelled
}

// TaskRunTaskRef sets the specified Task reference to the TaskRunSpec.
// Any number of TaskRef modifier can be passed to transform it.
func TaskRunTaskRef(name string, ops ...TaskRefOp) TaskRunSpecOp {
	return func(spec *v1alpha1.TaskRunSpec) {
		ref := &v1alpha1.TaskRef{Name: name}
		for _, op := range ops {
			op(ref)
		}
		spec.TaskRef = ref
	}
}

// TaskRunSpecStatus sets the Status in the Spec, used for operations
// such as cancelling executing TaskRuns.
func TaskRunSpecStatus(status v1alpha1.TaskRunSpecStatus) TaskRunSpecOp {
	return func(spec *v1alpha1.TaskRunSpec) {
		spec.Status = status
	}
}

// TaskRefKind set the specified kind to the TaskRef.
func TaskRefKind(kind v1alpha1.TaskKind) TaskRefOp {
	return func(ref *v1alpha1.TaskRef) {
		ref.Kind = kind
	}
}

// TaskRefAPIVersion sets the specified api version to the TaskRef.
func TaskRefAPIVersion(version string) TaskRefOp {
	return func(ref *v1alpha1.TaskRef) {
		ref.APIVersion = version
	}
}

// TaskRunTaskSpec sets the specified TaskRunSpec reference to the TaskRunSpec.
// Any number of TaskRunSpec modifier can be passed to transform it.
func TaskRunTaskSpec(ops ...TaskSpecOp) TaskRunSpecOp {
	return func(spec *v1alpha1.TaskRunSpec) {
		taskSpec := &v1alpha1.TaskSpec{}
		for _, op := range ops {
			op(taskSpec)
		}
		spec.TaskSpec = taskSpec
	}
}

// TaskRunServiceAccount sets the serviceAccount to the TaskRunSpec.
func TaskRunServiceAccount(sa string) TaskRunSpecOp {
	return func(trs *v1alpha1.TaskRunSpec) {
		trs.ServiceAccount = sa
	}
}

// TaskRunInputs sets inputs to the TaskRunSpec.
// Any number of TaskRunInputs modifier can be passed to transform it.
func TaskRunInputs(ops ...TaskRunInputsOp) TaskRunSpecOp {
	return func(spec *v1alpha1.TaskRunSpec) {
		inputs := &spec.Inputs
		for _, op := range ops {
			op(inputs)
		}
		spec.Inputs = *inputs
	}
}

// TaskRunInputsParam add a param, with specified name and value, to the TaskRunInputs.
func TaskRunInputsParam(name, value string) TaskRunInputsOp {
	return func(i *v1alpha1.TaskRunInputs) {
		i.Params = append(i.Params, v1alpha1.Param{
			Name:  name,
			Value: value,
		})
	}
}

// TaskRunInputsResource adds a resource, with specified name, to the TaskRunInputs.
// Any number of TaskResourceBinding modifier can be passed to transform it.
func TaskRunInputsResource(name string, ops ...TaskResourceBindingOp) TaskRunInputsOp {
	return func(i *v1alpha1.TaskRunInputs) {
		binding := &v1alpha1.TaskResourceBinding{
			Name: name,
		}
		for _, op := range ops {
			op(binding)
		}
		i.Resources = append(i.Resources, *binding)
	}
}

// TaskResourceBindingRef set the PipelineResourceRef name to the TaskResourceBinding.
func TaskResourceBindingRef(name string) TaskResourceBindingOp {
	return func(b *v1alpha1.TaskResourceBinding) {
		b.ResourceRef.Name = name
	}
}

// TaskResourceBindingResourceSpec set the PipelineResourceResourceSpec to the TaskResourceBinding.
func TaskResourceBindingResourceSpec(spec *v1alpha1.PipelineResourceSpec) TaskResourceBindingOp {
	return func(b *v1alpha1.TaskResourceBinding) {
		b.ResourceSpec = spec
	}
}

// TaskResourceBindingRefAPIVersion set the PipelineResourceRef APIVersion to the TaskResourceBinding.
func TaskResourceBindingRefAPIVersion(version string) TaskResourceBindingOp {
	return func(b *v1alpha1.TaskResourceBinding) {
		b.ResourceRef.APIVersion = version
	}
}

// TaskResourceBindingPaths add any number of path to the TaskResourceBinding.
func TaskResourceBindingPaths(paths ...string) TaskResourceBindingOp {
	return func(b *v1alpha1.TaskResourceBinding) {
		b.Paths = paths
	}
}

// TaskRunOutputs sets inputs to the TaskRunSpec.
// Any number of TaskRunOutputs modifier can be passed to transform it.
func TaskRunOutputs(ops ...TaskRunOutputsOp) TaskRunSpecOp {
	return func(spec *v1alpha1.TaskRunSpec) {
		outputs := &spec.Outputs
		for _, op := range ops {
			op(outputs)
		}
		spec.Outputs = *outputs
	}
}

// TaskRunOutputsResource adds a TaskResourceBinding, with specified name, to the TaskRunOutputs.
// Any number of TaskResourceBinding modifier can be passed to modifiy it.
func TaskRunOutputsResource(name string, ops ...TaskResourceBindingOp) TaskRunOutputsOp {
	return func(i *v1alpha1.TaskRunOutputs) {
		binding := &v1alpha1.TaskResourceBinding{
			Name: name,
			ResourceRef: v1alpha1.PipelineResourceRef{
				Name: name,
			},
		}
		for _, op := range ops {
			op(binding)
		}
		i.Resources = append(i.Resources, *binding)
	}
}

// ResolvedTaskResources creates a ResolvedTaskResources with default values.
// Any number of ResolvedTaskResources modifier can be passed to transform it.
func ResolvedTaskResources(ops ...ResolvedTaskResourcesOp) *resources.ResolvedTaskResources {
	resources := &resources.ResolvedTaskResources{}
	for _, op := range ops {
		op(resources)
	}
	return resources
}

// ResolvedTaskResourcesTaskSpec sets a TaskSpec to the ResolvedTaskResources.
// Any number of TaskSpec modifier can be passed to transform it.
func ResolvedTaskResourcesTaskSpec(ops ...TaskSpecOp) ResolvedTaskResourcesOp {
	return func(r *resources.ResolvedTaskResources) {
		spec := &v1alpha1.TaskSpec{}
		for _, op := range ops {
			op(spec)
		}
		r.TaskSpec = spec
	}
}

// ResolvedTaskResourcesInputs adds an input PipelineResource, with specified name, to the ResolvedTaskResources.
func ResolvedTaskResourcesInputs(name string, resource *v1alpha1.PipelineResource) ResolvedTaskResourcesOp {
	return func(r *resources.ResolvedTaskResources) {
		if r.Inputs == nil {
			r.Inputs = map[string]*v1alpha1.PipelineResource{}
		}
		r.Inputs[name] = resource
	}
}

// ResolvedTaskResourcesOutputs adds an output PipelineResource, with specified name, to the ResolvedTaskResources.
func ResolvedTaskResourcesOutputs(name string, resource *v1alpha1.PipelineResource) ResolvedTaskResourcesOp {
	return func(r *resources.ResolvedTaskResources) {
		if r.Outputs == nil {
			r.Outputs = map[string]*v1alpha1.PipelineResource{}
		}
		r.Outputs[name] = resource
	}
}
