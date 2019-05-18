package pipelinerun

import (
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/tektoncd/cli/pkg/cli"
	"github.com/tektoncd/cli/pkg/cmd/taskrun"
	"github.com/tektoncd/cli/pkg/errors"
	"github.com/tektoncd/cli/pkg/helper/pipelinerun"
	"github.com/tektoncd/cli/pkg/helper/pods/stream"
	trh "github.com/tektoncd/cli/pkg/helper/taskrun"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type LogReader struct {
	Run      string
	Ns       string
	Clients  *cli.Clients
	Streamer stream.NewStreamerFunc
	allSteps bool
	follow   bool
	tasks    []string
}

// Log is the data gets written to the log channel
type Log struct {
	Pipeline string
	Task     string
	Step     string
	Log      string
}

func (lr *LogReader) Read() (<-chan Log, <-chan error, error) {
	tkn := lr.Clients.Tekton
	pr, err := tkn.TektonV1alpha1().PipelineRuns(lr.Ns).Get(lr.Run, metav1.GetOptions{})
	if err != nil {
		return nil, nil, fmt.Errorf("%s : %s", msgPRNotFoundErr, err)
	}

	if lr.follow {
		return lr.readLiveLogs(pr)
	}
	return lr.readAvailableLogs(pr)

}

func (lr *LogReader) readLiveLogs(pr *v1alpha1.PipelineRun) (<-chan Log, <-chan error, error) {
	logC := make(chan Log)
	errC := make(chan error)

	go func() {
		defer close(logC)
		defer close(errC)

		prTracker := pipelinerun.NewTracker(pr.Name, lr.Ns, lr.Clients.Tekton)
		trC := prTracker.Monitor(lr.tasks)

		wg := sync.WaitGroup{}
		taskIndex := int32(1)

		for trs := range trC {
			wg.Add(len(trs))

			for _, run := range trs {
				// NOTE: passing tr, taskIdx to avoid data race
				go func(tr trh.Run, taskNum int32) {
					defer wg.Done()

					tlr := tr.NewLogReader(lr.Ns, lr.Clients, lr.Streamer,
						int(taskNum), lr.follow, lr.allSteps)
					pipeLogs(logC, errC, tlr)
				}(run, atomic.AddInt32(&taskIndex, 1))
			}
		}

		wg.Wait()
	}()

	return logC, errC, nil
}

func (lr *LogReader) readAvailableLogs(pr *v1alpha1.PipelineRun) (<-chan Log, <-chan error, error) {
	tkn := lr.Clients.Tekton

	pl, err := tkn.TektonV1alpha1().Pipelines(lr.Ns).Get(pr.Spec.PipelineRef.Name, metav1.GetOptions{})
	if err != nil {
		return nil, nil, fmt.Errorf("%s : %s", msgPipelineNotFoundErr, err)
	}

	//Sort taskruns, to display the taskrun logs as per pipeline tasks order
	ordered := trh.SortTasksBySpecOrder(pl.Spec.Tasks, pr.Status.TaskRuns)
	taskRuns := trh.Filter(ordered, lr.tasks)

	logC := make(chan Log)
	errC := make(chan error)

	go func() {
		defer close(logC)
		defer close(errC)

		for i, tr := range taskRuns {
			tlr := tr.NewLogReader(
				lr.Ns, lr.Clients, lr.Streamer,
				i+1, lr.follow, lr.allSteps)

			pipeLogs(logC, errC, tlr)
		}
	}()

	return logC, errC, nil
}

func pipeLogs(logC chan<- Log, errC chan<- error, tlr *taskrun.LogReader) {
	tlogC, terrC, err := tlr.Read()
	if err != nil {
		errC <- err
		if _, ok := err.(*errors.WarningError); !ok {
			return
		}
	}

	for tlogC != nil || terrC != nil {
		select {
		case l, ok := <-tlogC:
			if !ok {
				tlogC = nil
				continue
			}
			logC <- Log{Task: l.Task, Step: l.Step, Log: l.Log}

		case e, ok := <-terrC:
			if !ok {
				terrC = nil
				continue
			}
			errC <- fmt.Errorf("failed to get logs for task %s : %s", tlr.Task, e)
		}
	}
}
