/*
Copyright 2019 The Tekton Authors.

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

/*
Poll Pipeline resources

After creating Pipeline resources or making changes to them, you will need to
wait for the system to realize those changes. You can use polling methods to
check the resources reach the desired state.

The WaitFor* functions use the kubernetes
wait package (https://godoc.org/k8s.io/apimachinery/pkg/util/wait). To poll
they use
PollImmediate (https://godoc.org/k8s.io/apimachinery/pkg/util/wait#PollImmediate)
and the return values of the function you provide behave the same as
ConditionFunc (https://godoc.org/k8s.io/apimachinery/pkg/util/wait#ConditionFunc):
a boolean to indicate if the function should stop or continue polling, and an
error to indicate if there has been an error.


For example, you can poll a TaskRun object to wait for it to have a Status.Condition:

	err = WaitForTaskRunState(c, hwTaskRunName, func(tr *v1alpha1.TaskRun) (bool, error) {
		if len(tr.Status.Conditions) > 0 {
			return true, nil
		}
		return false, nil
	}, "TaskRunHasCondition")

*/
package test

import (
	"context"
	"fmt"
	"time"

	"github.com/knative/pkg/apis"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"go.opencensus.io/trace"
	"golang.org/x/xerrors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

const (
	interval = 1 * time.Second
	timeout  = 10 * time.Minute
)

// TaskRunStateFn is a condition function on TaskRun used polling functions
type TaskRunStateFn func(r *v1alpha1.TaskRun) (bool, error)

// PipelineRunStateFn is a condition function on TaskRun used polling functions
type PipelineRunStateFn func(pr *v1alpha1.PipelineRun) (bool, error)

// WaitForTaskRunState polls the status of the TaskRun called name from client every
// interval until inState returns `true` indicating it is done, returns an
// error or timeout. desc will be used to name the metric that is emitted to
// track how long it took for name to get into the state checked by inState.
func WaitForTaskRunState(c *clients, name string, inState TaskRunStateFn, desc string) error {
	metricName := fmt.Sprintf("WaitForTaskRunState/%s/%s", name, desc)
	_, span := trace.StartSpan(context.Background(), metricName)
	defer span.End()

	return wait.PollImmediate(interval, timeout, func() (bool, error) {
		r, err := c.TaskRunClient.Get(name, metav1.GetOptions{})
		if err != nil {
			return true, err
		}
		return inState(r)
	})
}

// WaitForPodState polls the status of the Pod called name from client every
// interval until inState returns `true` indicating it is done, returns an
// error or timeout. desc will be used to name the metric that is emitted to
// track how long it took for name to get into the state checked by inState.
func WaitForPodState(c *clients, name string, namespace string, inState func(r *corev1.Pod) (bool, error), desc string) error {
	metricName := fmt.Sprintf("WaitForPodState/%s/%s", name, desc)
	_, span := trace.StartSpan(context.Background(), metricName)
	defer span.End()

	return wait.PollImmediate(interval, timeout, func() (bool, error) {
		r, err := c.KubeClient.Kube.CoreV1().Pods(namespace).Get(name, metav1.GetOptions{})
		if err != nil {
			return true, err
		}
		return inState(r)
	})
}

// WaitForPipelineRunState polls the status of the PipelineRun called name from client every
// interval until inState returns `true` indicating it is done, returns an
// error or timeout. desc will be used to name the metric that is emitted to
// track how long it took for name to get into the state checked by inState.
func WaitForPipelineRunState(c *clients, name string, polltimeout time.Duration, inState PipelineRunStateFn, desc string) error {
	metricName := fmt.Sprintf("WaitForPipelineRunState/%s/%s", name, desc)
	_, span := trace.StartSpan(context.Background(), metricName)
	defer span.End()

	return wait.PollImmediate(interval, polltimeout, func() (bool, error) {
		r, err := c.PipelineRunClient.Get(name, metav1.GetOptions{})
		if err != nil {
			return true, err
		}
		return inState(r)
	})
}

// WaitForServiceExternalIPState polls the status of the a k8s Service called name from client every
// interval until an external ip is assigned indicating it is done, returns an
// error or timeout. desc will be used to name the metric that is emitted to
// track how long it took for name to get into the state checked by inState.
func WaitForServiceExternalIPState(c *clients, namespace, name string, inState func(s *corev1.Service) (bool, error), desc string) error {
	metricName := fmt.Sprintf("WaitForServiceExternalIPState/%s/%s", name, desc)
	_, span := trace.StartSpan(context.Background(), metricName)
	defer span.End()

	return wait.PollImmediate(interval, timeout, func() (bool, error) {
		r, err := c.KubeClient.Kube.CoreV1().Services(namespace).Get(name, metav1.GetOptions{})
		if err != nil {
			return true, err
		}
		return inState(r)
	})
}

// TaskRunSucceed provides a poll condition function that checks if the TaskRun
// has successfully completed.
func TaskRunSucceed(name string) TaskRunStateFn {
	return func(tr *v1alpha1.TaskRun) (bool, error) {
		c := tr.Status.GetCondition(apis.ConditionSucceeded)
		if c != nil {
			if c.Status == corev1.ConditionTrue {
				return true, nil
			} else if c.Status == corev1.ConditionFalse {
				return true, xerrors.Errorf("task run %s failed!", name)
			}
		}
		return false, nil
	}
}

// TaskRunFailed provides a poll condition function that checks if the TaskRun
// has failed.
func TaskRunFailed(name string) TaskRunStateFn {
	return func(tr *v1alpha1.TaskRun) (bool, error) {
		c := tr.Status.GetCondition(apis.ConditionSucceeded)
		if c != nil {
			if c.Status == corev1.ConditionTrue {
				return true, xerrors.Errorf("task run %s succeeded!", name)
			} else if c.Status == corev1.ConditionFalse {
				return true, nil
			}
		}
		return false, nil
	}
}

// PipelineRunSucceed provides a poll condition function that checks if the PipelineRun
// has successfully completed.
func PipelineRunSucceed(name string) PipelineRunStateFn {
	return func(pr *v1alpha1.PipelineRun) (bool, error) {
		c := pr.Status.GetCondition(apis.ConditionSucceeded)
		if c != nil {
			if c.Status == corev1.ConditionTrue {
				return true, nil
			} else if c.Status == corev1.ConditionFalse {
				return true, xerrors.Errorf("pipeline run %s failed!", name)
			}
		}
		return false, nil
	}
}

// PipelineRunFailed provides a poll condition function that checks if the PipelineRun
// has failed.
func PipelineRunFailed(name string) PipelineRunStateFn {
	return func(tr *v1alpha1.PipelineRun) (bool, error) {
		c := tr.Status.GetCondition(apis.ConditionSucceeded)
		if c != nil {
			if c.Status == corev1.ConditionTrue {
				return true, xerrors.Errorf("task run %s succeeded!", name)
			} else if c.Status == corev1.ConditionFalse {
				return true, nil
			}
		}
		return false, nil
	}
}
