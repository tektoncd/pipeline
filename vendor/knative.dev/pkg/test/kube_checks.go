/*
Copyright 2018 The Knative Authors

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

// kube_checks contains functions which poll Kubernetes objects until
// they get into the state desired by the caller or time out.

package test

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/google/go-cmp/cmp"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	k8styped "k8s.io/client-go/kubernetes/typed/core/v1"
	"knative.dev/pkg/test/logging"
)

const (
	interval   = 1 * time.Second
	podTimeout = 8 * time.Minute
	logTimeout = 1 * time.Minute
)

// WaitForDeploymentState polls the status of the Deployment called name
// from client every interval until inState returns `true` indicating it
// is done, returns an error or timeout. desc will be used to name the metric
// that is emitted to track how long it took for name to get into the state checked by inState.
func WaitForDeploymentState(ctx context.Context, client kubernetes.Interface, name string, inState func(d *appsv1.Deployment) (bool, error), desc string, namespace string, timeout time.Duration) error {
	d := client.AppsV1().Deployments(namespace)
	span := logging.GetEmitableSpan(ctx, fmt.Sprintf("WaitForDeploymentState/%s/%s", name, desc))
	defer span.End()
	var lastState *appsv1.Deployment
	waitErr := wait.PollImmediate(interval, timeout, func() (bool, error) {
		var err error
		lastState, err = d.Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return true, err
		}
		return inState(lastState)
	})

	if waitErr != nil {
		return fmt.Errorf("deployment %q is not in desired state, got: %s: %w", name, spew.Sprint(lastState), waitErr)
	}
	return nil
}

// WaitForPodListState polls the status of the PodList
// from client every interval until inState returns `true` indicating it
// is done, returns an error or timeout. desc will be used to name the metric
// that is emitted to track how long it took to get into the state checked by inState.
func WaitForPodListState(ctx context.Context, client kubernetes.Interface, inState func(p *corev1.PodList) (bool, error), desc string, namespace string) error {
	p := client.CoreV1().Pods(namespace)
	span := logging.GetEmitableSpan(ctx, "WaitForPodListState/"+desc)
	defer span.End()

	var lastState *corev1.PodList
	waitErr := wait.PollImmediate(interval, podTimeout, func() (bool, error) {
		var err error
		lastState, err = p.List(ctx, metav1.ListOptions{})
		if err != nil {
			return true, err
		}
		return inState(lastState)
	})

	if waitErr != nil {
		return fmt.Errorf("pod list is not in desired state, got: %s: %w", spew.Sprint(lastState), waitErr)
	}
	return nil
}

// WaitForPodState polls the status of the specified Pod
// from client every interval until inState returns `true` indicating it
// is done, returns an error or timeout. desc will be used to name the metric
// that is emitted to track how long it took to get into the state checked by inState.
func WaitForPodState(ctx context.Context, client kubernetes.Interface, inState func(p *corev1.Pod) (bool, error), name string, namespace string) error {
	p := client.CoreV1().Pods(namespace)
	span := logging.GetEmitableSpan(ctx, "WaitForPodState/"+name)
	defer span.End()

	var lastState *corev1.Pod
	waitErr := wait.PollImmediate(interval, podTimeout, func() (bool, error) {
		var err error
		lastState, err = p.Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		return inState(lastState)
	})

	if waitErr != nil {
		return fmt.Errorf("pod %q is not in desired state, got: %s: %w", name, spew.Sprint(lastState), waitErr)
	}
	return nil
}

// WaitForPodDeleted waits for the given pod to disappear from the given namespace.
func WaitForPodDeleted(ctx context.Context, client kubernetes.Interface, name, namespace string) error {
	if err := WaitForPodState(ctx, client, func(p *corev1.Pod) (bool, error) {
		// Always return false. We're oly interested in the error which indicates pod deletion or timeout.
		return false, nil
	}, name, namespace); err != nil {
		if !apierrs.IsNotFound(err) {
			return err
		}
	}
	return nil
}

// WaitForServiceEndpoints polls the status of the specified Service
// from client every interval until number of service endpoints = numOfEndpoints
func WaitForServiceEndpoints(ctx context.Context, client kubernetes.Interface, svcName string, svcNamespace string, numOfEndpoints int) error {
	endpointsService := client.CoreV1().Endpoints(svcNamespace)
	span := logging.GetEmitableSpan(ctx, "WaitForServiceHasAtLeastOneEndpoint/"+svcName)
	defer span.End()

	var endpoints *corev1.Endpoints
	waitErr := wait.PollImmediate(interval, podTimeout, func() (bool, error) {
		var err error
		endpoints, err = endpointsService.Get(ctx, svcName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}

		return countEndpointsNum(endpoints) == numOfEndpoints, nil
	})
	if waitErr != nil {
		return fmt.Errorf("did not reach the desired number of endpoints, got: %d: %w", countEndpointsNum(endpoints), waitErr)
	}
	return nil
}

func countEndpointsNum(e *corev1.Endpoints) int {
	if e == nil || e.Subsets == nil {
		return 0
	}
	num := 0
	for _, sub := range e.Subsets {
		num += len(sub.Addresses)
	}
	return num
}

// GetEndpointAddresses returns addresses of endpoints for the given service.
func GetEndpointAddresses(ctx context.Context, client kubernetes.Interface, svcName, svcNamespace string) ([]string, error) {
	endpoints, err := client.CoreV1().Endpoints(svcNamespace).Get(ctx, svcName, metav1.GetOptions{})
	if err != nil || countEndpointsNum(endpoints) == 0 {
		return nil, fmt.Errorf("no endpoints or error: %w", err)
	}
	var hosts []string
	for _, sub := range endpoints.Subsets {
		for _, addr := range sub.Addresses {
			hosts = append(hosts, addr.IP)
		}
	}
	return hosts, nil
}

// WaitForChangedEndpoints waits until the endpoints for the given service differ from origEndpoints.
func WaitForChangedEndpoints(ctx context.Context, client kubernetes.Interface, svcName, svcNamespace string, origEndpoints []string) error {
	var newEndpoints []string
	waitErr := wait.PollImmediate(1*time.Second, 2*time.Minute, func() (bool, error) {
		var err error
		newEndpoints, err = GetEndpointAddresses(ctx, client, svcName, svcNamespace)
		return !cmp.Equal(origEndpoints, newEndpoints), err
	})
	if waitErr != nil {
		return fmt.Errorf("new endpoints are not different from the original ones, got %q: %w", newEndpoints, waitErr)
	}
	return nil
}

// GetConfigMap gets the configmaps for a given namespace
func GetConfigMap(client kubernetes.Interface, namespace string) k8styped.ConfigMapInterface {
	return client.CoreV1().ConfigMaps(namespace)
}

// DeploymentScaledToZeroFunc returns a func that evaluates if a deployment has scaled to 0 pods
func DeploymentScaledToZeroFunc() func(d *appsv1.Deployment) (bool, error) {
	return func(d *appsv1.Deployment) (bool, error) {
		return d.Status.ReadyReplicas == 0, nil
	}
}

// WaitForLogContent waits until logs for given Pod/Container include the given content.
// If the content is not present within timeout it returns error.
func WaitForLogContent(ctx context.Context, client kubernetes.Interface, podName, containerName, namespace, content string) error {
	var logs []byte
	waitErr := wait.PollImmediate(interval, logTimeout, func() (bool, error) {
		var err error
		logs, err = PodLogs(ctx, client, podName, containerName, namespace)
		if err != nil {
			return true, err
		}
		return strings.Contains(string(logs), content), nil
	})
	if waitErr != nil {
		return fmt.Errorf("logs do not contain the desired content %q, got %q: %w", content, logs, waitErr)
	}
	return nil
}

// WaitForAllPodsRunning waits for all the pods to be in running state
func WaitForAllPodsRunning(ctx context.Context, client kubernetes.Interface, namespace string) error {
	return WaitForPodListState(ctx, client, podsRunning, "PodsAreRunning", namespace)
}

// WaitForPodRunning waits for the given pod to be in running state
func WaitForPodRunning(ctx context.Context, client kubernetes.Interface, name string, namespace string) error {
	var p *corev1.Pod
	pods := client.CoreV1().Pods(namespace)
	waitErr := wait.PollImmediate(interval, podTimeout, func() (bool, error) {
		var err error
		p, err = pods.Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return true, err
		}
		return podRunning(p), nil
	})
	if waitErr != nil {
		return fmt.Errorf("pod %q did not reach the running state, got %+v: %w", name, p.Status.Phase, waitErr)
	}
	return nil
}

// podsRunning will check the status conditions of the pod list and return true all pods are Running
func podsRunning(podList *corev1.PodList) (bool, error) {
	// Pods are big, so use indexing, to avoid copying.
	for i := range podList.Items {
		if isRunning := podRunning(&podList.Items[i]); !isRunning {
			return false, nil
		}
	}
	return true, nil
}

// podRunning will check the status conditions of the pod and return true if it's Running.
func podRunning(pod *corev1.Pod) bool {
	return pod.Status.Phase == corev1.PodRunning || pod.Status.Phase == corev1.PodSucceeded
}

// WaitForDeploymentScale waits until the given deployment has the expected scale.
func WaitForDeploymentScale(ctx context.Context, client kubernetes.Interface, name, namespace string, scale int) error {
	return WaitForDeploymentState(
		ctx,
		client,
		name,
		func(d *appsv1.Deployment) (bool, error) {
			return d.Status.ReadyReplicas == int32(scale), nil
		},
		"DeploymentIsScaled",
		namespace,
		time.Minute,
	)
}
