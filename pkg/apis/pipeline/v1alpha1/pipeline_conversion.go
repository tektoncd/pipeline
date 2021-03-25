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

// nolint: golint
package v1alpha1

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
)

// This annotation was used to serialize and deserialize Finally tasks in the
// 0.22 Pipelines release. Keep it around to continue supporting any documents
// converted during that time.
const finallyAnnotationKey = "tekton.dev/v1beta1Finally"

const V1Beta1PipelineSpecSerializedAnnotationKey = "tekton.dev/v1beta1PipelineSpec"

var _ apis.Convertible = (*Pipeline)(nil)

// ConvertTo implements api.Convertible
func (source *Pipeline) ConvertTo(ctx context.Context, obj apis.Convertible) error {
	switch sink := obj.(type) {
	case *v1beta1.Pipeline:
		sink.ObjectMeta = source.ObjectMeta
		if hasV1Beta1PipelineSpecAnnotation(sink.ObjectMeta.Annotations) {
			if err := deserializeIntoV1Beta1PipelineSpec(&sink.ObjectMeta, &sink.Spec); err != nil {
				return fmt.Errorf("error deserializing v1beta1 annotation into spec: %w", err)
			}
		} else if err := source.Spec.ConvertTo(ctx, &sink.Spec); err != nil {
			return err
		}
		if err := deserializeFinally(&sink.ObjectMeta, &sink.Spec); err != nil {
			return fmt.Errorf("error converting finally annotation into beta field: %w", err)
		}

		// The defaults and validation criteria might have changed since this
		// resource was serialized. This would likely be the result of a bug
		// in Pipelines so this is intended to help prevent or surface those
		// potential bugs.
		sink.SetDefaults(ctx)
		if err := sink.Validate(ctx); err != nil {
			return fmt.Errorf("invalid v1beta1 pipeline after conversion: %w", err)
		}
	default:
		return fmt.Errorf("unknown version, got: %T", sink)
	}
	return nil
}

func (source *PipelineSpec) ConvertTo(ctx context.Context, sink *v1beta1.PipelineSpec) error {
	sink.Resources = source.Resources
	sink.Params = source.Params
	sink.Workspaces = source.Workspaces
	sink.Description = source.Description
	sink.Results = source.Results
	if len(source.Tasks) > 0 {
		sink.Tasks = make([]v1beta1.PipelineTask, len(source.Tasks))
		for i := range source.Tasks {
			if err := source.Tasks[i].ConvertTo(ctx, &sink.Tasks[i]); err != nil {
				return err
			}
		}
	}
	sink.Finally = nil
	return nil
}

func (source *PipelineTask) ConvertTo(ctx context.Context, sink *v1beta1.PipelineTask) error {
	sink.Name = source.Name
	sink.TaskRef = source.TaskRef
	if source.TaskSpec != nil {
		sink.TaskSpec = &v1beta1.EmbeddedTask{TaskSpec: v1beta1.TaskSpec{}}
		if err := source.TaskSpec.ConvertTo(ctx, &sink.TaskSpec.TaskSpec); err != nil {
			return err
		}
	}
	sink.Conditions = source.Conditions
	sink.Retries = source.Retries
	sink.RunAfter = source.RunAfter
	sink.Resources = source.Resources
	sink.Params = source.Params
	sink.Workspaces = source.Workspaces
	sink.Timeout = source.Timeout
	return nil
}

// ConvertFrom implements api.Convertible
func (sink *Pipeline) ConvertFrom(ctx context.Context, obj apis.Convertible) error {
	switch source := obj.(type) {
	case *v1beta1.Pipeline:
		sink.ObjectMeta = source.ObjectMeta
		if err := serializeV1Beta1PipelineSpec(&sink.ObjectMeta, &source.Spec); err != nil {
			return err
		}
		return sink.Spec.ConvertFrom(ctx, source.Spec)
	default:
		return fmt.Errorf("unknown version, got: %T", sink)
	}
}

