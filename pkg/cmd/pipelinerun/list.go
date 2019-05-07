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
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/knative/pkg/apis"
	"github.com/spf13/cobra"
	"github.com/tektoncd/cli/pkg/cli"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	corev1 "k8s.io/api/core/v1"
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
		Short:   "List the pipelineruns",
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
				return printObject(cmd.OutOrStdout(), &prs, f)
			}

			err = printFormatted(cmd.OutOrStdout(), &prs, p.Time())
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

func list(p cli.Params, pipeline string) (v1alpha1.PipelineRunList, error) {
	var empty = v1alpha1.PipelineRunList{}
	cs, err := p.Clientset()
	if err != nil {
		return empty, err
	}

	options := v1.ListOptions{}
	if pipeline != "" {
		options = v1.ListOptions{
			LabelSelector: fmt.Sprintf("tekton.dev/pipeline=%s", pipeline),
		}
	}

	prc := cs.TektonV1alpha1().PipelineRuns(p.Namespace())
	prs, err := prc.List(options)
	if err != nil {
		return empty, err
	}

	// NOTE: this is require for -o json|yaml to work properly since
	// teckon go client fails to set these; probably a bug
	prs.GetObjectKind().SetGroupVersionKind(
		schema.GroupVersionKind{
			Version: "tekton.dev/v1alpha1",
			Kind:    "PipelineRunList",
		})

	return *prs, nil
}

func printObject(out io.Writer, prs *v1alpha1.PipelineRunList, f *cliopts.PrintFlags) error {
	//NOTE: no null checks for prs; caller needs to ensure it is not null
	printer, err := f.ToPrinter()
	if err != nil {
		return err
	}
	return printer.PrintObj(prs, out)
}

func printFormatted(out io.Writer, prs *v1alpha1.PipelineRunList, c clockwork.Clock) error {

	//NOTE: no null checks for prs; caller needs to ensure it is not null
	if len(prs.Items) == 0 {
		fmt.Fprintln(out, msgNoPRsFound)
		return nil
	}

	w := tabwriter.NewWriter(out, 0, 5, 3, ' ', tabwriter.TabIndent)
	defer w.Flush()
	fmt.Fprintln(w, "NAME\tSTATUS\tSTARTED\tDURATION\t")
	for _, pr := range prs.Items {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t\n",
			pr.Name,
			statusForCondition(pr.Status.Conditions[0]),
			age(pr.Status.StartTime, c),
			duration(pr.Status.StartTime, pr.Status.CompletionTime),
		)
	}
	return nil
}

func statusForCondition(c apis.Condition) string {
	if c.Status == corev1.ConditionFalse {
		s := "Failed"
		if c.Reason != "" {
			s = s + "(" + c.Reason + ")"
		}
		return s
	}

	return c.Reason
}

func age(t *v1.Time, c clockwork.Clock) string {
	if t.IsZero() {
		return "---"
	}

	return c.Since(t.Time).Round(time.Second).String()
}

func duration(t1, t2 *v1.Time) string {
	if t1.IsZero() || t2.IsZero() {
		return "---"
	}

	return t2.Time.Sub(t1.Time).String()
}
