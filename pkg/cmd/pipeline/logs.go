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
	"os"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/AlecAivazis/survey/v2/terminal"
	"github.com/spf13/cobra"
	"github.com/tektoncd/cli/pkg/cli"
	"github.com/tektoncd/cli/pkg/cmd/pipelinerun"
	"github.com/tektoncd/cli/pkg/formatted"
	prhsort "github.com/tektoncd/cli/pkg/helper/pipelinerun/sort"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type logOptions struct {
	params       cli.Params
	stream       *cli.Stream
	askOpts      survey.AskOpt
	last         bool
	allSteps     bool
	follow       bool
	pipelineName string
	runName      string
	limit        int
}

func nameArg(args []string, p cli.Params) error {
	if len(args) == 1 {
		c, err := p.Clients()
		if err != nil {
			return err
		}
		name, ns := args[0], p.Namespace()
		if _, err = c.Tekton.TektonV1alpha1().Pipelines(ns).Get(name, metav1.GetOptions{}); err != nil {
			return err
		}
	}
	return nil
}

func logCommand(p cli.Params) *cobra.Command {
	opts := &logOptions{params: p,
		askOpts: func(opt *survey.AskOptions) error {
			opt.Stdio = terminal.Stdio{
				In:  os.Stdin,
				Out: os.Stdout,
				Err: os.Stderr,
			}
			return nil
		},
	}

	eg := `
  # interactive mode: shows logs of the selected pipeline run
    tkn pipeline logs -n namespace

  # interactive mode: shows logs of the selected pipelinerun of the given pipeline
    tkn pipeline logs pipeline -n namespace

  # show logs of given pipeline for last run
    tkn pipeline logs pipeline -n namespace --last

  # show logs for given pipeline and pipelinerun
    tkn pipeline logs pipeline run -n namespace

   `
	c := &cobra.Command{
		Use:                   "logs",
		DisableFlagsInUseLine: true,
		Short:                 "Show pipeline logs",
		Example:               eg,
		SilenceUsage:          true,
		Annotations: map[string]string{
			"commandType": "main",
		},
		Args: func(cmd *cobra.Command, args []string) error {
			return nameArg(args, p)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.stream = &cli.Stream{
				Out: cmd.OutOrStdout(),
				Err: cmd.OutOrStderr(),
			}
			return opts.run(args)
		},
	}
	c.Flags().BoolVarP(&opts.last, "last", "l", false, "show logs for last run")
	c.Flags().BoolVarP(&opts.allSteps, "all", "a", false, "show all logs including init steps injected by tekton")
	c.Flags().BoolVarP(&opts.follow, "follow", "f", false, "stream live logs")
	c.Flags().IntVarP(&opts.limit, "limit", "L", 5, "lists number of pipelineruns")

	_ = c.MarkZshCompPositionalArgumentCustom(1, "__tkn_get_pipeline")
	return c
}

func (opts *logOptions) run(args []string) error {
	if err := opts.init(args); err != nil {
		return err
	}

	if opts.pipelineName == "" || opts.runName == "" {
		return nil
	}

	runLogOpts := &pipelinerun.LogOptions{
		PipelineName:    opts.pipelineName,
		PipelineRunName: opts.runName,
		Stream:          opts.stream,
		Params:          opts.params,
		Follow:          opts.follow,
		AllSteps:        opts.allSteps,
	}

	return runLogOpts.Run()
}

func (opts *logOptions) init(args []string) error {
	// ensure the client is properly initialized
	if _, err := opts.params.Clients(); err != nil {
		return err
	}

	switch len(args) {
	case 0: // no inputs
		return opts.getAllInputs()

	case 1: // pipeline name provided
		opts.pipelineName = args[0]
		return opts.askRunName()

	case 2: // both pipeline and run provided
		opts.pipelineName = args[0]
		opts.runName = args[1]

	default:
		return fmt.Errorf("too many arguments")
	}
	return nil
}

