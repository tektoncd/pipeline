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
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestCreateCerts(t *testing.T) {
	sKey, serverCertPEM, caCertBytes, err := CreateCerts(context.TODO(), "got-the-hook", "knative-webhook")
	if err != nil {
		t.Fatalf("Failed to create certs %v", err)
	}

	// Test server private key
	p, _ := pem.Decode(sKey)
	if p.Type != "RSA PRIVATE KEY" {
		t.Fatal("Expected the key to be RSA Private key type")
	}
	key, err := x509.ParsePKCS1PrivateKey(p.Bytes)
	if err != nil {
		t.Fatalf("Failed to parse private key %v", err)
	}
	if err := key.Validate(); err != nil {
		t.Fatalf("Failed to validate private key")
	}

	// Test Server Cert
	sCert, err := validCertificate(serverCertPEM, t)
	if err != nil {
		t.Fatal(err)
	}

	// Test CA Cert
	caParsedCert, err := validCertificate(caCertBytes, t)
	if err != nil {
		t.Fatal(err)
	}

	// Verify domain names
	expectedDNSNames := []string{
		"got-the-hook.knative-webhook",
		"got-the-hook.knative-webhook.svc",
		"got-the-hook.knative-webhook.svc.cluster.local",
	}
	if diff := cmp.Diff(caParsedCert.DNSNames, expectedDNSNames); diff != "" {
		t.Fatalf("Unexpected CA Cert DNS Name (-want +got) : %v", diff)
	}

	// Verify Server Cert is Signed by CA Cert
	if err = sCert.CheckSignatureFrom(caParsedCert); err != nil {
		t.Fatal("Failed to verify that the signature on server certificate is from parent CA cert", err)
	}
}

func validCertificate(cert []byte, t *testing.T) (*x509.Certificate, error) {
	t.Helper()
	caCert, _ := pem.Decode(cert)
	if caCert.Type != "CERTIFICATE" {
		return nil, fmt.Errorf("Expected %s but got %s", "CERTIFICATE", caCert.Type)
	}
	parsedCert, err := x509.ParseCertificate(caCert.Bytes)
	if err != nil {
		return nil, fmt.Errorf("Failed to parse cert %v", err)
	}
	if parsedCert.SignatureAlgorithm != x509.SHA256WithRSA {
		return nil, fmt.Errorf("Failed to match signature. Expect %s but got %s", x509.SHA256WithRSA, parsedCert.SignatureAlgorithm)
	}
	return parsedCert, nil
}
