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
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/go-containerregistry/pkg/registry"
	"github.com/sigstore/sigstore/pkg/signature"
	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/pod"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	resolutionv1beta1 "github.com/tektoncd/pipeline/pkg/apis/resolution/v1beta1"
	resolutionutil "github.com/tektoncd/pipeline/pkg/internal/resolution"
	podconvert "github.com/tektoncd/pipeline/pkg/pod"
	"github.com/tektoncd/pipeline/pkg/reconciler/events/cloudevent"
	"github.com/tektoncd/pipeline/pkg/reconciler/events/k8sevent"
	"github.com/tektoncd/pipeline/pkg/reconciler/taskrun/resources"
	ttesting "github.com/tektoncd/pipeline/pkg/reconciler/testing"
	"github.com/tektoncd/pipeline/pkg/reconciler/volumeclaim"
	resolutioncommon "github.com/tektoncd/pipeline/pkg/resolution/common"
	remoteresource "github.com/tektoncd/pipeline/pkg/resolution/resource"
	"github.com/tektoncd/pipeline/pkg/trustedresources"
	"github.com/tektoncd/pipeline/pkg/trustedresources/verifier"
	"github.com/tektoncd/pipeline/pkg/workspace"
	"github.com/tektoncd/pipeline/test"
	"github.com/tektoncd/pipeline/test/diff"
	"github.com/tektoncd/pipeline/test/names"
	"github.com/tektoncd/pipeline/test/parse"
	"go.opentelemetry.io/otel/trace"
	corev1 "k8s.io/api/core/v1"
	k8sapierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8sruntimeschema "k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	fakekubeclientset "k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/record"
	clock "k8s.io/utils/clock/testing"
	"knative.dev/pkg/apis"
	cminformer "knative.dev/pkg/configmap/informer"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/kmeta"
	"knative.dev/pkg/logging"
	pkgreconciler "knative.dev/pkg/reconciler"
	"knative.dev/pkg/system"
	_ "knative.dev/pkg/system/testing" // Setup system.Namespace()
	"sigs.k8s.io/yaml"
)

const (
	entrypointLocation = "/tekton/bin/entrypoint"
	workspaceDir       = "/workspace"
	currentAPIVersion  = "tekton.dev/v1"
)

var (
	defaultActiveDeadlineSeconds = int64(config.DefaultTimeoutMinutes * 60 * 1.5)
	images                       = pipeline.Images{
		EntrypointImage: "override-with-entrypoint:latest",
		NopImage:        "override-with-nop:latest",
		ShellImage:      "busybox",
	}
	now                      = time.Date(2022, time.January, 1, 0, 0, 0, 0, time.UTC)
	ignoreLastTransitionTime = cmpopts.IgnoreFields(apis.Condition{}, "LastTransitionTime.Inner.Time")
	// Pods are created with a random 5-character suffix that we want to
	// ignore in our diffs.
	ignoreRandomPodNameSuffix = cmp.FilterPath(func(path cmp.Path) bool {
		return path.GoString() == "{v1.ObjectMeta}.Name"
	}, cmp.Comparer(func(name1, name2 string) bool {
		return name1[:len(name1)-5] == name2[:len(name2)-5]
	}))
	ignoreStartTime           = cmpopts.IgnoreFields(v1.TaskRunStatusFields{}, "StartTime")
	ignoreCompletionTime      = cmpopts.IgnoreFields(v1.TaskRunStatusFields{}, "CompletionTime")
	ignoreObjectMeta          = cmpopts.IgnoreFields(metav1.ObjectMeta{}, "Labels", "ResourceVersion", "Annotations")
	ignoreStatusTaskSpec      = cmpopts.IgnoreFields(v1.TaskRunStatusFields{}, "TaskSpec")
	ignoreTaskRunStatusFields = cmpopts.IgnoreFields(v1.TaskRunStatusFields{}, "Steps", "Sidecars")

	resourceQuantityCmp = cmp.Comparer(func(x, y resource.Quantity) bool {
		return x.Cmp(y) == 0
	})

	ignoreEnvVarOrdering = cmpopts.SortSlices(func(x, y corev1.EnvVar) bool { return x.Name < y.Name })
	volumeSort           = cmpopts.SortSlices(func(i, j corev1.Volume) bool { return i.Name < j.Name })
	volumeMountSort      = cmpopts.SortSlices(func(i, j corev1.VolumeMount) bool { return i.Name < j.Name })

	simpleStep = v1.Step{
		Name:    "simple-step",
		Image:   "foo",
		Command: []string{"/mycmd"},
	}
	simpleTask = &v1.Task{
		ObjectMeta: objectMeta("test-task", "foo"),
		Spec: v1.TaskSpec{
			Steps: []v1.Step{simpleStep},
		},
	}
	simpleTypedTask = &v1.Task{
		ObjectMeta: objectMeta("test-task", "foo"),
		TypeMeta: metav1.TypeMeta{
			APIVersion: "tekton.dev/v1",
			Kind:       "Task",
		},
		Spec: v1.TaskSpec{
			Steps: []v1.Step{simpleStep},
		},
	}
	resultsTask = &v1.Task{
		ObjectMeta: objectMeta("test-results-task", "foo"),
		Spec: v1.TaskSpec{
			Steps: []v1.Step{simpleStep},
			Results: []v1.TaskResult{{
				Name: "aResult",
				Type: v1.ResultsTypeArray,
			}, {
				Name:       "objectResult",
				Type:       v1.ResultsTypeObject,
				Properties: map[string]v1.PropertySpec{"url": {Type: "string"}, "commit": {Type: "string"}},
			},
			},
		},
	}

	clustertask = &v1beta1.ClusterTask{
		ObjectMeta: metav1.ObjectMeta{Name: "test-cluster-task"},
		Spec: v1beta1.TaskSpec{
			Steps: []v1beta1.Step{{
				Name:    "simple-step",
				Image:   "foo",
				Command: []string{"/mycmd"},
			}},
		},
	}
	taskSidecar = &v1.Task{
		ObjectMeta: objectMeta("test-task-sidecar", "foo"),
		Spec: v1.TaskSpec{
			Sidecars: []v1.Sidecar{{
				Name:  "sidecar1",
				Image: "image-id",
			}},
		},
	}
	taskMultipleSidecars = &v1.Task{
		ObjectMeta: objectMeta("test-task-sidecar", "foo"),
		Spec: v1.TaskSpec{
			Sidecars: []v1.Sidecar{
				{
					Name:  "sidecar",
					Image: "image-id",
				},
				{
					Name:  "sidecar2",
					Image: "image-id",
				},
			},
		},
	}

	saTask = &v1.Task{
		ObjectMeta: objectMeta("test-with-sa", "foo"),
		Spec: v1.TaskSpec{
			Steps: []v1.Step{{
				Name:    "sa-step",
				Image:   "foo",
				Command: []string{"/mycmd"},
			}},
		},
	}

	templatedTask = &v1.Task{
		ObjectMeta: objectMeta("test-task-with-substitution", "foo"),
		Spec: v1.TaskSpec{
			Params: []v1.ParamSpec{
				{
					Name: "myarg",
					Type: v1.ParamTypeString,
				},
				{
					Name:    "myarghasdefault",
					Type:    v1.ParamTypeString,
					Default: v1.NewStructuredValues("dont see me"),
				},
				{
					Name:    "myarghasdefault2",
					Type:    v1.ParamTypeString,
					Default: v1.NewStructuredValues("thedefault"),
				},
				{
					Name: "configmapname",
					Type: v1.ParamTypeString,
				},
			},
			Steps: []v1.Step{
				{
					Image:   "myimage",
					Name:    "mycontainer",
					Command: []string{"/mycmd"},
					Args: []string{
						"--my-arg=$(inputs.params.myarg)",
						"--my-arg-with-default=$(inputs.params.myarghasdefault)",
						"--my-arg-with-default2=$(inputs.params.myarghasdefault2)",
						"--my-taskname-arg=$(context.task.name)",
						"--my-taskrun-arg=$(context.taskRun.name)",
					},
				},
				{
					Image:   "myotherimage",
					Name:    "myothercontainer",
					Command: []string{"/mycmd"},
					Args:    []string{"--my-other-arg=https://foo.git"},
				},
			},
			Volumes: []corev1.Volume{{
				Name: "volume-configmap",
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "$(inputs.params.configmapname)",
						},
					},
				},
			}},
		},
	}

	binVolume = corev1.Volume{
		Name: "tekton-internal-bin",
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	}
	internalStepsMount = corev1.VolumeMount{
		Name:      "tekton-internal-steps",
		MountPath: pipeline.StepsDir,
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
	stepsVolume = corev1.Volume{
		Name: "tekton-internal-steps",
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	}
)

const fakeVersion string = "unknown"

func placeToolsInitContainer(steps []string) corev1.Container {
	return corev1.Container{
		Command: append([]string{"/ko-app/entrypoint", "init", "/ko-app/entrypoint", entrypointLocation}, steps...),
		VolumeMounts: []corev1.VolumeMount{{
			MountPath: "/tekton/bin",
			Name:      "tekton-internal-bin",
		}, internalStepsMount},
		WorkingDir: "/",
		Name:       "prepare",
		Image:      "override-with-entrypoint:latest",
	}
}

var testClock = clock.NewFakePassiveClock(now)

func runVolume(i int) corev1.Volume {
	return corev1.Volume{
		Name: fmt.Sprintf("tekton-internal-run-%d", i),
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	}
}

func createServiceAccount(t *testing.T, assets test.Assets, name string, namespace string) {
	t.Helper()
	if name == "" {
		name = "default"
	}
	if _, err := assets.Clients.Kube.CoreV1().ServiceAccounts(namespace).Create(assets.Ctx, &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
}

func init() {
	os.Setenv("KO_DATA_PATH", "./testdata/")
}

func getRunName(tr *v1.TaskRun) string {
	return strings.Join([]string{tr.Namespace, tr.Name}, "/")
}

// getTaskRunController returns an instance of the TaskRun controller/reconciler that has been seeded with
// d, where d represents the state of the system (existing resources) needed for the test.
func getTaskRunController(t *testing.T, d test.Data) (test.Assets, func()) {
	t.Helper()
	names.TestingSeed()
	return initializeTaskRunControllerAssets(t, d, pipeline.Options{Images: images})
}

func initializeTaskRunControllerAssets(t *testing.T, d test.Data, opts pipeline.Options) (test.Assets, func()) {
	t.Helper()
	ctx, _ := ttesting.SetupFakeContext(t)
	ctx = ttesting.SetupFakeCloudClientContext(ctx, d.ExpectedCloudEventCount)
	ctx, cancel := context.WithCancel(ctx)
	test.EnsureConfigurationConfigMapsExist(&d)
	c, informers := test.SeedTestData(t, ctx, d)
	configMapWatcher := cminformer.NewInformedWatcher(c.Kube, system.Namespace())
	ctl := NewController(&opts, testClock, trace.NewNoopTracerProvider())(ctx, configMapWatcher)
	if err := configMapWatcher.Start(ctx.Done()); err != nil {
		t.Fatalf("error starting configmap watcher: %v", err)
	}

	if la, ok := ctl.Reconciler.(pkgreconciler.LeaderAware); ok {
		la.Promote(pkgreconciler.UniversalBucket(), func(pkgreconciler.Bucket, types.NamespacedName) {})
	}

	return test.Assets{
		Logger:     logging.FromContext(ctx),
		Controller: ctl,
		Clients:    c,
		Informers:  informers,
		Recorder:   controller.GetEventRecorder(ctx).(*record.FakeRecorder),
		Ctx:        ctx,
	}, cancel
}

func TestReconcile_ExplicitDefaultSA(t *testing.T) {
	taskRunSuccess := parse.MustParseV1TaskRun(t, `
metadata:
  name: test-taskrun-run-success
  namespace: foo
spec:
  taskRef:
    apiVersion: a1
    name: test-task
`)
	taskRunWithSaSuccess := parse.MustParseV1TaskRun(t, `
metadata:
  name: test-taskrun-with-sa-run-success
  namespace: foo
spec:
  serviceAccountName: test-sa
  taskRef:
    apiVersion: a1
    name: test-with-sa
`)
	taskruns := []*v1.TaskRun{taskRunSuccess, taskRunWithSaSuccess}
	defaultSAName := "pipelines"
	d := test.Data{
		TaskRuns: taskruns,
		Tasks:    []*v1.Task{simpleTask, saTask},
		ConfigMaps: []*corev1.ConfigMap{
			{
				ObjectMeta: metav1.ObjectMeta{Name: config.GetDefaultsConfigName(), Namespace: system.Namespace()},
				Data: map[string]string{
					"default-service-account":        defaultSAName,
					"default-timeout-minutes":        "60",
					"default-managed-by-label-value": "tekton-pipelines",
				},
			},
		},
	}
	for _, tc := range []struct {
		name    string
		taskRun *v1.TaskRun
		wantPod *corev1.Pod
	}{{
		name:    "success",
		taskRun: taskRunSuccess,
		wantPod: expectedPod("test-taskrun-run-success-pod", "test-task", "test-taskrun-run-success", "foo", defaultSAName, false, nil, []stepForExpectedPod{{
			image: "foo",
			name:  "simple-step",
			cmd:   "/mycmd",
		}}),
	}, {
		name:    "serviceaccount",
		taskRun: taskRunWithSaSuccess,
		wantPod: expectedPod("test-taskrun-with-sa-run-success-pod", "test-with-sa", "test-taskrun-with-sa-run-success", "foo", "test-sa", false, nil, []stepForExpectedPod{{
			image: "foo",
			name:  "sa-step",
			cmd:   "/mycmd",
		}}),
	}} {
		t.Run(tc.name, func(t *testing.T) {
			saName := tc.taskRun.Spec.ServiceAccountName
			if saName == "" {
				saName = defaultSAName
			}
			d.ServiceAccounts = append(d.ServiceAccounts, &corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name:      saName,
					Namespace: tc.taskRun.Namespace,
				},
			})
			testAssets, cancel := getTaskRunController(t, d)
			defer cancel()
			c := testAssets.Controller
			clients := testAssets.Clients

			if err := c.Reconciler.Reconcile(testAssets.Ctx, getRunName(tc.taskRun)); err == nil {
				t.Error("Wanted a wrapped requeue error, but got nil.")
			} else if ok, _ := controller.IsRequeueKey(err); !ok {
				t.Errorf("expected no error. Got error %v", err)
			}
			if len(clients.Kube.Actions()) == 0 {
				t.Errorf("Expected actions to be logged in the kubeclient, got none")
			}

			tr, err := clients.Pipeline.TektonV1().TaskRuns(tc.taskRun.Namespace).Get(testAssets.Ctx, tc.taskRun.Name, metav1.GetOptions{})
			if err != nil {
				t.Fatalf("getting updated taskrun: %v", err)
			}
			condition := tr.Status.GetCondition(apis.ConditionSucceeded)
			if condition == nil || condition.Status != corev1.ConditionUnknown {
				t.Errorf("Expected invalid TaskRun to have in progress status, but had %v", condition)
			}
			if condition != nil && condition.Reason != v1.TaskRunReasonRunning.String() {
				t.Errorf("Expected reason %q but was %s", v1.TaskRunReasonRunning.String(), condition.Reason)
			}

			if tr.Status.PodName == "" {
				t.Fatalf("Reconcile didn't set pod name")
			}

			pod, err := clients.Kube.CoreV1().Pods(tr.Namespace).Get(testAssets.Ctx, tr.Status.PodName, metav1.GetOptions{})
			if err != nil {
				t.Fatalf("Failed to fetch build pod: %v", err)
			}

			if d := cmp.Diff(tc.wantPod.ObjectMeta, pod.ObjectMeta, ignoreRandomPodNameSuffix); d != "" {
				t.Errorf("Pod metadata doesn't match %s", diff.PrintWantGot(d))
			}

			if d := cmp.Diff(tc.wantPod.Spec, pod.Spec, resourceQuantityCmp, volumeSort, volumeMountSort, ignoreEnvVarOrdering); d != "" {
				t.Errorf("Pod spec doesn't match, %s", diff.PrintWantGot(d))
			}
			if len(clients.Kube.Actions()) == 0 {
				t.Fatalf("Expected actions to be logged in the kubeclient, got none")
			}
		})
	}
}

// TestReconcile_CloudEvents runs reconcile with a cloud event sink configured
// to ensure that events are sent in different cases
func TestReconcile_CloudEvents(t *testing.T) {
	task := parse.MustParseV1Task(t, `
metadata:
  name: test-task
  namespace: foo
spec:
  steps:
  - command:
    - /mycmd
    env:
    - name: foo
      value: bar
    image: foo
    name: simple-step
`)
	taskRun := parse.MustParseV1TaskRun(t, `
metadata:
  name: test-taskrun-not-started
  namespace: foo
  selfLink: /test/taskrun1
spec:
  taskRef:
    name: test-task
`)
	d := test.Data{
		Tasks:    []*v1.Task{task},
		TaskRuns: []*v1.TaskRun{taskRun},
	}

	d.ConfigMaps = []*corev1.ConfigMap{
		{
			ObjectMeta: metav1.ObjectMeta{Name: config.GetDefaultsConfigName(), Namespace: system.Namespace()},
			Data: map[string]string{
				"default-cloud-events-sink": "http://synk:8080",
			},
		},
	}

	wantEvents := []string{
		"Normal Start",
		"Normal Running",
	}

	d.ExpectedCloudEventCount = len(wantEvents)

	testAssets, cancel := getTaskRunController(t, d)
	defer cancel()
	c := testAssets.Controller
	clients := testAssets.Clients
	saName := "default"
	if _, err := clients.Kube.CoreV1().ServiceAccounts(taskRun.Namespace).Create(testAssets.Ctx, &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      saName,
			Namespace: taskRun.Namespace,
		},
	}, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}

	if err := c.Reconciler.Reconcile(testAssets.Ctx, getRunName(taskRun)); err == nil {
		t.Error("Wanted a wrapped requeue error, but got nil.")
	} else if ok, _ := controller.IsRequeueKey(err); !ok {
		t.Errorf("expected no error. Got error %v", err)
	}
	if len(clients.Kube.Actions()) == 0 {
		t.Errorf("Expected actions to be logged in the kubeclient, got none")
	}

	tr, err := clients.Pipeline.TektonV1().TaskRuns(taskRun.Namespace).Get(testAssets.Ctx, taskRun.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("getting updated taskrun: %v", err)
	}
	condition := tr.Status.GetCondition(apis.ConditionSucceeded)
	if condition == nil || condition.Status != corev1.ConditionUnknown {
		t.Errorf("Expected fresh TaskRun to have in progress status, but had %v", condition)
	}
	if condition != nil && condition.Reason != v1.TaskRunReasonRunning.String() {
		t.Errorf("Expected reason %q but was %s", v1.TaskRunReasonRunning.String(), condition.Reason)
	}

	err = k8sevent.CheckEventsOrdered(t, testAssets.Recorder.Events, "reconcile-cloud-events", wantEvents)
	if !(err == nil) {
		t.Errorf(err.Error())
	}

	wantCloudEvents := []string{
		`(?s)dev.tekton.event.taskrun.started.v1.*test-taskrun-not-started`,
		`(?s)dev.tekton.event.taskrun.running.v1.*test-taskrun-not-started`,
	}
	ceClient := clients.CloudEvents.(cloudevent.FakeClient)
	ceClient.CheckCloudEventsUnordered(t, "reconcile-cloud-events", wantCloudEvents)
}

