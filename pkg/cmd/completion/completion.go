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

package completion

import (
	"bytes"
	"io"
	"os"

	"github.com/spf13/cobra"
)

const (
	desc = `
This command prints shell completion code which must be evaluated to provide
interactive completion

Supported Shells:
	- bash
	- zsh
`
	eg = `
  # generate completion code for bash
  source <(tkn completion bash)

  # generate completion code for zsh
  source <(tkn completion zsh)
`
)

func Command() *cobra.Command {
	var cmd = &cobra.Command{
		Use:       "completion [SHELL]",
		Short:     "Prints shell completion scripts",
		Long:      desc,
		Args:      exactValidArgs(1),
		ValidArgs: []string{"bash", "zsh"},
		Example:   eg,
		RunE: func(cmd *cobra.Command, args []string) error {
			switch args[0] {
			case "bash":
				return cmd.Root().GenBashCompletion(os.Stdout)
			case "zsh":
				return runCompletionZsh(os.Stdout, cmd.Parent())
			}
			return nil
		},
	}
	return cmd
}

// runCompletionZsh generate the zsh completion the same way kubectl is doing it
// https://git.io/fjRRc
// We are not using the builtin zsh completion that comes from cobra but instead doing it
// via bashcompinit and use the GenBashCompletion then
// This allows us the user to simply do a `source <(tkn completion zsh)` to get
// zsh completion
func runCompletionZsh(out io.Writer, tkn *cobra.Command) error {
	zsh_head := "#compdef tkn\n"

	if _, err := out.Write([]byte(zsh_head)); err != nil {
		return err
	}

	if _, err := out.Write([]byte(zsh_initialization)); err != nil {
		return err
	}

	buf := new(bytes.Buffer)
	if err := tkn.GenBashCompletion(buf); err != nil {
		return err
	}
	if _, err := out.Write(buf.Bytes()); err != nil {
		return err
	}

	zsh_tail := `
BASH_COMPLETION_EOF
}

__tkn_bash_source <(__tkn_convert_bash_to_zsh)
_complete tkn 2>/dev/null
`
	if _, err := out.Write([]byte(zsh_tail)); err != nil {
		return err
	}
	return nil
}

// TODO: replace with cobra.ExactValidArgs(n int); may be in the next release 0.0.4
// see: https://github.com/spf13/cobra/blob/67fc4837d267bc9bfd6e47f77783fcc3dffc68de/args.go#L84
func exactValidArgs(n int) cobra.PositionalArgs {
	nArgs := cobra.ExactArgs(n)

	return func(cmd *cobra.Command, args []string) error {
		if err := nArgs(cmd, args); err != nil {
			return err
		}
		return cobra.OnlyValidArgs(cmd, args)
	}
}
