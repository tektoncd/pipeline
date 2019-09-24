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
	"text/template"

	"github.com/spf13/cobra"
	"github.com/tektoncd/cli/pkg/cli"
	"github.com/tektoncd/cli/pkg/formatted"
	"github.com/tektoncd/cli/pkg/helper/pipeline"
	"github.com/tektoncd/cli/pkg/printer"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	cliopts "k8s.io/cli-runtime/pkg/genericclioptions"
)

const listTemplate = `{{- $pl := len .Pipelines.Items }}{{ if eq $pl 0 -}}
No pipelines
{{- else -}}
NAME	AGE	LAST RUN	STARTED	DURATION	STATUS
{{- range $_, $p := .Pipelines.Items }}
{{- $pr := accessMap $.PipelineRuns $p.Name }}
{{- if $pr }}
{{ $p.Name }}	{{ formatAge $p.CreationTimestamp $.Params.Time }}	{{ $pr.Name }}	{{ formatAge $pr.Status.StartTime $.Params.Time }}	{{ formatDuration $pr.Status.StartTime $pr.Status.CompletionTime }}	{{ formatCondition $pr.Status.Conditions }}
{{- else }}
{{ $p.Name }}	{{ formatAge $p.CreationTimestamp $.Params.Time }}	---	---	---	---
{{- end }}
{{- end }}
{{- end }}
`

func listCommand(p cli.Params) *cobra.Command {
	f := cliopts.NewPrintFlags("list")

	c := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "Lists pipelines in a namespace",
		Annotations: map[string]string{
			"commandType": "main",
		},
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

	ps, prs, err := listPipelineDetails(cs, p.Namespace())

	if err != nil {
		fmt.Fprintf(s.Err, "Failed to list pipelines from %s namespace\n", p.Namespace())
		return err
	}

	var data = struct {
		Pipelines    *v1alpha1.PipelineList
		PipelineRuns pipelineruns
		Params       cli.Params
	}{
		Pipelines:    ps,
		PipelineRuns: prs,
		Params:       p,
	}

	funcMap := template.FuncMap{
		"accessMap": func(prs pipelineruns, name string) *v1alpha1.PipelineRun {
			if pr, ok := prs[name]; ok {
				return &pr
			}

			return nil
		},
		"formatAge":       formatted.Age,
		"formatDuration":  formatted.Duration,
		"formatCondition": formatted.Condition,
	}

	w := tabwriter.NewWriter(s.Out, 0, 5, 3, ' ', tabwriter.TabIndent)
	t := template.Must(template.New("List Pipeline").Funcs(funcMap).Parse(listTemplate))
	err = t.Execute(w, data)
	if err != nil {
		return err
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

func listPipelineDetails(cs *cli.Clients, ns string) (*v1alpha1.PipelineList, pipelineruns, error) {

	ps, err := listAllPipelines(cs.Tekton, ns)
	if err != nil {
		return nil, nil, err
	}

	if len(ps.Items) == 0 {
		return ps, pipelineruns{}, nil
	}
	lastRuns := pipelineruns{}

	for _, p := range ps.Items {
		// TODO: may be just the pipeline details can be print
		lastRun, err := pipeline.LastRun(cs.Tekton, p.Name, ns)
		if err != nil {
			continue
		}
		lastRuns[p.Name] = *lastRun
	}

	return ps, lastRuns, nil
}
