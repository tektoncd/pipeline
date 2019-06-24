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
	"github.com/spf13/cobra"
	"github.com/tektoncd/cli/pkg/cli"
	"github.com/tektoncd/cli/pkg/cmd/pipelinerun"
	"github.com/tektoncd/cli/pkg/formatted"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"github.com/AlecAivazis/survey/v2/terminal"

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
  # show the logs of the latest pipelinerun of given pipeline
	tkn pipeline logs foo -n bar

   `
	c := &cobra.Command{
		Use: "logs",
		DisableFlagsInUseLine: true,
		Short:   "Show the logs of latest pipelinerun of given pipeline ",
		Example: eg,
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
	return c
}

func (opts *logOptions) run(args []string) error {
	if err := opts.init(args); err != nil {
		return err
	}

	runLogOpts := &pipelinerun.LogOptions{
		PipelineName:    opts.pipelineName,
		PipelineRunName: opts.runName,
		Stream:          opts.stream,
		Params:          opts.params,
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
		if opts.last {
			return opts.initLastRunName()
		}
		return opts.askRunName()

	case 2: // both pipeline and run provided
		opts.pipelineName = args[0]
		opts.runName = args[1]

	default:
		fmt.Fprintln(opts.stream.Err,"too many arguments")
		return  nil

	}
	return nil
}

func (opts *logOptions) getAllInputs() error {
	ps, err := allPipelines(opts.params)
	if err != nil {
		return err
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
	var ans string
	prs, err := allRuns(opts.params, opts.pipelineName)
	if err != nil {
		return err
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

	err = survey.Ask(qs2, &ans, opts.askOpts)
	if err != nil {
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
	opts.runName = lastrun
	return nil
}

func allPipelines(p cli.Params) ([]string, error) {

	cs, err := p.Clients()
	if err != nil {
		return nil, err
	}

	tkn := cs.Tekton.TektonV1alpha1()
	ps, err := tkn.Pipelines(p.Namespace()).List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	ret := []string{}
	for _, item := range ps.Items {
		ret = append(ret, item.ObjectMeta.Name)
	}
	return ret, nil
}

func allRuns(p cli.Params, pName string) ([]string, error) {
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

	if len(runs.Items) == 0 {
		return nil, fmt.Errorf("No pipeline runs found in namespace: %s", p.Namespace())
	}
	ret := []string{}
	for i, run := range runs.Items {
		if i < 5 {
			ret = append(ret, run.ObjectMeta.Name+" started "+formatted.Age(*run.Status.StartTime, p.Time()))
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
		return "", fmt.Errorf("No pipeline runs found for %s in namespace: %s", pName, ns)
	}

	var latest v1alpha1.PipelineRun
	for _, run := range runs.Items {
		if run.CreationTimestamp.After(latest.CreationTimestamp.Time) {
			latest = run
		}
	}

	return latest.ObjectMeta.Name, nil
}
