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

package test

import (
	"io/ioutil"
	"strings"

	"github.com/knative/pkg/test/logging"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// CollectBuildLogs will get the build logs for a task run
func CollectBuildLogs(c *clients, podName, namespace string, logf logging.FormatLogger) {
	logs, err := getInitContainerLogsFromPod(c.KubeClient.Kube, podName, namespace)
	if err != nil {
		logf("Expected there to be logs from build helm-deploy-pipeline-run-helm-deploy %s", err)
	}
	logf("build logs %s", logs)
}

func getInitContainerLogsFromPod(c kubernetes.Interface, pod, namespace string) (string, error) {
	p, err := c.CoreV1().Pods(namespace).Get(pod, metav1.GetOptions{})
	if err != nil {
		return "", err
	}

	sb := strings.Builder{}
	for _, initContainer := range p.Spec.InitContainers {
		req := c.CoreV1().Pods(namespace).GetLogs(pod, &corev1.PodLogOptions{Follow: true, Container: initContainer.Name})
		rc, err := req.Stream()
		if err != nil {
			return "", err
		}
		bs, err := ioutil.ReadAll(rc)
		if err != nil {
			return "", err
		}
		sb.Write(bs)
	}
	return sb.String(), nil
}
