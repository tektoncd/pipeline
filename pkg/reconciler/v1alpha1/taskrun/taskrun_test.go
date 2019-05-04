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

package taskrun

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/knative/pkg/apis"
	duckv1beta1 "github.com/knative/pkg/apis/duck/v1beta1"
	"github.com/knative/pkg/configmap"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	fakeclientset "github.com/tektoncd/pipeline/pkg/client/clientset/versioned/fake"
	informers "github.com/tektoncd/pipeline/pkg/client/informers/externalversions"
	"github.com/tektoncd/pipeline/pkg/logging"
	"github.com/tektoncd/pipeline/pkg/reconciler"
	"github.com/tektoncd/pipeline/pkg/reconciler/v1alpha1/taskrun/entrypoint"
	"github.com/tektoncd/pipeline/pkg/reconciler/v1alpha1/taskrun/resources"
	"github.com/tektoncd/pipeline/pkg/system"
	"github.com/tektoncd/pipeline/test"
	tb "github.com/tektoncd/pipeline/test/builder"
	"github.com/tektoncd/pipeline/test/names"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	fakekubeclientset "k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"
)

const (
	entrypointLocation  = "/builder/tools/entrypoint"
	taskNameLabelKey    = pipeline.GroupName + pipeline.TaskLabelKey
	taskRunNameLabelKey = pipeline.GroupName + pipeline.TaskRunLabelKey
	workspaceDir        = "/workspace"
	currentApiVersion   = "tekton.dev/v1alpha1"
)

var (
	entrypointCache          *entrypoint.Cache
	ignoreLastTransitionTime = cmpopts.IgnoreTypes(apis.Condition{}.LastTransitionTime.Inner.Time)
	ignoreVolatileTime       = cmp.Comparer(func(_, _ apis.VolatileTime) bool { return true })
	// Pods are created with a random 3-byte (6 hex character) suffix that we want to ignore in our diffs.
	ignoreRandomPodNameSuffix = cmp.FilterPath(func(path cmp.Path) bool {
		return path.GoString() == "{v1.ObjectMeta}.Name"
	}, cmp.Comparer(func(name1, name2 string) bool {
		return name1[:len(name1)-6] == name2[:len(name2)-6]
	}))
	resourceQuantityCmp = cmp.Comparer(func(x, y resource.Quantity) bool {
		return x.Cmp(y) == 0
	})

	simpleStep  = tb.Step("simple-step", "foo", tb.Command("/mycmd"))
	simpleTask  = tb.Task("test-task", "foo", tb.TaskSpec(simpleStep))
	clustertask = tb.ClusterTask("test-cluster-task", tb.ClusterTaskSpec(simpleStep))

	outputTask = tb.Task("test-output-task", "foo", tb.TaskSpec(
		simpleStep, tb.TaskInputs(
			tb.InputsResource(gitResource.Name, v1alpha1.PipelineResourceTypeGit),
			tb.InputsResource(anotherGitResource.Name, v1alpha1.PipelineResourceTypeGit),
		),
		tb.TaskOutputs(tb.OutputsResource(gitResource.Name, v1alpha1.PipelineResourceTypeGit)),
	))

	saTask = tb.Task("test-with-sa", "foo", tb.TaskSpec(tb.Step("sa-step", "foo", tb.Command("/mycmd"))))

	taskEnvTask = tb.Task("test-task-env", "foo", tb.TaskSpec(
		tb.TaskContainerTemplate(
			tb.EnvVar("FRUIT", "APPLE"),
		),
		tb.Step("env-step", "foo",
			tb.EnvVar("ANOTHER", "VARIABLE"),
			tb.EnvVar("FRUIT", "LEMON"),
			tb.Command("/mycmd"))))

	templatedTask = tb.Task("test-task-with-templating", "foo", tb.TaskSpec(
		tb.TaskInputs(
			tb.InputsResource("workspace", v1alpha1.PipelineResourceTypeGit),
			tb.InputsParam("myarg"), tb.InputsParam("myarghasdefault", tb.ParamDefault("dont see me")),
			tb.InputsParam("myarghasdefault2", tb.ParamDefault("thedefault")),
			tb.InputsParam("configmapname"),
		),
		tb.TaskOutputs(tb.OutputsResource("myimage", v1alpha1.PipelineResourceTypeImage)),
		tb.Step("mycontainer", "myimage", tb.Command("/mycmd"), tb.Args(
			"--my-arg=${inputs.params.myarg}",
			"--my-arg-with-default=${inputs.params.myarghasdefault}",
			"--my-arg-with-default2=${inputs.params.myarghasdefault2}",
			"--my-additional-arg=${outputs.resources.myimage.url}",
		)),
		tb.Step("myothercontainer", "myotherimage", tb.Command("/mycmd"), tb.Args(
			"--my-other-arg=${inputs.resources.workspace.url}",
		)),
		tb.TaskVolume("volume-configmap", tb.VolumeSource(corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: "${inputs.params.configmapname}",
				},
			},
		})),
	))

	gitResource = tb.PipelineResource("git-resource", "foo", tb.PipelineResourceSpec(
		v1alpha1.PipelineResourceTypeGit, tb.PipelineResourceSpecParam("URL", "https://foo.git"),
	))
	anotherGitResource = tb.PipelineResource("another-git-resource", "foo", tb.PipelineResourceSpec(
		v1alpha1.PipelineResourceTypeGit, tb.PipelineResourceSpecParam("URL", "https://foobar.git"),
	))
	imageResource = tb.PipelineResource("image-resource", "foo", tb.PipelineResourceSpec(
		v1alpha1.PipelineResourceTypeImage, tb.PipelineResourceSpecParam("URL", "gcr.io/kristoff/sven"),
	))

	toolsVolume = corev1.Volume{
		Name: "tools",
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	}
	workspaceVolume = corev1.Volume{
		Name: "workspace",
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	}
	homeVolume = corev1.Volume{
		Name: "home",
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	}

	getCredentialsInitContainer = func(suffix string, ops ...tb.ContainerOp) tb.PodSpecOp {
		actualOps := []tb.ContainerOp{
			tb.Command("/ko-app/creds-init"),
			tb.WorkingDir(workspaceDir),
			tb.EnvVar("HOME", "/builder/home"),
			tb.VolumeMount("workspace", workspaceDir),
			tb.VolumeMount("home", "/builder/home"),
		}

		actualOps = append(actualOps, ops...)

		return tb.PodInitContainer("build-step-credential-initializer-"+suffix, "override-with-creds:latest", actualOps...)
	}

	getPlaceToolsInitContainer = func(ops ...tb.ContainerOp) tb.PodSpecOp {
		actualOps := []tb.ContainerOp{
			tb.Command("/bin/sh"),
			tb.Args("-c", fmt.Sprintf("cp /ko-app/entrypoint %s", entrypointLocation)),
			tb.WorkingDir(workspaceDir),
			tb.EnvVar("HOME", "/builder/home"),
			tb.VolumeMount("tools", "/builder/tools"),
			tb.VolumeMount("workspace", workspaceDir),
			tb.VolumeMount("home", "/builder/home"),
		}

		actualOps = append(actualOps, ops...)

		return tb.PodInitContainer("build-step-place-tools", "override-with-entrypoint:latest", actualOps...)
	}
)

func getRunName(tr *v1alpha1.TaskRun) string {
	return strings.Join([]string{tr.Namespace, tr.Name}, "/")
}

// getTaskRunController returns an instance of the TaskRun controller/reconciler that has been seeded with
// d, where d represents the state of the system (existing resources) needed for the test.
func getTaskRunController(t *testing.T, d test.Data) test.TestAssets {
	entrypointCache, _ = entrypoint.NewCache()
	c, i := test.SeedTestData(t, d)
	observer, logs := observer.New(zap.InfoLevel)
	configMapWatcher := configmap.NewInformedWatcher(c.Kube, system.GetNamespace())
	stopCh := make(chan struct{})
	logger := zap.New(observer).Sugar()
	th := reconciler.NewTimeoutHandler(c.Kube, c.Pipeline, stopCh, logger)
	return test.TestAssets{
		Controller: NewController(
			reconciler.Options{
				Logger:            logger,
				KubeClientSet:     c.Kube,
				PipelineClientSet: c.Pipeline,
				ConfigMapWatcher:  configMapWatcher,
			},
			i.TaskRun,
			i.Task,
			i.ClusterTask,
			i.PipelineResource,
			i.Pod,
			entrypointCache,
			th,
		),
		Logs:      logs,
		Clients:   c,
		Informers: i,
	}
}

