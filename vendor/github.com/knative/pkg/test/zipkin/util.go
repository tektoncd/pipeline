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

//util has constants and helper methods useful for zipkin tracing support.

package zipkin

import (
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"

	"github.com/knative/pkg/test/logging"
	"go.opencensus.io/trace"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (

	//ZipkinTraceIDHeader HTTP response header key to be used to store Zipkin Trace ID.
	ZipkinTraceIDHeader = "ZIPKIN_TRACE_ID"

	// ZipkinPort port exposed by the Zipkin Pod
	// https://github.com/knative/serving/blob/master/config/monitoring/200-common/100-zipkin.yaml#L25 configures the Zipkin Port on the cluster.
	ZipkinPort = 9411

	// ZipkinTraceEndpoint port-forwarded zipkin endpoint
	ZipkinTraceEndpoint = "http://localhost:9411/api/v2/trace/"

	// ZipkinNamespace namespace where zipkin pod runs.
	// https://github.com/knative/serving/blob/master/config/monitoring/200-common/100-zipkin.yaml#L19 configures the namespace for zipkin.
	ZipkinNamespace = "istio-system"
)

var zipkinPortForwardPID int

// SetupZipkinTracing sets up zipkin tracing which involves a) Setting up port-forwarding from localhost to zipkin pod on the cluster (pid of the process doing Port-Forward is stored in a global variable).
// b) enable AlwaysSample config for tracing.
func SetupZipkinTracing(kubeClientset *kubernetes.Clientset) error {
	logger := logging.GetContextLogger("SpoofUtil")

	if err := CheckZipkinPortAvailability(); err != nil {
		return fmt.Errorf("Zipkin port not available on the machine: %v", err)
	}

	zipkinPods, err := kubeClientset.CoreV1().Pods(ZipkinNamespace).List(metav1.ListOptions{LabelSelector: "app=zipkin"})
	if err != nil {
		return fmt.Errorf("Error retrieving Zipkin pod details : %v", err)
	}

	if len(zipkinPods.Items) == 0 {
		return errors.New("No Zipkin Pod found on the cluster. Ensure monitoring is switched on for your Knative Setup")
	}

	portForwardCmd := exec.Command("kubectl", "port-forward", "--namespace="+ZipkinNamespace, zipkinPods.Items[0].Name, fmt.Sprintf("%d:%d", ZipkinPort, ZipkinPort))
	if err = portForwardCmd.Start(); err != nil {
		return fmt.Errorf("Error starting kubectl port-forward command : %v", err)

	}
	zipkinPortForwardPID = portForwardCmd.Process.Pid
	logger.Infof("Zipkin port-forward process started with PID: %d", zipkinPortForwardPID)

	// Applying AlwaysSample config to ensure we propagate zipkin header for every request made by this client.
	trace.ApplyConfig(trace.Config{DefaultSampler: trace.AlwaysSample()})

	logger.Infof("Successfully setup SpoofingClient for Zipkin Tracing")

	return nil
}

// CleanupZipkinTracingSetup cleans up the Zipkin tracing setup on the machine. This involves killing the process performing port-forward.
func CleanupZipkinTracingSetup() error {
	logger := logging.GetContextLogger("SpoofUtil")

	ps := os.Process{Pid: zipkinPortForwardPID}
	if err := ps.Kill(); err != nil {
		return fmt.Errorf("Encoutered error killing port-forward process in CleanupZipkingTracingSetup() : %v", err)
	}

	logger.Infof("Successfully killed Zipkin port-forward process")
	return nil
}

// CheckZipkinPortAvailability checks to see if Zipkin Port is available on the machine.
// returns error if the port is not available.
func CheckZipkinPortAvailability() error {
	server, err := net.Listen("tcp", fmt.Sprintf(":%d", ZipkinPort))
	if err != nil {
		// Port is likely taken
		return err
	}
	server.Close()
	return nil
}
