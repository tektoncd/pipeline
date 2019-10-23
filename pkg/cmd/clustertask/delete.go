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

package clustertask

import (
	"bufio"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tektoncd/cli/pkg/cli"
	validate "github.com/tektoncd/cli/pkg/helper/validate"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	cliopts "k8s.io/cli-runtime/pkg/genericclioptions"
)

type deleteOptions struct {
	forceDelete bool
}

func deleteCommand(p cli.Params) *cobra.Command {
	opts := &deleteOptions{forceDelete: false}
	f := cliopts.NewPrintFlags("delete")
	eg := `
# Delete a ClusterTask of name 'foo'
tkn clustertask delete foo

tkn ct rm foo
`

	c := &cobra.Command{
		Use:          "delete",
		Aliases:      []string{"rm"},
		Short:        "Delete a clustertask resource in a cluster",
		Example:      eg,
		Args:         cobra.MinimumNArgs(1),
		SilenceUsage: true,
		Annotations: map[string]string{
			"commandType": "main",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			s := &cli.Stream{
				In:  cmd.InOrStdin(),
				Out: cmd.OutOrStdout(),
				Err: cmd.OutOrStderr(),
			}

			if err := validate.NamespaceExists(p); err != nil {
				return err
			}

			if err := checkOptions(opts, s, p, args[0]); err != nil {
				return err
			}

			return deleteClusterTask(s, p, args[0])
		},
	}
	f.AddFlags(c)
	c.Flags().BoolVarP(&opts.forceDelete, "force", "f", false, "Whether to force deletion (default: false)")

	_ = c.MarkZshCompPositionalArgumentCustom(1, "__tkn_get_clustertasks")
	return c
}

func deleteClusterTask(s *cli.Stream, p cli.Params, tName string) error {
	cs, err := p.Clients()
	if err != nil {
		return fmt.Errorf("Failed to create tekton client")
	}

	if err := cs.Tekton.TektonV1alpha1().ClusterTasks().Delete(tName, &metav1.DeleteOptions{}); err != nil {
		return fmt.Errorf("Failed to delete clustertask %q: %s", tName, err)
	}

	fmt.Fprintf(s.Out, "ClusterTask deleted: %s\n", tName)
	return nil
}

func checkOptions(opts *deleteOptions, s *cli.Stream, p cli.Params, tName string) error {
	if opts.forceDelete {
		return nil
	}

	fmt.Fprintf(s.Out, "Are you sure you want to delete clustertask %q (y/n): ", tName)
	scanner := bufio.NewScanner(s.In)
	for scanner.Scan() {
		t := strings.TrimSpace(scanner.Text())
		if t == "y" {
			break
		} else if t == "n" {
			return fmt.Errorf("Canceled deleting clustertask %q", tName)
		}
		fmt.Fprint(s.Out, "Please enter (y/n): ")
	}

	return nil
}
