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
	"sort"
	"text/tabwriter"
	"text/template"

	"github.com/spf13/cobra"
	"github.com/tektoncd/cli/pkg/cli"
	"github.com/tektoncd/cli/pkg/formatted"
	validate "github.com/tektoncd/cli/pkg/helper/validate"
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

Params
{{- $l := len .Pipeline.Spec.Params }}{{ if eq $l 0 }}
No params
{{- else }}
NAME	TYPE	DEFAULT VALUE
{{- range $i, $p := .Pipeline.Spec.Params }}
{{- if not $p.Default }}
{{ $p.Name }}	{{ $p.Type }}	{{ "" }}
{{- else }}
{{- if eq $p.Type "string" }}
{{ $p.Name }}	{{ $p.Type }}	{{ $p.Default.StringVal }}
{{- else }}
{{ $p.Name }}	{{ $p.Type }}	{{ $p.Default.ArrayVal }}
{{- end }}
{{- end }}
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

Pipelineruns
{{- $rl := len .PipelineRuns.Items }}{{ if eq $rl 0 }}
No pipelineruns
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
		Use:     "describe",
		Aliases: []string{"desc"},
		Short:   "Describes a pipeline in a namespace",
		Annotations: map[string]string{
			"commandType": "main",
		},
		Args:         cobra.MinimumNArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {

			if err := validate.NamespaceExists(p); err != nil {
				return err
			}

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

	if len(pipeline.Spec.Resources) > 0 {
		pipeline.Spec.Resources = sortResourcesByTypeAndName(pipeline.Spec.Resources)
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

// this will sort the Resource by Type and then by Name
func sortResourcesByTypeAndName(pres []v1alpha1.PipelineDeclaredResource) []v1alpha1.PipelineDeclaredResource {
	sort.Slice(pres, func(i, j int) bool {
		if pres[j].Type < pres[i].Type {
			return false
		}

		if pres[j].Type > pres[i].Type {
			return true
		}

		return pres[j].Name > pres[i].Name
	})

	return pres
}
