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

package ingress

import (
	"context"
	"fmt"
	"os"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	// TODO(tcnghia): These probably shouldn't be hard-coded here?
	istioIngressNamespace = "istio-system"
	istioIngressName      = "istio-ingressgateway"
)

// GetIngressEndpoint gets the ingress public IP or hostname.
// address - is the endpoint to which we should actually connect.
// portMap - translates the request's port to the port on address to which the caller
//    should connect.  This is used when the resolution to address goes through some
//    sort of port-mapping, e.g. Kubernetes node ports.
// err - an error when address/portMap cannot be established.
func GetIngressEndpoint(ctx context.Context, kubeClientset *kubernetes.Clientset, endpointOverride string) (address string, portMap func(string) string, err error) {
	ingressName := istioIngressName
	if gatewayOverride := os.Getenv("GATEWAY_OVERRIDE"); gatewayOverride != "" {
		ingressName = gatewayOverride
	}
	ingressNamespace := istioIngressNamespace
	if gatewayNsOverride := os.Getenv("GATEWAY_NAMESPACE_OVERRIDE"); gatewayNsOverride != "" {
		ingressNamespace = gatewayNsOverride
	}

	ingress, err := kubeClientset.CoreV1().Services(ingressNamespace).Get(ctx, ingressName, metav1.GetOptions{})
	if err != nil {
		return "", nil, err
	}

	// If an override is provided, use it
	if endpointOverride != "" {
		return endpointOverride, func(port string) string {
			for _, sp := range ingress.Spec.Ports {
				if fmt.Sprint(sp.Port) == port {
					return fmt.Sprint(sp.NodePort)
				}
			}
			return port
		}, nil
	}
	endpoint, err := EndpointFromService(ingress)
	if err != nil {
		return "", nil, err
	}
	return endpoint, func(in string) string { return in }, nil
}

// EndpointFromService extracts the endpoint from the service's ingress.
func EndpointFromService(svc *v1.Service) (string, error) {
	ingresses := svc.Status.LoadBalancer.Ingress
	if len(ingresses) != 1 {
		return "", fmt.Errorf("expected exactly one ingress load balancer, instead had %d: %v", len(ingresses), ingresses)
	}
	itu := ingresses[0]

	switch {
	case itu.IP != "":
		return itu.IP, nil
	case itu.Hostname != "":
		return itu.Hostname, nil
	default:
		return "", fmt.Errorf("expected ingress loadbalancer IP or hostname for %s to be set, instead was empty", svc.Name)
	}
}
