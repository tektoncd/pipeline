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
	"github.com/tektoncd/cli/pkg/helper/pods"
	"github.com/tektoncd/cli/pkg/helper/pods/stream"
)

type LogOptions struct {
	AllSteps        bool
	Follow          bool
	Last            bool
	Params          cli.Params
	PipelineName    string
	PipelineRunName string
	Stream          *cli.Stream
	Streamer        stream.NewStreamerFunc
	Tasks           []string
}

func logCommand(p cli.Params) *cobra.Command {
	opts := &LogOptions{Params: p}
	eg := `
  # show the logs of PipelineRun named "foo" from the namesspace "bar"
    tkn pipelinerun logs foo -n bar

  # show the logs of PipelineRun named "microservice-1" for task "build" only, from the namespace "bar"
    tkn pr logs microservice-1 -t build -n bar

  # show the logs of PipelineRun named "microservice-1" for all tasks and steps (including init steps), 
    from the namespace "foo"
    tkn pr logs microservice-1 -a -n foo
   `

	c := &cobra.Command{
		Use:                   "logs",
		DisableFlagsInUseLine: true,
		Short:                 "Show the logs of PipelineRun",
		Example:               eg,
		Args:                  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.PipelineRunName = args[0]
			opts.Stream = &cli.Stream{
				Out: cmd.OutOrStdout(),
				Err: cmd.OutOrStderr(),
			}
			// opts.Streamer = pods.NewStream
			return opts.Run()
		},
	}

	c.Flags().BoolVarP(&opts.AllSteps, "all", "a", false, "show all logs including init steps injected by tekton")
	c.Flags().BoolVarP(&opts.Follow, "follow", "f", false, "stream live logs")
	c.Flags().StringSliceVarP(&opts.Tasks, "only-tasks", "t", []string{}, "show logs for mentioned tasks only")

	return c
}

func (lo *LogOptions) Run() error {
	if lo.PipelineRunName == "" {
		return fmt.Errorf("missing mandatory argument pipelinerun")
	}
	streamer := pods.NewStream
	if lo.Streamer != nil {
		streamer = lo.Streamer
	}
	cs, err := lo.Params.Clients()
	if err != nil {
		return err
	}

	lr := &LogReader{
		Run:      lo.PipelineRunName,
		Ns:       lo.Params.Namespace(),
		Clients:  cs,
		Streamer: streamer,
		Follow:   lo.Follow,
		AllSteps: lo.AllSteps,
		Tasks:    lo.Tasks,
	}

	logC, errC, err := lr.Read()
	if err != nil {
		return err
	}

	NewLogWriter().Write(lo.Stream, logC, errC)

	return nil
}