func TestReconcile(t *testing.T) {
	taskRunSuccess := tb.TaskRun("test-taskrun-run-success", "foo", tb.TaskRunSpec(
		tb.TaskRunTaskRef(simpleTask.Name, tb.TaskRefAPIVersion("a1")),
	))
	taskRunWithSaSuccess := tb.TaskRun("test-taskrun-with-sa-run-success", "foo", tb.TaskRunSpec(
		tb.TaskRunTaskRef(saTask.Name, tb.TaskRefAPIVersion("a1")), tb.TaskRunServiceAccount("test-sa"),
	))
	taskRunTaskEnv := tb.TaskRun("test-taskrun-task-env", "foo", tb.TaskRunSpec(
		tb.TaskRunTaskRef(taskEnvTask.Name, tb.TaskRefAPIVersion("a1")),
	))
	taskRunTemplating := tb.TaskRun("test-taskrun-templating", "foo", tb.TaskRunSpec(
		tb.TaskRunTaskRef(templatedTask.Name, tb.TaskRefAPIVersion("a1")),
		tb.TaskRunInputs(
			tb.TaskRunInputsParam("myarg", "foo"),
			tb.TaskRunInputsParam("myarghasdefault", "bar"),
			tb.TaskRunInputsParam("configmapname", "configbar"),
			tb.TaskRunInputsResource("workspace", tb.TaskResourceBindingRef(gitResource.Name)),
		),
		tb.TaskRunOutputs(tb.TaskRunOutputsResource("myimage", tb.TaskResourceBindingRef("image-resource"))),
	))
	taskRunInputOutput := tb.TaskRun("test-taskrun-input-output", "foo",
		tb.TaskRunOwnerReference("PipelineRun", "test"),
		tb.TaskRunSpec(
			tb.TaskRunTaskRef(outputTask.Name),
			tb.TaskRunInputs(
				tb.TaskRunInputsResource(gitResource.Name,
					tb.TaskResourceBindingRef(gitResource.Name),
					tb.TaskResourceBindingPaths("source-folder"),
				),
				tb.TaskRunInputsResource(anotherGitResource.Name,
					tb.TaskResourceBindingRef(anotherGitResource.Name),
					tb.TaskResourceBindingPaths("source-folder"),
				),
			),
			tb.TaskRunOutputs(
				tb.TaskRunOutputsResource(gitResource.Name,
					tb.TaskResourceBindingPaths("output-folder"),
				),
			),
		),
	)
	taskRunWithTaskSpec := tb.TaskRun("test-taskrun-with-taskspec", "foo", tb.TaskRunSpec(
		tb.TaskRunInputs(
			tb.TaskRunInputsParam("myarg", "foo"),
			tb.TaskRunInputsResource("workspace", tb.TaskResourceBindingRef(gitResource.Name)),
		),
		tb.TaskRunTaskSpec(
			tb.TaskInputs(
				tb.InputsResource("workspace", v1alpha1.PipelineResourceTypeGit),
				tb.InputsParam("myarg", tb.ParamDefault("mydefault")),
			),
			tb.Step("mycontainer", "myimage", tb.Command("/mycmd"),
				tb.Args("--my-arg=${inputs.params.myarg}"),
			),
		),
	))

	taskRunWithResourceSpecAndTaskSpec := tb.TaskRun("test-taskrun-with-resource-spec", "foo", tb.TaskRunSpec(
		tb.TaskRunInputs(
			tb.TaskRunInputsResource("workspace", tb.TaskResourceBindingResourceSpec(&v1alpha1.PipelineResourceSpec{
				Type: v1alpha1.PipelineResourceTypeGit,
				Params: []v1alpha1.Param{{
					Name:  "URL",
					Value: "github.com/foo/bar.git",
				}, {
					Name:  "revision",
					Value: "rel-can",
				}},
			})),
		),
		tb.TaskRunTaskSpec(
			tb.TaskInputs(
				tb.InputsResource("workspace", v1alpha1.PipelineResourceTypeGit)),
			tb.Step("mystep", "ubuntu", tb.Command("/mycmd")),
		),
	))

	taskRunWithClusterTask := tb.TaskRun("test-taskrun-with-cluster-task", "foo",
		tb.TaskRunSpec(tb.TaskRunTaskRef(clustertask.Name, tb.TaskRefKind(v1alpha1.ClusterTaskKind))),
	)

	taskRunWithLabels := tb.TaskRun("test-taskrun-with-labels", "foo",
		tb.TaskRunLabel("TaskRunLabel", "TaskRunValue"),
		tb.TaskRunLabel(taskRunNameLabelKey, "WillNotBeUsed"),
		tb.TaskRunSpec(
			tb.TaskRunTaskRef(simpleTask.Name),
		),
	)

	taskRunWithAnnotations := tb.TaskRun("test-taskrun-with-annotations", "foo",
		tb.TaskRunAnnotation("TaskRunAnnotation", "TaskRunValue"),
		tb.TaskRunSpec(
			tb.TaskRunTaskRef(simpleTask.Name),
		),
	)

	taskRunWithResourceRequests := tb.TaskRun("test-taskrun-with-resource-requests", "foo",
		tb.TaskRunSpec(
			tb.TaskRunTaskSpec(
				tb.Step("step1", "foo",
					tb.Command("/mycmd"),
					tb.Resources(
						tb.Limits(
							tb.CPU("8"),
							tb.Memory("10Gi"),
						),
						tb.Requests(
							tb.CPU("4"),
							tb.Memory("3Gi"),
						),
					),
				),
				tb.Step("step2", "foo",
					tb.Command("/mycmd"),
					tb.Resources(
						tb.Limits(tb.Memory("5Gi")),
						tb.Requests(
							tb.CPU("2"),
							tb.Memory("5Gi"),
							tb.EphemeralStorage("25Gi"),
						),
					),
				),
				tb.Step("step3", "foo",
					tb.Command("/mycmd"),
				),
			),
		),
	)

	taskRunWithPod := tb.TaskRun("test-taskrun-with-pod", "foo",
		tb.TaskRunSpec(tb.TaskRunTaskRef(simpleTask.Name)),
		tb.TaskRunStatus(tb.PodName("some-pod-that-no-longer-exists")),
	)

	taskruns := []*v1alpha1.TaskRun{
		taskRunSuccess, taskRunWithSaSuccess,
		taskRunTemplating, taskRunInputOutput,
		taskRunWithTaskSpec, taskRunWithClusterTask, taskRunWithResourceSpecAndTaskSpec,
		taskRunWithLabels, taskRunWithAnnotations, taskRunWithResourceRequests, taskRunTaskEnv, taskRunWithPod,
	}

	d := test.Data{
		TaskRuns:          taskruns,
		Tasks:             []*v1alpha1.Task{simpleTask, saTask, templatedTask, outputTask, taskEnvTask},
		ClusterTasks:      []*v1alpha1.ClusterTask{clustertask},
		PipelineResources: []*v1alpha1.PipelineResource{gitResource, anotherGitResource, imageResource},
	}
	for _, tc := range []struct {
		name    string
		taskRun *v1alpha1.TaskRun
		wantPod *corev1.Pod
	}{{
		name:    "success",
		taskRun: taskRunSuccess,
		wantPod: tb.Pod("test-taskrun-run-success-pod-123456", "foo",
			tb.PodAnnotation("sidecar.istio.io/inject", "false"),
			tb.PodLabel(taskNameLabelKey, "test-task"),
			tb.PodLabel(taskRunNameLabelKey, "test-taskrun-run-success"),
			tb.PodOwnerReference("TaskRun", "test-taskrun-run-success",
				tb.OwnerReferenceAPIVersion(currentApiVersion)),
			tb.PodSpec(
				tb.PodVolumes(toolsVolume, workspaceVolume, homeVolume),
				tb.PodRestartPolicy(corev1.RestartPolicyNever),
				getCredentialsInitContainer("9l9zj"),
				getPlaceToolsInitContainer(),
				tb.PodContainer("build-step-simple-step", "foo",
					tb.Command(entrypointLocation),
					tb.Args("-wait_file", "", "-post_file", "/builder/tools/0", "-entrypoint", "/mycmd", "--"),
					tb.WorkingDir(workspaceDir),
					tb.EnvVar("HOME", "/builder/home"),
					tb.VolumeMount("tools", "/builder/tools"),
					tb.VolumeMount("workspace", workspaceDir),
					tb.VolumeMount("home", "/builder/home"),
					tb.Resources(tb.Requests(
						tb.CPU("0"),
						tb.Memory("0"),
						tb.EphemeralStorage("0"),
					)),
				),
				tb.PodContainer("nop", "override-with-nop:latest",
					tb.Command("/builder/tools/entrypoint"),
					tb.Args("-wait_file", "/builder/tools/0", "-post_file", "/builder/tools/1", "-entrypoint", "/ko-app/nop", "--"),
					tb.VolumeMount(entrypoint.MountName, entrypoint.MountPoint),
				),
			),
		),
	}, {
		name:    "serviceaccount",
		taskRun: taskRunWithSaSuccess,
		wantPod: tb.Pod("test-taskrun-with-sa-run-success-pod-123456", "foo",
			tb.PodAnnotation("sidecar.istio.io/inject", "false"),
			tb.PodLabel(taskNameLabelKey, "test-with-sa"),
			tb.PodLabel(taskRunNameLabelKey, "test-taskrun-with-sa-run-success"),
			tb.PodOwnerReference("TaskRun", "test-taskrun-with-sa-run-success",
				tb.OwnerReferenceAPIVersion(currentApiVersion)),
			tb.PodSpec(
				tb.PodServiceAccountName("test-sa"),
				tb.PodVolumes(toolsVolume, workspaceVolume, homeVolume),
				tb.PodRestartPolicy(corev1.RestartPolicyNever),
				getCredentialsInitContainer("9l9zj"),
				getPlaceToolsInitContainer(),
				tb.PodContainer("build-step-sa-step", "foo",
					tb.Command(entrypointLocation),
					tb.Args("-wait_file", "", "-post_file", "/builder/tools/0", "-entrypoint", "/mycmd", "--"),
					tb.WorkingDir(workspaceDir),
					tb.EnvVar("HOME", "/builder/home"),
					tb.VolumeMount("tools", "/builder/tools"),
					tb.VolumeMount("workspace", workspaceDir),
					tb.VolumeMount("home", "/builder/home"),
					tb.Resources(tb.Requests(
						tb.CPU("0"),
						tb.Memory("0"),
						tb.EphemeralStorage("0"),
					)),
				),
				tb.PodContainer("nop", "override-with-nop:latest",
					tb.Command("/builder/tools/entrypoint"),
					tb.Args("-wait_file", "/builder/tools/0", "-post_file", "/builder/tools/1", "-entrypoint", "/ko-app/nop", "--"),
					tb.VolumeMount(entrypoint.MountName, entrypoint.MountPoint),
				),
			),
		),
	}, {
		name:    "params",
		taskRun: taskRunTemplating,
		wantPod: tb.Pod("test-taskrun-templating-pod-123456", "foo",
			tb.PodAnnotation("sidecar.istio.io/inject", "false"),
			tb.PodLabel(taskNameLabelKey, "test-task-with-templating"),
			tb.PodLabel(taskRunNameLabelKey, "test-taskrun-templating"),
			tb.PodOwnerReference("TaskRun", "test-taskrun-templating",
				tb.OwnerReferenceAPIVersion(currentApiVersion)),
			tb.PodSpec(
				tb.PodVolumes(corev1.Volume{
					Name: "volume-configmap",
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "configbar",
							},
						},
					},
				}, toolsVolume, workspaceVolume, homeVolume),
				tb.PodRestartPolicy(corev1.RestartPolicyNever),
				getCredentialsInitContainer("78c5n"),
				getPlaceToolsInitContainer(),
				tb.PodContainer("build-step-git-source-git-resource-mssqb", "override-with-git:latest",
					tb.Command(entrypointLocation),
					tb.Args("-wait_file", "", "-post_file", "/builder/tools/0", "-entrypoint", "/ko-app/git-init", "--",
						"-url", "https://foo.git", "-revision", "master", "-path", "/workspace/workspace"),
					tb.WorkingDir(workspaceDir),
					tb.EnvVar("HOME", "/builder/home"),
					tb.VolumeMount("tools", "/builder/tools"),
					tb.VolumeMount("workspace", workspaceDir),
					tb.VolumeMount("home", "/builder/home"),
					tb.Resources(tb.Requests(
						tb.CPU("0"),
						tb.Memory("0"),
						tb.EphemeralStorage("0"),
					)),
				),
				tb.PodContainer("build-step-mycontainer", "myimage",
					tb.Command(entrypointLocation),
					tb.Args("-wait_file", "/builder/tools/0", "-post_file", "/builder/tools/1", "-entrypoint", "/mycmd", "--",
						"--my-arg=foo", "--my-arg-with-default=bar", "--my-arg-with-default2=thedefault",
						"--my-additional-arg=gcr.io/kristoff/sven"),
					tb.WorkingDir(workspaceDir),
					tb.EnvVar("HOME", "/builder/home"),
					tb.VolumeMount("tools", "/builder/tools"),
					tb.VolumeMount("workspace", workspaceDir),
					tb.VolumeMount("home", "/builder/home"),
					tb.Resources(tb.Requests(
						tb.CPU("0"),
						tb.Memory("0"),
						tb.EphemeralStorage("0"),
					)),
				),
				tb.PodContainer("build-step-image-digest-exporter-mycontainer-9l9zj", "override-with-imagedigest-exporter-image:latest",
					tb.Command("/builder/tools/entrypoint"),
					tb.Args("-wait_file", "/builder/tools/1", "-post_file", "/builder/tools/2", "-entrypoint", "/ko-app/imagedigestexporter", "--",
						"-images", "[{\"name\":\"image-resource\",\"type\":\"image\",\"url\":\"gcr.io/kristoff/sven\",\"digest\":\"\",\"OutputImageDir\":\"\"}]"),
					tb.WorkingDir(workspaceDir),
					tb.EnvVar("HOME", "/builder/home"),
					tb.VolumeMount("tools", "/builder/tools"),
					tb.VolumeMount("workspace", workspaceDir),
					tb.VolumeMount("home", "/builder/home"),
					tb.Resources(tb.Requests(
						tb.CPU("0"),
						tb.Memory("0"),
						tb.EphemeralStorage("0"),
					)),
				),
				tb.PodContainer("build-step-myothercontainer", "myotherimage",
					tb.Command(entrypointLocation),
					tb.Args("-wait_file", "/builder/tools/2", "-post_file", "/builder/tools/3", "-entrypoint", "/mycmd", "--",
						"--my-other-arg=https://foo.git"),
					tb.WorkingDir(workspaceDir),
					tb.EnvVar("HOME", "/builder/home"),
					tb.VolumeMount("tools", "/builder/tools"),
					tb.VolumeMount("workspace", workspaceDir),
					tb.VolumeMount("home", "/builder/home"),
					tb.Resources(tb.Requests(
						tb.CPU("0"),
						tb.Memory("0"),
						tb.EphemeralStorage("0"),
					)),
				),
				tb.PodContainer("build-step-image-digest-exporter-myothercontainer-mz4c7", "override-with-imagedigest-exporter-image:latest",
					tb.Command(entrypointLocation),
					tb.Args("-wait_file", "/builder/tools/3", "-post_file", "/builder/tools/4", "-entrypoint", "/ko-app/imagedigestexporter", "--",
						"-images", "[{\"name\":\"image-resource\",\"type\":\"image\",\"url\":\"gcr.io/kristoff/sven\",\"digest\":\"\",\"OutputImageDir\":\"\"}]"),
					tb.WorkingDir(workspaceDir),
					tb.EnvVar("HOME", "/builder/home"),
					tb.VolumeMount("tools", "/builder/tools"),
					tb.VolumeMount("workspace", workspaceDir),
					tb.VolumeMount("home", "/builder/home"),
					tb.Resources(tb.Requests(
						tb.CPU("0"),
						tb.Memory("0"),
						tb.EphemeralStorage("0"),
					)),
				),
				tb.PodContainer("nop", "override-with-nop:latest",
					tb.Command("/builder/tools/entrypoint"),
					tb.Args("-wait_file", "/builder/tools/4", "-post_file", "/builder/tools/5", "-entrypoint", "/ko-app/nop", "--"),
					tb.VolumeMount(entrypoint.MountName, entrypoint.MountPoint),
				),
			),
		),
	}, {
		name:    "wrap-steps",
		taskRun: taskRunInputOutput,
		wantPod: tb.Pod("test-taskrun-input-output-pod-123456", "foo",
			tb.PodAnnotation("sidecar.istio.io/inject", "false"),
			tb.PodLabel(taskNameLabelKey, "test-output-task"),
			tb.PodLabel(taskRunNameLabelKey, "test-taskrun-input-output"),
			tb.PodOwnerReference("TaskRun", "test-taskrun-input-output",
				tb.OwnerReferenceAPIVersion(currentApiVersion)),
			tb.PodSpec(
				tb.PodVolumes(corev1.Volume{
					Name: "test-pvc",
					VolumeSource: corev1.VolumeSource{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: "test-pvc",
							ReadOnly:  false,
						},
					},
				}, toolsVolume, workspaceVolume, homeVolume),
				tb.PodRestartPolicy(corev1.RestartPolicyNever),
				getCredentialsInitContainer("vr6ds"),
				getPlaceToolsInitContainer(),
				tb.PodContainer("build-step-create-dir-another-git-resource-78c5n", "override-with-bash-noop:latest",
					tb.Command(entrypointLocation),
					tb.Args("-wait_file", "", "-post_file", "/builder/tools/0", "-entrypoint", "/ko-app/bash", "--",
						"-args", "mkdir -p /workspace/another-git-resource"),
					tb.WorkingDir(workspaceDir),
					tb.EnvVar("HOME", "/builder/home"),
					tb.VolumeMount("tools", "/builder/tools"),
					tb.VolumeMount("workspace", workspaceDir),
					tb.VolumeMount("home", "/builder/home"),
					tb.Resources(tb.Requests(
						tb.CPU("0"),
						tb.Memory("0"),
						tb.EphemeralStorage("0"),
					)),
				),
				tb.PodContainer("build-step-source-copy-another-git-resource-mssqb", "override-with-bash-noop:latest",
					tb.Command(entrypointLocation),
					tb.Args("-wait_file", "/builder/tools/0", "-post_file", "/builder/tools/1", "-entrypoint", "/ko-app/bash", "--",
						"-args", "cp -r source-folder/. /workspace/another-git-resource"),
					tb.WorkingDir(workspaceDir),
					tb.EnvVar("HOME", "/builder/home"),
					tb.VolumeMount("test-pvc", "/pvc"),
					tb.VolumeMount("tools", "/builder/tools"),
					tb.VolumeMount("workspace", workspaceDir),
					tb.VolumeMount("home", "/builder/home"),
					tb.Resources(tb.Requests(
						tb.CPU("0"),
						tb.Memory("0"),
						tb.EphemeralStorage("0"),
					)),
				),
				tb.PodContainer("build-step-create-dir-git-resource-mz4c7", "override-with-bash-noop:latest",
					tb.Command(entrypointLocation),
					tb.Args("-wait_file", "/builder/tools/1", "-post_file", "/builder/tools/2", "-entrypoint", "/ko-app/bash", "--",
						"-args", "mkdir -p /workspace/git-resource"),
					tb.WorkingDir(workspaceDir),
					tb.EnvVar("HOME", "/builder/home"),
					tb.VolumeMount("tools", "/builder/tools"),
					tb.VolumeMount("workspace", workspaceDir),
					tb.VolumeMount("home", "/builder/home"),
					tb.Resources(tb.Requests(
						tb.CPU("0"),
						tb.Memory("0"),
						tb.EphemeralStorage("0"),
					)),
				),
				tb.PodContainer("build-step-source-copy-git-resource-9l9zj", "override-with-bash-noop:latest",
					tb.Command(entrypointLocation),
					tb.Args("-wait_file", "/builder/tools/2", "-post_file", "/builder/tools/3", "-entrypoint", "/ko-app/bash", "--",
						"-args", "cp -r source-folder/. /workspace/git-resource"),
					tb.WorkingDir(workspaceDir),
					tb.EnvVar("HOME", "/builder/home"),
					tb.VolumeMount("test-pvc", "/pvc"),
					tb.VolumeMount("tools", "/builder/tools"),
					tb.VolumeMount("workspace", workspaceDir),
					tb.VolumeMount("home", "/builder/home"),
					tb.Resources(tb.Requests(
						tb.CPU("0"),
						tb.Memory("0"),
						tb.EphemeralStorage("0"),
					)),
				),
				tb.PodContainer("build-step-simple-step", "foo",
					tb.Command(entrypointLocation),
					tb.Args("-wait_file", "/builder/tools/3", "-post_file", "/builder/tools/4", "-entrypoint", "/mycmd", "--"),
					tb.WorkingDir(workspaceDir),
					tb.EnvVar("HOME", "/builder/home"),
					tb.VolumeMount("tools", "/builder/tools"),
					tb.VolumeMount("workspace", workspaceDir),
					tb.VolumeMount("home", "/builder/home"),
					tb.Resources(tb.Requests(
						tb.CPU("0"),
						tb.Memory("0"),
						tb.EphemeralStorage("0"),
					)),
				),
				tb.PodContainer("build-step-source-mkdir-git-resource-6nl7g", "override-with-bash-noop:latest",
					tb.Command(entrypointLocation),
					tb.Args("-wait_file", "/builder/tools/4", "-post_file", "/builder/tools/5", "-entrypoint", "/ko-app/bash", "--",
						"-args", "mkdir -p output-folder"),
					tb.WorkingDir(workspaceDir),
					tb.EnvVar("HOME", "/builder/home"),
					tb.VolumeMount("test-pvc", "/pvc"),
					tb.VolumeMount("tools", "/builder/tools"),
					tb.VolumeMount("workspace", workspaceDir),
					tb.VolumeMount("home", "/builder/home"),
					tb.Resources(tb.Requests(
						tb.CPU("0"),
						tb.Memory("0"),
						tb.EphemeralStorage("0"),
					)),
				),
				tb.PodContainer("build-step-source-copy-git-resource-j2tds", "override-with-bash-noop:latest",
					tb.Command(entrypointLocation),
					tb.Args("-wait_file", "/builder/tools/5", "-post_file", "/builder/tools/6", "-entrypoint", "/ko-app/bash", "--",
						"-args", "cp -r /workspace/git-resource/. output-folder"),
					tb.WorkingDir(workspaceDir),
					tb.EnvVar("HOME", "/builder/home"),
					tb.VolumeMount("test-pvc", "/pvc"),
					tb.VolumeMount("tools", "/builder/tools"),
					tb.VolumeMount("workspace", workspaceDir),
					tb.VolumeMount("home", "/builder/home"),
					tb.Resources(tb.Requests(
						tb.CPU("0"),
						tb.Memory("0"),
						tb.EphemeralStorage("0"),
					)),
				),
				tb.PodContainer("nop", "override-with-nop:latest",
					tb.Command("/builder/tools/entrypoint"),
					tb.Args("-wait_file", "/builder/tools/6", "-post_file", "/builder/tools/7", "-entrypoint", "/ko-app/nop", "--"),
					tb.VolumeMount(entrypoint.MountName, entrypoint.MountPoint),
				),
			),
		),
	}, {
		name:    "taskrun-with-taskspec",
		taskRun: taskRunWithTaskSpec,
		wantPod: tb.Pod("test-taskrun-with-taskspec-pod-123456", "foo",
			tb.PodAnnotation("sidecar.istio.io/inject", "false"),
			tb.PodLabel(taskRunNameLabelKey, "test-taskrun-with-taskspec"),
			tb.PodOwnerReference("TaskRun", "test-taskrun-with-taskspec",
				tb.OwnerReferenceAPIVersion(currentApiVersion)),
			tb.PodSpec(
				tb.PodVolumes(toolsVolume, workspaceVolume, homeVolume),
				tb.PodRestartPolicy(corev1.RestartPolicyNever),
				getCredentialsInitContainer("mz4c7"),
				getPlaceToolsInitContainer(),
				tb.PodContainer("build-step-git-source-git-resource-9l9zj", "override-with-git:latest",
					tb.Command(entrypointLocation),
					tb.Args("-wait_file", "", "-post_file", "/builder/tools/0", "-entrypoint", "/ko-app/git-init", "--",
						"-url", "https://foo.git", "-revision", "master", "-path", "/workspace/workspace"),
					tb.WorkingDir(workspaceDir),
					tb.EnvVar("HOME", "/builder/home"),
					tb.VolumeMount("tools", "/builder/tools"),
					tb.VolumeMount("workspace", workspaceDir),
					tb.VolumeMount("home", "/builder/home"),
					tb.Resources(tb.Requests(
						tb.CPU("0"),
						tb.Memory("0"),
						tb.EphemeralStorage("0"),
					)),
				),
				tb.PodContainer("build-step-mycontainer", "myimage",
					tb.Command(entrypointLocation),
					tb.WorkingDir(workspaceDir),
					tb.Args("-wait_file", "/builder/tools/0", "-post_file", "/builder/tools/1", "-entrypoint", "/mycmd", "--",
						"--my-arg=foo"),
					tb.EnvVar("HOME", "/builder/home"),
					tb.VolumeMount("tools", "/builder/tools"),
					tb.VolumeMount("workspace", workspaceDir),
					tb.VolumeMount("home", "/builder/home"),
					tb.Resources(tb.Requests(
						tb.CPU("0"),
						tb.Memory("0"),
						tb.EphemeralStorage("0"),
					)),
				),
				tb.PodContainer("nop", "override-with-nop:latest",
					tb.Command("/builder/tools/entrypoint"),
					tb.Args("-wait_file", "/builder/tools/1", "-post_file", "/builder/tools/2", "-entrypoint", "/ko-app/nop", "--"),
					tb.VolumeMount(entrypoint.MountName, entrypoint.MountPoint),
				),
			),
		),
	}, {
		name:    "success-with-cluster-task",
		taskRun: taskRunWithClusterTask,
		wantPod: tb.Pod("test-taskrun-with-cluster-task-pod-123456", "foo",
			tb.PodAnnotation("sidecar.istio.io/inject", "false"),
			tb.PodLabel(taskNameLabelKey, "test-cluster-task"),
			tb.PodLabel(taskRunNameLabelKey, "test-taskrun-with-cluster-task"),
			tb.PodOwnerReference("TaskRun", "test-taskrun-with-cluster-task",
				tb.OwnerReferenceAPIVersion(currentApiVersion)),
			tb.PodSpec(
				tb.PodVolumes(toolsVolume, workspaceVolume, homeVolume),
				tb.PodRestartPolicy(corev1.RestartPolicyNever),
				getCredentialsInitContainer("9l9zj"),
				getPlaceToolsInitContainer(),
				tb.PodContainer("build-step-simple-step", "foo",
					tb.Command(entrypointLocation),
					tb.Args("-wait_file", "", "-post_file", "/builder/tools/0", "-entrypoint", "/mycmd", "--"),
					tb.WorkingDir(workspaceDir),
					tb.EnvVar("HOME", "/builder/home"),
					tb.VolumeMount("tools", "/builder/tools"),
					tb.VolumeMount("workspace", workspaceDir),
					tb.VolumeMount("home", "/builder/home"),
					tb.Resources(tb.Requests(
						tb.CPU("0"),
						tb.Memory("0"),
						tb.EphemeralStorage("0"),
					)),
				),
				tb.PodContainer("nop", "override-with-nop:latest",
					tb.Command("/builder/tools/entrypoint"),
					tb.Args("-wait_file", "/builder/tools/0", "-post_file", "/builder/tools/1", "-entrypoint", "/ko-app/nop", "--"),
					tb.VolumeMount(entrypoint.MountName, entrypoint.MountPoint),
				),
			),
		),
	}, {
		name:    "taskrun-with-resource-spec-task-spec",
		taskRun: taskRunWithResourceSpecAndTaskSpec,
		wantPod: tb.Pod("test-taskrun-with-resource-spec-pod-123456", "foo",
			tb.PodAnnotation("sidecar.istio.io/inject", "false"),
			tb.PodLabel(taskRunNameLabelKey, "test-taskrun-with-resource-spec"),
			tb.PodOwnerReference("TaskRun", "test-taskrun-with-resource-spec",
				tb.OwnerReferenceAPIVersion(currentApiVersion)),
			tb.PodSpec(
				tb.PodVolumes(toolsVolume, workspaceVolume, homeVolume),
				tb.PodRestartPolicy(corev1.RestartPolicyNever),
				getCredentialsInitContainer("mz4c7"),
				getPlaceToolsInitContainer(),
				tb.PodContainer("build-step-git-source-workspace-9l9zj", "override-with-git:latest",
					tb.Command(entrypointLocation),
					tb.Args("-wait_file", "", "-post_file", "/builder/tools/0", "-entrypoint", "/ko-app/git-init", "--",
						"-url", "github.com/foo/bar.git", "-revision", "rel-can", "-path",
						"/workspace/workspace"),
					tb.WorkingDir(workspaceDir),
					tb.EnvVar("HOME", "/builder/home"),
					tb.VolumeMount("tools", "/builder/tools"),
					tb.VolumeMount("workspace", workspaceDir),
					tb.VolumeMount("home", "/builder/home"),
					tb.Resources(tb.Requests(
						tb.CPU("0"),
						tb.Memory("0"),
						tb.EphemeralStorage("0"),
					)),
				),
				tb.PodContainer("build-step-mystep", "ubuntu",
					tb.Command(entrypointLocation),
					tb.Args("-wait_file", "/builder/tools/0", "-post_file", "/builder/tools/1", "-entrypoint", "/mycmd", "--"),
					tb.WorkingDir(workspaceDir),
					tb.EnvVar("HOME", "/builder/home"),
					tb.VolumeMount("tools", "/builder/tools"),
					tb.VolumeMount("workspace", workspaceDir),
					tb.VolumeMount("home", "/builder/home"),
					tb.Resources(tb.Requests(
						tb.CPU("0"),
						tb.Memory("0"),
						tb.EphemeralStorage("0"),
					)),
				),
				tb.PodContainer("nop", "override-with-nop:latest",
					tb.Command("/builder/tools/entrypoint"),
					tb.Args("-wait_file", "/builder/tools/1", "-post_file", "/builder/tools/2", "-entrypoint", "/ko-app/nop", "--"),
					tb.VolumeMount(entrypoint.MountName, entrypoint.MountPoint),
				),
			),
		),
	}, {
		name:    "taskrun-with-labels",
		taskRun: taskRunWithLabels,
		wantPod: tb.Pod("test-taskrun-with-labels-pod-123456", "foo",
			tb.PodAnnotation("sidecar.istio.io/inject", "false"),
			tb.PodLabel("TaskRunLabel", "TaskRunValue"),
			tb.PodLabel(taskNameLabelKey, "test-task"),
			tb.PodLabel(taskRunNameLabelKey, "test-taskrun-with-labels"),
			tb.PodOwnerReference("TaskRun", "test-taskrun-with-labels",
				tb.OwnerReferenceAPIVersion(currentApiVersion)),
			tb.PodSpec(
				tb.PodVolumes(toolsVolume, workspaceVolume, homeVolume),
				tb.PodRestartPolicy(corev1.RestartPolicyNever),
				getCredentialsInitContainer("9l9zj"),
				getPlaceToolsInitContainer(),
				tb.PodContainer("build-step-simple-step", "foo",
					tb.Command(entrypointLocation),
					tb.Args("-wait_file", "", "-post_file", "/builder/tools/0", "-entrypoint", "/mycmd", "--"),
					tb.WorkingDir(workspaceDir),
					tb.EnvVar("HOME", "/builder/home"),
					tb.VolumeMount("tools", "/builder/tools"),
					tb.VolumeMount("workspace", workspaceDir),
					tb.VolumeMount("home", "/builder/home"),
					tb.Resources(tb.Requests(
						tb.CPU("0"),
						tb.Memory("0"),
						tb.EphemeralStorage("0"),
					)),
				),
				tb.PodContainer("nop", "override-with-nop:latest",
					tb.Command("/builder/tools/entrypoint"),
					tb.Args("-wait_file", "/builder/tools/0", "-post_file", "/builder/tools/1", "-entrypoint", "/ko-app/nop", "--"),
					tb.VolumeMount(entrypoint.MountName, entrypoint.MountPoint),
				),
			),
		),
	}, {
		name:    "taskrun-with-annotations",
		taskRun: taskRunWithAnnotations,
		wantPod: tb.Pod("test-taskrun-with-annotations-pod-123456", "foo",
			tb.PodAnnotation("sidecar.istio.io/inject", "false"),
			tb.PodAnnotation("TaskRunAnnotation", "TaskRunValue"),
			tb.PodLabel(taskNameLabelKey, "test-task"),
			tb.PodLabel(taskRunNameLabelKey, "test-taskrun-with-annotations"),
			tb.PodOwnerReference("TaskRun", "test-taskrun-with-annotations",
				tb.OwnerReferenceAPIVersion(currentApiVersion)),
			tb.PodSpec(
				tb.PodVolumes(toolsVolume, workspaceVolume, homeVolume),
				tb.PodRestartPolicy(corev1.RestartPolicyNever),
				getCredentialsInitContainer("9l9zj"),
				getPlaceToolsInitContainer(),
				tb.PodContainer("build-step-simple-step", "foo",
					tb.Command(entrypointLocation),
					tb.Args("-wait_file", "", "-post_file", "/builder/tools/0", "-entrypoint", "/mycmd", "--"),
					tb.WorkingDir(workspaceDir),
					tb.EnvVar("HOME", "/builder/home"),
					tb.VolumeMount("tools", "/builder/tools"),
					tb.VolumeMount("workspace", workspaceDir),
					tb.VolumeMount("home", "/builder/home"),
					tb.Resources(tb.Requests(
						tb.CPU("0"),
						tb.Memory("0"),
						tb.EphemeralStorage("0"),
					)),
				),
				tb.PodContainer("nop", "override-with-nop:latest",
					tb.Command("/builder/tools/entrypoint"),
					tb.Args("-wait_file", "/builder/tools/0", "-post_file", "/builder/tools/1", "-entrypoint", "/ko-app/nop", "--"),
					tb.VolumeMount(entrypoint.MountName, entrypoint.MountPoint),
				),
			),
		),
	}, {
		name:    "task-env",
		taskRun: taskRunTaskEnv,
		wantPod: tb.Pod("test-taskrun-task-env-pod-311bc9", "foo",
			tb.PodAnnotation("sidecar.istio.io/inject", "false"),
			tb.PodLabel(taskNameLabelKey, "test-task-env"),
			tb.PodLabel(taskRunNameLabelKey, "test-taskrun-task-env"),
			tb.PodOwnerReference("TaskRun", "test-taskrun-task-env",
				tb.OwnerReferenceAPIVersion("tekton.dev/v1alpha1")),
			tb.PodSpec(
				tb.PodVolumes(toolsVolume, workspaceVolume, homeVolume),
				tb.PodRestartPolicy(corev1.RestartPolicyNever),
				getCredentialsInitContainer("9l9zj", tb.EnvVar("FRUIT", "APPLE")),
				getPlaceToolsInitContainer(tb.EnvVar("FRUIT", "APPLE")),
				tb.PodContainer("build-step-env-step", "foo", tb.Command(entrypointLocation),
					tb.Command(entrypointLocation),
					tb.Args("-wait_file", "", "-post_file", "/builder/tools/0", "-entrypoint", "/mycmd", "--"),
					tb.WorkingDir(workspaceDir),
					tb.EnvVar("HOME", "/builder/home"),
					tb.Resources(tb.Requests(
						tb.CPU("0"),
						tb.Memory("0"),
						tb.EphemeralStorage("0"),
					)),
					tb.VolumeMount("tools", "/builder/tools"),
					tb.VolumeMount("workspace", workspaceDir),
					tb.VolumeMount("home", "/builder/home"),
					tb.EnvVar("ANOTHER", "VARIABLE"),
					tb.EnvVar("FRUIT", "LEMON"),
				),
				tb.PodContainer("nop", "override-with-nop:latest",
					tb.Command("/builder/tools/entrypoint"),
					tb.Args("-wait_file", "/builder/tools/0", "-post_file", "/builder/tools/1", "-entrypoint", "/ko-app/nop", "--"),
					tb.VolumeMount(entrypoint.MountName, entrypoint.MountPoint),
					tb.EnvVar("FRUIT", "APPLE"),
				),
			),
		),
	}, {
		name:    "taskrun-with-resource-requests",
		taskRun: taskRunWithResourceRequests,
		wantPod: tb.Pod("test-taskrun-with-resource-requests-pod-123456", "foo",
			tb.PodAnnotation("sidecar.istio.io/inject", "false"),
			tb.PodLabel(taskRunNameLabelKey, "test-taskrun-with-resource-requests"),
			tb.PodOwnerReference("TaskRun", "test-taskrun-with-resource-requests",
				tb.OwnerReferenceAPIVersion(currentApiVersion)),
			tb.PodSpec(
				tb.PodVolumes(toolsVolume, workspaceVolume, homeVolume),
				tb.PodRestartPolicy(corev1.RestartPolicyNever),
				getCredentialsInitContainer("9l9zj"),
				getPlaceToolsInitContainer(),
				tb.PodContainer("build-step-step1", "foo",
					tb.Command(entrypointLocation),
					tb.Args("-wait_file", "", "-post_file", "/builder/tools/0", "-entrypoint", "/mycmd", "--"),
					tb.WorkingDir(workspaceDir),
					tb.EnvVar("HOME", "/builder/home"),
					tb.VolumeMount("tools", "/builder/tools"),
					tb.VolumeMount("workspace", workspaceDir),
					tb.VolumeMount("home", "/builder/home"),
					tb.Resources(
						tb.Limits(
							tb.CPU("8"),
							tb.Memory("10Gi"),
						),
						tb.Requests(
							tb.CPU("4"),
							tb.Memory("0"),
							tb.EphemeralStorage("0"),
						),
					),
				),
				tb.PodContainer("build-step-step2", "foo",
					tb.Command(entrypointLocation),
					tb.Args("-wait_file", "/builder/tools/0", "-post_file", "/builder/tools/1", "-entrypoint", "/mycmd", "--"),
					tb.WorkingDir(workspaceDir),
					tb.EnvVar("HOME", "/builder/home"),
					tb.VolumeMount("tools", "/builder/tools"),
					tb.VolumeMount("workspace", workspaceDir),
					tb.VolumeMount("home", "/builder/home"),
					tb.Resources(
						tb.Limits(tb.Memory("5Gi")),
						tb.Requests(
							tb.CPU("0"),
							tb.Memory("5Gi"),
							tb.EphemeralStorage("25Gi"),
						),
					),
				),
				tb.PodContainer("build-step-step3", "foo",
					tb.Command(entrypointLocation),
					tb.Args("-wait_file", "/builder/tools/1", "-post_file", "/builder/tools/2", "-entrypoint", "/mycmd", "--"),
					tb.WorkingDir(workspaceDir),
					tb.EnvVar("HOME", "/builder/home"),
					tb.VolumeMount("tools", "/builder/tools"),
					tb.VolumeMount("workspace", workspaceDir),
					tb.VolumeMount("home", "/builder/home"),
					tb.Resources(tb.Requests(
						tb.CPU("0"),
						tb.Memory("0"),
						tb.EphemeralStorage("0"),
					)),
				),
				tb.PodContainer("nop", "override-with-nop:latest",
					tb.Command("/builder/tools/entrypoint"),
					tb.Args("-wait_file", "/builder/tools/2", "-post_file", "/builder/tools/3", "-entrypoint", "/ko-app/nop", "--"),
					tb.VolumeMount(entrypoint.MountName, entrypoint.MountPoint),
				),
			),
		),
	}, {
		name:    "taskrun-with-pod",
		taskRun: taskRunWithPod,
		wantPod: tb.Pod("test-taskrun-with-pod-pod-123456", "foo",
			tb.PodAnnotation("sidecar.istio.io/inject", "false"),
			tb.PodLabel(taskNameLabelKey, "test-task"),
			tb.PodLabel(taskRunNameLabelKey, "test-taskrun-with-pod"),
			tb.PodOwnerReference("TaskRun", "test-taskrun-with-pod",
				tb.OwnerReferenceAPIVersion(currentApiVersion)),
			tb.PodSpec(
				tb.PodVolumes(toolsVolume, workspaceVolume, homeVolume),
				tb.PodRestartPolicy(corev1.RestartPolicyNever),
				getCredentialsInitContainer("9l9zj"),
				getPlaceToolsInitContainer(),
				tb.PodContainer("build-step-simple-step", "foo",
					tb.Command(entrypointLocation),
					tb.Args("-wait_file", "", "-post_file", "/builder/tools/0", "-entrypoint", "/mycmd", "--"),
					tb.WorkingDir(workspaceDir),
					tb.EnvVar("HOME", "/builder/home"),
					tb.VolumeMount("tools", "/builder/tools"),
					tb.VolumeMount("workspace", workspaceDir),
					tb.VolumeMount("home", "/builder/home"),
					tb.Resources(tb.Requests(
						tb.CPU("0"),
						tb.Memory("0"),
						tb.EphemeralStorage("0"),
					)),
				),
				tb.PodContainer("nop", "override-with-nop:latest",
					tb.Command("/builder/tools/entrypoint"),
					tb.Args("-wait_file", "/builder/tools/0", "-post_file", "/builder/tools/1", "-entrypoint", "/ko-app/nop", "--"),
					tb.VolumeMount(entrypoint.MountName, entrypoint.MountPoint),
				),
			),
		),
	}} {
		t.Run(tc.name, func(t *testing.T) {
			names.TestingSeed()
			testAssets := getTaskRunController(t, d)
			c := testAssets.Controller
			clients := testAssets.Clients
			saName := tc.taskRun.Spec.ServiceAccount
			if saName == "" {
				saName = "default"
			}
			if _, err := clients.Kube.CoreV1().ServiceAccounts(tc.taskRun.Namespace).Create(&corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name:      saName,
					Namespace: tc.taskRun.Namespace,
				},
			}); err != nil {
				t.Fatal(err)
			}

			entrypoint.AddToEntrypointCache(entrypointCache, "override-with-git:latest", []string{"/ko-app/git-init"})
			entrypoint.AddToEntrypointCache(entrypointCache, "override-with-bash-noop:latest", []string{"/ko-app/bash"})

			if err := c.Reconciler.Reconcile(context.Background(), getRunName(tc.taskRun)); err != nil {
				t.Errorf("expected no error. Got error %v", err)
			}
			if len(clients.Kube.Actions()) == 0 {
				t.Errorf("Expected actions to be logged in the kubeclient, got none")
			}

			namespace, name, err := cache.SplitMetaNamespaceKey(tc.taskRun.Name)
			if err != nil {
				t.Errorf("Invalid resource key: %v", err)
			}

			tr, err := clients.Pipeline.TektonV1alpha1().TaskRuns(namespace).Get(name, metav1.GetOptions{})
			if err != nil {
				t.Fatalf("getting updated taskrun: %v", err)
			}
			condition := tr.Status.GetCondition(apis.ConditionSucceeded)
			if condition == nil || condition.Status != corev1.ConditionUnknown {
				t.Errorf("Expected invalid TaskRun to have in progress status, but had %v", condition)
			}
			if condition != nil && condition.Reason != reasonRunning {
				t.Errorf("Expected reason %q but was %s", reasonRunning, condition.Reason)
			}

			if tr.Status.PodName == "" {
				t.Fatalf("Reconcile didn't set pod name")
			}

			pod, err := clients.Kube.CoreV1().Pods(tr.Namespace).Get(tr.Status.PodName, metav1.GetOptions{})
			if err != nil {
				t.Fatalf("Failed to fetch build pod: %v", err)
			}

			if d := cmp.Diff(pod.ObjectMeta, tc.wantPod.ObjectMeta, ignoreRandomPodNameSuffix); d != "" {
				t.Errorf("Pod metadata doesn't match, diff: %s", d)
			}

			if d := cmp.Diff(pod.Spec, tc.wantPod.Spec, resourceQuantityCmp); d != "" {
				t.Errorf("Pod spec doesn't match, diff: %s", d)
			}
			if len(clients.Kube.Actions()) == 0 {
				t.Fatalf("Expected actions to be logged in the kubeclient, got none")
			}
		})
	}
}

