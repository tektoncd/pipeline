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
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"knative.dev/pkg/test/logging"
)

// CheckPortAvailability checks to see if the port is available on the machine.
func CheckPortAvailability(port int) error {
	server, err := net.Listen("tcp", fmt.Sprint(":", port))
	if err != nil {
		// Port is likely taken
		return err
	}
	return server.Close()
}

// GetPods retrieves the current existing podlist for the app in monitoring namespace
// This uses app=<app> as labelselector for selecting pods
func GetPods(ctx context.Context, kubeClientset *kubernetes.Clientset, app, namespace string) (*v1.PodList, error) {
	pods, err := kubeClientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{LabelSelector: "app=" + app})
	if err == nil && len(pods.Items) == 0 {
		err = fmt.Errorf("pod %s not found on the cluster. Ensure monitoring is switched on for your Knative Setup", app)
	}
	return pods, err
}

// Cleanup will clean the background process used for port forwarding
func Cleanup(pid int) error {
	ps := os.Process{Pid: pid}
	if err := ps.Kill(); err != nil {
		return err
	}

	errCh := make(chan error)
	go func() {
		_, err := ps.Wait()
		errCh <- err
	}()

	select {
	case err := <-errCh:
		return err
	case <-time.After(30 * time.Second):
		return fmt.Errorf("timed out waiting for process %d to exit", pid)
	}
}

// PortForward sets up local port forward to the pod specified by the "app" label in the given namespace
func PortForward(logf logging.FormatLogger, podList *v1.PodList, localPort, remotePort int, namespace string) (int, error) {
	podName := podList.Items[0].Name
	portFwdCmd := fmt.Sprintf("kubectl port-forward %s %d:%d -n %s", podName, localPort, remotePort, namespace)
	portFwdProcess, err := executeCmdBackground(logf, portFwdCmd)

	if err != nil {
		return 0, fmt.Errorf("failed to port forward: %w", err)
	}

	logf("Running %s port-forward in background, pid = %d", podName, portFwdProcess.Pid)
	return portFwdProcess.Pid, nil
}

// RunBackground starts a background process and returns the Process if succeed
func executeCmdBackground(logf logging.FormatLogger, format string, args ...interface{}) (*os.Process, error) {
	cmd := fmt.Sprintf(format, args...)
	logf("Executing command: %s", cmd)
	parts := strings.Split(cmd, " ")
	c := exec.Command(parts[0], parts[1:]...) // #nosec
	if err := c.Start(); err != nil {
		return nil, fmt.Errorf("%s command failed: %w", cmd, err)
	}
	return c.Process, nil
}
