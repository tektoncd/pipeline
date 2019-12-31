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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
)

// ConditionOp is an operation which modifies a Condition struct.
type ConditionOp func(*v1alpha1.Condition)

// ConditionSpecOp is an operation which modifies a ConditionSpec struct.
type ConditionSpecOp func(spec *v1alpha1.ConditionSpec)

// Condition creates a Condition with default values.
// Any number of Condition modifiers can be passed to transform it.
func Condition(name, namespace string, ops ...ConditionOp) *v1alpha1.Condition {
	condition := &v1alpha1.Condition{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
	}
	for _, op := range ops {
		op(condition)
	}
	return condition
}

func ConditionLabels(labels map[string]string) ConditionOp {
	return func(Condition *v1alpha1.Condition) {
		if Condition.ObjectMeta.Labels == nil {
			Condition.ObjectMeta.Labels = map[string]string{}
		}
		for key, value := range labels {
			Condition.ObjectMeta.Labels[key] = value
		}
	}
}

// ConditionSpec creates a ConditionSpec with default values.
// Any number of ConditionSpec modifiers can be passed to transform it.
func ConditionSpec(ops ...ConditionSpecOp) ConditionOp {
	return func(Condition *v1alpha1.Condition) {
		ConditionSpec := &Condition.Spec
		for _, op := range ops {
			op(ConditionSpec)
		}
		Condition.Spec = *ConditionSpec
	}
}

// ConditionSpecCheck adds a Container, with the specified name and image, to the Condition Spec Check.
// Any number of Container modifiers can be passed to transform it.
func ConditionSpecCheck(name, image string, ops ...ContainerOp) ConditionSpecOp {
	return func(spec *v1alpha1.ConditionSpec) {
		c := &corev1.Container{
			Name:  name,
			Image: image,
		}
		for _, op := range ops {
			op(c)
		}
		spec.Check.Container = *c
	}
}

func ConditionSpecCheckScript(script string) ConditionSpecOp {
	return func(spec *v1alpha1.ConditionSpec) {
		spec.Check.Script = script
	}
}

// ConditionParamSpec adds a param, with specified name, to the Spec.
// Any number of ParamSpec modifiers can be passed to transform it.
func ConditionParamSpec(name string, pt v1alpha1.ParamType, ops ...ParamSpecOp) ConditionSpecOp {
	return func(ps *v1alpha1.ConditionSpec) {
		pp := &v1alpha1.ParamSpec{Name: name, Type: pt}
		for _, op := range ops {
			op(pp)
		}
		ps.Params = append(ps.Params, *pp)
	}
}

// ConditionResource adds a resource with specified name, and type to the ConditionSpec.
func ConditionResource(name string, resourceType v1alpha1.PipelineResourceType) ConditionSpecOp {
	return func(spec *v1alpha1.ConditionSpec) {
		r := v1alpha1.ResourceDeclaration{
			Name: name,
			Type: resourceType,
		}
		spec.Resources = append(spec.Resources, r)
	}
}