func TestReconcile(t *testing.T) {
	taskRunSuccess := parse.MustParseV1TaskRun(t, `
metadata:
  name: test-taskrun-run-success
  namespace: foo
spec:
  taskRef:
    apiVersion: a1
    name: test-task
`)
	taskRunWithSaSuccess := parse.MustParseV1TaskRun(t, `
metadata:
  name: test-taskrun-with-sa-run-success
  namespace: foo
spec:
  serviceAccountName: test-sa
  taskRef:
    apiVersion: a1
    name: test-with-sa
`)
	taskRunSubstitution := parse.MustParseV1TaskRun(t, `
metadata:
  name: test-taskrun-substitution
  namespace: foo
spec:
  params:
  - name: myarg
    value: foo
  - name: myarghasdefault
    value: bar
  - name: configmapname
    value: configbar
  taskRef:
    apiVersion: a1
    name: test-task-with-substitution
`)
	taskRunWithTaskSpec := parse.MustParseV1TaskRun(t, `
metadata:
  name: test-taskrun-with-taskspec
  namespace: foo
spec:
  params:
  - name: myarg
    value: foo
  taskSpec:
    params:
    - default: mydefault
      name: myarg
      type: string
    steps:
    - args:
      command:
      - /mycmd
      image: myimage
      name: mycontainer
`)

	taskRunWithClusterTask := parse.MustParseV1TaskRun(t, `
metadata:
  name: test-taskrun-with-cluster-task
  namespace: foo
spec:
  taskRef:
    kind: ClusterTask
    name: test-cluster-task
`)

	taskRunWithLabels := parse.MustParseV1TaskRun(t, `
metadata:
  labels:
    TaskRunLabel: TaskRunValue
    tekton.dev/taskRun: WillNotBeUsed
  name: test-taskrun-with-labels
  namespace: foo
spec:
  taskRef:
    name: test-task
`)

	taskRunWithAnnotations := parse.MustParseV1TaskRun(t, `
metadata:
  annotations:
    TaskRunAnnotation: TaskRunValue
  name: test-taskrun-with-annotations
  namespace: foo
spec:
  taskRef:
    name: test-task
`)

	taskRunWithPod := parse.MustParseV1TaskRun(t, `
metadata:
  name: test-taskrun-with-pod
  namespace: foo
spec:
  taskRef:
    name: test-task
status:
  podName: some-pod-abcdethat-no-longer-exists
`)

	taskRunWithCredentialsVariable := parse.MustParseV1TaskRun(t, `
metadata:
  name: test-taskrun-with-credentials-variable
  namespace: foo
spec:
  taskSpec:
    steps:
    - command:
      - /mycmd $(credentials.path)
      image: myimage
      name: mycontainer
`)

	// Set up a fake registry to push an image to.
	s := httptest.NewServer(registry.New())
	defer s.Close()
	u, err := url.Parse(s.URL)
	if err != nil {
		t.Fatal(err)
	}

	// Upload the simple task to the registry for our taskRunBundle TaskRun.
	ref, err := test.CreateImage(u.Host+"/"+simpleTypedTask.Name, simpleTypedTask)
	if err != nil {
		t.Fatalf("failed to upload image with simple task: %s", err.Error())
	}

	taskRunBundle := parse.MustParseV1TaskRun(t, fmt.Sprintf(`
metadata:
  name: test-taskrun-bundle
  namespace: foo
spec:
  taskRef:
    bundle: %s
    name: test-task
`, ref))

	taskruns := []*v1.TaskRun{
		taskRunSuccess, taskRunWithSaSuccess, taskRunSubstitution,
		taskRunWithTaskSpec, taskRunWithClusterTask,
		taskRunWithLabels, taskRunWithAnnotations, taskRunWithPod,
		taskRunWithCredentialsVariable, taskRunBundle,
	}

	d := test.Data{
		TaskRuns:     taskruns,
		Tasks:        []*v1.Task{simpleTask, saTask, templatedTask},
		ClusterTasks: []*v1beta1.ClusterTask{clustertask},
	}
	for _, tc := range []struct {
		name       string
		taskRun    *v1.TaskRun
		wantPod    *corev1.Pod
		wantEvents []string
	}{{
		name:    "success",
		taskRun: taskRunSuccess,
		wantEvents: []string{
			"Normal Started ",
			"Normal Running Not all Steps",
		},
		wantPod: expectedPod("test-taskrun-run-success-pod", "test-task", "test-taskrun-run-success", "foo", config.DefaultServiceAccountValue, false, nil, []stepForExpectedPod{{
			image: "foo",
			name:  "simple-step",
			cmd:   "/mycmd",
		}}),
	}, {
		name:    "serviceaccount",
		taskRun: taskRunWithSaSuccess,
		wantEvents: []string{
			"Normal Started ",
			"Normal Running Not all Steps",
		},
		wantPod: expectedPod("test-taskrun-with-sa-run-success-pod", "test-with-sa", "test-taskrun-with-sa-run-success", "foo", "test-sa", false, nil, []stepForExpectedPod{{
			image: "foo",
			name:  "sa-step",
			cmd:   "/mycmd",
		}}),
	}, {
		name:    "params",
		taskRun: taskRunSubstitution,
		wantEvents: []string{
			"Normal Started ",
			"Normal Running Not all Steps",
		},
		wantPod: expectedPod("test-taskrun-substitution-pod", "test-task-with-substitution", "test-taskrun-substitution", "foo", config.DefaultServiceAccountValue, false, []corev1.Volume{{
			Name: "volume-configmap",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "configbar",
					},
				},
			},
		}}, []stepForExpectedPod{
			{
				name:  "mycontainer",
				image: "myimage",
				cmd:   "/mycmd",
				args: []string{
					"--my-arg=foo",
					"--my-arg-with-default=bar",
					"--my-arg-with-default2=thedefault",
					"--my-taskname-arg=test-task-with-substitution",
					"--my-taskrun-arg=test-taskrun-substitution",
				},
			},
			{
				name:  "myothercontainer",
				image: "myotherimage",
				cmd:   "/mycmd",
				args:  []string{"--my-other-arg=https://foo.git"},
			},
		}),
	}, {
		name:    "taskrun-with-taskspec",
		taskRun: taskRunWithTaskSpec,
		wantEvents: []string{
			"Normal Started ",
			"Normal Running Not all Steps",
		},
		wantPod: expectedPod("test-taskrun-with-taskspec-pod", "", "test-taskrun-with-taskspec", "foo", config.DefaultServiceAccountValue, false, nil, []stepForExpectedPod{
			{
				name:  "mycontainer",
				image: "myimage",
				cmd:   "/mycmd",
			},
		}),
	}, {
		name:    "success-with-cluster-task",
		taskRun: taskRunWithClusterTask,
		wantEvents: []string{
			"Normal Started ",
			"Normal Running Not all Steps",
		},
		wantPod: expectedPod("test-taskrun-with-cluster-task-pod", "test-cluster-task", "test-taskrun-with-cluster-task", "foo", config.DefaultServiceAccountValue, true, nil, []stepForExpectedPod{{
			name:  "simple-step",
			image: "foo",
			cmd:   "/mycmd",
		}}),
	}, {
		name:    "taskrun-with-pod",
		taskRun: taskRunWithPod,
		wantEvents: []string{
			"Normal Started ",
			"Normal Running Not all Steps",
		},
		wantPod: expectedPod("test-taskrun-with-pod-pod", "test-task", "test-taskrun-with-pod", "foo", config.DefaultServiceAccountValue, false, nil, []stepForExpectedPod{{
			name:  "simple-step",
			image: "foo",
			cmd:   "/mycmd",
		}}),
	}, {
		name:    "taskrun-with-credentials-variable-default-tekton-creds",
		taskRun: taskRunWithCredentialsVariable,
		wantEvents: []string{
			"Normal Started ",
			"Normal Running Not all Steps",
		},
		wantPod: expectedPod("test-taskrun-with-credentials-variable-pod", "", "test-taskrun-with-credentials-variable", "foo", config.DefaultServiceAccountValue, false, nil, []stepForExpectedPod{{
			name:  "mycontainer",
			image: "myimage",
			cmd:   "/mycmd /tekton/creds",
		}}),
	}, {
		name:    "remote-task",
		taskRun: taskRunBundle,
		wantEvents: []string{
			"Normal Started ",
			"Normal Running Not all Steps",
		},
		wantPod: expectedPod("test-taskrun-bundle-pod", "test-task", "test-taskrun-bundle", "foo", config.DefaultServiceAccountValue, false, nil, []stepForExpectedPod{{
			name:  "simple-step",
			image: "foo",
			cmd:   "/mycmd",
		}}),
	}} {
		t.Run(tc.name, func(t *testing.T) {
			testAssets, cancel := getTaskRunController(t, d)
			defer cancel()
			c := testAssets.Controller
			clients := testAssets.Clients
			saName := tc.taskRun.Spec.ServiceAccountName
			createServiceAccount(t, testAssets, saName, tc.taskRun.Namespace)

			if err := c.Reconciler.Reconcile(testAssets.Ctx, getRunName(tc.taskRun)); err == nil {
				t.Error("Wanted a wrapped requeue error, but got nil.")
			} else if ok, _ := controller.IsRequeueKey(err); !ok {
				t.Errorf("expected no error. Got error %v", err)
			}
			if len(clients.Kube.Actions()) == 0 {
				t.Errorf("Expected actions to be logged in the kubeclient, got none")
			}

			tr, err := clients.Pipeline.TektonV1().TaskRuns(tc.taskRun.Namespace).Get(testAssets.Ctx, tc.taskRun.Name, metav1.GetOptions{})
			if err != nil {
				t.Fatalf("getting updated taskrun: %v", err)
			}
			condition := tr.Status.GetCondition(apis.ConditionSucceeded)
			if condition == nil || condition.Status != corev1.ConditionUnknown {
				t.Errorf("Expected invalid TaskRun to have in progress status, but had %v", condition)
			}
			if condition != nil && condition.Reason != v1.TaskRunReasonRunning.String() {
				t.Errorf("Expected reason %q but was %s", v1.TaskRunReasonRunning.String(), condition.Reason)
			}

			if tr.Status.PodName == "" {
				t.Fatalf("Reconcile didn't set pod name")
			}

			pod, err := clients.Kube.CoreV1().Pods(tr.Namespace).Get(testAssets.Ctx, tr.Status.PodName, metav1.GetOptions{})
			if err != nil {
				t.Fatalf("Failed to fetch build pod: %v", err)
			}

			if d := cmp.Diff(tc.wantPod.ObjectMeta, pod.ObjectMeta, ignoreRandomPodNameSuffix); d != "" {
				t.Errorf("Pod metadata doesn't match %s", diff.PrintWantGot(d))
			}

			pod.Name = tc.wantPod.Name // Ignore pod name differences, the pod name is generated and tested in pod_test.go
			if d := cmp.Diff(tc.wantPod.Spec, pod.Spec, resourceQuantityCmp, volumeSort, volumeMountSort, ignoreEnvVarOrdering); d != "" {
				t.Errorf("Pod spec doesn't match %s", diff.PrintWantGot(d))
			}
			if len(clients.Kube.Actions()) == 0 {
				t.Fatalf("Expected actions to be logged in the kubeclient, got none")
			}

			err = k8sevent.CheckEventsOrdered(t, testAssets.Recorder.Events, tc.name, tc.wantEvents)
			if err != nil {
				t.Errorf(err.Error())
			}
		})
	}
}

func TestAlphaReconcile(t *testing.T) {
	names.TestingSeed()
	taskRunWithOutputConfig := parse.MustParseV1TaskRun(t, `
metadata:
  name: test-taskrun-with-output-config
  namespace: foo
spec:
  taskSpec:
    steps:
      - command:
        - /mycmd
        image: myimage
        name: mycontainer
        stdoutConfig:
          path: stdout.txt
`)

	taskRunWithOutputConfigAndWorkspace := parse.MustParseV1TaskRun(t, `
metadata:
  name: test-taskrun-with-output-config-ws
  namespace: foo
spec:
  workspaces:
    - name: data
      emptyDir: {}
  taskSpec:
    workspaces:
      - name: data
    steps:
      - command:
        - /mycmd
        image: myimage
        name: mycontainer
        stdoutConfig:
          path: stdout.txt
`)

	taskruns := []*v1.TaskRun{
		taskRunWithOutputConfig, taskRunWithOutputConfigAndWorkspace,
	}

	cms := []*corev1.ConfigMap{{
		ObjectMeta: metav1.ObjectMeta{Namespace: system.Namespace(), Name: config.GetFeatureFlagsConfigName()},
		Data: map[string]string{
			"enable-api-fields": config.AlphaAPIFields,
		},
	}}
	d := test.Data{
		ConfigMaps:   cms,
		TaskRuns:     taskruns,
		Tasks:        []*v1.Task{simpleTask, saTask, templatedTask},
		ClusterTasks: []*v1beta1.ClusterTask{clustertask},
	}
	for _, tc := range []struct {
		name       string
		taskRun    *v1.TaskRun
		wantPod    *corev1.Pod
		wantEvents []string
	}{{
		name:    "taskrun-with-output-config",
		taskRun: taskRunWithOutputConfig,
		wantEvents: []string{
			"Normal Started ",
			"Normal Running Not all Steps",
		},
		wantPod: expectedPod("test-taskrun-with-output-config-pod", "", "test-taskrun-with-output-config", "foo", config.DefaultServiceAccountValue, false, nil, []stepForExpectedPod{{
			name:       "mycontainer",
			image:      "myimage",
			stdoutPath: "stdout.txt",
			cmd:        "/mycmd",
		}}),
	}, {
		name:    "taskrun-with-output-config-ws",
		taskRun: taskRunWithOutputConfigAndWorkspace,
		wantEvents: []string{
			"Normal Started ",
			"Normal Running Not all Steps",
		},
		wantPod: addVolumeMounts(expectedPod("test-taskrun-with-output-config-ws-pod", "", "test-taskrun-with-output-config-ws", "foo", config.DefaultServiceAccountValue, false,
			[]corev1.Volume{{
				Name: "ws-9l9zj",
				VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{},
				},
			}},
			[]stepForExpectedPod{{
				name:       "mycontainer",
				image:      "myimage",
				stdoutPath: "stdout.txt",
				cmd:        "/mycmd",
			}}),
			[]corev1.VolumeMount{{
				Name:      "ws-9l9zj",
				MountPath: "/workspace/data",
			}}),
	}} {
		t.Run(tc.name, func(t *testing.T) {
			testAssets, cancel := getTaskRunController(t, d)
			defer cancel()
			c := testAssets.Controller
			clients := testAssets.Clients
			saName := tc.taskRun.Spec.ServiceAccountName
			createServiceAccount(t, testAssets, saName, tc.taskRun.Namespace)

			if err := c.Reconciler.Reconcile(testAssets.Ctx, getRunName(tc.taskRun)); err == nil {
				t.Error("Wanted a wrapped requeue error, but got nil.")
			} else if ok, _ := controller.IsRequeueKey(err); !ok {
				t.Errorf("expected no error. Got error %v", err)
			}
			if len(clients.Kube.Actions()) == 0 {
				t.Errorf("Expected actions to be logged in the kubeclient, got none")
			}

			tr, err := clients.Pipeline.TektonV1().TaskRuns(tc.taskRun.Namespace).Get(testAssets.Ctx, tc.taskRun.Name, metav1.GetOptions{})
			if err != nil {
				t.Fatalf("getting updated taskrun: %v", err)
			}
			condition := tr.Status.GetCondition(apis.ConditionSucceeded)
			if condition == nil || condition.Status != corev1.ConditionUnknown {
				t.Errorf("Expected invalid TaskRun to have in progress status, but had %v", condition)
			}
			if condition != nil && condition.Reason != v1.TaskRunReasonRunning.String() {
				t.Errorf("Expected reason %q but was %s", v1.TaskRunReasonRunning.String(), condition.Reason)
			}

			if tr.Status.PodName == "" {
				t.Fatalf("Reconcile didn't set pod name")
			}

			pod, err := clients.Kube.CoreV1().Pods(tr.Namespace).Get(testAssets.Ctx, tr.Status.PodName, metav1.GetOptions{})
			if err != nil {
				t.Fatalf("Failed to fetch build pod: %v", err)
			}

			if d := cmp.Diff(tc.wantPod.ObjectMeta, pod.ObjectMeta, ignoreRandomPodNameSuffix); d != "" {
				t.Errorf("Pod metadata doesn't match %s", diff.PrintWantGot(d))
			}

			pod.Name = tc.wantPod.Name // Ignore pod name differences, the pod name is generated and tested in pod_test.go
			if d := cmp.Diff(tc.wantPod.Spec, pod.Spec, resourceQuantityCmp, volumeSort, volumeMountSort, ignoreEnvVarOrdering); d != "" {
				t.Errorf("Pod spec doesn't match %s", diff.PrintWantGot(d))
			}
			if len(clients.Kube.Actions()) == 0 {
				t.Fatalf("Expected actions to be logged in the kubeclient, got none")
			}

			err = k8sevent.CheckEventsOrdered(t, testAssets.Recorder.Events, tc.name, tc.wantEvents)
			if err != nil {
				t.Errorf(err.Error())
			}
		})
	}
}

func addVolumeMounts(p *corev1.Pod, vms []corev1.VolumeMount) *corev1.Pod {
	for i, vm := range vms {
		p.Spec.Containers[i].VolumeMounts = append(p.Spec.Containers[i].VolumeMounts, vm)
	}
	return p
}

// TestReconcileWithResolver checks that a TaskRun with a populated Resolver
// field creates a ResolutionRequest object for that Resolver's type, and
// that when the request is successfully resolved the TaskRun begins running.
func TestReconcileWithResolver(t *testing.T) {
	resolverName := "foobar"
	tr := parse.MustParseV1TaskRun(t, `
metadata:
  name: tr
  namespace: default
spec:
  taskRef:
    resolver: foobar
  serviceAccountName: default
`)

	d := test.Data{
		TaskRuns: []*v1.TaskRun{tr},
		ServiceAccounts: []*corev1.ServiceAccount{{
			ObjectMeta: metav1.ObjectMeta{Name: tr.Spec.ServiceAccountName, Namespace: "foo"},
		}},
	}

	testAssets, cancel := getTaskRunController(t, d)
	defer cancel()
	c := testAssets.Controller
	clients := testAssets.Clients
	saName := "default"
	if _, err := clients.Kube.CoreV1().ServiceAccounts(tr.Namespace).Create(testAssets.Ctx, &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      saName,
			Namespace: tr.Namespace,
		},
	}, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}

	if err := c.Reconciler.Reconcile(testAssets.Ctx, getRunName(tr)); err == nil {
		t.Error("Wanted a resource request in progress error, but got nil.")
	} else if controller.IsPermanentError(err) {
		t.Errorf("expected no error. Got error %v", err)
	}

	client := testAssets.Clients.ResolutionRequests.ResolutionV1beta1().ResolutionRequests("default")
	resolutionrequests, err := client.List(testAssets.Ctx, metav1.ListOptions{})
	if err != nil {
		t.Fatalf("unexpected error listing resource requests: %v", err)
	}
	numResolutionRequests := len(resolutionrequests.Items)
	if numResolutionRequests != 1 {
		t.Fatalf("expected exactly 1 resource request but found %d", numResolutionRequests)
	}

	resreq := &resolutionrequests.Items[0]
	resolutionRequestType := resreq.ObjectMeta.Labels["resolution.tekton.dev/type"]
	if resolutionRequestType != resolverName {
		t.Fatalf("expected resource request type %q but saw %q", resolutionRequestType, resolverName)
	}

	// Mock a successful resolution
	var taskBytes = []byte(`
          kind: Task
          apiVersion: tekton.dev/v1
          metadata:
            name: foo
          spec:
            steps:
            - name: step1
              image: ubuntu
              script: |
                echo "hello world!"
        `)
	resreq.Status.ResolutionRequestStatusFields.Data = base64.StdEncoding.Strict().EncodeToString(taskBytes)
	resreq.Status.MarkSucceeded()
	resreq, err = client.UpdateStatus(testAssets.Ctx, resreq, metav1.UpdateOptions{})
	if err != nil {
		t.Fatalf("unexpected error updating resource request with resolved task data: %v", err)
	}

	// Check that the resolved task was recognized by the
	// TaskRun reconciler and that the TaskRun has now
	// started executing.
	if err := c.Reconciler.Reconcile(testAssets.Ctx, getRunName(tr)); err != nil {
		if ok, _ := controller.IsRequeueKey(err); !ok {
			t.Errorf("expected no error. Got error %v", err)
		}
	}

	updatedTR, err := clients.Pipeline.TektonV1().TaskRuns(tr.Namespace).Get(testAssets.Ctx, tr.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("getting updated taskrun: %v", err)
	}
	condition := updatedTR.Status.GetCondition(apis.ConditionSucceeded)
	if condition == nil || condition.Status != corev1.ConditionUnknown {
		t.Errorf("Expected fresh TaskRun to have in progress status, but had %v", condition)
	}
	if condition != nil && condition.Reason != v1.TaskRunReasonRunning.String() {
		t.Errorf("Expected reason %q but was %s", v1.TaskRunReasonRunning.String(), condition.Reason)
	}
}

// TestReconcileWithFailingResolver checks that a TaskRun with a failing Resolver
// field creates a ResolutionRequest object for that Resolver's type, and
// that when the request fails, the TaskRun fails.
func TestReconcileWithFailingResolver(t *testing.T) {
	resolverName := "foobar"
	tr := parse.MustParseV1TaskRun(t, `
metadata:
  name: tr
  namespace: default
spec:
  taskRef:
    resolver: foobar
  serviceAccountName: default
`)

	d := test.Data{
		TaskRuns: []*v1.TaskRun{tr},
		ServiceAccounts: []*corev1.ServiceAccount{{
			ObjectMeta: metav1.ObjectMeta{Name: tr.Spec.ServiceAccountName, Namespace: "foo"},
		}},
	}

	testAssets, cancel := getTaskRunController(t, d)
	defer cancel()
	c := testAssets.Controller
	clients := testAssets.Clients
	saName := "default"
	if _, err := clients.Kube.CoreV1().ServiceAccounts(tr.Namespace).Create(testAssets.Ctx, &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      saName,
			Namespace: tr.Namespace,
		},
	}, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}

	if err := c.Reconciler.Reconcile(testAssets.Ctx, getRunName(tr)); err == nil {
		t.Error("Wanted a resource request in progress error, but got nil.")
	} else if controller.IsPermanentError(err) {
		t.Errorf("expected no error. Got error %v", err)
	}

	client := testAssets.Clients.ResolutionRequests.ResolutionV1beta1().ResolutionRequests("default")
	resolutionrequests, err := client.List(testAssets.Ctx, metav1.ListOptions{})
	if err != nil {
		t.Fatalf("unexpected error listing resource requests: %v", err)
	}
	numResolutionRequests := len(resolutionrequests.Items)
	if numResolutionRequests != 1 {
		t.Fatalf("expected exactly 1 resource request but found %d", numResolutionRequests)
	}

	resreq := &resolutionrequests.Items[0]
	resolutionRequestType := resreq.ObjectMeta.Labels["resolution.tekton.dev/type"]
	if resolutionRequestType != resolverName {
		t.Fatalf("expected resource request type %q but saw %q", resolutionRequestType, resolverName)
	}

	resreq.Status.MarkFailed(resolutioncommon.ReasonResolutionTimedOut, "resolution took longer than global timeout of 1 minute")
	resreq, err = client.UpdateStatus(testAssets.Ctx, resreq, metav1.UpdateOptions{})
	if err != nil {
		t.Fatalf("unexpected error updating resource request with resolved pipeline data: %v", err)
	}

	// Check that the TaskRun fails.
	if err := c.Reconciler.Reconcile(testAssets.Ctx, getRunName(tr)); err == nil {
		t.Fatalf("expected an error")
	}

	updatedTR, err := clients.Pipeline.TektonV1().TaskRuns(tr.Namespace).Get(testAssets.Ctx, tr.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("getting updated taskrun: %v", err)
	}
	condition := updatedTR.Status.GetCondition(apis.ConditionSucceeded)
	if condition == nil || condition.Status != corev1.ConditionFalse {
		t.Errorf("Expected fresh TaskRun to have failed, but had %v", condition)
	}
	if condition != nil && condition.Reason != podconvert.ReasonFailedResolution {
		t.Errorf("Expected reason %q but was %s", podconvert.ReasonFailedResolution, condition.Reason)
	}
}

func TestReconcile_SetsStartTime(t *testing.T) {
	taskRun := parse.MustParseV1TaskRun(t, `
metadata:
  name: test-taskrun
  namespace: foo
spec:
  taskRef:
    name: test-task
`)
	d := test.Data{
		TaskRuns: []*v1.TaskRun{taskRun},
		Tasks:    []*v1.Task{simpleTask},
	}
	testAssets, cancel := getTaskRunController(t, d)
	defer cancel()
	createServiceAccount(t, testAssets, "default", "foo")

	if err := testAssets.Controller.Reconciler.Reconcile(testAssets.Ctx, getRunName(taskRun)); err == nil {
		t.Error("Wanted a wrapped requeue error, but got nil.")
	} else if ok, _ := controller.IsRequeueKey(err); !ok {
		t.Errorf("expected no error reconciling valid TaskRun but got %v", err)
	}

	newTr, err := testAssets.Clients.Pipeline.TektonV1().TaskRuns(taskRun.Namespace).Get(testAssets.Ctx, taskRun.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Expected TaskRun %s to exist but instead got error when getting it: %v", taskRun.Name, err)
	}
	if newTr.Status.StartTime == nil || newTr.Status.StartTime.IsZero() {
		t.Errorf("expected startTime to be set by reconcile but was %q", newTr.Status.StartTime)
	}
}

func TestReconcile_DoesntChangeStartTime(t *testing.T) {
	startTime := time.Date(2000, 1, 1, 1, 1, 1, 1, time.UTC)
	taskRun := parse.MustParseV1TaskRun(t, `
metadata:
  name: test-taskrun
  namespace: foo
spec:
  taskRef:
    name: test-task
status:
  podName: the-pod
`)
	taskRun.Status.StartTime = &metav1.Time{Time: startTime}
	d := test.Data{
		TaskRuns: []*v1.TaskRun{taskRun},
		Tasks:    []*v1.Task{simpleTask},
		Pods: []*corev1.Pod{{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "foo",
				Name:      "the-pod",
			},
		}},
	}
	testAssets, cancel := getTaskRunController(t, d)
	defer cancel()

	if err := testAssets.Controller.Reconciler.Reconcile(testAssets.Ctx, getRunName(taskRun)); err != nil {
		t.Errorf("expected no error reconciling valid TaskRun but got %v", err)
	}

	if taskRun.Status.StartTime.Time != startTime {
		t.Errorf("expected startTime %q to be preserved by reconcile but was %q", startTime, taskRun.Status.StartTime)
	}
}

