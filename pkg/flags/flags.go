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

package flags

import (
	"github.com/spf13/cobra"
	"github.com/tektoncd/cli/pkg/cli"
	"github.com/tektoncd/cli/pkg/cmd/completion"
)

const (
	kubeConfig = "kubeconfig"
	namespace  = "namespace"
)

// AddTektonOptions amends command to add flags required to initialise a cli.Param
func AddTektonOptions(cmd *cobra.Command) {
	cmd.PersistentFlags().StringP(
		kubeConfig, "k", "",
		"kubectl config file (default: $HOME/.kube/config)")

	cmd.PersistentFlags().StringP(
		namespace, "n", "",
		"namespace to use (default: from $KUBECONFIG)")

	// Add custom completion for that command as specified in
	// bashCompletionFlags map
	for name, completion := range completion.BashCompletionFlags {
		pflag := cmd.PersistentFlags().Lookup(name)
		if pflag != nil {
			if pflag.Annotations == nil {
				pflag.Annotations = map[string][]string{}
			}
			pflag.Annotations[cobra.BashCompCustom] = append(
				pflag.Annotations[cobra.BashCompCustom],
				completion,
			)
		}
	}
}

// InitParams initialises cli.Params based on flags defined in command
func InitParams(p cli.Params, cmd *cobra.Command) error {
	// NOTE: breaks symmetry with AddTektonOptions as this uses Flags instead of
	// PersistentFlags as it could be the sub command that is trying to access
	// the flags defined by the parent and hence need to use `Flag` instead
	// e.g. `list` accessing kubeconfig defined by `pipeline`
	kcPath, err := cmd.Flags().GetString(kubeConfig)
	if err != nil {
		return err
	}
	p.SetKubeConfigPath(kcPath)

	// ensure that the config is valid by creating a client
	if _, err := p.Clients(); err != nil {
		return err
	}

	ns, err := cmd.Flags().GetString(namespace)
	if err != nil {
		return err
	}
	if ns != "" {
		p.SetNamespace(ns)
	}

	return nil
}
