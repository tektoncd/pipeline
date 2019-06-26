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
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/tektoncd/cli/pkg/cli"
	"github.com/tektoncd/cli/pkg/printer"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	"k8s.io/apimachinery/pkg/runtime/schema"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	cliopts "k8s.io/cli-runtime/pkg/genericclioptions"
)

const (
	msgNoPREsFound = "No pipelineresources found."
)

func listCommand(p cli.Params) *cobra.Command {
	f := cliopts.NewPrintFlags("list")
	eg := `
# List all PipelineResources in a namespaces 'foo'
tkn pre list -n foo",
`

	cmd := &cobra.Command{
		Use:          "list",
		Aliases:      []string{"ls"},
		Short:        "Lists pipeline resources in a namespace",
		Example:      eg,
		SilenceUsage: true,
		Args:         cobra.MinimumNArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			cs, err := p.Clients()
			if err != nil {
				return err
			}
			pres, err := list(cs.Tekton, p.Namespace())
			stream := &cli.Stream{
				Out: cmd.OutOrStdout(),
				Err: cmd.OutOrStderr(),
			}
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to list pipelineresources from %s namespace \n", p.Namespace())
				return err
			}

			output, err := cmd.LocalFlags().GetString("output")
			if err != nil {
				fmt.Fprint(os.Stderr, "Error: output option not set properly \n")
				return err
			}

			if output != "" {
				return printer.PrintObject(stream.Out, pres, f)
			}

			err = printFormatted(stream, pres)
			if err != nil {
				fmt.Fprint(os.Stderr, "Failed to print Pipelineresources \n")
				return err
			}
			return nil
		},
	}

	f.AddFlags(cmd)

	return cmd
}

func list(client versioned.Interface, namespace string) (*v1alpha1.PipelineResourceList, error) {

	options := v1.ListOptions{}
	prec := client.TektonV1alpha1().PipelineResources(namespace)
	pres, err := prec.List(options)
	if err != nil {
		return nil, err
	}

	// NOTE: this is required for -o json|yaml to work properly since
	// tektoncd go client fails to set these; probably a bug
	pres.GetObjectKind().SetGroupVersionKind(
		schema.GroupVersionKind{
			Version: "tekton.dev/v1alpha1",
			Kind:    "PipelineResourceList",
		})

	return pres, nil
}

func printFormatted(s *cli.Stream, pres *v1alpha1.PipelineResourceList) error {
	if len(pres.Items) == 0 {
		fmt.Fprintln(s.Err, msgNoPREsFound)
		return nil
	}

	w := tabwriter.NewWriter(s.Out, 0, 5, 3, ' ', tabwriter.TabIndent)
	fmt.Fprintln(w, "NAME\tTYPE\tDETAILS")
	for _, pre := range pres.Items {
		fmt.Fprintf(w, "%s\t%s\t%s\n",
			pre.Name,
			pre.Spec.Type,
			details(pre),
		)
	}

	return w.Flush()
}

func details(pre v1alpha1.PipelineResource) string {
	var key = "URL"
	if pre.Spec.Type == v1alpha1.PipelineResourceTypeStorage {
		key = "Location"
	}

	for _, p := range pre.Spec.Params {
		if p.Name == key {
			return p.Name + ": " + p.Value
		}
	}

	return "---"
}
