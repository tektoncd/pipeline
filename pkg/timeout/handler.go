/*
Copyright 2020 The Tekton Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package timeout

import (
	"math"

	"time"

	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	clientset "github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	maxBackoffSeconds = 120
)

var (
	defaultFunc        = func(i interface{}) {}
	maxBackoffExponent = math.Ceil(math.Log2(maxBackoffSeconds))
)

// StatusKey interface to be implemented by Taskrun Pipelinerun types
type StatusKey interface {
	GetRunKey() string
}

// Handler knows how to track channels that can be used to communicate with go routines
// which timeout, and the functions to call when that happens.
type Handler struct {
	logger *zap.SugaredLogger

	// taskRunCallbackFunc is the function to call when a TaskRun has timed out.
	// This is usually set to the function that enqueues the taskRun for reconciling.
	taskRunCallbackFunc func(interface{})
	// pipelineRunCallbackFunc is the function to call when a PipelineRun has timed out
	// This is usually set to the function that enqueues the pipelineRun for reconciling.
	pipelineRunCallbackFunc func(interface{})
	// timer is used to start timers in separate goroutines
	timer *Timer
}

// NewHandler returns an instance of Handler with the specified stopCh and logger, instantiated
// and ready to track go routines.
func NewHandler(
	stopCh <-chan struct{},
	logger *zap.SugaredLogger,
) *Handler {
	return &Handler{
		timer:  NewTimer(stopCh, logger),
		logger: logger,
	}
}

// SetTaskRunCallbackFunc sets the callback function when timeout occurs for taskrun objects
func (t *Handler) SetTaskRunCallbackFunc(f func(interface{})) {
	t.taskRunCallbackFunc = f
}

// SetPipelineRunCallbackFunc sets the callback function when timeout occurs for pipelinerun objects
func (t *Handler) SetPipelineRunCallbackFunc(f func(interface{})) {
	t.pipelineRunCallbackFunc = f
}

// Release deletes channels and data that are specific to a StatusKey object.
func (t *Handler) Release(runObj StatusKey) {
	t.timer.Release(runObj)
}

// checkPipelineRunTimeouts function creates goroutines to wait for pipelinerun to
// finish/timeout in a given namespace
func (t *Handler) checkPipelineRunTimeouts(namespace string, pipelineclientset clientset.Interface) {
	pipelineRuns, err := pipelineclientset.TektonV1beta1().PipelineRuns(namespace).List(metav1.ListOptions{})
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

// CheckTimeouts function iterates through a given namespace or all namespaces
// (if empty string) and calls corresponding taskrun/pipelinerun timeout functions
func (t *Handler) CheckTimeouts(namespace string, kubeclientset kubernetes.Interface, pipelineclientset clientset.Interface) {
	// scoped namespace
	namespaceNames := []string{namespace}
	// all namespaces
	if namespace == "" {
		namespaces, err := kubeclientset.CoreV1().Namespaces().List(metav1.ListOptions{})
		if err != nil {
			t.logger.Errorf("Can't get namespaces list: %s", err)
			return
		}
		namespaceNames = make([]string, len(namespaces.Items))
		for i, namespace := range namespaces.Items {
			namespaceNames[i] = namespace.GetName()
		}
	}

	for _, namespace := range namespaceNames {
		t.checkTaskRunTimeouts(namespace, pipelineclientset)
		t.checkPipelineRunTimeouts(namespace, pipelineclientset)
	}
}

// checkTaskRunTimeouts function creates goroutines to wait for pipelinerun to
// finish/timeout in a given namespace
func (t *Handler) checkTaskRunTimeouts(namespace string, pipelineclientset clientset.Interface) {
	taskruns, err := pipelineclientset.TektonV1beta1().TaskRuns(namespace).List(metav1.ListOptions{})
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

// timeoutFromSpec will return the default timeout minutes if the timeout is not
// set; however this is probably overly cautious because the taskrun SetDefaults logic
// will ensure this value is set; in addition to that, it will use the default that is
// provided to the controller via a config map which this logic is not doing.
func timeoutFromSpec(timeout *metav1.Duration) time.Duration {
	if timeout == nil {
		return config.DefaultTimeoutMinutes * time.Minute
	}
	return timeout.Duration
}

// WaitTaskRun function creates a blocking function for taskrun to wait for
// 1. Stop signal, 2. TaskRun to complete or 3. Taskrun to time out, which is
// determined by checking if the tr's timeout has occurred since the startTime
func (t *Handler) WaitTaskRun(tr *v1beta1.TaskRun, startTime *metav1.Time) {
	t.waitRun(tr, timeoutFromSpec(tr.Spec.Timeout), startTime, t.taskRunCallbackFunc)
}

// WaitPipelineRun function creates a blocking function for pipelinerun to wait for
// 1. Stop signal, 2. pipelinerun to complete or 3. pipelinerun to time out which is
// determined by checking if the tr's timeout has occurred since the startTime
func (t *Handler) WaitPipelineRun(pr *v1beta1.PipelineRun, startTime *metav1.Time) {
	t.waitRun(pr, timeoutFromSpec(pr.Spec.Timeout), startTime, t.pipelineRunCallbackFunc)
}

func (t *Handler) waitRun(runObj StatusKey, timeout time.Duration, startTime *metav1.Time, callback func(interface{})) {
	if startTime == nil {
		t.logger.Errorf("startTime must be specified in order for a timeout to be calculated accurately for %s", runObj.GetRunKey())
		return
	}
	if callback == nil {
		callback = defaultFunc
	}
	runtime := time.Since(startTime.Time)
	t.logger.Infof("About to start timeout timer for %s. started at %s, timeout is %s, running for %s", runObj.GetRunKey(), startTime.Time, timeout, runtime)
	defer t.Release(runObj)
	t.timer.SetTimer(runObj, timeout-runtime, callback)
}