func TestReconcileInvalidTaskRuns(t *testing.T) {
	noTaskRun := parse.MustParseV1TaskRun(t, `
metadata:
  name: notaskrun
  namespace: foo
spec:
  taskRef:
    name: notask
`)
	withWrongRef := parse.MustParseV1TaskRun(t, `
metadata:
  name: taskrun-with-wrong-ref
  namespace: foo
spec:
  taskRef:
    kind: ClusterTask
    name: taskrun-with-wrong-ref
`)
	taskRuns := []*v1.TaskRun{noTaskRun, withWrongRef}
	tasks := []*v1.Task{simpleTask}

	d := test.Data{
		TaskRuns: taskRuns,
		Tasks:    tasks,
	}

	testcases := []struct {
		name       string
		taskRun    *v1.TaskRun
		reason     string
		wantEvents []string
	}{{
		name:    "task run with no task",
		taskRun: noTaskRun,
		reason:  podconvert.ReasonFailedResolution,
		wantEvents: []string{
			"Normal Started",
			"Warning Failed",
			"Warning InternalError",
		},
	}, {
		name:    "task run with wrong ref",
		taskRun: withWrongRef,
		reason:  podconvert.ReasonFailedResolution,
		wantEvents: []string{
			"Normal Started",
			"Warning Failed",
			"Warning InternalError",
		},
	}}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			testAssets, cancel := getTaskRunController(t, d)
			defer cancel()
			c := testAssets.Controller
			clients := testAssets.Clients
			reconcileErr := c.Reconciler.Reconcile(testAssets.Ctx, getRunName(tc.taskRun))

			// When a TaskRun is invalid and can't run, we return a permanent error because
			// a regular error will tell the Reconciler to keep trying to reconcile; instead we want to stop
			// and forget about the Run.
			if reconcileErr == nil {
				t.Fatalf("Expected to see error when reconciling invalid TaskRun but none")
			}
			if !controller.IsPermanentError(reconcileErr) {
				t.Fatalf("Expected to see a permanent error when reconciling invalid TaskRun, got %s instead", reconcileErr)
			}

			// Check actions and events
			actions := clients.Kube.Actions()
			if len(actions) != 2 {
				t.Errorf("expected 2 actions, got %d. Actions: %#v", len(actions), actions)
			}

			err := k8sevent.CheckEventsOrdered(t, testAssets.Recorder.Events, tc.name, tc.wantEvents)
			if !(err == nil) {
				t.Errorf(err.Error())
			}

			newTr, err := testAssets.Clients.Pipeline.TektonV1().TaskRuns(tc.taskRun.Namespace).Get(testAssets.Ctx, tc.taskRun.Name, metav1.GetOptions{})
			if err != nil {
				t.Fatalf("Expected TaskRun %s to exist but instead got error when getting it: %v", tc.taskRun.Name, err)
			}
			// Since the TaskRun is invalid, the status should say it has failed
			condition := newTr.Status.GetCondition(apis.ConditionSucceeded)
			if condition == nil || condition.Status != corev1.ConditionFalse {
				t.Errorf("Expected invalid TaskRun to have failed status, but had %v", condition)
			}
			if condition != nil && condition.Reason != tc.reason {
				t.Errorf("Expected failure to be because of reason %q but was %s", tc.reason, condition.Reason)
			}
		})
	}
}

func TestReconcileRetry(t *testing.T) {
	var (
		toBeCanceledTaskRun = parse.MustParseV1TaskRun(t, `
metadata:
  name: test-taskrun-run-retry-canceled
  namespace: foo
spec:
  retries: 1
  status: TaskRunCancelled
  taskRef:
    name: test-task
status:
  startTime: "2021-12-31T23:59:59Z"
  conditions:
  - reason: Running
    status: Unknown
    type: Succeeded
    message: "TaskRun \"test-taskrun-run-retry-canceled\" was cancelled. "
`)
		canceledTaskRun = parse.MustParseV1TaskRun(t, `
metadata:
  name: test-taskrun-run-retry-canceled
  namespace: foo
spec:
  retries: 1
  status: TaskRunCancelled
  taskRef:
    name: test-task
status:
  startTime: "2021-12-31T23:59:59Z"
  completionTime: "2022-01-01T00:00:00Z"
  conditions:
  - reason: TaskRunCancelled
    status: "False"
    type: Succeeded
    message: "TaskRun \"test-taskrun-run-retry-canceled\" was cancelled. "
`)
		toBeTimedOutTaskRun = parse.MustParseV1TaskRun(t, `
metadata:
  name: test-taskrun-run-retry-timedout
  namespace: foo
spec:
  retries: 1
  timeout: "10s"
  taskRef:
    name: test-task
status:
  startTime: "2021-12-31T00:00:00Z"
  conditions:
  - reason: Running
    status: Unknown
    type: Succeeded
    message: TaskRun "test-taskrun-run-retry-timedout" failed to finish within "10s"
`)
		timedOutTaskRun = parse.MustParseV1TaskRun(t, `
metadata:
  name: test-taskrun-run-retry-timedout
  namespace: foo
spec:
  retries: 1
  timeout: "10s"
  taskRef:
    name: test-task
status:
  conditions:
  - reason: ToBeRetried
    status: Unknown
    type: Succeeded
    message: TaskRun "test-taskrun-run-retry-timedout" failed to finish within "10s"
  retriesStatus:
  - conditions:
    - reason: "TaskRunTimeout"
      status: "False"
      type: Succeeded
      message: TaskRun "test-taskrun-run-retry-timedout" failed to finish within "10s"
    startTime: "2021-12-31T00:00:00Z"
    completionTime: "2022-01-01T00:00:00Z"
    `)
		toFailOnPodFailureTaskRun = parse.MustParseV1TaskRun(t, `
metadata:
  name: test-taskrun-run-retry-pod-failure
  namespace: foo
spec:
  retries: 1
  taskRef:
    name: test-task
status:
  startTime: "2021-12-31T23:59:59Z"
  podName: test-taskrun-run-retry-pod-failure-pod
  steps:
  - container: step-unamed-0
    name: unamed-0
    waiting:
      reason: "ImagePullBackOff"
`)
		failedOnPodFailureTaskRun = parse.MustParseV1TaskRun(t, `
metadata:
  name: test-taskrun-run-retry-pod-failure
  namespace: foo
spec:
  retries: 1
  taskRef:
    name: test-task
status:
  conditions:
  - reason: ToBeRetried
    status: Unknown
    type: Succeeded
    message: "The step \"unamed-0\" in TaskRun \"test-taskrun-run-retry-pod-failure\" failed to pull the image \"\". The pod errored with the message: \".\""
  steps:
  - container: step-unamed-0
    name: unamed-0
    terminated:
      exitCode: 1
      finishedAt: "2022-01-01T00:00:00Z"
      reason: "TaskRunImagePullFailed"
  retriesStatus:
  - conditions:
    - reason: "TaskRunImagePullFailed"
      status: "False"
      type: Succeeded
      message: "The step \"unamed-0\" in TaskRun \"test-taskrun-run-retry-pod-failure\" failed to pull the image \"\". The pod errored with the message: \".\""
    startTime: "2021-12-31T23:59:59Z"
    completionTime: "2022-01-01T00:00:00Z"
    podName: test-taskrun-run-retry-pod-failure-pod
    steps:
    - container: step-unamed-0
      name: unamed-0
      terminated:
        exitCode: 1
        finishedAt: "2022-01-01T00:00:00Z"
        reason: "TaskRunImagePullFailed"
`)
		failedPod = &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: "test-taskrun-run-retry-pod-failure-pod"},
			Status: corev1.PodStatus{Conditions: []corev1.PodCondition{{
				Type:   corev1.PodReady,
				Status: "False",
				Reason: "PodFailed",
			}}},
		}
		toFailOnPrepareTaskRun = parse.MustParseV1TaskRun(t, `
metadata:
  name: test-taskrun-run-retry-prepare-failure
  namespace: foo
spec:
  retries: 1
  taskRef:
    name: test-task
status:
  startTime: "2021-12-31T23:59:59Z"
  conditions:
  - reason: Running
    status: Unknown
    type: Succeeded
    lastTransitionTime: "2022-01-01T00:00:00Z"
`)
		failedOnPrepareTaskRun = parse.MustParseV1TaskRun(t, `
metadata:
  name: test-taskrun-run-retry-prepare-failure
  namespace: foo
spec:
  retries: 1
  taskRef:
    name: test-task
status:
  conditions:
  - reason: ToBeRetried
    status: Unknown
    type: Succeeded
    message: "error when listing tasks for taskRun test-taskrun-run-retry-prepare-failure: tasks.tekton.dev \"test-task\" not found"
  retriesStatus:
  - conditions:
    - reason: TaskRunResolutionFailed
      status: "False"
      type: "Succeeded"
      message: "error when listing tasks for taskRun test-taskrun-run-retry-prepare-failure: tasks.tekton.dev \"test-task\" not found"
    startTime: "2021-12-31T23:59:59Z"
    completionTime: "2022-01-01T00:00:00Z"
`)
		prepareError                    = fmt.Errorf("1 error occurred:\n\t* error when listing tasks for taskRun test-taskrun-run-retry-prepare-failure: tasks.tekton.dev \"test-task\" not found")
		toFailOnReconcileFailureTaskRun = parse.MustParseV1TaskRun(t, `
metadata:
  name: test-taskrun-results-type-mismatched
  namespace: foo
spec:
  retries: 1
  taskRef:
    name: test-results-task
status:
  startTime: "2021-12-31T23:59:59Z"
  results:
    - name: aResult
      type: string
      value: aResultValue
`)
		failedOnReconcileFailureTaskRun = parse.MustParseV1TaskRun(t, `
metadata:
  name: test-taskrun-results-type-mismatched
  namespace: foo
spec:
  retries: 1
  taskRef:
    name: test-results-task
status:
  conditions:
  - reason: ToBeRetried
    status: Unknown
    type: Succeeded
    message: "Provided results don't match declared results; may be invalid JSON or missing result declaration:  \"aResult\": task result is expected to be \"array\" type but was initialized to a different type \"string\""
  sideCars:
  retriesStatus:
  - conditions:
    - reason: TaskRunValidationFailed
      status: "False"
      type: "Succeeded"
      message: "Provided results don't match declared results; may be invalid JSON or missing result declaration:  \"aResult\": task result is expected to be \"array\" type but was initialized to a different type \"string\""
    startTime: "2021-12-31T23:59:59Z"
    completionTime: "2022-01-01T00:00:00Z"
    podName: "test-taskrun-results-type-mismatched-pod"
    provenance:
      featureFlags:
        RunningInEnvWithInjectedSidecars: true
        EnableTektonOCIBundles: true
        EnableAPIFields: "alpha"
        AwaitSidecarReadiness: true
        VerificationNoMatchPolicy: "ignore"
        EnableProvenanceInStatus: true
        ResultExtractionMethod: "termination-message"
        MaxResultSize: 4096
  provenance:
    featureFlags:
      RunningInEnvWithInjectedSidecars: true
      EnableTektonOCIBundles: true
      EnableAPIFields: "alpha"
      AwaitSidecarReadiness: true
      VerificationNoMatchPolicy: "ignore"
      EnableProvenanceInStatus: true
      ResultExtractionMethod: "termination-message"
      MaxResultSize: 4096
`)
		reconciliatonError = fmt.Errorf("1 error occurred:\n\t* Provided results don't match declared results; may be invalid JSON or missing result declaration:  \"aResult\": task result is expected to be \"array\" type but was initialized to a different type \"string\"")
		toBeRetriedTaskRun = parse.MustParseV1TaskRun(t, `
metadata:
  name: test-taskrun-to-be-retried
  namespace: foo
spec:
  retries: 1
  taskRef:
    name: test-task
status:
  conditions:
  - reason: ToBeRetried
    status: Unknown
    type: Succeeded
  retriesStatus:
  - conditions:
    - reason: TimedOut
      status: "False"
      type: Succeeded
`)
		retriedTaskRun = parse.MustParseV1TaskRun(t, `
metadata:
  name: test-taskrun-to-be-retried
  namespace: foo
spec:
  retries: 1
  taskRef:
    name: test-task
status:
  startTime: "2022-01-01T00:00:00Z"
  podName:   "test-taskrun-to-be-retried-pod-retry1"
  conditions:
  - reason: Running
    status: Unknown
    type: Succeeded
    message: Not all Steps in the Task have finished executing
  retriesStatus:
  - conditions:
    - reason: TimedOut
      status: "False"
      type: Succeeded
  provenance:
    featureFlags:
      RunningInEnvWithInjectedSidecars: true
      EnableAPIFields: "beta"
      AwaitSidecarReadiness: true
      VerificationNoMatchPolicy: "ignore"
      EnableProvenanceInStatus: true
      ResultExtractionMethod: "termination-message"
      MaxResultSize: 4096
`)
	)

	for _, tc := range []struct {
		name               string
		testData           test.Data
		task               *v1.Task
		tr                 *v1.TaskRun
		pod                *corev1.Pod
		wantTr             *v1.TaskRun
		wantReconcileError error
		wantCompletionTime bool
		wantStartTime      bool
	}{{
		name: "No Retry on Cancellation",
		testData: test.Data{
			TaskRuns: []*v1.TaskRun{toBeCanceledTaskRun},
			Tasks:    []*v1.Task{simpleTask},
		},
		tr:                 toBeCanceledTaskRun,
		wantTr:             canceledTaskRun,
		wantCompletionTime: true,
		wantStartTime:      true,
	}, {
		name: "Retry on TimedOut",
		testData: test.Data{
			TaskRuns: []*v1.TaskRun{toBeTimedOutTaskRun},
			Tasks:    []*v1.Task{simpleTask},
		},
		tr:            toBeTimedOutTaskRun,
		wantTr:        timedOutTaskRun,
		wantStartTime: false,
	}, {
		name: "Retry on TaskRun Pod Failure",
		testData: test.Data{
			TaskRuns: []*v1.TaskRun{toFailOnPodFailureTaskRun},
			Tasks:    []*v1.Task{simpleTask},
			Pods:     []*corev1.Pod{failedPod},
		},
		tr:            toFailOnPodFailureTaskRun,
		wantTr:        failedOnPodFailureTaskRun,
		wantStartTime: false,
	}, {
		name: "Retry on TaskRun Prepare Failure",
		testData: test.Data{
			TaskRuns: []*v1.TaskRun{toFailOnPrepareTaskRun},
		},
		tr:                 toFailOnPrepareTaskRun,
		wantTr:             failedOnPrepareTaskRun,
		wantReconcileError: prepareError,
		wantStartTime:      false,
	}, {
		name: "Retry on TaskRun Reconciliation Failure",
		testData: test.Data{
			TaskRuns: []*v1.TaskRun{toFailOnReconcileFailureTaskRun},
			Tasks:    []*v1.Task{resultsTask},
			ConfigMaps: []*corev1.ConfigMap{{
				ObjectMeta: metav1.ObjectMeta{Namespace: system.Namespace(), Name: config.GetFeatureFlagsConfigName()},
				Data: map[string]string{
					"enable-api-fields": config.AlphaAPIFields,
				},
			}},
		},
		tr:                 toFailOnReconcileFailureTaskRun,
		wantTr:             failedOnReconcileFailureTaskRun,
		wantReconcileError: reconciliatonError,
		wantStartTime:      false,
	}, {
		name: "Start a ToBeRetried TaskRun",
		testData: test.Data{
			TaskRuns: []*v1.TaskRun{toBeRetriedTaskRun},
			Tasks:    []*v1.Task{simpleTask},
		},
		tr:            toBeRetriedTaskRun,
		wantTr:        retriedTaskRun,
		wantStartTime: true,
	}} {
		t.Run(tc.name, func(t *testing.T) {
			testAssets, cancel := getTaskRunController(t, tc.testData)
			defer cancel()
			c := testAssets.Controller
			clients := testAssets.Clients
			createServiceAccount(t, testAssets, "default", tc.tr.Namespace)

			err := c.Reconciler.Reconcile(testAssets.Ctx, getRunName(tc.tr))
			if ok, _ := controller.IsRequeueKey(err); err == nil || !ok {
				// No error; or error equals each other
				if !(err == nil && tc.wantReconcileError == nil) &&
					!(err != nil && tc.wantReconcileError != nil && strings.TrimSuffix(err.Error(), "\n\n") == tc.wantReconcileError.Error()) {
					t.Errorf("Reconcile(): %v, want %v", err, tc.wantReconcileError)
				}
			}

			reconciledTaskRun, err := clients.Pipeline.TektonV1().TaskRuns("foo").Get(testAssets.Ctx, tc.tr.Name, metav1.GetOptions{})
			if err != nil {
				t.Fatalf("got %v; want nil", err)
			}

			ignoreFields := []cmp.Option{
				ignoreLastTransitionTime,
				ignoreStartTime,
				ignoreCompletionTime,
				ignoreObjectMeta,
				ignoreStatusTaskSpec,
				ignoreTaskRunStatusFields,
			}
			if d := cmp.Diff(reconciledTaskRun, tc.wantTr, ignoreFields...); d != "" {
				t.Errorf("Didn't get expected TaskRun: %v", diff.PrintWantGot(d))
			}

			if !tc.wantCompletionTime && reconciledTaskRun.Status.CompletionTime != nil {
				t.Error("Expect completion time to be nil")
			}
			if tc.wantCompletionTime && reconciledTaskRun.Status.CompletionTime == nil {
				t.Error("Didn't expect completion time to be nil")
			}
			if !tc.wantStartTime && reconciledTaskRun.Status.StartTime != nil {
				t.Error("Expect completion time to be nil")
			}
			if tc.wantStartTime && reconciledTaskRun.Status.StartTime == nil {
				t.Error("Didn't expect completion time to be nil")
			}
		})
	}
}

func TestReconcileGetTaskError(t *testing.T) {
	tr := parse.MustParseV1TaskRun(t, `
metadata:
  name: test-taskrun-run-success
  namespace: foo
spec:
  taskRef:
    name: test-task
`)
	d := test.Data{
		TaskRuns:     []*v1.TaskRun{tr},
		Tasks:        []*v1.Task{simpleTask},
		ClusterTasks: []*v1beta1.ClusterTask{},
	}
	testAssets, cancel := getTaskRunController(t, d)
	defer cancel()
	c := testAssets.Controller
	clients := testAssets.Clients
	createServiceAccount(t, testAssets, "default", tr.Namespace)

	failingReactorActivated := true
	clients.Pipeline.PrependReactor("*", "tasks", func(action ktesting.Action) (handled bool, ret runtime.Object, err error) {
		return failingReactorActivated, &v1.Task{}, errors.New("etcdserver: leader changed")
	})
	err := c.Reconciler.Reconcile(testAssets.Ctx, getRunName(tr))
	if err == nil {
		t.Error("Wanted a wrapped error, but got nil.")
	}
	if controller.IsPermanentError(err) {
		t.Errorf("Unexpected permanent error %v", err)
	}

	failingReactorActivated = false
	err = c.Reconciler.Reconcile(testAssets.Ctx, getRunName(tr))
	if err != nil {
		if ok, _ := controller.IsRequeueKey(err); !ok {
			t.Errorf("unexpected error in TaskRun reconciliation: %v", err)
		}
	}
	reconciledRun, err := clients.Pipeline.TektonV1().TaskRuns("foo").Get(testAssets.Ctx, tr.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Somehow had error getting reconciled run out of fake client: %s", err)
	}
	condition := reconciledRun.Status.GetCondition(apis.ConditionSucceeded)
	if !condition.IsUnknown() {
		t.Errorf("Expected TaskRun to still be running but succeeded condition is %v", condition.Status)
	}
}

func TestReconcileTaskRunWithPermanentError(t *testing.T) {
	noTaskRun := parse.MustParseV1TaskRun(t, `
metadata:
  name: notaskrun
  namespace: foo
spec:
  taskRef:
    name: notask
status:
  conditions:
  - message: 'error when listing tasks for taskRun taskrun-failure: tasks.tekton.dev
      "notask" not found'
    reason: TaskRunResolutionFailed
    status: "False"
    type: Succeeded
  startTime: "2022-01-01T00:00:00Z"
`)

	taskRuns := []*v1.TaskRun{noTaskRun}
	d := test.Data{
		TaskRuns: taskRuns,
	}

	testAssets, cancel := getTaskRunController(t, d)
	defer cancel()
	c := testAssets.Controller
	clients := testAssets.Clients
	reconcileErr := c.Reconciler.Reconcile(testAssets.Ctx, getRunName(noTaskRun))

	// When a TaskRun was rejected with a permanent error, reconciler must stop and forget about the run
	// Such TaskRun enters Reconciler and from within the isDone block, marks the run success so that
	// reconciler does not keep trying to reconcile
	if reconcileErr != nil {
		t.Fatalf("Expected to see no error when reconciling TaskRun with Permanent Error but was not none")
	}

	// Check actions
	actions := clients.Kube.Actions()
	if len(actions) != 2 || !actions[0].Matches("list", "configmaps") || !actions[1].Matches("watch", "configmaps") {
		t.Errorf("expected 2 actions (list configmaps, and watch configmaps) created by the reconciler,"+
			" got %d. Actions: %#v", len(actions), actions)
	}

	newTr, err := clients.Pipeline.TektonV1().TaskRuns(noTaskRun.Namespace).Get(context.Background(), noTaskRun.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Expected TaskRun %s to exist but instead got error when getting it: %v", noTaskRun.Name, err)
	}

	// Since the TaskRun is invalid, the status should say it has failed
	condition := newTr.Status.GetCondition(apis.ConditionSucceeded)
	if condition == nil || condition.Status != corev1.ConditionFalse {
		t.Errorf("Expected invalid TaskRun to have failed status, but had %v", condition)
	}
	if condition != nil && condition.Reason != podconvert.ReasonFailedResolution {
		t.Errorf("Expected failure to be because of reason %q but was %s", podconvert.ReasonFailedResolution, condition.Reason)
	}
}

func TestReconcilePodFetchError(t *testing.T) {
	taskRun := parse.MustParseV1TaskRun(t, `
metadata:
  name: test-taskrun-run-success
  namespace: foo
spec:
  taskRef:
    name: test-task
status:
  podName: will-not-be-found
`)
	d := test.Data{
		TaskRuns: []*v1.TaskRun{taskRun},
		Tasks:    []*v1.Task{simpleTask},
	}

	testAssets, cancel := getTaskRunController(t, d)
	defer cancel()
	c := testAssets.Controller
	clients := testAssets.Clients

	clients.Kube.PrependReactor("get", "pods", func(action ktesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, errors.New("induce failure fetching pods")
	})

	if err := c.Reconciler.Reconcile(testAssets.Ctx, getRunName(taskRun)); err == nil {
		t.Fatal("expected error when reconciling a Task for which we couldn't get the corresponding Pod but got nil")
	}
}

func makePod(taskRun *v1.TaskRun, task *v1.Task) (*corev1.Pod, error) {
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

	builder := podconvert.Builder{
		Images:          images,
		KubeClient:      kubeclient,
		EntrypointCache: entrypointCache,
	}
	return builder.Build(context.Background(), taskRun, task.Spec)
}

