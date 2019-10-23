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
	validate "github.com/tektoncd/cli/pkg/helper/validate"
)

const (
	msgTRNotFoundErr = "Unable to get Taskrun"
)

// LogOptions provides options on what logs to fetch. An empty LogOptions
// implies fetching all logs including init steps
type LogOptions struct {
	TaskrunName string
	AllSteps    bool
	Follow      bool
	Stream      *cli.Stream
	Params      cli.Params
	Streamer    stream.NewStreamerFunc
}

func logCommand(p cli.Params) *cobra.Command {
	opts := LogOptions{Params: p}
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
		Annotations: map[string]string{
			"commandType": "main",
		},
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.TaskrunName = args[0]
			opts.Stream = &cli.Stream{
				Out: cmd.OutOrStdout(),
				Err: cmd.OutOrStderr(),
			}

			if err := validate.NamespaceExists(p); err != nil {
				return err
			}

			opts.Streamer = pods.NewStream

			return opts.Run()
		},
	}

	c.Flags().BoolVarP(&opts.AllSteps, "all", "a", false, "show all logs including init steps injected by tekton")
	c.Flags().BoolVarP(&opts.Follow, "follow", "f", false, "stream live logs")

	_ = c.MarkZshCompPositionalArgumentCustom(1, "__tkn_get_taskrun")
	return c
}

func (lo *LogOptions) Run() error {
	if lo.TaskrunName == "" {
		return fmt.Errorf("missing mandatory argument taskrun")
	}

	cs, err := lo.Params.Clients()
	if err != nil {
		return err
	}

	lr := &LogReader{
		Run:      lo.TaskrunName,
		Ns:       lo.Params.Namespace(),
		Clients:  cs,
		Streamer: lo.Streamer,
		Follow:   lo.Follow,
		AllSteps: lo.AllSteps,
	}

	logC, errC, err := lr.Read()
	if err != nil {
		return err
	}

	NewLogWriter().Write(lo.Stream, logC, errC)
	return nil
}
