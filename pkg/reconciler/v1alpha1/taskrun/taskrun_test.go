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
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/knative/pkg/apis"
	"github.com/knative/pkg/configmap"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/logging"
	"github.com/tektoncd/pipeline/pkg/reconciler"
	"github.com/tektoncd/pipeline/pkg/reconciler/v1alpha1/taskrun/entrypoint"
	"github.com/tektoncd/pipeline/pkg/reconciler/v1alpha1/taskrun/resources"
	"github.com/tektoncd/pipeline/pkg/status"
	"github.com/tektoncd/pipeline/pkg/system"
	"github.com/tektoncd/pipeline/test"
	tb "github.com/tektoncd/pipeline/test/builder"
	"github.com/tektoncd/pipeline/test/names"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
	"golang.org/x/xerrors"
	corev1 "k8s.io/api/core/v1"
	k8sapierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8sruntimeschema "k8s.io/apimachinery/pkg/runtime/schema"
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
	// Pods are created with a random 3-byte (6 hex character) suffix that we want to ignore in our diffs.
	ignoreRandomPodNameSuffix = cmp.FilterPath(func(path cmp.Path) bool {
		return path.GoString() == "{v1.ObjectMeta}.Name"
	}, cmp.Comparer(func(name1, name2 string) bool {
		return name1[:len(name1)-6] == name2[:len(name2)-6]
	}))
	resourceQuantityCmp = cmp.Comparer(func(x, y resource.Quantity) bool {
		return x.Cmp(y) == 0
	})

	simpleStep        = tb.Step("simple-step", "foo", tb.Command("/mycmd"))
	simpleTask        = tb.Task("test-task", "foo", tb.TaskSpec(simpleStep))
	taskMultipleSteps = tb.Task("test-task-multi-steps", "foo", tb.TaskSpec(
		tb.Step("z-step", "foo",
			tb.Command("/mycmd"),
		),
		tb.Step("v-step", "foo",
			tb.Command("/mycmd"),
		),
		tb.Step("x-step", "foo",
			tb.Command("/mycmd"),
		),
	))
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
		// The ContainerTemplate field is deprecated (#977)
		tb.TaskContainerTemplate(
			tb.EnvVar("BREAD", "PUMPERNICKEL"),
		),
		tb.TaskStepTemplate(
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
	downward = corev1.Volume{
		Name: "downward",
		VolumeSource: corev1.VolumeSource{
			DownwardAPI: &corev1.DownwardAPIVolumeSource{
				Items: []corev1.DownwardAPIVolumeFile{
					{
						Path: "ready",
						FieldRef: &corev1.ObjectFieldSelector{
							FieldPath: "metadata.annotations['tekton.dev/ready']",
						},
					},
				},
			},
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

		return tb.PodInitContainer("step-credential-initializer-"+suffix, "override-with-creds:latest", actualOps...)
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

		return tb.PodInitContainer("step-place-tools", "override-with-entrypoint:latest", actualOps...)
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

	th := reconciler.NewTimeoutHandler(stopCh, logger)
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
			tb.PodAnnotation("tekton.dev/ready", ""),
			tb.PodLabel(taskNameLabelKey, "test-task"),
			tb.PodLabel(taskRunNameLabelKey, "test-taskrun-run-success"),
			tb.PodOwnerReference("TaskRun", "test-taskrun-run-success",
				tb.OwnerReferenceAPIVersion(currentApiVersion)),
			tb.PodSpec(
				tb.PodVolumes(toolsVolume, downward, workspaceVolume, homeVolume),
				tb.PodRestartPolicy(corev1.RestartPolicyNever),
				getCredentialsInitContainer("9l9zj"),
				getPlaceToolsInitContainer(),
				tb.PodContainer("step-simple-step", "foo",
					tb.Command(entrypointLocation),
					tb.Args("-wait_file", "/builder/downward/ready", "-post_file", "/builder/tools/0", "-wait_file_content", "-entrypoint", "/mycmd", "--"),
					tb.WorkingDir(workspaceDir),
					tb.EnvVar("HOME", "/builder/home"),
					tb.VolumeMount("tools", "/builder/tools"),
					tb.VolumeMount("downward", "/builder/downward"),
					tb.VolumeMount("workspace", workspaceDir),
					tb.VolumeMount("home", "/builder/home"),
					tb.Resources(tb.Requests(
						tb.CPU("0"),
						tb.Memory("0"),
						tb.EphemeralStorage("0"),
					)),
				),
			),
		),
	}, {
		name:    "serviceaccount",
		taskRun: taskRunWithSaSuccess,
		wantPod: tb.Pod("test-taskrun-with-sa-run-success-pod-123456", "foo",
			tb.PodAnnotation("tekton.dev/ready", ""),
			tb.PodLabel(taskNameLabelKey, "test-with-sa"),
			tb.PodLabel(taskRunNameLabelKey, "test-taskrun-with-sa-run-success"),
			tb.PodOwnerReference("TaskRun", "test-taskrun-with-sa-run-success",
				tb.OwnerReferenceAPIVersion(currentApiVersion)),
			tb.PodSpec(
				tb.PodServiceAccountName("test-sa"),
				tb.PodVolumes(toolsVolume, downward, workspaceVolume, homeVolume),
				tb.PodRestartPolicy(corev1.RestartPolicyNever),
				getCredentialsInitContainer("9l9zj"),
				getPlaceToolsInitContainer(),
				tb.PodContainer("step-sa-step", "foo",
					tb.Command(entrypointLocation),
					tb.Args("-wait_file", "/builder/downward/ready", "-post_file", "/builder/tools/0", "-wait_file_content", "-entrypoint", "/mycmd", "--"),
					tb.WorkingDir(workspaceDir),
					tb.EnvVar("HOME", "/builder/home"),
					tb.VolumeMount("tools", "/builder/tools"),
					tb.VolumeMount("downward", "/builder/downward"),
					tb.VolumeMount("workspace", workspaceDir),
					tb.VolumeMount("home", "/builder/home"),
					tb.Resources(tb.Requests(
						tb.CPU("0"),
						tb.Memory("0"),
						tb.EphemeralStorage("0"),
					)),
				),
			),
		),
	}, {
		name:    "params",
		taskRun: taskRunTemplating,
		wantPod: tb.Pod("test-taskrun-templating-pod-123456", "foo",
			tb.PodAnnotation("tekton.dev/ready", ""),
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
				}, toolsVolume, downward, workspaceVolume, homeVolume),
				tb.PodRestartPolicy(corev1.RestartPolicyNever),
				getCredentialsInitContainer("78c5n"),
				getPlaceToolsInitContainer(),
				tb.PodContainer("step-git-source-git-resource-mssqb", "override-with-git:latest",
					tb.Command(entrypointLocation),
					tb.Args("-wait_file", "/builder/downward/ready", "-post_file", "/builder/tools/0", "-wait_file_content", "-entrypoint", "/ko-app/git-init", "--",
						"-url", "https://foo.git", "-revision", "master", "-path", "/workspace/workspace"),
					tb.WorkingDir(workspaceDir),
					tb.EnvVar("HOME", "/builder/home"),
					tb.VolumeMount("tools", "/builder/tools"),
					tb.VolumeMount("downward", "/builder/downward"),
					tb.VolumeMount("workspace", workspaceDir),
					tb.VolumeMount("home", "/builder/home"),
					tb.Resources(tb.Requests(
						tb.CPU("0"),
						tb.Memory("0"),
						tb.EphemeralStorage("0"),
					)),
				),
				tb.PodContainer("step-mycontainer", "myimage",
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
				tb.PodContainer("step-image-digest-exporter-mycontainer-9l9zj", "override-with-imagedigest-exporter-image:latest",
					tb.Command("/builder/tools/entrypoint"),
					tb.Args("-wait_file", "/builder/tools/1", "-post_file", "/builder/tools/2", "-entrypoint", "/ko-app/imagedigestexporter", "--",
						"-images", "[{\"name\":\"image-resource\",\"type\":\"image\",\"url\":\"gcr.io/kristoff/sven\",\"digest\":\"\",\"OutputImageDir\":\"\"}]",
						"-terminationMessagePath", "/builder/home/image-outputs/termination-log"),
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
					tb.TerminationMessagePath("/builder/home/image-outputs/termination-log"),
					tb.TerminationMessagePolicy(corev1.TerminationMessageFallbackToLogsOnError),
				),
				tb.PodContainer("step-myothercontainer", "myotherimage",
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
				tb.PodContainer("step-image-digest-exporter-myothercontainer-mz4c7", "override-with-imagedigest-exporter-image:latest",
					tb.Command(entrypointLocation),
					tb.Args("-wait_file", "/builder/tools/3", "-post_file", "/builder/tools/4", "-entrypoint", "/ko-app/imagedigestexporter", "--",
						"-images", "[{\"name\":\"image-resource\",\"type\":\"image\",\"url\":\"gcr.io/kristoff/sven\",\"digest\":\"\",\"OutputImageDir\":\"\"}]",
						"-terminationMessagePath", "/builder/home/image-outputs/termination-log"),
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
					tb.TerminationMessagePath("/builder/home/image-outputs/termination-log"),
					tb.TerminationMessagePolicy(corev1.TerminationMessageFallbackToLogsOnError),
				),
			),
		),
	}, {
		name:    "wrap-steps",
		taskRun: taskRunInputOutput,
		wantPod: tb.Pod("test-taskrun-input-output-pod-123456", "foo",
			tb.PodAnnotation("tekton.dev/ready", ""),
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
				}, toolsVolume, downward, workspaceVolume, homeVolume),
				tb.PodRestartPolicy(corev1.RestartPolicyNever),
				getCredentialsInitContainer("vr6ds"),
				getPlaceToolsInitContainer(),
				tb.PodContainer("step-create-dir-another-git-resource-78c5n", "override-with-bash-noop:latest",
					tb.Command(entrypointLocation),
					tb.Args("-wait_file", "/builder/downward/ready", "-post_file", "/builder/tools/0", "-wait_file_content", "-entrypoint", "/ko-app/bash", "--",
						"-args", "mkdir -p /workspace/another-git-resource"),
					tb.WorkingDir(workspaceDir),
					tb.EnvVar("HOME", "/builder/home"),
					tb.VolumeMount("tools", "/builder/tools"),
					tb.VolumeMount("downward", "/builder/downward"),
					tb.VolumeMount("workspace", workspaceDir),
					tb.VolumeMount("home", "/builder/home"),
					tb.Resources(tb.Requests(
						tb.CPU("0"),
						tb.Memory("0"),
						tb.EphemeralStorage("0"),
					)),
				),
				tb.PodContainer("step-source-copy-another-git-resource-mssqb", "override-with-bash-noop:latest",
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
				tb.PodContainer("step-create-dir-git-resource-mz4c7", "override-with-bash-noop:latest",
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
				tb.PodContainer("step-source-copy-git-resource-9l9zj", "override-with-bash-noop:latest",
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
				tb.PodContainer("step-simple-step", "foo",
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
				tb.PodContainer("step-source-mkdir-git-resource-6nl7g", "override-with-bash-noop:latest",
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
				tb.PodContainer("step-source-copy-git-resource-j2tds", "override-with-bash-noop:latest",
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
			),
		),
	}, {
		name:    "taskrun-with-taskspec",
		taskRun: taskRunWithTaskSpec,
		wantPod: tb.Pod("test-taskrun-with-taskspec-pod-123456", "foo",
			tb.PodAnnotation("tekton.dev/ready", ""),
			tb.PodLabel(taskRunNameLabelKey, "test-taskrun-with-taskspec"),
			tb.PodOwnerReference("TaskRun", "test-taskrun-with-taskspec",
				tb.OwnerReferenceAPIVersion(currentApiVersion)),
			tb.PodSpec(
				tb.PodVolumes(toolsVolume, downward, workspaceVolume, homeVolume),
				tb.PodRestartPolicy(corev1.RestartPolicyNever),
				getCredentialsInitContainer("mz4c7"),
				getPlaceToolsInitContainer(),
				tb.PodContainer("step-git-source-git-resource-9l9zj", "override-with-git:latest",
					tb.Command(entrypointLocation),
					tb.Args("-wait_file", "/builder/downward/ready", "-post_file", "/builder/tools/0", "-wait_file_content", "-entrypoint", "/ko-app/git-init", "--",
						"-url", "https://foo.git", "-revision", "master", "-path", "/workspace/workspace"),
					tb.WorkingDir(workspaceDir),
					tb.EnvVar("HOME", "/builder/home"),
					tb.VolumeMount("tools", "/builder/tools"),
					tb.VolumeMount("downward", "/builder/downward"),
					tb.VolumeMount("workspace", workspaceDir),
					tb.VolumeMount("home", "/builder/home"),
					tb.Resources(tb.Requests(
						tb.CPU("0"),
						tb.Memory("0"),
						tb.EphemeralStorage("0"),
					)),
				),
				tb.PodContainer("step-mycontainer", "myimage",
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
			),
		),
	}, {
		name:    "success-with-cluster-task",
		taskRun: taskRunWithClusterTask,
		wantPod: tb.Pod("test-taskrun-with-cluster-task-pod-123456", "foo",
			tb.PodAnnotation("tekton.dev/ready", ""),
			tb.PodLabel(taskNameLabelKey, "test-cluster-task"),
			tb.PodLabel(taskRunNameLabelKey, "test-taskrun-with-cluster-task"),
			tb.PodOwnerReference("TaskRun", "test-taskrun-with-cluster-task",
				tb.OwnerReferenceAPIVersion(currentApiVersion)),
			tb.PodSpec(
				tb.PodVolumes(toolsVolume, downward, workspaceVolume, homeVolume),
				tb.PodRestartPolicy(corev1.RestartPolicyNever),
				getCredentialsInitContainer("9l9zj"),
				getPlaceToolsInitContainer(),
				tb.PodContainer("step-simple-step", "foo",
					tb.Command(entrypointLocation),
					tb.Args("-wait_file", "/builder/downward/ready", "-post_file", "/builder/tools/0", "-wait_file_content", "-entrypoint", "/mycmd", "--"),
					tb.WorkingDir(workspaceDir),
					tb.EnvVar("HOME", "/builder/home"),
					tb.VolumeMount("tools", "/builder/tools"),
					tb.VolumeMount("downward", "/builder/downward"),
					tb.VolumeMount("workspace", workspaceDir),
					tb.VolumeMount("home", "/builder/home"),
					tb.Resources(tb.Requests(
						tb.CPU("0"),
						tb.Memory("0"),
						tb.EphemeralStorage("0"),
					)),
				),
			),
		),
	}, {
		name:    "taskrun-with-resource-spec-task-spec",
		taskRun: taskRunWithResourceSpecAndTaskSpec,
		wantPod: tb.Pod("test-taskrun-with-resource-spec-pod-123456", "foo",
			tb.PodAnnotation("tekton.dev/ready", ""),
			tb.PodLabel(taskRunNameLabelKey, "test-taskrun-with-resource-spec"),
			tb.PodOwnerReference("TaskRun", "test-taskrun-with-resource-spec",
				tb.OwnerReferenceAPIVersion(currentApiVersion)),
			tb.PodSpec(
				tb.PodVolumes(toolsVolume, downward, workspaceVolume, homeVolume),
				tb.PodRestartPolicy(corev1.RestartPolicyNever),
				getCredentialsInitContainer("mz4c7"),
				getPlaceToolsInitContainer(),
				tb.PodContainer("step-git-source-workspace-9l9zj", "override-with-git:latest",
					tb.Command(entrypointLocation),
					tb.Args("-wait_file", "/builder/downward/ready", "-post_file", "/builder/tools/0", "-wait_file_content", "-entrypoint", "/ko-app/git-init", "--",
						"-url", "github.com/foo/bar.git", "-revision", "rel-can", "-path",
						"/workspace/workspace"),
					tb.WorkingDir(workspaceDir),
					tb.EnvVar("HOME", "/builder/home"),
					tb.VolumeMount("tools", "/builder/tools"),
					tb.VolumeMount("downward", "/builder/downward"),
					tb.VolumeMount("workspace", workspaceDir),
					tb.VolumeMount("home", "/builder/home"),
					tb.Resources(tb.Requests(
						tb.CPU("0"),
						tb.Memory("0"),
						tb.EphemeralStorage("0"),
					)),
				),
				tb.PodContainer("step-mystep", "ubuntu",
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
			),
		),
	}, {
		name:    "taskrun-with-labels",
		taskRun: taskRunWithLabels,
		wantPod: tb.Pod("test-taskrun-with-labels-pod-123456", "foo",
			tb.PodAnnotation("tekton.dev/ready", ""),
			tb.PodLabel("TaskRunLabel", "TaskRunValue"),
			tb.PodLabel(taskNameLabelKey, "test-task"),
			tb.PodLabel(taskRunNameLabelKey, "test-taskrun-with-labels"),
			tb.PodOwnerReference("TaskRun", "test-taskrun-with-labels",
				tb.OwnerReferenceAPIVersion(currentApiVersion)),
			tb.PodSpec(
				tb.PodVolumes(toolsVolume, downward, workspaceVolume, homeVolume),
				tb.PodRestartPolicy(corev1.RestartPolicyNever),
				getCredentialsInitContainer("9l9zj"),
				getPlaceToolsInitContainer(),
				tb.PodContainer("step-simple-step", "foo",
					tb.Command(entrypointLocation),
					tb.Args("-wait_file", "/builder/downward/ready", "-post_file", "/builder/tools/0", "-wait_file_content", "-entrypoint", "/mycmd", "--"),
					tb.WorkingDir(workspaceDir),
					tb.EnvVar("HOME", "/builder/home"),
					tb.VolumeMount("tools", "/builder/tools"),
					tb.VolumeMount("downward", "/builder/downward"),
					tb.VolumeMount("workspace", workspaceDir),
					tb.VolumeMount("home", "/builder/home"),
					tb.Resources(tb.Requests(
						tb.CPU("0"),
						tb.Memory("0"),
						tb.EphemeralStorage("0"),
					)),
				),
			),
		),
	}, {
		name:    "taskrun-with-annotations",
		taskRun: taskRunWithAnnotations,
		wantPod: tb.Pod("test-taskrun-with-annotations-pod-123456", "foo",
			tb.PodAnnotation("tekton.dev/ready", ""),
			tb.PodAnnotation("TaskRunAnnotation", "TaskRunValue"),
			tb.PodLabel(taskNameLabelKey, "test-task"),
			tb.PodLabel(taskRunNameLabelKey, "test-taskrun-with-annotations"),
			tb.PodOwnerReference("TaskRun", "test-taskrun-with-annotations",
				tb.OwnerReferenceAPIVersion(currentApiVersion)),
			tb.PodSpec(
				tb.PodVolumes(toolsVolume, downward, workspaceVolume, homeVolume),
				tb.PodRestartPolicy(corev1.RestartPolicyNever),
				getCredentialsInitContainer("9l9zj"),
				getPlaceToolsInitContainer(),
				tb.PodContainer("step-simple-step", "foo",
					tb.Command(entrypointLocation),
					tb.Args("-wait_file", "/builder/downward/ready", "-post_file", "/builder/tools/0", "-wait_file_content", "-entrypoint", "/mycmd", "--"),
					tb.WorkingDir(workspaceDir),
					tb.EnvVar("HOME", "/builder/home"),
					tb.VolumeMount("tools", "/builder/tools"),
					tb.VolumeMount("downward", "/builder/downward"),
					tb.VolumeMount("workspace", workspaceDir),
					tb.VolumeMount("home", "/builder/home"),
					tb.Resources(tb.Requests(
						tb.CPU("0"),
						tb.Memory("0"),
						tb.EphemeralStorage("0"),
					)),
				),
			),
		),
	}, {
		name:    "task-env",
		taskRun: taskRunTaskEnv,
		wantPod: tb.Pod("test-taskrun-task-env-pod-311bc9", "foo",
			tb.PodAnnotation("tekton.dev/ready", ""),
			tb.PodLabel(taskNameLabelKey, "test-task-env"),
			tb.PodLabel(taskRunNameLabelKey, "test-taskrun-task-env"),
			tb.PodOwnerReference("TaskRun", "test-taskrun-task-env",
				tb.OwnerReferenceAPIVersion("tekton.dev/v1alpha1")),
			tb.PodSpec(
				tb.PodVolumes(toolsVolume, downward, workspaceVolume, homeVolume),
				tb.PodRestartPolicy(corev1.RestartPolicyNever),
				getCredentialsInitContainer("9l9zj", tb.EnvVar("FRUIT", "APPLE"), tb.EnvVar("BREAD", "PUMPERNICKEL")),
				getPlaceToolsInitContainer(tb.EnvVar("FRUIT", "APPLE"), tb.EnvVar("BREAD", "PUMPERNICKEL")),
				tb.PodContainer("step-env-step", "foo", tb.Command(entrypointLocation),
					tb.Command(entrypointLocation),
					tb.Args("-wait_file", "/builder/downward/ready", "-post_file", "/builder/tools/0", "-wait_file_content", "-entrypoint", "/mycmd", "--"),
					tb.WorkingDir(workspaceDir),
					tb.EnvVar("HOME", "/builder/home"),
					tb.Resources(tb.Requests(
						tb.CPU("0"),
						tb.Memory("0"),
						tb.EphemeralStorage("0"),
					)),
					tb.VolumeMount("tools", "/builder/tools"),
					tb.VolumeMount("downward", "/builder/downward"),
					tb.VolumeMount("workspace", workspaceDir),
					tb.VolumeMount("home", "/builder/home"),
					tb.EnvVar("ANOTHER", "VARIABLE"),
					tb.EnvVar("FRUIT", "LEMON"),
					tb.EnvVar("BREAD", "PUMPERNICKEL"),
				),
			),
		),
	}, {
		name:    "taskrun-with-resource-requests",
		taskRun: taskRunWithResourceRequests,
		wantPod: tb.Pod("test-taskrun-with-resource-requests-pod-123456", "foo",
			tb.PodAnnotation("tekton.dev/ready", ""),
			tb.PodLabel(taskRunNameLabelKey, "test-taskrun-with-resource-requests"),
			tb.PodOwnerReference("TaskRun", "test-taskrun-with-resource-requests",
				tb.OwnerReferenceAPIVersion(currentApiVersion)),
			tb.PodSpec(
				tb.PodVolumes(toolsVolume, downward, workspaceVolume, homeVolume),
				tb.PodRestartPolicy(corev1.RestartPolicyNever),
				getCredentialsInitContainer("9l9zj"),
				getPlaceToolsInitContainer(),
				tb.PodContainer("step-step1", "foo",
					tb.Command(entrypointLocation),
					tb.Args("-wait_file", "/builder/downward/ready", "-post_file", "/builder/tools/0", "-wait_file_content", "-entrypoint", "/mycmd", "--"),
					tb.WorkingDir(workspaceDir),
					tb.EnvVar("HOME", "/builder/home"),
					tb.VolumeMount("tools", "/builder/tools"),
					tb.VolumeMount("downward", "/builder/downward"),
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
				tb.PodContainer("step-step2", "foo",
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
				tb.PodContainer("step-step3", "foo",
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
			),
		),
	}, {
		name:    "taskrun-with-pod",
		taskRun: taskRunWithPod,
		wantPod: tb.Pod("test-taskrun-with-pod-pod-123456", "foo",
			tb.PodAnnotation("tekton.dev/ready", ""),
			tb.PodLabel(taskNameLabelKey, "test-task"),
			tb.PodLabel(taskRunNameLabelKey, "test-taskrun-with-pod"),
			tb.PodOwnerReference("TaskRun", "test-taskrun-with-pod",
				tb.OwnerReferenceAPIVersion(currentApiVersion)),
			tb.PodSpec(
				tb.PodVolumes(toolsVolume, downward, workspaceVolume, homeVolume),
				tb.PodRestartPolicy(corev1.RestartPolicyNever),
				getCredentialsInitContainer("9l9zj"),
				getPlaceToolsInitContainer(),
				tb.PodContainer("step-simple-step", "foo",
					tb.Command(entrypointLocation),
					tb.Args("-wait_file", "/builder/downward/ready", "-post_file", "/builder/tools/0", "-wait_file_content", "-entrypoint", "/mycmd", "--"),
					tb.WorkingDir(workspaceDir),
					tb.EnvVar("HOME", "/builder/home"),
					tb.VolumeMount("tools", "/builder/tools"),
					tb.VolumeMount("downward", "/builder/downward"),
					tb.VolumeMount("workspace", workspaceDir),
					tb.VolumeMount("home", "/builder/home"),
					tb.Resources(tb.Requests(
						tb.CPU("0"),
						tb.Memory("0"),
						tb.EphemeralStorage("0"),
					)),
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
			if condition != nil && condition.Reason != status.ReasonRunning {
				t.Errorf("Expected reason %q but was %s", status.ReasonRunning, condition.Reason)
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

func TestReconcile_SortTaskRunStatusSteps(t *testing.T) {
	taskRun := tb.TaskRun("test-taskrun", "foo", tb.TaskRunSpec(
		tb.TaskRunTaskRef(taskMultipleSteps.Name)),
		tb.TaskRunStatus(
			tb.PodName("the-pod"),
		),
	)

	// The order of the container statuses has been shuffled, not aligning with the order of the
	// spec steps of the Task any more. After Reconcile is called, we should see the order of status
	// steps in TaksRun has been converted to the same one as in spec steps of the Task.
	d := test.Data{
		TaskRuns: []*v1alpha1.TaskRun{taskRun},
		Tasks:    []*v1alpha1.Task{taskMultipleSteps},
		Pods: []*corev1.Pod{{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "foo",
				Name:      "the-pod",
			},
			Status: corev1.PodStatus{
				Phase: corev1.PodSucceeded,
				ContainerStatuses: []corev1.ContainerStatus{{
					Name: "step-nop",
					State: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{
							ExitCode: 0,
						},
					},
				}, {
					Name: "step-x-step",
					State: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{
							ExitCode: 0,
						},
					},
				}, {
					Name: "step-v-step",
					State: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{
							ExitCode: 0,
						},
					},
				}, {
					Name: "step-z-step",
					State: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{
							ExitCode: 0,
						},
					},
				}},
			},
		}},
	}
	testAssets := getTaskRunController(t, d)
	if err := testAssets.Controller.Reconciler.Reconcile(context.Background(), getRunName(taskRun)); err != nil {
		t.Errorf("expected no error reconciling valid TaskRun but got %v", err)
	}
	verify_TaskRunStatusStep(t, taskRun, taskMultipleSteps)
}

func verify_TaskRunStatusStep(t *testing.T, taskRun *v1alpha1.TaskRun, task *v1alpha1.Task) {
	actualStepOrder := []string{}
	for _, state := range taskRun.Status.Steps {
		actualStepOrder = append(actualStepOrder, state.Name)
	}
	expectedStepOrder := []string{}
	for _, state := range taskMultipleSteps.Spec.Steps {
		expectedStepOrder = append(expectedStepOrder, state.Name)
	}
	// Add a nop in the end. This may be removed in future.
	expectedStepOrder = append(expectedStepOrder, "nop")
	if d := cmp.Diff(actualStepOrder, expectedStepOrder); d != "" {
		t.Errorf("The status steps in TaksRun doesn't match the spec steps in Task, diff: %s", d)
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
			reason:  status.ReasonFailedResolution,
		},
		{
			name:    "task run with no task",
			taskRun: withWrongRef,
			reason:  status.ReasonFailedResolution,
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
			return true, nil, xerrors.New("induce failure fetching pods")
		}
		return false, nil, nil
	})

	if err := c.Reconciler.Reconcile(context.Background(), fmt.Sprintf("%s/%s", taskRun.Namespace, taskRun.Name)); err == nil {
		t.Fatal("expected error when reconciling a Task for which we couldn't get the corresponding Build Pod but got nil")
	}
}

func makePod(taskRun *v1alpha1.TaskRun, task *v1alpha1.Task) (*corev1.Pod, error) {
	logger, _ := logging.NewLogger("", "")
	cache, _ := entrypoint.NewCache()
	// TODO(jasonhall): This avoids a circular dependency where
	// getTaskRunController takes a test.Data which must be populated with
	// a pod created from MakePod which requires a (fake) Kube client. When
	// we remove Build entirely from this controller, we should simply
	// specify the Pod we want to exist directly, and not call MakePod from
	// the build. This will break the cycle and allow us to simply use
	// clients normally.
	return resources.MakePod(taskRun, task.Spec, fakekubeclientset.NewSimpleClientset(&corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "default",
			Namespace: taskRun.Namespace,
		},
	}), cache, logger)
}

func TestReconcilePodUpdateStatus(t *testing.T) {
	taskRun := tb.TaskRun("test-taskrun-run-success", "foo", tb.TaskRunSpec(tb.TaskRunTaskRef("test-task")))

	pod, err := makePod(taskRun, simpleTask)
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
		Reason:  status.ReasonSucceeded,
		Message: "All Steps have completed executing",
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
	expectedVolumes := len(task.Spec.Volumes) + 2

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
			taskRun: tb.TaskRun("test-taskrun-default-timeout-60-minutes", "foo",
				tb.TaskRunSpec(
					tb.TaskRunTaskRef(simpleTask.Name),
				),
				tb.TaskRunStatus(tb.Condition(apis.Condition{
					Type:   apis.ConditionSucceeded,
					Status: corev1.ConditionUnknown}),
					tb.TaskRunStartTime(time.Now().Add(-61*time.Minute)))),

			expectedStatus: &apis.Condition{
				Type:    apis.ConditionSucceeded,
				Status:  corev1.ConditionFalse,
				Reason:  "TaskRunTimeout",
				Message: `TaskRun "test-taskrun-default-timeout-60-minutes" failed to finish within "1h0m0s"`,
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

func TestHandlePodCreationError(t *testing.T) {
	taskRun := tb.TaskRun("test-taskrun-pod-creation-failed", "foo", tb.TaskRunSpec(
		tb.TaskRunTaskRef(simpleTask.Name),
	), tb.TaskRunStatus(
		tb.TaskRunStartTime(time.Now()),
		tb.Condition(apis.Condition{
			Type:   apis.ConditionSucceeded,
			Status: corev1.ConditionUnknown,
		}),
	))
	d := test.Data{
		TaskRuns: []*v1alpha1.TaskRun{taskRun},
		Tasks:    []*v1alpha1.Task{simpleTask},
	}
	testAssets := getTaskRunController(t, d)
	c, ok := testAssets.Controller.Reconciler.(*Reconciler)
	if !ok {
		t.Errorf("failed to construct instance of taskrun reconciler")
		return
	}

	// Prevent backoff timer from starting
	c.timeoutHandler.SetTaskRunCallbackFunc(nil)

	testcases := []struct {
		description    string
		err            error
		expectedType   apis.ConditionType
		expectedStatus corev1.ConditionStatus
		expectedReason string
	}{
		{
			description:    "exceeded quota errors are surfaced in taskrun condition but do not fail taskrun",
			err:            k8sapierrors.NewForbidden(k8sruntimeschema.GroupResource{Group: "foo", Resource: "bar"}, "baz", errors.New("exceeded quota")),
			expectedType:   apis.ConditionSucceeded,
			expectedStatus: corev1.ConditionUnknown,
			expectedReason: status.ReasonExceededResourceQuota,
		},
		{
			description:    "errors other than exceeded quota fail the taskrun",
			err:            errors.New("this is a fatal error"),
			expectedType:   apis.ConditionSucceeded,
			expectedStatus: corev1.ConditionFalse,
			expectedReason: status.ReasonCouldntGetTask,
		},
	}
	for _, tc := range testcases {
		t.Run(tc.description, func(t *testing.T) {
			c.handlePodCreationError(taskRun, tc.err)
			foundCondition := false
			for _, cond := range taskRun.Status.Conditions {
				if cond.Type == tc.expectedType && cond.Status == tc.expectedStatus && cond.Reason == tc.expectedReason {
					foundCondition = true
					break
				}
			}
			if !foundCondition {
				t.Errorf("expected to find condition type %q, status %q and reason %q", tc.expectedType, tc.expectedStatus, tc.expectedReason)
			}
		})
	}
}
