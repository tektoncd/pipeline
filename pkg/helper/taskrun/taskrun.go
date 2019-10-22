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
	"github.com/tektoncd/cli/pkg/cli"
	"github.com/tektoncd/cli/pkg/cmd/taskrun"
	"github.com/tektoncd/cli/pkg/helper/pods/stream"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
)

type Run struct {
	Name string
	Task string
}

//NewLogReader returns the new instance of LogReader for
//a Run instance
func (t *Run) NewLogReader(ns string, clientSet *cli.Clients,
	streamer stream.NewStreamerFunc,
	num int, follow bool,
	allSteps bool) *taskrun.LogReader {

	return &taskrun.LogReader{
		Run:      t.Name,
		Task:     t.Task,
		Number:   num,
		Ns:       ns,
		Clients:  clientSet,
		Streamer: streamer,
		Follow:   follow,
		AllSteps: allSteps,
	}
}

func IsFiltered(tr Run, allowed []string) bool {
	trs := []Run{tr}
	return len(Filter(trs, allowed)) == 0
}

func HasScheduled(trs *v1alpha1.PipelineRunTaskRunStatus) bool {
	if trs.Status != nil {
		return trs.Status.PodName != ""
	}
	return false
}

func Filter(trs []Run, ts []string) []Run {
	if len(ts) == 0 {
		return trs
	}

	filter := map[string]bool{}
	for _, t := range ts {
		filter[t] = true
	}

	filtered := []Run{}
	for _, tr := range trs {
		if filter[tr.Task] {
			filtered = append(filtered, tr)
		}
	}

	return filtered
}

type taskRunMap map[string]*v1alpha1.PipelineRunTaskRunStatus

func SortTasksBySpecOrder(pipelineTasks []v1alpha1.PipelineTask, pipelinesTaskRuns taskRunMap) []Run {
	trNames := map[string]string{}

	for name, t := range pipelinesTaskRuns {
		trNames[t.PipelineTaskName] = name
	}

	trs := []Run{}
	for _, ts := range pipelineTasks {
		if n, ok := trNames[ts.Name]; ok {
			trs = append(trs, Run{
				Task: ts.Name,
				Name: n,
			})
		}
	}

	return trs
}
