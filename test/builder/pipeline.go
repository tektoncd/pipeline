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
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PipelineOp is an operation which modify a Pipeline struct.
type PipelineOp func(*v1alpha1.Pipeline)

// PipelineSpecOp is an operation which modify a PipelineSpec struct.
type PipelineSpecOp func(*v1alpha1.PipelineSpec)

// PipelineTaskOp is an operation which modify a PipelineTask struct.
type PipelineTaskOp func(*v1alpha1.PipelineTask)

// PipelineRunOp is an operation which modify a PipelineRun struct.
type PipelineRunOp func(*v1alpha1.PipelineRun)

// PipelineRunSpecOp is an operation which modify a PipelineRunSpec struct.
type PipelineRunSpecOp func(*v1alpha1.PipelineRunSpec)

// PipelineResourceOp is an operation which modify a PipelineResource struct.
type PipelineResourceOp func(*v1alpha1.PipelineResource)

// PipelineResourceSpecOp is an operation which modify a PipelineResourceSpec struct.
type PipelineResourceSpecOp func(*v1alpha1.PipelineResourceSpec)

// PipelineTaskResourceOp is an operation which modify a PipelineTaskResource struct.
type PipelineTaskResourceOp func(*v1alpha1.PipelineTaskResource)

// ResourceDependencyOp is an operation which modify a ResourceDependency
type ResourceDependencyOp func(*v1alpha1.ResourceDependency)

// PipelineRunStatusOp is an operation which modify a PipelineRunStatus
type PipelineRunStatusOp func(*v1alpha1.PipelineRunStatus)

// Pipeline creates a Pipeline with default values.
// Any number of Pipeline modifier can be passed to transform it.
func Pipeline(name, namespace string, ops ...PipelineOp) *v1alpha1.Pipeline {
	p := &v1alpha1.Pipeline{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
	}

	for _, op := range ops {
		op(p)
	}

	return p
}

// PipelineSpec sets the PipelineSpec to the Pipeline.
// Any number of PipelineSpec modifier can be passed to transform it.
func PipelineSpec(ops ...PipelineSpecOp) PipelineOp {
	return func(p *v1alpha1.Pipeline) {
		ps := &p.Spec

		for _, op := range ops {
			op(ps)
		}

		p.Spec = *ps
	}
}

// PipelineTask adds a PipelineTask, with specified name and task name, to the PipelineSpec.
// Any number of PipelineTask modifier can be passed to transform it.
func PipelineTask(name, taskName string, ops ...PipelineTaskOp) PipelineSpecOp {
	return func(ps *v1alpha1.PipelineSpec) {
		pTask := &v1alpha1.PipelineTask{
			Name: name,
			TaskRef: v1alpha1.TaskRef{
				Name: taskName,
			},
		}
		for _, op := range ops {
			op(pTask)
		}
		ps.Tasks = append(ps.Tasks, *pTask)
	}
}

// PipelineTaskRefKind sets the TaskKind to the PipelineTaskRef.
func PipelineTaskRefKind(kind v1alpha1.TaskKind) PipelineTaskOp {
	return func(pt *v1alpha1.PipelineTask) {
		pt.TaskRef.Kind = kind
	}
}

// PipelineTaskParam adds a Param, with specified name and value, to the PipelineTask.
func PipelineTaskParam(name, value string) PipelineTaskOp {
	return func(pt *v1alpha1.PipelineTask) {
		pt.Params = append(pt.Params, v1alpha1.Param{
			Name:  name,
			Value: value,
		})
	}
}

// PipelineTaskResourceDependency adds a ResourceDependency, with specified name, to the PipelineTask.
// Any number of ResourceDependency modifier can be passed to transform it.
func PipelineTaskResourceDependency(name string, ops ...ResourceDependencyOp) PipelineTaskOp {
	return func(pt *v1alpha1.PipelineTask) {
		d := &v1alpha1.ResourceDependency{Name: name}
		for _, op := range ops {
			op(d)
		}
		pt.ResourceDependencies = append(pt.ResourceDependencies, *d)
	}
}

// Providedy adds Providedy tasks to the ResourceDependency.
func ProvidedBy(tasks ...string) ResourceDependencyOp {
	return func(d *v1alpha1.ResourceDependency) {
		d.ProvidedBy = tasks
	}
}

// PipelineRun creates a PipelineRun with default values.
// Any number of PipelineRun modifier can be passed to transform it.
func PipelineRun(name, namespace string, ops ...PipelineRunOp) *v1alpha1.PipelineRun {
	pr := &v1alpha1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
		Spec: v1alpha1.PipelineRunSpec{
			Trigger: v1alpha1.PipelineTrigger{
				Type: v1alpha1.PipelineTriggerTypeManual,
			},
		},
	}

	for _, op := range ops {
		op(pr)
	}

	return pr
}

