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
	"sort"

	"text/tabwriter"
	"text/template"

	"github.com/spf13/cobra"
	"github.com/tektoncd/cli/pkg/cli"
	"github.com/tektoncd/cli/pkg/formatted"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	cliopts "k8s.io/cli-runtime/pkg/genericclioptions"
)

const templ = `Name:	{{ .PipelineRun.Name }}
Namespace:	{{ .PipelineRun.Namespace }}
Pipeline Ref:	{{ .PipelineRun.Spec.PipelineRef.Name }}
{{- if ne .PipelineRun.Spec.ServiceAccount "" }}
Service Account:	{{ .PipelineRun.Spec.ServiceAccount }}
{{- end }}

Status
STARTED	DURATION	STATUS
{{ formatAge .PipelineRun.Status.StartTime  .Params.Time }}	{{ formatDuration .PipelineRun.Status.StartTime .PipelineRun.Status.CompletionTime }}	{{ formatCondition .PipelineRun.Status.Conditions }}
{{- $msg := hasFailed .PipelineRun -}}
{{-  if ne $msg "" }}

Message
{{ $msg }}
{{- end }}

Resources
{{- $l := len .PipelineRun.Spec.Resources }}{{ if eq $l 0 }}
No resources
{{- else }}
NAME	RESOURCE REF
{{- range $i, $r := .PipelineRun.Spec.Resources }}
{{$r.Name }}	{{ $r.ResourceRef.Name }}
{{- end }}
{{- end }}

Params
{{- $l := len .PipelineRun.Spec.Params }}{{ if eq $l 0 }}
No params
{{- else }}
NAME	VALUE
{{- range $i, $p := .PipelineRun.Spec.Params }}
{{ $p.Name }}	{{ $p.Value }}
{{- end }}
{{- end }}

Taskruns
{{- $l := len .TaskRunList }}{{ if eq $l 0 }}
No taskruns
{{- else }}
NAME	TASK NAME	STARTED	DURATION	STATUS
{{- range $taskrun := .TaskRunList }}
{{ $taskrun.TaskRunName }}	{{ $taskrun.PipelineTaskName }}	{{ formatAge $taskrun.Status.StartTime $.Params.Time }}	{{ formatDuration $taskrun.Status.StartTime $taskrun.Status.CompletionTime }}	{{ formatCondition $taskrun.Status.Conditions }}
{{- end }}
{{- end }}
`

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
			s := &cli.Stream{
				Out: cmd.OutOrStdout(),
				Err: cmd.OutOrStderr(),
			}
			return printPipelineRunDescription(s, args[0], p)
		},
	}

	f.AddFlags(c)

	return c
}

func printPipelineRunDescription(s *cli.Stream, prName string, p cli.Params) error {
	cs, err := p.Clients()
	if err != nil {
		return fmt.Errorf("Failed to create tekton client")
	}

	pr, err := cs.Tekton.TektonV1alpha1().PipelineRuns(p.Namespace()).Get(prName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("Failed to find pipelinerun %q", prName)
	}

	var trl taskrunList

	if len(pr.Status.TaskRuns) != 0 {
		trl = newTaskrunListFromMap(pr.Status.TaskRuns)
		sort.Sort(trl)
	}

	var data = struct {
		PipelineRun *v1alpha1.PipelineRun
		Params      cli.Params
		TaskrunList taskrunList
	}{
		PipelineRun: pr,
		Params:      p,
		TaskrunList: trl,
	}

	funcMap := template.FuncMap{
		"formatAge":       formatted.Age,
		"formatDuration":  formatted.Duration,
		"formatCondition": formatted.Condition,
		"hasFailed":       hasFailed,
	}

	w := tabwriter.NewWriter(s.Out, 0, 5, 3, ' ', tabwriter.TabIndent)
	t := template.Must(template.New("Describe Pipelinerun").Funcs(funcMap).Parse(templ))

	err = t.Execute(w, data)
	if err != nil {
		fmt.Fprintf(s.Err, "Failed to execute template")
		return err
	}
	return w.Flush()
}

func hasFailed(pr *v1alpha1.PipelineRun) string {
	if len(pr.Status.Conditions) == 0 {
		return ""
	}

	if pr.Status.Conditions[0].Status == corev1.ConditionFalse {
		return pr.Status.Conditions[0].Message
	}
	return ""
}

type taskrunList []struct {
	TaskrunName string
	*v1alpha1.PipelineRunTaskRunStatus
}

func newTaskrunListFromMap(statusMap map[string]*v1alpha1.PipelineRunTaskRunStatus) taskrunList {

	var trl taskrunList

	for taskrunName, taskrunStatus := range statusMap {
		trl = append(trl, struct {
			TaskrunName string
			*v1alpha1.PipelineRunTaskRunStatus
		}{
			taskrunName,
			taskrunStatus,
		})
	}

	return trl
}

func (s taskrunList) Len() int      { return len(s) }
func (s taskrunList) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
func (s taskrunList) Less(i, j int) bool {
	return s[j].Status.StartTime.Before(s[i].Status.StartTime)
}
