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

package task

import (
	"fmt"
	"sort"
	"text/tabwriter"
	"text/template"

	"github.com/jonboulle/clockwork"
	"github.com/spf13/cobra"
	"github.com/tektoncd/cli/pkg/cli"
	"github.com/tektoncd/cli/pkg/formatted"
	validate "github.com/tektoncd/cli/pkg/helper/validate"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	cliopts "k8s.io/cli-runtime/pkg/genericclioptions"
)

const describeTemplate = `Name:	{{ .Task.Name }}
Namespace:	{{ .Task.Namespace }}

Input Resources
{{- if not .Task.Spec.Inputs }}
No input resources
{{- else }}
{{- if eq (len .Task.Spec.Inputs.Resources) 0 }}
No input resources
{{- else }}
NAME	TYPE
{{- range $ir := .Task.Spec.Inputs.Resources }}
{{ $ir.Name }}	{{ $ir.Type }}
{{- end }}
{{- end }}
{{- end }}

Output Resources
{{- if not .Task.Spec.Outputs }}
No output resources
{{- else }}
{{- if eq (len .Task.Spec.Outputs.Resources) 0 }}
No output resources
{{- else }}
NAME	TYPE
{{- range $or := .Task.Spec.Outputs.Resources }}
{{ $or.Name }}	{{ $or.Type }}
{{- end }}
{{- end }}
{{- end }}

Params
{{- if not .Task.Spec.Inputs }}
No params
{{- else }}
{{- if eq (len .Task.Spec.Inputs.Params) 0 }}
No params
{{- else }}
NAME	TYPE	DEFAULT_VALUE
{{- range $p := .Task.Spec.Inputs.Params }}
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
{{- end }}

Steps
{{- if eq (len .Task.Spec.Steps) 0 }}
No steps
{{- else }}
NAME
{{- range $step := .Task.Spec.Steps }}
{{ $step.Name }}
{{- end }}
{{- end }}

Taskruns
{{- if eq (len .TaskRuns.Items) 0 }}
No taskruns
{{- else }}
NAME	STARTED	DURATION	STATUS
{{ range $tr:=.TaskRuns.Items }}
{{- $tr.Name }}	{{ formatAge $tr.Status.StartTime $.Time}}	{{ formatDuration $tr.Status.StartTime $tr.Status.CompletionTime }}	{{ formatCondition $tr.Status.Conditions }}
{{ end }}
{{- end }}
`

func describeCommand(p cli.Params) *cobra.Command {
	f := cliopts.NewPrintFlags("describe")
	eg := `
# Describe a task of name 'foo' in namespace 'bar'
tkn task describe foo -n bar

tkn t desc foo -n bar
`

	c := &cobra.Command{
		Use:     "describe",
		Aliases: []string{"desc"},
		Short:   "Describes a task in a namespace",
		Example: eg,
		Annotations: map[string]string{
			"commandType": "main",
		},
		Args:         cobra.MinimumNArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			s := &cli.Stream{
				Out: cmd.OutOrStdout(),
				Err: cmd.OutOrStderr(),
			}

			if err := validate.NamespaceExists(p); err != nil {
				return err
			}

			return printTaskDescription(s, p, args[0])
		},
	}

	_ = c.MarkZshCompPositionalArgumentCustom(1, "__tkn_get_task")
	f.AddFlags(c)
	return c
}

func printTaskDescription(s *cli.Stream, p cli.Params, tname string) error {
	cs, err := p.Clients()
	if err != nil {
		return fmt.Errorf("failed to create tekton client")
	}

	task, err := cs.Tekton.TektonV1alpha1().Tasks(p.Namespace()).Get(tname, metav1.GetOptions{})
	if err != nil {
		fmt.Fprintf(s.Err, "failed to get task %s\n", tname)
		return err
	}

	if task.Spec.Inputs != nil {
		task.Spec.Inputs.Resources = sortResourcesByTypeAndName(task.Spec.Inputs.Resources)
	}

	if task.Spec.Outputs != nil {
		task.Spec.Outputs.Resources = sortResourcesByTypeAndName(task.Spec.Outputs.Resources)
	}

	opts := metav1.ListOptions{
		LabelSelector: fmt.Sprintf("tekton.dev/task=%s", tname),
	}
	taskRuns, err := cs.Tekton.TektonV1alpha1().TaskRuns(p.Namespace()).List(opts)
	if err != nil {
		fmt.Fprintf(s.Err, "failed to get taskruns for task %s \n", tname)
		return err
	}

	var data = struct {
		Task     *v1alpha1.Task
		TaskRuns *v1alpha1.TaskRunList
		Time     clockwork.Clock
	}{
		Task:     task,
		TaskRuns: taskRuns,
		Time:     p.Time(),
	}

	funcMap := template.FuncMap{
		"formatAge":       formatted.Age,
		"formatDuration":  formatted.Duration,
		"formatCondition": formatted.Condition,
	}

	w := tabwriter.NewWriter(s.Out, 0, 5, 3, ' ', tabwriter.TabIndent)
	t := template.Must(template.New("Describe Task").Funcs(funcMap).Parse(describeTemplate))
	err = t.Execute(w, data)
	if err != nil {
		fmt.Fprintf(s.Err, "Failed to execute template \n")
		return err
	}
	return nil
}

// this will sort the Task Resource by Type and then by Name
func sortResourcesByTypeAndName(tres []v1alpha1.TaskResource) []v1alpha1.TaskResource {
	sort.Slice(tres, func(i, j int) bool {
		if tres[j].Type < tres[i].Type {
			return false
		}

		if tres[j].Type > tres[i].Type {
			return true
		}

		return tres[j].Name > tres[i].Name
	})

	return tres
}
