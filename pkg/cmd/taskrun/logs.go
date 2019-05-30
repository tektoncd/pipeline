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
	"github.com/tektoncd/cli/pkg/logs"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/reconciler/v1alpha1/taskrun/resources"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	msgTRNotFoundErr = "Error in retrieving Taskrun"
)

// Logs fetches logs of a taskrun
type Logs struct {
	Task    string
	Run     string
	Ns      string
	Clients *cli.Clients
}

// LogOptions provides options on what logs to fetch. An empty LogOptions
// implies fetching all logs including init steps
type LogOptions struct {
	AllSteps bool
}

func logCommand(p cli.Params) *cobra.Command {
	opts := LogOptions{}
	eg := `
# show the logs of TaskRun named "foo" from the namespace "bar"
tkn taskrun logs foo -n bar
`

	c := &cobra.Command{
		Use:          "logs",
		Short:        "Show taskruns logs",
		Example:      eg,
		SilenceUsage: true,
		Args:         cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {

			cs, err := p.Clients()
			if err != nil {
				return err
			}

			trl := &Logs{
				Run:     args[0],
				Ns:      p.Namespace(),
				Clients: cs,
			}

			s := logs.Streams{
				Out: cmd.OutOrStdout(),
				Err: cmd.OutOrStderr(),
			}

			trl.Fetch(opts, s, logs.DefaultLogFetcher(cs.Kube))
			return nil
		},
	}

	c.Flags().BoolVarP(
		&opts.AllSteps,
		"all", "a", false,
		"show all logs including init steps injected by tekton")
	return c
}

//Fetch To fetch the TaskRun's logs.
//Stream provides output and error stream to print the logs and error messages.
//LogOptions provide way to print all(init) steps
func (trl *Logs) Fetch(opts LogOptions, stream logs.Streams, reader *logs.LogFetcher) {
	var (
		kube   = trl.Clients.Kube
		tekton = trl.Clients.Tekton
	)

	tr, err := tekton.TektonV1alpha1().TaskRuns(trl.Ns).Get(trl.Run, metav1.GetOptions{})
	if err != nil {
		fmt.Fprintf(stream.Err, "%s : %s \n", msgTRNotFoundErr, err)
		return
	}

	trl.Task = tr.Spec.TaskRef.Name
	trStatus := tr.Status
	if !taskRunHasStarted(trStatus) {
		fmt.Fprintf(stream.Out, "Task %s has not started yet \n", trl.Task)
		return
	}

	pod, err := kube.CoreV1().
		Pods(trl.Ns).
		Get(trStatus.PodName, metav1.GetOptions{})
	if err != nil {
		fmt.Fprintf(stream.Err, "Error in getting pod: %s \n", err)
		return
	}

	if !podHasStarted(pod) {
		fmt.Fprintf(stream.Out, "Task %s pod %s has not started yet \n", trl.Task, trStatus.PodName)
		return
	}

	steps := filterSteps(trStatus, pod, opts.AllSteps)

	for _, step := range steps {
		if !stepHasStarted(step) {
			fmt.Fprintf(stream.Out, "Step %s has not started yet \n", trl.Task)
			continue
		}

		pl := logs.NewPodLogs(trStatus.PodName, trl.Ns, reader)
		err := pl.Fetch(stream, ContainerNameForStep(step.Name), func(s string) string {
			return fmt.Sprintf("[%s : %s] %s", trl.Task, step.Name, s)
		})

		if err != nil {
			fmt.Fprintf(stream.Err, "Error in printing logs for the %s : %s \n", step.Name, err)
		}

		fmt.Fprint(stream.Out, "\n")
	}
}

func podHasStarted(pod *corev1.Pod) bool {
	return !(pod.Status.Phase == corev1.PodPending || pod.Status.Phase == corev1.PodUnknown)
}

func taskRunHasStarted(trStatus v1alpha1.TaskRunStatus) bool {
	return trStatus.StartTime != nil && !trStatus.StartTime.IsZero()
}

func filterSteps(trStatus v1alpha1.TaskRunStatus, pod *corev1.Pod, allSteps bool) []v1alpha1.StepState {
	if !allSteps {
		return trStatus.Steps
	}

	initSteps := []v1alpha1.StepState{}
	for _, ics := range pod.Status.InitContainerStatuses {
		initSteps = append(initSteps, v1alpha1.StepState{
			ContainerState: *ics.State.DeepCopy(),
			Name:           resources.TrimContainerNamePrefix(ics.Name),
		})
	}
	//append normal steps to preserve the order
	initSteps = append(initSteps, trStatus.Steps...)

	return initSteps
}

func stepHasStarted(stepState v1alpha1.StepState) bool {
	return stepState.ContainerState.Waiting == nil
}

//TODO: anonymous steps?
func ContainerNameForStep(name string) string {
	switch name {
	case "nop":
		return "nop"
	default:
		return "build-step-" + name
	}
}
