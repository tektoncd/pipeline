// Copyright © 2019 The tektoncd Authors.
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
	"time"

	"github.com/spf13/cobra"
	"github.com/tektoncd/cli/pkg/cli"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	cliopts "k8s.io/cli-runtime/pkg/genericclioptions"
)

var (
	since = time.Since
)

const (
	msgNoPRsFound = "No pipelineruns found."
)

func listCMD(p cli.Params) *cobra.Command {
	f := cliopts.NewPrintFlags("list")

	c := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List the pipelineruns",
		Example: "  # List all PipelineRuns of Pipeline name 'foo' \n" +
				 "  tkn pipelinerun list  foo -n baar \n\n" +
				 "  # List all pipelineruns in a namespaces 'foo' \n" +
				 "  tkn pr list -n foo \n",
		RunE: func(cmd *cobra.Command, args []string) error {
			var pipeline string

			if len(args) > 0 {
				pipeline = args[0]
			}

			prs, err := list(p, pipeline)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to list pipelineruns from %s namespace\n", p.Namespace())
				return err
			}

			err = print(prs, cmd, f)
			if err != nil {
				fmt.Fprint(os.Stderr, "Failed to print Pipelineruns\n")
				return err
			}

			return nil
		},
	}
	f.AddFlags(c)

	return c
}

func list(p cli.Params, pipeline string) (*v1alpha1.PipelineRunList, error) {
	cs, err := p.Clientset()
	if err != nil {
		return nil, err
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
		return nil, err
	}
	
	// Fix for -o yaml,json flag
	prs.GetObjectKind().
		SetGroupVersionKind(schema.GroupVersionKind{
			Version: "tekton.dev/v1alpha1",
			Kind:    "PipelineRunList",
		})

	return prs, nil
}

func print(prs *v1alpha1.PipelineRunList, cmd *cobra.Command, f *cliopts.PrintFlags) error {
	if len(prs.Items) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), msgNoPRsFound)
		return nil
	}

	flags := cmd.LocalFlags() 
	format, err := flags.GetString("output")
	if err != nil {
		return err
	}

	if format != "" {
		printer, err := f.ToPrinter()
		if err != nil {
			return err
		}
		return printer.PrintObj(prs, cmd.OutOrStdout())
	}

	writer := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 5, 3, ' ', tabwriter.TabIndent)
	fmt.Fprintln(writer, "NAME\tSTATUS\tSTARTED\tDURATION\t")
	for _, pr := range prs.Items {
		fmt.Fprintf(writer, "%s\t%s\t%s\t%s\t\n", pr.Name, pr.Status.Conditions[0].Reason, 
		age(pr.Status.StartTime),
		timeBetween(pr.Status.StartTime, pr.Status.CompletionTime))
	}

	return writer.Flush()
}

func age(t *v1.Time) string {
	if t.IsZero() {
		return "---"
	}

	//TODO: confirm print format and trim seconds
	return since(t.Time).String()
}

func timeBetween(t1, t2 *v1.Time) string {
	if t1.IsZero() || t2.IsZero() {
		return "---"
	}

	return t2.Time.Sub(t1.Time).String()
}