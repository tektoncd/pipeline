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
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	resources "github.com/tektoncd/pipeline/pkg/apis/resource/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/names"
	tb "github.com/tektoncd/pipeline/test/builder"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	knativetest "knative.dev/pkg/test"
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
	clusterRoleBindings  [3]*rbacv1.ClusterRoleBinding
	tillerServiceAccount *corev1.ServiceAccount
)

// TestHelmDeployPipelineRun is an integration test that will verify a pipeline build an image
// and then using helm to deploy it
func TestHelmDeployPipelineRun(t *testing.T) {
	repo := ensureDockerRepo(t)
	c, namespace := setup(t)
	setupClusterBindingForHelm(c, t, namespace)

	knativetest.CleanupOnInterrupt(func() { tearDown(t, c, namespace) }, t.Logf)
	defer tearDown(t, c, namespace)

	t.Logf("Creating Git PipelineResource %s", sourceResourceName)
	if _, err := c.PipelineResourceClient.Create(getGoHelloworldGitResource(namespace)); err != nil {
		t.Fatalf("Failed to create Pipeline Resource `%s`: %s", sourceResourceName, err)
	}

	t.Logf("Creating Image PipelineResource %s", sourceImageName)
	if _, err := c.PipelineResourceClient.Create(getHelmImageResource(namespace, repo)); err != nil {
		t.Fatalf("Failed to create Pipeline Resource `%s`: %s", sourceImageName, err)
	}

	t.Logf("Creating Task %s", createImageTaskName)
	if _, err := c.TaskClient.Create(getCreateImageTask(namespace)); err != nil {
		t.Fatalf("Failed to create Task `%s`: %s", createImageTaskName, err)
	}

	t.Logf("Creating Task %s", helmDeployTaskName)
	if _, err := c.TaskClient.Create(getHelmDeployTask(namespace)); err != nil {
		t.Fatalf("Failed to create Task `%s`: %s", helmDeployTaskName, err)
	}

	t.Logf("Creating Pipeline %s", helmDeployPipelineName)
	if _, err := c.PipelineClient.Create(getHelmDeployPipeline(namespace)); err != nil {
		t.Fatalf("Failed to create Pipeline `%s`: %s", helmDeployPipelineName, err)
	}

	t.Logf("Creating PipelineRun %s", helmDeployPipelineRunName)
	if _, err := c.PipelineRunClient.Create(getHelmDeployPipelineRun(namespace)); err != nil {
		t.Fatalf("Failed to create Pipeline `%s`: %s", helmDeployPipelineRunName, err)
	}

	// Verify status of PipelineRun (wait for it)
	if err := WaitForPipelineRunState(c, helmDeployPipelineRunName, timeout, PipelineRunSucceed(helmDeployPipelineRunName), "PipelineRunCompleted"); err != nil {
		t.Errorf("Error waiting for PipelineRun %s to finish: %s", helmDeployPipelineRunName, err)
		t.Fatalf("PipelineRun execution failed; helm may or may not have been installed :(")
	}

	t.Log("Waiting for service to get external IP")
	var serviceIP string
	if err := WaitForServiceExternalIPState(c, namespace, helmDeployServiceName, func(svc *corev1.Service) (bool, error) {
		ingress := svc.Status.LoadBalancer.Ingress
		if ingress != nil {
			if len(ingress) > 0 {
				serviceIP = ingress[0].IP
				return true, nil
			}
		}
		return false, nil
	}, "ServiceExternalIPisReady"); err != nil {
		t.Errorf("Error waiting for Service %s to get an external IP: %s", helmDeployServiceName, err)
	}

	// cleanup task to remove helm from cluster, will not fail the test if it fails, just log
	knativetest.CleanupOnInterrupt(func() { helmCleanup(c, t, namespace) }, t.Logf)
	defer helmCleanup(c, t, namespace)

	if serviceIP != "" {
		t.Log("Polling service with external IP")
		waitErr := wait.PollImmediate(100*time.Millisecond, 30*time.Second, func() (bool, error) {
			resp, err := http.Get(fmt.Sprintf("http://%s:8080", serviceIP))
			if err != nil {
				return false, nil
			}
			if resp != nil && resp.StatusCode != http.StatusOK {
				return true, fmt.Errorf("expected 200 but received %d response code from service at http://%s:8080", resp.StatusCode, serviceIP)
			}
			return true, nil
		})
		if waitErr != nil {
			t.Errorf("Error from pinging service IP %s : %s", serviceIP, waitErr)
		}

	} else {
		t.Errorf("Service IP is empty.")
	}
}

