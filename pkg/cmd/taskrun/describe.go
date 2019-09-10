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

package taskrun

import (
	"fmt"

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

const templ = `Name:	{{ .TaskRun.Name }}
Namespace:	{{ .TaskRun.Namespace }}
Task Ref:	{{ .TaskRun.Spec.TaskRef.Name }}
{{- if ne .TaskRun.Spec.ServiceAccount "" }}
Service Account:	{{ .TaskRun.Spec.ServiceAccount }}
{{- end }}

Status
STARTED 	DURATION 	STATUS
{{ formatAge .TaskRun.Status.StartTime  .Params.Time }}	{{ formatDuration .TaskRun.Status.StartTime .TaskRun.Status.CompletionTime }}	{{ formatCondition .TaskRun.Status.Conditions }}
{{- $msg := hasFailed .TaskRun -}}
{{-  if ne $msg "" }}

Message
{{ $msg }}
{{- end }}

Inputs
{{- $l := len .TaskRun.Spec.Inputs.Resources }}{{ if eq $l 0 }}
No resources
{{- else }}
NAME	RESOURCE REF
{{- range $i, $r := .TaskRun.Spec.Inputs.Resources }}
{{$r.Name }}	{{ $r.ResourceRef.Name }}
{{- end }}
{{- end }}

Outputs
{{- $l := len .TaskRun.Spec.Outputs.Resources }}{{ if eq $l 0 }}
No resources
{{- else }}
NAME	RESOURCE REF
{{- range $i, $r := .TaskRun.Spec.Outputs.Resources }}
{{$r.Name }}	{{ $r.ResourceRef.Name }}
{{- end }}
{{- end }}

Params
{{- $l := len .TaskRun.Spec.Inputs.Params }}{{ if eq $l 0 }}
No params
{{- else }}
NAME	VALUE
{{- range $i, $p := .TaskRun.Spec.Inputs.Params }}
{{- if eq $p.Value.Type "string" }}
{{ $p.Name }}	{{ $p.Value.StringVal }}
{{- else }}
{{ $p.Name }}	{{ $p.Value.ArrayVal }}
{{- end }}
{{- end }}
{{- end }}

Steps
{{- $l := len .TaskRun.Status.Steps }}{{ if eq $l 0 }}
No steps
{{- else }}
STEP NAME
{{- range $steps := .TaskRun.Status.Steps }}
{{ $steps.Name }}
{{- end }}
{{- end }}
`

func describeCommand(p cli.Params) *cobra.Command {
	f := cliopts.NewPrintFlags("describe")
	eg := `
# Describe a TaskRun of name 'foo' in namespace 'bar'
tkn taskrun describe foo -n bar

tkn tr desc foo -n bar
`

	c := &cobra.Command{
		Use:          "describe",
		Aliases:      []string{"desc"},
		Short:        "Describe a taskrun in a namespace",
		Example:      eg,
		Args:         cobra.MinimumNArgs(1),
		SilenceUsage: true,
		Annotations: map[string]string{
			"commandType": "main",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			s := &cli.Stream{
				Out: cmd.OutOrStdout(),
				Err: cmd.OutOrStderr(),
			}
			return printTaskRunDescription(s, args[0], p)
		},
	}

	_ = c.MarkZshCompPositionalArgumentCustom(1, "__tkn_get_taskrun")
	f.AddFlags(c)

	return c
}

func printTaskRunDescription(s *cli.Stream, trName string, p cli.Params) error {
	cs, err := p.Clients()
	if err != nil {
		return fmt.Errorf("failed to create tekton client")
	}

	tr, err := cs.Tekton.TektonV1alpha1().TaskRuns(p.Namespace()).Get(trName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to find taskrun %q", trName)
	}

	var data = struct {
		TaskRun *v1alpha1.TaskRun
		Params  cli.Params
	}{
		TaskRun: tr,
		Params:  p,
	}

	funcMap := template.FuncMap{
		"formatAge":       formatted.Age,
		"formatDuration":  formatted.Duration,
		"formatCondition": formatted.Condition,
		"hasFailed":       hasFailed,
	}

	w := tabwriter.NewWriter(s.Out, 0, 5, 3, ' ', tabwriter.TabIndent)
	t := template.Must(template.New("Describe taskrun").Funcs(funcMap).Parse(templ))

	err = t.Execute(w, data)
	if err != nil {
		fmt.Fprintf(s.Err, "Failed to execute template")
		return err
	}
	return w.Flush()
}

func hasFailed(tr *v1alpha1.TaskRun) string {
	if len(tr.Status.Conditions) == 0 {
		return ""
	}

	if tr.Status.Conditions[0].Status == corev1.ConditionFalse {
		return tr.Status.Conditions[0].Message
	}
	return ""
}
