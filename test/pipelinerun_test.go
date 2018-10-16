// +build e2e

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

package test

import (
	"fmt"
	"strings"
	"testing"

	buildv1alpha1 "github.com/knative/build/pkg/apis/build/v1alpha1"
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	knativetest "github.com/knative/pkg/test"
	"github.com/knative/pkg/test/logging"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/knative/build-pipeline/pkg/apis/pipeline/v1alpha1"
)

func TestPipelineRun(t *testing.T) {
	logger := logging.GetContextLogger(t.Name())
	c, namespace := setup(t, logger)

	knativetest.CleanupOnInterrupt(func() { tearDown(logger, c.KubeClient, namespace) }, logger)
	defer tearDown(logger, c.KubeClient, namespace)

	logger.Infof("Creating Pipeline Resources in namespace %s", namespace)
	if _, err := c.TaskClient.Create(getHelloWorldTask(namespace, []string{"echo", taskOutput})); err != nil {
		t.Fatalf("Failed to create Task `%s`: %s", hwTaskName, err)
	}
	if _, err := c.PipelineClient.Create(getHelloWorldPipeline(namespace)); err != nil {
		t.Fatalf("Failed to create Pipeline `%s`: %s", hwPipelineName, err)
	}
	if _, err := c.PipelineParamsClient.Create(getHelloWorldPipelineParams(namespace)); err != nil {
		t.Fatalf("Failed to create PipelineParams `%s`: %s", hwPipelineParamsName, err)
	}
	if _, err := c.PipelineRunClient.Create(getHelloWorldPipelineRun(namespace)); err != nil {
		t.Fatalf("Failed to create PipelineRun `%s`: %s", hwPipelineRunName, err)
	}

	logger.Infof("Waiting for PipelineRun %s in namespace %s to complete", hwPipelineRunName, namespace)
	if err := WaitForPipelineRunState(c, hwPipelineRunName, func(tr *v1alpha1.PipelineRun) (bool, error) {
		c := tr.Status.GetCondition(duckv1alpha1.ConditionSucceeded)
		if c != nil {
			if c.Status == corev1.ConditionTrue {
				return true, nil
			} else if c.Status == corev1.ConditionFalse {
				return true, fmt.Errorf("pipeline run %s failed!", hwPipelineRunName)
			}
		}
		return false, nil
	}, "PipelineRunSuccess"); err != nil {
		t.Errorf("Error waiting for PipelineRun %s to finish: %s", hwTaskRunName, err)
	}
	logger.Infof("Making sure the expected TaskRuns were created")
	expectedTaskRuns := []string{
		strings.Join([]string{hwPipelineRunName, hwPipelineTaskName1}, "-"),
		strings.Join([]string{hwPipelineRunName, hwPipelineTaskName2}, "-"),
	}
	for _, runName := range expectedTaskRuns {
		r, err := c.TaskRunClient.Get(runName, metav1.GetOptions{})
		if err != nil {
			t.Errorf("Couldn't get expected TaskRun %s: %s", runName, err)
		} else {
			c := r.Status.GetCondition(duckv1alpha1.ConditionSucceeded)
			if c.Status != corev1.ConditionTrue {
				t.Errorf("Expected TaskRun %s to have succeeded but Status is %s", runName, c.Status)
			}
		}
	}
}

