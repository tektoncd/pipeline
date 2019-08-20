package pipelinerun

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/tektoncd/cli/pkg/cli"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
		Args:         cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			pr := args[0]
			s := &cli.Stream{
				Out: cmd.OutOrStdout(),
				Err: cmd.OutOrStderr(),
			}
			// opts.Streamer = pods.NewStream
			return cancelPipelineRun(p, s, pr)
		},
	}

	c.MarkZshCompPositionalArgumentCustom(1, "__tkn_get_pipelinerun")
	return c
}

func cancelPipelineRun(p cli.Params, s *cli.Stream, prName string) error {
	cs, err := p.Clients()
	if err != nil {
		return fmt.Errorf("Failed to create tekton client")
	}

	pr, err := cs.Tekton.TektonV1alpha1().PipelineRuns(p.Namespace()).Get(prName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("Failed to find pipelinerun: %s", prName)
	}

	pr.Spec.Status = v1alpha1.PipelineRunSpecStatusCancelled
	_, err = cs.Tekton.TektonV1alpha1().PipelineRuns(p.Namespace()).Update(pr)
	if err != nil {
		return fmt.Errorf("Failed to cancel pipelinerun: %s, err: %s", prName, err.Error())
	}

	fmt.Fprintf(s.Out, "Pipelinerun cancelled: %s\n", pr.Name)
	return nil
}
