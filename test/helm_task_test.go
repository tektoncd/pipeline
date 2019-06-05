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
	"testing"
	"time"

	knativetest "github.com/knative/pkg/test"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/names"
	tb "github.com/tektoncd/pipeline/test/builder"
	"golang.org/x/xerrors"
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
	clusterRoleBindings  [3]*rbacv1.ClusterRoleBinding
	tillerServiceAccount *corev1.ServiceAccount
)

// TestHelmDeployPipelineRun is an integration test that will verify a pipeline build an image
// and then using helm to deploy it
func TestHelmDeployPipelineRun(t *testing.T) {
	repo := ensureDockerRepo(t)
	c, namespace := setup(t)
	setupClusterBindingForHelm(c, t, namespace)
	t.Parallel()

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
	knativetest.CleanupOnInterrupt(func() { helmCleanup(c, t, namespace) }, t.Logf)
	defer helmCleanup(c, t, namespace)

	if serviceIp != "" {
		t.Log("Polling service with external IP")
		waitErr := wait.PollImmediate(100*time.Millisecond, 30*time.Second, func() (bool, error) {
			resp, err := http.Get(fmt.Sprintf("http://%s:8080", serviceIp))
			if err != nil {
				return false, nil
			}
			if resp != nil && resp.StatusCode != http.StatusOK {
				return true, xerrors.Errorf("Expected 200 but received %d response code	from service at http://%s:8080", resp.StatusCode, serviceIp)
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

func getCreateImageTask(namespace string) *v1alpha1.Task {
	return tb.Task(createImageTaskName, namespace, tb.TaskSpec(
		tb.TaskInputs(tb.InputsResource("gitsource", v1alpha1.PipelineResourceTypeGit)),
		tb.TaskOutputs(tb.OutputsResource("builtimage", v1alpha1.PipelineResourceTypeImage)),
		tb.Step("kaniko", "gcr.io/kaniko-project/executor:v0.9.0", tb.Args(
			"--dockerfile=/workspace/gitsource/test/gohelloworld/Dockerfile",
			"--context=/workspace/gitsource/",
			"--destination=${outputs.resources.builtimage.url}",
		)),
	))
}

func getHelmDeployTask(namespace string) *v1alpha1.Task {
	return tb.Task(helmDeployTaskName, namespace, tb.TaskSpec(
		tb.TaskInputs(
			tb.InputsResource("gitsource", v1alpha1.PipelineResourceTypeGit),
			tb.InputsResource("image", v1alpha1.PipelineResourceTypeImage),
			tb.InputsParam("pathToHelmCharts", tb.ParamDescription("Path to the helm charts")),
			tb.InputsParam("chartname", tb.ParamDefault("")),
		),
		tb.Step("helm-init", "alpine/helm:2.14.0", tb.Args("init", "--wait")),
		tb.Step("helm-deploy", "alpine/helm:2.14.0", tb.Args(
			"install",
			"--debug",
			"--name=${inputs.params.chartname}",
			"${inputs.params.pathToHelmCharts}",
			"--set",
			"image.repository=${inputs.resources.image.url}",
		)),
	))
}

func getHelmDeployPipeline(namespace string) *v1alpha1.Pipeline {
	return tb.Pipeline(helmDeployPipelineName, namespace, tb.PipelineSpec(
		tb.PipelineDeclaredResource("git-repo", "git"),
		tb.PipelineDeclaredResource("the-image", "image"),
		tb.PipelineParam("chartname"),
		tb.PipelineTask("push-image", createImageTaskName,
			tb.PipelineTaskInputResource("gitsource", "git-repo"),
			tb.PipelineTaskOutputResource("builtimage", "the-image"),
		),
		tb.PipelineTask("helm-deploy", helmDeployTaskName,
			tb.PipelineTaskInputResource("gitsource", "git-repo"),
			tb.PipelineTaskInputResource("image", "the-image", tb.From("push-image")),
			tb.PipelineTaskParam("pathToHelmCharts", "/workspace/gitsource/test/gohelloworld/gohelloworld-chart"),
			tb.PipelineTaskParam("chartname", "${params.chartname}"),
		),
	))
}

func getHelmDeployPipelineRun(namespace string) *v1alpha1.PipelineRun {
	return tb.PipelineRun(helmDeployPipelineRunName, namespace, tb.PipelineRunSpec(
		helmDeployPipelineName,
		tb.PipelineRunParam("chartname", "gohelloworld"),
		tb.PipelineRunResourceBinding("git-repo", tb.PipelineResourceBindingRef(sourceResourceName)),
		tb.PipelineRunResourceBinding("the-image", tb.PipelineResourceBindingRef(sourceImageName)),
	))
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
	helmRemoveAllTask := tb.Task(helmRemoveAllTaskName, namespace, tb.TaskSpec(
		tb.Step("helm-remove-all", "alpine/helm:2.14.0", tb.Command("/bin/sh"),
			tb.Args("-c", "helm ls --short --all | xargs -n1 helm del --purge"),
		),
	))

	helmRemoveAllTaskRunName := "helm-remove-all-taskrun"
	helmRemoveAllTaskRun := tb.TaskRun(helmRemoveAllTaskRunName, namespace, tb.TaskRunSpec(
		tb.TaskRunTaskRef(helmRemoveAllTaskName),
	))

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
	helmResetTask := tb.Task(helmResetTaskName, namespace, tb.TaskSpec(
		tb.Step("helm-reset", "alpine/helm:2.14.0", tb.Args("reset", "--force")),
	))

	helmResetTaskRunName := "helm-reset-taskrun"
	helmResetTaskRun := tb.TaskRun(helmResetTaskRunName, namespace, tb.TaskRunSpec(
		tb.TaskRunTaskRef(helmResetTaskName),
	))

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
