package resources

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	tb "github.com/tektoncd/pipeline/test/builder"
)

func TestResolveWorkspaces(t *testing.T) {
	for _, tc := range []struct {
		description string
		in          *v1alpha1.Pipeline
		expected    *v1alpha1.Pipeline
	}{{
		description: "resolve workspace without variable",
		in: tb.Pipeline("test-pipeline", "foo", tb.PipelineSpec(
			tb.PipelineWorkspaceDeclaration("named-workspace1"),
			tb.PipelineTask("pt1", "task",
				tb.PipelineTaskWorkspaceBinding("foo", tb.PipelineTaskWorkspaceBindingWorkspace("named-workspace1")),
			),
		)),
		expected: tb.Pipeline("test-pipeline", "foo", tb.PipelineSpec(
			tb.PipelineWorkspaceDeclaration("named-workspace1"),
			tb.PipelineTask("pt1", "task",
				tb.PipelineTaskWorkspaceBinding("foo", tb.PipelineTaskWorkspaceBindingWorkspace("named-workspace1")),
			),
		)),
	}, {
		description: "resolve workspace variable to absolute name",
		in: tb.Pipeline("test-pipeline", "foo", tb.PipelineSpec(
			tb.PipelineWorkspaceDeclaration("named-workspace1"),
			tb.PipelineTask("pt1", "task",
				tb.PipelineTaskWorkspaceBinding("foo", tb.PipelineTaskWorkspaceBindingWorkspace("named-workspace1")),
			),
			tb.PipelineTask("pt2", "task",
				tb.PipelineTaskWorkspaceBinding("foo", tb.PipelineTaskWorkspaceBindingFrom("pt1", "foo")),
			),
		)),
		expected: tb.Pipeline("test-pipeline", "foo", tb.PipelineSpec(
			tb.PipelineWorkspaceDeclaration("named-workspace1"),
			tb.PipelineTask("pt1", "task",
				tb.PipelineTaskWorkspaceBinding("foo", tb.PipelineTaskWorkspaceBindingWorkspace("named-workspace1")),
			),
			tb.PipelineTask("pt2", "task",
				tb.PipelineTaskWorkspaceBinding("foo", tb.PipelineTaskWorkspaceBindingWorkspace("named-workspace1")),
			),
		)),
	}, {
		description: "two variable hops to resolve a workspace name",
		in: tb.Pipeline("test-pipeline", "foo", tb.PipelineSpec(
			tb.PipelineWorkspaceDeclaration("named-workspace1"),
			tb.PipelineTask("pt1", "task",
				tb.PipelineTaskWorkspaceBinding("foo", tb.PipelineTaskWorkspaceBindingWorkspace("named-workspace1")),
			),
			tb.PipelineTask("pt2", "task",
				tb.PipelineTaskWorkspaceBinding("foo", tb.PipelineTaskWorkspaceBindingFrom("pt1", "foo")),
			),
			tb.PipelineTask("pt3", "task",
				tb.PipelineTaskWorkspaceBinding("foo", tb.PipelineTaskWorkspaceBindingFrom("pt2", "foo")),
			),
		)),
		expected: tb.Pipeline("test-pipeline", "foo", tb.PipelineSpec(
			tb.PipelineWorkspaceDeclaration("named-workspace1"),
			tb.PipelineTask("pt1", "task",
				tb.PipelineTaskWorkspaceBinding("foo", tb.PipelineTaskWorkspaceBindingWorkspace("named-workspace1")),
			),
			tb.PipelineTask("pt2", "task",
				tb.PipelineTaskWorkspaceBinding("foo", tb.PipelineTaskWorkspaceBindingWorkspace("named-workspace1")),
			),
			tb.PipelineTask("pt3", "task",
				tb.PipelineTaskWorkspaceBinding("foo", tb.PipelineTaskWorkspaceBindingWorkspace("named-workspace1")),
			),
		)),
	}, {
		description: "multiple variables resolving to different workspace names",
		in: tb.Pipeline("test-pipeline", "foo", tb.PipelineSpec(
			tb.PipelineWorkspaceDeclaration("named-workspace1", "named-workspace2"),
			tb.PipelineTask("pt1", "task",
				tb.PipelineTaskWorkspaceBinding("foo", tb.PipelineTaskWorkspaceBindingWorkspace("named-workspace1")),
				tb.PipelineTaskWorkspaceBinding("bar", tb.PipelineTaskWorkspaceBindingWorkspace("named-workspace2")),
			),
			tb.PipelineTask("pt2", "task",
				tb.PipelineTaskWorkspaceBinding("foo", tb.PipelineTaskWorkspaceBindingFrom("pt1", "foo")),
			),
			tb.PipelineTask("pt3", "task",
				tb.PipelineTaskWorkspaceBinding("foo", tb.PipelineTaskWorkspaceBindingFrom("pt1", "bar")),
			),
			tb.PipelineTask("pt4", "task",
				tb.PipelineTaskWorkspaceBinding("foo", tb.PipelineTaskWorkspaceBindingFrom("pt2", "foo")),
			),
			tb.PipelineTask("pt5", "task",
				tb.PipelineTaskWorkspaceBinding("foo", tb.PipelineTaskWorkspaceBindingFrom("pt3", "foo")),
			),
		)),
		expected: tb.Pipeline("test-pipeline", "foo", tb.PipelineSpec(
			tb.PipelineWorkspaceDeclaration("named-workspace1", "named-workspace2"),
			tb.PipelineTask("pt1", "task",
				tb.PipelineTaskWorkspaceBinding("foo", tb.PipelineTaskWorkspaceBindingWorkspace("named-workspace1")),
				tb.PipelineTaskWorkspaceBinding("bar", tb.PipelineTaskWorkspaceBindingWorkspace("named-workspace2")),
			),
			tb.PipelineTask("pt2", "task",
				tb.PipelineTaskWorkspaceBinding("foo", tb.PipelineTaskWorkspaceBindingWorkspace("named-workspace1")),
			),
			tb.PipelineTask("pt3", "task",
				tb.PipelineTaskWorkspaceBinding("foo", tb.PipelineTaskWorkspaceBindingWorkspace("named-workspace2")),
			),
			tb.PipelineTask("pt4", "task",
				tb.PipelineTaskWorkspaceBinding("foo", tb.PipelineTaskWorkspaceBindingWorkspace("named-workspace1")),
			),
			tb.PipelineTask("pt5", "task",
				tb.PipelineTaskWorkspaceBinding("foo", tb.PipelineTaskWorkspaceBindingWorkspace("named-workspace2")),
			),
		)),
	}} {
		t.Run(tc.description, func(t *testing.T) {
			result, err := ResolveWorkspaces(tc.in.Spec.DeepCopy())
			if err != nil {
				t.Errorf("error resolving workspace: %v", err)
			}
			if d := cmp.Diff(&tc.expected.Spec, result); d != "" {
				t.Errorf("ResolveWorkspaces diff -want, +got %s", d)
			}
		})
	}
}

