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
	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	podconvert "github.com/tektoncd/pipeline/pkg/pod"
	"github.com/tektoncd/pipeline/pkg/reconciler/taskrun/resources/cloudevent"
	ttesting "github.com/tektoncd/pipeline/pkg/reconciler/testing"
	"github.com/tektoncd/pipeline/pkg/system"
	"github.com/tektoncd/pipeline/test"
	tb "github.com/tektoncd/pipeline/test/builder"
	"github.com/tektoncd/pipeline/test/names"
	corev1 "k8s.io/api/core/v1"
	k8sapierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8sruntimeschema "k8s.io/apimachinery/pkg/runtime/schema"
	fakekubeclientset "k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"
	"knative.dev/pkg/apis"
	duckv1beta1 "knative.dev/pkg/apis/duck/v1beta1"
	"knative.dev/pkg/configmap"
)

const (
	entrypointLocation  = "/tekton/tools/entrypoint"
	taskNameLabelKey    = pipeline.GroupName + pipeline.TaskLabelKey
	taskRunNameLabelKey = pipeline.GroupName + pipeline.TaskRunLabelKey
	workspaceDir        = "/workspace"
	currentAPIVersion   = "tekton.dev/v1alpha1"
)

var (
	images = pipeline.Images{
		EntrypointImage:          "override-with-entrypoint:latest",
		NopImage:                 "tianon/true",
		GitImage:                 "override-with-git:latest",
		CredsImage:               "override-with-creds:latest",
		KubeconfigWriterImage:    "override-with-kubeconfig-writer:latest",
		ShellImage:               "busybox",
		GsutilImage:              "google/cloud-sdk",
		BuildGCSFetcherImage:     "gcr.io/cloud-builders/gcs-fetcher:latest",
		PRImage:                  "override-with-pr:latest",
		ImageDigestExporterImage: "override-with-imagedigest-exporter-image:latest",
	}
	ignoreLastTransitionTime = cmpopts.IgnoreTypes(apis.Condition{}.LastTransitionTime.Inner.Time)
	// Pods are created with a random 5-character suffix that we want to
	// ignore in our diffs.
	ignoreRandomPodNameSuffix = cmp.FilterPath(func(path cmp.Path) bool {
		return path.GoString() == "{v1.ObjectMeta}.Name"
	}, cmp.Comparer(func(name1, name2 string) bool {
		return name1[:len(name1)-5] == name2[:len(name2)-5]
	}))
	resourceQuantityCmp = cmp.Comparer(func(x, y resource.Quantity) bool {
		return x.Cmp(y) == 0
	})
	cloudEventTarget1 = "https://foo"
	cloudEventTarget2 = "https://bar"

	simpleStep        = tb.Step("foo", tb.StepName("simple-step"), tb.StepCommand("/mycmd"))
	simpleTask        = tb.Task("test-task", "foo", tb.TaskSpec(simpleStep))
	taskMultipleSteps = tb.Task("test-task-multi-steps", "foo", tb.TaskSpec(
		tb.Step("foo", tb.StepName("z-step"),
			tb.StepCommand("/mycmd"),
		),
		tb.Step("foo", tb.StepName("v-step"),
			tb.StepCommand("/mycmd"),
		),
		tb.Step("foo", tb.StepName("x-step"),
			tb.StepCommand("/mycmd"),
		),
	))
	clustertask = tb.ClusterTask("test-cluster-task", tb.ClusterTaskSpec(simpleStep))
	taskSidecar = tb.Task("test-task-sidecar", "foo", tb.TaskSpec(
		tb.Sidecar("sidecar", "image-id"),
	))
	taskMultipleSidecars = tb.Task("test-task-sidecar", "foo", tb.TaskSpec(
		tb.Sidecar("sidecar", "image-id"),
		tb.Sidecar("sidecar2", "image-id"),
	))

	outputTask = tb.Task("test-output-task", "foo", tb.TaskSpec(
		simpleStep, tb.TaskInputs(
			tb.InputsResource(gitResource.Name, v1alpha1.PipelineResourceTypeGit),
			tb.InputsResource(anotherGitResource.Name, v1alpha1.PipelineResourceTypeGit),
		),
		tb.TaskOutputs(tb.OutputsResource(gitResource.Name, v1alpha1.PipelineResourceTypeGit)),
	))

	saTask = tb.Task("test-with-sa", "foo", tb.TaskSpec(tb.Step("foo", tb.StepName("sa-step"), tb.StepCommand("/mycmd"))))

	templatedTask = tb.Task("test-task-with-substitution", "foo", tb.TaskSpec(
		tb.TaskInputs(
			tb.InputsResource("workspace", v1alpha1.PipelineResourceTypeGit),
			tb.InputsParamSpec("myarg", v1alpha1.ParamTypeString), tb.InputsParamSpec("myarghasdefault", v1alpha1.ParamTypeString, tb.ParamSpecDefault("dont see me")),
			tb.InputsParamSpec("myarghasdefault2", v1alpha1.ParamTypeString, tb.ParamSpecDefault("thedefault")),
			tb.InputsParamSpec("configmapname", v1alpha1.ParamTypeString),
		),
		tb.TaskOutputs(tb.OutputsResource("myimage", v1alpha1.PipelineResourceTypeImage)),
		tb.Step("myimage", tb.StepName("mycontainer"), tb.StepCommand("/mycmd"), tb.StepArgs(
			"--my-arg=$(inputs.params.myarg)",
			"--my-arg-with-default=$(inputs.params.myarghasdefault)",
			"--my-arg-with-default2=$(inputs.params.myarghasdefault2)",
			"--my-additional-arg=$(outputs.resources.myimage.url)",
		)),
		tb.Step("myotherimage", tb.StepName("myothercontainer"), tb.StepCommand("/mycmd"), tb.StepArgs(
			"--my-other-arg=$(inputs.resources.workspace.url)",
		)),
		tb.TaskVolume("volume-configmap", tb.VolumeSource(corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: "$(inputs.params.configmapname)",
				},
			},
		})),
	))

	twoOutputsTask = tb.Task("test-two-output-task", "foo", tb.TaskSpec(
		simpleStep, tb.TaskOutputs(
			tb.OutputsResource(cloudEventResource.Name, v1alpha1.PipelineResourceTypeCloudEvent),
			tb.OutputsResource(anotherCloudEventResource.Name, v1alpha1.PipelineResourceTypeCloudEvent),
		),
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
	cloudEventResource = tb.PipelineResource("cloud-event-resource", "foo", tb.PipelineResourceSpec(
		v1alpha1.PipelineResourceTypeCloudEvent, tb.PipelineResourceSpecParam("TargetURI", cloudEventTarget1),
	))
	anotherCloudEventResource = tb.PipelineResource("another-cloud-event-resource", "foo", tb.PipelineResourceSpec(
		v1alpha1.PipelineResourceTypeCloudEvent, tb.PipelineResourceSpecParam("TargetURI", cloudEventTarget2),
	))

	toolsVolume = corev1.Volume{
		Name: "tekton-internal-tools",
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	}
	workspaceVolume = corev1.Volume{
		Name: "tekton-internal-workspace",
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	}
	homeVolume = corev1.Volume{
		Name: "tekton-internal-home",
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	}
	resultsVolume = corev1.Volume{
		Name: "tekton-internal-results",
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	}
	downwardVolume = corev1.Volume{
		Name: "tekton-internal-downward",
		VolumeSource: corev1.VolumeSource{
			DownwardAPI: &corev1.DownwardAPIVolumeSource{
				Items: []corev1.DownwardAPIVolumeFile{{
					Path: "ready",
					FieldRef: &corev1.ObjectFieldSelector{
						FieldPath: "metadata.annotations['tekton.dev/ready']",
					},
				}},
			},
		},
	}

	getMkdirResourceContainer = func(name, dir, suffix string, ops ...tb.ContainerOp) tb.PodSpecOp {
		actualOps := []tb.ContainerOp{
			tb.Command("/tekton/tools/entrypoint"),
			tb.Args("-wait_file",
				"/tekton/downward/ready",
				"-wait_file_content",
				"-post_file",
				"/tekton/tools/0",
				"-termination_path",
				"/tekton/termination",
				"-entrypoint",
				"mkdir",
				"--",
				"-p",
				dir),
			tb.WorkingDir(workspaceDir),
			tb.EnvVar("HOME", "/tekton/home"),
			tb.VolumeMount("tekton-internal-tools", "/tekton/tools"),
			tb.VolumeMount("tekton-internal-downward", "/tekton/downward"),
			tb.VolumeMount("tekton-internal-workspace", workspaceDir),
			tb.VolumeMount("tekton-internal-home", "/tekton/home"),
			tb.VolumeMount("tekton-internal-results", "/tekton/results"),
			tb.TerminationMessagePath("/tekton/termination"),
		}

		actualOps = append(actualOps, ops...)

		return tb.PodContainer(fmt.Sprintf("step-create-dir-%s-%s", name, suffix), "busybox", actualOps...)
	}

	getPlaceToolsInitContainer = func(ops ...tb.ContainerOp) tb.PodSpecOp {
		actualOps := []tb.ContainerOp{
			tb.Command("cp", "/ko-app/entrypoint", entrypointLocation),
			tb.VolumeMount("tekton-internal-tools", "/tekton/tools"),
			tb.Args(),
		}
		actualOps = append(actualOps, ops...)
		return tb.PodInitContainer("place-tools", "override-with-entrypoint:latest", actualOps...)
	}
)

func getRunName(tr *v1alpha1.TaskRun) string {
	return strings.Join([]string{tr.Namespace, tr.Name}, "/")
}

// getTaskRunController returns an instance of the TaskRun controller/reconciler that has been seeded with
// d, where d represents the state of the system (existing resources) needed for the test.
func getTaskRunController(t *testing.T, d test.Data) (test.Assets, func()) {
	ctx, _ := ttesting.SetupFakeContext(t)
	ctx, cancel := context.WithCancel(ctx)
	cloudEventClientBehaviour := cloudevent.FakeClientBehaviour{
		SendSuccessfully: true,
	}
	ctx = cloudevent.WithClient(ctx, &cloudEventClientBehaviour)
	c, _ := test.SeedTestData(t, ctx, d)
	configMapWatcher := configmap.NewInformedWatcher(c.Kube, system.GetNamespace())
	return test.Assets{
		Controller: NewController(images)(ctx, configMapWatcher),
		Clients:    c,
	}, cancel
}

func TestReconcile_ExplicitDefaultSA(t *testing.T) {
	taskRunSuccess := tb.TaskRun("test-taskrun-run-success", "foo", tb.TaskRunSpec(
		tb.TaskRunTaskRef(simpleTask.Name, tb.TaskRefAPIVersion("a1")),
	))
	taskRunWithSaSuccess := tb.TaskRun("test-taskrun-with-sa-run-success", "foo", tb.TaskRunSpec(
		tb.TaskRunTaskRef(saTask.Name, tb.TaskRefAPIVersion("a1")),
		tb.TaskRunServiceAccountName("test-sa"),
	))
	taskruns := []*v1alpha1.TaskRun{taskRunSuccess, taskRunWithSaSuccess}
	d := test.Data{
		TaskRuns: taskruns,
		Tasks:    []*v1alpha1.Task{simpleTask, saTask},
	}

	defaultSAName := "pipelines"
	defaultCfg := &config.Config{
		Defaults: &config.Defaults{
			DefaultServiceAccount:      defaultSAName,
			DefaultTimeoutMinutes:      60,
			DefaultManagedByLabelValue: "tekton-pipelines",
		},
	}

	for _, tc := range []struct {
		name    string
		taskRun *v1alpha1.TaskRun
		wantPod *corev1.Pod
	}{{
		name:    "success",
		taskRun: taskRunSuccess,
		wantPod: tb.Pod("test-taskrun-run-success-pod-abcde", "foo",
			tb.PodAnnotation(podconvert.ReleaseAnnotation, podconvert.ReleaseAnnotationValue),
			tb.PodLabel(taskNameLabelKey, "test-task"),
			tb.PodLabel(taskRunNameLabelKey, "test-taskrun-run-success"),
			tb.PodLabel("app.kubernetes.io/managed-by", "tekton-pipelines"),
			tb.PodOwnerReference("TaskRun", "test-taskrun-run-success",
				tb.OwnerReferenceAPIVersion(currentAPIVersion)),
			tb.PodSpec(
				tb.PodServiceAccountName(defaultSAName),
				tb.PodVolumes(workspaceVolume, homeVolume, resultsVolume, toolsVolume, downwardVolume),
				tb.PodRestartPolicy(corev1.RestartPolicyNever),
				getPlaceToolsInitContainer(),
				tb.PodContainer("step-simple-step", "foo",
					tb.Command(entrypointLocation),
					tb.Args("-wait_file",
						"/tekton/downward/ready",
						"-wait_file_content",
						"-post_file",
						"/tekton/tools/0",
						"-termination_path",
						"/tekton/termination",
						"-entrypoint",
						"/mycmd",
						"--",
					),
					tb.WorkingDir(workspaceDir),
					tb.EnvVar("HOME", "/tekton/home"),
					tb.VolumeMount("tekton-internal-tools", "/tekton/tools"),
					tb.VolumeMount("tekton-internal-downward", "/tekton/downward"),
					tb.VolumeMount("tekton-internal-workspace", workspaceDir),
					tb.VolumeMount("tekton-internal-home", "/tekton/home"),
					tb.VolumeMount("tekton-internal-results", "/tekton/results"),
					tb.TerminationMessagePath("/tekton/termination"),
				),
			),
		),
	}, {
		name:    "serviceaccount",
		taskRun: taskRunWithSaSuccess,
		wantPod: tb.Pod("test-taskrun-with-sa-run-success-pod-abcde", "foo",
			tb.PodAnnotation(podconvert.ReleaseAnnotation, podconvert.ReleaseAnnotationValue),
			tb.PodLabel(taskNameLabelKey, "test-with-sa"),
			tb.PodLabel(taskRunNameLabelKey, "test-taskrun-with-sa-run-success"),
			tb.PodLabel("app.kubernetes.io/managed-by", "tekton-pipelines"),
			tb.PodOwnerReference("TaskRun", "test-taskrun-with-sa-run-success",
				tb.OwnerReferenceAPIVersion(currentAPIVersion)),
			tb.PodSpec(
				tb.PodServiceAccountName("test-sa"),
				tb.PodVolumes(workspaceVolume, homeVolume, resultsVolume, toolsVolume, downwardVolume),
				tb.PodRestartPolicy(corev1.RestartPolicyNever),
				getPlaceToolsInitContainer(),
				tb.PodContainer("step-sa-step", "foo",
					tb.Command(entrypointLocation),
					tb.Args("-wait_file",
						"/tekton/downward/ready",
						"-wait_file_content",
						"-post_file",
						"/tekton/tools/0",
						"-termination_path",
						"/tekton/termination",
						"-entrypoint",
						"/mycmd",
						"--",
					),
					tb.WorkingDir(workspaceDir),
					tb.EnvVar("HOME", "/tekton/home"),
					tb.VolumeMount("tekton-internal-tools", "/tekton/tools"),
					tb.VolumeMount("tekton-internal-downward", "/tekton/downward"),
					tb.VolumeMount("tekton-internal-workspace", workspaceDir),
					tb.VolumeMount("tekton-internal-home", "/tekton/home"),
					tb.VolumeMount("tekton-internal-results", "/tekton/results"),
					tb.TerminationMessagePath("/tekton/termination"),
				),
			),
		),
	}} {
		t.Run(tc.name, func(t *testing.T) {
			names.TestingSeed()
			testAssets, cancel := getTaskRunController(t, d)
			defer cancel()
			c := testAssets.Controller
			clients := testAssets.Clients
			saName := tc.taskRun.Spec.ServiceAccountName
			if saName == "" {
				saName = defaultSAName
			}
			if _, err := clients.Kube.CoreV1().ServiceAccounts(tc.taskRun.Namespace).Create(&corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name:      saName,
					Namespace: tc.taskRun.Namespace,
				},
			}); err != nil {
				t.Fatal(err)
			}

			ctx := config.ToContext(context.Background(), defaultCfg)
			if err := c.Reconciler.Reconcile(ctx, getRunName(tc.taskRun)); err != nil {
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
			if condition != nil && condition.Reason != podconvert.ReasonRunning {
				t.Errorf("Expected reason %q but was %s", podconvert.ReasonRunning, condition.Reason)
			}

			if tr.Status.PodName == "" {
				t.Fatalf("Reconcile didn't set pod name")
			}

			pod, err := clients.Kube.CoreV1().Pods(tr.Namespace).Get(tr.Status.PodName, metav1.GetOptions{})
			if err != nil {
				t.Fatalf("Failed to fetch build pod: %v", err)
			}

			if d := cmp.Diff(tc.wantPod.ObjectMeta, pod.ObjectMeta, ignoreRandomPodNameSuffix); d != "" {
				t.Errorf("Pod metadata doesn't match (-want, +got): %s", d)
			}

			if d := cmp.Diff(tc.wantPod.Spec, pod.Spec, resourceQuantityCmp); d != "" {
				t.Errorf("Pod spec doesn't match, (-want, +got): %s", d)
			}
			if len(clients.Kube.Actions()) == 0 {
				t.Fatalf("Expected actions to be logged in the kubeclient, got none")
			}
		})
	}
}

