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

var (
	// BashCompletionFlags this maps between a flag (ie: namespace) to a custom
	// zsh completion
	BashCompletionFlags = map[string]string{
		"namespace":  "__kubectl_get_namespace",
		"kubeconfig": "_filedir",
	}
)

const (
	// BashCompletionFunc the custom bash completion function to complete object.
	// The bash completion mechanism will launch the __custom_func func if it cannot
	// find any completion and pass the object in the ${last_command}
	// variable. Which then are used to get the pipelineruns/taskruns etc..
	BashCompletionFunc = `
__tkn_get_object()
{
	local type=$1
	local template
	template="{{ range .items  }}{{ .metadata.name }} {{ end }}"
	local tkn_out
	if tkn_out=$(tkn ${type} ls -o template --template="${template}" 2>/dev/null); then
		COMPREPLY=( $( compgen -W "${tkn_out}" -- "$cur" ) )
	fi
}

__kubectl_get_object()
{
    local type=$1
    local template
    template="{{ range .items  }}{{ .metadata.name }} {{ end }}"
    local kubectl_out
    if kubectl_out=$(kubectl get -o template --template="${template}" ${type} 2>/dev/null); then
        COMPREPLY=( $( compgen -W "${kubectl_out}" -- "$cur" ) )
    fi
}
__kubectl_get_namespace() { __kubectl_get_object namespace }

__custom_func() {
	case ${last_command} in
		*_describe|*_logs)
			obj=${last_command/tkn_/};
			obj=${obj/_describe/}; obj=${obj/_logs/};
			__tkn_get_object ${obj}
			return
			;;
		*)
			;;
	esac
}
`
)
