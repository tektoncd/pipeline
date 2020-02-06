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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/kubernetes"
	corelisters "k8s.io/client-go/listers/core/v1"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/system"
	certresources "knative.dev/pkg/webhook/certificates/resources"
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
		secret, err = certresources.MakeSecret(ctx, r.secretName, system.Namespace(), r.serviceName)
		if err != nil {
			return err
		}
		secret, err = r.client.CoreV1().Secrets(secret.Namespace).Create(secret)
		if err != nil {
			return err
		}
	} else if err != nil {
		logger.Errorf("error accessing certificate secret %q: %v", r.secretName, err)
		return err
	}

	if _, haskey := secret.Data[certresources.ServerKey]; !haskey {
		logger.Infof("Certificate secret %q is missing key %q", r.secretName, certresources.ServerKey)
	} else if _, haskey := secret.Data[certresources.ServerCert]; !haskey {
		logger.Infof("Certificate secret %q is missing key %q", r.secretName, certresources.ServerCert)
	} else if _, haskey := secret.Data[certresources.CACert]; !haskey {
		logger.Infof("Certificate secret %q is missing key %q", r.secretName, certresources.CACert)
	} else {
		// It has all of the keys, it's good.
		return nil
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
