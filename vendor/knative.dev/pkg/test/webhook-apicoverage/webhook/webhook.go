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
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/markbates/inflect"
	"go.uber.org/zap"
	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/webhook"
)

var (
	// GroupVersionKind for deployment to be used to set the webhook's owner reference.
	deploymentKind = extensionsv1beta1.SchemeGroupVersion.WithKind("Deployment")
)

// APICoverageWebhook encapsulates necessary configuration details for the api-coverage webhook.
type APICoverageWebhook struct {
	// WebhookName is the name of the validation webhook we create to intercept API calls.
	WebhookName string

	// ServiceName is the name of K8 service under which the webhook runs.
	ServiceName string

	// DeploymentName is the deployment name for the webhook.
	DeploymentName string

	// Namespace is the namespace in which everything above lives.
	Namespace string

	// Port where the webhook is served.
	Port int

	// RegistrationDelay controls how long validation requests
	// occurs after the webhook is started. This is used to avoid
	// potential races where registration completes and k8s apiserver
	// invokes the webhook before the HTTP server is started.
	RegistrationDelay time.Duration

	// ClientAuthType declares the policy the webhook server will follow for TLS Client Authentication.
	ClientAuth tls.ClientAuthType

	// CaCert is the CA Cert for the webhook server.
	CaCert []byte

	// FailurePolicy policy governs the webhook validation decisions.
	FailurePolicy admissionregistrationv1beta1.FailurePolicyType

	// Logger is the configured logger for the webhook.
	Logger *zap.SugaredLogger

	// KubeClient is the K8 client to the target cluster.
	KubeClient kubernetes.Interface
}

func (acw *APICoverageWebhook) generateServerConfig() (*tls.Config, error) {
	serverKey, serverCert, caCert, err := webhook.CreateCerts(context.Background(), acw.ServiceName, acw.Namespace)
	if err != nil {
		return nil, fmt.Errorf("Error creating webhook certificates: %v", err)
	}

	cert, err := tls.X509KeyPair(serverCert, serverKey)
	if err != nil {
		return nil, fmt.Errorf("Error creating X509 Key pair for webhook server: %v", err)
	}

	acw.CaCert = caCert
	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientAuth:   acw.ClientAuth,
	}, nil
}

func (acw *APICoverageWebhook) getWebhookServer(handler http.Handler) (*http.Server, error) {
	tlsConfig, err := acw.generateServerConfig()
	if err != nil {
		// generateServerConfig() is expected to provided explanatory error message.
		return nil, err
	}

	return &http.Server{
		Handler:   handler,
		Addr:      fmt.Sprintf(":%d", acw.Port),
		TLSConfig: tlsConfig,
	}, nil
}

func (acw *APICoverageWebhook) registerWebhook(rules []admissionregistrationv1beta1.RuleWithOperations, namespace string) error {
	webhook := &admissionregistrationv1beta1.ValidatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name:      acw.WebhookName,
			Namespace: namespace,
		},
		Webhooks: []admissionregistrationv1beta1.ValidatingWebhook{{
			Name:  acw.WebhookName,
			Rules: rules,
			ClientConfig: admissionregistrationv1beta1.WebhookClientConfig{
				Service: &admissionregistrationv1beta1.ServiceReference{
					Namespace: namespace,
					Name:      acw.ServiceName,
				},
				CABundle: acw.CaCert,
			},
			FailurePolicy: &acw.FailurePolicy,
		},
		},
	}

	deployment, err := acw.KubeClient.ExtensionsV1beta1().Deployments(namespace).Get(acw.DeploymentName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("Error retrieving Deployment Extension object: %v", err)
	}
	deploymentRef := metav1.NewControllerRef(deployment, deploymentKind)
	webhook.OwnerReferences = append(webhook.OwnerReferences, *deploymentRef)

	_, err = acw.KubeClient.AdmissionregistrationV1beta1().ValidatingWebhookConfigurations().Create(webhook)
	if err != nil {
		return fmt.Errorf("Error creating ValidatingWebhookConfigurations object: %v", err)
	}

	return nil
}

func (acw *APICoverageWebhook) getValidationRules(resources map[schema.GroupVersionKind]webhook.GenericCRD) []admissionregistrationv1beta1.RuleWithOperations {
	var rules []admissionregistrationv1beta1.RuleWithOperations
	for gvk := range resources {
		plural := strings.ToLower(inflect.Pluralize(gvk.Kind))

		rules = append(rules, admissionregistrationv1beta1.RuleWithOperations{
			Operations: []admissionregistrationv1beta1.OperationType{
				admissionregistrationv1beta1.Create,
				admissionregistrationv1beta1.Update,
			},
			Rule: admissionregistrationv1beta1.Rule{
				APIGroups:   []string{gvk.Group},
				APIVersions: []string{gvk.Version},
				Resources:   []string{plural},
			},
		})
	}
	return rules
}

// SetupWebhook sets up the webhook with the provided http.handler, resourcegroup Map, namespace and stop channel.
func (acw *APICoverageWebhook) SetupWebhook(handler http.Handler, resources map[schema.GroupVersionKind]webhook.GenericCRD, namespace string, stop <-chan struct{}) error {
	server, err := acw.getWebhookServer(handler)
	rules := acw.getValidationRules(resources)
	if err != nil {
		return fmt.Errorf("Webhook server object creation failed: %v", err)
	}

	select {
	case <-time.After(acw.RegistrationDelay):
		err = acw.registerWebhook(rules, namespace)
		if err != nil {
			return fmt.Errorf("Webhook registration failed: %v", err)
		}
		acw.Logger.Info("Successfully registered webhook")
	case <-stop:
		return nil
	}

	serverBootstrapErrCh := make(chan struct{})
	go func() {
		if err := server.ListenAndServeTLS("", ""); err != nil {
			acw.Logger.Error("ListenAndServeTLS for admission webhook returned error", zap.Error(err))
			close(serverBootstrapErrCh)
			return
		}
		acw.Logger.Info("Successfully started webhook server")
	}()

	select {
	case <-stop:
		return server.Close()
	case <-serverBootstrapErrCh:
		return errors.New("webhook server bootstrap failed")
	}
}

// BuildWebhookConfiguration builds the APICoverageWebhook object using the provided names.
func BuildWebhookConfiguration(componentCommonName string, webhookName string, namespace string) *APICoverageWebhook {
	cm, err := configmap.Load("/etc/config-logging")
	if err != nil {
		log.Fatalf("Error loading logging configuration: %v", err)
	}

	config, err := logging.NewConfigFromMap(cm)
	if err != nil {
		log.Fatalf("Error parsing logging configuration: %v", err)
	}
	logger, _ := logging.NewLoggerFromConfig(config, "webhook")

	clusterConfig, err := rest.InClusterConfig()
	if err != nil {
		log.Fatalf("Failed to get in cluster config: %v", err)
	}

	kubeClient, err := kubernetes.NewForConfig(clusterConfig)
	if err != nil {
		log.Fatalf("Failed to get client set: %v", err)
	}

	return &APICoverageWebhook{
		Logger:            logger,
		KubeClient:        kubeClient,
		FailurePolicy:     admissionregistrationv1beta1.Fail,
		ClientAuth:        tls.NoClientCert,
		RegistrationDelay: time.Second * 2,
		Port:              8443, // Using port greater than 1024 as webhook runs as non-root user.
		Namespace:         namespace,
		DeploymentName:    componentCommonName,
		ServiceName:       componentCommonName,
		WebhookName:       webhookName,
	}
}