func TestResolveWorkspaces_Invalid(t *testing.T) {
	for _, tc := range []struct {
		description string
		in          *v1alpha1.Pipeline
	}{{
		description: "cycles of workspace variable substitutions are invalid",
		in: tb.Pipeline("test-pipeline", "foo", tb.PipelineSpec(
			tb.PipelineWorkspaceDeclaration("named-workspace1"),
			tb.PipelineTask("pt1", "task",
				tb.PipelineTaskWorkspaceBinding("foo", tb.PipelineTaskWorkspaceBindingFrom("pt2", "foo")),
			),
			tb.PipelineTask("pt2", "task",
				tb.PipelineTaskWorkspaceBinding("foo", tb.PipelineTaskWorkspaceBindingFrom("pt1", "foo")),
			),
		)),
	}, {
		description: "workspace variables that do not resolve to a pipeline task that exists result in an error",
		in: tb.Pipeline("test-pipeline", "foo", tb.PipelineSpec(
			tb.PipelineWorkspaceDeclaration("named-workspace1"),
			tb.PipelineTask("pt1", "task",
				tb.PipelineTaskWorkspaceBinding("foo", tb.PipelineTaskWorkspaceBindingWorkspace("named-workspace1")),
			),
			tb.PipelineTask("pt2", "task",
				tb.PipelineTaskWorkspaceBinding("foo", tb.PipelineTaskWorkspaceBindingFrom("pt3", "foo")),
			),
		)),
	}, {
		description: "workspace variables that do not resolve to a workspace that exists result in an error",
		in: tb.Pipeline("test-pipeline", "foo", tb.PipelineSpec(
			tb.PipelineWorkspaceDeclaration("named-workspace1"),
			tb.PipelineTask("pt1", "task",
				tb.PipelineTaskWorkspaceBinding("bar", tb.PipelineTaskWorkspaceBindingWorkspace("named-workspace1")),
			),
			tb.PipelineTask("pt2", "task",
				tb.PipelineTaskWorkspaceBinding("foo", tb.PipelineTaskWorkspaceBindingFrom("pt1", "foo")),
			),
		)),
	}} {
		t.Run(tc.description, func(t *testing.T) {
			_, err := ResolveWorkspaces(tc.in.Spec.DeepCopy())
			if err == nil {
				t.Errorf("error expected but not received")
			}
		})
	}
}

