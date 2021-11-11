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
	"testing"

	"github.com/tektoncd/pipeline/test/parse"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/pkg/names"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	knativetest "knative.dev/pkg/test"
	"knative.dev/pkg/test/helpers"
)

var (
	clusterRoleBindings [1]*rbacv1.ClusterRoleBinding
)

// TestHelmDeployPipelineRun is an integration test that will verify a pipeline build an image
// and then using helm to deploy it
func TestHelmDeployPipelineRun(t *testing.T) {
	repo := ensureDockerRepo(t)
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	c, namespace := setup(ctx, t)
	setupClusterBindingForHelm(ctx, c, t, namespace)

	var (
		sourceResourceName        = helpers.ObjectNameForTest(t)
		sourceImageName           = helpers.ObjectNameForTest(t)
		createImageTaskName       = helpers.ObjectNameForTest(t)
		helmDeployTaskName        = helpers.ObjectNameForTest(t)
		checkServiceTaskName      = helpers.ObjectNameForTest(t)
		helmDeployPipelineName    = helpers.ObjectNameForTest(t)
		helmDeployPipelineRunName = helpers.ObjectNameForTest(t)
	)

	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	t.Logf("Creating Git PipelineResource %s", sourceResourceName)
	if _, err := c.PipelineResourceClient.Create(ctx, getGoHelloworldGitResource(t, sourceResourceName), metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create Pipeline Resource `%s`: %s", sourceResourceName, err)
	}

	t.Logf("Creating Image PipelineResource %s", sourceImageName)
	if _, err := c.PipelineResourceClient.Create(ctx, getHelmImageResource(t, repo, sourceImageName), metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create Pipeline Resource `%s`: %s", sourceImageName, err)
	}

	t.Logf("Creating Task %s", createImageTaskName)
	if _, err := c.TaskClient.Create(ctx, getCreateImageTask(t, namespace, createImageTaskName), metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create Task `%s`: %s", createImageTaskName, err)
	}

	t.Logf("Creating Task %s", helmDeployTaskName)
	if _, err := c.TaskClient.Create(ctx, getHelmDeployTask(t, namespace, helmDeployTaskName), metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create Task `%s`: %s", helmDeployTaskName, err)
	}

	t.Logf("Creating Task %s", checkServiceTaskName)
	if _, err := c.TaskClient.Create(ctx, getCheckServiceTask(t, namespace, checkServiceTaskName), metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create Task `%s`: %s", checkServiceTaskName, err)
	}

	t.Logf("Creating Pipeline %s", helmDeployPipelineName)
	if _, err := c.PipelineClient.Create(ctx, getHelmDeployPipeline(t, namespace, createImageTaskName, helmDeployTaskName, checkServiceTaskName, helmDeployPipelineName), metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create Pipeline `%s`: %s", helmDeployPipelineName, err)
	}

	t.Logf("Creating PipelineRun %s", helmDeployPipelineRunName)
	if _, err := c.PipelineRunClient.Create(ctx, getHelmDeployPipelineRun(t, namespace, sourceResourceName, sourceImageName, helmDeployPipelineRunName, helmDeployPipelineName), metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create Pipeline `%s`: %s", helmDeployPipelineRunName, err)
	}

	// Verify status of PipelineRun (wait for it)
	if err := WaitForPipelineRunState(ctx, c, helmDeployPipelineRunName, timeout, PipelineRunSucceed(helmDeployPipelineRunName), "PipelineRunCompleted"); err != nil {
		t.Errorf("Error waiting for PipelineRun %s to finish: %s", helmDeployPipelineRunName, err)
		t.Fatalf("PipelineRun execution failed; helm may or may not have been installed :(")
	}

	// cleanup task to remove helm releases from cluster and cluster role bindings, will not fail the test if it fails, just log
	knativetest.CleanupOnInterrupt(func() { helmCleanup(ctx, c, t, namespace) }, t.Logf)
	defer helmCleanup(ctx, c, t, namespace)
}

func getGoHelloworldGitResource(t *testing.T, sourceResourceName string) *v1alpha1.PipelineResource {
	return parse.MustParsePipelineResource(t, fmt.Sprintf(`
metadata:
  name: %s
spec:
  type: git
  params:
  - name: url
    value: https://github.com/tektoncd/pipeline
`, sourceResourceName))
}

func getHelmImageResource(t *testing.T, dockerRepo, sourceImageName string) *v1alpha1.PipelineResource {
	imageName := fmt.Sprintf("%s/%s", dockerRepo, names.SimpleNameGenerator.RestrictLengthWithRandomSuffix(sourceImageName))

	return parse.MustParsePipelineResource(t, fmt.Sprintf(`
metadata:
  name: %s
spec:
  type: image
  params:
  - name: url
    value: %s
`, sourceImageName, imageName))
}

func getCreateImageTask(t *testing.T, namespace, createImageTaskName string) *v1beta1.Task {
	return parse.MustParseTask(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  resources:
    inputs:
    - name: gitsource
      type: git
    outputs:
    - name: builtimage
      type: image
  steps:
  - name: kaniko
    image: %s
    args: ['--dockerfile=/workspace/gitsource/test/gohelloworld/Dockerfile',
	  	   '--context=/workspace/gitsource/',
	  	   '--destination=$(outputs.resources.builtimage.url)']
`, createImageTaskName, namespace, getTestImage(kanikoImage)))
}

func getHelmDeployTask(t *testing.T, namespace, helmDeployTaskName string) *v1beta1.Task {
	return parse.MustParseTask(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  resources:
    inputs:
    - name: gitsource
      type: git
    - name: image
      type: image
  params:
  - name: pathToHelmCharts
    type: string
    description: 'Path to the helm charts'
  - name: chartname
    type: string
    default: ""
  steps:
  - image: alpine/helm:3.5.4
    args: ['upgrade',
		   '--wait',
		   '--debug',
		   '--install',
		   '--namespace',
		   '%s',
		   '$(inputs.params.chartname)',
		   '$(inputs.params.pathToHelmCharts)',
		   '--set',
		   'image.repository=$(inputs.resources.image.url)',
		   '--set',
		   'service.type=ClusterIP']
  - image: lachlanevenson/k8s-kubectl
    command: ['kubectl']
    args: ['get',
		   'all',
		   '--namespace',
		   '%s']
`, helmDeployTaskName, namespace, namespace, namespace))
}

