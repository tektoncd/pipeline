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

package taskrun

import (
	"fmt"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"knative.dev/pkg/apis"
)

// cancelTaskRun marks the TaskRun as cancelled and deletes pods linked to it.
func cancelTaskRun(tr *v1alpha1.TaskRun, clientset kubernetes.Interface) error {
	tr.Status.SetCondition(&apis.Condition{
		Type:    apis.ConditionSucceeded,
		Status:  corev1.ConditionFalse,
		Reason:  "TaskRunCancelled",
		Message: fmt.Sprintf("TaskRun %q was cancelled", tr.Name),
	})

	pod, err := getPod(tr, clientset)
	if err != nil {
		return err
	}

	return clientset.CoreV1().Pods(tr.Namespace).Delete(pod.Name, &metav1.DeleteOptions{})
}
