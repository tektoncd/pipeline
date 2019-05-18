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
	return trs.Status.PodName != ""
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
