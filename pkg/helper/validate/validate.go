// Copyright © 2019 The Tekton Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package validate

import (
	k8s "k8s.io/client-go/kubernetes"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Check if namespace exists. Returns error if namespace specified with -n doesn't exist or if user doesn't have permissions to view.
func NamespaceExists(kube k8s.Interface, ns string) error {

	_, err := kube.CoreV1().Namespaces().Get(ns, metav1.GetOptions{})
	if err != nil {
		return err
	}

	return nil
}
