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
	"k8s.io/apimachinery/pkg/runtime"
)

// MultiCertWriter composes a slice of CertWriters.
// This is useful if you need both SecretCertWriter and FSCertWriter.
type MultiCertWriter struct {
	CertWriters []CertWriter
}

var _ CertWriter = &MultiCertWriter{}

// EnsureCerts provisions certificates for a webhook configuration by invoking each CertWrite.
func (s *MultiCertWriter) EnsureCerts(webhookConfig runtime.Object) error {
	var err error
	for _, certWriter := range s.CertWriters {
		err = certWriter.EnsureCerts(webhookConfig)
		if err != nil {
			return err
		}
	}
	return nil
}
