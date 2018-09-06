/*
Copyright 2018 The Kubernetes Authors.

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

package writer

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"net/url"
	"time"

	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/admission/cert/generator"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// CACertName is the name of the CA certificate
	CACertName = "ca-cert.pem"
	// ServerKeyName is the name of the server private key
	ServerKeyName = "key.pem"
	// ServerCertName is the name of the serving certificate
	ServerCertName = "cert.pem"
)

// CertWriter provides method to handle webhooks.
type CertWriter interface {
	// EnsureCert ensures that the webhooks have proper certificates.
	EnsureCerts(runtime.Object) error
}

// Options are options for configuring a CertWriter.
type Options struct {
	Client        client.Client
	CertGenerator generator.CertGenerator
}

// NewCertWriter builds a new CertWriter using the provided options.
// By default, it builds a MultiCertWriter that is composed of a SecretCertWriter and a FSCertWriter.
func NewCertWriter(ops Options) (CertWriter, error) {
	if ops.CertGenerator == nil {
		ops.CertGenerator = &generator.SelfSignedCertGenerator{}
	}
	if ops.Client == nil {
		// TODO: default the client if possible
		return nil, errors.New("Options.Client is required")
	}
	s := &SecretCertWriter{
		Client:        ops.Client,
		CertGenerator: ops.CertGenerator,
	}
	f := &FSCertWriter{
		CertGenerator: ops.CertGenerator,
	}
	return &MultiCertWriter{
		CertWriters: []CertWriter{
			s,
			f,
		},
	}, nil
}

// handleCommon ensures the given webhook has a proper certificate.
// It uses the given certReadWriter to read and (or) write the certificate.
func handleCommon(webhook *admissionregistrationv1beta1.Webhook, ch certReadWriter) error {
	if webhook == nil {
		return nil
	}
	if ch == nil {
		return errors.New("certReaderWriter should not be nil")
	}

	certs, err := createIfNotExists(webhook.Name, ch)
	if err != nil {
		return err
	}

	dnsName, err := dnsNameForWebhook(&webhook.ClientConfig)
	if err != nil {
		return err
	}
	// Recreate the cert if it's invalid.
	valid := validCert(certs, dnsName)
	if !valid {
		log.Info("cert is invalid or expiring, regenerating a new one")
		certs, err = ch.overwrite(webhook.Name)
		if err != nil {
			return err
		}
	}

	// Ensure the CA bundle in the webhook configuration has the signing CA.
	caBundle := webhook.ClientConfig.CABundle
	caCert := certs.CACert
	if !bytes.Contains(caBundle, caCert) {
		webhook.ClientConfig.CABundle = append(caBundle, caCert...)
	}
	return nil
}

func createIfNotExists(webhookName string, ch certReadWriter) (*generator.Artifacts, error) {
	// Try to read first
	certs, err := ch.read(webhookName)
	if isNotFound(err) {
		// Create if not exists
		certs, err = ch.write(webhookName)
		switch {
		// This may happen if there is another racer.
		case isAlreadyExists(err):
			certs, err = ch.read(webhookName)
			if err != nil {
				return certs, err
			}
		case err != nil:
			return certs, err
		}
	} else if err != nil {
		return certs, err
	}
	return certs, nil
}

// certReadWriter provides methods for reading and writing certificates.
type certReadWriter interface {
	// read reads a wehbook name and returns the certs for it.
	read(webhookName string) (*generator.Artifacts, error)
	// write writes the certs and return the certs it wrote.
	write(webhookName string) (*generator.Artifacts, error)
	// overwrite overwrites the existing certs and return the certs it wrote.
	overwrite(webhookName string) (*generator.Artifacts, error)
}

func validCert(certs *generator.Artifacts, dnsName string) bool {
	if certs == nil {
		return false
	}

	// Verify key and cert are valid pair
	_, err := tls.X509KeyPair(certs.Cert, certs.Key)
	if err != nil {
		return false
	}

	// Verify cert is good for desired DNS name and signed by CA and will be valid for desired period of time.
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(certs.CACert) {
		return false
	}
	block, _ := pem.Decode([]byte(certs.Cert))
	if block == nil {
		return false
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return false
	}
	ops := x509.VerifyOptions{
		DNSName:     dnsName,
		Roots:       pool,
		CurrentTime: time.Now().AddDate(0, 6, 0),
	}
	_, err = cert.Verify(ops)
	return err == nil
}

func getWebhooksFromObject(obj runtime.Object) ([]admissionregistrationv1beta1.Webhook, error) {
	switch typed := obj.(type) {
	case *admissionregistrationv1beta1.MutatingWebhookConfiguration:
		return typed.Webhooks, nil
	case *admissionregistrationv1beta1.ValidatingWebhookConfiguration:
		return typed.Webhooks, nil
	//case *unstructured.Unstructured:
	// TODO: implement this if needed
	default:
		return nil, fmt.Errorf("unsupported type: %T, only support v1beta1.MutatingWebhookConfiguration and v1beta1.ValidatingWebhookConfiguration", typed)
	}
}

func dnsNameForWebhook(config *admissionregistrationv1beta1.WebhookClientConfig) (string, error) {
	if config.Service != nil && config.URL != nil {
		return "", fmt.Errorf("service and URL can't be set at the same time in a webhook: %v", config)
	}
	if config.Service == nil && config.URL == nil {
		return "", fmt.Errorf("one of service and URL need to be set in a webhook: %v", config)
	}
	if config.Service != nil {
		return generator.ServiceToCommonName(config.Service.Namespace, config.Service.Name), nil
	}
	// config.URL != nil
	u, err := url.Parse(*config.URL)
	return u.Host, err

}
