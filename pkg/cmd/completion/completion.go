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

var (
	// ShellCompletionMap define a map between a flag to a custom shell
	// completion which should be in commonCompletion variable
	ShellCompletionMap = map[string]string{
		"namespace":  "__kubectl_get_namespace",
		"kubeconfig": "_filedir",
	}
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

	commonCompletion = `
# Custom function for Completions
function __tkn_get_object() {
    local type=$1
    local util=$2
    local template begin tkn_out
    template="{{ range .items  }}{{ .metadata.name }} {{ end }}"

    if [[ ${util} == "kubectl" ]];then
        tkn_out=($(kubectl get ${type} -o template --template="${template}" 2>/dev/null))
    elif [[ ${util} == "tkn" ]];then
        tkn_out=($(tkn ${type} ls -o template --template="${template}" 2>/dev/null))
    fi

    if [[ -n ${tkn_out} ]]; then
        [[ -n ${BASH_VERSION} ]] && COMPREPLY=( $( compgen -W "${tkn_out}" -- "$cur" ) )
        [[ -n ${ZSH_VERSION} ]] && compadd ${tkn_out}
    fi
}

function __kubectl_get_namespace() { __tkn_get_object namespace kubectl ;}
function __kubectl_get_serviceaccount() { __tkn_get_object serviceaccount kubectl ;}
function __tkn_get_pipeline() { __tkn_get_object pipeline tkn ;}
function __tkn_get_pipelinerun() { __tkn_get_object pipelinerun tkn ;}
function __tkn_get_task() { __tkn_get_object task tkn ;}
function __tkn_get_taskrun() { __tkn_get_object taskrun tkn ;}
function __tkn_get_pipelineresource() { __tkn_get_object resource tkn ;}
function __tkn_get_clustertasks() { __tkn_get_object clustertasks tkn ;}
`
	bashCompletion = `
function __custom_func() {
	case ${last_command} in
		*_delete|*_start|*_describe|*_logs)
			obj=${last_command/tkn_/};
			# really need to find something better for this
			obj=${obj/_describe/}; obj=${obj/_logs/};obj=${obj/_start/};
			obj=${obj/_delete/};
			__tkn_get_object ${obj} tkn
			return
			;;
		*)
			;;
	esac
}
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
					return runCompletionBash(os.Stdout, cmd.Parent())
				case "zsh":
					return runCompletionZsh(os.Stdout, cmd.Parent())
				}
			}
			return nil
		},
	}
	return cmd
}

// runCompletionBash generate completion with custom functions for bash
func runCompletionBash(out io.Writer, tkn *cobra.Command) error {
	if err := tkn.Root().GenBashCompletion(out); err != nil {
		return err
	}

	if _, err := out.Write([]byte(commonCompletion)); err != nil {
		return err
	}

	if _, err := out.Write([]byte(bashCompletion)); err != nil {
		return err
	}

	return nil
}

// runCompletionZsh generate completion with custom functions for bash
func runCompletionZsh(out io.Writer, tkn *cobra.Command) error {
	if err := tkn.Root().GenZshCompletion(out); err != nil {
		return err
	}

	if _, err := out.Write([]byte(commonCompletion)); err != nil {
		return err
	}
	return nil
}
