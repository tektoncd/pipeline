// +build e2e

/*
Copyright 2018 Knative Authors LLC
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
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/knative/build-pipeline/pkg/apis/pipeline/v1alpha1"
	buildv1alpha1 "github.com/knative/build/pkg/apis/build/v1alpha1"
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	knativetest "github.com/knative/pkg/test"
	"github.com/knative/pkg/test/logging"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

const (
	sourceResourceName        = "go-helloworld-git"
	sourceImageName           = "go-helloworld-image"
	createImageTaskName       = "create-image-task"
	helmDeployTaskName        = "helm-deploy-task"
	helmDeployPipelineName    = "helm-deploy-pipeline"
	helmDeployPipelineRunName = "helm-deploy-pipeline-run"
	helmDeployServiceName     = "gohelloworld-chart"
)

var (
	imageName            string
	clusterRoleBindings  [3]*rbacv1.ClusterRoleBinding
	tillerServiceAccount *corev1.ServiceAccount
)

// TestHelmDeployPipelineRun is an integration test that will verify a pipeline build an image
// and then using helm to deploy it
func TestHelmDeployPipelineRun(t *testing.T) {
	logger := logging.GetContextLogger(t.Name())
	c, namespace := setup(t, logger)
	setupClusterBindingForHelm(c, t, namespace, logger)

	knativetest.CleanupOnInterrupt(func() { tearDown(t, logger, c, namespace) }, logger)
	defer tearDown(t, logger, c, namespace)

	logger.Infof("Creating Git PipelineResource %s", sourceResourceName)
	if _, err := c.PipelineResourceClient.Create(getGoHelloworldGitResource(namespace)); err != nil {
		t.Fatalf("Failed to create Pipeline Resource `%s`: %s", sourceResourceName, err)
	}

	logger.Infof("Creating Task %s", createImageTaskName)
	if _, err := c.TaskClient.Create(getCreateImageTask(namespace, t)); err != nil {
		t.Fatalf("Failed to create Task `%s`: %s", createImageTaskName, err)
	}

	logger.Infof("Creating Task %s", helmDeployTaskName)
	if _, err := c.TaskClient.Create(getHelmDeployTask(namespace)); err != nil {
		t.Fatalf("Failed to create Task `%s`: %s", helmDeployTaskName, err)
	}

	logger.Infof("Creating Pipeline %s", helmDeployPipelineName)
	if _, err := c.PipelineClient.Create(getHelmDeployPipeline(namespace)); err != nil {
		t.Fatalf("Failed to create Pipeline `%s`: %s", helmDeployPipelineName, err)
	}

	logger.Infof("Creating PipelineRun %s", helmDeployPipelineRunName)
	if _, err := c.PipelineRunClient.Create(getHelmDeployPipelineRun(namespace)); err != nil {
		t.Fatalf("Failed to create Pipeline `%s`: %s", helmDeployPipelineRunName, err)
	}

	// Verify status of PipelineRun (wait for it)
	if err := WaitForPipelineRunState(c, helmDeployPipelineRunName, func(pr *v1alpha1.PipelineRun) (bool, error) {
		c := pr.Status.GetCondition(duckv1alpha1.ConditionSucceeded)
		if c != nil {
			if c.Status == corev1.ConditionTrue {
				return true, nil
			} else if c.Status == corev1.ConditionFalse {
				return true, fmt.Errorf("pipeline run %s failed!", helmDeployPipelineRunName)
			}
		}
		return false, nil
	}, "PipelineRunCompleted"); err != nil {
		taskruns, err := c.TaskRunClient.List(metav1.ListOptions{})
		if err != nil {
			t.Errorf("Error getting TaskRun list for PipelineRun %s %s", helmDeployPipelineRunName, err)
		}
		for _, tr := range taskruns.Items {
			CollectBuildLogs(c, tr.Name, namespace, logger)
		}
		t.Errorf("Error waiting for PipelineRun %s to finish: %s", helmDeployPipelineRunName, err)
	}

	logger.Info("Waiting for service to get external IP")
	var serviceIp string
	if err := WaitForServiceExternalIPState(c, namespace, helmDeployServiceName, func(svc *corev1.Service) (bool, error) {
		ingress := svc.Status.LoadBalancer.Ingress
		if ingress != nil {
			if len(ingress) > 0 {
				serviceIp = ingress[0].IP
				return true, nil
			}
		}
		return false, nil
	}, "ServiceExternalIPisReady"); err != nil {
		t.Errorf("Error waiting for Service %s to get an external IP: %s", helmDeployServiceName, err)
	}

	// cleanup task to remove helm from cluster, will not fail the test if it fails, just log
	knativetest.CleanupOnInterrupt(func() { helmCleanup(c, t, namespace, logger) }, logger)
	defer helmCleanup(c, t, namespace, logger)

	if serviceIp != "" {
		logger.Info("Polling service with external IP")
		waitErr := wait.PollImmediate(100*time.Millisecond, 30*time.Second, func() (bool, error) {
			resp, err := http.Get(fmt.Sprintf("http://%s:8080", serviceIp))
			if err != nil {
				return false, nil
			}
			if resp != nil && resp.StatusCode != http.StatusOK {
				return true, fmt.Errorf("Expected 200 but received %d response code	from service at http://%s:8080", resp.StatusCode, serviceIp)
			}
			return true, nil
		})
		if waitErr != nil {
			t.Errorf("Error from pinging service IP %s : %s", serviceIp, waitErr)
		}

	} else {
		t.Errorf("Service IP is empty.")
	}
}

func getGoHelloworldGitResource(namespace string) *v1alpha1.PipelineResource {
	return &v1alpha1.PipelineResource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      sourceResourceName,
			Namespace: namespace,
		},
		Spec: v1alpha1.PipelineResourceSpec{
			Type: v1alpha1.PipelineResourceTypeGit,
			Params: []v1alpha1.Param{
				v1alpha1.Param{
					Name:  "Url",
					Value: "https://github.com/knative/build-pipeline",
				},
			},
		},
	}
}

func getCreateImageTask(namespace string, t *testing.T) *v1alpha1.Task {
	// according to knative/test-infra readme (https://github.com/knative/test-infra/blob/13055d769cc5e1756e605fcb3bcc1c25376699f1/scripts/README.md)
	// the KO_DOCKER_REPO will be set with according to the porject where the cluster is created
	// it is used here to dunamically get the docker registery to push the image to
	dockerRepo := os.Getenv("KO_DOCKER_REPO")
	if dockerRepo == "" {
		t.Fatalf("KO_DOCKER_REPO env variable is required")
	}

	imageName = fmt.Sprintf("%s/%s", dockerRepo, AppendRandomString(sourceImageName))
	t.Logf("Image to be pusblished: %s", imageName)

	return &v1alpha1.Task{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      createImageTaskName,
		},
		Spec: v1alpha1.TaskSpec{
			Inputs: &v1alpha1.Inputs{
				Resources: []v1alpha1.TaskResource{
					v1alpha1.TaskResource{
						Name: "workspace",
						Type: v1alpha1.PipelineResourceTypeGit,
					},
				},
			},
			BuildSpec: &buildv1alpha1.BuildSpec{
				Steps: []corev1.Container{{
					Name:    "kaniko",
					Image:   "gcr.io/kaniko-project/executor",
					Args: []string{"--dockerfile=/workspace/test/gohelloworld/Dockerfile",
						fmt.Sprintf("--destination=%s", imageName),
					},
				}},
			},
		},
	}
}

func getHelmDeployTask(namespace string) *v1alpha1.Task {
	return &v1alpha1.Task{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      helmDeployTaskName,
		},
		Spec: v1alpha1.TaskSpec{
			Inputs: &v1alpha1.Inputs{
				Resources: []v1alpha1.TaskResource{
					v1alpha1.TaskResource{
						Name: "workspace",
						Type: v1alpha1.PipelineResourceTypeGit,
					},
				},
				Params: []v1alpha1.TaskParam{{
					Name:        "pathToHelmCharts",
					Description: "Path to the helm charts",
				}, {
					Name: "image",
				}, {
					Name: "chartname",
				}},
			},
			BuildSpec: &buildv1alpha1.BuildSpec{
				Steps: []corev1.Container{{
					Name:    "helm-init",
					Image:   "alpine/helm",
					Args:    []string{"init", "--wait"},
				}, {
					Name:    "helm-deploy",
					Image:   "alpine/helm",
					Args: []string{"install",
						"--debug",
						"--name=${inputs.params.chartname}",
						"${inputs.params.pathToHelmCharts}",
						"--set",
						"image.repository=${inputs.params.image}",
					},
				}},
			},
		},
	}
}

func getHelmDeployPipeline(namespace string) *v1alpha1.Pipeline {
	return &v1alpha1.Pipeline{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      helmDeployPipelineName,
		},
		Spec: v1alpha1.PipelineSpec{
			Tasks: []v1alpha1.PipelineTask{
				v1alpha1.PipelineTask{
					Name: "push-image",
					TaskRef: v1alpha1.TaskRef{
						Name: createImageTaskName,
					},
					InputSourceBindings: []v1alpha1.SourceBinding{{
						Name: "workspace",
						ResourceRef: v1alpha1.PipelineResourceRef{
							Name: sourceResourceName,
						},
					}},
				},
				v1alpha1.PipelineTask{
					Name: "helm-deploy",
					TaskRef: v1alpha1.TaskRef{
						Name: helmDeployTaskName,
					},
					InputSourceBindings: []v1alpha1.SourceBinding{{
						Name: "workspace",
						ResourceRef: v1alpha1.PipelineResourceRef{
							Name: sourceResourceName,
						},
						PassedConstraints: []string{createImageTaskName},
					}},
					Params: []v1alpha1.Param{{
						Name:  "pathToHelmCharts",
						Value: "/workspace/test/gohelloworld/gohelloworld-chart",
					}, {
						Name:  "chartname",
						Value: "gohelloworld",
					}, {
						Name:  "image",
						Value: imageName,
					}},
				},
			},
		},
	}
}

func getHelmDeployPipelineRun(namespace string) *v1alpha1.PipelineRun {
	return &v1alpha1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      helmDeployPipelineRunName,
		},
		Spec: v1alpha1.PipelineRunSpec{
			PipelineRef: v1alpha1.PipelineRef{
				Name: helmDeployPipelineName,
			},
			PipelineTriggerRef: v1alpha1.PipelineTriggerRef{
				Type: v1alpha1.PipelineTriggerTypeManual,
			},
		},
	}
}

func setupClusterBindingForHelm(c *clients, t *testing.T, namespace string, logger *logging.BaseLogger) {
	tillerServiceAccount = &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tiller",
			Namespace: "kube-system",
		},
	}

	logger.Infof("Creating tiller service account")
	if _, err := c.KubeClient.Kube.CoreV1().ServiceAccounts("kube-system").Create(tillerServiceAccount); err != nil {
		if !errors.IsAlreadyExists(err) {
			t.Fatalf("Failed to create default Service account for Helm %s", err)
		}
	}

	clusterRoleBindings[0] = &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: AppendRandomString("tiller"),
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     "cluster-admin",
		},
		Subjects: []rbacv1.Subject{{
			Kind:      "ServiceAccount",
			Name:      "tiller",
			Namespace: "kube-system",
		}},
	}

	clusterRoleBindings[1] = &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: AppendRandomString("default-tiller"),
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     "cluster-admin",
		},
		Subjects: []rbacv1.Subject{{
			Kind:      "ServiceAccount",
			Name:      "default",
			Namespace: namespace,
		}},
	}

	clusterRoleBindings[2] = &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: AppendRandomString("default-tiller"),
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     "cluster-admin",
		},
		Subjects: []rbacv1.Subject{{
			Kind:      "ServiceAccount",
			Name:      "default",
			Namespace: "kube-system",
		}},
	}

	for _, crb := range clusterRoleBindings {
		logger.Infof("Creating Cluster Role binding %s for helm", crb.Name)
		if _, err := c.KubeClient.Kube.RbacV1beta1().ClusterRoleBindings().Create(crb); err != nil {
			t.Fatalf("Failed to create cluster role binding for Helm %s", err)
		}
	}
}

func helmCleanup(c *clients, t *testing.T, namespace string, logger *logging.BaseLogger) {
	logger.Infof("Cleaning up helm from cluster...")

	removeAllHelmReleases(c, t, namespace, logger)
	removeHelmFromCluster(c, t, namespace, logger)

	logger.Infof("Deleting tiller service account")
	if err := c.KubeClient.Kube.CoreV1().ServiceAccounts("kube-system").Delete("tiller", &metav1.DeleteOptions{}); err != nil {
		t.Fatalf("Failed to delete default Service account for Helm %s", err)
	}

	for _, crb := range clusterRoleBindings {
		logger.Infof("Deleting Cluster Role binding %s for helm", crb.Name)
		if err := c.KubeClient.Kube.RbacV1beta1().ClusterRoleBindings().Delete(crb.Name, &metav1.DeleteOptions{}); err != nil {
			t.Fatalf("Failed to delete cluster role binding for Helm %s", err)
		}
	}
}

func removeAllHelmReleases(c *clients, t *testing.T, namespace string, logger *logging.BaseLogger) {
	helmRemoveAllTaskName := "helm-remove-all-task"
	helmRemoveAllTask := &v1alpha1.Task{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      helmRemoveAllTaskName,
		},
		Spec: v1alpha1.TaskSpec{
			BuildSpec: &buildv1alpha1.BuildSpec{
				Steps: []corev1.Container{{
					Name:  "helm-remove-all",
					Image: "alpine/helm",
					Command: []string{
						"/bin/sh",
					},
					Args: []string{
						"-c",
						"helm ls --short --all | xargs -n1 helm del --purge",
					},
				},
				},
			},
		},
	}

	helmRemoveAllTaskRunName := "helm-remove-all-taskrun"
	helmRemoveAllTaskRun := &v1alpha1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      helmRemoveAllTaskRunName,
		},
		Spec: v1alpha1.TaskRunSpec{
			TaskRef: v1alpha1.TaskRef{
				Name: helmRemoveAllTaskName,
			},
			Trigger: v1alpha1.TaskTrigger{
				TriggerRef: v1alpha1.TaskTriggerRef{
					Type: v1alpha1.TaskTriggerTypeManual,
				},
			},
		},
	}

	logger.Infof("Creating Task %s", helmRemoveAllTaskName)
	if _, err := c.TaskClient.Create(helmRemoveAllTask); err != nil {
		t.Fatalf("Failed to create Task `%s`: %s", helmRemoveAllTaskName, err)
	}

	logger.Infof("Creating TaskRun %s", helmRemoveAllTaskRunName)
	if _, err := c.TaskRunClient.Create(helmRemoveAllTaskRun); err != nil {
		t.Fatalf("Failed to create TaskRun `%s`: %s", helmRemoveAllTaskRunName, err)
	}

	logger.Infof("Waiting for TaskRun %s in namespace %s to complete", helmRemoveAllTaskRunName, namespace)
	if err := WaitForTaskRunState(c, helmRemoveAllTaskRunName, func(tr *v1alpha1.TaskRun) (bool, error) {
		c := tr.Status.GetCondition(duckv1alpha1.ConditionSucceeded)
		if c != nil {
			if c.Status == corev1.ConditionTrue {
				return true, nil
			} else if c.Status == corev1.ConditionFalse {
				return true, fmt.Errorf("task run %s failed!", hwPipelineRunName)
			}
		}
		return false, nil
	}, "TaskRunSuccess"); err != nil {
		logger.Infof("TaskRun %s failed to finish: %s", helmRemoveAllTaskRunName, err)
	}
}

func removeHelmFromCluster(c *clients, t *testing.T, namespace string, logger *logging.BaseLogger) {
	helmResetTaskName := "helm-reset-task"
	helmResetTask := &v1alpha1.Task{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      helmResetTaskName,
		},
		Spec: v1alpha1.TaskSpec{
			BuildSpec: &buildv1alpha1.BuildSpec{
				Steps: []corev1.Container{{
					Name:    "helm-reset",
					Image:   "alpine/helm",
					Args:    []string{"reset", "--force"},
				},
				},
			},
		},
	}

	helmResetTaskRunName := "helm-reset-taskrun"
	helmResetTaskRun := &v1alpha1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      helmResetTaskRunName,
		},
		Spec: v1alpha1.TaskRunSpec{
			TaskRef: v1alpha1.TaskRef{
				Name: helmResetTaskName,
			},
			Trigger: v1alpha1.TaskTrigger{
				TriggerRef: v1alpha1.TaskTriggerRef{
					Type: v1alpha1.TaskTriggerTypeManual,
				},
			},
		},
	}

	logger.Infof("Creating Task %s", helmResetTaskName)
	if _, err := c.TaskClient.Create(helmResetTask); err != nil {
		t.Fatalf("Failed to create Task `%s`: %s", helmResetTaskName, err)
	}

	logger.Infof("Creating TaskRun %s", helmResetTaskRunName)
	if _, err := c.TaskRunClient.Create(helmResetTaskRun); err != nil {
		t.Fatalf("Failed to create TaskRun `%s`: %s", helmResetTaskRunName, err)
	}

	logger.Infof("Waiting for TaskRun %s in namespace %s to complete", helmResetTaskRunName, namespace)
	if err := WaitForTaskRunState(c, helmResetTaskRunName, func(tr *v1alpha1.TaskRun) (bool, error) {
		c := tr.Status.GetCondition(duckv1alpha1.ConditionSucceeded)
		if c != nil {
			if c.Status == corev1.ConditionTrue {
				return true, nil
			} else if c.Status == corev1.ConditionFalse {
				return true, fmt.Errorf("task run run %s failed!", hwPipelineRunName)
			}
		}
		return false, nil
	}, "TaskRunSuccess"); err != nil {
		logger.Infof("TaskRun %s failed to finish: %s", helmResetTaskRunName, err)
	}
}
