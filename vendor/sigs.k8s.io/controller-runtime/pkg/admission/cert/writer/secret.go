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
	"errors"
	"fmt"
	"strings"

	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/admission/cert/generator"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	// SecretCertProvisionAnnotationKeyPrefix should be used in an annotation in the following format:
	// secret.certprovisioner.kubernetes.io/<webhook-name>: <secret-namespace>/<secret-name>
	// the webhook cert manager library will provision the certificate for the webhook by
	// storing it in the specified secret.
	SecretCertProvisionAnnotationKeyPrefix = "secret.certprovisioner.kubernetes.io/"
)

// SecretCertWriter provisions the certificate by reading and writing to the k8s secrets.
type SecretCertWriter struct {
	Client        client.Client
	CertGenerator generator.CertGenerator
}

var _ CertWriter = &SecretCertWriter{}

// EnsureCerts provisions certificates for a webhook configuration by writing them in k8s secrets.
func (s *SecretCertWriter) EnsureCerts(webhookConfig runtime.Object) error {
	if webhookConfig == nil {
		return errors.New("unexpected nil webhook configuration object")
	}

	secretWebhookMap := map[string]*webhookAndSecret{}
	accessor, err := meta.Accessor(webhookConfig)
	if err != nil {
		return err
	}
	annotations := accessor.GetAnnotations()
	// Parse the annotations to extract info
	s.parseAnnotations(annotations, secretWebhookMap)

	webhooks, err := getWebhooksFromObject(webhookConfig)
	if err != nil {
		return err
	}
	for i, webhook := range webhooks {
		if s, found := secretWebhookMap[webhook.Name]; found {
			s.webhook = &webhooks[i]
		}
	}

	// validation
	for k, v := range secretWebhookMap {
		if v.webhook == nil {
			return fmt.Errorf("expecting a webhook named %q", k)
		}
	}

	certGenerator := s.CertGenerator
	if s.CertGenerator == nil {
		certGenerator = &generator.SelfSignedCertGenerator{}
	}

	srw := &secretReadWriter{
		client:        s.Client,
		certGenerator: certGenerator,
		webhookConfig: webhookConfig,
		webhookMap:    secretWebhookMap,
	}
	return srw.ensureCert()
}

func (s *SecretCertWriter) parseAnnotations(annotations map[string]string, secretWebhookMap map[string]*webhookAndSecret) {
	for k, v := range annotations {
		if strings.HasPrefix(k, SecretCertProvisionAnnotationKeyPrefix) {
			webhookName := strings.TrimPrefix(k, SecretCertProvisionAnnotationKeyPrefix)
			secretWebhookMap[webhookName] = &webhookAndSecret{
				secret: types.NewNamespacedNameFromString(v),
			}
		}
	}
}

func (s *secretReadWriter) ensureCert() error {
	for _, v := range s.webhookMap {
		err := handleCommon(v.webhook, s)
		if err != nil {
			return err
		}
	}
	return nil
}

// secretReadWriter deals with writing to the k8s secrets.
type secretReadWriter struct {
	client        client.Client
	certGenerator generator.CertGenerator

	webhookConfig runtime.Object
	webhookMap    map[string]*webhookAndSecret
}

type webhookAndSecret struct {
	webhook *admissionregistrationv1beta1.Webhook
	secret  types.NamespacedName
}

var _ certReadWriter = &secretReadWriter{}

func (s *secretReadWriter) buildSecret(webhookName string) (*corev1.Secret, *generator.Artifacts, error) {
	v := s.webhookMap[webhookName]

	webhook := v.webhook
	commonName, err := dnsNameForWebhook(&webhook.ClientConfig)
	if err != nil {
		return nil, nil, err
	}
	certs, err := s.certGenerator.Generate(commonName)
	if err != nil {
		return nil, nil, err
	}
	secret := certsToSecret(certs, v.secret)
	err = controllerutil.SetControllerReference(s.webhookConfig.(metav1.Object), secret, scheme.Scheme)
	return secret, certs, err
}

func (s *secretReadWriter) write(webhookName string) (*generator.Artifacts, error) {
	secret, certs, err := s.buildSecret(webhookName)
	if err != nil {
		return nil, err
	}
	err = s.client.Create(nil, secret)
	if apierrors.IsAlreadyExists(err) {
		return nil, alreadyExistError{err}
	}
	return certs, err
}

func (s *secretReadWriter) overwrite(webhookName string) (
	*generator.Artifacts, error) {
	secret, certs, err := s.buildSecret(webhookName)
	if err != nil {
		return nil, err
	}
	err = s.client.Update(nil, secret)
	return certs, err
}

func (s *secretReadWriter) read(webhookName string) (*generator.Artifacts, error) {
	v := s.webhookMap[webhookName]
	secret := &corev1.Secret{}
	err := s.client.Get(nil, v.secret, secret)
	if apierrors.IsNotFound(err) {
		return nil, notFoundError{err}
	}
	return secretToCerts(secret), err
}

func secretToCerts(secret *corev1.Secret) *generator.Artifacts {
	if secret.Data == nil {
		return nil
	}
	return &generator.Artifacts{
		CACert: secret.Data[CACertName],
		Cert:   secret.Data[ServerCertName],
		Key:    secret.Data[ServerKeyName],
	}
}

func certsToSecret(certs *generator.Artifacts, sec types.NamespacedName) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: sec.Namespace,
			Name:      sec.Name,
		},
		Data: map[string][]byte{
			CACertName:     certs.CACert,
			ServerKeyName:  certs.Key,
			ServerCertName: certs.Cert,
		},
	}
}