func TestReconcile_SetsStartTime(t *testing.T) {
	taskRun := tb.TaskRun("test-taskrun", "foo", tb.TaskRunSpec(
		tb.TaskRunTaskRef(simpleTask.Name),
	))
	d := test.Data{
		TaskRuns: []*v1alpha1.TaskRun{taskRun},
		Tasks:    []*v1alpha1.Task{simpleTask},
	}
	testAssets := getTaskRunController(t, d)

	if err := testAssets.Controller.Reconciler.Reconcile(context.Background(), getRunName(taskRun)); err != nil {
		t.Errorf("expected no error reconciling valid TaskRun but got %v", err)
	}

	if taskRun.Status.StartTime == nil || taskRun.Status.StartTime.IsZero() {
		t.Errorf("expected startTime to be set by reconcile but was %q", taskRun.Status.StartTime)
	}
}

func TestReconcile_DoesntChangeStartTime(t *testing.T) {
	startTime := time.Date(2000, 1, 1, 1, 1, 1, 1, time.UTC)
	taskRun := tb.TaskRun("test-taskrun", "foo", tb.TaskRunSpec(
		tb.TaskRunTaskRef(simpleTask.Name)),
		tb.TaskRunStatus(
			tb.TaskRunStartTime(startTime),
			tb.PodName("the-pod"),
		),
	)
	d := test.Data{
		TaskRuns: []*v1alpha1.TaskRun{taskRun},
		Tasks:    []*v1alpha1.Task{simpleTask},
		Pods: []*corev1.Pod{{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "foo",
				Name:      "the-pod",
			},
		}},
	}
	testAssets := getTaskRunController(t, d)

	if err := testAssets.Controller.Reconciler.Reconcile(context.Background(), getRunName(taskRun)); err != nil {
		t.Errorf("expected no error reconciling valid TaskRun but got %v", err)
	}

	if taskRun.Status.StartTime.Time != startTime {
		t.Errorf("expected startTime %q to be preserved by reconcile but was %q", startTime, taskRun.Status.StartTime)
	}
}

