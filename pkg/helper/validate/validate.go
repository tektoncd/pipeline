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
	"fmt"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8s "k8s.io/client-go/kubernetes"
)

const (
	fieldNotPresent = ""
	pendingState    = "---"
)

type params interface {
	KubeClient() (k8s.Interface, error)
	Namespace() string
}

// Check if namespace exists. Returns error if namespace specified with -n doesn't exist or if user doesn't have permissions to view.
func NamespaceExists(p params) error {

	cs, err := p.KubeClient()
	if err != nil {
		return fmt.Errorf("failed to create kube client")
	}

	_, err = cs.CoreV1().Namespaces().Get(p.Namespace(), metav1.GetOptions{})
	if err != nil {
		return err
	}

	return nil
}

// Check if TaskRef exists on a TaskRunSpec. Returns empty string if not present.
func TaskRefExists(spec v1alpha1.TaskRunSpec) string {

	if spec.TaskRef == nil {
		return fieldNotPresent
	}

	return spec.TaskRef.Name
}

// Check if step is in waiting, running, or terminated state by checking StepState of the step.
func StepReasonExists(state v1alpha1.StepState) string {

	if state.Waiting == nil {

		if state.Running != nil {
			return "Running"
		}

		if state.Terminated != nil {
			return state.Terminated.Reason
		}

		return pendingState
	}

	return state.Waiting.Reason
}
