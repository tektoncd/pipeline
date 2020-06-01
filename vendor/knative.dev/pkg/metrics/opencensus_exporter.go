/*
Copyright 2020 The Knative Authors

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

package metrics

import (
	"crypto/tls"
	"fmt"

	"contrib.go.opencensus.io/exporter/ocagent"
	"go.opencensus.io/stats/view"
	"go.uber.org/zap"
	"google.golang.org/grpc/credentials"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
)

func newOpenCensusExporter(config *metricsConfig, logger *zap.SugaredLogger) (view.Exporter, error) {
	opts := []ocagent.ExporterOption{ocagent.WithServiceName(config.component)}
	if config.collectorAddress != "" {
		opts = append(opts, ocagent.WithAddress(config.collectorAddress))
	}
	if config.requireSecure {
		opts = append(opts, ocagent.WithTLSCredentials(getCredentials(config.component, config.secret, logger)))
	} else {
		opts = append(opts, ocagent.WithInsecure())
	}
	e, err := ocagent.NewExporter(opts...)
	if err != nil {
		logger.Errorw("failed to create the OpenCensus exporter.", zap.Error(err))
		return nil, err
	}
	logger.Infof("created OpenCensus exporter with config: %+v.", *config)
	view.RegisterExporter(e)
	return e, nil
}

// getOpenCensusSecret attempts to locate a secret containing TLS credentials
// for communicating with the OpenCensus Agent. To do this, it first looks
// for a secret named "<component>-opencensus", then for a generic
// "opencensus" secret.
func getOpenCensusSecret(component string, lister SecretFetcher) (*corev1.Secret, error) {
	if lister == nil {
		return nil, fmt.Errorf("no secret lister provided for component %q; cannot use requireSecure=true", component)
	}
	secret, err := lister(component + "-opencensus")
	if errors.IsNotFound(err) {
		secret, err = lister("opencensus")
	}
	if err != nil {
		return nil, fmt.Errorf("unable to fetch opencensus secret for %q, cannot use requireSecure=true: %+v", component, err)
	}

	return secret, nil
}

// getCredentials attempts to create a certificate containing TLS credentials
// for communicating with the OpenCensus Agent.
func getCredentials(component string, secret *corev1.Secret, logger *zap.SugaredLogger) credentials.TransportCredentials {
	if secret == nil {
		logger.Errorf("no secret provided for component %q; cannot use requireSecure=true", component)
		return nil
	}
	return credentials.NewTLS(&tls.Config{
		GetClientCertificate: func(*tls.CertificateRequestInfo) (*tls.Certificate, error) {
			cert, err := tls.X509KeyPair(secret.Data["client-cert.pem"], secret.Data["client-key.pem"])
			if err != nil {
				return nil, err
			}
			return &cert, nil
		},
	})
}
