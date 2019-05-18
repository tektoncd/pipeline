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
	"io"

	"github.com/spf13/cobra"
	"github.com/tektoncd/cli/pkg/cli"
	"github.com/tektoncd/cli/pkg/formatted"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	cliopts "k8s.io/cli-runtime/pkg/genericclioptions"
	"text/tabwriter"
)

const (
	statusHeader   = "STARTED\tDURATION\tSTATUS\n"
	statusBody     = "%s\t%s\t%s\n"
	resourceHeader = "NAME\tRESOURCE REF\n"
	resourceBody   = "%s\t%s\n"
	paramHeader    = "NAME\tVALUE\n"
	paramBody      = "%s\t%s\n"
	taskrunHeader  = "NAME\tTASK NAME\tSTARTED\tDURATION\tSTATUS\n"
	taskrunBody    = "%s\t%s\t%s\t%s\t%s\n"
)

func describeCommand(p cli.Params) *cobra.Command {
	f := cliopts.NewPrintFlags("describe")
	eg := `
# Describe a PipelineRun of name 'foo' in namespace 'bar'
tkn pipelinerun describe foo -n bar

tkn pr desc foo -n bar",
`

	c := &cobra.Command{
		Use:          "describe",
		Aliases:      []string{"desc"},
		Short:        "Describe a pipelinerun in a namespace",
		Example:      eg,
		Args:         cobra.MinimumNArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return printPipelineRunDescription(cmd.OutOrStdout(), args[0], p)
		},
	}

	f.AddFlags(c)

	return c
}

func printPipelineRunDescription(out io.Writer, prname string, p cli.Params) error {
	cs, err := p.Clients()
	if err != nil {
		return fmt.Errorf("Failed to create tekton client\n")
	}

	pr, err := cs.Tekton.TektonV1alpha1().PipelineRuns(p.Namespace()).Get(prname, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("Failed to find pipelinerun %q", prname)
	}

	w := tabwriter.NewWriter(out, 0, 5, 3, ' ', tabwriter.TabIndent)
	printPipelineRunMetadata(w, pr)
	printPipelineRunStatus(w, pr, p)
	printPipelineRunResources(w, pr)
	printPipelineRunParams(w, pr)
	printPipelineRunTaskruns(w, pr, p)
	return w.Flush()
}

func printPipelineRunMetadata(w *tabwriter.Writer, pr *v1alpha1.PipelineRun) {
	fmt.Fprintf(w, "Name:\t%s\n", pr.Name)
	fmt.Fprintf(w, "Namespace:\t%s\n", pr.Namespace)
	fmt.Fprintf(w, "Pipeline Ref:\t%s\n", pr.Spec.PipelineRef.Name)
	fmt.Fprintf(w, "Service Account:\t%s\n", pr.Spec.ServiceAccount)
}

func printPipelineRunStatus(w *tabwriter.Writer, pr *v1alpha1.PipelineRun, p cli.Params) {
	fmt.Fprintln(w, "\nStatus")
	fmt.Fprintf(w, statusHeader)
	fmt.Fprintf(w, statusBody, formatted.Age(*pr.Status.StartTime, p.Time()),
		formatted.Duration(pr.Status.StartTime, pr.Status.CompletionTime),
		formatted.Condition(pr.Status.Conditions[0]))

	if failed, msg := hasFailed(pr); failed && msg != "" {
		fmt.Fprintln(w, "\nMessage")
		fmt.Fprintln(w, msg)
	}
}

func printPipelineRunResources(w *tabwriter.Writer, pr *v1alpha1.PipelineRun) {
	fmt.Fprintln(w, "\nResources")
	if len(pr.Spec.Resources) == 0 {
		fmt.Fprintln(w, "No resources")
		return
	}

	fmt.Fprintf(w, resourceHeader)
	for _, resource := range pr.Spec.Resources {
		fmt.Fprintf(w, resourceBody, resource.Name, resource.ResourceRef.Name)
	}
}

func printPipelineRunParams(w *tabwriter.Writer, pr *v1alpha1.PipelineRun) {
	fmt.Fprintln(w, "\nParams")
	if len(pr.Spec.Params) == 0 {
		fmt.Fprintln(w, "No params")
		return
	}

	fmt.Fprintf(w, paramHeader)
	for _, param := range pr.Spec.Params {
		fmt.Fprintf(w, paramBody, param.Name, param.Value)
	}
}

func printPipelineRunTaskruns(w *tabwriter.Writer, pr *v1alpha1.PipelineRun, p cli.Params) {
	fmt.Fprintln(w, "\nTaskruns")
	if len(pr.Status.TaskRuns) == 0 {
		fmt.Fprintln(w, "No taskruns")
		return
	}

	fmt.Fprintf(w, taskrunHeader)
	for taskrunname, taskrun := range pr.Status.TaskRuns {
		fmt.Fprintf(w, taskrunBody, taskrunname, taskrun.PipelineTaskName, formatted.Age(*taskrun.Status.StartTime, p.Time()),
			formatted.Duration(taskrun.Status.StartTime, taskrun.Status.CompletionTime),
			formatted.Condition(taskrun.Status.Conditions[0]))
	}
}

func hasFailed(pr *v1alpha1.PipelineRun) (bool, string) {
	if pr.Status.Conditions[0].Status == corev1.ConditionFalse {
		return true, pr.Status.Conditions[0].Message
	}
	return false, ""
}
