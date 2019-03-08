package reconciler

import (
	"sync"

	"fmt"
	"time"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	clientset "github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

var (
	defaultFunc = func(i interface{}, l *zap.SugaredLogger) {}
)

const (
	defaultTimeout = 10 * time.Minute
)

// TimeoutSet contains required k8s interfaces to handle build timeouts
type TimeoutSet struct {
	logger                  *zap.SugaredLogger
	kubeclientset           kubernetes.Interface
	pipelineclientset       clientset.Interface
	taskRuncallbackFunc     func(interface{})
	pipelineruncallbackFunc func(interface{})
	stopCh                  <-chan struct{}
	statusMap               *sync.Map
	done                    map[string]chan bool
	doneMut                 sync.Mutex
}

// NewTimeoutHandler returns TimeoutSet filled structure
func NewTimeoutHandler(
	logger *zap.SugaredLogger,
	kubeclientset kubernetes.Interface,
	pipelineclientset clientset.Interface,
	stopCh <-chan struct{},
) *TimeoutSet {
	return &TimeoutSet{
		logger:            logger,
		kubeclientset:     kubeclientset,
		pipelineclientset: pipelineclientset,
		stopCh:            stopCh,
		statusMap:         &sync.Map{},
		done:              make(map[string]chan bool),
		doneMut:           sync.Mutex{},
	}
}

func (t *TimeoutSet) AddtrCallBackFunc(f func(interface{})) {
	t.taskRuncallbackFunc = f
}

func (t *TimeoutSet) AddPrCallBackFunc(f func(interface{})) {
	t.pipelineruncallbackFunc = f
}

func (t *TimeoutSet) Release(key string) {
	t.doneMut.Lock()
	defer t.doneMut.Unlock()
	if finished, ok := t.done[key]; ok {
		delete(t.done, key)
		close(finished)
	}
}

func (t *TimeoutSet) StatusLock(key string) {
	m, _ := t.statusMap.LoadOrStore(key, &sync.Mutex{})
	mut := m.(*sync.Mutex)
	mut.Lock()
}

func (t *TimeoutSet) StatusUnlock(key string) {
	m, ok := t.statusMap.Load(key)
	if !ok {
		return
	}
	mut := m.(*sync.Mutex)
	mut.Unlock()
}

func (t *TimeoutSet) ReleaseKey(key string) {
	t.statusMap.Delete(key)
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
		t.logger.Errorf("Can't get taskruns list in namespace %s: %s", namespace, err)
	}
	for _, pipelineRun := range pipelineRuns.Items {
		pipelineRun := pipelineRun
		if pipelineRun.Status.IsDone() {
			continue
		}
		if pipelineRun.Spec.IsCancelled() {
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
		t.logger.Errorf("Can't get taskruns list in namespace %s: %s", namespace, err)
	}
	for _, taskrun := range taskruns.Items {
		taskrun := taskrun
		if taskrun.Status.IsDone() {
			continue
		}
		if taskrun.Spec.IsCancelled() {
			continue
		}
		go t.WaitTaskRun(&taskrun)
	}
}

// WaitTaskRun function creates a blocking function for taskrun to wait for
// 1. Stop signal, 2. TaskRun to complete or 3. Taskrun to time out
func (t *TimeoutSet) WaitTaskRun(tr *v1alpha1.TaskRun) {
	key := fmt.Sprintf("%s/%s/%s", "TaskRun", tr.Namespace, tr.Name)

	timeout := getTimeout(tr.Spec.Timeout)
	runtime := time.Duration(0)

	t.StatusLock(key)
	if tr.Status.StartTime != nil && !tr.Status.StartTime.Time.IsZero() {
		runtime = time.Since(tr.Status.StartTime.Time)
	}
	t.StatusUnlock(key)
	timeout -= runtime

	var finished chan bool
	t.doneMut.Lock()
	if existingfinishedChan, ok := t.done[key]; ok {
		finished = existingfinishedChan
	} else {
		finished = make(chan bool)
	}
	t.done[key] = finished
	t.doneMut.Unlock()

	defer t.Release(key)

	select {
	case <-t.stopCh:
	case <-finished:
	case <-time.After(timeout):
		t.StatusLock(key)
		if t.taskRuncallbackFunc == nil {
			defaultFunc(tr, t.logger)
		} else {
			t.taskRuncallbackFunc(tr)
		}
		t.StatusUnlock(key)
	}
}

// WaitPipelineRun function creates a blocking function for pipelinerun to wait for
// 1. Stop signal, 2. pipelinerun to complete or 3. pipelinerun to time out
func (t *TimeoutSet) WaitPipelineRun(pr *v1alpha1.PipelineRun) {
	key := fmt.Sprintf("%s/%s/%s", "PipelineRun", pr.Namespace, pr.Name)
	timeout := getTimeout(pr.Spec.Timeout)

	runtime := time.Duration(0)
	t.StatusLock(key)
	if pr.Status.StartTime != nil && !pr.Status.StartTime.Time.IsZero() {
		runtime = time.Since(pr.Status.StartTime.Time)
	}
	t.StatusUnlock(key)
	timeout -= runtime

	var finished chan bool
	t.doneMut.Lock()
	if existingfinishedChan, ok := t.done[key]; ok {
		finished = existingfinishedChan
	} else {
		finished = make(chan bool)
	}
	t.done[key] = finished
	t.doneMut.Unlock()
	defer t.Release(key)

	select {
	case <-t.stopCh:
	case <-finished:
	case <-time.After(timeout):
		t.StatusLock(key)
		if t.pipelineruncallbackFunc == nil {
			defaultFunc(pr, t.logger)
		} else {
			t.pipelineruncallbackFunc(pr)
		}
		t.StatusUnlock(key)
	}
}