func getCheckServiceTask(t *testing.T, namespace, checkServiceTaskName string) *v1beta1.Task {
	return parse.MustParseTask(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  params:
  - name: serviceUrl
    type: string
    description: 'Service url'
  steps:
  - image: %s
    args: ['-wait', '$(inputs.params.serviceUrl)', '-timeout', '1m']
`, checkServiceTaskName, namespace, getTestImage(dockerizeImage)))
}

func getHelmDeployPipeline(t *testing.T, namespace, createImageTaskName, helmDeployTaskName, checkServiceTaskName, helmDeployPipelineName string) *v1beta1.Pipeline {
	return parse.MustParsePipeline(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  params:
  - name: chartname
    type: string
  resources:
  - name: git-repo
    type: git
  - name: the-image
    type: image
  tasks:
  - name: push-image
    taskRef:
      name: %s
    resources:
      inputs:
      - name: gitsource
        resource: git-repo
      outputs:
      - name: builtimage
        resource: the-image
  - name: helm-deploy
    taskRef:
      name: %s
    resources:
      inputs:
      - name: gitsource
        resource: git-repo
      - name: image
        resource: the-image
    params:
    - name: pathToHelmCharts 
      value: /workspace/gitsource/test/gohelloworld/gohelloworld-chart
    - name: chartname
      value: '$(params.chartname)'
  - name: check-service
    taskRef:
      name: %s
    params:
    - name: serviceUrl
      value: http://gohelloworld-chart:8080
    runAfter: ['helm-deploy']
`, helmDeployPipelineName, namespace, createImageTaskName, helmDeployTaskName, checkServiceTaskName))
}

func getHelmDeployPipelineRun(t *testing.T, namespace, sourceResourceName, sourceImageName, helmDeployPipelineRunName, helmDeployPipelineName string) *v1beta1.PipelineRun {
	return parse.MustParsePipelineRun(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  pipelineRef:
    name: %s
  params:
  - name: chartname
    value: gohelloworld
  resources:
  - name: git-repo
    resourceRef:
      name: %s
  - name: the-image
    resourceRef:
      name: %s
`, helmDeployPipelineRunName, namespace, helmDeployPipelineName, sourceResourceName, sourceImageName))
}

func setupClusterBindingForHelm(ctx context.Context, c *clients, t *testing.T, namespace string) {
	clusterRoleBindings[0] = &rbacv1.ClusterRoleBinding{
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

	for _, crb := range clusterRoleBindings {
		t.Logf("Creating Cluster Role binding %s for helm", crb.Name)
		if _, err := c.KubeClient.RbacV1().ClusterRoleBindings().Create(ctx, crb, metav1.CreateOptions{}); err != nil {
			t.Fatalf("Failed to create cluster role binding for Helm %s", err)
		}
	}
}

func helmCleanup(ctx context.Context, c *clients, t *testing.T, namespace string) {
	t.Logf("Cleaning up helm from cluster...")

	removeAllHelmReleases(ctx, c, t, namespace)

	for _, crb := range clusterRoleBindings {
		t.Logf("Deleting Cluster Role binding %s for helm", crb.Name)
		if err := c.KubeClient.RbacV1().ClusterRoleBindings().Delete(ctx, crb.Name, metav1.DeleteOptions{}); err != nil {
			t.Fatalf("Failed to delete cluster role binding for Helm %s", err)
		}
	}
}

func removeAllHelmReleases(ctx context.Context, c *clients, t *testing.T, namespace string) {
	helmRemoveAllTaskName := "helm-remove-all-task"
	helmRemoveAllTask := parse.MustParseTask(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  steps:
  - name: helm-remove-all
    image: alpine/helm:3.5.4
    command: ['/bin/sh']
    args: ['-c', 'helm ls --short --all --namespace %s | xargs -n1 helm delete --namespace %s']
`, helmRemoveAllTaskName, namespace, namespace, namespace))

	helmRemoveAllTaskRunName := "helm-remove-all-taskrun"
	helmRemoveAllTaskRun := parse.MustParseTaskRun(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  taskRef:
    name: %s
`, helmRemoveAllTaskRunName, namespace, helmRemoveAllTaskName))

	t.Logf("Creating Task %s", helmRemoveAllTaskName)
	if _, err := c.TaskClient.Create(ctx, helmRemoveAllTask, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create Task `%s`: %s", helmRemoveAllTaskName, err)
	}

	t.Logf("Creating TaskRun %s", helmRemoveAllTaskRunName)
	if _, err := c.TaskRunClient.Create(ctx, helmRemoveAllTaskRun, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create TaskRun `%s`: %s", helmRemoveAllTaskRunName, err)
	}

	t.Logf("Waiting for TaskRun %s in namespace %s to complete", helmRemoveAllTaskRunName, namespace)
	if err := WaitForTaskRunState(ctx, c, helmRemoveAllTaskRunName, TaskRunSucceed(helmRemoveAllTaskRunName), "TaskRunSuccess"); err != nil {
		t.Logf("TaskRun %s failed to finish: %s", helmRemoveAllTaskRunName, err)
	}
}
