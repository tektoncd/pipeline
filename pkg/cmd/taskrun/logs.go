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

package taskrun

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/tektoncd/cli/pkg/cli"
	"github.com/tektoncd/cli/pkg/helper/pods"
	"github.com/tektoncd/cli/pkg/helper/pods/stream"
)

const (
	msgTRNotFoundErr = "Unable to get Taskrun"
)

// LogOptions provides options on what logs to fetch. An empty LogOptions
// implies fetching all logs including init steps
type LogOptions struct {
	taskrunName string
	allSteps    bool
	follow      bool
	stream      *cli.Stream
	params      cli.Params
	streamer    stream.NewStreamerFunc
}

func logCommand(p cli.Params) *cobra.Command {
	opts := LogOptions{params: p}
	eg := `
# show the logs of TaskRun named "foo" from the namespace "bar"
tkn taskrun logs foo -n bar

# show the live logs of TaskRun named "foo" from the namespace "bar"
tkn taskrun logs -f foo -n bar
`
	c := &cobra.Command{
		Use:          "logs",
		Short:        "Show taskruns logs",
		Example:      eg,
		SilenceUsage: true,
		Args:         cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.taskrunName = args[0]
			opts.stream = &cli.Stream{
				Out: cmd.OutOrStdout(),
				Err: cmd.OutOrStderr(),
			}
			opts.streamer = pods.NewStream

			return opts.run()
		},
	}

	c.Flags().BoolVarP(&opts.allSteps, "all", "a", false, "show all logs including init steps injected by tekton")
	c.Flags().BoolVarP(&opts.follow, "follow", "f", false, "stream live logs")

	_ = c.MarkZshCompPositionalArgumentCustom(1, "__tkn_get_taskrun")
	return c
}

func (lo *LogOptions) run() error {
	if lo.taskrunName == "" {
		return fmt.Errorf("missing mandatory argument taskrun")
	}

	cs, err := lo.params.Clients()
	if err != nil {
		return err
	}

	lr := &LogReader{
		Run:      lo.taskrunName,
		Ns:       lo.params.Namespace(),
		Clients:  cs,
		Streamer: lo.streamer,
		Follow:   lo.follow,
		AllSteps: lo.allSteps,
	}

	logC, errC, err := lr.Read()
	if err != nil {
		return err
	}

	NewLogWriter().Write(lo.stream, logC, errC)
	return nil
}
