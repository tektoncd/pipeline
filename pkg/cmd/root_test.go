package cmd

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/spf13/pflag"
	"github.com/tektoncd/cli/pkg/test"
)

func TestCommand_no_global_flags(t *testing.T) {
	unwantedflag := "OPTION-SHOULD-NOT-BE-HERE"
	pflag.String(unwantedflag, "", "An option that we really don't want to show")

	p := &test.Params{}
	pipelinerun := Root(p)
	out, err := test.ExecuteCommand(pipelinerun)
	if err != nil {
		t.Errorf("An error has occured. Output: %s", out)
	}

	if strings.Contains(out, unwantedflag) {
		t.Errorf("The Flag: %s, should not have been added to the global flags", unwantedflag)
	}
}

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
