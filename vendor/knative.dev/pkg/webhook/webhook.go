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
	"html"
	"log"
	"net"
	"net/http"
	"time"

	// Injection stuff

	"knative.dev/pkg/controller"
	kubeinformerfactory "knative.dev/pkg/injection/clients/namespacedkube/informers/factory"
	"knative.dev/pkg/network/handlers"

	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	admissionv1 "k8s.io/api/admission/v1"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/system"
	certresources "knative.dev/pkg/webhook/certificates/resources"
)

// Options contains the configuration for the webhook
type Options struct {
	// TLSMinVersion contains the minimum TLS version that is acceptable to communicate with the API server.
	// TLS 1.3 is the minimum version if not specified otherwise.
	TLSMinVersion uint16

	// ServiceName is the service name of the webhook.
	ServiceName string

	// SecretName is the name of k8s secret that contains the webhook
	// server key/cert and corresponding CA cert that signed them. The
	// server key/cert are used to serve the webhook and the CA cert
	// is provided to k8s apiserver during admission controller
	// registration.
	// If no SecretName is provided, then the webhook serves without TLS.
	SecretName string

	// Port where the webhook is served. Per k8s admission
	// registration requirements this should be 443 unless there is
	// only a single port for the service.
	Port int

	// StatsReporter reports metrics about the webhook.
	// This will be automatically initialized by the constructor if left uninitialized.
	StatsReporter StatsReporter

	// GracePeriod is how long to wait after failing readiness probes
	// before shutting down.
	GracePeriod time.Duration

	// ControllerOptions encapsulates options for creating a new controller,
	// including throttling and stats behavior.
	ControllerOptions *controller.ControllerOptions

	// EnableHTTP2 enables HTTP2 for webhooks.
	// Mitigate CVE-2023-44487 by disabling HTTP2 by default until the Go
	// standard library and golang.org/x/net are fully fixed.
	// Right now, it is possible for authenticated and unauthenticated users to
	// hold open HTTP2 connections and consume huge amounts of memory.
	// See:
	// * https://github.com/kubernetes/kubernetes/pull/121120
	// * https://github.com/kubernetes/kubernetes/issues/121197
	// * https://github.com/golang/go/issues/63417#issuecomment-1758858612
	EnableHTTP2 bool
}

// Operation is the verb being operated on
// it is aliased in Validation from the k8s admission package
type Operation = admissionv1.Operation

// Operation types
const (
	Create  Operation = admissionv1.Create
	Update  Operation = admissionv1.Update
	Delete  Operation = admissionv1.Delete
	Connect Operation = admissionv1.Connect
)

// Webhook implements the external webhook for validation of
// resources and configuration.
type Webhook struct {
	Options Options
	Logger  *zap.SugaredLogger

	// synced is function that is called when the informers have been synced.
	synced context.CancelFunc

	mux http.ServeMux

	// The TLS configuration to use for serving (or nil for non-TLS)
	tlsConfig *tls.Config

	// testListener is only used in testing so we don't get port conflicts
	testListener net.Listener
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

	defaultTLSMinVersion := uint16(tls.VersionTLS13)
	if opts.TLSMinVersion == 0 {
		opts.TLSMinVersion = TLSMinVersionFromEnv(defaultTLSMinVersion)
	} else if opts.TLSMinVersion != tls.VersionTLS12 && opts.TLSMinVersion != tls.VersionTLS13 {
		return nil, fmt.Errorf("unsupported TLS version: %d", opts.TLSMinVersion)
	}

	syncCtx, cancel := context.WithCancel(context.Background())

	webhook = &Webhook{
		Options: *opts,
		Logger:  logger,
		synced:  cancel,
	}

	if opts.SecretName != "" {
		// Injection is too aggressive for this case because by simply linking this
		// library we force consumers to have secret access.  If we require that one
		// of the admission controllers' informers *also* require the secret
		// informer, then we can fetch the shared informer factory here and produce
		// a new secret informer from it.
		secretInformer := kubeinformerfactory.Get(ctx).Core().V1().Secrets()

		webhook.tlsConfig = &tls.Config{
			MinVersion: opts.TLSMinVersion,

			// If we return (nil, error) the client sees - 'tls: internal error"
			// If we return (nil, nil) the client sees - 'tls: no certificates configured'
			//
			// We'll return (nil, nil) when we don't find a certificate
			GetCertificate: func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
				secret, err := secretInformer.Lister().Secrets(system.Namespace()).Get(opts.SecretName)
				if err != nil {
					logger.Errorw("failed to fetch secret", zap.Error(err))
					return nil, nil
				}

				serverKey, ok := secret.Data[certresources.ServerKey]
				if !ok {
					logger.Warn("server key missing")
					return nil, nil
				}
				serverCert, ok := secret.Data[certresources.ServerCert]
				if !ok {
					logger.Warn("server cert missing")
					return nil, nil
				}
				cert, err := tls.X509KeyPair(serverCert, serverKey)
				if err != nil {
					return nil, err
				}
				return &cert, nil
			},
		}
	}

	webhook.mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, fmt.Sprint("no controller registered for: ", html.EscapeString(r.URL.Path)), http.StatusBadRequest)
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

type zapWrapper struct {
	logger *zap.SugaredLogger
}

func (z *zapWrapper) Write(p []byte) (n int, err error) {
	z.logger.Errorw(string(p))
	return len(p), nil
}

// Run implements the admission controller run loop.
func (wh *Webhook) Run(stop <-chan struct{}) error {
	logger := wh.Logger
	ctx := logging.WithLogger(context.Background(), logger)

	drainer := &handlers.Drainer{
		Inner:       wh,
		QuietPeriod: wh.Options.GracePeriod,
	}

	// If TLSNextProto is not nil, HTTP/2 support is not enabled automatically.
	nextProto := map[string]func(*http.Server, *tls.Conn, http.Handler){}
	if wh.Options.EnableHTTP2 {
		nextProto = nil
	}

	server := &http.Server{
		ErrorLog:          log.New(&zapWrapper{logger}, "", 0),
		Handler:           drainer,
		Addr:              fmt.Sprint(":", wh.Options.Port),
		TLSConfig:         wh.tlsConfig,
		ReadHeaderTimeout: time.Minute, //https://medium.com/a-journey-with-go/go-understand-and-mitigate-slowloris-attack-711c1b1403f6
		TLSNextProto:      nextProto,
	}

	var serve = server.ListenAndServe

	if server.TLSConfig != nil && wh.testListener != nil {
		serve = func() error {
			return server.ServeTLS(wh.testListener, "", "")
		}
	} else if server.TLSConfig != nil {
		serve = func() error {
			return server.ListenAndServeTLS("", "")
		}
	} else if wh.testListener != nil {
		serve = func() error {
			return server.Serve(wh.testListener)
		}
	}

	eg, ctx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		if err := serve(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Errorw("ListenAndServe for admission webhook returned error", zap.Error(err))
			return err
		}
		return nil
	})

	select {
	case <-stop:
		eg.Go(func() error {
			// As we start to shutdown, disable keep-alives to avoid clients hanging onto connections.
			server.SetKeepAlivesEnabled(false)

			// Start failing readiness probes immediately.
			logger.Info("Starting to fail readiness probes...")
			drainer.Drain()

			return server.Shutdown(context.Background())
		})

		// Wait for all outstanding go routined to terminate, including our new one.
		return eg.Wait()

	case <-ctx.Done():
		return fmt.Errorf("webhook server bootstrap failed %w", ctx.Err())
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
