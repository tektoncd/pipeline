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

package certificates

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/kubernetes"
	corelisters "k8s.io/client-go/listers/core/v1"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/system"
	certresources "knative.dev/pkg/webhook/certificates/resources"
)

const (
	// Time used for updating a certificate before it expires.
	oneWeek = 7 * 24 * time.Hour
)

type reconciler struct {
	client       kubernetes.Interface
	secretlister corelisters.SecretLister
	secretName   string
	serviceName  string
}

var _ controller.Reconciler = (*reconciler)(nil)

// Reconcile implements controller.Reconciler
func (r *reconciler) Reconcile(ctx context.Context, key string) error {
	return r.reconcileCertificate(ctx)
}

func (r *reconciler) reconcileCertificate(ctx context.Context) error {
	logger := logging.FromContext(ctx)

	secret, err := r.secretlister.Secrets(system.Namespace()).Get(r.secretName)
	if apierrors.IsNotFound(err) {
		// The secret should be created explicitly by a higher-level system
		// that's responsible for install/updates.  We simply populate the
		// secret information.
		return nil
	} else if err != nil {
		logger.Errorf("Error accessing certificate secret %q: %v", r.secretName, err)
		return err
	}

	if _, haskey := secret.Data[certresources.ServerKey]; !haskey {
		logger.Infof("Certificate secret %q is missing key %q", r.secretName, certresources.ServerKey)
	} else if _, haskey := secret.Data[certresources.ServerCert]; !haskey {
		logger.Infof("Certificate secret %q is missing key %q", r.secretName, certresources.ServerCert)
	} else if _, haskey := secret.Data[certresources.CACert]; !haskey {
		logger.Infof("Certificate secret %q is missing key %q", r.secretName, certresources.CACert)
	} else {
		// Check the expiration date of the certificate to see if it needs to be updated
		cert, err := tls.X509KeyPair(secret.Data[certresources.ServerCert], secret.Data[certresources.ServerKey])
		if err != nil {
			logger.Warnf("Error creating pem from certificate and key: %v", err)
		} else {
			certData, err := x509.ParseCertificate(cert.Certificate[0])
			if err != nil {
				logger.Errorf("Error parsing certificate: %v", err)
			} else if time.Now().Add(oneWeek).Before(certData.NotAfter) {
				return nil
			}
		}
	}
	// Don't modify the informer copy.
	secret = secret.DeepCopy()

	// One of the secret's keys is missing, so synthesize a new one and update the secret.
	newSecret, err := certresources.MakeSecret(ctx, r.secretName, system.Namespace(), r.serviceName)
	if err != nil {
		return err
	}
	secret.Data = newSecret.Data
	_, err = r.client.CoreV1().Secrets(secret.Namespace).Update(secret)
	return err
}