func TestReconcileInvalidTaskRuns(t *testing.T) {
	noTaskRun := tb.TaskRun("notaskrun", "foo", tb.TaskRunSpec(tb.TaskRunTaskRef("notask")))
	withWrongRef := tb.TaskRun("taskrun-with-wrong-ref", "foo", tb.TaskRunSpec(
		tb.TaskRunTaskRef("taskrun-with-wrong-ref", tb.TaskRefKind(v1alpha1.ClusterTaskKind)),
	))
	taskRuns := []*v1alpha1.TaskRun{noTaskRun, withWrongRef}
	tasks := []*v1alpha1.Task{simpleTask}

	d := test.Data{
		TaskRuns: taskRuns,
		Tasks:    tasks,
	}

	testcases := []struct {
		name    string
		taskRun *v1alpha1.TaskRun
		reason  string
	}{
		{
			name:    "task run with no task",
			taskRun: noTaskRun,
			reason:  reasonFailedResolution,
		},
		{
			name:    "task run with no task",
			taskRun: withWrongRef,
			reason:  reasonFailedResolution,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			testAssets := getTaskRunController(t, d)
			c := testAssets.Controller
			clients := testAssets.Clients
			err := c.Reconciler.Reconcile(context.Background(), getRunName(tc.taskRun))
			// When a TaskRun is invalid and can't run, we don't want to return an error because
			// an error will tell the Reconciler to keep trying to reconcile; instead we want to stop
			// and forget about the Run.
			if err != nil {
				t.Errorf("Did not expect to see error when reconciling invalid TaskRun but saw %q", err)
			}
			if len(clients.Kube.Actions()) != 0 {
				t.Errorf("expected no actions created by the reconciler, got %v", clients.Kube.Actions())
			}
			// Since the TaskRun is invalid, the status should say it has failed
			condition := tc.taskRun.Status.GetCondition(apis.ConditionSucceeded)
			if condition == nil || condition.Status != corev1.ConditionFalse {
				t.Errorf("Expected invalid TaskRun to have failed status, but had %v", condition)
			}
			if condition != nil && condition.Reason != tc.reason {
				t.Errorf("Expected failure to be because of reason %q but was %s", tc.reason, condition.Reason)
			}
		})
	}

}

