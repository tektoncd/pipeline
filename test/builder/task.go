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

package builder

import (
	"github.com/knative/build-pipeline/pkg/apis/pipeline/v1alpha1"
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

// TaskParamOp is an operation which modify a TaskParam struct.
type TaskParamOp func(*v1alpha1.TaskParam)

// TaskRunOp is an operation which modify a TaskRun struct.
type TaskRunOp func(*v1alpha1.TaskRun)

// TaskRunSpecOp is an operation which modify a TaskRunSpec struct.
type TaskRunSpecOp func(*v1alpha1.TaskRunSpec)

// TaskResourceBindingOp is an operation which modify a TaskResourceBindingOp struct.
type TaskResourceBindingOp func(*v1alpha1.TaskResourceBinding)

// TaskRunStatusOp is an operation which modify a TaskRunStatus struct.
type TaskRunStatusOp func(*v1alpha1.TaskRunStatus)

// TaskRefOp is an operation which modify a TaskRef struct.
type TaskRefOp func(*v1alpha1.TaskRef)

// TaskRunInputsOp is an operation which modify a TaskRunInputs struct.
type TaskRunInputsOp func(*v1alpha1.TaskRunInputs)

// TaskRunOutputsOp is an operation which modify a TaskRunOutputs struct.
type TaskRunOutputsOp func(*v1alpha1.TaskRunOutputs)

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
func InputsResource(name string, resourceType v1alpha1.PipelineResourceType) InputsOp {
	return func(i *v1alpha1.Inputs) {
		i.Resources = append(i.Resources, v1alpha1.TaskResource{Name: name, Type: resourceType})
	}
}

// OutputsResource adds a resource, with specified name and type, to the Outputs.
func OutputsResource(name string, resourceType v1alpha1.PipelineResourceType) OutputsOp {
	return func(o *v1alpha1.Outputs) {
		o.Resources = append(o.Resources, v1alpha1.TaskResource{Name: name, Type: resourceType})
	}
}

// InputsParam adds a param, with specified name, to the Inputs.
// Any number of TaskParam modifier can be passed to transform it.
func InputsParam(name string, ops ...TaskParamOp) InputsOp {
	return func(i *v1alpha1.Inputs) {
		tp := &v1alpha1.TaskParam{Name: name}
		for _, op := range ops {
			op(tp)
		}
		i.Params = append(i.Params, *tp)
	}
}

// ParamDescripiton sets the description to the TaskParam.
func ParamDescription(desc string) TaskParamOp {
	return func(tp *v1alpha1.TaskParam) {
		tp.Description = desc
	}
}

// ParamDefault sets the default value to the TaskParam.
func ParamDefault(value string) TaskParamOp {
	return func(tp *v1alpha1.TaskParam) {
		tp.Default = value
	}
}

// TaskRun creates a TaskRun with default values.
// Any number of TaskRun modifier can be passed to transform it.
func TaskRun(name, namespace string, ops ...TaskRunOp) *v1alpha1.TaskRun {
	tr := &v1alpha1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
		Spec: v1alpha1.TaskRunSpec{
			Trigger: v1alpha1.TaskTrigger{
				TriggerRef: v1alpha1.TaskTriggerRef{
					Type: v1alpha1.TaskTriggerTypeManual,
				},
			},
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

// PodName sets the Pod name to the TaskRunStatus
func PodName(name string) TaskRunStatusOp {
	return func(s *v1alpha1.TaskRunStatus) {
		s.PodName = name
	}
}

// TaskRunOwnerReference sets the OwnerReference, with specified kind and name, to the TaskRun
func TaskRunOwnerReference(kind, name string) TaskRunOp {
	return func(tr *v1alpha1.TaskRun) {
		tr.ObjectMeta.OwnerReferences = append(tr.ObjectMeta.OwnerReferences, metav1.OwnerReference{
			Kind: kind,
			Name: name,
		})
	}
}

// TaskRunSpec sets the specified spec of the TaskRun.
// Any number of TaskRunSpec modifier can be passed to transform it.
func TaskRunSpec(ops ...TaskRunSpecOp) TaskRunOp {
	return func(tr *v1alpha1.TaskRun) {
		spec := &tr.Spec
		for _, op := range ops {
			op(spec)
		}
		tr.Spec = *spec
	}
}

// TaskRunTaskRef sets the specified Task reference to the TaskRunSpec.
// Any number of TaskRef modifier can be passed to transform it.
func TaskRunTaskRef(name string, ops ...TaskRefOp) TaskRunSpecOp {
	return func(spec *v1alpha1.TaskRunSpec) {
		ref := &v1alpha1.TaskRef{Name: name, APIVersion: defaultAPIVersion}
		for _, op := range ops {
			op(ref)
		}
		spec.TaskRef = ref
	}
}

// TaskRefKind set the specified kind to the TaskRef.
func TaskRefKind(kind v1alpha1.TaskKind) TaskRefOp {
	return func(ref *v1alpha1.TaskRef) {
		ref.Kind = kind
	}
}

// TaskRefAPIVersion sets the specified api version to the TaskRef.
func TaskRefAPIVersion(version string) TaskRunSpecOp {
	return func(spec *v1alpha1.TaskRunSpec) {
		spec.TaskRef.APIVersion = version
	}
}

// TaskRunTaskRef sets the specified TaskRunSpec reference to the TaskRunSpec.
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

// TaskTrigger set the TaskTrigger, with specified name and type, to the TaskRunSpec.
func TaskTrigger(name string, triggerType v1alpha1.TaskTriggerType) TaskRunSpecOp {
	return func(trs *v1alpha1.TaskRunSpec) {
		trs.Trigger = v1alpha1.TaskTrigger{
			TriggerRef: v1alpha1.TaskTriggerRef{
				Name: name,
				Type: triggerType,
			},
		}
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
			ResourceRef: v1alpha1.PipelineResourceRef{
				Name:       name,
				APIVersion: defaultAPIVersion,
			},
		}
		for _, op := range ops {
			op(binding)
		}
		i.Resources = append(i.Resources, *binding)
	}
}

// ResourceBindingRef set the PipelineResourceRef, with specified name and apiversion, to the TaskResourceBinding.
func ResourceBindingRef(name, apiversion string) TaskResourceBindingOp {
	return func(b *v1alpha1.TaskResourceBinding) {
		b.ResourceRef = v1alpha1.PipelineResourceRef{
			Name:       name,
			APIVersion: apiversion,
		}
	}
}

// ResourceBindingPaths add any number of path to the TaskResourceBinding.
func ResourceBindingPaths(paths ...string) TaskResourceBindingOp {
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
				Name:       name,
				APIVersion: "a1",
			},
		}
		for _, op := range ops {
			op(binding)
		}
		i.Resources = append(i.Resources, *binding)
	}
}
