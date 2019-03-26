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
	statusMap               *sync.Map
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
		statusMap:         &sync.Map{},
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
	defer t.statusMap.Delete(key)
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

// StatusLock function acquires lock for taskrun/pipelinerun status key
func (t *TimeoutSet) StatusLock(runObj StatusKey) {
	m, _ := t.statusMap.LoadOrStore(runObj.GetRunKey(), &sync.Mutex{})
	mut := m.(*sync.Mutex)
	mut.Lock()
}

// StatusUnlock function releases lock for taskrun/pipelinerun status key
func (t *TimeoutSet) StatusUnlock(runObj StatusKey) {
	m, ok := t.statusMap.Load(runObj.GetRunKey())
	if !ok {
		return
	}
	mut := m.(*sync.Mutex)
	mut.Unlock()
}

func getTimeout(d *metav1.Duration) time.Duration {
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
		go t.WaitPipelineRun(&pipelineRun)
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
		go t.WaitTaskRun(&taskrun)
	}
}

// WaitTaskRun function creates a blocking function for taskrun to wait for
// 1. Stop signal, 2. TaskRun to complete or 3. Taskrun to time out
func (t *TimeoutSet) WaitTaskRun(tr *v1alpha1.TaskRun) {
	timeout := getTimeout(tr.Spec.Timeout)
	runtime := time.Duration(0)

	t.StatusLock(tr)
	if tr.Status.StartTime != nil && !tr.Status.StartTime.Time.IsZero() {
		runtime = time.Since(tr.Status.StartTime.Time)
	}
	t.StatusUnlock(tr)
	timeout -= runtime
	finished := t.getOrCreateFinishedChan(tr)

	defer t.Release(tr)

	select {
	case <-t.stopCh:
		// we're stopping, give up
		return
	case <-finished:
		// taskrun finished, we can stop watching
		return
	case <-time.After(timeout):
		t.StatusLock(tr)
		defer t.StatusUnlock(tr)
		if t.taskRunCallbackFunc != nil {
			t.taskRunCallbackFunc(tr)
		} else {
			defaultFunc(tr)
		}
	}
}

// WaitPipelineRun function creates a blocking function for pipelinerun to wait for
// 1. Stop signal, 2. pipelinerun to complete or 3. pipelinerun to time out
func (t *TimeoutSet) WaitPipelineRun(pr *v1alpha1.PipelineRun) {
	timeout := getTimeout(pr.Spec.Timeout)

	runtime := time.Duration(0)
	t.StatusLock(pr)
	if pr.Status.StartTime != nil && !pr.Status.StartTime.Time.IsZero() {
		runtime = time.Since(pr.Status.StartTime.Time)
	}
	t.StatusUnlock(pr)
	timeout -= runtime
	finished := t.getOrCreateFinishedChan(pr)

	defer t.Release(pr)

	select {
	case <-t.stopCh:
		// we're stopping, give up
		return
	case <-finished:
		// pipelinerun finished, we can stop watching
		return
	case <-time.After(timeout):
		t.StatusLock(pr)
		defer t.StatusUnlock(pr)
		if t.pipelineRunCallbackFunc != nil {
			t.pipelineRunCallbackFunc(pr)
		} else {
			defaultFunc(pr)
		}
	}
}
