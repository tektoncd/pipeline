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

package flags

import (
	"testing"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/tektoncd/cli/pkg/cli"
)

func TestFlags_add_shell_completion(t *testing.T) {
	newflag := "newflag"
	shellfunc := "__test_function"
	cmd := cobra.Command{}
	cmd.PersistentFlags().String(newflag, "", "Completion pinpon pinpon 🎤")

	pflag := cmd.PersistentFlags().Lookup(newflag)
	AddShellCompletion(pflag, shellfunc)

	if pflag.Annotations[cobra.BashCompCustom] == nil {
		t.Errorf("annotation should be have been added to the flag")
	}

	if pflag.Annotations[cobra.BashCompCustom][0] != shellfunc {
		t.Errorf("annotation should have been added to the flag")
	}
}

func TestFlags_colouring(t *testing.T) {
	// When running it on CI, our test don't have a tty so this gets disabled
	// automatically, not really sure how can we workaround that :(
	// cmd := &cobra.Command{}
	// cmd.SetArgs([]string{"--nocolour"})
	// _ = InitParams(&cli.TektonParams{}, cmd)
	// assert.False(t, color.NoColor)

	_ = InitParams(&cli.TektonParams{}, &cobra.Command{})
	assert.True(t, color.NoColor)

}
