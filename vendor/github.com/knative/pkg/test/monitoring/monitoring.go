/*
Copyright 2019 The Knative Authors

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

package monitoring

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"

	"github.com/knative/pkg/test/logging"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// CheckPortAvailability checks to see if the port is available on the machine.
func CheckPortAvailability(port int) error {
	server, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		// Port is likely taken
		return err
	}
	server.Close()

	return nil
}

// GetPods retrieves the current existing podlist for the app in monitoring namespace
// This uses app=<app> as labelselector for selecting pods
func GetPods(kubeClientset *kubernetes.Clientset, app, namespace string) (*v1.PodList, error) {
	pods, err := kubeClientset.CoreV1().Pods(namespace).List(metav1.ListOptions{LabelSelector: fmt.Sprintf("app=%s", app)})
	if err == nil && len(pods.Items) == 0 {
		err = fmt.Errorf("No %s Pod found on the cluster. Ensure monitoring is switched on for your Knative Setup", app)
	}

	return pods, err
}

// Cleanup will clean the background process used for port forwarding
func Cleanup(pid int) error {
	ps := os.Process{Pid: pid}
	return ps.Kill()
}

// PortForward sets up local port forward to the pod specified by the "app" label in the given namespace
func PortForward(logf logging.FormatLogger, podList *v1.PodList, localPort, remotePort int, namespace string) (int, error) {
	podName := podList.Items[0].Name
	portFwdCmd := fmt.Sprintf("kubectl port-forward %s %d:%d -n %s", podName, localPort, remotePort, namespace)
	portFwdProcess, err := executeCmdBackground(logf, portFwdCmd)

	if err != nil {
		return 0, fmt.Errorf("Failed to port forward: %v", err)
	}

	logf("running %s port-forward in background, pid = %d", podName, portFwdProcess.Pid)
	return portFwdProcess.Pid, nil
}

// RunBackground starts a background process and returns the Process if succeed
func executeCmdBackground(logf logging.FormatLogger, format string, args ...interface{}) (*os.Process, error) {
	cmd := fmt.Sprintf(format, args...)
	logf("Executing command: %s", cmd)
	parts := strings.Split(cmd, " ")
	c := exec.Command(parts[0], parts[1:]...) // #nosec
	if err := c.Start(); err != nil {
		return nil, fmt.Errorf("%s command failed: %v", cmd, err)
	}
	return c.Process, nil
}
