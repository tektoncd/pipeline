// Copyright © 2019 The Knative Authors.
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
	"os"

	"github.com/spf13/cobra"
	"github.com/tektoncd/cli/pkg/cli"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	cliopts "k8s.io/cli-runtime/pkg/genericclioptions"
)

func ListCommand(p cli.Params) *cobra.Command {

	defaultOutput := `jsonpath={range .items[*]}{.metadata.name}{"\n"}{end}`
	f := cliopts.NewPrintFlags("").WithDefaultOutput(defaultOutput)

	c := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "Lists pipelines in a namespace",
		Long:    ``,
		RunE: func(cmd *cobra.Command, args []string) error {
			ps, err := List(p)

			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to list pipelines from %s namespace\n", p.Namespace())
				return err
			}

			printer, err := f.ToPrinter()
			if err != nil {
				return err
			}
			return printer.PrintObj(ps, cmd.OutOrStdout())
		},
	}
	f.AddFlags(c)

	return c
}

func List(p cli.Params) (*v1alpha1.PipelineList, error) {
	cs, err := p.Clientset()
	if err != nil {
		return nil, err
	}

	c := cs.TektonV1alpha1().Pipelines(p.Namespace())
	return c.List(v1.ListOptions{})
}
