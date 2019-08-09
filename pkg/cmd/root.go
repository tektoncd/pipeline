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
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/tektoncd/cli/pkg/cli"
	"github.com/tektoncd/cli/pkg/cmd/clustertask"
	"github.com/tektoncd/cli/pkg/cmd/completion"
	"github.com/tektoncd/cli/pkg/cmd/pipeline"
	"github.com/tektoncd/cli/pkg/cmd/pipelineresource"
	"github.com/tektoncd/cli/pkg/cmd/pipelinerun"
	"github.com/tektoncd/cli/pkg/cmd/task"
	"github.com/tektoncd/cli/pkg/cmd/taskrun"
	"github.com/tektoncd/cli/pkg/cmd/version"
)

const usageTemplate = `Usage:{{if .Runnable}}
{{.UseLine}}{{end}}{{if .HasAvailableSubCommands}}
  {{.CommandPath}} [command]{{end}}{{if gt (len .Aliases) 0}}

Aliases:
  {{.NameAndAliases}}{{end}}{{if .HasExample}}

Examples:
  {{.Example}}{{end}}{{if .HasAvailableSubCommands}}

Available Commands:{{range .Commands}}{{if (eq .Annotations.commandType "main")}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}

Other Commands:{{range .Commands}}{{if (eq .Annotations.commandType "utility")}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}

Flags:
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasAvailableInheritedFlags}}

Global Flags:
{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasHelpSubCommands}}

Additional help topics:{{range .Commands}}{{if .IsAdditionalHelpTopicCommand}}
{{rpad .CommandPath .CommandPathPadding}} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableSubCommands}}

Use "{{.CommandPath}} [command] --help" for more information about a command.{{end}}
`

func Root(p cli.Params) *cobra.Command {
	// Reset CommandLine so we don't get the flags from the libraries, i.e:
	// azure library adding --azure-container-registry-config
	pflag.CommandLine = pflag.NewFlagSet(os.Args[0], pflag.ExitOnError)

	var cmd = &cobra.Command{
		Use:                    "tkn",
		Short:                  "CLI for tekton pipelines",
		Long:                   ``,
		BashCompletionFunction: completion.BashCompletionFunc,
	}
	cmd.SetUsageTemplate(usageTemplate)

	cmd.AddCommand(
		completion.Command(),
		pipeline.Command(p),
		pipelinerun.Command(p),
		task.Command(p),
		taskrun.Command(p),
		pipelineresource.Command(p),
		clustertask.Command(p),
		version.Command(),
	)

	return cmd
}