func getGoHelloworldGitResource(namespace string) *v1alpha1.PipelineResource {
	return tb.PipelineResource(sourceResourceName, namespace, tb.PipelineResourceSpec(
		v1alpha1.PipelineResourceTypeGit,
		tb.PipelineResourceSpecParam("url", "https://github.com/tektoncd/pipeline"),
	))
}

func getHelmImageResource(namespace, dockerRepo string) *v1alpha1.PipelineResource {
	imageName := fmt.Sprintf("%s/%s", dockerRepo, names.SimpleNameGenerator.RestrictLengthWithRandomSuffix(sourceImageName))

	return tb.PipelineResource(sourceImageName, namespace, tb.PipelineResourceSpec(
		v1alpha1.PipelineResourceTypeImage,
		tb.PipelineResourceSpecParam("url", imageName),
	))
}

func getCreateImageTask(namespace string) *v1beta1.Task {
	return &v1beta1.Task{
		ObjectMeta: metav1.ObjectMeta{Name: createImageTaskName, Namespace: namespace},
		Spec: v1beta1.TaskSpec{
			Resources: &v1beta1.TaskResources{
				Inputs: []v1beta1.TaskResource{{ResourceDeclaration: v1beta1.ResourceDeclaration{
					Name: "gitsource", Type: resources.PipelineResourceTypeGit,
				}}},
				Outputs: []v1beta1.TaskResource{{ResourceDeclaration: v1beta1.ResourceDeclaration{
					Name: "builtimage", Type: resources.PipelineResourceTypeImage,
				}}},
			},
			Steps: []v1beta1.Step{{Container: corev1.Container{
				Name:  "kaniko",
				Image: "gcr.io/kaniko-project/executor:v0.17.1",
				Args: []string{
					"--dockerfile=/workspace/gitsource/test/gohelloworld/Dockerfile",
					"--context=/workspace/gitsource/",
					"--destination=$(outputs.resources.builtimage.url)",
				},
			}}},
		},
	}
}

func getHelmDeployTask(namespace string) *v1beta1.Task {
	empty := v1beta1.NewArrayOrString("")
	return &v1beta1.Task{
		ObjectMeta: metav1.ObjectMeta{Name: helmDeployTaskName, Namespace: namespace},
		Spec: v1beta1.TaskSpec{
			Resources: &v1beta1.TaskResources{
				Inputs: []v1beta1.TaskResource{{ResourceDeclaration: v1beta1.ResourceDeclaration{
					Name: "gitsource", Type: resources.PipelineResourceTypeGit,
				}}, {ResourceDeclaration: v1beta1.ResourceDeclaration{
					Name: "image", Type: resources.PipelineResourceTypeImage,
				}}},
			},
			Params: []v1beta1.ParamSpec{{
				Name: "pathToHelmCharts", Type: v1beta1.ParamTypeString, Description: "Path to the helm charts",
			}, {
				Name: "chartname", Type: v1beta1.ParamTypeString, Default: &empty,
			}},
			Steps: []v1beta1.Step{{Container: corev1.Container{
				Image: "alpine/helm:2.14.0",
				Args:  []string{"init", "--wait"},
			}}, {Container: corev1.Container{
				Image: "alpine/helm:2.14.0",
				Args: []string{"install",
					"--debug",
					"--name=$(inputs.params.chartname)",
					"$(inputs.params.pathToHelmCharts)",
					"--set",
					"image.repository=$(inputs.resources.image.url)",
				},
			}}},
		},
	}
}

