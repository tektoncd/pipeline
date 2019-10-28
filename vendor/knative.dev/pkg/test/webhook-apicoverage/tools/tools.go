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

package tools

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os/user"
	"path"

	"knative.dev/pkg/test/webhook-apicoverage/coveragecalculator"
	"knative.dev/pkg/test/webhook-apicoverage/view"
	"knative.dev/pkg/test/webhook-apicoverage/webhook"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	// Mysteriously required to support GCP auth (required by k8s libs).
	// Apparently just importing it is enough. @_@ side effects @_@.
	// https://github.com/kubernetes/client-go/issues/242
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/client-go/tools/clientcmd"
)

// tools.go contains utility methods to help repos use the webhook-apicoverage tool.

const (
	// WebhookResourceCoverageEndPoint constant for resource coverage API endpoint.
	WebhookResourceCoverageEndPoint = "https://%s:443" + webhook.ResourceCoverageEndPoint + "?resource=%s"

	// WebhookTotalCoverageEndPoint constant for total coverage API endpoint.
	WebhookTotalCoverageEndPoint = "https://%s:443" + webhook.TotalCoverageEndPoint
)

// GetDefaultKubePath helper method to fetch kubeconfig path.
func GetDefaultKubePath() (string, error) {
	var (
		usr *user.User
		err error
	)
	if usr, err = user.Current(); err != nil {
		return "", fmt.Errorf("error retrieving current user: %v", err)
	}

	return path.Join(usr.HomeDir, ".kube/config"), nil
}

func getKubeClient(kubeConfigPath string, clusterName string) (*kubernetes.Clientset, error) {
	overrides := clientcmd.ConfigOverrides{}
	overrides.Context.Cluster = clusterName
	clientCfg, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeConfigPath},
		&overrides).ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("error building kube client config: %v", err)
	}

	var kubeClient *kubernetes.Clientset
	if kubeClient, err = kubernetes.NewForConfig(clientCfg); err != nil {
		return nil, fmt.Errorf("error building KubeClient from config: %v", err)
	}

	return kubeClient, nil
}

// GetWebhookServiceIP is a helper method to fetch IP Address of the LoadBalancer webhook service.
func GetWebhookServiceIP(kubeConfigPath string, clusterName string, namespace string, serviceName string) (string, error) {
	kubeClient, err := getKubeClient(kubeConfigPath, clusterName)
	if err != nil {
		return "", err
	}

	svc, err := kubeClient.CoreV1().Services(namespace).Get(serviceName, v1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("error encountered while retrieving service: %s Error: %v", serviceName, err)
	}

	if len(svc.Status.LoadBalancer.Ingress) == 0 {
		return "", fmt.Errorf("found zero Ingress instances for service: %s", serviceName)
	}

	return svc.Status.LoadBalancer.Ingress[0].IP, nil
}

// GetResourceCoverage is a helper method to get Coverage data for a resource from the service webhook.
func GetResourceCoverage(webhookIP string, resourceName string) (string, error) {
	client := &http.Client{Transport: &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	},
	}
	resp, err := client.Get(fmt.Sprintf(WebhookResourceCoverageEndPoint, webhookIP, resourceName))
	if err != nil {
		return "", fmt.Errorf("encountered error making resource coverage request: %v", err)
	} else if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("invalid HTTP Status received for resource coverage request. Status: %d", resp.StatusCode)
	}

	var body []byte
	if body, err = ioutil.ReadAll(resp.Body); err != nil {
		return "", fmt.Errorf("error reading resource coverage response: %v", err)
	}

	return string(body), nil
}

// GetAndWriteResourceCoverage is a helper method that uses GetResourceCoverage to get coverage and write it to a file.
func GetAndWriteResourceCoverage(webhookIP string, resourceName string, outputFile string, displayRules view.DisplayRules) error {
	var (
		err              error
		resourceCoverage string
	)

	if resourceCoverage, err = GetResourceCoverage(webhookIP, resourceName); err != nil {
		return err
	}

	if err = ioutil.WriteFile(outputFile, []byte(resourceCoverage), 0400); err != nil {
		return fmt.Errorf("error writing resource coverage to output file: %s, error: %v coverage: %s", outputFile, err, resourceCoverage)
	}

	return nil
}

// GetTotalCoverage calls the total coverage API to retrieve total coverage values.
func GetTotalCoverage(webhookIP string) (*coveragecalculator.CoverageValues, error) {
	client := &http.Client{Transport: &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	},
	}

	resp, err := client.Get(fmt.Sprintf(WebhookTotalCoverageEndPoint, webhookIP))
	if err != nil {
		return nil, fmt.Errorf("encountered error making total coverage request: %v", err)
	} else if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("invalid HTTP Status received for total coverage request. Status: %d", resp.StatusCode)
	}

	var body []byte
	if body, err = ioutil.ReadAll(resp.Body); err != nil {
		return nil, fmt.Errorf("error reading total coverage response: %v", err)
	}

	var coverage coveragecalculator.CoverageValues
	if err = json.Unmarshal(body, &coverage); err != nil {
		return nil, fmt.Errorf("error unmarshalling response to CoverageValues instance: %v", err)
	}

	return &coverage, nil
}

// GetAndWriteTotalCoverage uses the GetTotalCoverage method to get total coverage and write it to a output file.
func GetAndWriteTotalCoverage(webhookIP string, outputFile string) error {
	var (
		totalCoverage *coveragecalculator.CoverageValues
		err           error
	)

	if totalCoverage, err = GetTotalCoverage(webhookIP); err != nil {
		return err
	}

	if htmlData, err := view.GetHTMLCoverageValuesDisplay(totalCoverage); err != nil {
		return fmt.Errorf("error building html file from total coverage. error: %v", err)
	} else {
		if err = ioutil.WriteFile(outputFile, []byte(htmlData), 0400); err != nil {
			return fmt.Errorf("error writing total coverage to output file: %s, error: %v", outputFile, err)
		}
	}

	return nil
}