func TestReconcile(t *testing.T) {
	taskRunSuccess := tb.TaskRun("test-taskrun-run-success", "foo", tb.TaskRunSpec(
		tb.TaskRunTaskRef(simpleTask.Name, tb.TaskRefAPIVersion("a1")),
	))
	taskRunWithSaSuccess := tb.TaskRun("test-taskrun-with-sa-run-success", "foo", tb.TaskRunSpec(
		tb.TaskRunTaskRef(saTask.Name, tb.TaskRefAPIVersion("a1")), tb.TaskRunServiceAccountName("test-sa"),
	))
	taskRunSubstitution := tb.TaskRun("test-taskrun-substitution", "foo", tb.TaskRunSpec(
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
					tb.TaskResourceBindingRef(gitResource.Name),
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
				tb.InputsParamSpec("myarg", v1alpha1.ParamTypeString, tb.ParamSpecDefault("mydefault")),
			),
			tb.Step("myimage", tb.StepName("mycontainer"), tb.StepCommand("/mycmd"),
				tb.StepArgs("--my-arg=$(inputs.params.myarg)"),
			),
		),
	))

	taskRunWithResourceSpecAndTaskSpec := tb.TaskRun("test-taskrun-with-resource-spec", "foo", tb.TaskRunSpec(
		tb.TaskRunInputs(
			tb.TaskRunInputsResource("workspace", tb.TaskResourceBindingResourceSpec(&v1alpha1.PipelineResourceSpec{
				Type: v1alpha1.PipelineResourceTypeGit,
				Params: []v1alpha1.ResourceParam{{
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
			tb.Step("ubuntu", tb.StepName("mystep"), tb.StepCommand("/mycmd")),
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

	taskRunWithPod := tb.TaskRun("test-taskrun-with-pod", "foo",
		tb.TaskRunSpec(tb.TaskRunTaskRef(simpleTask.Name)),
		tb.TaskRunStatus(tb.PodName("some-pod-abcdethat-no-longer-exists")),
	)

	taskruns := []*v1alpha1.TaskRun{
		taskRunSuccess, taskRunWithSaSuccess,
		taskRunSubstitution, taskRunInputOutput,
		taskRunWithTaskSpec, taskRunWithClusterTask, taskRunWithResourceSpecAndTaskSpec,
		taskRunWithLabels, taskRunWithAnnotations, taskRunWithPod,
	}

	d := test.Data{
		TaskRuns:          taskruns,
		Tasks:             []*v1alpha1.Task{simpleTask, saTask, templatedTask, outputTask},
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
		wantPod: tb.Pod("test-taskrun-run-success-pod-abcde", "foo",
			tb.PodAnnotation(podconvert.ReleaseAnnotation, podconvert.ReleaseAnnotationValue),
			tb.PodLabel(taskNameLabelKey, "test-task"),
			tb.PodLabel(taskRunNameLabelKey, "test-taskrun-run-success"),
			tb.PodLabel("app.kubernetes.io/managed-by", "tekton-pipelines"),
			tb.PodOwnerReference("TaskRun", "test-taskrun-run-success",
				tb.OwnerReferenceAPIVersion(currentAPIVersion)),
			tb.PodSpec(
				tb.PodVolumes(workspaceVolume, homeVolume, resultsVolume, toolsVolume, downwardVolume),
				tb.PodRestartPolicy(corev1.RestartPolicyNever),
				getPlaceToolsInitContainer(),
				tb.PodContainer("step-simple-step", "foo",
					tb.Command(entrypointLocation),
					tb.Args("-wait_file",
						"/tekton/downward/ready",
						"-wait_file_content",
						"-post_file",
						"/tekton/tools/0",
						"-termination_path",
						"/tekton/termination",
						"-entrypoint",
						"/mycmd",
						"--",
					),
					tb.WorkingDir(workspaceDir),
					tb.EnvVar("HOME", "/tekton/home"),
					tb.VolumeMount("tekton-internal-tools", "/tekton/tools"),
					tb.VolumeMount("tekton-internal-downward", "/tekton/downward"),
					tb.VolumeMount("tekton-internal-workspace", workspaceDir),
					tb.VolumeMount("tekton-internal-home", "/tekton/home"),
					tb.VolumeMount("tekton-internal-results", "/tekton/results"),
					tb.TerminationMessagePath("/tekton/termination"),
				),
			),
		),
	}, {
		name:    "serviceaccount",
		taskRun: taskRunWithSaSuccess,
		wantPod: tb.Pod("test-taskrun-with-sa-run-success-pod-abcde", "foo",
			tb.PodAnnotation(podconvert.ReleaseAnnotation, podconvert.ReleaseAnnotationValue),
			tb.PodLabel(taskNameLabelKey, "test-with-sa"),
			tb.PodLabel(taskRunNameLabelKey, "test-taskrun-with-sa-run-success"),
			tb.PodLabel("app.kubernetes.io/managed-by", "tekton-pipelines"),
			tb.PodOwnerReference("TaskRun", "test-taskrun-with-sa-run-success",
				tb.OwnerReferenceAPIVersion(currentAPIVersion)),
			tb.PodSpec(
				tb.PodServiceAccountName("test-sa"),
				tb.PodVolumes(workspaceVolume, homeVolume, resultsVolume, toolsVolume, downwardVolume),
				tb.PodRestartPolicy(corev1.RestartPolicyNever),
				getPlaceToolsInitContainer(),
				tb.PodContainer("step-sa-step", "foo",
					tb.Command(entrypointLocation),
					tb.Args("-wait_file",
						"/tekton/downward/ready",
						"-wait_file_content",
						"-post_file",
						"/tekton/tools/0",
						"-termination_path",
						"/tekton/termination",
						"-entrypoint",
						"/mycmd",
						"--",
					),
					tb.WorkingDir(workspaceDir),
					tb.EnvVar("HOME", "/tekton/home"),
					tb.VolumeMount("tekton-internal-tools", "/tekton/tools"),
					tb.VolumeMount("tekton-internal-downward", "/tekton/downward"),
					tb.VolumeMount("tekton-internal-workspace", workspaceDir),
					tb.VolumeMount("tekton-internal-home", "/tekton/home"),
					tb.VolumeMount("tekton-internal-results", "/tekton/results"),
					tb.TerminationMessagePath("/tekton/termination"),
				),
			),
		),
	}, {
		name:    "params",
		taskRun: taskRunSubstitution,
		wantPod: tb.Pod("test-taskrun-substitution-pod-abcde", "foo",
			tb.PodAnnotation(podconvert.ReleaseAnnotation, podconvert.ReleaseAnnotationValue),
			tb.PodLabel(taskNameLabelKey, "test-task-with-substitution"),
			tb.PodLabel(taskRunNameLabelKey, "test-taskrun-substitution"),
			tb.PodLabel("app.kubernetes.io/managed-by", "tekton-pipelines"),
			tb.PodOwnerReference("TaskRun", "test-taskrun-substitution",
				tb.OwnerReferenceAPIVersion(currentAPIVersion)),
			tb.PodSpec(
				tb.PodVolumes(
					workspaceVolume, homeVolume, resultsVolume, toolsVolume, downwardVolume,
					corev1.Volume{
						Name: "volume-configmap",
						VolumeSource: corev1.VolumeSource{
							ConfigMap: &corev1.ConfigMapVolumeSource{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: "configbar",
								},
							},
						},
					},
				),
				tb.PodRestartPolicy(corev1.RestartPolicyNever),
				getPlaceToolsInitContainer(),
				getMkdirResourceContainer("myimage", "/workspace/output/myimage", "mssqb"),
				tb.PodContainer("step-git-source-git-resource-mz4c7", "override-with-git:latest",
					tb.Command(entrypointLocation),
					tb.Args("-wait_file", "/tekton/tools/0", "-post_file", "/tekton/tools/1", "-termination_path",
						"/tekton/termination", "-entrypoint", "/ko-app/git-init", "--", "-url", "https://foo.git",
						"-revision", "master", "-path", "/workspace/workspace"),
					tb.WorkingDir(workspaceDir),
					tb.EnvVar("HOME", "/tekton/home"),
					tb.EnvVar("TEKTON_RESOURCE_NAME", "git-resource"),
					tb.VolumeMount("tekton-internal-tools", "/tekton/tools"),
					tb.VolumeMount("tekton-internal-workspace", workspaceDir),
					tb.VolumeMount("tekton-internal-home", "/tekton/home"),
					tb.VolumeMount("tekton-internal-results", "/tekton/results"),
					tb.TerminationMessagePath("/tekton/termination"),
				),
				tb.PodContainer("step-mycontainer", "myimage",
					tb.Command(entrypointLocation),
					tb.Args("-wait_file", "/tekton/tools/1", "-post_file", "/tekton/tools/2", "-termination_path",
						"/tekton/termination", "-entrypoint", "/mycmd", "--", "--my-arg=foo", "--my-arg-with-default=bar",
						"--my-arg-with-default2=thedefault", "--my-additional-arg=gcr.io/kristoff/sven"),
					tb.WorkingDir(workspaceDir),
					tb.EnvVar("HOME", "/tekton/home"),
					tb.VolumeMount("tekton-internal-tools", "/tekton/tools"),
					tb.VolumeMount("tekton-internal-workspace", workspaceDir),
					tb.VolumeMount("tekton-internal-home", "/tekton/home"),
					tb.VolumeMount("tekton-internal-results", "/tekton/results"),
					tb.TerminationMessagePath("/tekton/termination"),
				),
				tb.PodContainer("step-myothercontainer", "myotherimage",
					tb.Command(entrypointLocation),
					tb.Args("-wait_file", "/tekton/tools/2", "-post_file", "/tekton/tools/3", "-termination_path",
						"/tekton/termination", "-entrypoint", "/mycmd", "--", "--my-other-arg=https://foo.git"),
					tb.WorkingDir(workspaceDir),
					tb.EnvVar("HOME", "/tekton/home"),
					tb.VolumeMount("tekton-internal-tools", "/tekton/tools"),
					tb.VolumeMount("tekton-internal-workspace", workspaceDir),
					tb.VolumeMount("tekton-internal-home", "/tekton/home"),
					tb.VolumeMount("tekton-internal-results", "/tekton/results"),
					tb.TerminationMessagePath("/tekton/termination"),
				),
				tb.PodContainer("step-image-digest-exporter-9l9zj", "override-with-imagedigest-exporter-image:latest",
					tb.Command(entrypointLocation),
					tb.Args("-wait_file", "/tekton/tools/3", "-post_file", "/tekton/tools/4", "-termination_path",
						"/tekton/termination", "-entrypoint", "/ko-app/imagedigestexporter", "--",
						"-images", "[{\"name\":\"image-resource\",\"type\":\"image\",\"url\":\"gcr.io/kristoff/sven\",\"digest\":\"\",\"OutputImageDir\":\"/workspace/output/myimage\"}]"),
					tb.WorkingDir(workspaceDir),
					tb.EnvVar("HOME", "/tekton/home"),
					tb.VolumeMount("tekton-internal-tools", "/tekton/tools"),
					tb.VolumeMount("tekton-internal-workspace", workspaceDir),
					tb.VolumeMount("tekton-internal-home", "/tekton/home"),
					tb.VolumeMount("tekton-internal-results", "/tekton/results"),
					tb.TerminationMessagePolicy(corev1.TerminationMessageFallbackToLogsOnError),
					tb.TerminationMessagePath("/tekton/termination"),
				),
			),
		),
	}, {
		name:    "taskrun-with-taskspec",
		taskRun: taskRunWithTaskSpec,
		wantPod: tb.Pod("test-taskrun-with-taskspec-pod-abcde", "foo",
			tb.PodAnnotation(podconvert.ReleaseAnnotation, podconvert.ReleaseAnnotationValue),
			tb.PodLabel(taskRunNameLabelKey, "test-taskrun-with-taskspec"),
			tb.PodLabel("app.kubernetes.io/managed-by", "tekton-pipelines"),
			tb.PodOwnerReference("TaskRun", "test-taskrun-with-taskspec",
				tb.OwnerReferenceAPIVersion(currentAPIVersion)),
			tb.PodSpec(
				tb.PodVolumes(workspaceVolume, homeVolume, resultsVolume, toolsVolume, downwardVolume),
				tb.PodRestartPolicy(corev1.RestartPolicyNever),
				getPlaceToolsInitContainer(),
				tb.PodContainer("step-git-source-git-resource-9l9zj", "override-with-git:latest",
					tb.Command(entrypointLocation),
					tb.Args("-wait_file",
						"/tekton/downward/ready",
						"-wait_file_content",
						"-post_file",
						"/tekton/tools/0",
						"-termination_path",
						"/tekton/termination",
						"-entrypoint",
						"/ko-app/git-init",
						"--",
						"-url",
						"https://foo.git",
						"-revision",
						"master",
						"-path",
						"/workspace/workspace",
					),
					tb.WorkingDir(workspaceDir),
					tb.EnvVar("HOME", "/tekton/home"),
					tb.EnvVar("TEKTON_RESOURCE_NAME", "git-resource"),
					tb.VolumeMount("tekton-internal-tools", "/tekton/tools"),
					tb.VolumeMount("tekton-internal-downward", "/tekton/downward"),
					tb.VolumeMount("tekton-internal-workspace", workspaceDir),
					tb.VolumeMount("tekton-internal-home", "/tekton/home"),
					tb.VolumeMount("tekton-internal-results", "/tekton/results"),
					tb.TerminationMessagePath("/tekton/termination"),
				),
				tb.PodContainer("step-mycontainer", "myimage",
					tb.Command(entrypointLocation),
					tb.WorkingDir(workspaceDir),
					tb.Args("-wait_file", "/tekton/tools/0", "-post_file", "/tekton/tools/1", "-termination_path",
						"/tekton/termination", "-entrypoint", "/mycmd", "--", "--my-arg=foo"),
					tb.EnvVar("HOME", "/tekton/home"),
					tb.VolumeMount("tekton-internal-tools", "/tekton/tools"),
					tb.VolumeMount("tekton-internal-workspace", workspaceDir),
					tb.VolumeMount("tekton-internal-home", "/tekton/home"),
					tb.VolumeMount("tekton-internal-results", "/tekton/results"),
					tb.TerminationMessagePath("/tekton/termination"),
				),
			),
		),
	}, {
		name:    "success-with-cluster-task",
		taskRun: taskRunWithClusterTask,
		wantPod: tb.Pod("test-taskrun-with-cluster-task-pod-abcde", "foo",
			tb.PodAnnotation(podconvert.ReleaseAnnotation, podconvert.ReleaseAnnotationValue),
			tb.PodLabel(taskNameLabelKey, "test-cluster-task"),
			tb.PodLabel(taskRunNameLabelKey, "test-taskrun-with-cluster-task"),
			tb.PodLabel("app.kubernetes.io/managed-by", "tekton-pipelines"),
			tb.PodOwnerReference("TaskRun", "test-taskrun-with-cluster-task",
				tb.OwnerReferenceAPIVersion(currentAPIVersion)),
			tb.PodSpec(
				tb.PodVolumes(workspaceVolume, homeVolume, resultsVolume, toolsVolume, downwardVolume),
				tb.PodRestartPolicy(corev1.RestartPolicyNever),
				getPlaceToolsInitContainer(),
				tb.PodContainer("step-simple-step", "foo",
					tb.Command(entrypointLocation),
					tb.Args("-wait_file",
						"/tekton/downward/ready",
						"-wait_file_content",
						"-post_file",
						"/tekton/tools/0",
						"-termination_path",
						"/tekton/termination",
						"-entrypoint",
						"/mycmd",
						"--",
					),
					tb.WorkingDir(workspaceDir),
					tb.EnvVar("HOME", "/tekton/home"),
					tb.VolumeMount("tekton-internal-tools", "/tekton/tools"),
					tb.VolumeMount("tekton-internal-downward", "/tekton/downward"),
					tb.VolumeMount("tekton-internal-workspace", workspaceDir),
					tb.VolumeMount("tekton-internal-home", "/tekton/home"),
					tb.VolumeMount("tekton-internal-results", "/tekton/results"),
					tb.TerminationMessagePath("/tekton/termination"),
				),
			),
		),
	}, {
		name:    "taskrun-with-resource-spec-task-spec",
		taskRun: taskRunWithResourceSpecAndTaskSpec,
		wantPod: tb.Pod("test-taskrun-with-resource-spec-pod-abcde", "foo",
			tb.PodAnnotation(podconvert.ReleaseAnnotation, podconvert.ReleaseAnnotationValue),
			tb.PodLabel(taskRunNameLabelKey, "test-taskrun-with-resource-spec"),
			tb.PodLabel("app.kubernetes.io/managed-by", "tekton-pipelines"),
			tb.PodOwnerReference("TaskRun", "test-taskrun-with-resource-spec",
				tb.OwnerReferenceAPIVersion(currentAPIVersion)),
			tb.PodSpec(
				tb.PodVolumes(workspaceVolume, homeVolume, resultsVolume, toolsVolume, downwardVolume),
				tb.PodRestartPolicy(corev1.RestartPolicyNever),
				getPlaceToolsInitContainer(),
				tb.PodContainer("step-git-source-workspace-9l9zj", "override-with-git:latest",
					tb.Command(entrypointLocation),
					tb.Args("-wait_file",
						"/tekton/downward/ready",
						"-wait_file_content",
						"-post_file",
						"/tekton/tools/0",
						"-termination_path",
						"/tekton/termination",
						"-entrypoint",
						"/ko-app/git-init",
						"--",
						"-url",
						"github.com/foo/bar.git",
						"-revision",
						"rel-can",
						"-path",
						"/workspace/workspace"),
					tb.WorkingDir(workspaceDir),
					tb.EnvVar("HOME", "/tekton/home"),
					tb.EnvVar("TEKTON_RESOURCE_NAME", "workspace"),
					tb.VolumeMount("tekton-internal-tools", "/tekton/tools"),
					tb.VolumeMount("tekton-internal-downward", "/tekton/downward"),
					tb.VolumeMount("tekton-internal-workspace", workspaceDir),
					tb.VolumeMount("tekton-internal-home", "/tekton/home"),
					tb.VolumeMount("tekton-internal-results", "/tekton/results"),
					tb.TerminationMessagePath("/tekton/termination"),
				),
				tb.PodContainer("step-mystep", "ubuntu",
					tb.Command(entrypointLocation),
					tb.Args("-wait_file", "/tekton/tools/0", "-post_file", "/tekton/tools/1", "-termination_path",
						"/tekton/termination", "-entrypoint", "/mycmd", "--"),
					tb.WorkingDir(workspaceDir),
					tb.EnvVar("HOME", "/tekton/home"),
					tb.VolumeMount("tekton-internal-tools", "/tekton/tools"),
					tb.VolumeMount("tekton-internal-workspace", workspaceDir),
					tb.VolumeMount("tekton-internal-home", "/tekton/home"),
					tb.VolumeMount("tekton-internal-results", "/tekton/results"),
					tb.TerminationMessagePath("/tekton/termination"),
				),
			),
		),
	}, {
		name:    "taskrun-with-pod",
		taskRun: taskRunWithPod,
		wantPod: tb.Pod("test-taskrun-with-pod-pod-abcde", "foo",
			tb.PodAnnotation(podconvert.ReleaseAnnotation, podconvert.ReleaseAnnotationValue),
			tb.PodLabel(taskNameLabelKey, "test-task"),
			tb.PodLabel(taskRunNameLabelKey, "test-taskrun-with-pod"),
			tb.PodLabel("app.kubernetes.io/managed-by", "tekton-pipelines"),
			tb.PodOwnerReference("TaskRun", "test-taskrun-with-pod",
				tb.OwnerReferenceAPIVersion(currentAPIVersion)),
			tb.PodSpec(
				tb.PodVolumes(workspaceVolume, homeVolume, resultsVolume, toolsVolume, downwardVolume),
				tb.PodRestartPolicy(corev1.RestartPolicyNever),
				getPlaceToolsInitContainer(),
				tb.PodContainer("step-simple-step", "foo",
					tb.Command(entrypointLocation),
					tb.Args("-wait_file",
						"/tekton/downward/ready",
						"-wait_file_content",
						"-post_file",
						"/tekton/tools/0",
						"-termination_path",
						"/tekton/termination",
						"-entrypoint",
						"/mycmd",
						"--"),
					tb.WorkingDir(workspaceDir),
					tb.EnvVar("HOME", "/tekton/home"),
					tb.VolumeMount("tekton-internal-tools", "/tekton/tools"),
					tb.VolumeMount("tekton-internal-downward", "/tekton/downward"),
					tb.VolumeMount("tekton-internal-workspace", workspaceDir),
					tb.VolumeMount("tekton-internal-home", "/tekton/home"),
					tb.VolumeMount("tekton-internal-results", "/tekton/results"),
					tb.TerminationMessagePath("/tekton/termination"),
				),
			),
		),
	}} {
		t.Run(tc.name, func(t *testing.T) {
			names.TestingSeed()
			testAssets, cancel := getTaskRunController(t, d)
			defer cancel()
			c := testAssets.Controller
			clients := testAssets.Clients
			saName := tc.taskRun.Spec.ServiceAccountName
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
			if condition != nil && condition.Reason != podconvert.ReasonRunning {
				t.Errorf("Expected reason %q but was %s", podconvert.ReasonRunning, condition.Reason)
			}

			if tr.Status.PodName == "" {
				t.Fatalf("Reconcile didn't set pod name")
			}

			pod, err := clients.Kube.CoreV1().Pods(tr.Namespace).Get(tr.Status.PodName, metav1.GetOptions{})
			if err != nil {
				t.Fatalf("Failed to fetch build pod: %v", err)
			}

			if d := cmp.Diff(tc.wantPod.ObjectMeta, pod.ObjectMeta, ignoreRandomPodNameSuffix); d != "" {
				t.Errorf("Pod metadata doesn't match (-want, +got): %s", d)
			}

			pod.Name = tc.wantPod.Name // Ignore pod name differences, the pod name is generated and tested in pod_test.go
			if d := cmp.Diff(tc.wantPod.Spec, pod.Spec, resourceQuantityCmp); d != "" {
				t.Errorf("Pod spec doesn't match (-want, +got): %s", d)
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
	testAssets, cancel := getTaskRunController(t, d)
	defer cancel()

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
	testAssets, cancel := getTaskRunController(t, d)
	defer cancel()
	if err := testAssets.Controller.Reconciler.Reconcile(context.Background(), getRunName(taskRun)); err != nil {
		t.Errorf("expected no error reconciling valid TaskRun but got %v", err)
	}
	verifyTaskRunStatusStep(t, taskRun)
}

func verifyTaskRunStatusStep(t *testing.T, taskRun *v1alpha1.TaskRun) {
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
	if d := cmp.Diff(expectedStepOrder, actualStepOrder); d != "" {
		t.Errorf("The status steps in TaksRun doesn't match the spec steps in Task (-want, +got): %s", d)
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
	testAssets, cancel := getTaskRunController(t, d)
	defer cancel()

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
	}{{
		name:    "task run with no task",
		taskRun: noTaskRun,
		reason:  podconvert.ReasonFailedResolution,
	}, {
		name:    "task run with no task",
		taskRun: withWrongRef,
		reason:  podconvert.ReasonFailedResolution,
	}}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			testAssets, cancel := getTaskRunController(t, d)
			defer cancel()
			c := testAssets.Controller
			clients := testAssets.Clients
			err := c.Reconciler.Reconcile(context.Background(), getRunName(tc.taskRun))
			// When a TaskRun is invalid and can't run, we don't want to return an error because
			// an error will tell the Reconciler to keep trying to reconcile; instead we want to stop
			// and forget about the Run.
			if err != nil {
				t.Errorf("Did not expect to see error when reconciling invalid TaskRun but saw %q", err)
			}
			if len(clients.Kube.Actions()) != 1 ||
				clients.Kube.Actions()[0].GetVerb() != "list" ||
				clients.Kube.Actions()[0].GetResource().Resource != "namespaces" {
				t.Errorf("expected only one action (list namespaces) created by the reconciler, got %+v", clients.Kube.Actions())
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

	testAssets, cancel := getTaskRunController(t, d)
	defer cancel()
	c := testAssets.Controller
	clients := testAssets.Clients

	clients.Kube.PrependReactor("get", "pods", func(action ktesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, errors.New("induce failure fetching pods")
	})

	if err := c.Reconciler.Reconcile(context.Background(), getRunName(taskRun)); err == nil {
		t.Fatal("expected error when reconciling a Task for which we couldn't get the corresponding Build Pod but got nil")
	}
}

func makePod(taskRun *v1alpha1.TaskRun, task *v1alpha1.Task) (*corev1.Pod, error) {
	// TODO(jasonhall): This avoids a circular dependency where
	// getTaskRunController takes a test.Data which must be populated with
	// a pod created from MakePod which requires a (fake) Kube client. When
	// we remove Build entirely from this controller, we should simply
	// specify the Pod we want to exist directly, and not call MakePod from
	// the build. This will break the cycle and allow us to simply use
	// clients normally.
	kubeclient := fakekubeclientset.NewSimpleClientset(&corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "default",
			Namespace: taskRun.Namespace,
		},
	})

	entrypointCache, err := podconvert.NewEntrypointCache(kubeclient)
	if err != nil {
		return nil, err
	}

	return podconvert.MakePod(images, taskRun, task.Spec, kubeclient, entrypointCache)
}

func TestReconcilePodUpdateStatus(t *testing.T) {
	taskRun := tb.TaskRun("test-taskrun-run-success", "foo", tb.TaskRunSpec(tb.TaskRunTaskRef("test-task")))

	pod, err := makePod(taskRun, simpleTask)
	if err != nil {
		t.Fatalf("MakePod: %v", err)
	}
	taskRun.Status = v1alpha1.TaskRunStatus{
		TaskRunStatusFields: v1alpha1.TaskRunStatusFields{
			PodName: pod.Name,
		},
	}
	d := test.Data{
		TaskRuns: []*v1alpha1.TaskRun{taskRun},
		Tasks:    []*v1alpha1.Task{simpleTask},
		Pods:     []*corev1.Pod{pod},
	}

	testAssets, cancel := getTaskRunController(t, d)
	defer cancel()
	c := testAssets.Controller
	clients := testAssets.Clients

	if err := c.Reconciler.Reconcile(context.Background(), getRunName(taskRun)); err != nil {
		t.Fatalf("Unexpected error when Reconcile() : %v", err)
	}
	newTr, err := clients.Pipeline.TektonV1alpha1().TaskRuns(taskRun.Namespace).Get(taskRun.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Expected TaskRun %s to exist but instead got error when getting it: %v", taskRun.Name, err)
	}
	if d := cmp.Diff(&apis.Condition{
		Type:    apis.ConditionSucceeded,
		Status:  corev1.ConditionUnknown,
		Reason:  "Running",
		Message: "Not all Steps in the Task have finished executing",
	}, newTr.Status.GetCondition(apis.ConditionSucceeded), ignoreLastTransitionTime); d != "" {
		t.Fatalf("Did not get expected condition (-want, +got): %v", d)
	}

	// update pod status and trigger reconcile : build is completed
	pod.Status = corev1.PodStatus{
		Phase: corev1.PodSucceeded,
	}
	if _, err := clients.Kube.CoreV1().Pods(taskRun.Namespace).UpdateStatus(pod); err != nil {
		t.Errorf("Unexpected error while updating build: %v", err)
	}
	if err := c.Reconciler.Reconcile(context.Background(), getRunName(taskRun)); err != nil {
		t.Fatalf("Unexpected error when Reconcile(): %v", err)
	}

	newTr, err = clients.Pipeline.TektonV1alpha1().TaskRuns(taskRun.Namespace).Get(taskRun.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Unexpected error fetching taskrun: %v", err)
	}
	if d := cmp.Diff(&apis.Condition{
		Type:    apis.ConditionSucceeded,
		Status:  corev1.ConditionTrue,
		Reason:  podconvert.ReasonSucceeded,
		Message: "All Steps have completed executing",
	}, newTr.Status.GetCondition(apis.ConditionSucceeded), ignoreLastTransitionTime); d != "" {
		t.Errorf("Did not get expected condition (-want, +got): %v", d)
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
	), tb.TaskRunStatus(tb.StatusCondition(*taskSt)))
	d := test.Data{
		TaskRuns: []*v1alpha1.TaskRun{
			taskRun,
		},
		Tasks: []*v1alpha1.Task{simpleTask},
	}

	testAssets, cancel := getTaskRunController(t, d)
	defer cancel()
	c := testAssets.Controller
	clients := testAssets.Clients

	if err := c.Reconciler.Reconcile(context.Background(), getRunName(taskRun)); err != nil {
		t.Fatalf("Unexpected error when reconciling completed TaskRun : %v", err)
	}
	newTr, err := clients.Pipeline.TektonV1alpha1().TaskRuns(taskRun.Namespace).Get(taskRun.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Expected completed TaskRun %s to exist but instead got error when getting it: %v", taskRun.Name, err)
	}
	if d := cmp.Diff(taskSt, newTr.Status.GetCondition(apis.ConditionSucceeded), ignoreLastTransitionTime); d != "" {
		t.Fatalf("Did not get expected condition (-want, +got): %v", d)
	}
}

func TestReconcileOnCancelledTaskRun(t *testing.T) {
	taskRun := tb.TaskRun("test-taskrun-run-cancelled", "foo", tb.TaskRunSpec(
		tb.TaskRunTaskRef(simpleTask.Name),
		tb.TaskRunCancelled,
	), tb.TaskRunStatus(tb.StatusCondition(apis.Condition{
		Type:   apis.ConditionSucceeded,
		Status: corev1.ConditionUnknown,
	})))
	d := test.Data{
		TaskRuns: []*v1alpha1.TaskRun{taskRun},
		Tasks:    []*v1alpha1.Task{simpleTask},
	}

	testAssets, cancel := getTaskRunController(t, d)
	defer cancel()
	c := testAssets.Controller
	clients := testAssets.Clients

	if err := c.Reconciler.Reconcile(context.Background(), getRunName(taskRun)); err != nil {
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
		t.Fatalf("Did not get expected condition (-want, +got): %v", d)
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
				tb.TaskRunStatus(tb.StatusCondition(apis.Condition{
					Type:   apis.ConditionSucceeded,
					Status: corev1.ConditionUnknown}),
					tb.TaskRunStartTime(time.Now().Add(-15*time.Second)))),

			expectedStatus: &apis.Condition{
				Type:    apis.ConditionSucceeded,
				Status:  corev1.ConditionFalse,
				Reason:  "TaskRunTimeout",
				Message: `TaskRun "test-taskrun-timeout" failed to finish within "10s"`,
			},
		}, {
			taskRun: tb.TaskRun("test-taskrun-default-timeout-60-minutes", "foo",
				tb.TaskRunSpec(
					tb.TaskRunTaskRef(simpleTask.Name),
				),
				tb.TaskRunStatus(tb.StatusCondition(apis.Condition{
					Type:   apis.ConditionSucceeded,
					Status: corev1.ConditionUnknown}),
					tb.TaskRunStartTime(time.Now().Add(-61*time.Minute)))),

			expectedStatus: &apis.Condition{
				Type:    apis.ConditionSucceeded,
				Status:  corev1.ConditionFalse,
				Reason:  "TaskRunTimeout",
				Message: `TaskRun "test-taskrun-default-timeout-60-minutes" failed to finish within "1h0m0s"`,
			},
		}, {
			taskRun: tb.TaskRun("test-taskrun-nil-timeout-default-60-minutes", "foo",
				tb.TaskRunSpec(
					tb.TaskRunTaskRef(simpleTask.Name),
					tb.TaskRunNilTimeout,
				),
				tb.TaskRunStatus(tb.StatusCondition(apis.Condition{
					Type:   apis.ConditionSucceeded,
					Status: corev1.ConditionUnknown}),
					tb.TaskRunStartTime(time.Now().Add(-61*time.Minute)))),

			expectedStatus: &apis.Condition{
				Type:    apis.ConditionSucceeded,
				Status:  corev1.ConditionFalse,
				Reason:  "TaskRunTimeout",
				Message: `TaskRun "test-taskrun-nil-timeout-default-60-minutes" failed to finish within "1h0m0s"`,
			},
		}}

	for _, tc := range testcases {
		d := test.Data{
			TaskRuns: []*v1alpha1.TaskRun{tc.taskRun},
			Tasks:    []*v1alpha1.Task{simpleTask},
		}
		testAssets, cancel := getTaskRunController(t, d)
		defer cancel()
		c := testAssets.Controller
		clients := testAssets.Clients

		if err := c.Reconciler.Reconcile(context.Background(), getRunName(tc.taskRun)); err != nil {
			t.Fatalf("Unexpected error when reconciling completed TaskRun : %v", err)
		}
		newTr, err := clients.Pipeline.TektonV1alpha1().TaskRuns(tc.taskRun.Namespace).Get(tc.taskRun.Name, metav1.GetOptions{})
		if err != nil {
			t.Fatalf("Expected completed TaskRun %s to exist but instead got error when getting it: %v", tc.taskRun.Name, err)
		}
		condition := newTr.Status.GetCondition(apis.ConditionSucceeded)
		if d := cmp.Diff(tc.expectedStatus, condition, ignoreLastTransitionTime); d != "" {
			t.Fatalf("Did not get expected condition (-want, +got): %v", d)
		}
	}
}

func TestHandlePodCreationError(t *testing.T) {
	taskRun := tb.TaskRun("test-taskrun-pod-creation-failed", "foo", tb.TaskRunSpec(
		tb.TaskRunTaskRef(simpleTask.Name),
	), tb.TaskRunStatus(
		tb.TaskRunStartTime(time.Now()),
		tb.StatusCondition(apis.Condition{
			Type:   apis.ConditionSucceeded,
			Status: corev1.ConditionUnknown,
		}),
	))
	d := test.Data{
		TaskRuns: []*v1alpha1.TaskRun{taskRun},
		Tasks:    []*v1alpha1.Task{simpleTask},
	}
	testAssets, cancel := getTaskRunController(t, d)
	defer cancel()
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
	}{{
		description:    "exceeded quota errors are surfaced in taskrun condition but do not fail taskrun",
		err:            k8sapierrors.NewForbidden(k8sruntimeschema.GroupResource{Group: "foo", Resource: "bar"}, "baz", errors.New("exceeded quota")),
		expectedType:   apis.ConditionSucceeded,
		expectedStatus: corev1.ConditionUnknown,
		expectedReason: podconvert.ReasonExceededResourceQuota,
	}, {
		description:    "errors other than exceeded quota fail the taskrun",
		err:            errors.New("this is a fatal error"),
		expectedType:   apis.ConditionSucceeded,
		expectedStatus: corev1.ConditionFalse,
		expectedReason: podconvert.ReasonCouldntGetTask,
	}}
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

func TestReconcileCloudEvents(t *testing.T) {

	taskRunWithNoCEResources := tb.TaskRun("test-taskrun-no-ce-resources", "foo",
		tb.TaskRunSpec(
			tb.TaskRunTaskRef(simpleTask.Name, tb.TaskRefAPIVersion("a1")),
		))
	taskRunWithTwoCEResourcesNoInit := tb.TaskRun("test-taskrun-two-ce-resources-no-init", "foo",
		tb.TaskRunSpec(
			tb.TaskRunTaskRef(twoOutputsTask.Name),
			tb.TaskRunOutputs(
				tb.TaskRunOutputsResource(cloudEventResource.Name, tb.TaskResourceBindingRef(cloudEventResource.Name)),
				tb.TaskRunOutputsResource(anotherCloudEventResource.Name, tb.TaskResourceBindingRef(anotherCloudEventResource.Name)),
			),
		),
	)
	taskRunWithTwoCEResourcesInit := tb.TaskRun("test-taskrun-two-ce-resources-init", "foo",
		tb.TaskRunSpec(
			tb.TaskRunTaskRef(twoOutputsTask.Name),
			tb.TaskRunOutputs(
				tb.TaskRunOutputsResource(cloudEventResource.Name, tb.TaskResourceBindingRef(cloudEventResource.Name)),
				tb.TaskRunOutputsResource(anotherCloudEventResource.Name, tb.TaskResourceBindingRef(anotherCloudEventResource.Name)),
			),
		),
		tb.TaskRunStatus(
			tb.TaskRunCloudEvent(cloudEventTarget1, "", 0, v1alpha1.CloudEventConditionUnknown),
			tb.TaskRunCloudEvent(cloudEventTarget2, "", 0, v1alpha1.CloudEventConditionUnknown),
		),
	)
	taskRunWithCESucceded := tb.TaskRun("test-taskrun-ce-succeeded", "foo",
		tb.TaskRunSelfLink("/task/1234"),
		tb.TaskRunSpec(
			tb.TaskRunTaskRef(twoOutputsTask.Name),
			tb.TaskRunOutputs(
				tb.TaskRunOutputsResource(cloudEventResource.Name, tb.TaskResourceBindingRef(cloudEventResource.Name)),
				tb.TaskRunOutputsResource(anotherCloudEventResource.Name, tb.TaskResourceBindingRef(anotherCloudEventResource.Name)),
			),
		),
		tb.TaskRunStatus(
			tb.StatusCondition(apis.Condition{
				Type:   apis.ConditionSucceeded,
				Status: corev1.ConditionTrue,
			}),
			tb.TaskRunCloudEvent(cloudEventTarget1, "", 0, v1alpha1.CloudEventConditionUnknown),
			tb.TaskRunCloudEvent(cloudEventTarget2, "", 0, v1alpha1.CloudEventConditionUnknown),
		),
	)
	taskRunWithCEFailed := tb.TaskRun("test-taskrun-ce-failed", "foo",
		tb.TaskRunSelfLink("/task/1234"),
		tb.TaskRunSpec(
			tb.TaskRunTaskRef(twoOutputsTask.Name),
			tb.TaskRunOutputs(
				tb.TaskRunOutputsResource(cloudEventResource.Name, tb.TaskResourceBindingRef(cloudEventResource.Name)),
				tb.TaskRunOutputsResource(anotherCloudEventResource.Name, tb.TaskResourceBindingRef(anotherCloudEventResource.Name)),
			),
		),
		tb.TaskRunStatus(
			tb.StatusCondition(apis.Condition{
				Type:   apis.ConditionSucceeded,
				Status: corev1.ConditionFalse,
			}),
			tb.TaskRunCloudEvent(cloudEventTarget1, "", 0, v1alpha1.CloudEventConditionUnknown),
			tb.TaskRunCloudEvent(cloudEventTarget2, "", 0, v1alpha1.CloudEventConditionUnknown),
		),
	)
	taskRunWithCESuccededOneAttempt := tb.TaskRun("test-taskrun-ce-succeeded-one-attempt", "foo",
		tb.TaskRunSelfLink("/task/1234"),
		tb.TaskRunSpec(
			tb.TaskRunTaskRef(twoOutputsTask.Name),
			tb.TaskRunOutputs(
				tb.TaskRunOutputsResource(cloudEventResource.Name, tb.TaskResourceBindingRef(cloudEventResource.Name)),
				tb.TaskRunOutputsResource(anotherCloudEventResource.Name, tb.TaskResourceBindingRef(anotherCloudEventResource.Name)),
			),
		),
		tb.TaskRunStatus(
			tb.StatusCondition(apis.Condition{
				Type:   apis.ConditionSucceeded,
				Status: corev1.ConditionTrue,
			}),
			tb.TaskRunCloudEvent(cloudEventTarget1, "", 1, v1alpha1.CloudEventConditionUnknown),
			tb.TaskRunCloudEvent(cloudEventTarget2, "fakemessage", 0, v1alpha1.CloudEventConditionUnknown),
		),
	)
	taskruns := []*v1alpha1.TaskRun{
		taskRunWithNoCEResources, taskRunWithTwoCEResourcesNoInit,
		taskRunWithTwoCEResourcesInit, taskRunWithCESucceded, taskRunWithCEFailed,
		taskRunWithCESuccededOneAttempt,
	}

	d := test.Data{
		TaskRuns:          taskruns,
		Tasks:             []*v1alpha1.Task{simpleTask, twoOutputsTask},
		ClusterTasks:      []*v1alpha1.ClusterTask{},
		PipelineResources: []*v1alpha1.PipelineResource{cloudEventResource, anotherCloudEventResource},
	}
	for _, tc := range []struct {
		name            string
		taskRun         *v1alpha1.TaskRun
		wantCloudEvents []v1alpha1.CloudEventDelivery
	}{{
		name:            "no-ce-resources",
		taskRun:         taskRunWithNoCEResources,
		wantCloudEvents: taskRunWithNoCEResources.Status.CloudEvents,
	}, {
		name:    "ce-resources-no-init",
		taskRun: taskRunWithTwoCEResourcesNoInit,
		wantCloudEvents: tb.TaskRun("want", "foo", tb.TaskRunStatus(
			tb.TaskRunCloudEvent(cloudEventTarget1, "", 0, v1alpha1.CloudEventConditionUnknown),
			tb.TaskRunCloudEvent(cloudEventTarget2, "", 0, v1alpha1.CloudEventConditionUnknown),
		)).Status.CloudEvents,
	}, {
		name:    "ce-resources-init",
		taskRun: taskRunWithTwoCEResourcesInit,
		wantCloudEvents: tb.TaskRun("want2", "foo", tb.TaskRunStatus(
			tb.TaskRunCloudEvent(cloudEventTarget1, "", 0, v1alpha1.CloudEventConditionUnknown),
			tb.TaskRunCloudEvent(cloudEventTarget2, "", 0, v1alpha1.CloudEventConditionUnknown),
		)).Status.CloudEvents,
	}, {
		name:    "ce-resources-init-task-successful",
		taskRun: taskRunWithCESucceded,
		wantCloudEvents: tb.TaskRun("want3", "foo", tb.TaskRunStatus(
			tb.TaskRunCloudEvent(cloudEventTarget1, "", 1, v1alpha1.CloudEventConditionSent),
			tb.TaskRunCloudEvent(cloudEventTarget2, "", 1, v1alpha1.CloudEventConditionSent),
		)).Status.CloudEvents,
	}, {
		name:    "ce-resources-init-task-failed",
		taskRun: taskRunWithCEFailed,
		wantCloudEvents: tb.TaskRun("want4", "foo", tb.TaskRunStatus(
			tb.TaskRunCloudEvent(cloudEventTarget1, "", 1, v1alpha1.CloudEventConditionSent),
			tb.TaskRunCloudEvent(cloudEventTarget2, "", 1, v1alpha1.CloudEventConditionSent),
		)).Status.CloudEvents,
	}, {
		name:    "ce-resources-init-task-successful-one-attempt",
		taskRun: taskRunWithCESuccededOneAttempt,
		wantCloudEvents: tb.TaskRun("want5", "foo", tb.TaskRunStatus(
			tb.TaskRunCloudEvent(cloudEventTarget1, "", 1, v1alpha1.CloudEventConditionUnknown),
			tb.TaskRunCloudEvent(cloudEventTarget2, "fakemessage", 1, v1alpha1.CloudEventConditionSent),
		)).Status.CloudEvents,
	}} {
		t.Run(tc.name, func(t *testing.T) {
			names.TestingSeed()
			testAssets, cancel := getTaskRunController(t, d)
			defer cancel()
			c := testAssets.Controller
			clients := testAssets.Clients

			saName := tc.taskRun.Spec.ServiceAccountName
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

			if err := c.Reconciler.Reconcile(context.Background(), getRunName(tc.taskRun)); err != nil {
				t.Errorf("expected no error. Got error %v", err)
			}
			namespace, name, err := cache.SplitMetaNamespaceKey(tc.taskRun.Name)
			if err != nil {
				t.Errorf("Invalid resource key: %v", err)
			}

			tr, err := clients.Pipeline.TektonV1alpha1().TaskRuns(namespace).Get(name, metav1.GetOptions{})
			if err != nil {
				t.Fatalf("getting updated taskrun: %v", err)
			}
			opts := cloudevent.GetCloudEventDeliveryCompareOptions()
			t.Log(tr.Status.CloudEvents)
			if diff := cmp.Diff(tc.wantCloudEvents, tr.Status.CloudEvents, opts...); diff != "" {
				t.Errorf("Unexpected status of cloud events (-want +got) = %s", diff)
			}
		})
	}
}

func TestUpdateTaskRunResourceResult(t *testing.T) {
	for _, c := range []struct {
		desc          string
		podStatus     corev1.PodStatus
		taskRunStatus *v1alpha1.TaskRunStatus
		want          []v1alpha1.PipelineResourceResult
	}{{
		desc: "image resource updated",
		podStatus: corev1.PodStatus{
			ContainerStatuses: []corev1.ContainerStatus{{
				State: corev1.ContainerState{
					Terminated: &corev1.ContainerStateTerminated{
						Message: `[{"key":"digest","value":"sha256:1234","resourceRef":{"name":"source-image"}}]`,
					},
				},
			}},
		},
		want: []v1alpha1.PipelineResourceResult{{
			Key:         "digest",
			Value:       "sha256:1234",
			ResourceRef: v1alpha1.PipelineResourceRef{Name: "source-image"},
		}},
	}} {
		t.Run(c.desc, func(t *testing.T) {
			names.TestingSeed()
			tr := &v1alpha1.TaskRun{}
			tr.Status.SetCondition(&apis.Condition{
				Type:   apis.ConditionSucceeded,
				Status: corev1.ConditionTrue,
			})
			if err := updateTaskRunResourceResult(tr, c.podStatus); err != nil {
				t.Errorf("updateTaskRunResourceResult: %s", err)
			}
			if d := cmp.Diff(c.want, tr.Status.ResourcesResult); d != "" {
				t.Errorf("updateTaskRunResourceResult (-want, +got): %s", d)
			}
		})
	}
}

func TestUpdateTaskRunResult(t *testing.T) {
	for _, c := range []struct {
		desc          string
		podStatus     corev1.PodStatus
		taskRunStatus *v1alpha1.TaskRunStatus
		wantResults   []v1alpha1.TaskRunResult
		want          []v1alpha1.PipelineResourceResult
	}{{
		desc: "test result with pipeline result",
		podStatus: corev1.PodStatus{
			ContainerStatuses: []corev1.ContainerStatus{{
				State: corev1.ContainerState{
					Terminated: &corev1.ContainerStateTerminated{
						Message: `[{"key":"resultName","value":"resultValue", "type": "TaskRunResult"}, {"key":"digest","value":"sha256:1234","resourceRef":{"name":"source-image"}, "type": "PipelineResourceResult"}]`,
					},
				},
			}},
		},
		wantResults: []v1alpha1.TaskRunResult{{
			Name:  "resultName",
			Value: "resultValue",
		}},
		want: []v1alpha1.PipelineResourceResult{{
			Key:         "digest",
			Value:       "sha256:1234",
			ResourceRef: v1alpha1.PipelineResourceRef{Name: "source-image"},
			ResultType:  "PipelineResourceResult",
		}},
	}} {
		t.Run(c.desc, func(t *testing.T) {
			names.TestingSeed()
			tr := &v1alpha1.TaskRun{}
			tr.Status.SetCondition(&apis.Condition{
				Type:   apis.ConditionSucceeded,
				Status: corev1.ConditionTrue,
			})
			if err := updateTaskRunResourceResult(tr, c.podStatus); err != nil {
				t.Errorf("updateTaskRunResourceResult: %s", err)
			}
			if d := cmp.Diff(c.wantResults, tr.Status.TaskRunResults); d != "" {
				t.Errorf("updateTaskRunResourceResult TaskRunResults (-want, +got): %s", d)
			}
			if d := cmp.Diff(c.want, tr.Status.ResourcesResult); d != "" {
				t.Errorf("updateTaskRunResourceResult ResourcesResult (-want, +got): %s", d)
			}
		})
	}
}
func TestUpdateTaskRunResult2(t *testing.T) {
	for _, c := range []struct {
		desc          string
		podStatus     corev1.PodStatus
		taskRunStatus *v1alpha1.TaskRunStatus
		wantResults   []v1alpha1.TaskRunResult
		want          []v1alpha1.PipelineResourceResult
	}{{
		desc: "test result with pipeline result - no result type",
		podStatus: corev1.PodStatus{
			ContainerStatuses: []corev1.ContainerStatus{{
				State: corev1.ContainerState{
					Terminated: &corev1.ContainerStateTerminated{
						Message: `[{"key":"resultName","value":"resultValue", "type": "TaskRunResult"}, {"key":"digest","value":"sha256:1234","resourceRef":{"name":"source-image"}}]`,
					},
				},
			}},
		},
		wantResults: []v1alpha1.TaskRunResult{{
			Name:  "resultName",
			Value: "resultValue",
		}},
		want: []v1alpha1.PipelineResourceResult{{
			Key:         "digest",
			Value:       "sha256:1234",
			ResourceRef: v1alpha1.PipelineResourceRef{Name: "source-image"},
		}},
	}} {
		t.Run(c.desc, func(t *testing.T) {
			names.TestingSeed()
			tr := &v1alpha1.TaskRun{}
			tr.Status.SetCondition(&apis.Condition{
				Type:   apis.ConditionSucceeded,
				Status: corev1.ConditionTrue,
			})
			if err := updateTaskRunResourceResult(tr, c.podStatus); err != nil {
				t.Errorf("updateTaskRunResourceResult: %s", err)
			}
			if d := cmp.Diff(c.wantResults, tr.Status.TaskRunResults); d != "" {
				t.Errorf("updateTaskRunResourceResult (-want, +got): %s", d)
			}
			if d := cmp.Diff(c.want, tr.Status.ResourcesResult); d != "" {
				t.Errorf("updateTaskRunResourceResult (-want, +got): %s", d)
			}
		})
	}
}
func TestUpdateTaskRunResultTwoResults(t *testing.T) {
	for _, c := range []struct {
		desc          string
		podStatus     corev1.PodStatus
		taskRunStatus *v1alpha1.TaskRunStatus
		want          []v1alpha1.TaskRunResult
	}{{
		desc: "two test results",
		podStatus: corev1.PodStatus{
			ContainerStatuses: []corev1.ContainerStatus{{
				State: corev1.ContainerState{
					Terminated: &corev1.ContainerStateTerminated{
						Message: `[{"key":"resultNameOne","value":"resultValueOne", "type": "TaskRunResult"},{"key":"resultNameTwo","value":"resultValueTwo", "type": "TaskRunResult"}]`,
					},
				},
			}},
		},
		want: []v1alpha1.TaskRunResult{{
			Name:  "resultNameOne",
			Value: "resultValueOne",
		}, {
			Name:  "resultNameTwo",
			Value: "resultValueTwo",
		}},
	}} {
		t.Run(c.desc, func(t *testing.T) {
			names.TestingSeed()
			tr := &v1alpha1.TaskRun{}
			tr.Status.SetCondition(&apis.Condition{
				Type:   apis.ConditionSucceeded,
				Status: corev1.ConditionTrue,
			})
			if err := updateTaskRunResourceResult(tr, c.podStatus); err != nil {
				t.Errorf("updateTaskRunResourceResult: %s", err)
			}
			if d := cmp.Diff(c.want, tr.Status.TaskRunResults); d != "" {
				t.Errorf("updateTaskRunResourceResult (-want, +got): %s", d)
			}
		})
	}
}
func TestUpdateTaskRunResultWhenTaskFailed(t *testing.T) {
	for _, c := range []struct {
		desc          string
		podStatus     corev1.PodStatus
		taskRunStatus *v1alpha1.TaskRunStatus
		wantResults   []v1alpha1.TaskRunResult
		want          []v1alpha1.PipelineResourceResult
	}{{
		desc: "update task results when task fails",
		podStatus: corev1.PodStatus{
			ContainerStatuses: []corev1.ContainerStatus{{
				State: corev1.ContainerState{
					Terminated: &corev1.ContainerStateTerminated{
						Message: `[{"key":"resultName","value":"resultValue", "type": "TaskRunResult"}, {"name":"source-image","digest":"sha256:1234"}]`,
					},
				},
			}},
		},
		taskRunStatus: &v1alpha1.TaskRunStatus{
			Status: duckv1beta1.Status{Conditions: []apis.Condition{{
				Type:   apis.ConditionSucceeded,
				Status: corev1.ConditionFalse,
			}}},
		},
		wantResults: nil,
		want:        nil,
	}} {
		t.Run(c.desc, func(t *testing.T) {
			names.TestingSeed()
			if d := cmp.Diff(c.want, c.taskRunStatus.ResourcesResult); d != "" {
				t.Errorf("updateTaskRunResourceResult resources (-want, +got): %s", d)
			}
			if d := cmp.Diff(c.wantResults, c.taskRunStatus.TaskRunResults); d != "" {
				t.Errorf("updateTaskRunResourceResult results (-want, +got): %s", d)
			}
		})
	}
}
func TestUpdateTaskRunResourceResult_Errors(t *testing.T) {
	for _, c := range []struct {
		desc          string
		podStatus     corev1.PodStatus
		taskRunStatus *v1alpha1.TaskRunStatus
		want          []v1alpha1.PipelineResourceResult
	}{{
		desc: "image resource exporter with malformed json output",
		podStatus: corev1.PodStatus{
			ContainerStatuses: []corev1.ContainerStatus{{
				State: corev1.ContainerState{
					Terminated: &corev1.ContainerStateTerminated{
						Message: `MALFORMED JSON{"digest":"sha256:1234"}`,
					},
				},
			}},
		},
		taskRunStatus: &v1alpha1.TaskRunStatus{
			Status: duckv1beta1.Status{Conditions: []apis.Condition{{
				Type:   apis.ConditionSucceeded,
				Status: corev1.ConditionTrue,
			}}},
		},
		want: nil,
	}} {
		t.Run(c.desc, func(t *testing.T) {
			names.TestingSeed()
			if err := updateTaskRunResourceResult(&v1alpha1.TaskRun{Status: *c.taskRunStatus}, c.podStatus); err == nil {
				t.Error("Expected error, got nil")
			}
			if d := cmp.Diff(c.want, c.taskRunStatus.ResourcesResult); d != "" {
				t.Errorf("updateTaskRunResourceResult (-want, +got): %s", d)
			}
		})
	}
}

func TestReconcile_Single_SidecarState(t *testing.T) {
	runningState := corev1.ContainerStateRunning{StartedAt: metav1.Time{Time: time.Now()}}
	taskRun := tb.TaskRun("test-taskrun-sidecars", "foo",
		tb.TaskRunSpec(
			tb.TaskRunTaskRef(taskSidecar.Name),
		),
		tb.TaskRunStatus(
			tb.SidecarState(
				tb.SidecarStateName("sidecar"),
				tb.SidecarStateImageID("image-id"),
				tb.SidecarStateContainerName("sidecar-sidecar"),
				tb.SetSidecarStateRunning(runningState),
			),
		),
	)

	d := test.Data{
		TaskRuns: []*v1alpha1.TaskRun{taskRun},
		Tasks:    []*v1alpha1.Task{taskSidecar},
	}

	testAssets, cancel := getTaskRunController(t, d)
	defer cancel()
	clients := testAssets.Clients

	if err := testAssets.Controller.Reconciler.Reconcile(context.Background(), getRunName(taskRun)); err != nil {
		t.Errorf("expected no error reconciling valid TaskRun but got %v", err)
	}

	getTaskRun, err := clients.Pipeline.TektonV1alpha1().TaskRuns(taskRun.Namespace).Get(taskRun.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Expected completed TaskRun %s to exist but instead got error when getting it: %v", taskRun.Name, err)
	}

	expected := v1alpha1.SidecarState{
		Name:          "sidecar",
		ImageID:       "image-id",
		ContainerName: "sidecar-sidecar",
		ContainerState: corev1.ContainerState{
			Running: &runningState,
		},
	}

	if c := cmp.Diff(expected, getTaskRun.Status.Sidecars[0]); c != "" {
		t.Errorf("TestReconcile_Single_SidecarState (-want, +got): %s", c)
	}
}

func TestReconcile_Multiple_SidecarStates(t *testing.T) {
	runningState := corev1.ContainerStateRunning{StartedAt: metav1.Time{Time: time.Now()}}
	waitingState := corev1.ContainerStateWaiting{Reason: "PodInitializing"}
	taskRun := tb.TaskRun("test-taskrun-sidecars", "foo",
		tb.TaskRunSpec(
			tb.TaskRunTaskRef(taskMultipleSidecars.Name),
		),
		tb.TaskRunStatus(
			tb.SidecarState(
				tb.SidecarStateName("sidecar1"),
				tb.SidecarStateImageID("image-id"),
				tb.SidecarStateContainerName("sidecar-sidecar1"),
				tb.SetSidecarStateRunning(runningState),
			),
		),
		tb.TaskRunStatus(
			tb.SidecarState(
				tb.SidecarStateName("sidecar2"),
				tb.SidecarStateImageID("image-id"),
				tb.SidecarStateContainerName("sidecar-sidecar2"),
				tb.SetSidecarStateWaiting(waitingState),
			),
		),
	)

	d := test.Data{
		TaskRuns: []*v1alpha1.TaskRun{taskRun},
		Tasks:    []*v1alpha1.Task{taskMultipleSidecars},
	}

	testAssets, cancel := getTaskRunController(t, d)
	defer cancel()
	clients := testAssets.Clients

	if err := testAssets.Controller.Reconciler.Reconcile(context.Background(), getRunName(taskRun)); err != nil {
		t.Errorf("expected no error reconciling valid TaskRun but got %v", err)
	}

	getTaskRun, err := clients.Pipeline.TektonV1alpha1().TaskRuns(taskRun.Namespace).Get(taskRun.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Expected completed TaskRun %s to exist but instead got error when getting it: %v", taskRun.Name, err)
	}

	expected := []v1alpha1.SidecarState{
		{
			Name:          "sidecar1",
			ImageID:       "image-id",
			ContainerName: "sidecar-sidecar1",
			ContainerState: corev1.ContainerState{
				Running: &runningState,
			},
		},
		{
			Name:          "sidecar2",
			ImageID:       "image-id",
			ContainerName: "sidecar-sidecar2",
			ContainerState: corev1.ContainerState{
				Waiting: &waitingState,
			},
		},
	}

	for i, sc := range getTaskRun.Status.Sidecars {
		if c := cmp.Diff(expected[i], sc); c != "" {
			t.Errorf("TestReconcile_Multiple_SidecarStates sidecar%d (-want, +got): %s", i+1, c)
		}
	}
}
