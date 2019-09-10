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

package pipelineresource

import (
	"fmt"
	"text/tabwriter"
	"text/template"

	"github.com/spf13/cobra"
	"github.com/tektoncd/cli/pkg/cli"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	cliopts "k8s.io/cli-runtime/pkg/genericclioptions"
)

const templ = `Name:	{{ .PipelineResource.Name }}
Namespace:	{{ .PipelineResource.Namespace }}
PipelineResource Type:	{{ .PipelineResource.Spec.Type }}

Params
{{- $l := len .PipelineResource.Spec.Params }}{{ if eq $l 0 }}
No params
{{- else }}
NAME	VALUE
{{- range $i, $p := .PipelineResource.Spec.Params }}
{{ $p.Name }}	{{ $p.Value }}
{{- end }}
{{- end }}

Secret Params
{{- $l := len .PipelineResource.Spec.SecretParams }}{{ if eq $l 0 }}
No secret params
{{- else }}
FIELDNAME	SECRETNAME
{{- range $i, $p := .PipelineResource.Spec.SecretParams }}
{{ $p.FieldName }}	{{ $p.SecretName }}
{{- end }}
{{- end }}
`

func describeCommand(p cli.Params) *cobra.Command {
	f := cliopts.NewPrintFlags("describe")
	eg := `
# Describe a PipelineResource of name 'foo' in namespace 'bar'
tkn resource describe foo -n bar

tkn res desc foo -n bar
`

	c := &cobra.Command{
		Use:          "describe",
		Aliases:      []string{"desc"},
		Short:        "Describes a pipeline resource in a namespace",
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
			return printPipelineResourceDescription(s, p, args[0])
		},
	}

	_ = c.MarkZshCompPositionalArgumentCustom(1, "__tkn_get_pipelineresource")
	f.AddFlags(c)
	return c
}

func printPipelineResourceDescription(s *cli.Stream, p cli.Params, preName string) error {
	cs, err := p.Clients()
	if err != nil {
		return fmt.Errorf("failed to create tekton client")
	}

	pre, err := cs.Tekton.TektonV1alpha1().PipelineResources(p.Namespace()).Get(preName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to find pipelineresource %q", preName)
	}

	var data = struct {
		PipelineResource *v1alpha1.PipelineResource
		Params           cli.Params
	}{
		PipelineResource: pre,
		Params:           p,
	}

	w := tabwriter.NewWriter(s.Out, 0, 5, 3, ' ', tabwriter.TabIndent)
	t := template.Must(template.New("Describe PipelineResource").Parse(templ))

	err = t.Execute(w, data)
	if err != nil {
		fmt.Fprintf(s.Err, "Failed to execute template")
		return err
	}
	return w.Flush()
}