func TestReconcilePodFetchError(t *testing.T) {
	taskRun := tb.TaskRun("test-taskrun-run-success", "foo",
		tb.TaskRunSpec(tb.TaskRunTaskRef("test-task")),
		tb.TaskRunStatus(tb.PodName("will-not-be-found")),
	)
	d := test.Data{
		TaskRuns: []*v1alpha1.TaskRun{taskRun},
		Tasks:    []*v1alpha1.Task{simpleTask},
	}

	testAssets := getTaskRunController(t, d)
	c := testAssets.Controller
	clients := testAssets.Clients

	clients.Kube.PrependReactor("*", "*", func(action ktesting.Action) (handled bool, ret runtime.Object, err error) {
		if action.GetVerb() == "get" && action.GetResource().Resource == "pods" {
			// handled fetching pods
			return true, nil, fmt.Errorf("induce failure fetching pods")
		}
		return false, nil, nil
	})

	if err := c.Reconciler.Reconcile(context.Background(), fmt.Sprintf("%s/%s", taskRun.Namespace, taskRun.Name)); err == nil {
		t.Fatal("expected error when reconciling a Task for which we couldn't get the corresponding Build Pod but got nil")
	}
}

func TestReconcilePodUpdateStatus(t *testing.T) {
	taskRun := tb.TaskRun("test-taskrun-run-success", "foo", tb.TaskRunSpec(tb.TaskRunTaskRef("test-task")))

	logger, _ := logging.NewLogger("", "")
	cache, _ := entrypoint.NewCache()
	// TODO(jasonhall): This avoids a circular dependency where
	// getTaskRunController takes a test.Data which must be populated with
	// a pod created from MakePod which requires a (fake) Kube client. When
	// we remove Build entirely from this controller, we should simply
	// specify the Pod we want to exist directly, and not call MakePod from
	// the build. This will break the cycle and allow us to simply use
	// clients normally.
	pod, err := resources.MakePod(taskRun, simpleTask.Spec, fakekubeclientset.NewSimpleClientset(&corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "default",
			Namespace: taskRun.Namespace,
		},
	}), cache, logger)
	if err != nil {
		t.Fatalf("MakePod: %v", err)
	}
	taskRun.Status = v1alpha1.TaskRunStatus{
		PodName: pod.Name,
	}
	d := test.Data{
		TaskRuns: []*v1alpha1.TaskRun{taskRun},
		Tasks:    []*v1alpha1.Task{simpleTask},
		Pods:     []*corev1.Pod{pod},
	}

	testAssets := getTaskRunController(t, d)
	c := testAssets.Controller
	clients := testAssets.Clients

	if err := c.Reconciler.Reconcile(context.Background(), fmt.Sprintf("%s/%s", taskRun.Namespace, taskRun.Name)); err != nil {
		t.Fatalf("Unexpected error when Reconcile() : %v", err)
	}
	newTr, err := clients.Pipeline.TektonV1alpha1().TaskRuns(taskRun.Namespace).Get(taskRun.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Expected TaskRun %s to exist but instead got error when getting it: %v", taskRun.Name, err)
	}
	if d := cmp.Diff(newTr.Status.GetCondition(apis.ConditionSucceeded), &apis.Condition{
		Type:    apis.ConditionSucceeded,
		Status:  corev1.ConditionUnknown,
		Message: "Running",
		Reason:  "Running",
	}, ignoreLastTransitionTime); d != "" {
		t.Fatalf("-got, +want: %v", d)
	}

	// update pod status and trigger reconcile : build is completed
	pod.Status = corev1.PodStatus{
		Phase: corev1.PodSucceeded,
	}
	if _, err := clients.Kube.CoreV1().Pods(taskRun.Namespace).UpdateStatus(pod); err != nil {
		t.Errorf("Unexpected error while updating build: %v", err)
	}
	if err := c.Reconciler.Reconcile(context.Background(), fmt.Sprintf("%s/%s", taskRun.Namespace, taskRun.Name)); err != nil {
		t.Fatalf("Unexpected error when Reconcile(): %v", err)
	}

	newTr, err = clients.Pipeline.TektonV1alpha1().TaskRuns(taskRun.Namespace).Get(taskRun.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Unexpected error fetching taskrun: %v", err)
	}
	if d := cmp.Diff(newTr.Status.GetCondition(apis.ConditionSucceeded), &apis.Condition{
		Type:   apis.ConditionSucceeded,
		Status: corev1.ConditionTrue,
	}, ignoreLastTransitionTime); d != "" {
		t.Errorf("Taskrun Status diff -got, +want: %v", d)
	}
}

