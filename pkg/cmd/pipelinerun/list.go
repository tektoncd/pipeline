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

package pipelinerun

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/jonboulle/clockwork"
	"github.com/spf13/cobra"
	"github.com/tektoncd/cli/pkg/cli"
	"github.com/tektoncd/cli/pkg/formatted"
	prhsort "github.com/tektoncd/cli/pkg/helper/pipelinerun/sort"
	"github.com/tektoncd/cli/pkg/printer"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	cliopts "k8s.io/cli-runtime/pkg/genericclioptions"
)

const (
	emptyMsg = "No pipelineruns found"
)

type ListOptions struct {
	Limit int
}

func listCommand(p cli.Params) *cobra.Command {

	opts := &ListOptions{Limit: 0}
	f := cliopts.NewPrintFlags("list")
	eg := `
# List all PipelineRuns of Pipeline name 'foo'
tkn pipelinerun list foo -n bar

# List all pipelineruns in a namespaces 'foo'
tkn pr list -n foo",
`

	c := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "Lists pipelineruns in a namespace",
		Annotations: map[string]string{
			"commandType": "main",
		},
		Example: eg,
		RunE: func(cmd *cobra.Command, args []string) error {
			var pipeline string

			if len(args) > 0 {
				pipeline = args[0]
			}

			if opts.Limit < 0 {
				fmt.Fprintf(os.Stderr, "Limit was %d but must be a positive number\n", opts.Limit)
				return nil
			}

			prs, err := list(p, pipeline, opts.Limit)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to list pipelineruns from %s namespace \n", p.Namespace())
				return err
			}

			output, err := cmd.LocalFlags().GetString("output")
			if err != nil {
				fmt.Fprint(os.Stderr, "Error: output option not set properly \n")
				return err
			}

			if output != "" && prs != nil {
				return printer.PrintObject(cmd.OutOrStdout(), prs, f)
			}
			stream := &cli.Stream{
				Out: cmd.OutOrStdout(),
				Err: cmd.OutOrStderr(),
			}

			if prs != nil {
				err = printFormatted(stream, prs, p.Time())
			}

			if err != nil {
				fmt.Fprint(os.Stderr, "Failed to print Pipelineruns \n")
				return err
			}
			return nil
		},
	}

	f.AddFlags(c)
	c.Flags().IntVarP(&opts.Limit, "limit", "l", 0, "limit pipelineruns listed (default: return all pipelineruns)")

	return c
}

func list(p cli.Params, pipeline string, limit int) (*v1alpha1.PipelineRunList, error) {
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

	prslen := len(prs.Items)

	if prslen != 0 {
		prs.Items = prhsort.SortPipelineRunsByStartTime(prs.Items)
	}

	// If greater than maximum amount of pipelineruns, return all pipelineruns by setting limit to default
	if limit > prslen {
		limit = 0
	}

	// Return all pipelineruns if limit is 0 or is same as prslen
	if limit != 0 && prslen > limit {
		prs.Items = prs.Items[0:limit]
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

func printFormatted(s *cli.Stream, prs *v1alpha1.PipelineRunList, c clockwork.Clock) error {
	if len(prs.Items) == 0 {
		fmt.Fprintln(s.Err, emptyMsg)
		return nil
	}

	w := tabwriter.NewWriter(s.Out, 0, 5, 3, ' ', tabwriter.TabIndent)
	fmt.Fprintln(w, "NAME\tSTARTED\tDURATION\tSTATUS\t")
	for _, pr := range prs.Items {

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t\n",
			pr.Name,
			formatted.Age(pr.Status.StartTime, c),
			formatted.Duration(pr.Status.StartTime, pr.Status.CompletionTime),
			formatted.Condition(pr.Status.Conditions),
		)
	}

	return w.Flush()
}
