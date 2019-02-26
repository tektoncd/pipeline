package reconciler

import (
	"sync"

	"fmt"
	"time"

	clientset "github.com/knative/build-pipeline/pkg/client/clientset/versioned"
	"go.uber.org/zap"

	"github.com/knative/build-pipeline/pkg/apis/pipeline/v1alpha1"
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	defaultTimeout = 10 * time.Minute
)

var (
	done    = make(map[string]chan bool)
	doneMut = sync.Mutex{}
	// TimedOut indicates that the TaskRun/PipelineRun has taken longer than its configured timeout
	TimedOut = "Timeout"
)

// TimeoutSet contains required k8s interfaces to handle build timeouts
type TimeoutSet struct {
	logger            *zap.SugaredLogger
	kubeclientset     kubernetes.Interface
	pipelineclientset clientset.Interface
	stopCh            <-chan struct{}
	statusMap         *sync.Map
}

// NewTimeoutHandler returns TimeoutSet filled structure
func NewTimeoutHandler(logger *zap.SugaredLogger,
	kubeclientset kubernetes.Interface,
	pipelineclientset clientset.Interface,
	stopCh <-chan struct{}) *TimeoutSet {
	return &TimeoutSet{
		logger:            logger,
		kubeclientset:     kubeclientset,
		pipelineclientset: pipelineclientset,
		stopCh:            stopCh,
		statusMap:         &sync.Map{},
	}
}

func (t *TimeoutSet) Release(key string) {
	doneMut.Lock()
	defer doneMut.Unlock()
	if finished, ok := done[key]; ok {
		delete(done, key)
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

func (t *TimeoutSet) WaitTaskRun(tr *v1alpha1.TaskRun) {
	key := getTaskrunKey(tr.Namespace, tr.Name)
	timeout := defaultTimeout
	if tr.Spec.Timeout != nil {
		timeout = tr.Spec.Timeout.Duration
	}
	originalTimeout := timeout
	runtime := time.Duration(0)

	t.StatusLock(key)
	if tr.Status.StartTime != nil && !tr.Status.StartTime.Time.IsZero() {
		runtime = time.Since(tr.Status.StartTime.Time)
	}
	t.StatusUnlock(key)
	timeout -= runtime

	var finished chan bool
	doneMut.Lock()
	if existingfinishedChan, ok := done[key]; ok {
		finished = existingfinishedChan
	} else {
		finished = make(chan bool)
	}
	done[key] = finished
	doneMut.Unlock()
	defer t.Release(key)

	select {
	case <-t.stopCh:
	case <-finished:
	case <-time.After(timeout):
		if err := t.stopTaskRun(tr.Name, tr.Namespace, originalTimeout); err != nil {
			t.logger.Errorf("Can't stop taskrun %q pod after timeout: %s", tr.Name, err)
		}
	}
}

func (t *TimeoutSet) WaitPipelineRun(pr *v1alpha1.PipelineRun) {
	key := getPipelinerunKey(pr.Namespace, pr.Name)
	timeout := defaultTimeout
	if pr.Spec.Timeout != nil {
		timeout = pr.Spec.Timeout.Duration
	}
	originalTimeout := timeout
	runtime := time.Duration(0)
	t.StatusLock(key)
	if pr.Status.StartTime != nil && !pr.Status.StartTime.Time.IsZero() {
		runtime = time.Since(pr.Status.StartTime.Time)
	}
	t.StatusUnlock(key)
	timeout -= runtime

	var finished chan bool
	doneMut.Lock()
	if existingfinishedChan, ok := done[key]; ok {
		finished = existingfinishedChan
	} else {
		finished = make(chan bool)
	}
	done[key] = finished
	doneMut.Unlock()
	defer t.Release(key)

	select {
	case <-t.stopCh:
	case <-finished:
	case <-time.After(timeout):
		if err := t.stopPipelineRun(pr, originalTimeout); err != nil {
			t.logger.Errorf("Can't stop PipelineRun %q after timeout: %s", pr.Name, err)
		}
	}
}

func (t *TimeoutSet) stopPipelineRun(pr *v1alpha1.PipelineRun, timeout time.Duration) error {
	var errors []error
	key := getPipelinerunKey(pr.Namespace, pr.Name)
	t.StatusLock(key)
	defer t.StatusUnlock(key)

	pr.Status.SetCondition(&duckv1alpha1.Condition{
		Type:    duckv1alpha1.ConditionSucceeded,
		Status:  corev1.ConditionFalse,
		Reason:  TimedOut,
		Message: fmt.Sprintf("PipelineRun %q failed to finish within %q", pr.Name, timeout.String()),
	})
	pr.Status.CompletionTime = &metav1.Time{Time: time.Now()}

	newPr, err := t.pipelineclientset.TektonV1alpha1().PipelineRuns(pr.Namespace).Get(pr.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	newPr.Status = pr.Status
	if _, err = t.pipelineclientset.TektonV1alpha1().PipelineRuns(pr.Namespace).UpdateStatus(newPr); err != nil {
		errors = append(errors, err)
	}

	for taskRunName := range pr.Status.TaskRuns {
		if taskrunErr := t.stopTaskRun(taskRunName, pr.Namespace, timeout); taskrunErr != nil {
			errors = append(errors, taskrunErr)
		}
	}
	return fmt.Errorf("Error stopping Pipelinerun %s", errors)
}

func (t *TimeoutSet) stopTaskRun(taskrunName, taskrunNamespace string, timeout time.Duration) error {
	key := getTaskrunKey(taskrunNamespace, taskrunName)

	t.StatusLock(key)
	defer t.StatusUnlock(key)

	newTaskRun, err := t.pipelineclientset.TektonV1alpha1().TaskRuns(taskrunNamespace).Get(taskrunName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	if newTaskRun.Status.PodName != "" {
		if err := t.kubeclientset.CoreV1().Pods(taskrunNamespace).Delete(newTaskRun.Status.PodName, &metav1.DeleteOptions{}); err != nil && !errors.IsNotFound(err) {
			return err
		}
	}

	newTaskRun.Status.SetCondition(&duckv1alpha1.Condition{
		Type:    duckv1alpha1.ConditionSucceeded,
		Status:  corev1.ConditionFalse,
		Reason:  TimedOut,
		Message: fmt.Sprintf("TaskRun %q failed to finish within %q", taskrunName, timeout.String()),
	})

	newTaskRun.Status.CompletionTime = &metav1.Time{Time: time.Now()}

	_, taskRunErr := t.pipelineclientset.TektonV1alpha1().TaskRuns(taskrunNamespace).UpdateStatus(newTaskRun)
	return taskRunErr
}

func getTaskrunKey(ns, name string) string {
	return fmt.Sprintf("%s/%s/%s", "TaskRun", ns, name)
}

func getPipelinerunKey(ns, name string) string {
	return fmt.Sprintf("%s/%s/%s", "PipelineRun", ns, name)
}