func TestCreateRedirectedTaskSpec(t *testing.T) {
	tr := tb.TaskRun("tr", "tr", tb.TaskRunSpec(
		tb.TaskRunServiceAccount("sa"),
	))
	task := tb.Task("tr-ts", "tr", tb.TaskSpec(
		tb.Step("foo1", "bar1", tb.Command("abcd"), tb.Args("efgh")),
		tb.Step("foo2", "bar2", tb.Command("abcd"), tb.Args("efgh")),
		tb.TaskVolume("v"),
	))

	expectedSteps := len(task.Spec.Steps) + 1
	expectedVolumes := len(task.Spec.Volumes) + 1

	observer, _ := observer.New(zap.InfoLevel)
	entrypointCache, _ := entrypoint.NewCache()
	c := fakekubeclientset.NewSimpleClientset()
	ts, err := createRedirectedTaskSpec(c, &task.Spec, tr, entrypointCache, zap.New(observer).Sugar())
	if err != nil {
		t.Errorf("expected createRedirectedTaskSpec to pass: %v", err)
	}
	if len(ts.Steps) != expectedSteps {
		t.Errorf("step counts do not match: %d should be %d", len(ts.Steps), expectedSteps)
	}
	if len(ts.Volumes) != expectedVolumes {
		t.Errorf("volumes do not match: %d should be %d", len(ts.Volumes), expectedVolumes)
	}
}

