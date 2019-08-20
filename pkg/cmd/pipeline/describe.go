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
	"text/tabwriter"
	"text/template"

	"github.com/spf13/cobra"
	"github.com/tektoncd/cli/pkg/cli"
	"github.com/tektoncd/cli/pkg/formatted"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	cliopts "k8s.io/cli-runtime/pkg/genericclioptions"
)

const describeTemplate = `Name:	{{ .PipelineName }}

Resources
{{- $rl := len .Pipeline.Spec.Resources }}{{ if eq $rl 0 }}
No resources
{{- else }}
NAME	TYPE
{{- range $i, $r := .Pipeline.Spec.Resources }}
{{$r.Name }}	{{ $r.Type }}
{{- end }}
{{- end }}

Tasks
{{- $tl := len .Pipeline.Spec.Tasks }}{{ if eq $tl 0 }}
No tasks
{{- else }}
NAME	TASKREF	RUNAFTER
{{- range $i, $t := .Pipeline.Spec.Tasks }}
{{ $t.Name }}	{{ $t.TaskRef.Name }}	{{ $t.RunAfter }}
{{- end }}
{{- end }}

Runs
{{- $rl := len .PipelineRuns.Items }}{{ if eq $rl 0 }}
No runs
{{- else }}
NAME	STARTED	DURATION	STATUS
{{- range $i, $pr := .PipelineRuns.Items }}
{{ $pr.Name }}	{{ formatAge $pr.Status.StartTime $.Params.Time }}	{{ formatDuration $pr.Status.StartTime $pr.Status.CompletionTime }}	{{ formatCondition $pr.Status.Conditions }}
{{- end }}
{{- end }}
`

func describeCommand(p cli.Params) *cobra.Command {
	f := cliopts.NewPrintFlags("describe")

	c := &cobra.Command{
		Use:          "describe",
		Aliases:      []string{"desc"},
		Short:        "Describes a pipeline in a namespace",
		Args:         cobra.MinimumNArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return printPipelineDescription(cmd.OutOrStdout(), p, args[0])
		},
	}

	_ = c.MarkZshCompPositionalArgumentCustom(1, "__tkn_get_pipeline")
	f.AddFlags(c)
	return c
}

func printPipelineDescription(out io.Writer, p cli.Params, pname string) error {
	cs, err := p.Clients()
	if err != nil {
		return err
	}

	pipeline, err := cs.Tekton.TektonV1alpha1().Pipelines(p.Namespace()).Get(pname, metav1.GetOptions{})
	if err != nil {
		return err
	}

	opts := metav1.ListOptions{
		LabelSelector: fmt.Sprintf("tekton.dev/pipeline=%s", pname),
	}
	pipelineRuns, err := cs.Tekton.TektonV1alpha1().PipelineRuns(p.Namespace()).List(opts)
	if err != nil {
		return err
	}

	var data = struct {
		Pipeline     *v1alpha1.Pipeline
		PipelineRuns *v1alpha1.PipelineRunList
		PipelineName string
		Params       cli.Params
	}{
		Pipeline:     pipeline,
		PipelineRuns: pipelineRuns,
		PipelineName: pname,
		Params:       p,
	}

	funcMap := template.FuncMap{
		"formatAge":       formatted.Age,
		"formatDuration":  formatted.Duration,
		"formatCondition": formatted.Condition,
	}

	w := tabwriter.NewWriter(out, 0, 5, 3, ' ', tabwriter.TabIndent)
	t := template.Must(template.New("Describe Pipeline").Funcs(funcMap).Parse(describeTemplate))
	err = t.Execute(w, data)
	if err != nil {
		return err
	}

	return w.Flush()
}
