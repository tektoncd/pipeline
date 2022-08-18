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

package v1beta1

import (
	"encoding/json"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
)

// mergeData is used to store the intermediate data needed to merge an object
// with a template. It's provided to avoid repeatedly re-serializing the template.
// +k8s:openapi-gen=false
type mergeData struct {
	emptyJSON    []byte
	templateJSON []byte
	patchSchema  strategicpatch.PatchMetaFromStruct
}

// MergeStepsWithStepTemplate takes a possibly nil container template and a
// list of steps, merging each of the steps with the container template, if
// it's not nil, and returning the resulting list.
func MergeStepsWithStepTemplate(template *StepTemplate, steps []Step) ([]Step, error) {
	if template == nil {
		return steps, nil
	}

	md, err := getMergeData(template.ToK8sContainer(), &corev1.Container{})
	if err != nil {
		return nil, err
	}

	for i, s := range steps {
		merged := corev1.Container{}
		err := mergeObjWithTemplateBytes(md, s.ToK8sContainer(), &merged)
		if err != nil {
			return nil, err
		}

		// If the container's args is nil, reset it to empty instead
		if merged.Args == nil && s.Args != nil {
			merged.Args = []string{}
		}

		amendConflictingContainerFields(&merged, s)

		// Pass through original step Script, for later conversion.
		newStep := Step{Script: s.Script, OnError: s.OnError, Timeout: s.Timeout, StdoutConfig: s.StdoutConfig, StderrConfig: s.StderrConfig}
		newStep.SetContainerFields(merged)
		steps[i] = newStep
	}
	return steps, nil
}

// MergeStepsWithOverrides takes a possibly nil list of overrides and a
// list of steps, merging each of the steps with the overrides' resource requirements, if
// it's not nil, and returning the resulting list.
func MergeStepsWithOverrides(steps []Step, overrides []TaskRunStepOverride) ([]Step, error) {
	stepNameToOverride := make(map[string]TaskRunStepOverride, len(overrides))
	for _, o := range overrides {
		stepNameToOverride[o.Name] = o
	}
	for i, s := range steps {
		o, found := stepNameToOverride[s.Name]
		if !found {
			continue
		}
		merged := v1.ResourceRequirements{}
		err := mergeObjWithTemplate(&s.Resources, &o.Resources, &merged)
		if err != nil {
			return nil, err
		}
		steps[i].Resources = merged
	}
	return steps, nil
}

// MergeSidecarsWithOverrides takes a possibly nil list of overrides and a
// list of sidecars, merging each of the sidecars with the overrides' resource requirements, if
// it's not nil, and returning the resulting list.
func MergeSidecarsWithOverrides(sidecars []Sidecar, overrides []TaskRunSidecarOverride) ([]Sidecar, error) {
	if len(overrides) == 0 {
		return sidecars, nil
	}
	sidecarNameToOverride := make(map[string]TaskRunSidecarOverride, len(overrides))
	for _, o := range overrides {
		sidecarNameToOverride[o.Name] = o
	}
	for i, s := range sidecars {
		o, found := sidecarNameToOverride[s.Name]
		if !found {
			continue
		}
		merged := v1.ResourceRequirements{}
		err := mergeObjWithTemplate(&s.Resources, &o.Resources, &merged)
		if err != nil {
			return nil, err
		}
		sidecars[i].Resources = merged
	}
	return sidecars, nil
}

// mergeObjWithTemplate merges obj with template and updates out to reflect the merged result.
// template, obj, and out should point to the same type. out points to the zero value of that type.
func mergeObjWithTemplate(template, obj, out interface{}) error {
	md, err := getMergeData(template, out)
	if err != nil {
		return err
	}
	return mergeObjWithTemplateBytes(md, obj, out)
}

// getMergeData serializes the template and empty object to get the intermediate results necessary for
// merging an object of the same type with this template.
// This function is provided to avoid repeatedly serializing an identical template.
func getMergeData(template, empty interface{}) (*mergeData, error) {
	// We need JSON bytes to generate a patch to merge the object
	// onto the template, so marshal the template.
	templateJSON, err := json.Marshal(template)
	if err != nil {
		return nil, err
	}
	// We need to do a three-way merge to actually merge the template and
	// object, so we need an empty object as the "original"
	emptyJSON, err := json.Marshal(empty)
	if err != nil {
		return nil, err
	}
	// Get the patch meta, which is needed for generating and applying the merge patch.
	patchSchema, err := strategicpatch.NewPatchMetaFromStruct(template)
	if err != nil {
		return nil, err
	}
	return &mergeData{templateJSON: templateJSON, emptyJSON: emptyJSON, patchSchema: patchSchema}, nil
}

// mergeObjWithTemplateBytes merges obj with md's template JSON and updates out to reflect the merged result.
// out is a pointer to the zero value of obj's type.
// This function is provided to avoid repeatedly serializing an identical template.
func mergeObjWithTemplateBytes(md *mergeData, obj, out interface{}) error {
	// Marshal the object to JSON
	objAsJSON, err := json.Marshal(obj)
	if err != nil {
		return err
	}
	// Create a merge patch, with the empty JSON as the original, the object JSON as the modified, and the template
	// JSON as the current - this lets us do a deep merge of the template and object, with awareness of
	// the "patchMerge" tags.
	patch, err := strategicpatch.CreateThreeWayMergePatch(md.emptyJSON, objAsJSON, md.templateJSON, md.patchSchema, true)
	if err != nil {
		return err
	}

	// Actually apply the merge patch to the template JSON.
	mergedAsJSON, err := strategicpatch.StrategicMergePatchUsingLookupPatchMeta(md.templateJSON, patch, md.patchSchema)
	if err != nil {
		return err
	}
	// Unmarshal the merged JSON to a pointer, and return it.
	return json.Unmarshal(mergedAsJSON, out)
}

// amendConflictingContainerFields amends conflicting container fields after merge, and overrides conflicting fields
// by fields in step.
func amendConflictingContainerFields(container *corev1.Container, step Step) {
	if container == nil || len(step.Env) == 0 {
		return
	}

	envNameToStepEnv := make(map[string]corev1.EnvVar, len(step.Env))
	for _, e := range step.Env {
		envNameToStepEnv[e.Name] = e
	}

	for index, env := range container.Env {
		if env.ValueFrom != nil && len(env.Value) > 0 {
			if e, ok := envNameToStepEnv[env.Name]; ok {
				container.Env[index] = e
			}
		}
	}
}