func TestReconcileOnCompletedTaskRun(t *testing.T) {
	taskSt := &apis.Condition{
		Type:    apis.ConditionSucceeded,
		Status:  corev1.ConditionTrue,
		Reason:  "Build succeeded",
		Message: "Build succeeded",
	}
	taskRun := tb.TaskRun("test-taskrun-run-success", "foo", tb.TaskRunSpec(
		tb.TaskRunTaskRef(simpleTask.Name),
	), tb.TaskRunStatus(tb.Condition(*taskSt)))
	d := test.Data{
		TaskRuns: []*v1alpha1.TaskRun{
			taskRun,
		},
		Tasks: []*v1alpha1.Task{simpleTask},
	}

	testAssets := getTaskRunController(t, d)
	c := testAssets.Controller
	clients := testAssets.Clients

	if err := c.Reconciler.Reconcile(context.Background(), fmt.Sprintf("%s/%s", taskRun.Namespace, taskRun.Name)); err != nil {
		t.Fatalf("Unexpected error when reconciling completed TaskRun : %v", err)
	}
	newTr, err := clients.Pipeline.TektonV1alpha1().TaskRuns(taskRun.Namespace).Get(taskRun.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Expected completed TaskRun %s to exist but instead got error when getting it: %v", taskRun.Name, err)
	}
	if d := cmp.Diff(taskSt, newTr.Status.GetCondition(apis.ConditionSucceeded), ignoreLastTransitionTime); d != "" {
		t.Fatalf("-want, +got: %v", d)
	}
}

func TestReconcileOnCancelledTaskRun(t *testing.T) {
	taskRun := tb.TaskRun("test-taskrun-run-cancelled", "foo", tb.TaskRunSpec(
		tb.TaskRunTaskRef(simpleTask.Name),
		tb.TaskRunCancelled,
	), tb.TaskRunStatus(tb.Condition(apis.Condition{
		Type:   apis.ConditionSucceeded,
		Status: corev1.ConditionUnknown,
	})))
	d := test.Data{
		TaskRuns: []*v1alpha1.TaskRun{taskRun},
		Tasks:    []*v1alpha1.Task{simpleTask},
	}

	testAssets := getTaskRunController(t, d)
	c := testAssets.Controller
	clients := testAssets.Clients

	if err := c.Reconciler.Reconcile(context.Background(), fmt.Sprintf("%s/%s", taskRun.Namespace, taskRun.Name)); err != nil {
		t.Fatalf("Unexpected error when reconciling completed TaskRun : %v", err)
	}
	newTr, err := clients.Pipeline.TektonV1alpha1().TaskRuns(taskRun.Namespace).Get(taskRun.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Expected completed TaskRun %s to exist but instead got error when getting it: %v", taskRun.Name, err)
	}

	expectedStatus := &apis.Condition{
		Type:    apis.ConditionSucceeded,
		Status:  corev1.ConditionFalse,
		Reason:  "TaskRunCancelled",
		Message: `TaskRun "test-taskrun-run-cancelled" was cancelled`,
	}
	if d := cmp.Diff(expectedStatus, newTr.Status.GetCondition(apis.ConditionSucceeded), ignoreLastTransitionTime); d != "" {
		t.Fatalf("-want, +got: %v", d)
	}
}

func TestReconcileTimeouts(t *testing.T) {
	type testCase struct {
		taskRun        *v1alpha1.TaskRun
		expectedStatus *apis.Condition
	}

	testcases := []testCase{
		{
			taskRun: tb.TaskRun("test-taskrun-timeout", "foo",
				tb.TaskRunSpec(
					tb.TaskRunTaskRef(simpleTask.Name),
					tb.TaskRunTimeout(10*time.Second),
				),
				tb.TaskRunStatus(tb.Condition(apis.Condition{
					Type:   apis.ConditionSucceeded,
					Status: corev1.ConditionUnknown}),
					tb.TaskRunStartTime(time.Now().Add(-15*time.Second)))),

			expectedStatus: &apis.Condition{
				Type:    apis.ConditionSucceeded,
				Status:  corev1.ConditionFalse,
				Reason:  "TaskRunTimeout",
				Message: `TaskRun "test-taskrun-timeout" failed to finish within "10s"`,
			},
		},
		{
			taskRun: tb.TaskRun("test-taskrun-default-timeout-10-minutes", "foo",
				tb.TaskRunSpec(
					tb.TaskRunTaskRef(simpleTask.Name),
				),
				tb.TaskRunStatus(tb.Condition(apis.Condition{
					Type:   apis.ConditionSucceeded,
					Status: corev1.ConditionUnknown}),
					tb.TaskRunStartTime(time.Now().Add(-11*time.Minute)))),

			expectedStatus: &apis.Condition{
				Type:    apis.ConditionSucceeded,
				Status:  corev1.ConditionFalse,
				Reason:  "TaskRunTimeout",
				Message: `TaskRun "test-taskrun-default-timeout-10-minutes" failed to finish within "10m0s"`,
			},
		},
	}

	for _, tc := range testcases {
		d := test.Data{
			TaskRuns: []*v1alpha1.TaskRun{tc.taskRun},
			Tasks:    []*v1alpha1.Task{simpleTask},
		}
		testAssets := getTaskRunController(t, d)
		c := testAssets.Controller
		clients := testAssets.Clients

		if err := c.Reconciler.Reconcile(context.Background(), fmt.Sprintf("%s/%s", tc.taskRun.Namespace, tc.taskRun.Name)); err != nil {
			t.Fatalf("Unexpected error when reconciling completed TaskRun : %v", err)
		}
		newTr, err := clients.Pipeline.TektonV1alpha1().TaskRuns(tc.taskRun.Namespace).Get(tc.taskRun.Name, metav1.GetOptions{})
		if err != nil {
			t.Fatalf("Expected completed TaskRun %s to exist but instead got error when getting it: %v", tc.taskRun.Name, err)
		}
		condition := newTr.Status.GetCondition(apis.ConditionSucceeded)
		if d := cmp.Diff(tc.expectedStatus, condition, ignoreLastTransitionTime); d != "" {
			t.Fatalf("-want, +got: %v", d)
		}
	}
}

