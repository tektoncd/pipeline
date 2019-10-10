/*
Copyright 2017 The Knative Authors

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
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"go.uber.org/zap"

	"knative.dev/pkg/logging"
	"knative.dev/pkg/logging/logkey"

	admissionv1beta1 "k8s.io/api/admission/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	secretServerKey  = "server-key.pem"
	secretServerCert = "server-cert.pem"
	secretCACert     = "ca-cert.pem"
)

var (
	deploymentKind      = appsv1.SchemeGroupVersion.WithKind("Deployment")
	errMissingNewObject = errors.New("the new object may not be nil")
)

// ControllerOptions contains the configuration for the webhook
type ControllerOptions struct {
	// WebhookName is the name of the webhook we create to handle
	// mutations before they get stored in the storage.
	WebhookName string

	// ServiceName is the service name of the webhook.
	ServiceName string

	// DeploymentName is the service name of the webhook.
	DeploymentName string

	// SecretName is the name of k8s secret that contains the webhook
	// server key/cert and corresponding CA cert that signed them. The
	// server key/cert are used to serve the webhook and the CA cert
	// is provided to k8s apiserver during admission controller
	// registration.
	SecretName string

	// Namespace is the namespace in which everything above lives.
	Namespace string

	// Port where the webhook is served. Per k8s admission
	// registration requirements this should be 443 unless there is
	// only a single port for the service.
	Port int

	// RegistrationDelay controls how long admission registration
	// occurs after the webhook is started. This is used to avoid
	// potential races where registration completes and k8s apiserver
	// invokes the webhook before the HTTP server is started.
	RegistrationDelay time.Duration

	// ClientAuthType declares the policy the webhook server will follow for
	// TLS Client Authentication.
	// The default value is tls.NoClientCert.
	ClientAuth tls.ClientAuthType

	// StatsReporter reports metrics about the webhook.
	// This will be automatically initialized by the constructor if left uninitialized.
	StatsReporter StatsReporter

	// Service path for ResourceAdmissionController webhook
	// Default is "/" for backward compatibility and is set by the constructor
	ResourceAdmissionControllerPath string
}

// AdmissionController provides the interface for different admission controllers
type AdmissionController interface {
	Admit(context.Context, *admissionv1beta1.AdmissionRequest) *admissionv1beta1.AdmissionResponse
	Register(context.Context, kubernetes.Interface, []byte) error
}

// Webhook implements the external webhook for validation of
// resources and configuration.
type Webhook struct {
	Client               kubernetes.Interface
	Options              ControllerOptions
	Logger               *zap.SugaredLogger
	admissionControllers map[string]AdmissionController

	WithContext func(context.Context) context.Context
}

// New constructs a Webhook
func New(
	client kubernetes.Interface,
	opts ControllerOptions,
	admissionControllers map[string]AdmissionController,
	logger *zap.SugaredLogger,
	ctx func(context.Context) context.Context,
) (*Webhook, error) {

	if opts.StatsReporter == nil {
		reporter, err := NewStatsReporter()
		if err != nil {
			return nil, err
		}
		opts.StatsReporter = reporter
	}

	return &Webhook{
		Client:               client,
		Options:              opts,
		admissionControllers: admissionControllers,
		Logger:               logger,
		WithContext:          ctx,
	}, nil
}

// Run implements the admission controller run loop.
func (ac *Webhook) Run(stop <-chan struct{}) error {
	logger := ac.Logger
	ctx := logging.WithLogger(context.TODO(), logger)
	tlsConfig, caCert, err := configureCerts(ctx, ac.Client, &ac.Options)
	if err != nil {
		logger.Errorw("could not configure admission webhook certs", zap.Error(err))
		return err
	}

	server := &http.Server{
		Handler:   ac,
		Addr:      fmt.Sprintf(":%v", ac.Options.Port),
		TLSConfig: tlsConfig,
	}

	logger.Info("Found certificates for webhook...")
	if ac.Options.RegistrationDelay != 0 {
		logger.Infof("Delaying admission webhook registration for %v", ac.Options.RegistrationDelay)
	}

	select {
	case <-time.After(ac.Options.RegistrationDelay):
		for _, c := range ac.admissionControllers {
			if err := c.Register(ctx, ac.Client, caCert); err != nil {
				logger.Errorw("failed to register webhook", zap.Error(err))
				return err
			}
		}
		logger.Info("Successfully registered webhook")
	case <-stop:
		return nil
	}

	serverBootstrapErrCh := make(chan struct{})
	go func() {
		if err := server.ListenAndServeTLS("", ""); err != nil {
			logger.Errorw("ListenAndServeTLS for admission webhook returned error", zap.Error(err))
			close(serverBootstrapErrCh)
		}
	}()

	select {
	case <-stop:
		return server.Close()
	case <-serverBootstrapErrCh:
		return errors.New("webhook server bootstrap failed")
	}
}

// ServeHTTP implements the external admission webhook for mutating
// serving resources.
func (ac *Webhook) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var ttStart = time.Now()
	logger := ac.Logger
	logger.Infof("Webhook ServeHTTP request=%#v", r)

	// Verify the content type is accurate.
	contentType := r.Header.Get("Content-Type")
	if contentType != "application/json" {
		http.Error(w, "invalid Content-Type, want `application/json`", http.StatusUnsupportedMediaType)
		return
	}

	var review admissionv1beta1.AdmissionReview
	if err := json.NewDecoder(r.Body).Decode(&review); err != nil {
		http.Error(w, fmt.Sprintf("could not decode body: %v", err), http.StatusBadRequest)
		return
	}

	logger = logger.With(
		zap.String(logkey.Kind, fmt.Sprint(review.Request.Kind)),
		zap.String(logkey.Namespace, review.Request.Namespace),
		zap.String(logkey.Name, review.Request.Name),
		zap.String(logkey.Operation, fmt.Sprint(review.Request.Operation)),
		zap.String(logkey.Resource, fmt.Sprint(review.Request.Resource)),
		zap.String(logkey.SubResource, fmt.Sprint(review.Request.SubResource)),
		zap.String(logkey.UserInfo, fmt.Sprint(review.Request.UserInfo)))
	ctx := logging.WithLogger(r.Context(), logger)

	if ac.WithContext != nil {
		ctx = ac.WithContext(ctx)
	}

	if _, ok := ac.admissionControllers[r.URL.Path]; !ok {
		http.Error(w, fmt.Sprintf("no admission controller registered for: %s", r.URL.Path), http.StatusBadRequest)
		return
	}

	c := ac.admissionControllers[r.URL.Path]
	reviewResponse := c.Admit(ctx, review.Request)
	var response admissionv1beta1.AdmissionReview
	if reviewResponse != nil {
		response.Response = reviewResponse
		response.Response.UID = review.Request.UID
	}

	logger.Infof("AdmissionReview for %#v: %s/%s response=%#v",
		review.Request.Kind, review.Request.Namespace, review.Request.Name, reviewResponse)

	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, fmt.Sprintf("could encode response: %v", err), http.StatusInternalServerError)
		return
	}

	if ac.Options.StatsReporter != nil {
		// Only report valid requests
		ac.Options.StatsReporter.ReportRequest(review.Request, response.Response, time.Since(ttStart))
	}
}

// GetAPIServerExtensionCACert gets the Kubernetes aggregate apiserver
// client CA cert used by validator.
//
// NOTE: this certificate is provided kubernetes. We do not control
// its name or location.
func getAPIServerExtensionCACert(cl kubernetes.Interface) ([]byte, error) {
	const name = "extension-apiserver-authentication"
	c, err := cl.CoreV1().ConfigMaps(metav1.NamespaceSystem).Get(name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	const caFileName = "requestheader-client-ca-file"
	pem, ok := c.Data[caFileName]
	if !ok {
		return nil, fmt.Errorf("cannot find %s in ConfigMap %s: ConfigMap.Data is %#v", caFileName, name, c.Data)
	}
	return []byte(pem), nil
}

// MakeTLSConfig makes a TLS configuration suitable for use with the server
func makeTLSConfig(serverCert, serverKey, caCert []byte, clientAuthType tls.ClientAuthType) (*tls.Config, error) {
	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)
	cert, err := tls.X509KeyPair(serverCert, serverKey)
	if err != nil {
		return nil, err
	}
	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientCAs:    caCertPool,
		ClientAuth:   clientAuthType,
	}, nil
}

func getOrGenerateKeyCertsFromSecret(ctx context.Context, client kubernetes.Interface,
	options *ControllerOptions) (serverKey, serverCert, caCert []byte, err error) {
	logger := logging.FromContext(ctx)
	secret, err := client.CoreV1().Secrets(options.Namespace).Get(options.SecretName, metav1.GetOptions{})
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return nil, nil, nil, err
		}
		logger.Info("Did not find existing secret, creating one")
		newSecret, err := generateSecret(ctx, options)
		if err != nil {
			return nil, nil, nil, err
		}
		secret, err = client.CoreV1().Secrets(newSecret.Namespace).Create(newSecret)
		if err != nil {
			if !apierrors.IsAlreadyExists(err) {
				return nil, nil, nil, err
			}
			// OK, so something else might have created, try fetching it instead.
			secret, err = client.CoreV1().Secrets(options.Namespace).Get(options.SecretName, metav1.GetOptions{})
			if err != nil {
				return nil, nil, nil, err
			}
		}
	}

	var ok bool
	if serverKey, ok = secret.Data[secretServerKey]; !ok {
		return nil, nil, nil, errors.New("server key missing")
	}
	if serverCert, ok = secret.Data[secretServerCert]; !ok {
		return nil, nil, nil, errors.New("server cert missing")
	}
	if caCert, ok = secret.Data[secretCACert]; !ok {
		return nil, nil, nil, errors.New("ca cert missing")
	}
	return serverKey, serverCert, caCert, nil
}

func configureCerts(ctx context.Context, client kubernetes.Interface, options *ControllerOptions) (*tls.Config, []byte, error) {
	var apiServerCACert []byte
	if options.ClientAuth >= tls.VerifyClientCertIfGiven {
		var err error
		apiServerCACert, err = getAPIServerExtensionCACert(client)
		if err != nil {
			return nil, nil, err
		}
	}

	serverKey, serverCert, caCert, err := getOrGenerateKeyCertsFromSecret(ctx, client, options)
	if err != nil {
		return nil, nil, err
	}
	tlsConfig, err := makeTLSConfig(serverCert, serverKey, apiServerCACert, options.ClientAuth)
	if err != nil {
		return nil, nil, err
	}
	return tlsConfig, caCert, nil
}

func makeErrorStatus(reason string, args ...interface{}) *admissionv1beta1.AdmissionResponse {
	result := apierrors.NewBadRequest(fmt.Sprintf(reason, args...)).Status()
	return &admissionv1beta1.AdmissionResponse{
		Result:  &result,
		Allowed: false,
	}
}

func generateSecret(ctx context.Context, options *ControllerOptions) (*corev1.Secret, error) {
	serverKey, serverCert, caCert, err := CreateCerts(ctx, options.ServiceName, options.Namespace)
	if err != nil {
		return nil, err
	}
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      options.SecretName,
			Namespace: options.Namespace,
		},
		Data: map[string][]byte{
			secretServerKey:  serverKey,
			secretServerCert: serverCert,
			secretCACert:     caCert,
		},
	}, nil
}