// Test pipeline trun which refers to service account.
// create service account
func TestPipelineRun_WithServiceAccount(t *testing.T) {
	logger := logging.GetContextLogger(t.Name())
	c, namespace := setup(t, logger)

	knativetest.CleanupOnInterrupt(func() { tearDown(logger, c.KubeClient, namespace) }, logger)
	defer tearDown(logger, c.KubeClient, namespace)

	logger.Infof("Creating pipeline resources in namespace %s", namespace)

	if _, err := c.KubeClient.Kube.CoreV1().Secrets(namespace).Create(createPipelineRunSecret(namespace)); err != nil {
		t.Fatalf("Failed to create secret `%s`: %s", hwSecret, err)
	}

	if _, err := c.KubeClient.Kube.CoreV1().ServiceAccounts(namespace).Create(createPipelineRunServiceAccount(namespace)); err != nil {
		t.Fatalf("Failed to create SA `%s`: %s", hwSA, err)
	}

	pp := getHelloWorldPipelineParams(namespace)
	// Set SA pipeline params spec
	pp.Spec.ServiceAccount = hwSA
	if _, err := c.PipelineParamsClient.Create(pp); err != nil {
		t.Fatalf("Failed to create PipelineParams `%s`: %s", hwPipelineParamsName, err)
	}

	if _, err := c.TaskClient.Create(&v1alpha1.Task{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      hwTaskName,
		},
		Spec: v1alpha1.TaskSpec{
			// Reference build: https://github.com/knative/build/tree/master/test/docker-basic
			BuildSpec: &buildv1alpha1.BuildSpec{
				Steps: []corev1.Container{
					corev1.Container{
						Name:  "config-docker",
						Image: "gcr.io/cloud-builders/docker",
						// Private docker image for Build CRD testing
						Args: []string{"pull", "gcr.io/build-crd-testing/secret-sauce"},
						VolumeMounts: []corev1.VolumeMount{{
							Name:      "docker-socket",
							MountPath: "/var/run/docker.sock",
						}},
					},
				},
				Volumes: []corev1.Volume{{
					Name: "docker-socket",
					VolumeSource: corev1.VolumeSource{
						HostPath: &corev1.HostPathVolumeSource{
							Path: "/var/run/docker.sock",
							Type: newHostPathType(string(corev1.HostPathSocket)),
						},
					},
				}},
			},
		},
	}); err != nil {
		t.Fatalf("Failed to create Task `%s`: %s", hwTaskName, err)
	}

	if _, err := c.PipelineClient.Create(getHelloWorldPipelineWithSingularTask(namespace)); err != nil {
		t.Fatalf("Failed to create Pipeline `%s`: %s", hwPipelineName, err)
	}

	if _, err := c.PipelineRunClient.Create(getHelloWorldPipelineRun(namespace)); err != nil {
		t.Fatalf("Failed to create PipelineRun `%s`: %s", hwPipelineRunName, err)
	}

	logger.Infof("Waiting for PipelineRun %s in namespace %s to complete", hwPipelineRunName, namespace)
	if err := WaitForPipelineRunState(c, hwPipelineRunName, func(tr *v1alpha1.PipelineRun) (bool, error) {
		c := tr.Status.GetCondition(duckv1alpha1.ConditionSucceeded)
		if c != nil {
			if c.Status == corev1.ConditionTrue {
				return true, nil
			} else if c.Status == corev1.ConditionFalse {
				return true, fmt.Errorf("pipeline run %s failed! status info %#v", hwPipelineRunName, c.Status)
			}
		}
		return false, nil
	}, "PipelineRunSuccess"); err != nil {
		t.Errorf("Error waiting for PipelineRun %s to finish: %s", hwTaskRunName, err)
	}

	logger.Infof("Making sure the expected TaskRuns were created")
	expectedTaskRun := strings.Join([]string{hwPipelineRunName, hwPipelineTaskName1}, "-")

	tr, err := c.TaskRunClient.Get(expectedTaskRun, metav1.GetOptions{})
	if err != nil {
		t.Errorf("Couldn't get expected TaskRun %s: %s", expectedTaskRun, err)
	} else {
		c := tr.Status.GetCondition(duckv1alpha1.ConditionSucceeded)
		if c.Status != corev1.ConditionTrue {
			t.Errorf("Expected TaskRun %s to have succeeded but Status is %s", expectedTaskRun, c.Status)
		}
	}
}
func newHostPathType(pathType string) *corev1.HostPathType {
	hostPathType := new(corev1.HostPathType)
	*hostPathType = corev1.HostPathType(pathType)
	return hostPathType
}
