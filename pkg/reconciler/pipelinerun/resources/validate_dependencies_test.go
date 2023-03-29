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
	"strings"
	"testing"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/pkg/reconciler/taskrun/resources"
	"k8s.io/apimachinery/pkg/selection"
)

// TestValidatePipelineTaskResults_ValidStates tests that a pipeline task with
// valid content and result variables does not trigger validation errors.
func TestValidatePipelineTaskResults_ValidStates(t *testing.T) {
	for _, tc := range []struct {
		desc  string
		state PipelineRunState
	}{{
		desc: "no variables used",
		state: PipelineRunState{{
			PipelineTask: &v1beta1.PipelineTask{
				Name: "pt1",
				Params: v1beta1.Params{{
					Name:  "p1",
					Value: *v1beta1.NewStructuredValues("foo"),
				}},
			},
		}},
	}, {
		desc: "correct use of task and result names",
		state: PipelineRunState{{
			PipelineTask: &v1beta1.PipelineTask{
				Name: "pt1",
			},
			ResolvedTask: &resources.ResolvedTask{
				TaskName: "t",
				TaskSpec: &v1beta1.TaskSpec{
					Results: []v1beta1.TaskResult{{
						Name: "result",
					}},
				},
			},
		}, {
			PipelineTask: &v1beta1.PipelineTask{
				Name: "pt2",
				Params: v1beta1.Params{{
					Name:  "p",
					Value: *v1beta1.NewStructuredValues("$(tasks.pt1.results.result)"),
				}},
			},
		}},
	}, {
		desc: "correct use of task and result names in matrix",
		state: PipelineRunState{{
			PipelineTask: &v1beta1.PipelineTask{
				Name: "pt1",
			},
			ResolvedTask: &resources.ResolvedTask{
				TaskName: "t",
				TaskSpec: &v1beta1.TaskSpec{
					Results: []v1beta1.TaskResult{{
						Name: "result",
					}},
				},
			},
		}, {
			PipelineTask: &v1beta1.PipelineTask{
				Name: "pt2",
				Matrix: &v1beta1.Matrix{
					Params: v1beta1.Params{{
						Name:  "p",
						Value: *v1beta1.NewStructuredValues("$(tasks.pt1.results.result)", "foo"),
					}}},
			},
		}},
	}, {
		desc: "custom task results are not validated",
		state: PipelineRunState{{
			PipelineTask: &v1beta1.PipelineTask{
				Name: "pt1",
			},
			CustomTask:    true,
			RunObjectName: "foo-run",
		}, {
			PipelineTask: &v1beta1.PipelineTask{
				Name: "pt2",
				Params: v1beta1.Params{{
					Name:  "p",
					Value: *v1beta1.NewStructuredValues("$(tasks.pt1.results.a-dynamic-custom-task-result)"),
				}},
			},
		}},
	}} {
		t.Run(tc.desc, func(t *testing.T) {
			if err := ValidatePipelineTaskResults(tc.state); err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// TestValidatePipelineTaskResults_IncorrectTaskName tests that a result variable with
// a misnamed PipelineTask is correctly caught by the validatePipelineTaskResults func.
func TestValidatePipelineTaskResults_IncorrectTaskName(t *testing.T) {
	missingPipelineTaskVariable := "$(tasks.pt2.results.result1)"
	for _, tc := range []struct {
		desc  string
		state PipelineRunState
	}{{
		desc: "invalid result reference in param",
		state: PipelineRunState{{
			PipelineTask: &v1beta1.PipelineTask{
				Name: "pt1",
				Params: v1beta1.Params{{
					Name:  "p1",
					Value: *v1beta1.NewStructuredValues(missingPipelineTaskVariable),
				}},
			},
		}},
	}, {
		desc: "invalid result reference in matrix",
		state: PipelineRunState{{
			PipelineTask: &v1beta1.PipelineTask{
				Name: "pt1",
				Params: v1beta1.Params{{
					Name:  "p1",
					Value: *v1beta1.NewStructuredValues(missingPipelineTaskVariable, "foo"),
				}},
			},
		}},
	}, {
		desc: "invalid result reference in when expression",
		state: PipelineRunState{{
			PipelineTask: &v1beta1.PipelineTask{
				Name: "pt1",
				WhenExpressions: []v1beta1.WhenExpression{{
					Input:    "foo",
					Operator: selection.In,
					Values: []string{
						missingPipelineTaskVariable,
					},
				}},
			},
		}},
	}} {
		t.Run(tc.desc, func(t *testing.T) {
			err := ValidatePipelineTaskResults(tc.state)
			if err == nil || !strings.Contains(err.Error(), `referenced pipeline task "pt2" does not exist`) {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// TestValidatePipelineTaskResults_IncorrectResultName tests that a result variable with
// a misnamed Result is correctly caught by the validatePipelineTaskResults func.
func TestValidatePipelineTaskResults_IncorrectResultName(t *testing.T) {
	pt1 := &ResolvedPipelineTask{
		PipelineTask: &v1beta1.PipelineTask{
			Name: "pt1",
		},
		ResolvedTask: &resources.ResolvedTask{
			TaskName: "t",
			TaskSpec: &v1beta1.TaskSpec{
				Results: []v1beta1.TaskResult{{
					Name: "not-the-result-youre-looking-for",
				}},
			},
		},
	}
	for _, tc := range []struct {
		desc  string
		state PipelineRunState
	}{{
		desc: "invalid result reference in param",
		state: PipelineRunState{pt1, {
			PipelineTask: &v1beta1.PipelineTask{
				Name: "pt2",
				Params: v1beta1.Params{{
					Name:  "p1",
					Value: *v1beta1.NewStructuredValues("$(tasks.pt1.results.result1)"),
				}},
			},
		}},
	}, {
		desc: "invalid result reference in matrix",
		state: PipelineRunState{pt1, {
			PipelineTask: &v1beta1.PipelineTask{
				Name: "pt2",
				Matrix: &v1beta1.Matrix{
					Params: v1beta1.Params{{
						Name:  "p1",
						Value: *v1beta1.NewStructuredValues("$(tasks.pt1.results.result1)", "$(tasks.pt1.results.result2)"),
					}}},
			},
		}},
	}, {
		desc: "invalid result reference in when expression",
		state: PipelineRunState{pt1, {
			PipelineTask: &v1beta1.PipelineTask{
				Name: "pt2",
				WhenExpressions: []v1beta1.WhenExpression{{
					Input:    "foo",
					Operator: selection.In,
					Values: []string{
						"$(tasks.pt1.results.result1)",
					},
				}},
			},
		}},
	}} {
		t.Run(tc.desc, func(t *testing.T) {
			err := ValidatePipelineTaskResults(tc.state)
			if err == nil || !strings.Contains(err.Error(), `"result1" is not a named result returned by pipeline task "pt1"`) {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// TestValidatePipelineTaskResults_MissingTaskSpec tests that a malformed PipelineTask
// with a name but no spec results in a validation error being returned.
func TestValidatePipelineTaskResults_MissingTaskSpec(t *testing.T) {
	pt1 := &ResolvedPipelineTask{
		PipelineTask: &v1beta1.PipelineTask{
			Name: "pt1",
		},
		ResolvedTask: &resources.ResolvedTask{
			TaskName: "t",
			TaskSpec: nil,
		},
	}
	state := PipelineRunState{pt1, {
		PipelineTask: &v1beta1.PipelineTask{
			Name: "pt2",
			Params: v1beta1.Params{{
				Name:  "p1",
				Value: *v1beta1.NewStructuredValues("$(tasks.pt1.results.result1)"),
			}},
		},
	}, {
		PipelineTask: &v1beta1.PipelineTask{
			Name: "pt3",
			Matrix: &v1beta1.Matrix{
				Params: v1beta1.Params{{
					Name:  "p1",
					Value: *v1beta1.NewStructuredValues("$(tasks.pt1.results.result1)", "$(tasks.pt1.results.result2)"),
				}}},
		},
	}}
	err := ValidatePipelineTaskResults(state)
	if err == nil || !strings.Contains(err.Error(), `task spec not found`) {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestValidatePipelineResults_ValidStates tests that a pipeline results with
// valid content and result variables do not trigger a validation error.
func TestValidatePipelineResults_ValidStates(t *testing.T) {
	for _, tc := range []struct {
		desc  string
		spec  *v1beta1.PipelineSpec
		state PipelineRunState
	}{{
		desc: "no result variables",
		spec: &v1beta1.PipelineSpec{
			Results: []v1beta1.PipelineResult{{
				Name:  "foo-result",
				Value: *v1beta1.NewStructuredValues("just a text pipeline result"),
			}},
		},
		state: nil,
	}, {
		desc: "correct use of task and result names",
		spec: &v1beta1.PipelineSpec{
			Results: []v1beta1.PipelineResult{{
				Name:  "foo-result",
				Value: *v1beta1.NewStructuredValues("test $(tasks.pt1.results.result1) 123"),
			}},
		},
		state: PipelineRunState{{
			PipelineTask: &v1beta1.PipelineTask{
				Name: "pt1",
			},
			ResolvedTask: &resources.ResolvedTask{
				TaskName: "t",
				TaskSpec: &v1beta1.TaskSpec{
					Results: []v1beta1.TaskResult{{
						Name: "result1",
					}},
				},
			},
		}},
	}} {
		t.Run(tc.desc, func(t *testing.T) {
			if err := ValidatePipelineResults(tc.spec, tc.state); err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// TestValidatePipelineResults tests that a result variable used in a PipelineResult
// with a misnamed PipelineTask is correctly caught by the validatePipelineResults func.
func TestValidatePipelineResults_IncorrectTaskName(t *testing.T) {
	spec := &v1beta1.PipelineSpec{
		Results: []v1beta1.PipelineResult{{
			Name:  "foo-result",
			Value: *v1beta1.NewStructuredValues("$(tasks.pt1.results.result1)"),
		}},
	}
	state := PipelineRunState{}
	err := ValidatePipelineResults(spec, state)
	if err == nil || !strings.Contains(err.Error(), `referenced pipeline task "pt1" does not exist`) {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestValidatePipelineResults tests that a result variable used in a PipelineResult
// with a misnamed Result is correctly caught by the validatePipelineResults func.
func TestValidatePipelineResults_IncorrectResultName(t *testing.T) {
	spec := &v1beta1.PipelineSpec{
		Results: []v1beta1.PipelineResult{{
			Name:  "foo-result",
			Value: *v1beta1.NewStructuredValues("$(tasks.pt1.results.result1)"),
		}},
	}
	state := PipelineRunState{{
		PipelineTask: &v1beta1.PipelineTask{
			Name: "pt1",
		},
		ResolvedTask: &resources.ResolvedTask{
			TaskName: "t",
			TaskSpec: &v1beta1.TaskSpec{
				Results: []v1beta1.TaskResult{{
					Name: "not-the-result-youre-looking-for",
				}},
			},
		},
	}}
	err := ValidatePipelineResults(spec, state)
	if err == nil || !strings.Contains(err.Error(), `"result1" is not a named result returned by pipeline task "pt1"`) {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestValidateOptionalWorkspaces_ValidStates tests that a pipeline sending
// correctly configured optional workspaces does not trigger validation errors.
func TestValidateOptionalWorkspaces_ValidStates(t *testing.T) {
	for _, tc := range []struct {
		desc       string
		workspaces []v1beta1.PipelineWorkspaceDeclaration
		state      PipelineRunState
	}{{
		desc:       "no workspaces declared",
		workspaces: nil,
		state: PipelineRunState{{
			PipelineTask: &v1beta1.PipelineTask{
				Name:       "pt1",
				Workspaces: nil,
			},
			ResolvedTask: &resources.ResolvedTask{
				TaskSpec: &v1beta1.TaskSpec{
					Workspaces: nil,
				},
			},
		}},
	}, {
		desc:       "pipeline can omit workspace if task workspace is optional",
		workspaces: nil,
		state: PipelineRunState{{
			PipelineTask: &v1beta1.PipelineTask{
				Name:       "pt1",
				Workspaces: []v1beta1.WorkspacePipelineTaskBinding{},
			},
			ResolvedTask: &resources.ResolvedTask{
				TaskSpec: &v1beta1.TaskSpec{
					Workspaces: []v1beta1.WorkspaceDeclaration{{
						Name:     "foo",
						Optional: true,
					}},
				},
			},
		}},
	}, {
		desc: "optional pipeline workspace matches optional task workspace",
		workspaces: []v1beta1.PipelineWorkspaceDeclaration{{
			Name:     "ws1",
			Optional: true,
		}},
		state: PipelineRunState{{
			PipelineTask: &v1beta1.PipelineTask{
				Name: "pt1",
				Workspaces: []v1beta1.WorkspacePipelineTaskBinding{{
					Name:      "foo",
					Workspace: "ws1",
				}},
			},
			ResolvedTask: &resources.ResolvedTask{
				TaskSpec: &v1beta1.TaskSpec{
					Workspaces: []v1beta1.WorkspaceDeclaration{{
						Name:     "foo",
						Optional: true,
					}},
				},
			},
		}},
	}} {
		t.Run(tc.desc, func(t *testing.T) {
			if err := ValidateOptionalWorkspaces(tc.workspaces, tc.state); err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// TestValidateOptionalWorkspaces tests that an error is generated if an optional pipeline
// workspace is bound to a non-optional task workspace.
func TestValidateOptionalWorkspaces_NonOptionalTaskWorkspace(t *testing.T) {
	workspaces := []v1beta1.PipelineWorkspaceDeclaration{{
		Name:     "ws1",
		Optional: true,
	}}
	state := PipelineRunState{{
		PipelineTask: &v1beta1.PipelineTask{
			Name: "pt1",
			Workspaces: []v1beta1.WorkspacePipelineTaskBinding{{
				Name:      "foo",
				Workspace: "ws1",
			}},
		},
		ResolvedTask: &resources.ResolvedTask{
			TaskSpec: &v1beta1.TaskSpec{
				Workspaces: []v1beta1.WorkspaceDeclaration{{
					Name:     "foo",
					Optional: false,
				}},
			},
		},
	}}
	err := ValidateOptionalWorkspaces(workspaces, state)
	if err == nil || !strings.Contains(err.Error(), `pipeline workspace "ws1" is marked optional but pipeline task "pt1" requires it be provided`) {
		t.Errorf("unexpected error: %v", err)
	}
}