func getHelmDeployPipeline(namespace string) *v1beta1.Pipeline {
	return &v1beta1.Pipeline{
		ObjectMeta: metav1.ObjectMeta{Name: helmDeployPipelineName, Namespace: namespace},
		Spec: v1beta1.PipelineSpec{
			Params: []v1beta1.ParamSpec{{
				Name: "chartname", Type: v1beta1.ParamTypeString,
			}},
			Resources: []v1beta1.PipelineDeclaredResource{{
				Name: "git-repo", Type: "git",
			}, {
				Name: "the-image", Type: "image",
			}},
			Tasks: []v1beta1.PipelineTask{{
				Name:    "push-image",
				TaskRef: &v1beta1.TaskRef{Name: createImageTaskName},
				Resources: &v1beta1.PipelineTaskResources{
					Inputs: []v1beta1.PipelineTaskInputResource{{
						Name: "gitsource", Resource: "git-repo",
					}},
					Outputs: []v1beta1.PipelineTaskOutputResource{{
						Name: "builtimage", Resource: "the-image",
					}},
				},
			}, {
				Name:    "helm-deploy",
				TaskRef: &v1beta1.TaskRef{Name: helmDeployTaskName},
				Resources: &v1beta1.PipelineTaskResources{
					Inputs: []v1beta1.PipelineTaskInputResource{{
						Name: "gitsource", Resource: "git-repo",
					}, {
						Name: "image", Resource: "the-image", From: []string{"push-image"},
					}},
				},
				Params: []v1beta1.Param{{
					Name: "pathToHelmCharts", Value: v1beta1.NewArrayOrString("/workspace/gitsource/test/gohelloworld/gohelloworld-chart"),
				}, {
					Name: "chartname", Value: v1beta1.NewArrayOrString("$(params.chartname)"),
				}},
			}},
		},
	}
}

func getHelmDeployPipelineRun(namespace string) *v1beta1.PipelineRun {
	return &v1beta1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{Name: helmDeployPipelineRunName, Namespace: namespace},
		Spec: v1beta1.PipelineRunSpec{
			PipelineRef: &v1beta1.PipelineRef{Name: helmDeployPipelineName},
			Params: []v1beta1.Param{{
				Name: "chartname", Value: v1beta1.NewArrayOrString("gohelloworld"),
			}},
			Resources: []v1beta1.PipelineResourceBinding{{
				Name: "git-repo", ResourceRef: &v1beta1.PipelineResourceRef{Name: sourceResourceName},
			}, {
				Name: "the-image", ResourceRef: &v1beta1.PipelineResourceRef{Name: sourceImageName},
			}},
		},
	}
}

func setupClusterBindingForHelm(c *clients, t *testing.T, namespace string) {
	tillerServiceAccount = &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tiller",
			Namespace: "kube-system",
		},
	}

	t.Logf("Creating tiller service account")
	if _, err := c.KubeClient.Kube.CoreV1().ServiceAccounts("kube-system").Create(tillerServiceAccount); err != nil {
		if !errors.IsAlreadyExists(err) {
			t.Fatalf("Failed to create default Service account for Helm %s", err)
		}
	}

	clusterRoleBindings[0] = &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: names.SimpleNameGenerator.RestrictLengthWithRandomSuffix("tiller"),
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
			Name: names.SimpleNameGenerator.RestrictLengthWithRandomSuffix("default-tiller"),
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
			Name: names.SimpleNameGenerator.RestrictLengthWithRandomSuffix("default-tiller"),
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
		t.Logf("Creating Cluster Role binding %s for helm", crb.Name)
		if _, err := c.KubeClient.Kube.RbacV1beta1().ClusterRoleBindings().Create(crb); err != nil {
			t.Fatalf("Failed to create cluster role binding for Helm %s", err)
		}
	}
}

func helmCleanup(c *clients, t *testing.T, namespace string) {
	t.Logf("Cleaning up helm from cluster...")

	removeAllHelmReleases(c, t, namespace)
	removeHelmFromCluster(c, t, namespace)

	t.Logf("Deleting tiller service account")
	if err := c.KubeClient.Kube.CoreV1().ServiceAccounts("kube-system").Delete("tiller", &metav1.DeleteOptions{}); err != nil {
		t.Fatalf("Failed to delete default Service account for Helm %s", err)
	}

	for _, crb := range clusterRoleBindings {
		t.Logf("Deleting Cluster Role binding %s for helm", crb.Name)
		if err := c.KubeClient.Kube.RbacV1beta1().ClusterRoleBindings().Delete(crb.Name, &metav1.DeleteOptions{}); err != nil {
			t.Fatalf("Failed to delete cluster role binding for Helm %s", err)
		}
	}
}

