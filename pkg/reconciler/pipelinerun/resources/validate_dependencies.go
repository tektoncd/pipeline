/*
Copyright 2021 The Tekton Authors

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

package resources

import (
	"fmt"

	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"k8s.io/apimachinery/pkg/util/sets"
)

// ValidatePipelineTaskResults ensures that any result references used by pipeline tasks
// resolve to valid results. This prevents a situation where a PipelineTask references
// a result in another PipelineTask that doesn't exist or where the user has either misspelled
// a result name or the referenced task just doesn't return a result with that name.
func ValidatePipelineTaskResults(state PipelineRunState) error {
	ptMap := state.ToMap()
	for _, rpt := range state {
		for _, ref := range v1.PipelineTaskResultRefs(rpt.PipelineTask) {
			if err := validateResultRef(ref, ptMap); err != nil {
				return fmt.Errorf("invalid result reference in pipeline task %q: %w", rpt.PipelineTask.Name, err)
			}
		}
	}
	return nil
}

// ValidatePipelineResults ensures that any result references used by PipelineResults
// resolve to valid results. This prevents a situation where a PipelineResult references
// a result in a PipelineTask that doesn't exist or where the user has either misspelled
// a result name or the referenced task just doesn't return a result with that name.
func ValidatePipelineResults(ps *v1.PipelineSpec, state PipelineRunState) error {
	ptMap := state.ToMap()
	for _, result := range ps.Results {
		expressions, _ := v1.GetVarSubstitutionExpressionsForPipelineResult(result)
		refs := v1.NewResultRefs(expressions)
		for _, ref := range refs {
			if err := validateResultRef(ref, ptMap); err != nil {
				return fmt.Errorf("invalid pipeline result %q: %w", result.Name, err)
			}
		}
	}
	return nil
}

// validateResultRef takes a ResultRef and searches for the result using the given
// map of PipelineTask name to ResolvedPipelineTask. If the ResultRef does not point
// to a pipeline task or named result then an error is returned.
func validateResultRef(ref *v1.ResultRef, ptMap map[string]*ResolvedPipelineTask) error {
	if _, ok := ptMap[ref.PipelineTask]; !ok {
		return fmt.Errorf("referenced pipeline task %q does not exist", ref.PipelineTask)
	}
	taskProvidesResult := false
	if ptMap[ref.PipelineTask].CustomTask {
		// We're not able to validate results pointing to custom tasks because
		// there's no facility to check what the result names will be before the
		// custom task executes.
		return nil
	}
	if ptMap[ref.PipelineTask].ResolvedTask == nil || ptMap[ref.PipelineTask].ResolvedTask.TaskSpec == nil {
		return fmt.Errorf("unable to validate result referencing pipeline task %q: task spec not found", ref.PipelineTask)
	}
	for _, taskResult := range ptMap[ref.PipelineTask].ResolvedTask.TaskSpec.Results {
		if taskResult.Name == ref.Result {
			taskProvidesResult = true
			break
		}
	}
	if !taskProvidesResult {
		return fmt.Errorf("%q is not a named result returned by pipeline task %q", ref.Result, ref.PipelineTask)
	}
	return nil
}

// ValidateOptionalWorkspaces validates that any workspaces in the Pipeline that are
// marked as optional are also marked optional in the Tasks that receive them. This
// prevents a situation where a Task requires a workspace but a Pipeline does not offer
// the same guarantee the workspace will be provided at runtime.
func ValidateOptionalWorkspaces(pipelineWorkspaces []v1.PipelineWorkspaceDeclaration, state PipelineRunState) error {
	optionalWorkspaces := sets.NewString()
	for _, ws := range pipelineWorkspaces {
		if ws.Optional {
			optionalWorkspaces.Insert(ws.Name)
		}
	}

	for _, rpt := range state {
		for _, pws := range rpt.PipelineTask.Workspaces {
			if optionalWorkspaces.Has(pws.Workspace) {
				for _, tws := range rpt.ResolvedTask.TaskSpec.Workspaces {
					if tws.Name == pws.Name {
						if !tws.Optional {
							return fmt.Errorf("pipeline workspace %q is marked optional but pipeline task %q requires it be provided", pws.Workspace, rpt.PipelineTask.Name)
						}
					}
				}
			}
		}
	}
	return nil
}
