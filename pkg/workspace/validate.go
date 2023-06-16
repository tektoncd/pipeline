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

package workspace

import (
	"context"
	"fmt"

	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"k8s.io/apimachinery/pkg/util/sets"
)

// ValidateBindings will return an error if the bound workspaces in binds don't satisfy the declared
// workspaces in decls.
func ValidateBindings(ctx context.Context, decls []v1.WorkspaceDeclaration, binds []v1.WorkspaceBinding) error {
	// This will also be validated at webhook time but in case the webhook isn't invoked for some
	// reason we'll invoke the same validation here.
	for _, b := range binds {
		if err := b.Validate(ctx); err != nil {
			return fmt.Errorf("binding %q is invalid: %w", b.Name, err)
		}
	}

	declNames := sets.NewString()
	bindNames := sets.NewString()
	for _, decl := range decls {
		declNames.Insert(decl.Name)
	}
	for _, bind := range binds {
		bindNames.Insert(bind.Name)
	}

	for _, decl := range decls {
		if decl.Optional {
			continue
		}
		if !bindNames.Has(decl.Name) {
			return fmt.Errorf("declared workspace %q is required but has not been bound", decl.Name)
		}
	}
	for _, bind := range binds {
		if !declNames.Has(bind.Name) {
			return fmt.Errorf("workspace binding %q does not match any declared workspace", bind.Name)
		}
	}

	return nil
}

// ValidateOnlyOnePVCIsUsed checks that a list of WorkspaceBinding uses only one
// persistent volume claim.
//
// This is only useful to validate that WorkspaceBindings in TaskRuns are compatible
// with affinity rules enforced by the AffinityAssistant.
func ValidateOnlyOnePVCIsUsed(wb []v1.WorkspaceBinding) error {
	workspaceVolumes := make(map[string]bool)
	for _, w := range wb {
		if w.PersistentVolumeClaim != nil {
			workspaceVolumes[w.PersistentVolumeClaim.ClaimName] = true
		}
		if w.VolumeClaimTemplate != nil {
			workspaceVolumes[w.Name] = true
		}
	}

	if len(workspaceVolumes) > 1 {
		return fmt.Errorf("more than one PersistentVolumeClaim is bound")
	}
	return nil
}
