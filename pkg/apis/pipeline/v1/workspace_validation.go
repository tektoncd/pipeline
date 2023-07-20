/*
Copyright 2022 The Tekton Authors

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

package v1

import (
	"context"

	"k8s.io/apimachinery/pkg/api/equality"
	"knative.dev/pkg/apis"
)

// allVolumeSourceFields is a list of all the volume source field paths that a
// WorkspaceBinding may include.
var allVolumeSourceFields = []string{
	"persistentvolumeclaim",
	"volumeclaimtemplate",
	"emptydir",
	"configmap",
	"secret",
}

// Validate looks at the Volume provided in wb and makes sure that it is valid.
// This means that only one VolumeSource can be specified, and also that the
// supported VolumeSource is itself valid.
func (b *WorkspaceBinding) Validate(ctx context.Context) (errs *apis.FieldError) {
	if equality.Semantic.DeepEqual(b, &WorkspaceBinding{}) || b == nil {
		return apis.ErrMissingField(apis.CurrentField)
	}

	numSources := b.numSources()

	if numSources > 1 {
		return apis.ErrMultipleOneOf(allVolumeSourceFields...)
	}

	if numSources == 0 {
		return apis.ErrMissingOneOf(allVolumeSourceFields...)
	}

	// For a PersistentVolumeClaim to work, you must at least provide the name of the PVC to use.
	if b.PersistentVolumeClaim != nil && b.PersistentVolumeClaim.ClaimName == "" {
		return apis.ErrMissingField("persistentvolumeclaim.claimname")
	}

	// For a ConfigMap to work, you must provide the name of the ConfigMap to use.
	if b.ConfigMap != nil && b.ConfigMap.LocalObjectReference.Name == "" {
		return apis.ErrMissingField("configmap.name")
	}

	// For a Secret to work, you must provide the name of the Secret to use.
	if b.Secret != nil && b.Secret.SecretName == "" {
		return apis.ErrMissingField("secret.secretName")
	}

	// For a Projected volume to work, you must provide at least one source.
	if b.Projected != nil && len(b.Projected.Sources) == 0 {
		if len(b.Projected.Sources) == 0 {
			return apis.ErrMissingField("projected.sources")
		}
	}

	// For a CSI to work, you must provide and have installed the driver to use.
	if b.CSI != nil {
		if b.CSI.Driver == "" {
			return apis.ErrMissingField("csi.driver")
		}
	}

	return nil
}

// numSources returns the total number of volume sources that this WorkspaceBinding
// has been configured with.
func (b *WorkspaceBinding) numSources() int {
	n := 0
	if b.VolumeClaimTemplate != nil {
		n++
	}
	if b.PersistentVolumeClaim != nil {
		n++
	}
	if b.EmptyDir != nil {
		n++
	}
	if b.ConfigMap != nil {
		n++
	}
	if b.Secret != nil {
		n++
	}
	if b.Projected != nil {
		n++
	}
	if b.CSI != nil {
		n++
	}
	return n
}
