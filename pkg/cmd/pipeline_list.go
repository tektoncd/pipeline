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

package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cli "k8s.io/cli-runtime/pkg/genericclioptions"
)

// pipelineListCmd holds everything related to "pipelines list"" to
// namespace all package variables
var pipelineListCmd = struct {
	cmd      *cobra.Command
	cliFlags *genericclioptions.PrintFlags
}{}

func init() {
	pipelinesCmd.AddCommand(pipelineList())
}

func pipelineList() *cobra.Command {
	if pipelineListCmd.cmd != nil {
		return pipelineListCmd.cmd
	}

	c := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "A brief description of your command",
		Long: `A longer description that spans multiple lines and likely contains examples
	and usage of using your command. For example:

	Cobra is a CLI library for Go that empowers applications.
	This application is a tool to generate the needed files
	to quickly create a Cobra application.`,
		RunE: run,
	}

	defaultOutput := `jsonpath={range .items[*]}{.metadata.name}{"\n"}{end}`
	f := cli.NewPrintFlags("").WithDefaultOutput(defaultOutput)
	f.AddFlags(c)

	pipelineListCmd.cmd = c
	pipelineListCmd.cliFlags = f

	return pipelineListCmd.cmd
}

func run(cmd *cobra.Command, args []string) error {
	var err error

	cs, err := versioned.NewForConfig(kubeConfig)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create client from config %s  %s", kubeCfgFile, err)
		return err
	}
	c := cs.TektonV1alpha1().Pipelines(namespace)
	ps, err := c.List(v1.ListOptions{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to list pipelines from namespace %s  %s", namespace, err)
		return err
	}
	printer, err := pipelineListCmd.cliFlags.ToPrinter()
	if err != nil {
		return err
	}
	return printer.PrintObj(ps, cmd.OutOrStdout())
}
