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

package builder

import (
	resource "github.com/tektoncd/pipeline/pkg/apis/resource/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PipelineResourceOp is an operation which modify a PipelineResource struct.
type PipelineResourceOp func(*resource.PipelineResource)

// PipelineResourceSpecOp is an operation which modify a PipelineResourceSpec struct.
type PipelineResourceSpecOp func(*resource.PipelineResourceSpec)

// PipelineResource creates a PipelineResource with default values.
// Any number of PipelineResource modifier can be passed to transform it.
func PipelineResource(name string, ops ...PipelineResourceOp) *resource.PipelineResource {
	resource := &resource.PipelineResource{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
	for _, op := range ops {
		op(resource)
	}
	return resource
}

// PipelineResourceNamespace sets the namespace on a PipelineResource.
func PipelineResourceNamespace(namespace string) PipelineResourceOp {
	return func(t *resource.PipelineResource) {
		t.ObjectMeta.Namespace = namespace
	}
}

// PipelineResourceSpec set the PipelineResourceSpec, with specified type, to the PipelineResource.
// Any number of PipelineResourceSpec modifier can be passed to transform it.
func PipelineResourceSpec(resourceType resource.PipelineResourceType, ops ...PipelineResourceSpecOp) PipelineResourceOp {
	return func(r *resource.PipelineResource) {
		spec := &r.Spec
		spec.Type = resourceType
		for _, op := range ops {
			op(spec)
		}
		r.Spec = *spec
	}
}

// PipelineResourceDescription sets the description of the pipeline resource
func PipelineResourceDescription(desc string) PipelineResourceSpecOp {
	return func(spec *resource.PipelineResourceSpec) {
		spec.Description = desc
	}
}

// PipelineResourceSpecParam adds a ResourceParam, with specified name and value, to the PipelineResourceSpec.
func PipelineResourceSpecParam(name, value string) PipelineResourceSpecOp {
	return func(spec *resource.PipelineResourceSpec) {
		spec.Params = append(spec.Params, resource.ResourceParam{
			Name:  name,
			Value: value,
		})
	}
}

// PipelineResourceSpecSecretParam adds a SecretParam, with specified fieldname, secretKey and secretName, to the PipelineResourceSpec.
func PipelineResourceSpecSecretParam(fieldname, secretName, secretKey string) PipelineResourceSpecOp {
	return func(spec *resource.PipelineResourceSpec) {
		spec.SecretParams = append(spec.SecretParams, resource.SecretParam{
			FieldName:  fieldname,
			SecretKey:  secretKey,
			SecretName: secretName,
		})
	}
}
