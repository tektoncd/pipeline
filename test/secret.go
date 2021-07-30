// +build e2e

/*
Copyright 2019 The Tekton Authors

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

package test

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// CreateGCPServiceAccountSecret will create a kube secret called secretName in namespace
// from the value in the GCP_SERVICE_ACCOUNT_KEY_PATH environment variable. If the env var
// doesn't exist, no secret will be created. Returns true if the secret was created, false
// otherwise.
func CreateGCPServiceAccountSecret(t *testing.T, c kubernetes.Interface, namespace string, secretName string) (bool, error) {
	t.Helper()
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	file := os.Getenv("GCP_SERVICE_ACCOUNT_KEY_PATH")
	if file == "" {
		t.Logf("Not creating service account secret, relying on default credentials in namespace %s.", namespace)
		return false, nil
	}

	sec := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: namespace,
		},
	}

	bs, err := ioutil.ReadFile(file)
	if err != nil {
		return false, fmt.Errorf("couldn't read secret json from %s: %w", file, err)
	}

	sec.Data = map[string][]byte{
		"config.json": bs,
	}
	_, err = c.CoreV1().Secrets(namespace).Create(ctx, sec, metav1.CreateOptions{})

	t.Log("Creating service account secret")
	return true, err
}
