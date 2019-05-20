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

package pipelinerun

import (
	"fmt"
	"io"
	"os"
	"text/tabwriter"

	"github.com/jonboulle/clockwork"
	"github.com/spf13/cobra"
	"github.com/tektoncd/cli/pkg/cli"
	"github.com/tektoncd/cli/pkg/formatted"
	"github.com/tektoncd/cli/pkg/printer"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	cliopts "k8s.io/cli-runtime/pkg/genericclioptions"
)

const (
	msgNoPRsFound = "No pipelineruns found."
)

func listCommand(p cli.Params) *cobra.Command {
	f := cliopts.NewPrintFlags("list")
	eg := `
# List all PipelineRuns of Pipeline name 'foo'
tkn pipelinerun list  foo -n bar

# List all pipelineruns in a namespaces 'foo'
tkn pr list -n foo \n",
`

	c := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "Lists pipelineruns in a namespace",
		Example: eg,
		RunE: func(cmd *cobra.Command, args []string) error {
			var pipeline string

			if len(args) > 0 {
				pipeline = args[0]
			}

			prs, err := list(p, pipeline)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to list pipelineruns from %s namespace \n", p.Namespace())
				return err
			}

			output, err := cmd.LocalFlags().GetString("output")
			if err != nil {
				fmt.Fprint(os.Stderr, "Error: output option not set properly \n")
				return err
			}

			if output != "" {
				return printer.PrintObject(cmd.OutOrStdout(), prs, f)
			}

			err = printFormatted(cmd.OutOrStdout(), prs, p.Time())
			if err != nil {
				fmt.Fprint(os.Stderr, "Failed to print Pipelineruns \n")
				return err
			}
			return nil
		},
	}

	f.AddFlags(c)

	return c
}

func list(p cli.Params, pipeline string) (*v1alpha1.PipelineRunList, error) {
	cs, err := p.Clients()
	if err != nil {
		return nil, err
	}

	options := v1.ListOptions{}
	if pipeline != "" {
		options = v1.ListOptions{
			LabelSelector: fmt.Sprintf("tekton.dev/pipeline=%s", pipeline),
		}
	}

	prc := cs.Tekton.TektonV1alpha1().PipelineRuns(p.Namespace())
	prs, err := prc.List(options)
	if err != nil {
		return nil, err
	}

	// NOTE: this is required for -o json|yaml to work properly since
	// tektoncd go client fails to set these; probably a bug
	prs.GetObjectKind().SetGroupVersionKind(
		schema.GroupVersionKind{
			Version: "tekton.dev/v1alpha1",
			Kind:    "PipelineRunList",
		})

	return prs, nil
}

func printFormatted(out io.Writer, prs *v1alpha1.PipelineRunList, c clockwork.Clock) error {
	if len(prs.Items) == 0 {
		fmt.Fprintln(out, msgNoPRsFound)
		return nil
	}

	w := tabwriter.NewWriter(out, 0, 5, 3, ' ', tabwriter.TabIndent)
	fmt.Fprintln(w, "NAME\tSTARTED\tDURATION\tSTATUS\t")
	for _, pr := range prs.Items {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t\n",
			pr.Name,
			formatted.Age(*pr.Status.StartTime, c),
			formatted.Duration(pr.Status.StartTime, pr.Status.CompletionTime),
			formatted.Condition(pr.Status.Conditions[0]),
		)
	}

	return w.Flush()
}
