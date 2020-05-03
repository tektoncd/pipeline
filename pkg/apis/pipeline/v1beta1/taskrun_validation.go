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
	"context"
	"fmt"
	"strings"

	"github.com/tektoncd/pipeline/pkg/apis/validate"
	"k8s.io/apimachinery/pkg/api/equality"
	"knative.dev/pkg/apis"
)

var _ apis.Validatable = (*TaskRun)(nil)

// Validate taskrun
func (tr *TaskRun) Validate(ctx context.Context) *apis.FieldError {
	if err := validate.ObjectMetadata(tr.GetObjectMeta()).ViaField("metadata"); err != nil {
		return err
	}
	return tr.Spec.Validate(ctx)
}

// Validate taskrun spec
func (ts *TaskRunSpec) Validate(ctx context.Context) *apis.FieldError {
	if equality.Semantic.DeepEqual(ts, &TaskRunSpec{}) {
		return apis.ErrMissingField("spec")
	}

	// can't have both taskRef and taskSpec at the same time
	if (ts.TaskRef != nil && ts.TaskRef.Name != "") && ts.TaskSpec != nil {
		return apis.ErrDisallowedFields("spec.taskref", "spec.taskspec")
	}

	// Check that one of TaskRef and TaskSpec is present
	if (ts.TaskRef == nil || (ts.TaskRef != nil && ts.TaskRef.Name == "")) && ts.TaskSpec == nil {
		return apis.ErrMissingField("spec.taskref.name", "spec.taskspec")
	}

	// Validate TaskSpec if it's present
	if ts.TaskSpec != nil {
		if err := ts.TaskSpec.Validate(ctx); err != nil {
			return err
		}
	}

	if err := validateParameters(ts.Params); err != nil {
		return err
	}

	if err := validateWorkspaceBindings(ctx, ts.Workspaces); err != nil {
		return err
	}

	// Validate Resources declaration
	if err := ts.Resources.Validate(ctx); err != nil {
		return err
	}

	if ts.Status != "" {
		if ts.Status != TaskRunSpecStatusCancelled {
			return apis.ErrInvalidValue(fmt.Sprintf("%s should be %s", ts.Status, TaskRunSpecStatusCancelled), "spec.status")
		}
	}

	if ts.Timeout != nil {
		// timeout should be a valid duration of at least 0.
		if ts.Timeout.Duration < 0 {
			return apis.ErrInvalidValue(fmt.Sprintf("%s should be >= 0", ts.Timeout.Duration.String()), "spec.timeout")
		}
	}

	return nil
}

// validateWorkspaceBindings makes sure the volumes provided for the Task's declared workspaces make sense.
func validateWorkspaceBindings(ctx context.Context, wb []WorkspaceBinding) *apis.FieldError {
	seen := map[string]struct{}{}
	for _, w := range wb {
		if _, ok := seen[w.Name]; ok {
			return apis.ErrMultipleOneOf("spec.workspaces.name")
		}
		seen[w.Name] = struct{}{}

		if err := w.Validate(ctx); err != nil {
			return err
		}
	}

	return nil
}

func validateParameters(params []Param) *apis.FieldError {
	// Template must not duplicate parameter names.
	seen := map[string]struct{}{}
	for _, p := range params {
		if _, ok := seen[strings.ToLower(p.Name)]; ok {
			return apis.ErrMultipleOneOf("spec.params.name")
		}
		seen[p.Name] = struct{}{}
	}
	return nil
}
