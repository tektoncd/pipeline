// Copyright © 2019 The Knative Authors.
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
	"os"

	"github.com/spf13/cobra"
)

var completionCmd = &cobra.Command{
	Use:   "completion SHELL",
	Short: "Prints shell completion scripts",
	Long: `
This command prints shell completion code which must be evaluated to provide interactive completion

Supported Shells:
  - bash
`,
	Args:      exactValidArgs(1),
	ValidArgs: []string{"bash"},
	Example: `
  # generate completion code for bash
  tkn completion bash > bash_completion.sh
  source bash_completion.sh
	`,
	RunE: func(cmd *cobra.Command, args []string) error {

		switch args[0] {
		case "bash":
			return rootCmd.GenBashCompletion(os.Stdout)
			// TODO add zsh completion
			// case "zsh":
			// return rootCmd.GenZshCompletion(os.Stdout)
			// NOTE: cobra v0.0.3 zsh completion seems fail on zsh 5.5.1
		}
		return nil
	},
}

func exactValidArgs(n int) cobra.PositionalArgs {
	nArgs := cobra.ExactArgs(n)

	return func(cmd *cobra.Command, args []string) error {
		if err := nArgs(cmd, args); err != nil {
			return err
		}
		return cobra.OnlyValidArgs(cmd, args)
	}
}

func init() {
	rootCmd.AddCommand(completionCmd)
}