// PipelineRunSpec sets the PipelineRunSpec, with specified name, to the PipelineRun.
// Any number of PipelineRunSpec modifier can be passed to transform it.
func PipelineRunSpec(name string, ops ...PipelineRunSpecOp) PipelineRunOp {
	return func(pr *v1alpha1.PipelineRun) {
		prs := &pr.Spec

		prs.PipelineRef.Name = name

		for _, op := range ops {
			op(prs)
		}

		pr.Spec = *prs
	}
}

// PipelineRunServiceAccount sets the service account to the PipelineRunSpec.
func PipelineRunServiceAccount(sa string) PipelineRunSpecOp {
	return func(prs *v1alpha1.PipelineRunSpec) {
		prs.ServiceAccount = sa
	}
}

// PipelineRunTaskResource adds a PipelineTaskResource, with specified name, to the PipelineRunSpec.
// Any number of PipelineTaskResource modifier can be passed to transform it.
func PipelineRunTaskResource(name string, ops ...PipelineTaskResourceOp) PipelineRunSpecOp {
	return func(prs *v1alpha1.PipelineRunSpec) {
		r := &v1alpha1.PipelineTaskResource{Name: name}
		for _, op := range ops {
			op(r)
		}
		prs.PipelineTaskResources = append(prs.PipelineTaskResources, *r)
	}
}

// PipelineTaskResourceInputs adds an Inputs, with specified name, to the PipelineTaskResource.
// Any number of TaskResourceBinding modifier can be passed to transform it.
func PipelineTaskResourceInputs(name string, ops ...TaskResourceBindingOp) PipelineTaskResourceOp {
	return func(r *v1alpha1.PipelineTaskResource) {
		b := &v1alpha1.TaskResourceBinding{
			Name: name,
		}
		for _, op := range ops {
			op(b)
		}
		r.Inputs = append(r.Inputs, *b)
	}
}

// PipelineTaskResourceOutputs adds an Outputs, with specified name, to the PipelineTaskResource.
// Any number of TaskResourceBinding modifier can be passed to transform it.
func PipelineTaskResourceOutputs(name string, ops ...TaskResourceBindingOp) PipelineTaskResourceOp {
	return func(r *v1alpha1.PipelineTaskResource) {
		b := &v1alpha1.TaskResourceBinding{
			Name: name,
		}
		for _, op := range ops {
			op(b)
		}
		r.Outputs = append(r.Outputs, *b)
	}
}

// PipelineRunStatus sets the PipelineRunStatus to the PipelineRun.
// Any number of PipelineRunStatus modifier can be passed to transform it.
func PipelineRunStatus(ops ...PipelineRunStatusOp) PipelineRunOp {
	return func(pr *v1alpha1.PipelineRun) {
		s := &v1alpha1.PipelineRunStatus{}
		for _, op := range ops {
			op(s)
		}
		pr.Status = *s
	}
}

// PipelineRunStatusCondition adds a Condition to the TaskRunStatus.
func PipelineRunStatusCondition(condition duckv1alpha1.Condition) PipelineRunStatusOp {
	return func(s *v1alpha1.PipelineRunStatus) {
		s.Conditions = append(s.Conditions, condition)
	}
}

// PipelineResource creates a PipelineResource with default values.
// Any number of PipelineResource modifier can be passed to transform it.
func PipelineResource(name, namespace string, ops ...PipelineResourceOp) *v1alpha1.PipelineResource {
	resource := &v1alpha1.PipelineResource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
	for _, op := range ops {
		op(resource)
	}
	return resource
}

// PipelineResourceSpec set the PipelineResourceSpec, with specified type, to the PipelineResource.
// Any number of PipelineResourceSpec modifier can be passed to transform it.
func PipelineResourceSpec(resourceType v1alpha1.PipelineResourceType, ops ...PipelineResourceSpecOp) PipelineResourceOp {
	return func(r *v1alpha1.PipelineResource) {
		spec := &r.Spec
		spec.Type = resourceType
		for _, op := range ops {
			op(spec)
		}
		r.Spec = *spec
	}
}

// PipelineResourceSpecParam adds a Param, with specified name and value, to the PipelineResourceSpec.
func PipelineResourceSpecParam(name, value string) PipelineResourceSpecOp {
	return func(spec *v1alpha1.PipelineResourceSpec) {
		spec.Params = append(spec.Params, v1alpha1.Param{
			Name:  name,
			Value: value,
		})
	}
}
