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

/*
Package writer provides method to ensure each webhook has a working certificate and private key in the right
place for consuming.

It will create the certificates if they don't exist.
It will ensure the certificates are valid and not expiring. If not, it will recreate them.

Example Webhook Configuration

There is an example annotation to get the webhook managed by the SecretCertWriter.
SecretCertProvisionAnnotationKeyPrefix is the prefix of the annotation key.

	secret.certprovisioner.kubernetes.io/webhook-1: namespace-bar/secret-foo

The following is an example MutatingWebhookConfiguration in yaml.

	apiVersion: admissionregistration.k8s.io/v1beta1
	kind: MutatingWebhookConfiguration
	metadata:
	  name: myMutatingWebhookConfiguration
	  annotations:
	    secret.certprovisioner.kubernetes.io/webhook-1: namespace-bar/secret-foo
	webhooks:
	- name: webhook-1
	  rules:
	  - apiGroups:
		- ""
		apiVersions:
		- v1
		operations:
		- "*"
		resources:
		- pods
	  clientConfig:
		service:
		  namespace: service-ns-1
		  name: service-foo
		  path: "/mutating-pods"
		caBundle: [] # CA bundle here

Create a default CertWriter

	writer, err := NewCertWriter(Options{client: client}))
	if err != nil {
		// handler error
	}

Create a SecretCertWriter

	writer, err := &SecretCertWriter{
		Client: client
	}
	if err != nil {
		// handler error
	}

Provision the certificates using the CertWriter. The certificate will be available in the desired secrets or
the desired path.

	// writer can be either one of the CertWriters created above
	err = writer.EnsureCerts(webhookConfiguration) // webhookConfiguration is an existing runtime.Object
	if err != nil {
		// handler error
	}

*/
package writer

import (
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

var log = logf.KBLog.WithName("admission").WithName("cert").WithName("writer")