func (opts *logOptions) getAllInputs() error {
	err := validate(opts)

	if err != nil {
		return err
	}

	ps, err := allPipelines(opts)
	if err != nil {
		return err
	}

	if len(ps) == 0 {
		fmt.Fprintln(opts.stream.Err, "No pipelines found in namespace:", opts.params.Namespace())
		return nil
	}

	var qs1 = []*survey.Question{{
		Name: "pipeline",
		Prompt: &survey.Select{
			Message: "Select pipeline :",
			Options: ps,
		},
	}}

	err = survey.Ask(qs1, &opts.pipelineName, opts.askOpts)
	if err != nil {
		fmt.Println(err.Error())
		return err
	}

	return opts.askRunName()
}

func (opts *logOptions) askRunName() error {
	err := validate(opts)
	if err != nil {
		return err
	}

	if opts.last {
		return opts.initLastRunName()
	}

	var ans string

	prs, err := allRuns(opts.params, opts.pipelineName, opts.limit)
	if err != nil {
		return err
	}
	if len(prs) == 0 {
		fmt.Fprintln(opts.stream.Err, "No pipelineruns found for pipeline:", opts.pipelineName)
		return nil
	}

	if len(prs) == 1 {
		opts.runName = strings.Fields(prs[0])[0]
		return nil
	}

	var qs2 = []*survey.Question{
		{
			Name: "pipelinerun",
			Prompt: &survey.Select{
				Message: "Select pipelinerun :",
				Options: prs,
			},
		},
	}

	if err = survey.Ask(qs2, &ans, opts.askOpts); err != nil {
		fmt.Println(err.Error())
		return err
	}

	opts.runName = strings.Fields(ans)[0]
	return nil
}

func (opts *logOptions) initLastRunName() error {
	cs, err := opts.params.Clients()
	if err != nil {
		return err
	}
	lastrun, err := lastRun(cs, opts.params.Namespace(), opts.pipelineName)
	if err != nil {
		return err
	}
	opts.runName = lastrun
	return nil
}

func allPipelines(opts *logOptions) ([]string, error) {
	cs, err := opts.params.Clients()
	if err != nil {
		return nil, err
	}

	tkn := cs.Tekton.TektonV1alpha1()
	ps, err := tkn.Pipelines(opts.params.Namespace()).List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	ret := []string{}
	for _, item := range ps.Items {
		ret = append(ret, item.ObjectMeta.Name)
	}
	return ret, nil
}

func allRuns(p cli.Params, pName string, limit int) ([]string, error) {
	cs, err := p.Clients()
	if err != nil {
		return nil, err
	}
	opts := metav1.ListOptions{
		LabelSelector: fmt.Sprintf("tekton.dev/pipeline=%s", pName),
	}

	runs, err := cs.Tekton.TektonV1alpha1().PipelineRuns(p.Namespace()).List(opts)
	if err != nil {
		return nil, err
	}

	runslen := len(runs.Items)

	if runslen > 1 {
		runs.Items = prhsort.SortPipelineRunsByStartTime(runs.Items)
	}

	if limit > runslen {
		limit = runslen
	}

	ret := []string{}
	for i, run := range runs.Items {
		if i < limit {
			ret = append(ret, run.ObjectMeta.Name+" started "+formatted.Age(run.Status.StartTime, p.Time()))
		}
	}
	return ret, nil
}

func lastRun(cs *cli.Clients, ns, pName string) (string, error) {
	opts := metav1.ListOptions{
		LabelSelector: fmt.Sprintf("tekton.dev/pipeline=%s", pName),
	}

	runs, err := cs.Tekton.TektonV1alpha1().PipelineRuns(ns).List(opts)
	if err != nil {
		return "", err
	}

	if len(runs.Items) == 0 {
		return "", fmt.Errorf("no pipeline runs found for %s in namespace: %s", pName, ns)
	}

	var latest v1alpha1.PipelineRun
	for _, run := range runs.Items {
		if run.CreationTimestamp.After(latest.CreationTimestamp.Time) {
			latest = run
		}
	}
	return latest.ObjectMeta.Name, nil
}

func validate(opts *logOptions) error {

	if opts.limit <= 0 {
		return fmt.Errorf("limit was %d but must be a positive number", opts.limit)
	}

	return nil
}
