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

package webhook

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"net"
	"net/http"
	"strconv"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
)

func waitForServerAvailable(t *testing.T, serverURL string, timeout time.Duration) error {
	t.Helper()
	var interval = 100 * time.Millisecond

	conditionFunc := func() (done bool, err error) {
		var conn net.Conn
		conn, err = net.DialTimeout("tcp", serverURL, timeout)
		if conn != nil {
			conn.Close()
			return true, nil
		}
		return false, nil
	}

	return wait.PollImmediate(interval, timeout, conditionFunc)
}

func newTestPort() (port int, err error) {
	server, err := net.Listen("tcp", ":0")
	if err != nil {
		return 0, err
	}

	defer server.Close()

	_, portString, err := net.SplitHostPort(server.Addr().String())
	if err != nil {
		return 0, err
	}

	return strconv.Atoi(portString)
}

func createNamespace(t *testing.T, kubeClient kubernetes.Interface, name string) error {
	t.Helper()
	testns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
	_, err := kubeClient.CoreV1().Namespaces().Create(testns)
	if err != nil {
		return err
	}
	return nil
}

func createTestConfigMap(t *testing.T, kubeClient kubernetes.Interface) error {
	t.Helper()
	configMaps := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: "extension-apiserver-authentication",
		},
		Data: map[string]string{"requestheader-client-ca-file": "test-client-file"},
	}
	_, err := kubeClient.CoreV1().ConfigMaps(metav1.NamespaceSystem).Create(configMaps)
	if err != nil {
		return err
	}
	return nil
}

func createSecureTLSClient(t *testing.T, kubeClient kubernetes.Interface, acOpts *ControllerOptions) (*http.Client, error) {
	t.Helper()
	tlsServerConfig, caCert, err := configureCerts(context.TODO(), kubeClient, acOpts)
	if err != nil {
		return nil, err
	}
	// Build cert pool with CA Cert
	pool := x509.NewCertPool()
	pool.AppendCertsFromPEM(caCert)

	tlsClientConfig := &tls.Config{
		// Add knative namespace as CN
		ServerName:   "webhook.knative-something",
		RootCAs:      pool,
		Certificates: tlsServerConfig.Certificates,
	}
	return &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: tlsClientConfig,
		},
	}, nil
}
