/*
Copyright 2019 The Tekton Authors

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
	"io"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"knative.dev/pkg/test/logging"
)

// CollectPodLogs will get the logs for all containers in a Pod
func CollectPodLogs(ctx context.Context, c *clients, podName, namespace string, logf logging.FormatLogger) {
	logs, err := getContainersLogsFromPod(ctx, c.KubeClient, podName, namespace)
	if err != nil {
		logf("Could not get logs for pod %s: %s", podName, err)
	}
	logf("build logs %s", logs)
}

func getContainersLogsFromPod(ctx context.Context, c kubernetes.Interface, pod, namespace string) (string, error) {
	p, err := c.CoreV1().Pods(namespace).Get(ctx, pod, metav1.GetOptions{})
	if err != nil {
		return "", err
	}

	sb := strings.Builder{}
	for _, container := range p.Spec.Containers {
		sb.WriteString(fmt.Sprintf("\n>>> Pod %s Container %s:\n", p.Name, container.Name))
		logs, err := getContainerLogsFromPod(ctx, c, pod, container.Name, namespace)
		if err != nil {
			return "", err
		}
		sb.WriteString(logs)
	}
	return sb.String(), nil
}

func getContainerLogsFromPod(ctx context.Context, c kubernetes.Interface, pod, container, namespace string) (string, error) {
	sb := strings.Builder{}
	// Do not follow, which will block until the Pod terminates, and potentially deadlock the test.
	// If done in the wrong order, this could actually block things and prevent the Pod from being
	// deleted at all.
	req := c.CoreV1().Pods(namespace).GetLogs(pod, &corev1.PodLogOptions{Follow: false, Container: container})
	rc, err := req.Stream(ctx)
	if err != nil {
		return "", err
	}
	bs, err := io.ReadAll(rc)
	if err != nil {
		return "", err
	}
	sb.Write(bs)
	return sb.String(), nil
}
