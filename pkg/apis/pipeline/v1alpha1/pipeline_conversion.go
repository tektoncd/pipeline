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

// nolint: revive
package v1alpha1

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
)

const FinallyFieldName = "finally"
const finallyAnnotationKey = "tekton.dev/v1beta1Finally"

var _ apis.Convertible = (*Pipeline)(nil)

// ConvertTo implements api.Convertible
func (source *Pipeline) ConvertTo(ctx context.Context, obj apis.Convertible) error {
	switch sink := obj.(type) {
	case *v1beta1.Pipeline:
		sink.ObjectMeta = source.ObjectMeta
		if err := source.Spec.ConvertTo(ctx, &sink.Spec); err != nil {
			return err
		}
		if err := deserializeFinally(&sink.ObjectMeta, &sink.Spec); err != nil {
			return err
		}
		if err := v1beta1.ValidatePipelineTasks(ctx, sink.Spec.Tasks, sink.Spec.Finally); err != nil {
			return fmt.Errorf("error converting finally annotation into beta field: %w", err)
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
		if err := serializeFinally(&sink.ObjectMeta, source.Spec.Finally); err != nil {
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

// serializeFinally serializes a list of Finally Tasks to the annotations
// of an object's metadata section. This can then be used to re-instantiate
// the Finally Tasks when converting back up to v1beta1 and beyond.
func serializeFinally(meta *metav1.ObjectMeta, finally []v1beta1.PipelineTask) error {
	if len(finally) != 0 {
		b, err := json.Marshal(finally)
		if err != nil {
			return err
		}
		if meta.Annotations == nil {
			meta.Annotations = make(map[string]string)
		}
		meta.Annotations[finallyAnnotationKey] = string(b)
	}
	return nil
}

// deserializeFinally populates a PipelineSpec's Finally list
// from an annotation found on resources that have been previously
// converted down from v1beta1 to v1alpha1.
func deserializeFinally(meta *metav1.ObjectMeta, spec *v1beta1.PipelineSpec) error {
	if meta.Annotations != nil {
		if str, ok := meta.Annotations[finallyAnnotationKey]; ok {
			finally := []v1beta1.PipelineTask{}
			if err := json.Unmarshal([]byte(str), &finally); err != nil {
				return fmt.Errorf("error converting finally annotation into beta field: %w", err)
			}
			delete(meta.Annotations, finallyAnnotationKey)
			if len(meta.Annotations) == 0 {
				meta.Annotations = nil
			}
			spec.Finally = finally
		}
	}
	return nil
}