func TestReconcilePodUpdateStatus(t *testing.T) {
	const taskLabel = "test-task"
	taskRun := parse.MustParseV1TaskRun(t, `
metadata:
  name: test-taskrun-run-success
  namespace: foo
spec:
  taskRef:
    name: test-task
status:
  podName: test-taskrun-run-success-pod
`)

	pod, err := makePod(taskRun, simpleTask)
	if err != nil {
		t.Fatalf("MakePod: %v", err)
	}
	d := test.Data{
		TaskRuns: []*v1.TaskRun{taskRun},
		Tasks:    []*v1.Task{simpleTask},
		Pods:     []*corev1.Pod{pod},
	}

	testAssets, cancel := getTaskRunController(t, d)
	defer cancel()
	c := testAssets.Controller
	clients := testAssets.Clients

	if err := c.Reconciler.Reconcile(testAssets.Ctx, getRunName(taskRun)); err == nil {
		t.Error("Wanted a wrapped requeue error, but got nil.")
	} else if ok, _ := controller.IsRequeueKey(err); !ok {
		t.Fatalf("Unexpected error when Reconcile() : %v", err)
	}
	newTr, err := clients.Pipeline.TektonV1().TaskRuns(taskRun.Namespace).Get(testAssets.Ctx, taskRun.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Expected TaskRun %s to exist but instead got error when getting it: %v", taskRun.Name, err)
	}
	if d := cmp.Diff(&apis.Condition{
		Type:    apis.ConditionSucceeded,
		Status:  corev1.ConditionUnknown,
		Reason:  v1.TaskRunReasonRunning.String(),
		Message: "Not all Steps in the Task have finished executing",
	}, newTr.Status.GetCondition(apis.ConditionSucceeded), ignoreLastTransitionTime); d != "" {
		t.Fatalf("Did not get expected condition %s", diff.PrintWantGot(d))
	}

	trLabel, ok := newTr.ObjectMeta.Labels[pipeline.TaskLabelKey]
	if !ok {
		t.Errorf("Labels were not added to task run")
	}
	if ld := cmp.Diff(taskLabel, trLabel); ld != "" {
		t.Errorf("Did not get expected label %s", diff.PrintWantGot(ld))
	}

	// update pod status and trigger reconcile : build is completed
	pod.Status = corev1.PodStatus{
		Phase: corev1.PodSucceeded,
	}
	if _, err := clients.Kube.CoreV1().Pods(taskRun.Namespace).UpdateStatus(testAssets.Ctx, pod, metav1.UpdateOptions{}); err != nil {
		t.Errorf("Unexpected error while updating build: %v", err)
	}

	// Before calling Reconcile again, we need to ensure that the informer's
	// lister cache is update to reflect the result of the previous Reconcile.
	testAssets.Informers.TaskRun.Informer().GetIndexer().Add(newTr)

	if err := c.Reconciler.Reconcile(testAssets.Ctx, getRunName(taskRun)); err == nil {
		t.Error("Wanted a wrapped requeue error, but got nil.")
	} else if ok, _ := controller.IsRequeueKey(err); !ok {
		t.Fatalf("Unexpected error when Reconcile(): %v", err)
	}

	newTr, err = clients.Pipeline.TektonV1().TaskRuns(taskRun.Namespace).Get(testAssets.Ctx, taskRun.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Unexpected error fetching taskrun: %v", err)
	}
	if d := cmp.Diff(&apis.Condition{
		Type:    apis.ConditionSucceeded,
		Status:  corev1.ConditionTrue,
		Reason:  v1.TaskRunReasonSuccessful.String(),
		Message: "All Steps have completed executing",
	}, newTr.Status.GetCondition(apis.ConditionSucceeded), ignoreLastTransitionTime); d != "" {
		t.Errorf("Did not get expected condition %s", diff.PrintWantGot(d))
	}

	wantEvents := []string{
		"Normal Started ",
		"Normal Running Not all Steps",
		"Normal Succeeded",
	}
	err = k8sevent.CheckEventsOrdered(t, testAssets.Recorder.Events, "test-reconcile-pod-updateStatus", wantEvents)
	if !(err == nil) {
		t.Errorf(err.Error())
	}
}

func TestReconcileOnCompletedTaskRun(t *testing.T) {
	taskRun := parse.MustParseV1TaskRun(t, `
metadata:
  name: test-taskrun-run-success
  namespace: foo
spec:
  taskRef:
    name: test-task
status:
  conditions:
  - message: Build succeeded
    reason: Build succeeded
    status: "True"
    type: Succeeded
  startTime: "2021-12-31T23:59:45Z"
`)
	d := test.Data{
		TaskRuns: []*v1.TaskRun{
			taskRun,
		},
		Tasks: []*v1.Task{simpleTask},
	}

	testAssets, cancel := getTaskRunController(t, d)
	defer cancel()
	c := testAssets.Controller
	clients := testAssets.Clients

	if err := c.Reconciler.Reconcile(testAssets.Ctx, getRunName(taskRun)); err != nil {
		t.Fatalf("Unexpected error when reconciling completed TaskRun : %v", err)
	}
	newTr, err := clients.Pipeline.TektonV1().TaskRuns(taskRun.Namespace).Get(testAssets.Ctx, taskRun.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Expected completed TaskRun %s to exist but instead got error when getting it: %v", taskRun.Name, err)
	}
	if d := cmp.Diff(taskRun.Status.GetCondition(apis.ConditionSucceeded), newTr.Status.GetCondition(apis.ConditionSucceeded), ignoreLastTransitionTime); d != "" {
		t.Fatalf("Did not get expected condition %s", diff.PrintWantGot(d))
	}
}

func TestReconcileOnCancelledTaskRun(t *testing.T) {
	taskRun := parse.MustParseV1TaskRun(t, `
metadata:
  name: test-taskrun-run-cancelled
  namespace: foo
spec:
  status: TaskRunCancelled
  statusMessage: "Test cancellation message."
  taskRef:
    name: test-task
status:
  conditions:
  - status: Unknown
    type: Succeeded
  podName: test-taskrun-run-cancelled-pod
`)
	pod, err := makePod(taskRun, simpleTask)
	if err != nil {
		t.Fatalf("MakePod: %v", err)
	}
	d := test.Data{
		TaskRuns: []*v1.TaskRun{taskRun},
		Tasks:    []*v1.Task{simpleTask},
		Pods:     []*corev1.Pod{pod},
	}

	testAssets, cancel := getTaskRunController(t, d)
	defer cancel()
	c := testAssets.Controller
	clients := testAssets.Clients

	if err := c.Reconciler.Reconcile(testAssets.Ctx, getRunName(taskRun)); err != nil {
		t.Fatalf("Unexpected error when reconciling completed TaskRun : %v", err)
	}
	newTr, err := clients.Pipeline.TektonV1().TaskRuns(taskRun.Namespace).Get(testAssets.Ctx, taskRun.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Expected completed TaskRun %s to exist but instead got error when getting it: %v", taskRun.Name, err)
	}

	expectedStatus := &apis.Condition{
		Type:    apis.ConditionSucceeded,
		Status:  corev1.ConditionFalse,
		Reason:  "TaskRunCancelled",
		Message: `TaskRun "test-taskrun-run-cancelled" was cancelled. Test cancellation message.`,
	}
	if d := cmp.Diff(expectedStatus, newTr.Status.GetCondition(apis.ConditionSucceeded), ignoreLastTransitionTime); d != "" {
		t.Fatalf("Did not get expected condition %s", diff.PrintWantGot(d))
	}

	wantEvents := []string{
		"Normal Started",
		"Warning Failed TaskRun \"test-taskrun-run-cancelled\" was cancelled",
	}
	err = k8sevent.CheckEventsOrdered(t, testAssets.Recorder.Events, "test-reconcile-on-cancelled-taskrun", wantEvents)
	if !(err == nil) {
		t.Errorf(err.Error())
	}

	// reconcile the completed TaskRun again without the pod as that was deleted
	d = test.Data{
		TaskRuns: []*v1.TaskRun{newTr},
		Tasks:    []*v1.Task{simpleTask},
	}

	testAssets, cancel = getTaskRunController(t, d)
	defer cancel()
	c = testAssets.Controller

	if err := c.Reconciler.Reconcile(testAssets.Ctx, getRunName(newTr)); err != nil {
		t.Fatalf("Unexpected error when reconciling completed TaskRun : %v", err)
	}
}

func TestReconcileOnTimedOutTaskRun(t *testing.T) {
	taskRun := parse.MustParseV1TaskRun(t, `
metadata:
  name: test-taskrun-run-timedout
  namespace: foo
spec:
  status: TaskRunCancelled
  statusMessage: TaskRun cancelled as pipeline has been cancelled.
  taskRef:
    name: test-task
status:
  conditions:
  - status: Unknown
    type: Succeeded
  podName: test-taskrun-run-timedout-pod
`)
	pod, err := makePod(taskRun, simpleTask)
	if err != nil {
		t.Fatalf("MakePod: %v", err)
	}
	d := test.Data{
		TaskRuns: []*v1.TaskRun{taskRun},
		Tasks:    []*v1.Task{simpleTask},
		Pods:     []*corev1.Pod{pod},
	}

	testAssets, cancel := getTaskRunController(t, d)
	defer cancel()
	c := testAssets.Controller
	clients := testAssets.Clients

	if err := c.Reconciler.Reconcile(testAssets.Ctx, getRunName(taskRun)); err != nil {
		t.Fatalf("Unexpected error when reconciling completed TaskRun : %v", err)
	}
	newTr, err := clients.Pipeline.TektonV1().TaskRuns(taskRun.Namespace).Get(testAssets.Ctx, taskRun.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Expected completed TaskRun %s to exist but instead got error when getting it: %v", taskRun.Name, err)
	}

	expectedStatus := &apis.Condition{
		Type:    apis.ConditionSucceeded,
		Status:  corev1.ConditionFalse,
		Reason:  v1.TaskRunReasonCancelled.String(),
		Message: `TaskRun "test-taskrun-run-timedout" was cancelled. TaskRun cancelled as pipeline has been cancelled.`,
	}
	if d := cmp.Diff(expectedStatus, newTr.Status.GetCondition(apis.ConditionSucceeded), ignoreLastTransitionTime); d != "" {
		t.Fatalf("Did not get expected condition %s", diff.PrintWantGot(d))
	}

	wantEvents := []string{
		"Normal Started",
		"Warning Failed TaskRun \"test-taskrun-run-timedout\" was cancelled. TaskRun cancelled as pipeline has been cancelled.",
	}
	err = k8sevent.CheckEventsOrdered(t, testAssets.Recorder.Events, "test-reconcile-on-timedout-taskrun", wantEvents)
	if !(err == nil) {
		t.Errorf(err.Error())
	}

	// reconcile the completed TaskRun again without the pod as that was deleted
	d = test.Data{
		TaskRuns: []*v1.TaskRun{newTr},
		Tasks:    []*v1.Task{simpleTask},
	}

	testAssets, cancel = getTaskRunController(t, d)
	defer cancel()
	c = testAssets.Controller

	if err := c.Reconciler.Reconcile(testAssets.Ctx, getRunName(newTr)); err != nil {
		t.Fatalf("Unexpected error when reconciling completed TaskRun : %v", err)
	}
}

func TestReconcilePodFailuresStepImagePullFailed(t *testing.T) {
	taskRun := parse.MustParseV1TaskRun(t, `
metadata:
  name: test-imagepull-fail
  namespace: foo
spec:
  taskSpec:
    steps:
    - image: whatever
status:
  steps:
  - container: step-unnamed-0
    name: unnamed-0
    imageID: whatever
    waiting:
      message: Back-off pulling image "whatever"
      reason: ImagePullBackOff
  taskSpec:
    steps:
    - image: whatever
`)
	expectedStatus := &apis.Condition{
		Type:    apis.ConditionSucceeded,
		Status:  corev1.ConditionFalse,
		Reason:  "TaskRunImagePullFailed",
		Message: `The step "unnamed-0" in TaskRun "test-imagepull-fail" failed to pull the image "whatever". The pod errored with the message: "Back-off pulling image "whatever"."`,
	}

	wantEvents := []string{
		"Normal Started ",
		`Warning Failed The step "unnamed-0" in TaskRun "test-imagepull-fail" failed to pull the image "whatever". The pod errored with the message: "Back-off pulling image "whatever".`,
	}
	d := test.Data{
		TaskRuns: []*v1.TaskRun{taskRun},
	}
	testAssets, cancel := getTaskRunController(t, d)
	defer cancel()
	c := testAssets.Controller
	clients := testAssets.Clients

	if err := c.Reconciler.Reconcile(testAssets.Ctx, getRunName(taskRun)); err != nil {
		t.Fatalf("Unexpected error when reconciling completed TaskRun : %v", err)
	}
	newTr, err := clients.Pipeline.TektonV1().TaskRuns(taskRun.Namespace).Get(testAssets.Ctx, taskRun.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Expected completed TaskRun %s to exist but instead got error when getting it: %v", taskRun.Name, err)
	}
	condition := newTr.Status.GetCondition(apis.ConditionSucceeded)
	if d := cmp.Diff(expectedStatus, condition, ignoreLastTransitionTime); d != "" {
		t.Fatalf("Did not get expected condition %s", diff.PrintWantGot(d))
	}
	err = k8sevent.CheckEventsOrdered(t, testAssets.Recorder.Events, taskRun.Name, wantEvents)
	if err != nil {
		t.Errorf(err.Error())
	}
}

func TestReconcilePodFailuresSidecarImagePullFailed(t *testing.T) {
	taskRun := parse.MustParseV1TaskRun(t, `
metadata:
  name: test-imagepull-fail
  namespace: foo
spec:
  taskSpec:
    sidecars:
    - image: ubuntu
    - image: whatever
    steps:
    - image: alpine
status:
  sidecars:
  - container: step-unnamed-0
    name: unnamed-0
    running:
      startedAt: "2022-06-09T10:13:41Z"
  - container: step-unnamed-1
    name: unnamed-1
    imageID: whatever
    waiting:
      message: Back-off pulling image "whatever"
      reason: ImagePullBackOff
  steps:
  - container: step-unnamed-2
    name: unnamed-2
    running:
      startedAt: "2022-06-09T10:13:41Z"
  taskSpec:
    sidecars:
    - image: ubuntu
    - image: whatever
    steps:
    - image: alpine
`)
	expectedStatus := &apis.Condition{
		Type:    apis.ConditionSucceeded,
		Status:  corev1.ConditionFalse,
		Reason:  "TaskRunImagePullFailed",
		Message: `The sidecar "unnamed-1" in TaskRun "test-imagepull-fail" failed to pull the image "whatever". The pod errored with the message: "Back-off pulling image "whatever"."`,
	}

	wantEvents := []string{
		"Normal Started ",
		`Warning Failed The sidecar "unnamed-1" in TaskRun "test-imagepull-fail" failed to pull the image "whatever". The pod errored with the message: "Back-off pulling image "whatever".`,
	}
	d := test.Data{
		TaskRuns: []*v1.TaskRun{taskRun},
	}
	testAssets, cancel := getTaskRunController(t, d)
	defer cancel()
	c := testAssets.Controller
	clients := testAssets.Clients

	if err := c.Reconciler.Reconcile(testAssets.Ctx, getRunName(taskRun)); err != nil {
		t.Fatalf("Unexpected error when reconciling completed TaskRun : %v", err)
	}
	newTr, err := clients.Pipeline.TektonV1().TaskRuns(taskRun.Namespace).Get(testAssets.Ctx, taskRun.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Expected completed TaskRun %s to exist but instead got error when getting it: %v", taskRun.Name, err)
	}
	condition := newTr.Status.GetCondition(apis.ConditionSucceeded)
	if d := cmp.Diff(expectedStatus, condition, ignoreLastTransitionTime); d != "" {
		t.Fatalf("Did not get expected condition %s", diff.PrintWantGot(d))
	}
	err = k8sevent.CheckEventsOrdered(t, testAssets.Recorder.Events, taskRun.Name, wantEvents)
	if err != nil {
		t.Errorf(err.Error())
	}
}

func TestReconcileTimeouts(t *testing.T) {
	type testCase struct {
		name           string
		taskRun        *v1.TaskRun
		expectedStatus *apis.Condition
		wantEvents     []string
	}

	testcases := []testCase{
		{
			name: "taskrun with timeout",
			taskRun: parse.MustParseV1TaskRun(t, `
metadata:
  name: test-taskrun-timeout
  namespace: foo
spec:
  taskRef:
    name: test-task
  timeout: 10s
status:
  conditions:
  - status: Unknown
    type: Succeeded
  startTime: "2021-12-31T23:59:45Z"
`),

			expectedStatus: &apis.Condition{
				Type:    apis.ConditionSucceeded,
				Status:  corev1.ConditionFalse,
				Reason:  "TaskRunTimeout",
				Message: `TaskRun "test-taskrun-timeout" failed to finish within "10s"`,
			},
			wantEvents: []string{
				"Warning Failed ",
			},
		}, {
			name: "taskrun with default timeout",
			taskRun: parse.MustParseV1TaskRun(t, `
metadata:
  name: test-taskrun-default-timeout-60-minutes
  namespace: foo
spec:
  taskRef:
    name: test-task
status:
  conditions:
  - status: Unknown
    type: Succeeded
  startTime: "2021-12-31T22:59:00Z"
`),
			expectedStatus: &apis.Condition{
				Type:    apis.ConditionSucceeded,
				Status:  corev1.ConditionFalse,
				Reason:  "TaskRunTimeout",
				Message: `TaskRun "test-taskrun-default-timeout-60-minutes" failed to finish within "1h0m0s"`,
			},
			wantEvents: []string{
				"Warning Failed ",
			},
		}, {
			name: "task run with nil timeout uses default",
			taskRun: parse.MustParseV1TaskRun(t, `
metadata:
  name: test-taskrun-nil-timeout-default-60-minutes
  namespace: foo
spec:
  taskRef:
    name: test-task
  timeout: null
status:
  conditions:
  - status: Unknown
    type: Succeeded
  startTime: "2021-12-31T22:59:00Z"
`),

			expectedStatus: &apis.Condition{
				Type:    apis.ConditionSucceeded,
				Status:  corev1.ConditionFalse,
				Reason:  "TaskRunTimeout",
				Message: `TaskRun "test-taskrun-nil-timeout-default-60-minutes" failed to finish within "1h0m0s"`,
			},
			wantEvents: []string{
				"Warning Failed ",
			},
		}}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			d := test.Data{
				TaskRuns: []*v1.TaskRun{tc.taskRun},
				Tasks:    []*v1.Task{simpleTask},
			}
			testAssets, cancel := getTaskRunController(t, d)
			defer cancel()
			c := testAssets.Controller
			clients := testAssets.Clients

			if err := c.Reconciler.Reconcile(testAssets.Ctx, getRunName(tc.taskRun)); err != nil {
				t.Fatalf("Unexpected error when reconciling completed TaskRun : %v", err)
			}
			newTr, err := clients.Pipeline.TektonV1().TaskRuns(tc.taskRun.Namespace).Get(testAssets.Ctx, tc.taskRun.Name, metav1.GetOptions{})
			if err != nil {
				t.Fatalf("Expected completed TaskRun %s to exist but instead got error when getting it: %v", tc.taskRun.Name, err)
			}
			condition := newTr.Status.GetCondition(apis.ConditionSucceeded)
			if d := cmp.Diff(tc.expectedStatus, condition, ignoreLastTransitionTime); d != "" {
				t.Fatalf("Did not get expected condition %s", diff.PrintWantGot(d))
			}
			err = k8sevent.CheckEventsOrdered(t, testAssets.Recorder.Events, tc.taskRun.Name, tc.wantEvents)
			if !(err == nil) {
				t.Errorf(err.Error())
			}
		})
	}
}

func TestPropagatedWorkspaces(t *testing.T) {
	taskRun := parse.MustParseV1TaskRun(t, `
metadata:
  name: test-taskrun-propagating-workspaces
  namespace: foo
spec:
  taskSpec:
    steps:
    - args:
      - replacedArgs - $(workspaces.tr-workspace.path)
      command:
      - echo
      image: foo
      name: simple-step
  workspaces:
  - emptyDir: {}
    name: tr-workspace
`)
	d := test.Data{
		TaskRuns: []*v1.TaskRun{taskRun},
	}
	testAssets, cancel := getTaskRunController(t, d)
	defer cancel()
	createServiceAccount(t, testAssets, "default", taskRun.Namespace)
	c := testAssets.Controller
	if err := c.Reconciler.Reconcile(testAssets.Ctx, getRunName(taskRun)); err == nil {
		t.Fatalf("Could not reconcile the taskrun: %v", err)
	}
	getTaskRun, _ := testAssets.Clients.Pipeline.TektonV1().TaskRuns(taskRun.Namespace).Get(testAssets.Ctx, taskRun.Name, metav1.GetOptions{})

	want := []v1.WorkspaceDeclaration{{
		Name: "tr-workspace",
	}}
	if c := cmp.Diff(want, getTaskRun.Status.TaskSpec.Workspaces); c != "" {
		t.Errorf("TestPropagatedWorkspaces errored with: %s", diff.PrintWantGot(c))
	}
}

func TestExpandMountPath(t *testing.T) {
	expectedMountPath := "/temppath/replaced"
	expectedReplacedArgs := fmt.Sprintf("replacedArgs - %s", expectedMountPath)
	// The task's Workspace has a parameter variable
	simpleTask := parse.MustParseV1Task(t, `
metadata:
  name: test-task
  namespace: foo
spec:
  params:
  - name: source-path
    type: string
  - name: source-path-two
    type: string
  steps:
  - args:
    - replacedArgs - $(workspaces.tr-workspace.path)
    command:
    - echo
    image: foo
    name: simple-step
  workspaces:
  - description: a test task workspace
    mountPath: /temppath/$(params.source-path)
    name: tr-workspace
    readOnly: true
`)

	taskRun := parse.MustParseV1TaskRun(t, `
metadata:
  name: test-taskrun-not-started
  namespace: foo
spec:
  params:
  - name: source-path
    value: replaced
  taskRef:
    name: test-task
  workspaces:
  - emptyDir: {}
    name: tr-workspace
`)
	d := test.Data{
		TaskRuns: []*v1.TaskRun{taskRun},
		Tasks:    []*v1.Task{simpleTask},
	}
	d.ConfigMaps = []*corev1.ConfigMap{
		{
			ObjectMeta: metav1.ObjectMeta{Name: config.GetDefaultsConfigName(), Namespace: system.Namespace()},
			Data: map[string]string{
				"default-cloud-events-sink": "http://synk:8080",
			},
		},
	}

	testAssets, cancel := getTaskRunController(t, d)
	defer cancel()
	createServiceAccount(t, testAssets, "default", taskRun.Namespace)

	// Use the test assets to create a *Reconciler directly for focused testing.
	r := &Reconciler{
		KubeClientSet:     testAssets.Clients.Kube,
		PipelineClientSet: testAssets.Clients.Pipeline,
		Clock:             testClock,
		taskRunLister:     testAssets.Informers.TaskRun.Lister(),
		limitrangeLister:  testAssets.Informers.LimitRange.Lister(),
		cloudEventClient:  testAssets.Clients.CloudEvents,
		metrics:           nil, // Not used
		entrypointCache:   nil, // Not used
		pvcHandler:        volumeclaim.NewPVCHandler(testAssets.Clients.Kube, testAssets.Logger),
		tracerProvider:    trace.NewNoopTracerProvider(),
	}

	rtr := &resources.ResolvedTask{
		TaskName: "test-task",
		Kind:     "Task",
		TaskSpec: &v1.TaskSpec{Steps: simpleTask.Spec.Steps, Workspaces: simpleTask.Spec.Workspaces},
	}
	ctx := config.EnableAlphaAPIFields(context.Background())
	workspaceVolumes := workspace.CreateVolumes(taskRun.Spec.Workspaces)
	taskSpec, err := applyParamsContextsResultsAndWorkspaces(ctx, taskRun, rtr, workspaceVolumes)
	if err != nil {
		t.Fatalf("update task spec threw error %v", err)
	}

	pod, err := r.createPod(testAssets.Ctx, taskSpec, taskRun, rtr, workspaceVolumes)

	if err != nil {
		t.Fatalf("create pod threw error %v", err)
	}

	if vm := pod.Spec.Containers[0].VolumeMounts[0]; !strings.HasPrefix(vm.Name, "ws-9l9zj") || vm.MountPath != expectedMountPath {
		t.Fatalf("failed to find expanded Workspace mountpath %v", expectedMountPath)
	}

	if a := pod.Spec.Containers[0].Args; a[len(a)-1] != expectedReplacedArgs {
		t.Fatalf("failed to replace Workspace mountpath variable, expected %s, actual: %s", expectedReplacedArgs, a)
	}
}

