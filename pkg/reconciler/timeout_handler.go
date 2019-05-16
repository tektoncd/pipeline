package reconciler

import (
	"sync"

	"time"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	clientset "github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

var (
	defaultFunc = func(i interface{}) {}
)

const (
	defaultTimeout = 10 * time.Minute
)

// StatusKey interface to be implemented by Taskrun Pipelinerun types
type StatusKey interface {
	GetRunKey() string
}

// TimeoutSet contains required k8s interfaces to handle build timeouts
type TimeoutSet struct {
	logger                  *zap.SugaredLogger
	kubeclientset           kubernetes.Interface
	pipelineclientset       clientset.Interface
	taskRunCallbackFunc     func(interface{})
	pipelineRunCallbackFunc func(interface{})
	stopCh                  <-chan struct{}
	done                    map[string]chan bool
	doneMut                 sync.Mutex
}

// NewTimeoutHandler returns TimeoutSet filled structure
func NewTimeoutHandler(
	kubeclientset kubernetes.Interface,
	pipelineclientset clientset.Interface,
	stopCh <-chan struct{},
	logger *zap.SugaredLogger,
) *TimeoutSet {
	return &TimeoutSet{
		kubeclientset:     kubeclientset,
		pipelineclientset: pipelineclientset,
		stopCh:            stopCh,
		done:              make(map[string]chan bool),
		doneMut:           sync.Mutex{},
		logger:            logger,
	}
}

// SetTaskRunCallbackFunc sets the callback function when timeout occurs for taskrun objects
func (t *TimeoutSet) SetTaskRunCallbackFunc(f func(interface{})) {
	t.taskRunCallbackFunc = f
}

// SetPipelineRunCallbackFunc sets the callback function when timeout occurs for pipelinerun objects
func (t *TimeoutSet) SetPipelineRunCallbackFunc(f func(interface{})) {
	t.pipelineRunCallbackFunc = f
}

// Release function deletes key from timeout map
func (t *TimeoutSet) Release(runObj StatusKey) {
	key := runObj.GetRunKey()
	t.doneMut.Lock()
	defer t.doneMut.Unlock()

	if finished, ok := t.done[key]; ok {
		delete(t.done, key)
		close(finished)
	}
}

func (t *TimeoutSet) getOrCreateFinishedChan(runObj StatusKey) chan bool {
	var finished chan bool
	key := runObj.GetRunKey()
	t.doneMut.Lock()
	defer t.doneMut.Unlock()
	if existingfinishedChan, ok := t.done[key]; ok {
		finished = existingfinishedChan
	} else {
		finished = make(chan bool)
	}
	t.done[key] = finished
	return finished
}

// GetTimeout takes a kubernetes Duration representing the timeout period for a
// resource and returns it as a time.Duration. If the provided duration is nil
// then fallback behaviour is to return a default timeout period.
func GetTimeout(d *metav1.Duration) time.Duration {
	timeout := defaultTimeout
	if d != nil {
		timeout = d.Duration
	}
	return timeout
}

// checkPipelineRunTimeouts function creates goroutines to wait for pipelinerun to
// finish/timeout in a given namespace
func (t *TimeoutSet) checkPipelineRunTimeouts(namespace string) {
	pipelineRuns, err := t.pipelineclientset.TektonV1alpha1().PipelineRuns(namespace).List(metav1.ListOptions{})
	if err != nil {
		t.logger.Errorf("Can't get pipelinerun list in namespace %s: %s", namespace, err)
		return
	}
	for _, pipelineRun := range pipelineRuns.Items {
		pipelineRun := pipelineRun
		if pipelineRun.IsDone() || pipelineRun.IsCancelled() {
			continue
		}
		if pipelineRun.HasStarted() {
			go t.WaitPipelineRun(&pipelineRun, pipelineRun.Status.StartTime)
		}
	}
}

// CheckTimeouts function iterates through all namespaces and calls corresponding
// taskrun/pipelinerun timeout functions
func (t *TimeoutSet) CheckTimeouts() {
	namespaces, err := t.kubeclientset.CoreV1().Namespaces().List(metav1.ListOptions{})
	if err != nil {
		t.logger.Errorf("Can't get namespaces list: %s", err)
		return
	}
	for _, namespace := range namespaces.Items {
		t.checkTaskRunTimeouts(namespace.GetName())
		t.checkPipelineRunTimeouts(namespace.GetName())
	}
}

// checkTaskRunTimeouts function creates goroutines to wait for pipelinerun to
// finish/timeout in a given namespace
func (t *TimeoutSet) checkTaskRunTimeouts(namespace string) {
	taskruns, err := t.pipelineclientset.TektonV1alpha1().TaskRuns(namespace).List(metav1.ListOptions{})
	if err != nil {
		t.logger.Errorf("Can't get taskrun list in namespace %s: %s", namespace, err)
		return
	}
	for _, taskrun := range taskruns.Items {
		taskrun := taskrun
		if taskrun.IsDone() || taskrun.IsCancelled() {
			continue
		}
		if taskrun.HasStarted() {
			go t.WaitTaskRun(&taskrun, taskrun.Status.StartTime)
		}
	}
}

// WaitTaskRun function creates a blocking function for taskrun to wait for
// 1. Stop signal, 2. TaskRun to complete or 3. Taskrun to time out, which is
// determined by checking if the tr's timeout has occurred since the startTime
func (t *TimeoutSet) WaitTaskRun(tr *v1alpha1.TaskRun, startTime *metav1.Time) {
	t.waitRun(tr, GetTimeout(tr.Spec.Timeout), startTime, t.taskRunCallbackFunc)
}

// WaitPipelineRun function creates a blocking function for pipelinerun to wait for
// 1. Stop signal, 2. pipelinerun to complete or 3. pipelinerun to time out which is
// determined by checking if the tr's timeout has occurred since the startTime
func (t *TimeoutSet) WaitPipelineRun(pr *v1alpha1.PipelineRun, startTime *metav1.Time) {
	t.waitRun(pr, GetTimeout(pr.Spec.Timeout), startTime, t.pipelineRunCallbackFunc)
}

func (t *TimeoutSet) waitRun(runObj StatusKey, timeout time.Duration, startTime *metav1.Time, callback func(interface{})) {
	if startTime == nil {
		t.logger.Errorf("startTime must be specified in order for a timeout to be calculated accurately for %s", runObj.GetRunKey())
		return
	}
	runtime := time.Since(startTime.Time)
	finished := t.getOrCreateFinishedChan(runObj)

	defer t.Release(runObj)

	t.logger.Infof("About to start timeout timer for %s. started at %s, timeout is %s, running for %s", runObj.GetRunKey(), startTime.Time, timeout, runtime)

	select {
	case <-t.stopCh:
		t.logger.Infof("Stopping timeout timer for %s", runObj.GetRunKey())
		return
	case <-finished:
		t.logger.Infof("%s finished, stopping the timeout timer", runObj.GetRunKey())
		return
	case <-time.After(timeout - runtime):
		t.logger.Infof("Timeout timer for %s has timed out (started at %s, timeout is %s, running for %s", runObj.GetRunKey(), startTime, timeout, time.Since(startTime.Time))
		if callback != nil {
			callback(runObj)
		} else {
			defaultFunc(runObj)
		}
	}
}
