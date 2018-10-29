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

package errors

import (
	"github.com/knative/build-pipeline/pkg/apis/pipeline/v1alpha1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// NewDuplicatePipelineTask creates a new invalid pipeline error for duplicate pipeline tasks.
func NewDuplicatePipelineTask(p *v1alpha1.Pipeline, value string) *apierrors.StatusError {
	errs := field.ErrorList{{
		Type:     field.ErrorTypeDuplicate,
		Field:    "spec.tasks.name",
		BadValue: value,
	}}
	return apierrors.NewInvalid(p.GroupVersionKind().GroupKind(), p.Name, errs)
}

// NewPipelineTaskNotFound creates a new invalid pipeline error.
func NewPipelineTaskNotFound(p *v1alpha1.Pipeline, value string) *apierrors.StatusError {
	errs := field.ErrorList{{
		Type:     field.ErrorTypeNotFound,
		Field:    "spec.tasks.name",
		BadValue: value,
	}}
	return apierrors.NewInvalid(p.GroupVersionKind().GroupKind(), p.Name, errs)
}

// NewInvalidPipeline creates a new invalid pipeline error.
func NewInvalidPipeline(p *v1alpha1.Pipeline, d string) *apierrors.StatusError {
	errs := field.ErrorList{{
		Type:   field.ErrorTypeInternal,
		Detail: d,
	}}
	return apierrors.NewInvalid(p.GroupVersionKind().GroupKind(), p.Name, errs)
}
