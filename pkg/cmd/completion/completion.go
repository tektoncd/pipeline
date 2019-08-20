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
		ValidArgs: []string{"bash", "zsh"},
		Example:   eg,
		Annotations: map[string]string{
			"commandType": "utility",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 1 {
				switch args[0] {
				case "bash":
					return cmd.Root().GenBashCompletion(os.Stdout)
				case "zsh":
					return runCompletionZsh(os.Stdout, cmd.Parent())
				}
			}
			return nil
		},
	}
	return cmd
}

// runCompletionZsh generate completion manually, we are not using cobra
// completion since it's not flexible enough for us.
func runCompletionZsh(out io.Writer, tkn *cobra.Command) error {
	if err := tkn.Root().GenZshCompletion(out); err != nil {
		return err
	}

	if _, err := out.Write([]byte(zshCompletion)); err != nil {
		return err
	}
	return nil
}
