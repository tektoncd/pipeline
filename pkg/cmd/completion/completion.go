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
		Use:       "completion SHELL",
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

	zsh_initialization := `
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
