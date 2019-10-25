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

	"github.com/spf13/cobra"
	"github.com/tektoncd/cli/pkg/cli"
	"github.com/tektoncd/cli/pkg/formatted"
	validate "github.com/tektoncd/cli/pkg/helper/validate"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	succeeded   = "Succeeded"
	failed      = "Failed"
	prCancelled = "Failed(PipelineRunCancelled)"
)

func cancelCommand(p cli.Params) *cobra.Command {
	eg := `
  # cancel the PipelineRun named "foo" from the namespace "bar"
    tkn pipelinerun cancel foo -n bar
   `

	c := &cobra.Command{
		Use:          "cancel pipelinerunName",
		Short:        "Cancel the PipelineRun",
		Example:      eg,
		SilenceUsage: true,
		Annotations: map[string]string{
			"commandType": "main",
		},
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			pr := args[0]

			s := &cli.Stream{
				Out: cmd.OutOrStdout(),
				Err: cmd.OutOrStderr(),
			}

			if err := validate.NamespaceExists(p); err != nil {
				return err
			}

			return cancelPipelineRun(p, s, pr)
		},
	}

	_ = c.MarkZshCompPositionalArgumentCustom(1, "__tkn_get_pipelinerun")
	return c
}

func cancelPipelineRun(p cli.Params, s *cli.Stream, prName string) error {
	cs, err := p.Clients()
	if err != nil {
		return fmt.Errorf("failed to create tekton client")
	}

	pr, err := cs.Tekton.TektonV1alpha1().PipelineRuns(p.Namespace()).Get(prName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to find pipelinerun: %s", prName)
	}

	prCond := formatted.Condition(pr.Status.Conditions)
	if prCond == succeeded || prCond == failed || prCond == prCancelled {
		return fmt.Errorf("failed to cancel pipelinerun %s: pipelinerun has already finished execution", prName)
	}

	pr.Spec.Status = v1alpha1.PipelineRunSpecStatusCancelled
	_, err = cs.Tekton.TektonV1alpha1().PipelineRuns(p.Namespace()).Update(pr)
	if err != nil {
		return fmt.Errorf("failed to cancel pipelinerun: %s, err: %s", prName, err.Error())
	}

	fmt.Fprintf(s.Out, "Pipelinerun cancelled: %s\n", pr.Name)
	return nil
}
