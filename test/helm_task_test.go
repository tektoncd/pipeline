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

	tb "github.com/tektoncd/pipeline/internal/builder/v1beta1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	resources "github.com/tektoncd/pipeline/pkg/apis/resource/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/names"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1beta1"
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
	if _, err := c.PipelineResourceClient.Create(ctx, getGoHelloworldGitResource(sourceResourceName), metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create Pipeline Resource `%s`: %s", sourceResourceName, err)
	}

	t.Logf("Creating Image PipelineResource %s", sourceImageName)
	if _, err := c.PipelineResourceClient.Create(ctx, getHelmImageResource(repo, sourceImageName), metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create Pipeline Resource `%s`: %s", sourceImageName, err)
	}

	t.Logf("Creating Task %s", createImageTaskName)
	if _, err := c.TaskClient.Create(ctx, getCreateImageTask(namespace, createImageTaskName), metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create Task `%s`: %s", createImageTaskName, err)
	}

	t.Logf("Creating Task %s", helmDeployTaskName)
	if _, err := c.TaskClient.Create(ctx, getHelmDeployTask(namespace, helmDeployTaskName), metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create Task `%s`: %s", helmDeployTaskName, err)
	}

	t.Logf("Creating Task %s", checkServiceTaskName)
	if _, err := c.TaskClient.Create(ctx, getCheckServiceTask(namespace, checkServiceTaskName), metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create Task `%s`: %s", checkServiceTaskName, err)
	}

	t.Logf("Creating Pipeline %s", helmDeployPipelineName)
	if _, err := c.PipelineClient.Create(ctx, getHelmDeployPipeline(namespace, createImageTaskName, helmDeployTaskName, checkServiceTaskName, helmDeployPipelineName), metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create Pipeline `%s`: %s", helmDeployPipelineName, err)
	}

	t.Logf("Creating PipelineRun %s", helmDeployPipelineRunName)
	if _, err := c.PipelineRunClient.Create(ctx, getHelmDeployPipelineRun(namespace, sourceResourceName, sourceImageName, helmDeployPipelineRunName, helmDeployPipelineName), metav1.CreateOptions{}); err != nil {
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

func getGoHelloworldGitResource(sourceResourceName string) *v1alpha1.PipelineResource {
	return tb.PipelineResource(sourceResourceName, tb.PipelineResourceSpec(
		v1alpha1.PipelineResourceTypeGit,
		tb.PipelineResourceSpecParam("url", "https://github.com/tektoncd/pipeline"),
	))
}

func getHelmImageResource(dockerRepo, sourceImageName string) *v1alpha1.PipelineResource {
	imageName := fmt.Sprintf("%s/%s", dockerRepo, names.SimpleNameGenerator.RestrictLengthWithRandomSuffix(sourceImageName))

	return tb.PipelineResource(sourceImageName, tb.PipelineResourceSpec(
		v1alpha1.PipelineResourceTypeImage,
		tb.PipelineResourceSpecParam("url", imageName),
	))
}

func getCreateImageTask(namespace, createImageTaskName string) *v1beta1.Task {
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
				Image: getTestImage(kanikoImage),
				Args: []string{
					"--dockerfile=/workspace/gitsource/test/gohelloworld/Dockerfile",
					"--context=/workspace/gitsource/",
					"--destination=$(outputs.resources.builtimage.url)",
				},
			}}},
		},
	}
}

func getHelmDeployTask(namespace, helmDeployTaskName string) *v1beta1.Task {
	empty := *v1beta1.NewArrayOrString("")
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
				Image: "alpine/helm:3.5.4",
				Args: []string{
					"upgrade",
					"--wait",
					"--debug",
					"--install",
					"--namespace",
					namespace,
					"$(inputs.params.chartname)",
					"$(inputs.params.pathToHelmCharts)",
					"--set",
					"image.repository=$(inputs.resources.image.url)",
					"--set",
					"service.type=ClusterIP",
				},
			}}, {Container: corev1.Container{
				Image:   "lachlanevenson/k8s-kubectl",
				Command: []string{"kubectl"},
				Args: []string{
					"get",
					"all",
					"--namespace",
					namespace,
				},
			}}},
		},
	}
}

