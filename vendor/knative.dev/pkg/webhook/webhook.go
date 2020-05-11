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
	"errors"
	"fmt"
	"net/http"

	// Injection stuff
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	kubeinformerfactory "knative.dev/pkg/injection/clients/namespacedkube/informers/factory"

	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	admissionv1beta1 "k8s.io/api/admission/v1beta1"
	"k8s.io/client-go/kubernetes"
	corelisters "k8s.io/client-go/listers/core/v1"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/system"
	certresources "knative.dev/pkg/webhook/certificates/resources"
)

// Options contains the configuration for the webhook
type Options struct {
	// ServiceName is the service name of the webhook.
	ServiceName string

	// SecretName is the name of k8s secret that contains the webhook
	// server key/cert and corresponding CA cert that signed them. The
	// server key/cert are used to serve the webhook and the CA cert
	// is provided to k8s apiserver during admission controller
	// registration.
	SecretName string

	// Port where the webhook is served. Per k8s admission
	// registration requirements this should be 443 unless there is
	// only a single port for the service.
	Port int

	// StatsReporter reports metrics about the webhook.
	// This will be automatically initialized by the constructor if left uninitialized.
	StatsReporter StatsReporter
}

// Operation is the verb being operated on
// it is aliasde in Validation from the k8s admission package
type Operation = admissionv1beta1.Operation

// Operation types
const (
	Create  Operation = admissionv1beta1.Create
	Update  Operation = admissionv1beta1.Update
	Delete  Operation = admissionv1beta1.Delete
	Connect Operation = admissionv1beta1.Connect
)

// Webhook implements the external webhook for validation of
// resources and configuration.
type Webhook struct {
	Client  kubernetes.Interface
	Options Options
	Logger  *zap.SugaredLogger

	// synced is function that is called when the informers have been synced.
	synced context.CancelFunc

	mux          http.ServeMux
	secretlister corelisters.SecretLister
}

// New constructs a Webhook
func New(
	ctx context.Context,
	controllers []interface{},
) (webhook *Webhook, err error) {

	// ServeMux.Handle panics on duplicate paths
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("error creating webhook %v", r)
		}
	}()

	client := kubeclient.Get(ctx)

	// Injection is too aggressive for this case because by simply linking this
	// library we force consumers to have secret access.  If we require that one
	// of the admission controllers' informers *also* require the secret
	// informer, then we can fetch the shared informer factory here and produce
	// a new secret informer from it.
	secretInformer := kubeinformerfactory.Get(ctx).Core().V1().Secrets()

	opts := GetOptions(ctx)
	if opts == nil {
		return nil, errors.New("context must have Options specified")
	}
	logger := logging.FromContext(ctx)

	if opts.StatsReporter == nil {
		reporter, err := NewStatsReporter()
		if err != nil {
			return nil, err
		}
		opts.StatsReporter = reporter
	}

	syncCtx, cancel := context.WithCancel(context.Background())

	webhook = &Webhook{
		Client:       client,
		Options:      *opts,
		secretlister: secretInformer.Lister(),
		Logger:       logger,
		synced:       cancel,
	}

	webhook.mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, fmt.Sprintf("no controller registered for: %s", r.URL.Path), http.StatusBadRequest)
	})

	for _, controller := range controllers {
		switch c := controller.(type) {
		case AdmissionController:
			handler := admissionHandler(logger, opts.StatsReporter, c, syncCtx.Done())
			webhook.mux.Handle(c.Path(), handler)

		case ConversionController:
			handler := conversionHandler(logger, opts.StatsReporter, c)
			webhook.mux.Handle(c.Path(), handler)

		default:
			return nil, fmt.Errorf("unknown webhook controller type:  %T", controller)
		}

	}

	return
}

// InformersHaveSynced is called when the informers have all been synced, which allows any outstanding
// admission webhooks through.
func (wh *Webhook) InformersHaveSynced() {
	wh.synced()
	wh.Logger.Info("Informers have been synced, unblocking admission webhooks.")
}

// Run implements the admission controller run loop.
func (wh *Webhook) Run(stop <-chan struct{}) error {
	logger := wh.Logger
	ctx := logging.WithLogger(context.Background(), logger)

	server := &http.Server{
		Handler: wh,
		Addr:    fmt.Sprintf(":%d", wh.Options.Port),
		TLSConfig: &tls.Config{
			GetCertificate: func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
				secret, err := wh.secretlister.Secrets(system.Namespace()).Get(wh.Options.SecretName)
				if err != nil {
					return nil, err
				}

				serverKey, ok := secret.Data[certresources.ServerKey]
				if !ok {
					return nil, errors.New("server key missing")
				}
				serverCert, ok := secret.Data[certresources.ServerCert]
				if !ok {
					return nil, errors.New("server cert missing")
				}
				cert, err := tls.X509KeyPair(serverCert, serverKey)
				if err != nil {
					return nil, err
				}
				return &cert, nil
			},
		},
	}

	logger.Info("Found certificates for webhook...")

	eg, ctx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		if err := server.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
			logger.Errorw("ListenAndServeTLS for admission webhook returned error", zap.Error(err))
			return err
		}
		return nil
	})

	select {
	case <-stop:
		if err := server.Close(); err != nil {
			return err
		}
		return eg.Wait()
	case <-ctx.Done():
		return fmt.Errorf("webhook server bootstrap failed %v", ctx.Err())
	}
}

func (wh *Webhook) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Verify the content type is accurate.
	contentType := r.Header.Get("Content-Type")
	if contentType != "application/json" {
		http.Error(w, "invalid Content-Type, want `application/json`", http.StatusUnsupportedMediaType)
		return
	}

	wh.mux.ServeHTTP(w, r)
}
