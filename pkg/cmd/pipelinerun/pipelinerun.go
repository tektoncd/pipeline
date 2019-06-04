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

package pipelinerun

import (
	"github.com/spf13/cobra"
	"github.com/tektoncd/cli/pkg/cli"
	"github.com/tektoncd/cli/pkg/flags"
)

//Command instantiates the pipelinerun command
func Command(p cli.Params) *cobra.Command {
	c := &cobra.Command{
		Use:     "pipelinerun",
		Aliases: []string{"pr", "pipelineruns"},
		Short:   "Manage pipelineruns",
		Args:    cobra.NoArgs,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return flags.InitParams(p, cmd)
		},
	}

	flags.AddTektonOptions(c)
	c.AddCommand(listCommand(p))
	c.AddCommand(logCommand(p))
	c.AddCommand(describeCommand(p))

	return c
}