func TestExpandMountPath_DuplicatePaths(t *testing.T) {
	expectedError := "workspace mount path \"/temppath/duplicate\" must be unique: workspaces[1].mountpath"
	// The task has two workspaces, with different mount path strings.
	simpleTask := parse.MustParseV1Task(t, `
metadata:
  name: test-task
  namespace: foo
spec:
  params:
  - name: source-path
    type: string
  - name: source-path-two
    type: string
  steps:
  - command:
    - /mycmd
    env:
    - name: foo
      value: bar
    image: foo
    name: simple-step
  workspaces:
  - description: a test task workspace
    mountPath: /temppath/$(params.source-path)
    name: tr-workspace
    readOnly: true
  - description: a second task workspace
    mountPath: /temppath/$(params.source-path-two)
    name: tr-workspace-two
    readOnly: true
`)

	// The parameter values will cause the two Workspaces to have duplicate mount path values after the parameters are expanded.
	taskRun := parse.MustParseV1TaskRun(t, `
metadata:
  name: test-taskrun-not-started
  namespace: foo
spec:
  params:
  - name: source-path
    value: duplicate
  - name: source-path-two
    value: duplicate
  taskRef:
    name: test-task
  workspaces:
  - emptyDir: {}
    name: tr-workspace
  - emptyDir: {}
    name: tr-workspace-two
`)
	d := test.Data{
		TaskRuns: []*v1.TaskRun{taskRun},
		Tasks:    []*v1.Task{simpleTask},
	}

	d.ConfigMaps = []*corev1.ConfigMap{
		{
			ObjectMeta: metav1.ObjectMeta{Name: config.GetDefaultsConfigName(), Namespace: system.Namespace()},
			Data: map[string]string{
				"default-cloud-events-sink": "http://synk:8080",
			},
		},
	}

	testAssets, cancel := getTaskRunController(t, d)
	defer cancel()
	createServiceAccount(t, testAssets, "default", taskRun.Namespace)

	r := &Reconciler{
		KubeClientSet:     testAssets.Clients.Kube,
		PipelineClientSet: testAssets.Clients.Pipeline,
		Clock:             testClock,
		taskRunLister:     testAssets.Informers.TaskRun.Lister(),
		limitrangeLister:  testAssets.Informers.LimitRange.Lister(),
		cloudEventClient:  testAssets.Clients.CloudEvents,
		metrics:           nil, // Not used
		entrypointCache:   nil, // Not used
		pvcHandler:        volumeclaim.NewPVCHandler(testAssets.Clients.Kube, testAssets.Logger),
		tracerProvider:    trace.NewNoopTracerProvider(),
	}

	rtr := &resources.ResolvedTask{
		TaskName: "test-task",
		Kind:     "Task",
		TaskSpec: &v1.TaskSpec{Steps: simpleTask.Spec.Steps, Workspaces: simpleTask.Spec.Workspaces},
	}

	workspaceVolumes := workspace.CreateVolumes(taskRun.Spec.Workspaces)
	ctx := config.EnableAlphaAPIFields(context.Background())
	taskSpec, err := applyParamsContextsResultsAndWorkspaces(ctx, taskRun, rtr, workspaceVolumes)
	if err != nil {
		t.Errorf("update task spec threw an error: %v", err)
	}
	_, err = r.createPod(testAssets.Ctx, taskSpec, taskRun, rtr, workspaceVolumes)

	if err == nil || err.Error() != expectedError {
		t.Errorf("Expected to fail validation for duplicate Workspace mount paths, error was %v", err)
	}
}

func TestHandlePodCreationError(t *testing.T) {
	taskRun := parse.MustParseV1TaskRun(t, `
metadata:
  name: test-taskrun-pod-creation-failed
spec:
  taskRef:
    name: test-task
status:
  conditions:
  - status: Unknown
    type: Succeeded
  startTime: "2022-01-01T00:00:00Z"
`)
	d := test.Data{
		TaskRuns: []*v1.TaskRun{taskRun},
		Tasks:    []*v1.Task{simpleTask},
	}
	testAssets, cancel := getTaskRunController(t, d)
	defer cancel()

	// Use the test assets to create a *Reconciler directly for focused testing.
	c := &Reconciler{
		KubeClientSet:     testAssets.Clients.Kube,
		PipelineClientSet: testAssets.Clients.Pipeline,
		Clock:             testClock,
		taskRunLister:     testAssets.Informers.TaskRun.Lister(),
		limitrangeLister:  testAssets.Informers.LimitRange.Lister(),
		cloudEventClient:  testAssets.Clients.CloudEvents,
		metrics:           nil, // Not used
		entrypointCache:   nil, // Not used
		pvcHandler:        volumeclaim.NewPVCHandler(testAssets.Clients.Kube, testAssets.Logger),
		tracerProvider:    trace.NewNoopTracerProvider(),
	}

	testcases := []struct {
		description    string
		err            error
		expectedType   apis.ConditionType
		expectedStatus corev1.ConditionStatus
		expectedReason string
	}{{
		description:    "ResourceQuotaConflictError does not fail taskrun",
		err:            k8sapierrors.NewConflict(k8sruntimeschema.GroupResource{Group: "v1", Resource: "resourcequotas"}, "dummy", errors.New("operation cannot be fulfilled on resourcequotas dummy the object has been modified please apply your changes to the latest version and try again")),
		expectedType:   apis.ConditionSucceeded,
		expectedStatus: corev1.ConditionUnknown,
		expectedReason: podconvert.ReasonPending,
	}, {
		description:    "exceeded quota errors are surfaced in taskrun condition but do not fail taskrun",
		err:            k8sapierrors.NewForbidden(k8sruntimeschema.GroupResource{Group: "foo", Resource: "bar"}, "baz", errors.New("exceeded quota")),
		expectedType:   apis.ConditionSucceeded,
		expectedStatus: corev1.ConditionUnknown,
		expectedReason: podconvert.ReasonExceededResourceQuota,
	}, {
		description:    "taskrun validation failed",
		err:            errors.New("TaskRun validation failed"),
		expectedType:   apis.ConditionSucceeded,
		expectedStatus: corev1.ConditionFalse,
		expectedReason: podconvert.ReasonFailedValidation,
	}, {
		description:    "errors other than exceeded quota fail the taskrun",
		err:            errors.New("this is a fatal error"),
		expectedType:   apis.ConditionSucceeded,
		expectedStatus: corev1.ConditionFalse,
		expectedReason: podconvert.ReasonPodCreationFailed,
	}, {
		description: "errors violating PodSecurity fail the taskrun",
		err: k8sapierrors.NewForbidden(k8sruntimeschema.GroupResource{Group: "foo", Resource: "bar"}, "baz",
			errors.New("violates PodSecurity \"restricted:latest\": allowPrivilegeEscalation != false ("+
				"containers \"prepare\", \"place-scripts\", \"test-task\", \"test-task\" must set securityContext."+
				"allowPrivilegeEscalation=false)")),
		expectedType:   apis.ConditionSucceeded,
		expectedStatus: corev1.ConditionFalse,
		expectedReason: podconvert.ReasonPodAdmissionFailed,
	}, {
		description: "errors validating security context constraint (Openshift) fail the taskrun",
		err: k8sapierrors.NewForbidden(k8sruntimeschema.GroupResource{Group: "foo", Resource: "bar"}, "baz",
			errors.New("unable to validate against any security context constraint: [provider restricted: .spec.securityContext.hostNetwork: Invalid value: true: Host network is not allowed to be used provider restricted: .spec.securityContext.hostPID: Invalid value: true: Host PID is not allowed to be used")),
		expectedType:   apis.ConditionSucceeded,
		expectedStatus: corev1.ConditionFalse,
		expectedReason: podconvert.ReasonPodAdmissionFailed,
	},
	}
	for _, tc := range testcases {
		t.Run(tc.description, func(t *testing.T) {
			c.handlePodCreationError(taskRun, tc.err)
			foundCondition := false
			reason := ""
			var status corev1.ConditionStatus
			for _, cond := range taskRun.Status.Conditions {
				if cond.Type == tc.expectedType {
					reason = cond.Reason
					status = cond.Status
					if status == tc.expectedStatus && reason == tc.expectedReason {
						foundCondition = true
						break
					}
				}
			}
			if !foundCondition {
				t.Errorf("expected to find condition type %q, status %q and reason %q [Found reason: %q ] [Found status: %q]", tc.expectedType, tc.expectedStatus, tc.expectedReason, reason, status)
			}
		})
	}
}
func TestReconcile_Single_SidecarState(t *testing.T) {
	runningState := corev1.ContainerStateRunning{StartedAt: metav1.Time{Time: now}}
	taskRun := parse.MustParseV1TaskRun(t, `
metadata:
  name: test-taskrun-sidecars
spec:
  taskRef:
    name: test-task-sidecar
status:
  sidecars:
  - container: sidecar-sidecar
    imageID: image-id
    name: sidecar
    running:
      startedAt: "2022-01-01T00:00:00Z"
`)

	d := test.Data{
		TaskRuns: []*v1.TaskRun{taskRun},
		Tasks:    []*v1.Task{taskSidecar},
	}

	testAssets, cancel := getTaskRunController(t, d)
	defer cancel()
	clients := testAssets.Clients

	if err := testAssets.Controller.Reconciler.Reconcile(context.Background(), getRunName(taskRun)); err != nil {
		t.Errorf("expected no error reconciling valid TaskRun but got %v", err)
	}

	getTaskRun, err := clients.Pipeline.TektonV1().TaskRuns(taskRun.Namespace).Get(testAssets.Ctx, taskRun.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Expected completed TaskRun %s to exist but instead got error when getting it: %v", taskRun.Name, err)
	}

	expected := v1.SidecarState{
		Name:      "sidecar",
		ImageID:   "image-id",
		Container: "sidecar-sidecar",
		ContainerState: corev1.ContainerState{
			Running: &runningState,
		},
	}

	if c := cmp.Diff(expected, getTaskRun.Status.Sidecars[0]); c != "" {
		t.Errorf("TestReconcile_Single_SidecarState %s", diff.PrintWantGot(c))
	}
}

func TestReconcile_Multiple_SidecarStates(t *testing.T) {
	runningState := corev1.ContainerStateRunning{StartedAt: metav1.Time{Time: now}}
	waitingState := corev1.ContainerStateWaiting{Reason: "PodInitializing"}
	taskRun := parse.MustParseV1TaskRun(t, `
metadata:
  name: test-taskrun-sidecars
spec:
  taskRef:
    name: test-task-sidecar
status:
  sidecars:
  - container: sidecar-sidecar1
    imageID: image-id
    name: sidecar1
    running:
      startedAt: "2022-01-01T00:00:00Z"
  - container: sidecar-sidecar2
    imageID: image-id
    name: sidecar2
    waiting:
      reason: PodInitializing
`)

	d := test.Data{
		TaskRuns: []*v1.TaskRun{taskRun},
		Tasks:    []*v1.Task{taskMultipleSidecars},
	}

	testAssets, cancel := getTaskRunController(t, d)
	defer cancel()
	clients := testAssets.Clients

	if err := testAssets.Controller.Reconciler.Reconcile(context.Background(), getRunName(taskRun)); err != nil {
		t.Errorf("expected no error reconciling valid TaskRun but got %v", err)
	}

	getTaskRun, err := clients.Pipeline.TektonV1().TaskRuns(taskRun.Namespace).Get(testAssets.Ctx, taskRun.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Expected completed TaskRun %s to exist but instead got error when getting it: %v", taskRun.Name, err)
	}

	expected := []v1.SidecarState{
		{
			Name:      "sidecar1",
			ImageID:   "image-id",
			Container: "sidecar-sidecar1",
			ContainerState: corev1.ContainerState{
				Running: &runningState,
			},
		},
		{
			Name:      "sidecar2",
			ImageID:   "image-id",
			Container: "sidecar-sidecar2",
			ContainerState: corev1.ContainerState{
				Waiting: &waitingState,
			},
		},
	}

	for i, sc := range getTaskRun.Status.Sidecars {
		if c := cmp.Diff(expected[i], sc); c != "" {
			t.Errorf("TestReconcile_Multiple_SidecarStates sidecar%d %s", i+1, diff.PrintWantGot(c))
		}
	}
}

// TestReconcileWorkspaceMissing tests a reconcile of a TaskRun that does
// not include a Workspace that the Task is expecting.
func TestReconcileWorkspaceMissing(t *testing.T) {
	taskWithWorkspace := parse.MustParseV1Task(t, `
metadata:
  name: test-task-with-workspace
  namespace: foo
spec:
  workspaces:
  - description: a test task workspace
    name: ws1
    readOnly: true
`)
	taskRun := parse.MustParseV1TaskRun(t, `
metadata:
  name: test-taskrun-missing-workspace
  namespace: foo
spec:
  taskRef:
    apiVersion: a1
    name: test-task-with-workspace
`)
	d := test.Data{
		Tasks:        []*v1.Task{taskWithWorkspace},
		TaskRuns:     []*v1.TaskRun{taskRun},
		ClusterTasks: nil,
	}
	testAssets, cancel := getTaskRunController(t, d)
	defer cancel()
	clients := testAssets.Clients

	err := testAssets.Controller.Reconciler.Reconcile(context.Background(), getRunName(taskRun))
	if err == nil {
		t.Fatalf("expected error reconciling invalid TaskRun but got none")
	}
	if !controller.IsPermanentError(err) {
		t.Fatalf("Expected to see a permanent error when reconciling invalid TaskRun, got %s instead", err)
	}

	tr, err := clients.Pipeline.TektonV1().TaskRuns(taskRun.Namespace).Get(testAssets.Ctx, taskRun.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Expected TaskRun %s to exist but instead got error when getting it: %v", taskRun.Name, err)
	}

	failedCorrectly := false
	for _, c := range tr.Status.Conditions {
		if c.Type == apis.ConditionSucceeded && c.Status == corev1.ConditionFalse && c.Reason == podconvert.ReasonFailedValidation {
			failedCorrectly = true
		}
	}
	if !failedCorrectly {
		t.Fatalf("Expected TaskRun to fail validation but it did not. Final conditions were:\n%#v", tr.Status.Conditions)
	}
}

// TestReconcileValidDefaultWorkspace tests a reconcile of a TaskRun that does
// not include a Workspace that the Task is expecting and it uses the default Workspace instead.
func TestReconcileValidDefaultWorkspace(t *testing.T) {
	taskWithWorkspace := parse.MustParseV1Task(t, `
metadata:
  name: test-task-with-workspace
  namespace: foo
spec:
  steps:
  - command:
    - /mycmd
    image: foo
    name: simple-step
  workspaces:
  - description: a test task workspace
    name: ws1
    readOnly: true
`)
	taskRun := parse.MustParseV1TaskRun(t, `
metadata:
  name: test-taskrun-default-workspace
  namespace: foo
spec:
  taskRef:
    apiVersion: a1
    name: test-task-with-workspace
`)
	d := test.Data{
		Tasks:        []*v1.Task{taskWithWorkspace},
		TaskRuns:     []*v1.TaskRun{taskRun},
		ClusterTasks: nil,
	}

	d.ConfigMaps = append(d.ConfigMaps, &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: config.GetDefaultsConfigName(), Namespace: system.Namespace()},
		Data: map[string]string{
			"default-task-run-workspace-binding": "emptyDir: {}",
		},
	})
	testAssets, cancel := getTaskRunController(t, d)
	defer cancel()
	clients := testAssets.Clients

	createServiceAccount(t, testAssets, "default", "foo")

	if err := testAssets.Controller.Reconciler.Reconcile(testAssets.Ctx, getRunName(taskRun)); err == nil {
		// No error is ok.
	} else if ok, _ := controller.IsRequeueKey(err); !ok {
		t.Errorf("Expected no error reconciling valid TaskRun but got %v", err)
	}

	tr, err := clients.Pipeline.TektonV1().TaskRuns(taskRun.Namespace).Get(testAssets.Ctx, taskRun.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Expected TaskRun %s to exist but instead got error when getting it: %v", taskRun.Name, err)
	}

	for _, c := range tr.Status.Conditions {
		if c.Type == apis.ConditionSucceeded && c.Status == corev1.ConditionFalse && c.Reason == podconvert.ReasonFailedValidation {
			t.Errorf("Expected TaskRun to pass Validation by using the default workspace but it did not. Final conditions were:\n%#v", tr.Status.Conditions)
		}
	}
}

// TestReconcileInvalidDefaultWorkspace tests a reconcile of a TaskRun that does
// not include a Workspace that the Task is expecting, and gets an error updating
// the TaskRun with an invalid default workspace.
func TestReconcileInvalidDefaultWorkspace(t *testing.T) {
	taskWithWorkspace := parse.MustParseV1Task(t, `
metadata:
  name: test-task-with-workspace
  namespace: foo
spec:
  steps:
  - command:
    - /mycmd
    image: foo
    name: simple-step
  workspaces:
  - description: a test task workspace
    name: ws1
    readOnly: true
`)
	taskRun := parse.MustParseV1TaskRun(t, `
metadata:
  name: test-taskrun-default-workspace
  namespace: foo
spec:
  taskRef:
    apiVersion: a1
    name: test-task-with-workspace
`)
	d := test.Data{
		Tasks:        []*v1.Task{taskWithWorkspace},
		TaskRuns:     []*v1.TaskRun{taskRun},
		ClusterTasks: nil,
	}

	d.ConfigMaps = append(d.ConfigMaps, &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: config.GetDefaultsConfigName(), Namespace: system.Namespace()},
		Data: map[string]string{
			"default-task-run-workspace-binding": "emptyDir == {}",
		},
	})
	testAssets, cancel := getTaskRunController(t, d)
	defer cancel()
	clients := testAssets.Clients

	t.Logf("Creating SA %s in %s", "default", "foo")
	if _, err := clients.Kube.CoreV1().ServiceAccounts("foo").Create(testAssets.Ctx, &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "default",
			Namespace: "foo",
		},
	}, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}

	if err := testAssets.Controller.Reconciler.Reconcile(context.Background(), getRunName(taskRun)); err == nil {
		t.Errorf("Expected error reconciling invalid TaskRun due to invalid workspace but got %v", err)
	}
}

// TestReconcileValidDefaultWorkspaceOmittedOptionalWorkspace tests a reconcile
// of a TaskRun that has omitted a Workspace that the Task has marked as optional
// with a Default TaskRun workspace defined. The default workspace should not be
// injected in place of the omitted optional workspace.
func TestReconcileValidDefaultWorkspaceOmittedOptionalWorkspace(t *testing.T) {
	optionalWorkspaceMountPath := "/foo/bar/baz"
	taskWithOptionalWorkspace := parse.MustParseV1Task(t, `
metadata:
  name: test-task-with-optional-workspace
  namespace: default
spec:
  steps:
  - command:
    - /mycmd
    image: foo
    name: simple-step
  workspaces:
  - mountPath: /foo/bar/baz
    name: optional-ws
    optional: true
`)
	taskRunOmittingWorkspace := parse.MustParseV1TaskRun(t, `
metadata:
  name: test-taskrun
  namespace: default
spec:
  taskRef:
    name: test-task-with-optional-workspace
`)

	d := test.Data{
		Tasks:    []*v1.Task{taskWithOptionalWorkspace},
		TaskRuns: []*v1.TaskRun{taskRunOmittingWorkspace},
	}

	d.ConfigMaps = append(d.ConfigMaps, &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: config.GetDefaultsConfigName(), Namespace: system.Namespace()},
		Data: map[string]string{
			"default-task-run-workspace-binding": "emptyDir: {}",
		},
	})
	testAssets, cancel := getTaskRunController(t, d)
	defer cancel()
	clients := testAssets.Clients

	createServiceAccount(t, testAssets, "default", "default")

	if err := testAssets.Controller.Reconciler.Reconcile(testAssets.Ctx, getRunName(taskRunOmittingWorkspace)); err == nil {
		t.Error("Wanted a wrapped requeue error, but got nil.")
	} else if ok, _ := controller.IsRequeueKey(err); !ok {
		t.Errorf("Unexpected reconcile error for TaskRun %q: %v", taskRunOmittingWorkspace.Name, err)
	}

	tr, err := clients.Pipeline.TektonV1().TaskRuns(taskRunOmittingWorkspace.Namespace).Get(testAssets.Ctx, taskRunOmittingWorkspace.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Error getting TaskRun %q: %v", taskRunOmittingWorkspace.Name, err)
	}

	pod, err := clients.Kube.CoreV1().Pods(taskRunOmittingWorkspace.Namespace).Get(testAssets.Ctx, tr.Status.PodName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Error getting Pod for TaskRun %q: %v", taskRunOmittingWorkspace.Name, err)
	}
	for _, c := range pod.Spec.Containers {
		for _, vm := range c.VolumeMounts {
			if vm.MountPath == optionalWorkspaceMountPath {
				t.Errorf("Workspace with VolumeMount at %s should not have been found for Optional Workspace but was injected by Default TaskRun Workspace", optionalWorkspaceMountPath)
			}
		}
	}

	for _, c := range tr.Status.Conditions {
		if c.Type == apis.ConditionSucceeded && c.Status == corev1.ConditionFalse {
			t.Errorf("Unexpected unsuccessful condition for TaskRun %q:\n%#v", taskRunOmittingWorkspace.Name, tr.Status.Conditions)
		}
	}
}

// TestReconcileWithWorkspacesIncompatibleWithAffinityAssistant tests that a TaskRun used with an associated
// Affinity Assistant is validated and that the validation fails for a TaskRun that is incompatible with
// Affinity Assistant; e.g. using more than one PVC-backed workspace.
func TestReconcileWithWorkspacesIncompatibleWithAffinityAssistant(t *testing.T) {
	taskWithTwoWorkspaces := parse.MustParseV1Task(t, `
metadata:
  name: test-task-two-workspaces
  namespace: foo
spec:
  workspaces:
  - description: task workspace
    name: ws1
    readOnly: true
  - description: another workspace
    name: ws2
`)
	taskRun := parse.MustParseV1TaskRun(t, `
metadata:
  annotations:
    pipeline.tekton.dev/affinity-assistant: dummy-affinity-assistant
  name: taskrun-with-two-workspaces
  namespace: foo
spec:
  taskRef:
    apiVersion: a1
    name: test-task-two-workspaces
  workspaces:
  - name: ws1
    persistentVolumeClaim:
      claimName: pvc1
  - name: ws2
    volumeClaimTemplate:
      metadata:
        name: pvc2
`)

	d := test.Data{
		Tasks:        []*v1.Task{taskWithTwoWorkspaces},
		TaskRuns:     []*v1.TaskRun{taskRun},
		ClusterTasks: nil,
	}
	testAssets, cancel := getTaskRunController(t, d)
	defer cancel()
	clients := testAssets.Clients
	createServiceAccount(t, testAssets, "default", "foo")
	_ = testAssets.Controller.Reconciler.Reconcile(testAssets.Ctx, getRunName(taskRun))

	_, err := clients.Pipeline.TektonV1().Tasks(taskRun.Namespace).Get(testAssets.Ctx, taskWithTwoWorkspaces.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("krux: %v", err)
	}

	ttt, err := clients.Pipeline.TektonV1().TaskRuns(taskRun.Namespace).Get(testAssets.Ctx, taskRun.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("expected TaskRun %s to exist but instead got error when getting it: %v", taskRun.Name, err)
	}

	if len(ttt.Status.Conditions) != 1 {
		t.Errorf("unexpected number of Conditions, expected 1 Condition")
	}

	for _, cond := range ttt.Status.Conditions {
		if cond.Reason != podconvert.ReasonFailedValidation {
			t.Errorf("unexpected Reason on the Condition, expected: %s, got: %s", podconvert.ReasonFailedValidation, cond.Reason)
		}
	}
}

