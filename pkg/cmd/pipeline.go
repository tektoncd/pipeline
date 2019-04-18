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

var (
	namespace string
)

// pipelinesCmd represents the pipelines command
var pipelinesCmd = &cobra.Command{
	Use:     "pipelines",
	Aliases: []string{"p", "pipeline"},
	Short:   "Manage pipelines",
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			cmd.Help()
			os.Exit(0)
		}
	},
}

func init() {
	rootCmd.AddCommand(pipelinesCmd)

	pipelinesCmd.PersistentFlags().StringVarP(
		&namespace,
		"namespace", "n",
		"",
		"namespace to use (mandatory)")
	pipelinesCmd.MarkPersistentFlagRequired("namespace")
}
