package cmd

import (
	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/cli/pkg/test"
	"testing"
)

func TestCommand_invalid(t *testing.T) {
	p := &test.Params{}
	pipelinerun := Root(p)
	out, err := test.ExecuteCommand(pipelinerun)
	if err == nil {
		t.Errorf("No errors was defined. Output: %s", out)
	}
	expected := "Use one of available subcommands: completion [SHELL], help [command], pipeline, pipelinerun, task, taskrun, version"
	if d := cmp.Diff(expected, err.Error()); d != "" {
		t.Errorf("Unexpected output mismatch: %s", d)
	}
}