// TestReconcileWorkspaceWithVolumeClaimTemplate tests a reconcile of a TaskRun that has
// a Workspace with VolumeClaimTemplate and check that it is translated to a created PersistentVolumeClaim.
func TestReconcileWorkspaceWithVolumeClaimTemplate(t *testing.T) {
	taskWithWorkspace := parse.MustParseV1Task(t, `
metadata:
  name: test-task-with-workspace
  namespace: foo
spec:
  steps:
  - command:
    - /mycmd
    image: foo
    name: simple-step
  workspaces:
  - description: a test task workspace
    name: ws1
    readOnly: true
`)
	taskRun := parse.MustParseV1TaskRun(t, `
metadata:
  name: test-taskrun-missing-workspace
  namespace: foo
spec:
  taskRef:
    apiVersion: a1
    name: test-task-with-workspace
  workspaces:
  - name: ws1
    volumeClaimTemplate:
      metadata:
        creationTimestamp: null
        name: mypvc
`)
	d := test.Data{
		Tasks:        []*v1.Task{taskWithWorkspace},
		TaskRuns:     []*v1.TaskRun{taskRun},
		ClusterTasks: nil,
	}
	testAssets, cancel := getTaskRunController(t, d)
	defer cancel()
	clients := testAssets.Clients
	createServiceAccount(t, testAssets, "default", "foo")

	if err := testAssets.Controller.Reconciler.Reconcile(testAssets.Ctx, getRunName(taskRun)); err == nil {
		t.Error("Wanted a wrapped requeue error, but got nil.")
	} else if ok, _ := controller.IsRequeueKey(err); !ok {
		t.Errorf("expected no error reconciling valid TaskRun but got %v", err)
	}

	ttt, err := clients.Pipeline.TektonV1().TaskRuns(taskRun.Namespace).Get(testAssets.Ctx, taskRun.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("expected TaskRun %s to exist but instead got error when getting it: %v", taskRun.Name, err)
	}

	for _, w := range ttt.Spec.Workspaces {
		if w.PersistentVolumeClaim != nil {
			t.Fatalf("expected workspace from volumeClaimTemplate to be translated to PVC")
		}
		expectedPVCName := volumeclaim.GetPVCNameWithoutAffinityAssistant(w.VolumeClaimTemplate.Name, w, *kmeta.NewControllerRef(ttt))
		_, err = clients.Kube.CoreV1().PersistentVolumeClaims(taskRun.Namespace).Get(testAssets.Ctx, expectedPVCName, metav1.GetOptions{})
		if err != nil {
			t.Fatalf("expected PVC %s to exist but instead got error when getting it: %v", expectedPVCName, err)
		}
	}
}

func TestFailTaskRun(t *testing.T) {
	testCases := []struct {
		name               string
		taskRun            *v1.TaskRun
		pod                *corev1.Pod
		reason             v1.TaskRunReason
		message            string
		expectedStatus     apis.Condition
		expectedStepStates []v1.StepState
	}{{
		name: "no-pod-scheduled",
		taskRun: parse.MustParseV1TaskRun(t, `
metadata:
  name: test-taskrun-run-failed
  namespace: foo
spec:
  status: TaskRunCancelled
  statusMessage: "Test cancellation message."
  taskRef:
    name: test-task
status:
  conditions:
  - status: Unknown
    type: Succeeded
`),
		reason:  "some reason",
		message: "some message",
		expectedStatus: apis.Condition{
			Type:    apis.ConditionSucceeded,
			Status:  corev1.ConditionFalse,
			Reason:  "some reason",
			Message: "some message",
		},
	}, {
		name: "pod-scheduled",
		taskRun: parse.MustParseV1TaskRun(t, `
metadata:
  name: test-taskrun-run-failed
  namespace: foo
spec:
  status: TaskRunCancelled
  taskRef:
    name: test-task
status:
  conditions:
  - status: Unknown
    type: Succeeded
  podName: foo-is-bar
`),
		pod: &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
			Namespace: "foo",
			Name:      "foo-is-bar",
		}},
		reason:  "some reason",
		message: "some message",
		expectedStatus: apis.Condition{
			Type:    apis.ConditionSucceeded,
			Status:  corev1.ConditionFalse,
			Reason:  "some reason",
			Message: "some message",
		},
	}, {
		name: "step-status-update-cancel",
		taskRun: parse.MustParseV1TaskRun(t, `
metadata:
  name: test-taskrun-run-cancel
  namespace: foo
spec:
  status: TaskRunCancelled
  statusMessage: "Test cancellation message."
  taskRef:
    name: test-task
status:
  conditions:
  - status: Unknown
    type: Succeeded
  podName: foo-is-bar
  steps:
  - running:
      startedAt: "2022-01-01T00:00:00Z"
`),
		pod: &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
			Namespace: "foo",
			Name:      "foo-is-bar",
		}},
		reason:  v1.TaskRunReasonCancelled,
		message: "TaskRun test-taskrun-run-cancel was cancelled. Test cancellation message.",
		expectedStatus: apis.Condition{
			Type:    apis.ConditionSucceeded,
			Status:  corev1.ConditionFalse,
			Reason:  v1.TaskRunReasonCancelled.String(),
			Message: "TaskRun test-taskrun-run-cancel was cancelled. Test cancellation message.",
		},
		expectedStepStates: []v1.StepState{
			{
				ContainerState: corev1.ContainerState{
					Terminated: &corev1.ContainerStateTerminated{
						ExitCode: 1,
						Reason:   v1.TaskRunReasonCancelled.String(),
					},
				},
			},
		},
	}, {
		name: "step-status-update-timeout",
		taskRun: parse.MustParseV1TaskRun(t, `
metadata:
  name: test-taskrun-run-timeout
  namespace: foo
spec:
  taskRef:
    name: test-task
  timeout: 10s
status:
  conditions:
  - status: Unknown
    type: Succeeded
  podName: foo-is-bar
  steps:
  - running:
      startedAt: "2022-01-01T00:00:00Z"
`),
		pod: &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
			Namespace: "foo",
			Name:      "foo-is-bar",
		}},
		reason:  v1.TaskRunReasonTimedOut,
		message: "TaskRun test-taskrun-run-timeout failed to finish within 10s",
		expectedStatus: apis.Condition{
			Type:    apis.ConditionSucceeded,
			Status:  corev1.ConditionFalse,
			Reason:  v1.TaskRunReasonTimedOut.String(),
			Message: "TaskRun test-taskrun-run-timeout failed to finish within 10s",
		},
		expectedStepStates: []v1.StepState{
			{
				ContainerState: corev1.ContainerState{
					Terminated: &corev1.ContainerStateTerminated{
						ExitCode: 1,
						Reason:   v1.TaskRunReasonTimedOut.String(),
					},
				},
			},
		},
	}, {
		name: "step-status-update-multiple-steps",
		taskRun: parse.MustParseV1TaskRun(t, `
metadata:
  name: test-taskrun-run-timeout-multiple-steps
  namespace: foo
spec:
  taskRef:
    name: test-task-multi-steps
  timeout: 10s
status:
  conditions:
  - status: Unknown
    type: Succeeded
  podName: foo-is-bar
  steps:
  - terminated:
      exitCode: 0
      finishedAt: "2022-01-01T00:00:00Z"
      reason: Completed
      startedAt: "2022-01-01T00:00:00Z"
  - running:
      startedAt: "2022-01-01T00:00:00Z"
  - running:
      startedAt: "2022-01-01T00:00:00Z"
`),
		pod: &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
			Namespace: "foo",
			Name:      "foo-is-bar",
		}},
		reason:  v1.TaskRunReasonTimedOut,
		message: "TaskRun test-taskrun-run-timeout-multiple-steps failed to finish within 10s",
		expectedStatus: apis.Condition{
			Type:    apis.ConditionSucceeded,
			Status:  corev1.ConditionFalse,
			Reason:  v1.TaskRunReasonTimedOut.String(),
			Message: "TaskRun test-taskrun-run-timeout-multiple-steps failed to finish within 10s",
		},
		expectedStepStates: []v1.StepState{
			{
				ContainerState: corev1.ContainerState{
					Terminated: &corev1.ContainerStateTerminated{
						ExitCode: 0,
						Reason:   "Completed",
					},
				},
			},
			{
				ContainerState: corev1.ContainerState{
					Terminated: &corev1.ContainerStateTerminated{
						ExitCode: 1,
						Reason:   v1.TaskRunReasonTimedOut.String(),
					},
				},
			},
			{
				ContainerState: corev1.ContainerState{
					Terminated: &corev1.ContainerStateTerminated{
						ExitCode: 1,
						Reason:   v1.TaskRunReasonTimedOut.String(),
					},
				},
			},
		},
	}, {
		name: "step-status-update-multiple-steps-waiting-state",
		taskRun: parse.MustParseV1TaskRun(t, `
metadata:
  name: test-taskrun-run-timeout-multiple-steps-waiting
  namespace: foo
spec:
  taskRef:
    name: test-task-multi-steps
  timeout: 10s
status:
  conditions:
  - status: Unknown
    type: Succeeded
  podName: foo-is-bar
  steps:
  - waiting:
      reason: PodInitializing
  - waiting:
      reason: PodInitializing
  - waiting:
      reason: PodInitializing
`),
		pod: &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
			Namespace: "foo",
			Name:      "foo-is-bar",
		}},
		reason:  v1.TaskRunReasonTimedOut,
		message: "TaskRun test-taskrun-run-timeout-multiple-steps-waiting failed to finish within 10s",
		expectedStatus: apis.Condition{
			Type:    apis.ConditionSucceeded,
			Status:  corev1.ConditionFalse,
			Reason:  v1.TaskRunReasonTimedOut.String(),
			Message: "TaskRun test-taskrun-run-timeout-multiple-steps-waiting failed to finish within 10s",
		},
		expectedStepStates: []v1.StepState{
			{
				ContainerState: corev1.ContainerState{
					Terminated: &corev1.ContainerStateTerminated{
						ExitCode: 1,
						Reason:   v1.TaskRunReasonTimedOut.String(),
					},
				},
			},
			{
				ContainerState: corev1.ContainerState{
					Terminated: &corev1.ContainerStateTerminated{
						ExitCode: 1,
						Reason:   v1.TaskRunReasonTimedOut.String(),
					},
				},
			},
			{
				ContainerState: corev1.ContainerState{
					Terminated: &corev1.ContainerStateTerminated{
						ExitCode: 1,
						Reason:   v1.TaskRunReasonTimedOut.String(),
					},
				},
			},
		},
	}, {
		name: "step-status-update-with-multiple-steps-and-some-continue-on-error",
		taskRun: parse.MustParseV1TaskRun(t, `
metadata:
  name: test-taskrun-run-ignore-step-error
  namespace: foo
spec:
  taskRef:
    name: test-task-multi-steps-with-ignore-error
status:
  conditions:
  - status: "True"
    type: Succeeded
  podName: foo-is-bar
  steps:
  - terminated:
      exitCode: 12
      finishedAt: "2022-01-01T00:00:00Z"
      reason: Completed
      startedAt: "2022-01-01T00:00:00Z"
  - running:
      startedAt: "2022-01-01T00:00:00Z"
`),
		pod: &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
			Namespace: "foo",
			Name:      "foo-is-bar",
		}},
		reason:  v1.TaskRunReasonTimedOut,
		message: "TaskRun test-taskrun-run-timeout-multiple-steps failed to finish within 10s",
		expectedStatus: apis.Condition{
			Type:    apis.ConditionSucceeded,
			Status:  corev1.ConditionFalse,
			Reason:  v1.TaskRunReasonTimedOut.String(),
			Message: "TaskRun test-taskrun-run-timeout-multiple-steps failed to finish within 10s",
		},
		expectedStepStates: []v1.StepState{
			{
				ContainerState: corev1.ContainerState{
					Terminated: &corev1.ContainerStateTerminated{
						ExitCode: 12,
						Reason:   "Completed",
					},
				},
			},
			{
				ContainerState: corev1.ContainerState{
					Terminated: &corev1.ContainerStateTerminated{
						ExitCode: 1,
						Reason:   v1.TaskRunReasonTimedOut.String(),
					},
				},
			},
		},
	}}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			d := test.Data{
				TaskRuns: []*v1.TaskRun{tc.taskRun},
			}
			if tc.pod != nil {
				d.Pods = []*corev1.Pod{tc.pod}
			}

			testAssets, cancel := getTaskRunController(t, d)
			defer cancel()

			// Use the test assets to create a *Reconciler directly for focused testing.
			c := &Reconciler{
				KubeClientSet:     testAssets.Clients.Kube,
				PipelineClientSet: testAssets.Clients.Pipeline,
				Clock:             testClock,
				taskRunLister:     testAssets.Informers.TaskRun.Lister(),
				limitrangeLister:  testAssets.Informers.LimitRange.Lister(),
				cloudEventClient:  testAssets.Clients.CloudEvents,
				metrics:           nil, // Not used
				entrypointCache:   nil, // Not used
				pvcHandler:        volumeclaim.NewPVCHandler(testAssets.Clients.Kube, testAssets.Logger),
				tracerProvider:    trace.NewNoopTracerProvider(),
			}

			err := c.failTaskRun(testAssets.Ctx, tc.taskRun, tc.reason, tc.message)
			if err != nil {
				t.Fatal(err)
			}
			if d := cmp.Diff(tc.taskRun.Status.GetCondition(apis.ConditionSucceeded), &tc.expectedStatus, ignoreLastTransitionTime); d != "" {
				t.Fatalf(diff.PrintWantGot(d))
			}

			if tc.expectedStepStates != nil {
				ignoreTerminatedFields := cmpopts.IgnoreFields(corev1.ContainerStateTerminated{}, "StartedAt", "FinishedAt")
				if c := cmp.Diff(tc.expectedStepStates, tc.taskRun.Status.Steps, ignoreTerminatedFields); c != "" {
					t.Errorf("test %s failed: %s", tc.name, diff.PrintWantGot(c))
				}
			}
		})
	}
}

func Test_storeTaskSpecAndConfigSource(t *testing.T) {
	tr := parse.MustParseV1TaskRun(t, `
metadata:
  annotations:
    io.annotation: value
  labels:
    lbl1: value1
  name: foo
spec:
  taskRef:
    name: foo-task
`)

	refSource := &v1.RefSource{
		URI: "https://abc.com.git",
		Digest: map[string]string{
			"sha1": "xyz",
		},
		EntryPoint: "foo/bar",
	}

	ts := v1.TaskSpec{
		Description: "foo-task",
	}
	ts1 := v1.TaskSpec{
		Description: "bar-task",
	}

	want := tr.DeepCopy()
	want.Status = v1.TaskRunStatus{
		TaskRunStatusFields: v1.TaskRunStatusFields{
			TaskSpec: ts.DeepCopy(),
			Provenance: &v1.Provenance{
				RefSource:    refSource.DeepCopy(),
				FeatureFlags: config.DefaultFeatureFlags.DeepCopy(),
			},
		},
	}
	want.ObjectMeta.Labels["tekton.dev/task"] = tr.ObjectMeta.Name

	type args struct {
		taskSpec           *v1.TaskSpec
		resolvedObjectMeta *resolutionutil.ResolvedObjectMeta
	}

	var tests = []struct {
		name           string
		reconcile1Args *args
		reconcile2Args *args
		wantTaskRun    *v1.TaskRun
	}{
		{
			name: "spec and refSource are available in the same reconcile",
			reconcile1Args: &args{
				taskSpec: &ts,
				resolvedObjectMeta: &resolutionutil.ResolvedObjectMeta{
					ObjectMeta: &tr.ObjectMeta,
					RefSource:  refSource.DeepCopy(),
				},
			},
			reconcile2Args: &args{
				taskSpec:           &ts1,
				resolvedObjectMeta: &resolutionutil.ResolvedObjectMeta{},
			},
			wantTaskRun: want,
		},
		{
			name: "spec comes in the first reconcile and refSource comes in next reconcile",
			reconcile1Args: &args{
				taskSpec: &ts,
				resolvedObjectMeta: &resolutionutil.ResolvedObjectMeta{
					ObjectMeta: &tr.ObjectMeta,
				},
			},
			reconcile2Args: &args{
				taskSpec: &ts,
				resolvedObjectMeta: &resolutionutil.ResolvedObjectMeta{
					RefSource: refSource.DeepCopy(),
				},
			},
			wantTaskRun: want,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// mock first reconcile
			if err := storeTaskSpecAndMergeMeta(context.Background(), tr, tc.reconcile1Args.taskSpec, tc.reconcile1Args.resolvedObjectMeta); err != nil {
				t.Errorf("storePipelineSpec() error = %v", err)
			}
			if d := cmp.Diff(tr, tc.wantTaskRun); d != "" {
				t.Fatalf(diff.PrintWantGot(d))
			}

			// mock second reconcile
			if err := storeTaskSpecAndMergeMeta(context.Background(), tr, tc.reconcile2Args.taskSpec, tc.reconcile2Args.resolvedObjectMeta); err != nil {
				t.Errorf("storePipelineSpec() error = %v", err)
			}
			if d := cmp.Diff(tr, tc.wantTaskRun); d != "" {
				t.Fatalf(diff.PrintWantGot(d))
			}
		})
	}
}

func Test_storeTaskSpec_metadata(t *testing.T) {
	taskrunlabels := map[string]string{"lbl1": "value1", "lbl2": "value2"}
	taskrunannotations := map[string]string{"io.annotation.1": "value1", "io.annotation.2": "value2"}
	tasklabels := map[string]string{"lbl1": "another value", "lbl2": "another value", "lbl3": "value3"}
	taskannotations := map[string]string{"io.annotation.1": "another value", "io.annotation.2": "another value", "io.annotation.3": "value3", "kubectl.kubernetes.io/last-applied-configuration": "foo-is-bar"}
	wantedlabels := map[string]string{"lbl1": "value1", "lbl2": "value2", "lbl3": "value3"}
	wantedannotations := map[string]string{"io.annotation.1": "value1", "io.annotation.2": "value2", "io.annotation.3": "value3"}

	tr := &v1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{Name: "foo", Labels: taskrunlabels, Annotations: taskrunannotations},
	}
	resolvedMeta := resolutionutil.ResolvedObjectMeta{
		ObjectMeta: &metav1.ObjectMeta{Labels: tasklabels, Annotations: taskannotations},
	}

	if err := storeTaskSpecAndMergeMeta(context.Background(), tr, &v1.TaskSpec{}, &resolvedMeta); err != nil {
		t.Errorf("storeTaskSpecAndMergeMeta error = %v", err)
	}
	if d := cmp.Diff(tr.ObjectMeta.Labels, wantedlabels); d != "" {
		t.Fatalf(diff.PrintWantGot(d))
	}
	if d := cmp.Diff(tr.ObjectMeta.Annotations, wantedannotations); d != "" {
		t.Fatalf(diff.PrintWantGot(d))
	}
}

func TestWillOverwritePodAffinity(t *testing.T) {
	affinity := &corev1.Affinity{
		PodAffinity: &corev1.PodAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
				{
					Namespaces: []string{"tekton-pipelines"},
				},
			},
		},
	}
	affinityAssistantName := "pipeline.tekton.dev/affinity-assistant"

	tcs := []struct {
		name                string
		hasTemplateAffinity bool
		annotations         map[string]string
		expected            bool
	}{
		{
			name:     "no settings",
			expected: false,
		},
		{
			name: "no PodTemplate affinity set",
			annotations: map[string]string{
				affinityAssistantName: "affinity-assistant",
			},
			expected: false,
		},
		{
			name:                "affinity assistant not set",
			hasTemplateAffinity: true,
			expected:            false,
		},
		{
			name:                "PodTemplate affinity will be overwritten with affinity assistant",
			hasTemplateAffinity: true,
			annotations: map[string]string{
				affinityAssistantName: "affinity-assistant",
			},
			expected: true,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			tr := &v1.TaskRun{
				Spec: v1.TaskRunSpec{
					PodTemplate: &pod.Template{},
				},
				ObjectMeta: metav1.ObjectMeta{
					Annotations: tc.annotations,
				},
			}
			if tc.hasTemplateAffinity {
				tr.Spec.PodTemplate.Affinity = affinity
			}

			if got := willOverwritePodSetAffinity(tr); got != tc.expected {
				t.Errorf("expected: %t got: %t", tc.expected, got)
			}
		})
	}
}

func TestPodAdoption(t *testing.T) {
	tr := parse.MustParseV1TaskRun(t, `
metadata:
  labels:
    mylabel: myvalue
  name: test-taskrun
  namespace: foo
spec:
  taskSpec:
    steps:
    - command:
      - /mycmd
      image: myimage
      name: mycontainer
`)

	d := test.Data{
		TaskRuns: []*v1.TaskRun{tr},
	}
	testAssets, cancel := getTaskRunController(t, d)
	defer cancel()
	c := testAssets.Controller
	clients := testAssets.Clients
	createServiceAccount(t, testAssets, "default", tr.Namespace)

	// Reconcile the TaskRun.  This creates a Pod.
	if err := c.Reconciler.Reconcile(testAssets.Ctx, getRunName(tr)); err == nil {
		t.Error("Wanted a wrapped requeue error, but got nil.")
	} else if ok, _ := controller.IsRequeueKey(err); !ok {
		t.Errorf("Error reconciling TaskRun. Got error %v", err)
	}

	// Get the updated TaskRun.
	reconciledRun, err := clients.Pipeline.TektonV1().TaskRuns(tr.Namespace).Get(testAssets.Ctx, tr.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Error getting updated TaskRun: %v", err)
	}

	// Save the name of the Pod that was created.
	podName := reconciledRun.Status.PodName
	if podName == "" {
		t.Fatal("Expected a pod to be created but the pod name is not set in the TaskRun")
	}

	// Add a label to the TaskRun.  This tests a scenario in issue 3656 which could prevent the reconciler
	// from finding a Pod when the pod name is missing from the status.
	reconciledRun.ObjectMeta.Labels["bah"] = "humbug"
	reconciledRun, err = clients.Pipeline.TektonV1().TaskRuns(tr.Namespace).Update(testAssets.Ctx, reconciledRun, metav1.UpdateOptions{})
	if err != nil {
		t.Fatalf("Unexpected error when updating status: %v", err)
	}

	// The label update triggers another reconcile.  Depending on timing, the TaskRun passed to the reconcile may or may not
	// have the updated status with the name of the created pod.  Clear the status because we want to test the case where the
	// status does not have the pod name.
	reconciledRun.Status = v1.TaskRunStatus{}
	if _, err := clients.Pipeline.TektonV1().TaskRuns("foo").UpdateStatus(testAssets.Ctx, reconciledRun, metav1.UpdateOptions{}); err != nil {
		t.Fatalf("Unexpected error when updating status: %v", err)
	}

	// Reconcile the TaskRun again.
	if err := c.Reconciler.Reconcile(testAssets.Ctx, getRunName(tr)); err == nil {
		t.Error("Wanted a wrapped requeue error, but got nil.")
	} else if ok, _ := controller.IsRequeueKey(err); !ok {
		t.Errorf("Error reconciling TaskRun again. Got error %v", err)
	}

	// Get the updated TaskRun.
	reconciledRun, err = clients.Pipeline.TektonV1().TaskRuns(tr.Namespace).Get(testAssets.Ctx, tr.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Error getting updated TaskRun after second reconcile: %v", err)
	}

	// Verify that the reconciler found the existing pod instead of creating a new one.
	if reconciledRun.Status.PodName != podName {
		t.Fatalf("First reconcile created pod %s but TaskRun now has another pod name %s", podName, reconciledRun.Status.PodName)
	}
}

