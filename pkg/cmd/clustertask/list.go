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
	"fmt"
	"io"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/tektoncd/cli/pkg/cli"
	"github.com/tektoncd/cli/pkg/formatted"
	"github.com/tektoncd/cli/pkg/printer"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	cliopts "k8s.io/cli-runtime/pkg/genericclioptions"
)

const (
	emptyMsg = "No clustertasks found"
	header   = "NAME\tAGE"
	body     = "%s\t%s\n"
)

func listCommand(p cli.Params) *cobra.Command {
	f := cliopts.NewPrintFlags("list")

	c := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "Lists clustertasks in a namespace",
		Annotations: map[string]string{
			"commandType": "main",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			output, err := cmd.LocalFlags().GetString("output")
			if err != nil {
				fmt.Fprint(os.Stderr, "Error: output option not set properly \n")
				return err
			}

			if output != "" {
				return printClusterTaskListObj(cmd.OutOrStdout(), p, f)
			}
			stream := &cli.Stream{
				Out: cmd.OutOrStdout(),
				Err: cmd.OutOrStderr(),
			}
			return printClusterTaskDetails(stream, p)
		},
	}
	f.AddFlags(c)

	return c
}

func printClusterTaskDetails(s *cli.Stream, p cli.Params) error {

	cs, err := p.Clients()
	if err != nil {
		return err
	}

	clustertasks, err := listAllClusterTasks(cs.Tekton)
	if err != nil {
		fmt.Fprintln(s.Err, emptyMsg)
		return err
	}

	if len(clustertasks.Items) == 0 {
		fmt.Fprintln(s.Err, emptyMsg)
		return nil
	}

	w := tabwriter.NewWriter(s.Out, 0, 5, 3, ' ', tabwriter.TabIndent)
	fmt.Fprintln(w, header)

	for _, clustertask := range clustertasks.Items {
		fmt.Fprintf(w, body,
			clustertask.Name,
			formatted.Age(&clustertask.CreationTimestamp, p.Time()),
		)
	}
	return w.Flush()
}

func printClusterTaskListObj(w io.Writer, p cli.Params, f *cliopts.PrintFlags) error {
	cs, err := p.Clients()
	if err != nil {
		return err
	}

	clustertasks, err := listAllClusterTasks(cs.Tekton)
	if err != nil {
		return err
	}
	return printer.PrintObject(w, clustertasks, f)
}

func listAllClusterTasks(cs versioned.Interface) (*v1alpha1.ClusterTaskList, error) {
	c := cs.TektonV1alpha1().ClusterTasks()

	clustertasks, err := c.List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	// NOTE: this is required for -o json|yaml to work properly since
	// tektoncd go client fails to set these; probably a bug
	clustertasks.GetObjectKind().SetGroupVersionKind(
		schema.GroupVersionKind{
			Version: "tekton.dev/v1alpha1",
			Kind:    "ClusterTaskList",
		})
	return clustertasks, nil
}
