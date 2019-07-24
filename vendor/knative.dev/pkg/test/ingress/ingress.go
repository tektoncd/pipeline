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

// GetIngressEndpoint gets the endpoint IP or hostname to use for the service.
func GetIngressEndpoint(kubeClientset *kubernetes.Clientset) (*string, error) {
	ingressName := istioIngressName
	if gatewayOverride := os.Getenv("GATEWAY_OVERRIDE"); gatewayOverride != "" {
		ingressName = gatewayOverride
	}
	ingressNamespace := istioIngressNamespace
	if gatewayNsOverride := os.Getenv("GATEWAY_NAMESPACE_OVERRIDE"); gatewayNsOverride != "" {
		ingressNamespace = gatewayNsOverride
	}

	ingress, err := kubeClientset.CoreV1().Services(ingressNamespace).Get(ingressName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	endpoint, err := EndpointFromService(ingress)
	if err != nil {
		return nil, err
	}
	return &endpoint, nil
}

// EndpointFromService extracts the endpoint from the service's ingress.
func EndpointFromService(svc *v1.Service) (string, error) {
	ingresses := svc.Status.LoadBalancer.Ingress
	if len(ingresses) != 1 {
		return "", fmt.Errorf("Expected exactly one ingress load balancer, instead had %d: %v", len(ingresses), ingresses)
	}
	itu := ingresses[0]

	switch {
	case itu.IP != "":
		return itu.IP, nil
	case itu.Hostname != "":
		return itu.Hostname, nil
	default:
		return "", fmt.Errorf("Expected ingress loadbalancer IP or hostname for %s to be set, instead was empty", svc.Name)
	}
}
