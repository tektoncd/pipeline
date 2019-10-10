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
	"os"
	"os/user"
	"path"
	"path/filepath"
	"regexp"

	"github.com/pkg/errors"
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

	// WebhookResourcePercentageCoverageEndPoint constant for
	// ResourcePercentageCoverage API endpoint.
	WebhookResourcePercentageCoverageEndPoint = "https://%s:443" + webhook.ResourcePercentageCoverageEndPoint
)

var (
	jUnitFileRegexExpr = regexp.MustCompile(`junit_.*\.xml`)
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
		return "", errors.Wrap(err, "encountered error making resource coverage request")
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("invalid HTTP Status received for resource coverage request. Status: %d", resp.StatusCode)
	}

	var body []byte
	if body, err = ioutil.ReadAll(resp.Body); err != nil {
		return "", errors.Wrap(err, "Failed reading resource coverage response")
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

	return ioutil.WriteFile(outputFile, []byte(resourceCoverage), 0400)
}

// GetTotalCoverage calls the total coverage API to retrieve total coverage values.
func GetTotalCoverage(webhookIP string) (*coveragecalculator.CoverageValues, error) {
	client := &http.Client{Transport: &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	},
	}

	resp, err := client.Get(fmt.Sprintf(WebhookTotalCoverageEndPoint, webhookIP))
	if err != nil {
		return nil, errors.Wrap(err, "encountered error making total coverage request")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("invalid HTTP Status received for total coverage request. Status: %d", resp.StatusCode)
	}

	var body []byte
	if body, err = ioutil.ReadAll(resp.Body); err != nil {
		return nil, fmt.Errorf("error reading total coverage response: %v", err)
	}

	var coverage coveragecalculator.CoverageValues
	if err = json.Unmarshal(body, &coverage); err != nil {
		return nil, errors.Wrap(err, "Failed unmarshalling response to CoverageValues instance")
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

	htmlData, err := view.GetHTMLCoverageValuesDisplay(totalCoverage)
	if err != nil {
		return errors.Wrap(err, "Failed building html file from total coverage. error")
	}

	return ioutil.WriteFile(outputFile, []byte(htmlData), 0400)
}

// GetResourcePercentages calls resource percentage coverage API to retrieve
// percentage values.
func GetResourcePercentages(webhookIP string) (
	*coveragecalculator.CoveragePercentages, error) {
	client := &http.Client{Transport: &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	},
	}

	resp, err := client.Get(fmt.Sprintf(WebhookResourcePercentageCoverageEndPoint,
		webhookIP))
	if err != nil {
		return nil, errors.Wrap(err, "encountered error making resource percentage coverage request")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Invalid HTTP Status received for resource"+
			" percentage coverage request. Status: %d", resp.StatusCode)
	}

	var body []byte
	if body, err = ioutil.ReadAll(resp.Body); err != nil {
		return nil, errors.Wrap(err, "Failed reading resource percentage coverage response")
	}

	coveragePercentages := &coveragecalculator.CoveragePercentages{}
	if err = json.Unmarshal(body, coveragePercentages); err != nil {
		return nil, errors.Wrap(err, "Failed unmarshalling response to CoveragePercentages instance")
	}

	return coveragePercentages, nil
}

// WriteResourcePercentages writes CoveragePercentages to junit_xml output file.
func WriteResourcePercentages(outputFile string,
	coveragePercentages *coveragecalculator.CoveragePercentages) error {
	htmlData, err := view.GetCoveragePercentageXMLDisplay(coveragePercentages)
	if err != nil {
		errors.Wrap(err, "Failed building coverage percentage xml file")
	}

	return ioutil.WriteFile(outputFile, []byte(htmlData), 0400)
}

// Helper function to cleanup any existing Junit XML files.
// This is done to ensure that we only have one Junit XML file providing the
// API Coverage summary.
func CleanupJunitFiles(artifactsDir string) {
	filepath.Walk(artifactsDir, func(path string, info os.FileInfo, err error) error {
		if !info.IsDir() && jUnitFileRegexExpr.MatchString(info.Name()) {
			os.Remove(path)
		}
		return nil
	})
}
