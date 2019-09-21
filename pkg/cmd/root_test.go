// Copyright © 2019 The Tekton Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"strings"
	"testing"

	"github.com/spf13/pflag"
	"github.com/tektoncd/cli/pkg/test"
	tu "github.com/tektoncd/cli/pkg/test"
)

func TestCommand_no_global_flags(t *testing.T) {
	unwantedflag := "OPTION-SHOULD-NOT-BE-HERE"
	pflag.String(unwantedflag, "", "An option that we really don't want to show")

	p := &test.Params{}
	pipelinerun := Root(p)
	out, err := test.ExecuteCommand(pipelinerun)
	if err == nil {
		t.Errorf("test should have command with an error: `subcommand is required` Output: `%s`", out)
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
	tu.AssertOutput(t, expected, err.Error())
}
