/*
Copyright 2025 The Tekton Authors

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
	"encoding/json"
	"fmt"

	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/apis/version"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ComputeResourcesAnnotationKey is the annotation key used to preserve v1
// computeResources values that hold unresolved variable references (e.g.
// $(params.MEM)) across a conversion to v1beta1.
//
// v1beta1 stores compute resources as corev1.ResourceRequirements, whose values are
// parsed resource.Quantity and therefore cannot represent a variable reference. Without
// this annotation a v1 Task read and written back through a v1beta1 client would lose
// the reference, leaving the Task with no resource requirements at all.
//
// Like TaskDeprecationsAnnotationKey, the annotation is keyed by Task name so a
// Pipeline or PipelineRun embedding several Task specs round-trips correctly.
const ComputeResourcesAnnotationKey = "tekton.dev/v1ComputeResources"

// rawComputeResources
// +k8s:openapi-gen=false
type rawComputeResources struct {
	Steps        map[int]v1.ComputeResourceRequirements `json:"steps,omitempty"`
	StepTemplate *v1.ComputeResourceRequirements        `json:"stepTemplate,omitempty"`
	Sidecars     map[int]v1.ComputeResourceRequirements `json:"sidecars,omitempty"`
}

func (r rawComputeResources) isEmpty() bool {
	return len(r.Steps) == 0 && r.StepTemplate == nil && len(r.Sidecars) == 0
}

// rawComputeResourcesByTask
// +k8s:openapi-gen=false
type rawComputeResourcesByTask map[string]rawComputeResources

// serializeComputeResources stores the compute resources of source that hold unresolved
// variable references in the annotations of the v1beta1 object being converted to.
func serializeComputeResources(meta *metav1.ObjectMeta, source *v1.TaskSpec, taskName string) error {
	if meta == nil || source == nil {
		return nil
	}
	raw := rawComputeResources{}
	for i, s := range source.Steps {
		if s.ComputeResources.HasUnresolvedReferences() {
			if raw.Steps == nil {
				raw.Steps = map[int]v1.ComputeResourceRequirements{}
			}
			raw.Steps[i] = s.ComputeResources
		}
	}
	if source.StepTemplate != nil && source.StepTemplate.ComputeResources.HasUnresolvedReferences() {
		cr := source.StepTemplate.ComputeResources
		raw.StepTemplate = &cr
	}
	for i, s := range source.Sidecars {
		if s.ComputeResources.HasUnresolvedReferences() {
			if raw.Sidecars == nil {
				raw.Sidecars = map[int]v1.ComputeResourceRequirements{}
			}
			raw.Sidecars[i] = s.ComputeResources
		}
	}
	if raw.isEmpty() {
		return nil
	}

	existing := rawComputeResourcesByTask{}
	if str, ok := meta.Annotations[ComputeResourcesAnnotationKey]; ok {
		if err := json.Unmarshal([]byte(str), &existing); err != nil {
			return fmt.Errorf("error serializing key %s from metadata: %w", ComputeResourcesAnnotationKey, err)
		}
	}
	existing[taskName] = raw
	return version.SerializeToMetadata(meta, existing, ComputeResourcesAnnotationKey)
}

// deserializeComputeResources restores the compute resources previously stored by
// serializeComputeResources onto sink, and drops the annotation once consumed.
func deserializeComputeResources(meta *metav1.ObjectMeta, sink *v1.TaskSpec, taskName string) error {
	if meta == nil || meta.Annotations == nil || sink == nil {
		return nil
	}
	str, ok := meta.Annotations[ComputeResourcesAnnotationKey]
	if !ok {
		return nil
	}
	existing := rawComputeResourcesByTask{}
	if err := json.Unmarshal([]byte(str), &existing); err != nil {
		return fmt.Errorf("error deserializing key %s from metadata: %w", ComputeResourcesAnnotationKey, err)
	}
	raw, ok := existing[taskName]
	if !ok {
		return nil
	}

	for i, cr := range raw.Steps {
		if i >= len(sink.Steps) {
			return fmt.Errorf("deserialized step index %d is out of range of the target steps", i)
		}
		sink.Steps[i].ComputeResources = cr
	}
	if raw.StepTemplate != nil {
		if sink.StepTemplate == nil {
			sink.StepTemplate = &v1.StepTemplate{}
		}
		sink.StepTemplate.ComputeResources = *raw.StepTemplate
	}
	for i, cr := range raw.Sidecars {
		if i >= len(sink.Sidecars) {
			return fmt.Errorf("deserialized sidecar index %d is out of range of the target sidecars", i)
		}
		sink.Sidecars[i].ComputeResources = cr
	}

	delete(existing, taskName)
	if len(existing) == 0 {
		delete(meta.Annotations, ComputeResourcesAnnotationKey)
		if len(meta.Annotations) == 0 {
			meta.Annotations = nil
		}
		return nil
	}
	updated, err := json.Marshal(existing)
	if err != nil {
		return err
	}
	meta.Annotations[ComputeResourcesAnnotationKey] = string(updated)
	return nil
}
