package e2e

import (
	"fmt"
	"io/ioutil"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"knative.dev/pkg/test/logging"
)

// CollectPodLogs will get the logs for all containers in a Pod
func CollectPodLogs(c *Clients, podName, namespace string, logf logging.FormatLogger) {
	logs, err := getContainerLogsFromPod(c.KubeClient.Kube, podName, namespace)
	if err != nil {
		logf("Could not get logs for pod %s: %s", podName, err)
	}
	logf("build logs %s", logs)
}

func getContainerLogsFromPod(c kubernetes.Interface, pod, namespace string) (string, error) {
	p, err := c.CoreV1().Pods(namespace).Get(pod, metav1.GetOptions{})
	if err != nil {
		return "", err
	}

	sb := strings.Builder{}
	for _, container := range p.Spec.Containers {
		sb.WriteString(fmt.Sprintf("\n>>> Container %s:\n", container.Name))
		req := c.CoreV1().Pods(namespace).GetLogs(pod, &corev1.PodLogOptions{Follow: true, Container: container.Name})
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