func TestStopSidecars_ClientGetPodForTaskSpecWithSidecars(t *testing.T) {
	tr := parse.MustParseV1TaskRun(t, `
metadata:
  name: test-taskrun
  namespace: foo
status:
  conditions:
  - status: "True"
    type: Succeeded
  podName: test-taskrun-pod
  sidecars:
  - running:
      startedAt: "2000-01-01T01:01:01Z"
  startTime: "2000-01-01T01:01:01Z"
`)

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-taskrun-pod",
			Namespace: "foo",
		},
	}

	d := test.Data{
		Pods:     []*corev1.Pod{pod},
		TaskRuns: []*v1.TaskRun{tr},
	}

	testAssets, cancel := getTaskRunController(t, d)
	defer cancel()
	c := testAssets.Controller
	clients := testAssets.Clients
	reconcileErr := c.Reconciler.Reconcile(testAssets.Ctx, getRunName(tr))

	// We do not expect an error
	if reconcileErr != nil {
		t.Errorf("Expected no error to be returned by reconciler: %v", reconcileErr)
	}

	// Verify that the pod was retrieved.
	getPodFound := false
	for _, action := range clients.Kube.Actions() {
		if action.Matches("get", "pods") {
			getPodFound = true
			break
		}
	}
	if !getPodFound {
		t.Errorf("expected the pod to be retrieved to check if sidecars need to be stopped")
	}
}

func TestStopSidecars_WithInjectedSidecarsNoTaskSpecSidecars(t *testing.T) {
	taskRun := parse.MustParseV1TaskRun(t, `
metadata:
  name: test-taskrun-injected-sidecars
  namespace: foo
spec:
  taskRef:
    name: test-task
status:
  podName: test-taskrun-injected-sidecars-pod
  conditions:
  - message: Build succeeded
    reason: Build succeeded
    status: "True"
    type: Succeeded
`)

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-taskrun-injected-sidecars-pod",
			Namespace: "foo",
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "step-do-something",
					Image: "my-step-image",
				},
				{
					Name:  "injected-sidecar",
					Image: "some-image",
				},
			},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			ContainerStatuses: []corev1.ContainerStatus{
				{
					Name:  "step-do-something",
					State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{}},
				},
				{
					Name:  "injected-sidecar",
					State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}},
				},
			},
		},
	}

	d := test.Data{
		Pods:     []*corev1.Pod{pod},
		TaskRuns: []*v1.TaskRun{taskRun},
		Tasks:    []*v1.Task{simpleTask},
	}

	testAssets, cancel := getTaskRunController(t, d)
	defer cancel()
	c := testAssets.Controller
	clients := testAssets.Clients

	reconcileErr := c.Reconciler.Reconcile(testAssets.Ctx, getRunName(taskRun))

	// We do not expect an error
	if reconcileErr != nil {
		t.Errorf("Expected no error to be returned by reconciler: %v", reconcileErr)
	}

	retrievedPod, err := clients.Kube.CoreV1().Pods(pod.Namespace).Get(testAssets.Ctx, pod.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("error retrieving pod: %s", err)
	}

	if len(retrievedPod.Spec.Containers) != 2 {
		t.Fatalf("expected pod with two containers")
	}

	// check that injected sidecar is replaced with nop image
	if d := cmp.Diff(retrievedPod.Spec.Containers[1].Image, images.NopImage); d != "" {
		t.Errorf("expected injected sidecar image to be replaced with nop image %s", diff.PrintWantGot(d))
	}
}

func Test_validateTaskSpecRequestResources_ValidResources(t *testing.T) {
	tcs := []struct {
		name     string
		taskSpec *v1.TaskSpec
	}{{
		name: "no requested resources",
		taskSpec: &v1.TaskSpec{
			Steps: []v1.Step{
				{
					Image:   "image",
					Command: []string{"cmd"},
				}},
			StepTemplate: &v1.StepTemplate{
				ComputeResources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("4"),
						corev1.ResourceMemory: resource.MustParse("8Gi"),
					},
				},
			},
		},
	}, {
		name: "no limit configured",
		taskSpec: &v1.TaskSpec{
			Steps: []v1.Step{{
				Image:   "image",
				Command: []string{"cmd"},
				ComputeResources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("8"),
						corev1.ResourceMemory: resource.MustParse("4Gi"),
					},
				},
			}},
		},
	}, {
		name: "request less or equal than step limit but larger than steptemplate limit",
		taskSpec: &v1.TaskSpec{
			Steps: []v1.Step{{
				Image:   "image",
				Command: []string{"cmd"},
				ComputeResources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("8"),
						corev1.ResourceMemory: resource.MustParse("4Gi"),
					},
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("8"),
						corev1.ResourceMemory: resource.MustParse("8Gi"),
					},
				},
			}},
			StepTemplate: &v1.StepTemplate{
				ComputeResources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("4"),
						corev1.ResourceMemory: resource.MustParse("4Gi"),
					},
				},
			},
		},
	}, {
		name: "request less or equal than step limit",
		taskSpec: &v1.TaskSpec{
			Steps: []v1.Step{{
				Image:   "image",
				Command: []string{"cmd"},
				ComputeResources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("8"),
						corev1.ResourceMemory: resource.MustParse("4Gi"),
					},
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("8"),
						corev1.ResourceMemory: resource.MustParse("8Gi"),
					},
				},
			}},
		},
	}, {
		name: "request less or equal than steptemplate limit",
		taskSpec: &v1.TaskSpec{
			Steps: []v1.Step{{
				Image:   "image",
				Command: []string{"cmd"},
				ComputeResources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("8"),
						corev1.ResourceMemory: resource.MustParse("4Gi"),
					},
				},
			}},
			StepTemplate: &v1.StepTemplate{
				ComputeResources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("8"),
						corev1.ResourceMemory: resource.MustParse("8Gi"),
					},
				},
			},
		},
	}}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			if err := validateTaskSpecRequestResources(tc.taskSpec); err != nil {
				t.Fatalf("Expected to see error when validating invalid TaskSpec resources but saw none")
			}
		})
	}
}

func Test_validateTaskSpecRequestResources_InvalidResources(t *testing.T) {
	tcs := []struct {
		name     string
		taskSpec *v1.TaskSpec
	}{{
		name: "step request larger than step limit",
		taskSpec: &v1.TaskSpec{
			Steps: []v1.Step{{
				Image:   "image",
				Command: []string{"cmd"},
				ComputeResources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("8"),
						corev1.ResourceMemory: resource.MustParse("8Gi"),
					},
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("8"),
						corev1.ResourceMemory: resource.MustParse("4Gi"),
					},
				},
			}}},
	}, {
		name: "step request larger than steptemplate limit",
		taskSpec: &v1.TaskSpec{
			Steps: []v1.Step{{
				Image:   "image",
				Command: []string{"cmd"},
				ComputeResources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("8"),
						corev1.ResourceMemory: resource.MustParse("8Gi"),
					},
				},
			}},
			StepTemplate: &v1.StepTemplate{
				ComputeResources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("8"),
						corev1.ResourceMemory: resource.MustParse("4Gi"),
					},
				},
			},
		},
	}}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			if err := validateTaskSpecRequestResources(tc.taskSpec); err == nil {
				t.Fatalf("Expected to see error when validating invalid TaskSpec resources but saw none")
			}
		})
	}
}

func podVolumeMounts(idx, totalSteps int) []corev1.VolumeMount {
	var mnts []corev1.VolumeMount
	mnts = append(mnts, corev1.VolumeMount{
		Name:      "tekton-internal-bin",
		MountPath: "/tekton/bin",
		ReadOnly:  true,
	})
	for i := 0; i < totalSteps; i++ {
		mnts = append(mnts, corev1.VolumeMount{
			Name:      fmt.Sprintf("tekton-internal-run-%d", i),
			MountPath: filepath.Join("/tekton/run", strconv.Itoa(i)),
			ReadOnly:  i != idx,
		})
	}
	if idx == 0 {
		mnts = append(mnts, corev1.VolumeMount{
			Name:      "tekton-internal-downward",
			MountPath: "/tekton/downward",
			ReadOnly:  true,
		})
	}
	mnts = append(mnts, corev1.VolumeMount{
		Name:      fmt.Sprintf("tekton-creds-init-home-%d", idx),
		MountPath: "/tekton/creds",
	})
	mnts = append(mnts, corev1.VolumeMount{
		Name:      "tekton-internal-workspace",
		MountPath: workspaceDir,
	})
	mnts = append(mnts, corev1.VolumeMount{
		Name:      "tekton-internal-home",
		MountPath: "/tekton/home",
	})
	mnts = append(mnts, corev1.VolumeMount{
		Name:      "tekton-internal-results",
		MountPath: "/tekton/results",
	})
	mnts = append(mnts, corev1.VolumeMount{
		Name:      "tekton-internal-steps",
		MountPath: "/tekton/steps",
		ReadOnly:  true,
	})

	return mnts
}

func podArgs(cmd string, stdoutPath string, stderrPath string, additionalArgs []string, idx int) []string {
	args := []string{
		"-wait_file",
	}
	if idx == 0 {
		args = append(args, "/tekton/downward/ready", "-wait_file_content")
	} else {
		args = append(args, fmt.Sprintf("/tekton/run/%d/out", idx-1))
	}
	args = append(args,
		"-post_file",
		fmt.Sprintf("/tekton/run/%d/out", idx),
		"-termination_path",
		"/tekton/termination",
		"-step_metadata_dir",
		fmt.Sprintf("/tekton/run/%d/status", idx),
	)
	if stdoutPath != "" {
		args = append(args, "-stdout_path", stdoutPath)
	}
	if stderrPath != "" {
		args = append(args, "-stderr_path", stderrPath)
	}
	args = append(args,
		"-entrypoint",
		cmd,
		"--",
	)

	args = append(args, additionalArgs...)

	return args
}

func podObjectMeta(name, taskName, taskRunName, ns string, isClusterTask bool) metav1.ObjectMeta {
	trueB := true
	om := metav1.ObjectMeta{
		Name:      name,
		Namespace: ns,
		Annotations: map[string]string{
			podconvert.ReleaseAnnotation: fakeVersion,
		},
		Labels: map[string]string{
			pipeline.TaskRunLabelKey:       taskRunName,
			"app.kubernetes.io/managed-by": "tekton-pipelines",
		},
		OwnerReferences: []metav1.OwnerReference{{
			Kind:               "TaskRun",
			Name:               taskRunName,
			Controller:         &trueB,
			BlockOwnerDeletion: &trueB,
			APIVersion:         currentAPIVersion,
		}},
	}

	if taskName != "" {
		if isClusterTask {
			om.Labels[pipeline.ClusterTaskLabelKey] = taskName
		} else {
			om.Labels[pipeline.TaskLabelKey] = taskName
		}
	}

	return om
}

type stepForExpectedPod struct {
	name            string
	image           string
	cmd             string
	args            []string
	envVars         map[string]string
	workingDir      string
	securityContext *corev1.SecurityContext
	stdoutPath      string
	stderrPath      string
}

func expectedPod(podName, taskName, taskRunName, ns, saName string, isClusterTask bool, extraVolumes []corev1.Volume, steps []stepForExpectedPod) *corev1.Pod {
	stepNames := make([]string, 0, len(steps))
	for _, s := range steps {
		stepNames = append(stepNames, fmt.Sprintf("step-%s", s.name))
	}
	p := &corev1.Pod{
		ObjectMeta: podObjectMeta(podName, taskName, taskRunName, ns, isClusterTask),
		Spec: corev1.PodSpec{
			Volumes: []corev1.Volume{
				workspaceVolume,
				homeVolume,
				resultsVolume,
				stepsVolume,
				binVolume,
				downwardVolume,
			},
			InitContainers:        []corev1.Container{placeToolsInitContainer(stepNames)},
			RestartPolicy:         corev1.RestartPolicyNever,
			ActiveDeadlineSeconds: &defaultActiveDeadlineSeconds,
			ServiceAccountName:    saName,
		},
	}

	for idx, s := range steps {
		p.Spec.Volumes = append(p.Spec.Volumes, corev1.Volume{
			Name:         fmt.Sprintf("tekton-creds-init-home-%d", idx),
			VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{Medium: corev1.StorageMediumMemory}},
		})
		p.Spec.Volumes = append(p.Spec.Volumes, runVolume(idx))

		stepContainer := corev1.Container{
			Image:                  s.image,
			Name:                   fmt.Sprintf("step-%s", s.name),
			Command:                []string{entrypointLocation},
			VolumeMounts:           podVolumeMounts(idx, len(steps)),
			TerminationMessagePath: "/tekton/termination",
		}
		stepContainer.Args = podArgs(s.cmd, s.stdoutPath, s.stderrPath, s.args, idx)

		for k, v := range s.envVars {
			stepContainer.Env = append(stepContainer.Env, corev1.EnvVar{
				Name:  k,
				Value: v,
			})
		}

		if s.workingDir != "" {
			stepContainer.WorkingDir = s.workingDir
		}

		if s.securityContext != nil {
			stepContainer.SecurityContext = s.securityContext
		}

		p.Spec.Containers = append(p.Spec.Containers, stepContainer)
	}

	p.Spec.Volumes = append(p.Spec.Volumes, extraVolumes...)

	return p
}

func objectMeta(name, ns string) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:        name,
		Namespace:   ns,
		Labels:      map[string]string{},
		Annotations: map[string]string{},
	}
}

func TestReconcile_validateLargerResultsSidecarLogs_invalid(t *testing.T) {
	taskRun := parse.MustParseV1TaskRun(t, `
metadata:
  name: test-taskrun-larger-results-sidecar-logs
  namespace: foo
spec:
  taskSpec:
    results:
      - name: result1
    steps:
    - script: echo foo >> $(results.result1.path)
      image: myimage
      name: mycontainer
status:
  taskSpec:
    results:
      - name: result1
    steps:
    - script: echo foo >> $(results.result1.path)
      image: myimage
      name: mycontainer
`)

	taskruns := []*v1.TaskRun{
		taskRun,
	}

	for _, tc := range []struct {
		name          string
		taskRun       *v1.TaskRun
		maxResultSize string
		expectError   bool
		reason        string
	}{{
		name:          "taskrun results too large",
		taskRun:       taskRun,
		maxResultSize: "1",
		expectError:   false,
		reason:        "TaskRunResultLargerThanAllowedLimit",
	}, {
		name:          "taskrun results bad json",
		taskRun:       taskRun,
		maxResultSize: "4096",
		expectError:   true,
		reason:        "Running",
	}} {
		t.Run(tc.name, func(t *testing.T) {
			d := test.Data{
				TaskRuns: taskruns,
				Tasks:    []*v1.Task{},
				ConfigMaps: []*corev1.ConfigMap{{
					ObjectMeta: metav1.ObjectMeta{Namespace: system.Namespace(), Name: config.GetFeatureFlagsConfigName()},
					Data: map[string]string{
						"results-from":    config.ResultExtractionMethodSidecarLogs,
						"max-result-size": tc.maxResultSize,
					},
				}},
			}
			testAssets, cancel := getTaskRunController(t, d)
			defer cancel()
			clientset := fakekubeclientset.NewSimpleClientset()
			pod := &corev1.Pod{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Pod",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-taskrun-larger-results-sidecar-logs-pod",
					Namespace: "foo",
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "sidecar-tekton-log-results",
							Image: "image",
						},
					},
				},
				Status: corev1.PodStatus{
					Phase: corev1.PodRunning,
				},
			}
			pod, err := clientset.CoreV1().Pods(pod.Namespace).Create(context.TODO(), pod, metav1.CreateOptions{})
			if err != nil {
				t.Errorf("Error occurred while creating pod %s: %s", pod.Name, err.Error())
			}
			createServiceAccount(t, testAssets, tc.taskRun.Spec.ServiceAccountName, tc.taskRun.Namespace)

			// Reconcile the TaskRun.  This creates a Pod.
			err = testAssets.Controller.Reconciler.Reconcile(testAssets.Ctx, getRunName(tc.taskRun))
			if err != nil && tc.expectError == false {
				t.Errorf("did not expect to get an error but got %v", err)
			} else if err == nil && tc.expectError == true {
				t.Error("expected to get an error but did not")
			}

			tr, err := testAssets.Clients.Pipeline.TektonV1().TaskRuns(tc.taskRun.Namespace).Get(testAssets.Ctx, tc.taskRun.Name, metav1.GetOptions{})
			if err != nil {
				t.Fatalf("getting updated taskrun: %v", err)
			}
			condition := tr.Status.GetCondition(apis.ConditionSucceeded)
			if condition.Type != apis.ConditionSucceeded || condition.Reason != tc.reason {
				t.Errorf("Expected TaskRun to terminate with %s reason. Final conditions were:\n%#v", tc.reason, tr.Status.Conditions)
			}
		})
	}
}

func TestReconcile_validateTaskRunResults_valid(t *testing.T) {
	taskRunResultsTypeMatched := parse.MustParseV1TaskRun(t, `
metadata:
  name: test-taskrun-results-type-valid
  namespace: foo
spec:
  taskRef:
    name: test-results-task
status:
  taskResults:
    - name: aResult
      type: array
      value: aResultValue
`)

	taskRunResultsObjectValid := parse.MustParseV1TaskRun(t, `
metadata:
  name: test-taskrun-results-object-valid
  namespace: foo
spec:
  taskRef:
    name: test-results-task
status:
  taskResults:
    - name: objectResult
      type: object
      value:
        url: abc
        commit: xyz
`)

	taskruns := []*v1.TaskRun{
		taskRunResultsTypeMatched, taskRunResultsObjectValid,
	}

	d := test.Data{
		TaskRuns: taskruns,
		Tasks:    []*v1.Task{resultsTask},
		ConfigMaps: []*corev1.ConfigMap{{
			ObjectMeta: metav1.ObjectMeta{Namespace: system.Namespace(), Name: config.GetFeatureFlagsConfigName()},
			Data: map[string]string{
				"enable-api-fields": config.AlphaAPIFields,
			},
		}},
	}
	for _, tc := range []struct {
		name    string
		taskRun *v1.TaskRun
	}{{
		name:    "taskrun results type valid",
		taskRun: taskRunResultsTypeMatched,
	}, {
		name:    "taskrun results object valid",
		taskRun: taskRunResultsObjectValid,
	}} {
		t.Run(tc.name, func(t *testing.T) {
			testAssets, cancel := getTaskRunController(t, d)
			defer cancel()
			createServiceAccount(t, testAssets, tc.taskRun.Spec.ServiceAccountName, tc.taskRun.Namespace)

			// Reconcile the TaskRun.  This creates a Pod.
			if err := testAssets.Controller.Reconciler.Reconcile(testAssets.Ctx, getRunName(tc.taskRun)); err == nil {
				t.Error("Wanted a wrapped requeue error, but got nil.")
			} else if ok, _ := controller.IsRequeueKey(err); !ok {
				t.Errorf("Error reconciling TaskRun. Got error %v", err)
			}

			tr, err := testAssets.Clients.Pipeline.TektonV1().TaskRuns(tc.taskRun.Namespace).Get(testAssets.Ctx, tc.taskRun.Name, metav1.GetOptions{})
			if err != nil {
				t.Fatalf("getting updated taskrun: %v", err)
			}
			condition := tr.Status.GetCondition(apis.ConditionSucceeded)
			if condition.Type != apis.ConditionSucceeded || condition.Reason != "Running" {
				t.Errorf("Expected TaskRun to succeed but it did not. Final conditions were:\n%#v", tr.Status.Conditions)
			}
		})
	}
}

func TestReconcile_validateTaskRunResults_invalid(t *testing.T) {
	taskRunResultsTypeMismatched := parse.MustParseV1TaskRun(t, `
metadata:
  name: test-taskrun-results-type-mismatched
  namespace: foo
spec:
  taskRef:
    name: test-results-task
status:
  results:
    - name: aResult
      type: string
      value: aResultValue
    - name: objectResult
      type: string
      value: objectResultValue
`)

	taskRunResultsObjectMissKey := parse.MustParseV1TaskRun(t, `
metadata:
  name: test-taskrun-results-object-miss-key
  namespace: foo
spec:
  taskRef:
    name: test-results-task
status:
  results:
    - name: aResult
      type: array
      value:
       - "1"
       - "2"
    - name: objectResult
      type: object
      value:
        url: abc
`)

	taskruns := []*v1.TaskRun{
		taskRunResultsTypeMismatched, taskRunResultsObjectMissKey,
	}

	d := test.Data{
		TaskRuns: taskruns,
		Tasks:    []*v1.Task{resultsTask},
		ConfigMaps: []*corev1.ConfigMap{{
			ObjectMeta: metav1.ObjectMeta{Namespace: system.Namespace(), Name: config.GetFeatureFlagsConfigName()},
			Data: map[string]string{
				"enable-api-fields": config.AlphaAPIFields,
			},
		}},
	}
	for _, tc := range []struct {
		name             string
		taskRun          *v1.TaskRun
		wantFailedReason string
		expectedError    error
		expectedResults  []v1.TaskRunResult
	}{{
		name:             "taskrun results type mismatched",
		taskRun:          taskRunResultsTypeMismatched,
		wantFailedReason: podconvert.ReasonFailedValidation,
		expectedError:    fmt.Errorf("1 error occurred:\n\t* Provided results don't match declared results; may be invalid JSON or missing result declaration:  \"aResult\": task result is expected to be \"array\" type but was initialized to a different type \"string\", \"objectResult\": task result is expected to be \"object\" type but was initialized to a different type \"string\""),
		expectedResults:  nil,
	}, {
		name:             "taskrun results object miss key",
		taskRun:          taskRunResultsObjectMissKey,
		wantFailedReason: podconvert.ReasonFailedValidation,
		expectedError:    fmt.Errorf("1 error occurred:\n\t* missing keys for these results which are required in TaskResult's properties map[objectResult:[commit]]"),
		expectedResults: []v1.TaskRunResult{
			{
				Name:  "aResult",
				Type:  "array",
				Value: *v1.NewStructuredValues("1", "2"),
			}, {
				Name:  "objectResult",
				Type:  "object",
				Value: *v1.NewObject(map[string]string{"url": "abc"}),
			}},
	}} {
		t.Run(tc.name, func(t *testing.T) {
			testAssets, cancel := getTaskRunController(t, d)
			defer cancel()
			createServiceAccount(t, testAssets, tc.taskRun.Spec.ServiceAccountName, tc.taskRun.Namespace)

			err := testAssets.Controller.Reconciler.Reconcile(testAssets.Ctx, getRunName(tc.taskRun))
			if d := cmp.Diff(strings.TrimSuffix(err.Error(), "\n\n"), tc.expectedError.Error()); d != "" {
				t.Errorf("Expected: %v, but Got: %v", tc.expectedError, err)
			}
			tr, err := testAssets.Clients.Pipeline.TektonV1().TaskRuns(tc.taskRun.Namespace).Get(testAssets.Ctx, tc.taskRun.Name, metav1.GetOptions{})
			if err != nil {
				t.Fatalf("getting updated taskrun: %v", err)
			}
			if d := cmp.Diff(tr.Status.Results, tc.expectedResults); d != "" {
				t.Errorf("got unexpected results %s", diff.PrintWantGot(d))
			}
			condition := tr.Status.GetCondition(apis.ConditionSucceeded)
			if condition.Type != apis.ConditionSucceeded || condition.Status != corev1.ConditionFalse || condition.Reason != tc.wantFailedReason {
				t.Errorf("Expected TaskRun to fail with reason \"%s\" but it did not. Final conditions were:\n%#v", tc.wantFailedReason, tr.Status.Conditions)
			}
		})
	}
}

