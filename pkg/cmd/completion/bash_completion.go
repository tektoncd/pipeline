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
		"namespace":  "__kubectl_get_object namespace",
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
	zsh_initialization = `
__tkn_bash_source() {
	alias shopt=':'
	alias _expand=_bash_expand
	alias _complete=_bash_comp
	emulate -L sh
	setopt kshglob noshglob braceexpand

	source "$@"
}

__tkn_type() {
	# -t is not supported by zsh
	if [ "$1" == "-t" ]; then
		shift

		# fake Bash 4 to disable "complete -o nospace". Instead
		# "compopt +-o nospace" is used in the code to toggle trailing
		# spaces. We don't support that, but leave trailing spaces on
		# all the time
		if [ "$1" = "__tkn_compopt" ]; then
			echo builtin
			return 0
		fi
	fi
	type "$@"
}

__tkn_compgen() {
	local completions w
	completions=( $(compgen "$@") ) || return $?

	# filter by given word as prefix
	while [[ "$1" = -* && "$1" != -- ]]; do
		shift
		shift
	done
	if [[ "$1" == -- ]]; then
		shift
	fi
	for w in "${completions[@]}"; do
		if [[ "${w}" = "$1"* ]]; then
			echo "${w}"
		fi
	done
}

__tkn_compopt() {
	true # don't do anything. Not supported by bashcompinit in zsh
}

__tkn_ltrim_colon_completions()
{
	if [[ "$1" == *:* && "$COMP_WORDBREAKS" == *:* ]]; then
		# Remove colon-word prefix from COMPREPLY items
		local colon_word=${1%${1##*:}}
		local i=${#COMPREPLY[*]}
		while [[ $((--i)) -ge 0 ]]; do
			COMPREPLY[$i]=${COMPREPLY[$i]#"$colon_word"}
		done
	fi
}

__tkn_get_comp_words_by_ref() {
	cur="${COMP_WORDS[COMP_CWORD]}"
	prev="${COMP_WORDS[${COMP_CWORD}-1]}"
	words=("${COMP_WORDS[@]}")
	cword=("${COMP_CWORD[@]}")
}

__tkn_filedir() {
	local RET OLD_IFS w qw

	if [[ "$1" = \~* ]]; then
		# somehow does not work. Maybe, zsh does not call this at all
		eval echo "$1"
		return 0
	fi

	OLD_IFS="$IFS"
	IFS=$'\n'
	if [ "$1" = "-d" ]; then
		shift
		RET=( $(compgen -d) )
	else
		RET=( $(compgen -f) )
	fi
	IFS="$OLD_IFS"

	IFS=","

	for w in ${RET[@]}; do
		if [[ ! "${w}" = "${cur}"* ]]; then
			continue
		fi
		if eval "[[ \"\${w}\" = *.$1 || -d \"\${w}\" ]]"; then
			qw="$(__tkn_quote "${w}")"
			if [ -d "${w}" ]; then
				COMPREPLY+=("${qw}/")
			else
				COMPREPLY+=("${qw}")
			fi
		fi
	done
}

__tkn_quote() {
	if [[ $1 == \'* || $1 == \"* ]]; then
		# Leave out first character
		printf %q "${1:1}"
	else
	printf %q "$1"
	fi
}

autoload -U +X bashcompinit && bashcompinit

# use word boundary patterns for BSD or GNU sed
LWORD='[[:<:]]'
RWORD='[[:>:]]'
if sed --help 2>&1 | grep -q GNU; then
	LWORD='\<'
	RWORD='\>'
fi

__tkn_convert_bash_to_zsh() {
	sed \
	-e 's/declare -F/whence -w/' \
	-e 's/_get_comp_words_by_ref "\$@"/_get_comp_words_by_ref "\$*"/' \
	-e 's/local \([a-zA-Z0-9_]*\)=/local \1; \1=/' \
	-e 's/flags+=("\(--.*\)=")/flags+=("\1"); two_word_flags+=("\1")/' \
	-e 's/must_have_one_flag+=("\(--.*\)=")/must_have_one_flag+=("\1")/' \
	-e "s/${LWORD}_filedir${RWORD}/__tkn_filedir/g" \
	-e "s/${LWORD}_get_comp_words_by_ref${RWORD}/__tkn_get_comp_words_by_ref/g" \
	-e "s/${LWORD}__ltrim_colon_completions${RWORD}/__tkn_ltrim_colon_completions/g" \
	-e "s/${LWORD}compgen${RWORD}/__tkn_compgen/g" \
	-e "s/${LWORD}compopt${RWORD}/__tkn_compopt/g" \
	-e "s/${LWORD}declare${RWORD}/builtin declare/g" \
	-e "s/\\\$(type${RWORD}/\$(__tkn_type/g" \
	<<'BASH_COMPLETION_EOF'
`
)
