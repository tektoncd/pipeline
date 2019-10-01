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

package condition

import (
	"fmt"
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
	emptyMsg = "No conditions found"
	header   = "NAME\tAGE"
	body     = "%s\t%s\n"
)

func listCommand(p cli.Params) *cobra.Command {
	f := cliopts.NewPrintFlags("list")

	c := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "Lists conditions in a namespace",
		Annotations: map[string]string{
			"commandType": "main",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			stream := &cli.Stream{
				Out: cmd.OutOrStdout(),
				Err: cmd.OutOrStderr(),
			}

			output, err := cmd.LocalFlags().GetString("output")
			if err != nil {
				fmt.Fprint(stream.Err, "Error: output option not set properly \n")
				return err
			}

			if output != "" {
				return printConditionListObj(stream, p, f)
			}
			return printConditionDetails(stream, p)
		},
	}
	f.AddFlags(c)

	return c
}

func printConditionDetails(s *cli.Stream, p cli.Params) error {

	cs, err := p.Clients()
	if err != nil {
		return err
	}

	conditions, err := listAllConditions(cs.Tekton, p.Namespace())
	if err != nil {
		fmt.Fprintf(s.Err, "Failed to list conditions from %s namespace \n", p.Namespace())
		return err
	}

	if len(conditions.Items) == 0 {
		fmt.Fprintln(s.Err, emptyMsg)
		return nil
	}

	w := tabwriter.NewWriter(s.Out, 0, 5, 3, ' ', tabwriter.TabIndent)
	fmt.Fprintln(w, header)

	for _, condition := range conditions.Items {
		fmt.Fprintf(w, body,
			condition.Name,
			formatted.Age(&condition.CreationTimestamp, p.Time()),
		)
	}
	return w.Flush()
}

func printConditionListObj(s *cli.Stream, p cli.Params, f *cliopts.PrintFlags) error {
	cs, err := p.Clients()
	if err != nil {
		return err
	}

	conditions, err := listAllConditions(cs.Tekton, p.Namespace())
	if err != nil {
		fmt.Fprintf(s.Err, "Failed to list conditions from %s namespace \n", p.Namespace())
		return err
	}
	return printer.PrintObject(s.Out, conditions, f)
}

func listAllConditions(cs versioned.Interface, ns string) (*v1alpha1.ConditionList, error) {
	c := cs.TektonV1alpha1().Conditions(ns)

	conditions, err := c.List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	// NOTE: this is required for -o json|yaml to work properly since
	// tektoncd go client fails to set these; probably a bug
	conditions.GetObjectKind().SetGroupVersionKind(
		schema.GroupVersionKind{
			Version: "tekton.dev/v1alpha1",
			Kind:    "ConditionList",
		})
	return conditions, nil
}