func removeAllHelmReleases(c *clients, t *testing.T, namespace string) {
	helmRemoveAllTaskName := "helm-remove-all-task"
	helmRemoveAllTask := &v1beta1.Task{
		ObjectMeta: metav1.ObjectMeta{Name: helmRemoveAllTaskName, Namespace: namespace},
		Spec: v1beta1.TaskSpec{
			Steps: []v1beta1.Step{{Container: corev1.Container{
				Name:    "helm-remove-all",
				Image:   "alpine/helm:2.14.0",
				Command: []string{"/bin/sh"},
				Args:    []string{"-c", "helm ls --short --all | xargs -n1 helm del --purge"},
			}}},
		},
	}

	helmRemoveAllTaskRunName := "helm-remove-all-taskrun"
	helmRemoveAllTaskRun := &v1beta1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{Name: helmRemoveAllTaskRunName, Namespace: namespace},
		Spec: v1beta1.TaskRunSpec{
			TaskRef: &v1beta1.TaskRef{Name: helmRemoveAllTaskName},
		},
	}

	t.Logf("Creating Task %s", helmRemoveAllTaskName)
	if _, err := c.TaskClient.Create(helmRemoveAllTask); err != nil {
		t.Fatalf("Failed to create Task `%s`: %s", helmRemoveAllTaskName, err)
	}

	t.Logf("Creating TaskRun %s", helmRemoveAllTaskRunName)
	if _, err := c.TaskRunClient.Create(helmRemoveAllTaskRun); err != nil {
		t.Fatalf("Failed to create TaskRun `%s`: %s", helmRemoveAllTaskRunName, err)
	}

	t.Logf("Waiting for TaskRun %s in namespace %s to complete", helmRemoveAllTaskRunName, namespace)
	if err := WaitForTaskRunState(c, helmRemoveAllTaskRunName, TaskRunSucceed(helmRemoveAllTaskRunName), "TaskRunSuccess"); err != nil {
		t.Logf("TaskRun %s failed to finish: %s", helmRemoveAllTaskRunName, err)
	}
}

func removeHelmFromCluster(c *clients, t *testing.T, namespace string) {
	helmResetTaskName := "helm-reset-task"
	helmResetTask := &v1beta1.Task{
		ObjectMeta: metav1.ObjectMeta{Name: helmResetTaskName, Namespace: namespace},
		Spec: v1beta1.TaskSpec{
			Steps: []v1beta1.Step{{Container: corev1.Container{
				Image: "alpine/helm:2.14.0",
				Args:  []string{"reset", "--force"},
			}}},
		},
	}

	helmResetTaskRunName := "helm-reset-taskrun"
	helmResetTaskRun := &v1beta1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{Name: helmResetTaskRunName, Namespace: namespace},
		Spec: v1beta1.TaskRunSpec{
			TaskRef: &v1beta1.TaskRef{Name: helmResetTaskName},
		},
	}

	t.Logf("Creating Task %s", helmResetTaskName)
	if _, err := c.TaskClient.Create(helmResetTask); err != nil {
		t.Fatalf("Failed to create Task `%s`: %s", helmResetTaskName, err)
	}

	t.Logf("Creating TaskRun %s", helmResetTaskRunName)
	if _, err := c.TaskRunClient.Create(helmResetTaskRun); err != nil {
		t.Fatalf("Failed to create TaskRun `%s`: %s", helmResetTaskRunName, err)
	}

	t.Logf("Waiting for TaskRun %s in namespace %s to complete", helmResetTaskRunName, namespace)
	if err := WaitForTaskRunState(c, helmResetTaskRunName, TaskRunSucceed(helmResetTaskRunName), "TaskRunSuccess"); err != nil {
		t.Logf("TaskRun %s failed to finish: %s", helmResetTaskRunName, err)
	}
}