func TestUpdateStatusFromPod(t *testing.T) {
	conditionRunning := apis.Condition{
		Type:    apis.ConditionSucceeded,
		Status:  corev1.ConditionUnknown,
		Reason:  reasonRunning,
		Message: reasonRunning,
	}
	conditionTrue := apis.Condition{
		Type:   apis.ConditionSucceeded,
		Status: corev1.ConditionTrue,
	}
	conditionBuilding := apis.Condition{
		Type:   apis.ConditionSucceeded,
		Status: corev1.ConditionUnknown,
		Reason: "Building",
	}
	for _, c := range []struct {
		desc      string
		podStatus corev1.PodStatus
		want      v1alpha1.TaskRunStatus
	}{{
		desc:      "empty",
		podStatus: corev1.PodStatus{},

		want: v1alpha1.TaskRunStatus{
			Status: duckv1beta1.Status{
				Conditions: []apis.Condition{conditionRunning},
			},
			Steps: []v1alpha1.StepState{},
		},
	}, {
		desc: "ignore-creds-init",
		podStatus: corev1.PodStatus{
			InitContainerStatuses: []corev1.ContainerStatus{{
				// creds-init; ignored
			}},
			ContainerStatuses: []corev1.ContainerStatus{{
				Name: "state-name",
				State: corev1.ContainerState{
					Terminated: &corev1.ContainerStateTerminated{
						ExitCode: 123,
					},
				},
			}},
		},
		want: v1alpha1.TaskRunStatus{
			Status: duckv1beta1.Status{
				Conditions: []apis.Condition{conditionRunning},
			},
			Steps: []v1alpha1.StepState{{
				ContainerState: corev1.ContainerState{
					Terminated: &corev1.ContainerStateTerminated{
						ExitCode: 123,
					}},
				Name: "state-name",
			}},
		},
	}, {
		desc: "ignore-init-containers",
		podStatus: corev1.PodStatus{
			InitContainerStatuses: []corev1.ContainerStatus{{
				// creds-init; ignored.
			}, {
				// git-init; ignored.
			}},
			ContainerStatuses: []corev1.ContainerStatus{{
				Name: "state-name",
				State: corev1.ContainerState{
					Terminated: &corev1.ContainerStateTerminated{
						ExitCode: 123,
					},
				},
			}},
		},
		want: v1alpha1.TaskRunStatus{
			Status: duckv1beta1.Status{
				Conditions: []apis.Condition{conditionRunning},
			},
			Steps: []v1alpha1.StepState{{
				ContainerState: corev1.ContainerState{
					Terminated: &corev1.ContainerStateTerminated{
						ExitCode: 123,
					}},
				Name: "state-name",
			}},
		},
	}, {
		desc: "success",
		podStatus: corev1.PodStatus{
			Phase: corev1.PodSucceeded,
			ContainerStatuses: []corev1.ContainerStatus{{
				Name: "build-step-build-step-push",
				State: corev1.ContainerState{
					Terminated: &corev1.ContainerStateTerminated{
						ExitCode: 0,
					},
				},
			}},
		},
		want: v1alpha1.TaskRunStatus{
			Status: duckv1beta1.Status{
				Conditions: []apis.Condition{conditionTrue},
			},
			Steps: []v1alpha1.StepState{{
				ContainerState: corev1.ContainerState{
					Terminated: &corev1.ContainerStateTerminated{
						ExitCode: 0,
					}},
				Name: "build-step-push",
			}},
			// We don't actually care about the time, just that it's not nil
			CompletionTime: &metav1.Time{Time: time.Now()},
		},
	}, {
		desc: "running",
		podStatus: corev1.PodStatus{
			Phase: corev1.PodRunning,
			ContainerStatuses: []corev1.ContainerStatus{{
				Name: "build-step-running-step",
				State: corev1.ContainerState{
					Running: &corev1.ContainerStateRunning{},
				},
			}},
		},
		want: v1alpha1.TaskRunStatus{
			Status: duckv1beta1.Status{
				Conditions: []apis.Condition{conditionBuilding},
			},
			Steps: []v1alpha1.StepState{{
				ContainerState: corev1.ContainerState{
					Running: &corev1.ContainerStateRunning{},
				},
				Name: "running-step",
			}},
		},
	}, {
		desc: "failure-terminated",
		podStatus: corev1.PodStatus{
			Phase:                 corev1.PodFailed,
			InitContainerStatuses: []corev1.ContainerStatus{{
				// creds-init status; ignored
			}},
			ContainerStatuses: []corev1.ContainerStatus{{
				Name:    "build-step-failure",
				ImageID: "image-id",
				State: corev1.ContainerState{
					Terminated: &corev1.ContainerStateTerminated{
						ExitCode: 123,
					},
				},
			}},
		},
		want: v1alpha1.TaskRunStatus{
			Status: duckv1beta1.Status{
				Conditions: []apis.Condition{{
					Type:    apis.ConditionSucceeded,
					Status:  corev1.ConditionFalse,
					Message: `"build-step-failure" exited with code 123 (image: "image-id"); for logs run: kubectl -n foo logs pod -c build-step-failure`,
				}},
			},
			Steps: []v1alpha1.StepState{{
				ContainerState: corev1.ContainerState{
					Terminated: &corev1.ContainerStateTerminated{
						ExitCode: 123,
					}},
				Name: "failure",
			}},
			// We don't actually care about the time, just that it's not nil
			CompletionTime: &metav1.Time{Time: time.Now()},
		},
	}, {
		desc: "failure-message",
		podStatus: corev1.PodStatus{
			Phase:   corev1.PodFailed,
			Message: "boom",
		},
		want: v1alpha1.TaskRunStatus{
			Status: duckv1beta1.Status{
				Conditions: []apis.Condition{{
					Type:    apis.ConditionSucceeded,
					Status:  corev1.ConditionFalse,
					Message: "boom",
				}},
			},
			Steps: []v1alpha1.StepState{},
			// We don't actually care about the time, just that it's not nil
			CompletionTime: &metav1.Time{Time: time.Now()},
		},
	}, {
		desc:      "failure-unspecified",
		podStatus: corev1.PodStatus{Phase: corev1.PodFailed},
		want: v1alpha1.TaskRunStatus{
			Status: duckv1beta1.Status{
				Conditions: []apis.Condition{{
					Type:    apis.ConditionSucceeded,
					Status:  corev1.ConditionFalse,
					Message: "build failed for unspecified reasons.",
				}},
			},
			Steps: []v1alpha1.StepState{},
			// We don't actually care about the time, just that it's not nil
			CompletionTime: &metav1.Time{Time: time.Now()},
		},
	}, {
		desc: "pending-waiting-message",
		podStatus: corev1.PodStatus{
			Phase:                 corev1.PodPending,
			InitContainerStatuses: []corev1.ContainerStatus{{
				// creds-init status; ignored
			}},
			ContainerStatuses: []corev1.ContainerStatus{{
				Name: "status-name",
				State: corev1.ContainerState{
					Waiting: &corev1.ContainerStateWaiting{
						Message: "i'm pending",
					},
				},
			}},
		},
		want: v1alpha1.TaskRunStatus{
			Status: duckv1beta1.Status{
				Conditions: []apis.Condition{{
					Type:    apis.ConditionSucceeded,
					Status:  corev1.ConditionUnknown,
					Reason:  "Pending",
					Message: `build step "status-name" is pending with reason "i'm pending"`,
				}},
			},
			Steps: []v1alpha1.StepState{{
				ContainerState: corev1.ContainerState{
					Waiting: &corev1.ContainerStateWaiting{
						Message: "i'm pending",
					},
				},
				Name: "status-name",
			}},
		},
	}, {
		desc: "pending-pod-condition",
		podStatus: corev1.PodStatus{
			Phase: corev1.PodPending,
			Conditions: []corev1.PodCondition{{
				Status:  corev1.ConditionUnknown,
				Type:    "the type",
				Message: "the message",
			}},
		},
		want: v1alpha1.TaskRunStatus{
			Status: duckv1beta1.Status{
				Conditions: []apis.Condition{{
					Type:    apis.ConditionSucceeded,
					Status:  corev1.ConditionUnknown,
					Reason:  "Pending",
					Message: `pod status "the type":"Unknown"; message: "the message"`,
				}},
			},
			Steps: []v1alpha1.StepState{},
		},
	}, {
		desc: "pending-message",
		podStatus: corev1.PodStatus{
			Phase:   corev1.PodPending,
			Message: "pod status message",
		},
		want: v1alpha1.TaskRunStatus{
			Status: duckv1beta1.Status{
				Conditions: []apis.Condition{{
					Type:    apis.ConditionSucceeded,
					Status:  corev1.ConditionUnknown,
					Reason:  "Pending",
					Message: "pod status message",
				}},
			},
			Steps: []v1alpha1.StepState{},
		},
	}, {
		desc:      "pending-no-message",
		podStatus: corev1.PodStatus{Phase: corev1.PodPending},
		want: v1alpha1.TaskRunStatus{
			Status: duckv1beta1.Status{
				Conditions: []apis.Condition{{
					Type:    apis.ConditionSucceeded,
					Status:  corev1.ConditionUnknown,
					Reason:  "Pending",
					Message: "Pending",
				}},
			},
			Steps: []v1alpha1.StepState{},
		},
	}, {
		desc: "pending-not-enough-node-resources",
		podStatus: corev1.PodStatus{
			Phase: corev1.PodPending,
			Conditions: []corev1.PodCondition{{
				Reason:  corev1.PodReasonUnschedulable,
				Message: "0/1 nodes are available: 1 Insufficient cpu.",
			}},
		},
		want: v1alpha1.TaskRunStatus{
			Status: duckv1beta1.Status{
				Conditions: []apis.Condition{{
					Type:    apis.ConditionSucceeded,
					Status:  corev1.ConditionUnknown,
					Reason:  reasonExceededNodeResources,
					Message: `TaskRun pod "taskRun" exceeded available resources`,
				}},
			},
			Steps: []v1alpha1.StepState{},
		},
	}} {
		t.Run(c.desc, func(t *testing.T) {
			observer, _ := observer.New(zap.InfoLevel)
			logger := zap.New(observer).Sugar()
			fakeClient := fakeclientset.NewSimpleClientset()
			sharedInfomer := informers.NewSharedInformerFactory(fakeClient, 0)
			pipelineResourceInformer := sharedInfomer.Tekton().V1alpha1().PipelineResources()
			resourceLister := pipelineResourceInformer.Lister()
			fakekubeclient := fakekubeclientset.NewSimpleClientset()

			rs := []*v1alpha1.PipelineResource{{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "source-image",
					Namespace: "marshmallow",
				},
				Spec: v1alpha1.PipelineResourceSpec{
					Type: "image",
				},
			}}

			for _, r := range rs {
				err := pipelineResourceInformer.Informer().GetIndexer().Add(r)
				if err != nil {
					t.Errorf("pipelineResourceInformer.Informer().GetIndexer().Add(r) failed with err: %s", err)
				}
			}

			now := metav1.Now()
			p := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "pod",
					Namespace:         "foo",
					CreationTimestamp: now,
				},
				Status: c.podStatus,
			}
			startTime := time.Date(2010, 1, 1, 1, 1, 1, 1, time.UTC)
			tr := tb.TaskRun("taskRun", "foo", tb.TaskRunStatus(tb.TaskRunStartTime(startTime)))
			updateStatusFromPod(tr, p, resourceLister, fakekubeclient, logger)

			// Common traits, set for test case brevity.
			c.want.PodName = "pod"
			c.want.StartTime = &metav1.Time{Time: startTime}

			ensureTimeNotNil := cmp.Comparer(func(x, y *metav1.Time) bool {
				if x == nil {
					return y == nil
				}
				return y != nil
			})
			if d := cmp.Diff(tr.Status, c.want, ignoreVolatileTime, ensureTimeNotNil); d != "" {
				t.Errorf("Diff:\n%s", d)
			}
			if tr.Status.StartTime.Time != c.want.StartTime.Time {
				t.Errorf("Expected TaskRun startTime to be unchanged but was %s", tr.Status.StartTime)
			}
		})
	}
}