func TestGetNextWorkspace(t *testing.T) {
	for _, tc := range []struct {
		description   string
		bindings      v1alpha1.PipelineTaskWorkspaceBinding
		tasks         map[string]*v1alpha1.PipelineTask
		expectBinding *v1alpha1.PipelineTaskWorkspaceBinding
		expectError   bool
	}{{
		description: "correctly selects next binding from set of pipeline tasks and their workspaces",
		bindings: v1alpha1.PipelineTaskWorkspaceBinding{
			Name: "foo",
			From: v1alpha1.WorkspaceFromClause{
				Task: "baz",
				Name: "ws",
			},
		},
		tasks: map[string]*v1alpha1.PipelineTask{
			"bar": {
				Name: "bar",
				Workspaces: []v1alpha1.PipelineTaskWorkspaceBinding{{
					Name: "ws",
				}},
			},
			"baz": {
				Name: "baz",
				Workspaces: []v1alpha1.PipelineTaskWorkspaceBinding{{
					Name: "ws",
				}},
			},
		},
		expectBinding: &v1alpha1.PipelineTaskWorkspaceBinding{
			Name: "ws",
		},
	}, {
		description: "returns error if a binding's task name doesnt map to a task",
		bindings: v1alpha1.PipelineTaskWorkspaceBinding{
			Name: "foo",
			From: v1alpha1.WorkspaceFromClause{
				Task: "bar",
				Name: "",
			},
		},
		tasks:       map[string]*v1alpha1.PipelineTask{},
		expectError: true,
	}, {
		description: "returns error if a binding's workspace doesnt map to a task's workspace",
		bindings: v1alpha1.PipelineTaskWorkspaceBinding{
			Name: "foo",
			From: v1alpha1.WorkspaceFromClause{
				Task: "bar",
				Name: "ws-missing",
			},
		},
		tasks: map[string]*v1alpha1.PipelineTask{
			"bar": {
				Name: "bar",
				Workspaces: []v1alpha1.PipelineTaskWorkspaceBinding{{
					Name: "ws-actual",
				}},
			},
		},
		expectError: true,
	}, {
		description: "returns error if a binding doesn't include a from clause",
		bindings: v1alpha1.PipelineTaskWorkspaceBinding{
			Name: "foo",
		},
		tasks: map[string]*v1alpha1.PipelineTask{
			"bar": {
				Name: "bar",
				Workspaces: []v1alpha1.PipelineTaskWorkspaceBinding{{
					Name: "ws",
				}},
			},
		},
		expectError: true,
	}} {
		t.Run(tc.description, func(t *testing.T) {
			if result, err := getNextWorkspace(&tc.bindings, tc.tasks); err != nil {
				if !tc.expectError {
					t.Errorf("unexpected error received: %v", err)
				}
			} else if d := cmp.Diff(tc.expectBinding, result); d != "" {
				t.Errorf("ResolveWorkspaces diff -want, +got %s", d)
			}
		})
	}
}
