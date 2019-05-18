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
	"github.com/tektoncd/cli/pkg/helper/pods/stream"

	"github.com/spf13/cobra"
	"github.com/tektoncd/cli/pkg/cli"
	"github.com/tektoncd/cli/pkg/errors"
	"github.com/tektoncd/cli/pkg/helper/pods"
)

const (
	msgPipelineNotFoundErr = "Error in retrieving Pipeline"
	msgPRNotFoundErr       = "Error in retrieving PipelineRun"
)

type LogOptions struct {
	pipelinerunName string
	tasks           []string
	allSteps        bool
	follow          bool
	stream          *cli.Stream
	params          cli.Params
	streamer        stream.NewStreamerFunc
}

func logCommand(p cli.Params) *cobra.Command {
	opts := &LogOptions{params: p}
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
			opts.pipelinerunName = args[0]
			opts.stream = &cli.Stream{
				Out: cmd.OutOrStdout(),
				Err: cmd.OutOrStderr(),
			}
			opts.streamer = pods.NewStream

			return opts.run()
		},
	}

	c.Flags().BoolVarP(&opts.allSteps, "all", "a", false, "show all logs including init steps injected by tekton")
	c.Flags().BoolVarP(&opts.follow, "follow", "f", false, "stream the live logs")
	c.Flags().StringSliceVarP(&opts.tasks, "only-tasks", "t", []string{}, "show logs for mentioned tasks only")

	return c
}

func (lo *LogOptions) run() error {
	if lo.pipelinerunName == "" {
		return fmt.Errorf("missing mandatory argument taskrun")
	}

	cs, err := lo.params.Clients()
	if err != nil {
		return err
	}

	lr := &LogReader{
		Run:      lo.pipelinerunName,
		Ns:       lo.params.Namespace(),
		Clients:  cs,
		Streamer: lo.streamer,
		follow:   lo.follow,
		allSteps: lo.allSteps,
		tasks:    lo.tasks,
	}

	logC, errC, err := lr.Read()
	switch e := err.(type) {
	case *errors.WarningError:
		fmt.Fprintf(lo.stream.Err, "%s \n", e.Error())
	case nil: // ignore nil
	default:
		return err
	}

	lw := &LogWriter{}
	lw.Write(lo.stream, logC, errC)

	return nil
}
