/*
Copyright 2018 The Knative Authors.

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

package test

import (
	"context"
	"fmt"
	"time"

	"github.com/knative/build-pipeline/pkg/apis/pipeline/v1alpha1"
	"go.opencensus.io/trace"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

const (
	interval = 1 * time.Second
	// Currently using a super short timeout b/c tests are expected to fail so this way
	// we can get to that failure faster - knative/serving is currently using `6 * time.Minute`
	// which we could use, or we could use timeouts more specific to what each `Task` is
	// actually expected to do.
	timeout = 60 * time.Second
)

// WaitForTaskRunState polls the status of the TaskRun called name from client every
// interval until inState returns `true` indicating it is done, returns an
// error or timeout. desc will be used to name the metric that is emitted to
// track how long it took for name to get into the state checked by inState.
func WaitForTaskRunState(c *clients, name string, inState func(r *v1alpha1.TaskRun) (bool, error), desc string) error {
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

// WaitForTaskRunState polls the status of the TaskRun called name from client every
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
