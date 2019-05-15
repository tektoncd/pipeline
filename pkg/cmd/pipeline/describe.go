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
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	cliopts "k8s.io/cli-runtime/pkg/genericclioptions"
)

const (
	resHeader  = "NAME\tTYPE\n"
	resBody    = "%s\t%s\n"
	taskHeader = "NAME\tTASKREF\tRUNAFTER\n"
	taskBody   = "%s\t%s\t%v\n"
	runsHeader = "NAME\tSTARTED\tDURATION\tSTATUS\n"
	runsBody   = "%s\t%s\t%s\t%s\n"
)

var pname string

func describeCommand(p cli.Params) *cobra.Command {
	f := cliopts.NewPrintFlags("describe")

	c := &cobra.Command{
		Use:     "describe",
		Aliases: []string{"desc"},
		Short:   "Describes a pipeline in a namespace",
		Long:    ``,
		Args:    cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			pname = args[0]
			return printPipelineDescription(cmd.OutOrStdout(), p)
		},
	}
	f.AddFlags(c)
	return c
}

func printPipelineDescription(out io.Writer, p cli.Params) error {
	cs, err := p.Clients()
	if err != nil {
		return err
	}

	pipeline, err := cs.Tekton.TektonV1alpha1().Pipelines(p.Namespace()).Get(pname, metav1.GetOptions{})
	if err != nil {
		fmt.Fprintln(out, "Error :", err)
		return nil
	}

	w := tabwriter.NewWriter(out, 0, 5, 3, ' ', tabwriter.TabIndent)
	fmt.Fprintf(out, "Name: %s\n", pname)
	printResources(w, pipeline)
	printTasks(w, pipeline)
	printRuns(w, cs, p)

	return w.Flush()
}

func printResources(w *tabwriter.Writer, pipeline *v1alpha1.Pipeline) {
	fmt.Fprintln(w, "\nResources")
	if len(pipeline.Spec.Resources) == 0 {
		fmt.Fprintln(w, "No resources found")
		return
	}
	fmt.Fprintf(w, resHeader)
	for _, val := range pipeline.Spec.Resources {
		fmt.Fprintf(w, resBody,
			val.Name,
			val.Type,
		)

	}
}

func printTasks(w *tabwriter.Writer, pipeline *v1alpha1.Pipeline) {
	fmt.Fprintln(w, "\nTasks")
	if len(pipeline.Spec.Tasks) == 0 {
		fmt.Fprintln(w, "No tasks found")
		return
	}
	fmt.Fprintf(w, taskHeader)
	for _, val := range pipeline.Spec.Tasks {
		fmt.Fprintf(w, taskBody,
			val.Name,
			val.TaskRef.Name,
			val.RunAfter,
		)

	}
}

func printRuns(w *tabwriter.Writer, cs *cli.Clients, p cli.Params) {
	fmt.Fprintln(w, "\nRuns")
	opts := metav1.ListOptions{
		LabelSelector: fmt.Sprintf("tekton.dev/pipeline=%s", pname),
	}

	runs, err := cs.Tekton.TektonV1alpha1().PipelineRuns(p.Namespace()).List(opts)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
	if len(runs.Items) == 0 {
		fmt.Fprintln(w, "No runs found")
		return
	}
	fmt.Fprintf(w, runsHeader)
	for _, run := range runs.Items {
		fmt.Fprintf(w, runsBody,
			run.Name,
			formatted.Age(*run.Status.StartTime, p.Time()),
			formatted.Duration(run.Status.StartTime, run.Status.CompletionTime),
			formatted.Condition(run.Status.Conditions[0]),
		)

	}
}
