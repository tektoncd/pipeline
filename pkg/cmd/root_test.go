package cmd

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/cli/pkg/test"
)

func TestCommand_suggest(t *testing.T) {
	p := &test.Params{}
	pipelinerun := Root(p)
	out, err := test.ExecuteCommand(pipelinerun, "pi")
	if err == nil {
		t.Errorf("No errors was defined. Output: %s", out)
	}
	expected := "unknown command \"pi\" for \"tkn\"\n\nDid you mean this?\n\tpipeline\n\tpipelinerun\n"
	if d := cmp.Diff(expected, err.Error()); d != "" {
		t.Errorf("Unexpected output mismatch: %s", d)

	}
}
