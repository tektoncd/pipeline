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

package task

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
	deleteAll   bool
}

func deleteCommand(p cli.Params) *cobra.Command {
	opts := &deleteOptions{
		forceDelete: false,
		deleteAll:   false,
	}
	f := cliopts.NewPrintFlags("delete")
	eg := `
# Delete a Task of name 'foo' in namespace 'bar'
tkn task delete foo -n bar

tkn t rm foo -n bar
`

	c := &cobra.Command{
		Use:          "delete",
		Aliases:      []string{"rm"},
		Short:        "Delete a task resource in a namespace",
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

			return deleteTask(opts, s, p, args[0])
		},
	}
	f.AddFlags(c)
	c.Flags().BoolVarP(&opts.forceDelete, "force", "f", false, "Whether to force deletion (default: false)")
	c.Flags().BoolVarP(&opts.deleteAll, "all", "a", false, "Whether to delete related resources (taskruns) (default: false)")

	_ = c.MarkZshCompPositionalArgumentCustom(1, "__tkn_get_task")
	return c
}

func deleteTask(opts *deleteOptions, s *cli.Stream, p cli.Params, tName string) error {
	cs, err := p.Clients()
	if err != nil {
		return fmt.Errorf("failed to create tekton client")
	}

	if err := cs.Tekton.TektonV1alpha1().Tasks(p.Namespace()).Delete(tName, &metav1.DeleteOptions{}); err != nil {
		return fmt.Errorf("failed to delete task %q: %s", tName, err)
	}
	fmt.Fprintf(s.Out, "Task deleted: %s\n", tName)

	if !opts.deleteAll {
		return nil
	}

	lOpts := metav1.ListOptions{
		LabelSelector: fmt.Sprintf("tekton.dev/task=%s", tName),
	}

	taskRuns, err := cs.Tekton.TektonV1alpha1().TaskRuns(p.Namespace()).List(lOpts)
	if err != nil {
		return err
	}

	for _, tr := range taskRuns.Items {
		if err := cs.Tekton.TektonV1alpha1().TaskRuns(p.Namespace()).Delete(tr.Name, &metav1.DeleteOptions{}); err != nil {
			return fmt.Errorf("failed to delete taskrun %q: %s", tr.Name, err)
		}

		fmt.Fprintf(s.Out, "TaskRun deleted: %s\n", tr.Name)
	}

	return nil
}

func checkOptions(opts *deleteOptions, s *cli.Stream, p cli.Params, tName string) error {
	if opts.forceDelete {
		return nil
	}

	if opts.deleteAll {
		fmt.Fprintf(s.Out, "Are you sure you want to delete task and related resources (taskruns) %q (y/n): ", tName)
	} else {
		fmt.Fprintf(s.Out, "Are you sure you want to delete task %q (y/n): ", tName)
	}

	scanner := bufio.NewScanner(s.In)
	for scanner.Scan() {
		t := strings.TrimSpace(scanner.Text())
		if t == "y" {
			break
		} else if t == "n" {
			return fmt.Errorf("canceled deleting task %q", tName)
		}
		fmt.Fprint(s.Out, "Please enter (y/n): ")
	}

	return nil
}
