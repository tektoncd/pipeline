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
	"io/ioutil"
	"os"
	"path"
	"strings"

	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	certgenerator "sigs.k8s.io/controller-runtime/pkg/admission/cert/generator"
	"sigs.k8s.io/controller-runtime/pkg/admission/cert/writer/internal/atomic"
)

const (
	// FSCertProvisionAnnotationKeyPrefix should be used in an annotation in the following format:
	// fs.certprovisioner.kubernetes.io/<webhook-name>: path/to/certs/
	// the webhook cert manager library will provision the certificate for the webhook by
	// storing it under the specified path.
	// format: fs.certprovisioner.kubernetes.io/webhookName: path/to/certs/
	FSCertProvisionAnnotationKeyPrefix = "fs.certprovisioner.kubernetes.io/"
)

// FSCertWriter provisions the certificate by reading and writing to the filesystem.
type FSCertWriter struct {
	CertGenerator certgenerator.CertGenerator
}

var _ CertWriter = &FSCertWriter{}

// EnsureCerts provisions certificates for a webhook configuration by writing them in the filesystem.
func (f *FSCertWriter) EnsureCerts(webhookConfig runtime.Object) error {
	if webhookConfig == nil {
		return errors.New("unexpected nil webhook configuration object")
	}

	fsWebhookMap := map[string]*webhookAndPath{}
	accessor, err := meta.Accessor(webhookConfig)
	if err != nil {
		return err
	}
	annotations := accessor.GetAnnotations()
	// Parse the annotations to extract info
	f.parseAnnotations(annotations, fsWebhookMap)

	webhooks, err := getWebhooksFromObject(webhookConfig)
	if err != nil {
		return err
	}
	for i, webhook := range webhooks {
		if p, found := fsWebhookMap[webhook.Name]; found {
			p.webhook = &webhooks[i]
		}
	}

	// validation
	for k, v := range fsWebhookMap {
		if v.webhook == nil {
			return fmt.Errorf("expecting a webhook named %q", k)
		}
	}

	generator := f.CertGenerator
	if f.CertGenerator == nil {
		generator = &certgenerator.SelfSignedCertGenerator{}
	}

	cw := &fsCertWriter{
		certGenerator: generator,
		webhookConfig: webhookConfig,
		webhookMap:    fsWebhookMap,
	}
	return cw.ensureCert()
}

func (f *FSCertWriter) parseAnnotations(annotations map[string]string, fsWebhookMap map[string]*webhookAndPath) {
	for k, v := range annotations {
		if strings.HasPrefix(k, FSCertProvisionAnnotationKeyPrefix) {
			webhookName := strings.TrimPrefix(k, FSCertProvisionAnnotationKeyPrefix)
			fsWebhookMap[webhookName] = &webhookAndPath{
				path: v,
			}
		}
	}
}

// fsCertWriter deals with writing to the local filesystem.
type fsCertWriter struct {
	certGenerator certgenerator.CertGenerator

	webhookConfig runtime.Object
	webhookMap    map[string]*webhookAndPath
}

type webhookAndPath struct {
	webhook *admissionregistrationv1beta1.Webhook
	path    string
}

var _ certReadWriter = &fsCertWriter{}

func (f *fsCertWriter) ensureCert() error {
	var err error
	for _, v := range f.webhookMap {
		err = handleCommon(v.webhook, f)
		if err != nil {
			return err
		}
	}
	return nil
}

func (f *fsCertWriter) write(webhookName string) (*certgenerator.Artifacts, error) {
	return f.doWrite(webhookName)
}

func (f *fsCertWriter) overwrite(webhookName string) (*certgenerator.Artifacts, error) {
	return f.doWrite(webhookName)
}

func (f *fsCertWriter) doWrite(webhookName string) (*certgenerator.Artifacts, error) {
	v := f.webhookMap[webhookName]
	commonName, err := dnsNameForWebhook(&v.webhook.ClientConfig)
	if err != nil {
		return nil, err
	}
	certs, err := f.certGenerator.Generate(commonName)
	if err != nil {
		return nil, err
	}
	aw, err := atomic.NewAtomicWriter(v.path, log.WithName("atomic-writer").WithValues("task", "processing webhook", "webhook", webhookName))
	if err != nil {
		return nil, err
	}
	// AtomicWriter's algorithm only manages files using symbolic link.
	// If a file is not a symbolic link, will ignore the update for it.
	// We want to cleanup for AtomicWriter by removing old files that are not symbolic links.
	prepareToWrite(v.path)
	err = aw.Write(certToProjectionMap(certs))
	return certs, err
}

func prepareToWrite(dir string) {
	filenames := []string{CACertName, ServerCertName, ServerKeyName}
	for _, f := range filenames {
		abspath := path.Join(dir, f)
		_, err := os.Stat(abspath)
		if os.IsNotExist(err) {
			continue
		} else if err != nil {
			log.Error(err, "unable to stat file", "file", abspath)
		}
		_, err = os.Readlink(abspath)
		// if it's not a symbolic link
		if err != nil {
			err = os.Remove(abspath)
			if err != nil {
				log.Error(err, "unable to remove old file", "file", abspath)
			}
		}
	}
}

func (f *fsCertWriter) read(webhookName string) (*certgenerator.Artifacts, error) {
	dir := f.webhookMap[webhookName].path
	if err := ensureExist(dir); err != nil {
		return nil, err
	}
	caBytes, err := ioutil.ReadFile(path.Join(dir, CACertName))
	if err != nil {
		return nil, err
	}
	certBytes, err := ioutil.ReadFile(path.Join(dir, ServerCertName))
	if err != nil {
		return nil, err
	}
	keyBytes, err := ioutil.ReadFile(path.Join(dir, ServerKeyName))
	if err != nil {
		return nil, err
	}
	return &certgenerator.Artifacts{
		CACert: caBytes,
		Cert:   certBytes,
		Key:    keyBytes,
	}, nil
}

func ensureExist(dir string) error {
	filenames := []string{CACertName, ServerCertName, ServerKeyName}
	for _, filename := range filenames {
		_, err := os.Stat(path.Join(dir, filename))
		switch {
		case err == nil:
			continue
		case os.IsNotExist(err):
			return notFoundError{err}
		default:
			return err
		}
	}
	return nil
}

func certToProjectionMap(cert *certgenerator.Artifacts) map[string]atomic.FileProjection {
	// TODO: figure out if we can reduce the permission. (Now it's 0666)
	return map[string]atomic.FileProjection{
		CACertName: {
			Data: cert.CACert,
			Mode: 0666,
		},
		ServerCertName: {
			Data: cert.Cert,
			Mode: 0666,
		},
		ServerKeyName: {
			Data: cert.Key,
			Mode: 0666,
		},
	}
}
