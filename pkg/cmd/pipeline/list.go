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

package pipeline

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
	emptyMsg = "No pipelines found"
	header   = "NAME\tAGE\tLAST RUN\tSTARTED\tDURATION\tSTATUS"
	body     = "%s\t%s\t%s\t%s\t%s\t%s\n"
	blank    = "---"
)

func listCommand(p cli.Params) *cobra.Command {
	f := cliopts.NewPrintFlags("list")

	c := &cobra.Command{
		Use:          "list",
		Aliases:      []string{"ls"},
		Short:        "Lists pipelines in a namespace",
		Long:         ``,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			output, err := cmd.LocalFlags().GetString("output")
			if err != nil {
				fmt.Fprint(os.Stderr, "Error: output option not set properly \n")
				return err
			}

			if output != "" {
				return printPipelineListObj(cmd.OutOrStdout(), p, f)
			}
			stream := &cli.Stream{
				Out: cmd.OutOrStdout(),
				Err: cmd.OutOrStderr(),
			}
			return printPipelineDetails(stream, p)
		},
	}
	f.AddFlags(c)

	return c
}

func printPipelineDetails(s *cli.Stream, p cli.Params) error {

	cs, err := p.Clients()
	if err != nil {
		return err
	}

	ps, prs, err := listPipelineDetails(cs.Tekton, p.Namespace())

	if err != nil {
		fmt.Fprintf(s.Err, "Failed to list pipelines from %s namespace\n", p.Namespace())
		return err
	}

	if len(ps.Items) == 0 {
		fmt.Fprintln(s.Err, emptyMsg)
		return nil
	}

	w := tabwriter.NewWriter(s.Out, 0, 5, 3, ' ', tabwriter.TabIndent)
	fmt.Fprintln(w, header)

	for _, pipeline := range ps.Items {
		pr, ok := prs[pipeline.Name]
		if !ok {
			fmt.Fprintf(w, body,
				pipeline.Name,
				formatted.Age(pipeline.CreationTimestamp, p.Time()),
				blank,
				blank,
				blank,
				blank,
			)
			continue
		}

		fmt.Fprintf(w, body,
			pipeline.Name,
			formatted.Age(pipeline.CreationTimestamp, p.Time()),
			pr.Name,
			formatted.Age(*pr.Status.StartTime, p.Time()),
			formatted.Duration(pr.Status.StartTime, pr.Status.CompletionTime),
			formatted.Condition(pr.Status.Conditions[0]),
		)

	}
	return w.Flush()
}

func printPipelineListObj(w io.Writer, p cli.Params, f *cliopts.PrintFlags) error {
	cs, err := p.Clients()
	if err != nil {
		return err
	}

	ps, err := listAllPipelines(cs.Tekton, p.Namespace())
	if err != nil {
		return err
	}

	return printer.PrintObject(w, ps, f)
}

func listAllPipelines(cs versioned.Interface, ns string) (*v1alpha1.PipelineList, error) {
	c := cs.TektonV1alpha1().Pipelines(ns)

	ps, err := c.List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	// NOTE: this is required for -o json|yaml to work properly since
	// tektoncd go client fails to set these; probably a bug
	ps.GetObjectKind().SetGroupVersionKind(
		schema.GroupVersionKind{
			Version: "tekton.dev/v1alpha1",
			Kind:    "PipelineList",
		})
	return ps, nil
}

type pipelineruns map[string]v1alpha1.PipelineRun

func listPipelineDetails(cs versioned.Interface, ns string) (*v1alpha1.PipelineList, pipelineruns, error) {

	ps, err := listAllPipelines(cs, ns)
	if err != nil {
		return nil, nil, err
	}

	if len(ps.Items) == 0 {
		return ps, pipelineruns{}, nil
	}

	runs, err := cs.TektonV1alpha1().PipelineRuns(ns).List(metav1.ListOptions{})
	if err != nil {
		// TODO: may be just the pipeline details can be print
		return nil, nil, err
	}

	latestRuns := pipelineruns{}

	for _, run := range runs.Items {
		pipelineName := run.Spec.PipelineRef.Name
		latest, ok := latestRuns[pipelineName]
		if !ok {
			latestRuns[pipelineName] = run
			continue
		}
		if run.CreationTimestamp.After(latest.CreationTimestamp.Time) {
			latestRuns[pipelineName] = run
		}
	}

	return ps, latestRuns, nil
}
