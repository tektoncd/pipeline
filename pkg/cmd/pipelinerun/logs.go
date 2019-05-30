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
	"github.com/tektoncd/cli/pkg/cmd/taskrun"
	"github.com/tektoncd/cli/pkg/logs"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	msgPipelineNotFoundErr = "Error in retrieving Pipeline"
	msgPRNotFoundErr       = "Error in retrieving PipelineRun"
)

type taskRunMap map[string]*v1alpha1.PipelineRunTaskRunStatus

type PipelineRunLogs struct {
	Name    string
	Ns      string
	Clients *cli.Clients
}

type taskRun struct {
	name string
	task string
}

type LogOptions struct {
	taskrun.LogOptions
	Tasks []string
}

func logCommand(p cli.Params) *cobra.Command {
	var (
		allSteps  bool
		onlyTasks []string
		eg        = `
  # show the logs of PipelineRun named "foo" from the namesspace "bar"
    tkn pipelinerun logs foo -n bar

  # show the logs of PipelineRun named "microservice-1" for task "build" only, from the namespace "bar"
    tkn pr logs microservice-1 -t build -n bar

  # show the logs of PipelineRun named "microservice-1" for all tasks and steps (including init steps), 
    from the namespace "foo"
    tkn pr logs microservice-1 -a -n foo
   `
	)

	c := &cobra.Command{
		Use:                   "logs",
		DisableFlagsInUseLine: true,
		Short:                 "Show the logs of PipelineRun",
		Example:               eg,
		Args:                  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			prName := args[0]

			cs, err := p.Clients()
			if err != nil {
				return err
			}

			prLogs := NewPipelineRunLogs(prName, p.Namespace(), cs)

			opt := LogOptions{
				LogOptions: taskrun.LogOptions{
					AllSteps: allSteps,
				},
				Tasks: onlyTasks,
			}

			s := logs.Streams{
				Out: cmd.OutOrStderr(),
				Err: cmd.OutOrStderr(),
			}

			//TODO: put log fetcher as pipelinerun dependency
			prLogs.Fetch(s, opt, logs.DefaultLogFetcher(cs.Kube))

			return nil
		},
	}

	c.Flags().BoolVarP(&allSteps, "all", "a", false, "show all logs including init steps injected by tekton")
	c.Flags().StringSliceVarP(&onlyTasks, "only-tasks", "t", []string{}, "show the logs for mentioned task only")

	return c
}

//NewPipelineRunLogs create a new instance of PipelineRunLogs.
func NewPipelineRunLogs(pipelineRunName string, namespace string, clients *cli.Clients) *PipelineRunLogs {
	return &PipelineRunLogs{
		Name:    pipelineRunName,
		Ns:      namespace,
		Clients: clients,
	}
}

//Fetch To fetch the PipelineRun's logs.
//Stream provides output and error stream to print the logs and error messages.
//LogOptions provide way to print all(init) steps or specified task logs
func (p *PipelineRunLogs) Fetch(s logs.Streams, opts LogOptions, r *logs.LogFetcher) {
	var (
		tkn = p.Clients.Tekton
	)

	pr, err := tkn.TektonV1alpha1().
		PipelineRuns(p.Ns).
		Get(p.Name, metav1.GetOptions{})

	if err != nil {
		fmt.Fprintf(s.Err, "%s : %s \n", msgPRNotFoundErr, err)
		return
	}

	if failed, msg := pipelineRunFailed(pr.Status); failed {
		fmt.Fprintf(s.Out, "PipelineRun is failed: %s \n", msg)
	}

	pl, err := tkn.TektonV1alpha1().
		Pipelines(p.Ns).
		Get(pr.Spec.PipelineRef.Name, metav1.GetOptions{})

	if err != nil {
		fmt.Fprintf(s.Err, "%s : %s \n", msgPipelineNotFoundErr, err)
		return
	}

	//Sort taskruns, to display the taskrun logs as per pipeline tasks order
	ordered := orderBy(pl.Spec.Tasks, pr.Status.TaskRuns)
	taskRuns := filterBy(opts.Tasks, ordered)

	for _, tr := range taskRuns {
		trl := tr.toTaskRunLogs(p.Ns, p.Clients)
		trl.Fetch(opts.LogOptions, s, r)
	}
}

func pipelineRunFailed(status v1alpha1.PipelineRunStatus) (bool, string) {
	c := status.Conditions[0]
	if c.Status == corev1.ConditionFalse {
		return true, c.Message
	}

	return false, ""
}

func orderBy(pipelineTasks []v1alpha1.PipelineTask, pipelinesTaskRuns taskRunMap) []taskRun {
	trNames := map[string]string{}

	for name, t := range pipelinesTaskRuns {
		trNames[t.PipelineTaskName] = name
	}

	trs := []taskRun{}
	for _, ts := range pipelineTasks {
		if n, ok := trNames[ts.Name]; ok {
			trs = append(trs, taskRun{
				task: ts.Name,
				name: n,
			})
		}
	}

	return trs
}

func filterBy(ts []string, trs []taskRun) []taskRun {
	if len(ts) == 0 {
		return trs
	}

	filter := map[string]bool{}
	for _, t := range ts {
		filter[t] = true
	}

	filtered := []taskRun{}
	for _, tr := range trs {
		if filter[tr.task] {
			filtered = append(filtered, tr)
		}
	}

	return filtered
}

func (t taskRun) toTaskRunLogs(namespae string, clientSet *cli.Clients) *taskrun.Logs {
	return &taskrun.Logs{
		Run:     t.name,
		Task:    t.task,
		Ns:      namespae,
		Clients: clientSet,
	}
}