func (sink *PipelineSpec) ConvertFrom(ctx context.Context, source v1beta1.PipelineSpec) error {
	sink.Resources = source.Resources
	sink.Params = source.Params
	sink.Workspaces = source.Workspaces
	sink.Description = source.Description
	sink.Results = source.Results
	if len(source.Tasks) > 0 {
		sink.Tasks = make([]PipelineTask, len(source.Tasks))
		for i := range source.Tasks {
			if err := sink.Tasks[i].ConvertFrom(ctx, source.Tasks[i]); err != nil {
				return err
			}
		}
	}
	return nil
}

func (sink *PipelineTask) ConvertFrom(ctx context.Context, source v1beta1.PipelineTask) error {
	sink.Name = source.Name
	sink.TaskRef = source.TaskRef
	if source.TaskSpec != nil {
		sink.TaskSpec = &TaskSpec{}
		if err := sink.TaskSpec.ConvertFrom(ctx, &source.TaskSpec.TaskSpec); err != nil {
			return err
		}
	}
	sink.Conditions = source.Conditions
	sink.Retries = source.Retries
	sink.RunAfter = source.RunAfter
	sink.Resources = source.Resources
	sink.Params = source.Params
	sink.Workspaces = source.Workspaces
	sink.Timeout = source.Timeout
	return nil
}

// deserializeFinally populates a PipelineSpec's Finally list
// from an annotation found on resources that were converted
// from v1beta1 to v1alpha1. This annotation was only used
// for a short time (the 0.22 release) but is kept here to
// continue supporting any documents converted to v1alpha1 during
// that period.
func deserializeFinally(meta *metav1.ObjectMeta, spec *v1beta1.PipelineSpec) error {
	if meta.Annotations != nil {
		if str, ok := meta.Annotations[finallyAnnotationKey]; ok {
			finally := []v1beta1.PipelineTask{}
			if err := json.Unmarshal([]byte(str), &finally); err != nil {
				return fmt.Errorf("error converting finally annotation into beta field: %w", err)
			}
			if len(meta.Annotations) == 1 {
				meta.Annotations = nil
			} else {
				delete(meta.Annotations, finallyAnnotationKey)
			}
			spec.Finally = finally
		}
	}
	return nil
}

// hasV1Beta1PipelineSpecAnnotation returns true if the provided annotations
// map contains the key expected for serialized v1beta1 PipelineSpecs.
func hasV1Beta1PipelineSpecAnnotation(annotations map[string]string) bool {
	if annotations == nil {
		return false
	}
	_, has := annotations[V1Beta1PipelineSpecSerializedAnnotationKey]
	return has
}

// serializeV1Beta1PipelineSpec serializes a v1beta1.PipelineSpec to an
// annotation in the provided meta. This will be used when applying the
// v1alpha1 resource back to the cluster in order to rehydrate a v1beta1 doc
// exactly as it appeared before being converted.
func serializeV1Beta1PipelineSpec(meta *metav1.ObjectMeta, spec *v1beta1.PipelineSpec) error {
	b, err := json.Marshal(spec)
	if err != nil {
		return fmt.Errorf("error serializing v1beta1 document into annotation: %w", err)
	}
	if meta.Annotations == nil {
		meta.Annotations = make(map[string]string)
	}
	meta.Annotations[V1Beta1PipelineSpecSerializedAnnotationKey] = string(b)
	return nil
}

// deserializeIntoV1Beta1PipelineSpec populates a PipelineSpec
// from a JSON annotation found on resources that have been previously
// converted from v1beta1 to v1alpha1.
func deserializeIntoV1Beta1PipelineSpec(meta *metav1.ObjectMeta, spec *v1beta1.PipelineSpec) error {
	if meta.Annotations != nil {
		if str, ok := meta.Annotations[V1Beta1PipelineSpecSerializedAnnotationKey]; ok {
			if err := json.Unmarshal([]byte(str), spec); err != nil {
				return fmt.Errorf("error deserializing v1beta1 document from annotation: %w", err)
			}
			delete(meta.Annotations, V1Beta1PipelineSpecSerializedAnnotationKey)
			if len(meta.Annotations) == 0 {
				meta.Annotations = nil
			}
		}
	}
	return nil
}