func getCheckServiceTask(namespace, checkServiceTaskName string) *v1beta1.Task {
	return &v1beta1.Task{
		ObjectMeta: metav1.ObjectMeta{Name: checkServiceTaskName, Namespace: namespace},
		Spec: v1beta1.TaskSpec{
			Params: []v1beta1.ParamSpec{{
				Name: "serviceUrl", Type: v1beta1.ParamTypeString, Description: "Service url",
			}},
			Steps: []v1beta1.Step{{Container: corev1.Container{
				Image: getTestImage(dockerizeImage),
				Args: []string{
					"-wait",
					"$(inputs.params.serviceUrl)",
					"-timeout",
					"1m",
				},
			}}},
		},
	}
}

func getHelmDeployPipeline(namespace, createImageTaskName, helmDeployTaskName, checkServiceTaskName, helmDeployPipelineName string) *v1beta1.Pipeline {
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
					Name: "pathToHelmCharts", Value: *v1beta1.NewArrayOrString("/workspace/gitsource/test/gohelloworld/gohelloworld-chart"),
				}, {
					Name: "chartname", Value: *v1beta1.NewArrayOrString("$(params.chartname)"),
				}},
			}, {
				Name:    "check-service",
				TaskRef: &v1beta1.TaskRef{Name: checkServiceTaskName},
				Params: []v1beta1.Param{{
					Name: "serviceUrl", Value: *v1beta1.NewArrayOrString("http://gohelloworld-chart:8080"),
				}},
				RunAfter: []string{"helm-deploy"},
			}},
		},
	}
}

func getHelmDeployPipelineRun(namespace, sourceResourceName, sourceImageName, helmDeployPipelineRunName, helmDeployPipelineName string) *v1beta1.PipelineRun {
	return &v1beta1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{Name: helmDeployPipelineRunName, Namespace: namespace},
		Spec: v1beta1.PipelineRunSpec{
			PipelineRef: &v1beta1.PipelineRef{Name: helmDeployPipelineName},
			Params: []v1beta1.Param{{
				Name: "chartname", Value: *v1beta1.NewArrayOrString("gohelloworld"),
			}},
			Resources: []v1beta1.PipelineResourceBinding{{
				Name: "git-repo", ResourceRef: &v1beta1.PipelineResourceRef{Name: sourceResourceName},
			}, {
				Name: "the-image", ResourceRef: &v1beta1.PipelineResourceRef{Name: sourceImageName},
			}},
		},
	}
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
		if _, err := c.KubeClient.RbacV1beta1().ClusterRoleBindings().Create(ctx, crb, metav1.CreateOptions{}); err != nil {
			t.Fatalf("Failed to create cluster role binding for Helm %s", err)
		}
	}
}

func helmCleanup(ctx context.Context, c *clients, t *testing.T, namespace string) {
	t.Logf("Cleaning up helm from cluster...")

	removeAllHelmReleases(ctx, c, t, namespace)

	for _, crb := range clusterRoleBindings {
		t.Logf("Deleting Cluster Role binding %s for helm", crb.Name)
		if err := c.KubeClient.RbacV1beta1().ClusterRoleBindings().Delete(ctx, crb.Name, metav1.DeleteOptions{}); err != nil {
			t.Fatalf("Failed to delete cluster role binding for Helm %s", err)
		}
	}
}

func removeAllHelmReleases(ctx context.Context, c *clients, t *testing.T, namespace string) {
	helmRemoveAllTaskName := "helm-remove-all-task"
	helmRemoveAllTask := &v1beta1.Task{
		ObjectMeta: metav1.ObjectMeta{Name: helmRemoveAllTaskName, Namespace: namespace},
		Spec: v1beta1.TaskSpec{
			Steps: []v1beta1.Step{{Container: corev1.Container{
				Name:    "helm-remove-all",
				Image:   "alpine/helm:3.5.4",
				Command: []string{"/bin/sh"},
				Args:    []string{"-c", fmt.Sprintf("helm ls --short --all --namespace %s | xargs -n1 helm delete --namespace %s", namespace, namespace)},
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
