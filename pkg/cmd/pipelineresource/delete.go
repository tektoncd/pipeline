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

package pipelineresource

import (
	"bufio"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tektoncd/cli/pkg/cli"
	validateinput "github.com/tektoncd/cli/pkg/helper/validate"
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
# Delete a PipelineResource of name 'foo' in namespace 'bar'
tkn resource delete foo -n bar

tkn res rm foo -n bar
`

	c := &cobra.Command{
		Use:          "delete",
		Aliases:      []string{"rm"},
		Short:        "Delete a pipeline resource in a namespace",
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

			if err := validateinput.NamespaceExists(p); err != nil {
				return err
			}

			if err := checkOptions(opts, s, p, args[0]); err != nil {
				return err
			}

			return deleteResource(s, p, args[0])
		},
	}
	f.AddFlags(c)
	c.Flags().BoolVarP(&opts.forceDelete, "force", "f", false, "Whether to force deletion (default: false)")

	_ = c.MarkZshCompPositionalArgumentCustom(1, "__tkn_get_pipelineresource")
	return c
}

func deleteResource(s *cli.Stream, p cli.Params, preName string) error {
	cs, err := p.Clients()
	if err != nil {
		return fmt.Errorf("failed to create tekton client")
	}

	if err := cs.Tekton.TektonV1alpha1().PipelineResources(p.Namespace()).Delete(preName, &metav1.DeleteOptions{}); err != nil {
		return fmt.Errorf("failed to delete pipelineresource %q: %s", preName, err)
	}

	fmt.Fprintf(s.Out, "PipelineResource deleted: %s\n", preName)
	return nil
}

func checkOptions(opts *deleteOptions, s *cli.Stream, p cli.Params, preName string) error {
	if opts.forceDelete {
		return nil
	}

	fmt.Fprintf(s.Out, "Are you sure you want to delete pipelineresource %q (y/n): ", preName)
	scanner := bufio.NewScanner(s.In)
	for scanner.Scan() {
		t := strings.TrimSpace(scanner.Text())
		if t == "y" {
			break
		} else if t == "n" {
			return fmt.Errorf("Canceled deleting pipelineresource %q", preName)
		}
		fmt.Fprint(s.Out, "Please enter (y/n): ")
	}

	return nil
}
