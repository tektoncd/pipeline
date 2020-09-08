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

	"github.com/google/go-cmp/cmp"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
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
func WaitForDeploymentState(client *KubeClient, name string, inState func(d *appsv1.Deployment) (bool, error), desc string, namespace string, timeout time.Duration) error {
	d := client.Kube.AppsV1().Deployments(namespace)
	span := logging.GetEmitableSpan(context.Background(), fmt.Sprintf("WaitForDeploymentState/%s/%s", name, desc))
	defer span.End()

	return wait.PollImmediate(interval, timeout, func() (bool, error) {
		d, err := d.Get(name, metav1.GetOptions{})
		if err != nil {
			return true, err
		}
		return inState(d)
	})
}

// WaitForPodListState polls the status of the PodList
// from client every interval until inState returns `true` indicating it
// is done, returns an error or timeout. desc will be used to name the metric
// that is emitted to track how long it took to get into the state checked by inState.
func WaitForPodListState(client *KubeClient, inState func(p *corev1.PodList) (bool, error), desc string, namespace string) error {
	p := client.Kube.CoreV1().Pods(namespace)
	span := logging.GetEmitableSpan(context.Background(), fmt.Sprintf("WaitForPodListState/%s", desc))
	defer span.End()

	return wait.PollImmediate(interval, podTimeout, func() (bool, error) {
		p, err := p.List(metav1.ListOptions{})
		if err != nil {
			return true, err
		}
		return inState(p)
	})
}

// WaitForPodState polls the status of the specified Pod
// from client every interval until inState returns `true` indicating it
// is done, returns an error or timeout. desc will be used to name the metric
// that is emitted to track how long it took to get into the state checked by inState.
func WaitForPodState(client *KubeClient, inState func(p *corev1.Pod) (bool, error), name string, namespace string) error {
	p := client.Kube.CoreV1().Pods(namespace)
	span := logging.GetEmitableSpan(context.Background(), "WaitForPodState/"+name)
	defer span.End()

	return wait.PollImmediate(interval, podTimeout, func() (bool, error) {
		p, err := p.Get(name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		return inState(p)
	})
}

// WaitForPodDeleted waits for the given pod to disappear from the given namespace.
func WaitForPodDeleted(client *KubeClient, name, namespace string) error {
	if err := WaitForPodState(client, func(p *corev1.Pod) (bool, error) {
		// Always return false. We're oly interested in the error which indicates pod deletion or timeout.
		return false, nil
	}, name, namespace); err != nil {
		if !apierrs.IsNotFound(err) {
			return err
		}
	}
	return nil
}

// WaitForServiceHasAtLeastOneEndpoint polls the status of the specified Service
// from client every interval until number of service endpoints = numOfEndpoints
func WaitForServiceEndpoints(client *KubeClient, svcName string, svcNamespace string, numOfEndpoints int) error {
	endpointsService := client.Kube.CoreV1().Endpoints(svcNamespace)
	span := logging.GetEmitableSpan(context.Background(), "WaitForServiceHasAtLeastOneEndpoint/"+svcName)
	defer span.End()

	return wait.PollImmediate(interval, podTimeout, func() (bool, error) {
		endpoint, err := endpointsService.Get(svcName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}

		return countEndpointsNum(endpoint) == numOfEndpoints, nil
	})
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
func GetEndpointAddresses(client *KubeClient, svcName, svcNamespace string) ([]string, error) {
	endpoints, err := client.Kube.CoreV1().Endpoints(svcNamespace).Get(svcName, metav1.GetOptions{})
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
func WaitForChangedEndpoints(client *KubeClient, svcName, svcNamespace string, origEndpoints []string) error {
	return wait.PollImmediate(1*time.Second, 2*time.Minute, func() (bool, error) {
		newEndpoints, err := GetEndpointAddresses(client, svcName, svcNamespace)
		return !cmp.Equal(origEndpoints, newEndpoints), err
	})
}

// GetConfigMap gets the configmaps for a given namespace
func GetConfigMap(client *KubeClient, namespace string) k8styped.ConfigMapInterface {
	return client.Kube.CoreV1().ConfigMaps(namespace)
}

// DeploymentScaledToZeroFunc returns a func that evaluates if a deployment has scaled to 0 pods
func DeploymentScaledToZeroFunc() func(d *appsv1.Deployment) (bool, error) {
	return func(d *appsv1.Deployment) (bool, error) {
		return d.Status.ReadyReplicas == 0, nil
	}
}

// WaitForLogContent waits until logs for given Pod/Container include the given content.
// If the content is not present within timeout it returns error.
func WaitForLogContent(client *KubeClient, podName, containerName, namespace, content string) error {
	return wait.PollImmediate(interval, logTimeout, func() (bool, error) {
		logs, err := client.PodLogs(podName, containerName, namespace)
		if err != nil {
			return true, err
		}
		return strings.Contains(string(logs), content), nil
	})
}

// WaitForAllPodsRunning waits for all the pods to be in running state
func WaitForAllPodsRunning(client *KubeClient, namespace string) error {
	return WaitForPodListState(client, podsRunning, "PodsAreRunning", namespace)
}

// WaitForPodRunning waits for the given pod to be in running state
func WaitForPodRunning(client *KubeClient, name string, namespace string) error {
	p := client.Kube.CoreV1().Pods(namespace)
	return wait.PollImmediate(interval, podTimeout, func() (bool, error) {
		p, err := p.Get(name, metav1.GetOptions{})
		if err != nil {
			return true, err
		}
		return podRunning(p), nil
	})
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
func WaitForDeploymentScale(client *KubeClient, name, namespace string, scale int) error {
	return WaitForDeploymentState(
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