func TestReconcile_ReplacementsInStatusTaskSpec(t *testing.T) {
	task := parse.MustParseV1Task(t, `
metadata:
  name: test-task-with-replacements
  namespace: foo
spec:
  params:
  - default: mydefault
    name: myarg
    type: string
  steps:
  - script: echo $(inputs.params.myarg)
    image: myimage
    name: mycontainer
`)
	tr := parse.MustParseV1TaskRun(t, `
metadata:
  name: test-taskrun-with-replacements
  namespace: foo
spec:
  params:
  - name: myarg
    value: foo
  taskRef:
    name: test-task-with-replacements
status:
  podName: the-pod
`)

	expectedStatusSpec := &v1.TaskSpec{
		Params: []v1.ParamSpec{{
			Name:    "myarg",
			Default: v1.NewStructuredValues("mydefault"),
			Type:    v1.ParamTypeString,
		}},
		Steps: []v1.Step{{
			Script: "echo foo",
			Image:  "myimage",
			Name:   "mycontainer",
		}},
	}

	d := test.Data{
		TaskRuns: []*v1.TaskRun{tr},
		Tasks:    []*v1.Task{task},
		Pods: []*corev1.Pod{{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "foo",
				Name:      "the-pod",
			},
		}},
	}
	testAssets, cancel := getTaskRunController(t, d)
	defer cancel()

	if err := testAssets.Controller.Reconciler.Reconcile(testAssets.Ctx, getRunName(tr)); err == nil {
		t.Error("Wanted a wrapped requeue error, but got nil.")
	} else if ok, _ := controller.IsRequeueKey(err); !ok {
		t.Errorf("expected no error. Got error %v", err)
	}

	updatedTR, err := testAssets.Clients.Pipeline.TektonV1().TaskRuns(tr.Namespace).Get(testAssets.Ctx, tr.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("getting updated taskrun: %v", err)
	}

	if d := cmp.Diff(expectedStatusSpec, updatedTR.Status.TaskSpec); d != "" {
		t.Errorf("expected Status.TaskSpec to match, but differed: %s", diff.PrintWantGot(d))
	}
}

func TestReconcile_verifyResolved_V1beta1Task_NoError(t *testing.T) {
	resolverName := "foobar"
	ts := parse.MustParseV1beta1Task(t, `
metadata:
  name: test-task
  namespace: foo
spec:
  params:
  - default: mydefault
    name: myarg
    type: string
  steps:
  - script: echo $(inputs.params.myarg)
    image: myimage
    name: mycontainer
`)

	signer, _, vps := test.SetupMatchAllVerificationPolicies(t, ts.Namespace)
	signedTask, err := test.GetSignedV1beta1Task(ts, signer, "test-task")
	if err != nil {
		t.Fatal("fail to sign task", err)
	}
	signedTaskBytes, err := yaml.Marshal(signedTask)
	if err != nil {
		t.Fatal("fail to marshal task", err)
	}

	noMatchPolicy := []*v1alpha1.VerificationPolicy{{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "no-match",
			Namespace: ts.Namespace,
		},
		Spec: v1alpha1.VerificationPolicySpec{
			Resources: []v1alpha1.ResourcePattern{{Pattern: "no-match"}},
		}}}
	// warnPolicy doesn't contain keys so it will fail verification but doesn't fail the run
	warnPolicy := []*v1alpha1.VerificationPolicy{{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "warn-policy",
			Namespace: ts.Namespace,
		},
		Spec: v1alpha1.VerificationPolicySpec{
			Resources: []v1alpha1.ResourcePattern{{Pattern: ".*"}},
			Mode:      v1alpha1.ModeWarn,
		}}}
	tr := parse.MustParseV1TaskRun(t, fmt.Sprintf(`
metadata:
  name: test-taskrun
  namespace: foo
spec:
  params:
  - name: myarg
    value: foo
  taskRef:
    resolver: %s
  serviceAccountName: default
status:
  podName: the-pod
`, resolverName))

	failNoMatchCondition := &apis.Condition{
		Type:    trustedresources.ConditionTrustedResourcesVerified,
		Status:  corev1.ConditionFalse,
		Message: fmt.Sprintf("failed to get matched policies: %s: no matching policies are found for resource: %s against source: %s", trustedresources.ErrNoMatchedPolicies, ts.Name, ""),
	}
	passCondition := &apis.Condition{
		Type:   trustedresources.ConditionTrustedResourcesVerified,
		Status: corev1.ConditionTrue,
	}
	failNoKeysCondition := &apis.Condition{
		Type:    trustedresources.ConditionTrustedResourcesVerified,
		Status:  corev1.ConditionFalse,
		Message: fmt.Sprintf("failed to get verifiers for resource %s from namespace %s: %s", ts.Name, ts.Namespace, verifier.ErrEmptyPublicKeys),
	}
	testCases := []struct {
		name                          string
		task                          []*v1.Task
		noMatchPolicy                 string
		verificationPolicies          []*v1alpha1.VerificationPolicy
		wantTrustedResourcesCondition *apis.Condition
	}{{
		name:                          "ignore no match policy",
		noMatchPolicy:                 config.IgnoreNoMatchPolicy,
		verificationPolicies:          noMatchPolicy,
		wantTrustedResourcesCondition: nil,
	}, {
		name:                          "warn no match policy",
		noMatchPolicy:                 config.WarnNoMatchPolicy,
		verificationPolicies:          noMatchPolicy,
		wantTrustedResourcesCondition: failNoMatchCondition,
	}, {
		name:                          "pass enforce policy",
		noMatchPolicy:                 config.FailNoMatchPolicy,
		verificationPolicies:          vps,
		wantTrustedResourcesCondition: passCondition,
	}, {
		name:                          "only fail warn policy",
		noMatchPolicy:                 config.FailNoMatchPolicy,
		verificationPolicies:          warnPolicy,
		wantTrustedResourcesCondition: failNoKeysCondition,
	},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cms := []*corev1.ConfigMap{
				{
					ObjectMeta: metav1.ObjectMeta{Name: config.GetFeatureFlagsConfigName(), Namespace: system.Namespace()},
					Data: map[string]string{
						"trusted-resources-verification-no-match-policy": tc.noMatchPolicy,
					},
				},
			}
			rr := getResolvedResolutionRequest(t, resolverName, signedTaskBytes, tr.Namespace, tr.Name)
			d := test.Data{
				TaskRuns:             []*v1.TaskRun{tr},
				ConfigMaps:           cms,
				VerificationPolicies: tc.verificationPolicies,
				ResolutionRequests:   []*resolutionv1beta1.ResolutionRequest{&rr},
			}

			testAssets, cancel := getTaskRunController(t, d)
			defer cancel()
			createServiceAccount(t, testAssets, tr.Spec.ServiceAccountName, tr.Namespace)
			err = testAssets.Controller.Reconciler.Reconcile(testAssets.Ctx, getRunName(tr))

			if ok, _ := controller.IsRequeueKey(err); !ok {
				t.Errorf("Error reconciling TaskRun. Got error %v", err)
			}
			reconciledRun, err := testAssets.Clients.Pipeline.TektonV1().TaskRuns(tr.Namespace).Get(testAssets.Ctx, tr.Name, metav1.GetOptions{})
			if err != nil {
				t.Fatalf("getting updated taskrun: %v", err)
			}
			condition := reconciledRun.Status.GetCondition(apis.ConditionSucceeded)
			if condition == nil || condition.Status != corev1.ConditionUnknown {
				t.Errorf("Expected fresh TaskRun to have in progress status, but had %v", condition)
			}
			if condition != nil && condition.Reason != v1beta1.TaskRunReasonRunning.String() {
				t.Errorf("Expected reason %q but was %s", v1beta1.TaskRunReasonRunning.String(), condition.Reason)
			}
			gotVerificationCondition := reconciledRun.Status.GetCondition(trustedresources.ConditionTrustedResourcesVerified)
			if d := cmp.Diff(tc.wantTrustedResourcesCondition, gotVerificationCondition, ignoreLastTransitionTime); d != "" {
				t.Error(diff.PrintWantGot(d))
			}
		})
	}
}

func TestReconcile_verifyResolved_V1beta1Task_Error(t *testing.T) {
	resolverName := "foobar"
	unsignedTask := parse.MustParseV1beta1Task(t, `
metadata:
  name: test-task
  namespace: foo
spec:
  params:
  - default: mydefault
    name: myarg
    type: string
  steps:
  - script: echo $(inputs.params.myarg)
    image: myimage
    name: mycontainer
`)
	unsignedTaskBytes, err := yaml.Marshal(unsignedTask)
	if err != nil {
		t.Fatal("fail to marshal task", err)
	}

	signer, _, vps := test.SetupMatchAllVerificationPolicies(t, unsignedTask.Namespace)
	signedTask, err := test.GetSignedV1beta1Task(unsignedTask, signer, "test-task")
	if err != nil {
		t.Fatal("fail to sign task", err)
	}

	modifiedTask := signedTask.DeepCopy()
	if modifiedTask.Annotations == nil {
		modifiedTask.Annotations = make(map[string]string)
	}
	modifiedTask.Annotations["random"] = "attack"
	modifiedTaskBytes, err := yaml.Marshal(modifiedTask)
	if err != nil {
		t.Fatal("fail to marshal task", err)
	}

	cms := []*corev1.ConfigMap{
		{
			ObjectMeta: metav1.ObjectMeta{Name: config.GetFeatureFlagsConfigName(), Namespace: system.Namespace()},
			Data: map[string]string{
				"trusted-resources-verification-no-match-policy": config.FailNoMatchPolicy,
				"enable-tekton-oci-bundles":                      "true",
			},
		},
	}
	tr := parse.MustParseV1TaskRun(t, fmt.Sprintf(`
metadata:
  name: test-taskrun
  namespace: foo
spec:
  params:
  - name: myarg
    value: foo
  taskRef:
    resolver: %s
    name: test-task
status:
  podName: the-pod
`, resolverName))
	testCases := []struct {
		name      string
		taskBytes []byte
	}{
		{
			name:      "unsigned task fails verification",
			taskBytes: unsignedTaskBytes,
		},
		{
			name:      "modified task fails verification",
			taskBytes: modifiedTaskBytes,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			rr := getResolvedResolutionRequest(t, resolverName, tc.taskBytes, tr.Namespace, tr.Name)
			d := test.Data{
				TaskRuns:             []*v1.TaskRun{tr},
				ConfigMaps:           cms,
				VerificationPolicies: vps,
				ResolutionRequests:   []*resolutionv1beta1.ResolutionRequest{&rr},
			}

			testAssets, cancel := getTaskRunController(t, d)
			defer cancel()
			createServiceAccount(t, testAssets, tr.Spec.ServiceAccountName, tr.Namespace)
			err := testAssets.Controller.Reconciler.Reconcile(testAssets.Ctx, getRunName(tr))

			if !errors.Is(err, trustedresources.ErrResourceVerificationFailed) {
				t.Errorf("Reconcile got %v but want %v", err, trustedresources.ErrResourceVerificationFailed)
			}
			reconciledRun, err := testAssets.Clients.Pipeline.TektonV1().TaskRuns(tr.Namespace).Get(testAssets.Ctx, tr.Name, metav1.GetOptions{})
			if err != nil {
				t.Fatalf("getting updated taskrun: %v", err)
			}
			condition := reconciledRun.Status.GetCondition(apis.ConditionSucceeded)
			if condition.Type != apis.ConditionSucceeded || condition.Status != corev1.ConditionFalse || condition.Reason != podconvert.ReasonResourceVerificationFailed {
				t.Errorf("Expected TaskRun to fail with reason \"%s\" but it did not. Final conditions were:\n%#v", podconvert.ReasonResourceVerificationFailed, tr.Status.Conditions)
			}
			gotVerificationCondition := reconciledRun.Status.GetCondition(trustedresources.ConditionTrustedResourcesVerified)
			if gotVerificationCondition == nil || gotVerificationCondition.Status != corev1.ConditionFalse {
				t.Errorf("Expected to have false condition, but had %v", gotVerificationCondition)
			}
		})
	}
}

func TestReconcile_verifyResolved_V1Task_NoError(t *testing.T) {
	resolverName := "foobar"
	ts := parse.MustParseV1Task(t, `
metadata:
  name: test-task
  namespace: foo
spec:
  params:
  - default: mydefault
    name: myarg
    type: string
  steps:
  - script: echo $(params.myarg)
    image: myimage
    name: mycontainer
`)

	signer, _, vps := test.SetupMatchAllVerificationPolicies(t, ts.Namespace)
	signedTask, err := getSignedV1Task(ts, signer, "test-task")
	if err != nil {
		t.Fatal("fail to sign task", err)
	}
	signedTaskBytes, err := yaml.Marshal(signedTask)
	if err != nil {
		t.Fatal("fail to marshal task", err)
	}

	noMatchPolicy := []*v1alpha1.VerificationPolicy{{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "no-match",
			Namespace: ts.Namespace,
		},
		Spec: v1alpha1.VerificationPolicySpec{
			Resources: []v1alpha1.ResourcePattern{{Pattern: "no-match"}},
		}}}
	// warnPolicy doesn't contain keys so it will fail verification but doesn't fail the run
	warnPolicy := []*v1alpha1.VerificationPolicy{{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "warn-policy",
			Namespace: ts.Namespace,
		},
		Spec: v1alpha1.VerificationPolicySpec{
			Resources: []v1alpha1.ResourcePattern{{Pattern: ".*"}},
			Mode:      v1alpha1.ModeWarn,
		}}}
	tr := parse.MustParseV1TaskRun(t, fmt.Sprintf(`
metadata:
  name: test-taskrun
  namespace: foo
spec:
  params:
  - name: myarg
    value: foo
  taskRef:
    resolver: %s
  serviceAccountName: default
status:
  podName: the-pod
`, resolverName))

	failNoMatchCondition := &apis.Condition{
		Type:    trustedresources.ConditionTrustedResourcesVerified,
		Status:  corev1.ConditionFalse,
		Message: fmt.Sprintf("failed to get matched policies: %s: no matching policies are found for resource: %s against source: %s", trustedresources.ErrNoMatchedPolicies, ts.Name, ""),
	}
	passCondition := &apis.Condition{
		Type:   trustedresources.ConditionTrustedResourcesVerified,
		Status: corev1.ConditionTrue,
	}
	failNoKeysCondition := &apis.Condition{
		Type:    trustedresources.ConditionTrustedResourcesVerified,
		Status:  corev1.ConditionFalse,
		Message: fmt.Sprintf("failed to get verifiers for resource %s from namespace %s: %s", ts.Name, ts.Namespace, verifier.ErrEmptyPublicKeys),
	}
	testCases := []struct {
		name                          string
		task                          []*v1beta1.Task
		noMatchPolicy                 string
		verificationPolicies          []*v1alpha1.VerificationPolicy
		wantTrustedResourcesCondition *apis.Condition
	}{{
		name:                          "ignore no match policy",
		noMatchPolicy:                 config.IgnoreNoMatchPolicy,
		verificationPolicies:          noMatchPolicy,
		wantTrustedResourcesCondition: nil,
	}, {
		name:                          "warn no match policy",
		noMatchPolicy:                 config.WarnNoMatchPolicy,
		verificationPolicies:          noMatchPolicy,
		wantTrustedResourcesCondition: failNoMatchCondition,
	}, {
		name:                          "pass enforce policy",
		noMatchPolicy:                 config.FailNoMatchPolicy,
		verificationPolicies:          vps,
		wantTrustedResourcesCondition: passCondition,
	}, {
		name:                          "only fail warn policy",
		noMatchPolicy:                 config.FailNoMatchPolicy,
		verificationPolicies:          warnPolicy,
		wantTrustedResourcesCondition: failNoKeysCondition,
	},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cms := []*corev1.ConfigMap{
				{
					ObjectMeta: metav1.ObjectMeta{Name: config.GetFeatureFlagsConfigName(), Namespace: system.Namespace()},
					Data: map[string]string{
						"trusted-resources-verification-no-match-policy": tc.noMatchPolicy,
					},
				},
			}
			rr := getResolvedResolutionRequest(t, resolverName, signedTaskBytes, tr.Namespace, tr.Name)
			d := test.Data{
				TaskRuns:             []*v1.TaskRun{tr},
				ConfigMaps:           cms,
				VerificationPolicies: tc.verificationPolicies,
				ResolutionRequests:   []*resolutionv1beta1.ResolutionRequest{&rr},
			}

			testAssets, cancel := getTaskRunController(t, d)
			defer cancel()
			createServiceAccount(t, testAssets, tr.Spec.ServiceAccountName, tr.Namespace)
			err = testAssets.Controller.Reconciler.Reconcile(testAssets.Ctx, getRunName(tr))

			if ok, _ := controller.IsRequeueKey(err); !ok {
				t.Errorf("Error reconciling TaskRun. Got error %v", err)
			}
			reconciledRun, err := testAssets.Clients.Pipeline.TektonV1().TaskRuns(tr.Namespace).Get(testAssets.Ctx, tr.Name, metav1.GetOptions{})
			if err != nil {
				t.Fatalf("getting updated taskrun: %v", err)
			}
			condition := reconciledRun.Status.GetCondition(apis.ConditionSucceeded)
			if condition == nil || condition.Status != corev1.ConditionUnknown {
				t.Errorf("Expected fresh TaskRun to have in progress status, but had %v", condition)
			}
			if condition != nil && condition.Reason != v1beta1.TaskRunReasonRunning.String() {
				t.Errorf("Expected reason %q but was %s", v1beta1.TaskRunReasonRunning.String(), condition.Reason)
			}
			gotVerificationCondition := reconciledRun.Status.GetCondition(trustedresources.ConditionTrustedResourcesVerified)
			if d := cmp.Diff(tc.wantTrustedResourcesCondition, gotVerificationCondition, ignoreLastTransitionTime); d != "" {
				t.Error(diff.PrintWantGot(d))
			}
		})
	}
}

func TestReconcile_verifyResolved_V1Task_Error(t *testing.T) {
	resolverName := "foobar"
	unsignedTask := parse.MustParseV1Task(t, `
metadata:
  name: test-task
  namespace: foo
spec:
  params:
  - default: mydefault
    name: myarg
    type: string
  steps:
  - script: echo $(params.myarg)
    image: myimage
    name: mycontainer
`)
	unsignedTaskBytes, err := yaml.Marshal(unsignedTask)
	if err != nil {
		t.Fatal("fail to marshal task", err)
	}

	signer, _, vps := test.SetupMatchAllVerificationPolicies(t, unsignedTask.Namespace)
	signedTask, err := getSignedV1Task(unsignedTask, signer, "test-task")
	if err != nil {
		t.Fatal("fail to sign task", err)
	}

	modifiedTask := signedTask.DeepCopy()
	if modifiedTask.Annotations == nil {
		modifiedTask.Annotations = make(map[string]string)
	}
	modifiedTask.Annotations["random"] = "attack"
	modifiedTaskBytes, err := yaml.Marshal(modifiedTask)
	if err != nil {
		t.Fatal("fail to marshal task", err)
	}

	cms := []*corev1.ConfigMap{
		{
			ObjectMeta: metav1.ObjectMeta{Name: config.GetFeatureFlagsConfigName(), Namespace: system.Namespace()},
			Data: map[string]string{
				"trusted-resources-verification-no-match-policy": config.FailNoMatchPolicy,
				"enable-tekton-oci-bundles":                      "true",
			},
		},
	}
	tr := parse.MustParseV1TaskRun(t, fmt.Sprintf(`
metadata:
  name: test-taskrun
  namespace: foo
spec:
  params:
  - name: myarg
    value: foo
  taskRef:
    resolver: %s
    name: test-task
status:
  podName: the-pod
`, resolverName))
	testCases := []struct {
		name      string
		taskBytes []byte
	}{
		{
			name:      "unsigned task fails verification",
			taskBytes: unsignedTaskBytes,
		},
		{
			name:      "modified task fails verification",
			taskBytes: modifiedTaskBytes,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			rr := getResolvedResolutionRequest(t, resolverName, tc.taskBytes, tr.Namespace, tr.Name)
			d := test.Data{
				TaskRuns:             []*v1.TaskRun{tr},
				ConfigMaps:           cms,
				VerificationPolicies: vps,
				ResolutionRequests:   []*resolutionv1beta1.ResolutionRequest{&rr},
			}

			testAssets, cancel := getTaskRunController(t, d)
			defer cancel()
			createServiceAccount(t, testAssets, tr.Spec.ServiceAccountName, tr.Namespace)
			err := testAssets.Controller.Reconciler.Reconcile(testAssets.Ctx, getRunName(tr))

			if !errors.Is(err, trustedresources.ErrResourceVerificationFailed) {
				t.Errorf("Reconcile got %v but want %v", err, trustedresources.ErrResourceVerificationFailed)
			}
			reconciledRun, err := testAssets.Clients.Pipeline.TektonV1().TaskRuns(tr.Namespace).Get(testAssets.Ctx, tr.Name, metav1.GetOptions{})
			if err != nil {
				t.Fatalf("getting updated taskrun: %v", err)
			}
			condition := reconciledRun.Status.GetCondition(apis.ConditionSucceeded)
			if condition.Type != apis.ConditionSucceeded || condition.Status != corev1.ConditionFalse || condition.Reason != podconvert.ReasonResourceVerificationFailed {
				t.Errorf("Expected TaskRun to fail with reason \"%s\" but it did not. Final conditions were:\n%#v", podconvert.ReasonResourceVerificationFailed, tr.Status.Conditions)
			}
			gotVerificationCondition := reconciledRun.Status.GetCondition(trustedresources.ConditionTrustedResourcesVerified)
			if gotVerificationCondition == nil || gotVerificationCondition.Status != corev1.ConditionFalse {
				t.Errorf("Expected to have false condition, but had %v", gotVerificationCondition)
			}
		})
	}
}

// getResolvedResolutionRequest is a helper function to return the ResolutionRequest and the data is filled with resourceBytes,
// the ResolutionRequest's name is generated by resolverName, namespace and runName.
func getResolvedResolutionRequest(t *testing.T, resolverName string, resourceBytes []byte, namespace string, runName string) resolutionv1beta1.ResolutionRequest {
	t.Helper()
	name, err := remoteresource.GenerateDeterministicName(resolverName, namespace+"/"+runName, nil)
	if err != nil {
		t.Errorf("error generating name for %s/%s/%s: %v", resolverName, namespace, runName, err)
	}
	rr := resolutionv1beta1.ResolutionRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:              name,
			Namespace:         namespace,
			CreationTimestamp: metav1.Time{Time: time.Now()},
		},
	}
	rr.Status.ResolutionRequestStatusFields.Data = base64.StdEncoding.Strict().EncodeToString(resourceBytes)
	rr.Status.MarkSucceeded()
	return rr
}

func getSignedV1Task(unsigned *v1.Task, signer signature.Signer, name string) (*v1.Task, error) {
	signed := unsigned.DeepCopy()
	signed.Name = name
	if signed.Annotations == nil {
		signed.Annotations = map[string]string{}
	}
	signature, err := signInterface(signer, signed)
	if err != nil {
		return nil, err
	}
	signed.Annotations[trustedresources.SignatureAnnotation] = base64.StdEncoding.EncodeToString(signature)
	return signed, nil
}

func signInterface(signer signature.Signer, i interface{}) ([]byte, error) {
	if signer == nil {
		return nil, fmt.Errorf("signer is nil")
	}
	b, err := json.Marshal(i)
	if err != nil {
		return nil, err
	}
	h := sha256.New()
	h.Write(b)

	sig, err := signer.SignMessage(bytes.NewReader(h.Sum(nil)))
	if err != nil {
		return nil, err
	}

	return sig, nil
}
