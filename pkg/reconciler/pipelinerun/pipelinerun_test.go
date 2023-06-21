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

package pipelinerun

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"testing"
	"time"

	"sigs.k8s.io/yaml"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/sigstore/sigstore/pkg/signature"
	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	pipelinev1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	resolutionv1beta1 "github.com/tektoncd/pipeline/pkg/apis/resolution/v1beta1"
	resolutionutil "github.com/tektoncd/pipeline/pkg/internal/resolution"
	"github.com/tektoncd/pipeline/pkg/reconciler/events/cloudevent"
	"github.com/tektoncd/pipeline/pkg/reconciler/events/k8sevent"
	"github.com/tektoncd/pipeline/pkg/reconciler/pipelinerun/resources"
	ttesting "github.com/tektoncd/pipeline/pkg/reconciler/testing"
	"github.com/tektoncd/pipeline/pkg/reconciler/volumeclaim"
	resolutioncommon "github.com/tektoncd/pipeline/pkg/resolution/common"
	remoteresource "github.com/tektoncd/pipeline/pkg/resolution/resource"
	"github.com/tektoncd/pipeline/pkg/trustedresources"
	"github.com/tektoncd/pipeline/pkg/trustedresources/verifier"
	"github.com/tektoncd/pipeline/test"
	"github.com/tektoncd/pipeline/test/diff"
	"github.com/tektoncd/pipeline/test/names"
	"github.com/tektoncd/pipeline/test/parse"
	"go.opentelemetry.io/otel/trace"
	"gomodules.xyz/jsonpatch/v2"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	fakek8s "k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/record"
	clock "k8s.io/utils/clock/testing"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	cminformer "knative.dev/pkg/configmap/informer"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/kmeta"
	"knative.dev/pkg/logging"
	logtesting "knative.dev/pkg/logging/testing"
	"knative.dev/pkg/reconciler"
	"knative.dev/pkg/system"
	_ "knative.dev/pkg/system/testing" // Setup system.Namespace()
)

var (
	images = pipeline.Images{
		EntrypointImage: "override-with-entrypoint:latest",
		NopImage:        "override-with-nop:latest",
		ShellImage:      "busybox",
	}

	ignoreResourceVersion    = cmpopts.IgnoreFields(metav1.ObjectMeta{}, "ResourceVersion")
	ignoreTypeMeta           = cmpopts.IgnoreFields(metav1.TypeMeta{}, "Kind", "APIVersion")
	ignoreLastTransitionTime = cmpopts.IgnoreFields(apis.Condition{}, "LastTransitionTime.Inner.Time")
	ignoreStartTime          = cmpopts.IgnoreFields(v1.PipelineRunStatusFields{}, "StartTime")
	ignoreCompletionTime     = cmpopts.IgnoreFields(v1.PipelineRunStatusFields{}, "CompletionTime")
	ignoreFinallyStartTime   = cmpopts.IgnoreFields(v1.PipelineRunStatusFields{}, "FinallyStartTime")
	ignoreProvenance         = cmpopts.IgnoreFields(v1.PipelineRunStatusFields{}, "Provenance")
	trueb                    = true
	simpleHelloWorldTask     = &v1.Task{ObjectMeta: baseObjectMeta("hello-world", "foo")}
	simpleSomeTask           = &v1.Task{ObjectMeta: baseObjectMeta("some-task", "foo")}
	simpleHelloWorldPipeline = &v1.Pipeline{
		ObjectMeta: baseObjectMeta("test-pipeline", "foo"),
		Spec: v1.PipelineSpec{
			Tasks: []v1.PipelineTask{{
				Name: "hello-world-1",
				TaskRef: &v1.TaskRef{
					Name: "hello-world",
				},
			}},
		},
	}

	now       = time.Date(2022, time.January, 1, 0, 0, 0, 0, time.UTC)
	testClock = clock.NewFakePassiveClock(now)
)

const (
	apiFieldsFeatureFlag           = "enable-api-fields"
	ociBundlesFeatureFlag          = "enable-tekton-oci-bundles"
	maxMatrixCombinationsCountFlag = "default-max-matrix-combinations-count"
	disableAffinityAssistantFlag   = "disable-affinity-assistant"
)

type PipelineRunTest struct {
	test.Data  `json:"inline"`
	Test       *testing.T
	TestAssets test.Assets
	Cancel     func()
}

// getPipelineRunController returns an instance of the PipelineRun controller/reconciler that has been seeded with
// d, where d represents the state of the system (existing resources) needed for the test.
func getPipelineRunController(t *testing.T, d test.Data) (test.Assets, func()) {
	t.Helper()
	return initializePipelineRunControllerAssets(t, d, pipeline.Options{Images: images})
}

// initiailizePipelinerunControllerAssets is a shared helper for
// controller initialization.
func initializePipelineRunControllerAssets(t *testing.T, d test.Data, opts pipeline.Options) (test.Assets, func()) {
	t.Helper()
	ctx, _ := ttesting.SetupFakeContext(t)
	ctx = ttesting.SetupFakeCloudClientContext(ctx, d.ExpectedCloudEventCount)
	ctx, cancel := context.WithCancel(ctx)
	test.EnsureConfigurationConfigMapsExist(&d)
	c, informers := test.SeedTestData(t, ctx, d)
	configMapWatcher := cminformer.NewInformedWatcher(c.Kube, system.Namespace())
	ctl := NewController(&opts, testClock, trace.NewNoopTracerProvider())(ctx, configMapWatcher)
	if la, ok := ctl.Reconciler.(reconciler.LeaderAware); ok {
		if err := la.Promote(reconciler.UniversalBucket(), func(reconciler.Bucket, types.NamespacedName) {}); err != nil {
			t.Fatalf("error promoting reconciler leader: %v", err)
		}
	}
	if err := configMapWatcher.Start(ctx.Done()); err != nil {
		t.Fatalf("error starting configmap watcher: %v", err)
	}
	return test.Assets{
		Logger:     logging.FromContext(ctx),
		Clients:    c,
		Controller: ctl,
		Informers:  informers,
		Recorder:   controller.GetEventRecorder(ctx).(*record.FakeRecorder),
		Ctx:        ctx,
	}, cancel
}

// validateTaskRunsCount ensure that there are `expectedCount` TaskRuns
// It will fatal the test if the number of TaskRuns is not `expectedCount`
func validateTaskRunsCount(t *testing.T, taskRuns map[string]*v1.TaskRun, expectedCount int) {
	t.Helper()

	actualCount := len(taskRuns)
	if actualCount != expectedCount {
		t.Fatalf("Expected %d taskruns but it has %d", expectedCount, actualCount)
	}
}

// getTaskRunByName retrieves the TaskRun with the specified name from the given TaskRuns
// It will fatal the test if the name does not exist
func getTaskRunByName(t *testing.T, taskRuns map[string]*v1.TaskRun, expectedName string) *v1.TaskRun {
	t.Helper()

	tr, exist := taskRuns[expectedName]
	if !exist {
		t.Fatalf("Expected taskrun %s does not exist", expectedName)
	}

	return tr
}

// getTaskRunsForPipelineRun returns the set of TaskRuns associated with the input PipelineRun.
// It will fatal the test if an error occurred.
func getTaskRunsForPipelineRun(ctx context.Context, t *testing.T, clients test.Clients, namespace string, prName string) map[string]*v1.TaskRun {
	t.Helper()
	labelSelector := pipeline.PipelineRunLabelKey + "=" + prName
	return getTaskRuns(ctx, t, clients, namespace, labelSelector)
}

// getTaskRunsForPipelineTask returns the set of TaskRuns associated with the input PipelineRun and PipelineTask
// It will fatal the test if an error occurred.
func getTaskRunsForPipelineTask(ctx context.Context, t *testing.T, clients test.Clients, namespace string, prName string, ptLabel string) map[string]*v1.TaskRun {
	t.Helper()
	labelSelector := pipeline.PipelineRunLabelKey + "=" + prName + "," + pipeline.PipelineTaskLabelKey + "=" + ptLabel
	return getTaskRuns(ctx, t, clients, namespace, labelSelector)
}

// getTaskRuns returns the set of TaskRuns matching the label selector.
// It will fatal the test if an error occurred.
func getTaskRuns(ctx context.Context, t *testing.T, clients test.Clients, namespace string, labelSelector string) map[string]*v1.TaskRun {
	t.Helper()

	opt := metav1.ListOptions{
		LabelSelector: labelSelector,
	}

	taskRuns, err := clients.Pipeline.TektonV1().TaskRuns(namespace).List(ctx, opt)
	if err != nil {
		t.Fatalf("failed to list taskruns, %s", err)
	}

	outputs := make(map[string]*v1.TaskRun)
	for _, item := range taskRuns.Items {
		tr := item
		outputs[tr.Name] = &tr
	}

	return outputs
}

// runTestReconcile runs "Reconcile" on a PipelineRun with one
// Task that has not been started yet.  It verifies that the TaskRun is created,
// it checks the resulting API actions, status and events.
func TestReconcile(t *testing.T) {
	namespace := "foo"
	prName := "test-pipeline-run-success"
	trName := "test-pipeline-run-success-unit-test-1"

	prs := []*v1.PipelineRun{parse.MustParseV1PipelineRun(t, `
metadata:
  name: test-pipeline-run-success
  namespace: foo
spec:
  params:
  - name: bar
    value: somethingmorefun
  pipelineRef:
    name: test-pipeline
  taskRunTemplate:
    serviceAccountName: test-sa
`)}
	ps := []*v1.Pipeline{parse.MustParseV1Pipeline(t, `
metadata:
  name: test-pipeline
  namespace: foo
spec:
  params:
  - default: somethingdifferent
    name: pipeline-param
    type: string
  - default: revision
    name: rev-param
    type: string
  - name: bar
    type: string
  tasks:
  - name: unit-test-2
    params:
    - name: foo
      value: somethingfun
    - name: bar
      value: $(params.bar)
    - name: contextRunParam
      value: $(context.pipelineRun.name)
    - name: contextPipelineParam
      value: $(context.pipeline.name)
    - name: contextRetriesParam
      value: $(context.pipelineTask.retries)
    runAfter:
    - unit-test-1
    taskRef:
      name: unit-test-task
  - name: unit-test-1
    params:
    - name: foo
      value: somethingfun
    - name: bar
      value: $(params.bar)
    - name: contextRunParam
      value: $(context.pipelineRun.name)
    - name: contextPipelineParam
      value: $(context.pipeline.name)
    - name: contextRetriesParam
      value: $(context.pipelineTask.retries)
    retries: 5
    taskRef:
      name: unit-test-task
  - name: unit-test-cluster-task
    params:
    - name: foo
      value: somethingfun
    - name: bar
      value: $(params.bar)
    - name: contextRunParam
      value: $(context.pipelineRun.name)
    - name: contextPipelineParam
      value: $(context.pipeline.name)
    taskRef:
      kind: ClusterTask
      name: unit-test-cluster-task
`)}
	ts := []*v1.Task{
		parse.MustParseV1Task(t, `
metadata:
  name: unit-test-task
  namespace: foo
spec:
  params:
  - name: foo
    type: string
  - name: bar
    type: string
  - name: contextRunParam
    type: string
  - name: contextPipelineParam
    type: string
  - name: contextRetriesParam
    type: string
`)}
	clusterTasks := []*v1beta1.ClusterTask{
		parse.MustParseClusterTask(t, `
metadata:
  name: unit-test-cluster-task
spec:
  params:
  - name: foo
    type: string
  - name: bar
    type: string
  - name: contextRunParam
    type: string
  - name: contextPipelineParam
    type: string
`)}

	d := test.Data{
		PipelineRuns: prs,
		Pipelines:    ps,
		Tasks:        ts,
		ClusterTasks: clusterTasks,
		ConfigMaps:   []*corev1.ConfigMap{newFeatureFlagsConfigMap()},
	}
	prt := newPipelineRunTest(t, d)
	defer prt.Cancel()

	wantEvents := []string{
		"Normal Started",
		"Normal Running Tasks Completed: 0",
	}
	reconciledRun, clients := prt.reconcileRun(namespace, prName, wantEvents, false)

	taskRuns := getTaskRunsForPipelineRun(prt.TestAssets.Ctx, t, clients, namespace, prName)
	// Ensure that there are 2 TaskRuns associated with this PipelineRun
	validateTaskRunsCount(t, taskRuns, 2)

	// Check that the expected TaskRun was created
	actual := getTaskRunByName(t, taskRuns, trName)
	expectedTaskRun := mustParseTaskRunWithObjectMeta(t,
		taskRunObjectMeta(trName, namespace, prName,
			"test-pipeline", "unit-test-1", false),
		`
spec:
  params:
  - name: foo
    value: somethingfun
  - name: bar
    value: somethingmorefun
  - name: contextRunParam
    value: test-pipeline-run-success
  - name: contextPipelineParam
    value: test-pipeline
  - name: contextRetriesParam
    value: "5"
  retries: 5
  serviceAccountName: test-sa
  taskRef:
    name: unit-test-task
    kind: Task
`)
	// ignore IgnoreUnexported ignore both after and before steps fields
	if d := cmp.Diff(expectedTaskRun, actual, ignoreTypeMeta, ignoreResourceVersion); d != "" {
		t.Errorf("expected to see TaskRun %v created. Diff %s", expectedTaskRun, diff.PrintWantGot(d))
	}
	// test taskrun is able to recreate correct pipeline-pvc-name
	if expectedTaskRun.GetPipelineRunPVCName() != "test-pipeline-run-success-pvc" {
		t.Errorf("expected to see TaskRun PVC name set to %q created but got %s", "test-pipeline-run-success-pvc", expectedTaskRun.GetPipelineRunPVCName())
	}

	// This PipelineRun is in progress now and the status should reflect that
	checkPipelineRunConditionStatusAndReason(t, reconciledRun, corev1.ConditionUnknown, v1.PipelineRunReasonRunning.String())

	tr1Name := "test-pipeline-run-success-unit-test-1"
	tr2Name := "test-pipeline-run-success-unit-test-cluster-task"

	verifyTaskRunStatusesCount(t, reconciledRun.Status, 2)
	verifyTaskRunStatusesNames(t, reconciledRun.Status, tr1Name, tr2Name)
}

// TestReconcile_V1Beta1CustomTask runs "Reconcile" on a PipelineRun with one Custom
// Task reference that has not been run yet
// It verifies that the CustomRun is created, it checks the resulting API actions, status and events.
func TestReconcile_V1Beta1CustomTask(t *testing.T) {
	names.TestingSeed()
	const pipelineRunName = "test-pipelinerun"
	const namespace = "namespace"

	simpleCustomTaskPRYAML := `metadata:
  name: test-pipelinerun
  namespace: namespace
spec:
  pipelineSpec:
    tasks:
    - name: custom-task
      params:
      - name: param1
        value: value1
      retries: 3
      taskRef:
        apiVersion: example.dev/v0
        kind: Example
`
	simpleCustomTaskWantRunYAML := `metadata:
  annotations: {}
  labels:
    tekton.dev/memberOf: tasks
    tekton.dev/pipeline: test-pipelinerun
    tekton.dev/pipelineRun: test-pipelinerun
    tekton.dev/pipelineTask: custom-task
  name: test-pipelinerun-custom-task
  namespace: namespace
  ownerReferences:
  - apiVersion: tekton.dev/v1
    blockOwnerDeletion: true
    controller: true
    kind: PipelineRun
    name: test-pipelinerun
spec:
  params:
  - name: param1
    value: value1
  customRef:
    apiVersion: example.dev/v0
    kind: Example
  retries: 3
  serviceAccountName: default
`

	tcs := []struct {
		name    string
		pr      *v1.PipelineRun
		wantRun *v1beta1.CustomRun
	}{{
		name:    "simple custom task with taskRef",
		pr:      parse.MustParseV1PipelineRun(t, simpleCustomTaskPRYAML),
		wantRun: parse.MustParseCustomRun(t, simpleCustomTaskWantRunYAML),
	}, {
		name: "simple custom task with taskSpec",
		pr: parse.MustParseV1PipelineRun(t, `
metadata:
  name: test-pipelinerun
  namespace: namespace
spec:
  pipelineSpec:
    tasks:
    - name: custom-task
      params:
      - name: param1
        value: value1
      - name: contextPipelineParam
        value: $(context.pipeline.name)
      taskSpec:
        apiVersion: example.dev/v0
        kind: Example
        metadata:
          labels:
            test-label: test
        spec:
          field1: 123
          field2: value
`),
		wantRun: mustParseCustomRunWithObjectMeta(t,
			taskRunObjectMeta("test-pipelinerun-custom-task", "namespace", "test-pipelinerun", "test-pipelinerun", "custom-task", false),
			`
spec:
  params:
  - name: param1
    value: value1
  - name: contextPipelineParam
    value: test-pipelinerun
  serviceAccountName: default
  customSpec:
    apiVersion: example.dev/v0
    kind: Example
    metadata:
      labels:
        test-label: test
    spec:
      field1: 123
      field2: value
`),
	}, {
		name: "custom task with workspace",
		pr: parse.MustParseV1PipelineRun(t, `
metadata:
  name: test-pipelinerun
  namespace: namespace
spec:
  pipelineSpec:
    tasks:
    - name: custom-task
      taskRef:
        apiVersion: example.dev/v0
        kind: Example
      workspaces:
      - name: taskws
        subPath: bar
        workspace: pipelinews
    workspaces:
    - name: pipelinews
  workspaces:
  - name: pipelinews
    persistentVolumeClaim:
      claimName: myclaim
    subPath: foo
`),
		wantRun: mustParseCustomRunWithObjectMeta(t,
			taskRunObjectMetaWithAnnotations("test-pipelinerun-custom-task", "namespace", "test-pipelinerun",
				"test-pipelinerun", "custom-task", false, map[string]string{
					"pipeline.tekton.dev/affinity-assistant": getAffinityAssistantName("pipelinews", pipelineRunName),
				}),
			`
spec:
  customRef:
    apiVersion: example.dev/v0
    kind: Example
  serviceAccountName: default
  workspaces:
  - name: taskws
    persistentVolumeClaim:
      claimName: myclaim
    subPath: foo/bar
`),
	}}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			d := test.Data{
				PipelineRuns: []*v1.PipelineRun{tc.pr},
			}
			prt := newPipelineRunTest(t, d)
			defer prt.Cancel()

			wantEvents := []string{
				"Normal Started",
				"Normal Running Tasks Completed: 0",
			}
			reconciledRun, clients := prt.reconcileRun(namespace, pipelineRunName, wantEvents, false)

			actions := clients.Pipeline.Actions()
			if len(actions) < 2 {
				t.Fatalf("Expected client to have at least two action implementation but it has %d", len(actions))
			}

			// Check that the expected CustomRun was created.
			actual := actions[0].(ktesting.CreateAction).GetObject()
			// Ignore the TypeMeta field, because parse.MustParseCustomRun automatically populates it but the "actual" CustomRun won't have it.
			if d := cmp.Diff(tc.wantRun, actual, cmpopts.IgnoreFields(v1beta1.CustomRun{}, "TypeMeta"), cmpopts.EquateEmpty()); d != "" {
				t.Errorf("expected to see CustomRun created: %s", diff.PrintWantGot(d))
			}

			// This PipelineRun is in progress now and the status should reflect that
			checkPipelineRunConditionStatusAndReason(t, reconciledRun, corev1.ConditionUnknown, v1.PipelineRunReasonRunning.String())

			verifyCustomRunOrRunStatusesCount(t, customRun, reconciledRun.Status, 1)
			verifyCustomRunOrRunStatusesNames(t, customRun, reconciledRun.Status, tc.wantRun.Name)
		})
	}
}

func TestReconcile_PipelineSpecTaskSpec(t *testing.T) {
	// TestReconcile_PipelineSpecTaskSpec runs "Reconcile" on a PipelineRun that has an embedded PipelineSpec that has an embedded TaskSpec.
	// It verifies that a TaskRun is created, it checks the resulting API actions, status and events.
	names.TestingSeed()

	namespace := "foo"
	prName := "test-pipeline-run-success"
	trName := "test-pipeline-run-success-unit-test-task-spec"

	prs := []*v1.PipelineRun{
		parse.MustParseV1PipelineRun(t, `
metadata:
  name: test-pipeline-run-success
  namespace: foo
spec:
  pipelineRef:
    name: test-pipeline
`),
	}
	ps := []*v1.Pipeline{
		parse.MustParseV1Pipeline(t, `
metadata:
  name: test-pipeline
  namespace: foo
spec:
  tasks:
    - name: unit-test-task-spec
      taskSpec:
        steps:
          - name: mystep
            image: myimage
`),
	}

	d := test.Data{
		PipelineRuns: prs,
		Pipelines:    ps,
		ConfigMaps:   []*corev1.ConfigMap{newFeatureFlagsConfigMap()},
	}
	prt := newPipelineRunTest(t, d)
	defer prt.Cancel()

	wantEvents := []string{
		"Normal Started",
		"Normal Running Tasks Completed: 0",
	}
	reconciledRun, clients := prt.reconcileRun(namespace, prName, wantEvents, false)

	// Check that the expected TaskRun was created
	taskRuns := getTaskRunsForPipelineRun(prt.TestAssets.Ctx, t, clients, namespace, prName)
	validateTaskRunsCount(t, taskRuns, 1)

	actual := getTaskRunByName(t, taskRuns, trName)
	expectedTaskRun := parse.MustParseV1TaskRun(t, fmt.Sprintf(`
spec:
  taskSpec:
    steps:
      - name: mystep
        image: myimage
  serviceAccountName: %s
`, config.DefaultServiceAccountValue))

	expectedTaskRun.ObjectMeta = taskRunObjectMeta(trName, "foo", "test-pipeline-run-success", "test-pipeline", "unit-test-task-spec", false)

	// ignore IgnoreUnexported ignore both after and before steps fields
	if d := cmp.Diff(expectedTaskRun, actual, ignoreTypeMeta, ignoreResourceVersion, cmpopts.SortSlices(func(x, y v1.TaskSpec) bool { return len(x.Steps) == len(y.Steps) })); d != "" {
		t.Errorf("expected to see TaskRun %v created. Diff %s", expectedTaskRun, diff.PrintWantGot(d))
	}

	// test taskrun is able to recreate correct pipeline-pvc-name
	if expectedTaskRun.GetPipelineRunPVCName() != "test-pipeline-run-success-pvc" {
		t.Errorf("expected to see TaskRun PVC name set to %q created but got %s", "test-pipeline-run-success-pvc", expectedTaskRun.GetPipelineRunPVCName())
	}

	verifyTaskRunStatusesCount(t, reconciledRun.Status, 1)
	verifyTaskRunStatusesNames(t, reconciledRun.Status, trName)
}

// TestReconcile_InvalidPipelineRuns runs "Reconcile" on several PipelineRuns that are invalid in different ways.
// It verifies that reconcile fails, how it fails and which events are triggered.
func TestReconcile_InvalidPipelineRuns(t *testing.T) {
	ts := []*v1.Task{
		parse.MustParseV1Task(t, `
metadata:
  name: a-task-that-exists
  namespace: foo
`),
		parse.MustParseV1Task(t, `
metadata:
  name: a-task-that-needs-params
  namespace: foo
spec:
  params:
    - name: some-param
`),
		parse.MustParseV1Task(t, fmt.Sprintf(`
metadata:
  name: a-task-that-needs-array-params
  namespace: foo
spec:
  params:
    - name: some-param
      type: %s
`, v1.ParamTypeArray)),
		parse.MustParseV1Task(t, fmt.Sprintf(`
metadata:
  name: a-task-that-needs-object-params
  namespace: foo
spec:
  params:
    - name: some-param
      type: %s
      properties:
        key1: {}
        key2: {}
`, v1.ParamTypeObject)),
	}

	ps := []*v1.Pipeline{parse.MustParseV1Pipeline(t, `
metadata:
  name: pipeline-missing-tasks
  namespace: foo
spec:
  tasks:
    - name: myspecialtask
      taskRef:
        name: sometask
`),
		parse.MustParseV1Pipeline(t, `
metadata:
  name: a-pipeline-without-params
  namespace: foo
spec:
  tasks:
    - name: some-task
      taskRef:
        name: a-task-that-needs-params
`),
		parse.MustParseV1Pipeline(t, `
metadata:
  name: a-pipeline-that-should-be-caught-by-admission-control
  namespace: foo
spec:
  tasks:
    - name: some-task
      taskRef:
        name: a-task-that-exists
`),
		parse.MustParseV1Pipeline(t, fmt.Sprintf(`
metadata:
  name: a-pipeline-with-array-params
  namespace: foo
spec:
  params:
    - name: some-param
      type: %s
  tasks:
    - name: some-task
      taskRef:
        name: a-task-that-needs-array-params
`, v1.ParamTypeArray)),
		parse.MustParseV1Pipeline(t, fmt.Sprintf(`
metadata:
  name: a-pipeline-with-array-indexing-params
  namespace: foo
spec:
  params:
    - name: some-param
      type: %s
  tasks:
    - name: some-task
      taskRef:
        name: a-task-that-needs-array-params
      params:
        - name: param
          value: "$(params.some-param[2])"
`, v1.ParamTypeArray)),
		parse.MustParseV1Pipeline(t, fmt.Sprintf(`
metadata:
  name: a-pipeline-with-object-params
  namespace: foo
spec:
  params:
    - name: some-param
      type: %s
      properties:
        key1: {type: string}
        key2: {type: string}
  tasks:
    - name: some-task
      taskRef:
        name: a-task-that-needs-object-params
`, v1.ParamTypeObject)),
	}

	for _, tc := range []struct {
		name               string
		pipelineRun        *v1.PipelineRun
		reason             string
		hasNoDefaultLabels bool
		permanentError     bool
		wantEvents         []string
	}{{
		name: "invalid-pipeline-shd-be-stop-reconciling",
		pipelineRun: parse.MustParseV1PipelineRun(t, `
metadata:
  name: invalid-pipeline
  namespace: foo
spec:
  pipelineRef:
    name: pipeline-not-exist
`),
		reason:             ReasonCouldntGetPipeline,
		hasNoDefaultLabels: true,
		permanentError:     true,
		wantEvents: []string{
			"Normal Started",
			"Warning Failed Error retrieving pipeline for pipelinerun",
		},
	}, {
		name: "invalid-pipeline-run-missing-tasks-shd-stop-reconciling",
		pipelineRun: parse.MustParseV1PipelineRun(t, `
metadata:
  name: pipelinerun-missing-tasks
  namespace: foo
spec:
  pipelineRef:
    name: pipeline-missing-tasks
`),
		reason:         ReasonCouldntGetTask,
		permanentError: true,
		wantEvents: []string{
			"Normal Started",
			"Warning Failed Pipeline foo/pipeline-missing-tasks can't be Run",
		},
	}, {
		name: "invalid-pipeline-run-params-dont-exist-shd-stop-reconciling",
		pipelineRun: parse.MustParseV1PipelineRun(t, `
metadata:
  name: pipeline-params-dont-exist
  namespace: foo
spec:
  pipelineRef:
    name: a-pipeline-without-params
`),
		reason:         ReasonFailedValidation,
		permanentError: true,
		wantEvents: []string{
			"Normal Started",
			"Warning Failed invalid input params for task a-task-that-needs-params: missing values",
		},
	}, {
		name: "invalid-pipeline-mismatching-parameter-types",
		pipelineRun: parse.MustParseV1PipelineRun(t, `
metadata:
  name: pipeline-mismatching-param-type
  namespace: foo
spec:
  pipelineRef:
    name: a-pipeline-with-array-params
  params:
    - name: some-param
      value: stringval
`),
		reason:         ReasonParameterTypeMismatch,
		permanentError: true,
		wantEvents: []string{
			"Normal Started",
			"Warning Failed PipelineRun foo/pipeline-mismatching-param-type parameters have mismatching types",
		},
	}, {
		name: "invalid-pipeline-missing-object-keys",
		pipelineRun: parse.MustParseV1PipelineRun(t, `
metadata:
  name: pipeline-missing-object-param-keys
  namespace: foo
spec:
  pipelineRef:
    name: a-pipeline-with-object-params
  params:
    - name: some-param
      value:
        key1: "a"
`),
		reason:         ReasonObjectParameterMissKeys,
		permanentError: true,
		wantEvents: []string{
			"Normal Started",
			"Warning Failed PipelineRun foo/pipeline-missing-object-param-keys parameters is missing object keys required by Pipeline foo/a-pipeline-with-object-params's parameters: PipelineRun missing object keys for parameters",
		},
	}, {
		name: "invalid-pipeline-array-index-out-of-bound",
		pipelineRun: parse.MustParseV1PipelineRun(t, `
metadata:
  name: pipeline-param-array-out-of-bound
  namespace: foo
spec:
  pipelineRef:
    name: a-pipeline-with-array-indexing-params
  params:
    - name: some-param
      value:
        - "a"
        - "b"
    `),
		reason:         ReasonParamArrayIndexingInvalid,
		permanentError: true,
		wantEvents: []string{
			"Normal Started",
			"Warning Failed PipelineRun foo/pipeline-param-array-out-of-bound failed validation: failed to validate Pipeline foo/a-pipeline-with-array-indexing-params's parameter which has an invalid index while referring to an array: non-existent param references:[$(params.some-param[2]",
		},
	}, {
		name: "invalid-embedded-pipeline-bad-name-shd-stop-reconciling",
		pipelineRun: parse.MustParseV1PipelineRun(t, `
metadata:
  name: embedded-pipeline-invalid
  namespace: foo
spec:
  pipelineSpec:
    tasks:
      - name: bad-t@$k
        taskRef:
          name: b@d-t@$k
`),
		reason:         ReasonFailedValidation,
		permanentError: true,
		wantEvents: []string{
			"Normal Started",
			"Warning Failed Pipeline foo/embedded-pipeline-invalid can't be Run; it has an invalid spec",
		},
	}, {
		name: "invalid-embedded-pipeline-mismatching-parameter-types",
		pipelineRun: parse.MustParseV1PipelineRun(t, fmt.Sprintf(`
metadata:
  name: embedded-pipeline-mismatching-param-type
  namespace: foo
spec:
  pipelineSpec:
    params:
      - name: some-param
        type: %s
    tasks:
      - name: some-task
        taskRef:
          name: a-task-that-needs-array-params
  params:
    - name: some-param
      value: stringval
`, v1.ParamTypeArray)),
		reason:         ReasonParameterTypeMismatch,
		permanentError: true,
		wantEvents: []string{
			"Normal Started",
			"Warning Failed PipelineRun foo/embedded-pipeline-mismatching-param-type parameters have mismatching types",
		},
	}, {
		name: "invalid-pipeline-run-missing-params-shd-stop-reconciling",
		pipelineRun: parse.MustParseV1PipelineRun(t, fmt.Sprintf(`
metadata:
  name: pipelinerun-missing-params
  namespace: foo
spec:
  pipelineSpec:
    params:
      - name: some-param
        type: %s
    tasks:
      - name: some-task
        taskRef:
          name: a-task-that-needs-params
`, v1.ParamTypeString)),
		reason:         ReasonParameterMissing,
		permanentError: true,
		wantEvents: []string{
			"Normal Started",
			"Warning Failed PipelineRun foo parameters is missing some parameters required by Pipeline pipelinerun-missing-params",
		},
	}, {
		name: "invalid-pipeline-with-invalid-dag-graph",
		pipelineRun: parse.MustParseV1PipelineRun(t, `
metadata:
  name: pipeline-invalid-dag-graph
  namespace: foo
spec:
  pipelineSpec:
    tasks:
      - name: dag-task-1
        taskRef:
          name: dag-task-1
        runAfter: [ dag-task-1 ]
`),
		reason:         ReasonInvalidGraph,
		permanentError: true,
		wantEvents: []string{
			"Normal Started",
			"Warning Failed PipelineRun foo/pipeline-invalid-dag-graph's Pipeline DAG is invalid",
		},
	}, {
		name: "invalid-pipeline-with-invalid-final-tasks-graph",
		pipelineRun: parse.MustParseV1PipelineRun(t, `
metadata:
  name: pipeline-invalid-final-graph
  namespace: foo
spec:
  pipelineSpec:
    tasks:
      - name: dag-task-1
        taskRef:
          name: taskName
    finally:
      - name: final-task-1
        taskRef:
          name: taskName
      - name: final-task-1
        taskRef:
          name: taskName
`),
		reason:         ReasonInvalidGraph,
		permanentError: true,
		wantEvents: []string{
			"Normal Started",
			"Warning Failed PipelineRun foo's Pipeline DAG is invalid for finally clause",
		},
	}} {
		t.Run(tc.name, func(t *testing.T) {
			cms := []*corev1.ConfigMap{withEnabledAlphaAPIFields(newFeatureFlagsConfigMap())}

			d := test.Data{
				PipelineRuns: []*v1.PipelineRun{tc.pipelineRun},
				Pipelines:    ps,
				Tasks:        ts,
				ConfigMaps:   cms,
			}
			prt := newPipelineRunTest(t, d)
			defer prt.Cancel()

			wantEvents := append(tc.wantEvents, "Warning InternalError 1 error occurred") //nolint:gocritic
			reconciledRun, _ := prt.reconcileRun("foo", tc.pipelineRun.Name, wantEvents, tc.permanentError)

			if reconciledRun.Status.CompletionTime == nil {
				t.Errorf("Expected a CompletionTime on invalid PipelineRun but was nil")
			}

			// Since the PipelineRun is invalid, the status should say it has failed
			checkPipelineRunConditionStatusAndReason(t, reconciledRun, corev1.ConditionFalse, tc.reason)
			if !tc.hasNoDefaultLabels {
				expectedPipelineLabel := reconciledRun.Name
				// Embedded pipelines use the pipelinerun name
				if reconciledRun.Spec.PipelineRef != nil {
					expectedPipelineLabel = reconciledRun.Spec.PipelineRef.Name
				}
				expectedLabels := map[string]string{pipeline.PipelineLabelKey: expectedPipelineLabel}
				if len(reconciledRun.ObjectMeta.Labels) != len(expectedLabels) {
					t.Errorf("Expected labels : %v, got %v", expectedLabels, reconciledRun.ObjectMeta.Labels)
				}
				for k, ev := range expectedLabels {
					if v, ok := reconciledRun.ObjectMeta.Labels[k]; ok {
						if ev != v {
							t.Errorf("Expected labels %s=%s, but was %s", k, ev, v)
						}
					} else {
						t.Errorf("Expected labels %s=%v, but was not present", k, ev)
					}
				}
			}
		})
	}
}

func TestMissingResultWhenStepErrorIsIgnored(t *testing.T) {
	prs := []*v1.PipelineRun{parse.MustParseV1PipelineRun(t, `
metadata:
  name: test-pipeline-missing-results
  namespace: foo
spec:
  serviceAccountName: test-sa-0
  pipelineSpec:
    tasks:
    - name: task1
      taskSpec:
        results:
        - name: result1
          type: string
        - name: result2
          type: string
        steps:
        - name: failing-step
          onError: continue
          image: busybox
          script: 'echo -n 123 | tee $(results.result1.path); exit 1; echo -n 456 | tee $(results.result2.path)'
    - name: task2
      runAfter: [ task1 ]
      params:
      - name: param1
        value: $(tasks.task1.results.result1)
      - name: param2
        value: $(tasks.task1.results.result2)
      taskSpec:
        params:
        - name: param1
          type: string
        - name: param2
          type: string
        steps:
        - name: foo
          image: busybox
          script: 'exit 0'
`)}

	trs := []*v1.TaskRun{mustParseTaskRunWithObjectMeta(t,
		taskRunObjectMeta("test-pipeline-missing-results-task1", "foo",
			"test-pipeline-missing-results", "test-pipeline", "task1", true),
		`
spec:
  serviceAccountName: test-sa
  timeout: 1h0m0s
status:
  conditions:
  - status: "True"
    type: Succeeded
  results:
  - name: result1
    value: 123
`)}

	expectedPipelineRun :=
		parse.MustParseV1PipelineRun(t, `
metadata:
  name: test-pipeline-missing-results
  namespace: foo
  annotations: {}
  labels:
    tekton.dev/pipeline: test-pipeline-missing-results
spec:
  serviceAccountName: test-sa-0
  pipelineSpec:
    tasks:
    - name: task1
      taskSpec:
        results:
        - name: result1
          type: string
        - name: result2
          type: string
        steps:
        - name: failing-step
          onError: continue
          image: busybox
          script: 'echo -n 123 | tee $(results.result1.path); exit 1; echo -n 456 | tee $(results.result2.path)'
    - name: task2
      runAfter: [ task1 ]
      params:
      - name: param1
        value: $(tasks.task1.results.result1)
      - name: param2
        value: $(tasks.task1.results.result2)
      taskSpec:
        params:
        - name: param1
          type: string
        - name: param2
          type: string
        steps:
        - name: foo
          image: busybox
          script: 'exit 0'
status:
  pipelineSpec:
    tasks:
    - name: task1
      taskSpec:
        results:
        - name: result1
          type: string
        - name: result2
          type: string
        steps:
        - name: failing-step
          onError: continue
          image: busybox
          script: 'echo -n 123 | tee $(results.result1.path); exit 1; echo -n 456 | tee $(results.result2.path)'
    - name: task2
      runAfter: [ task1 ]
      params:
      - name: param1
        value: $(tasks.task1.results.result1)
      - name: param2
        value: $(tasks.task1.results.result2)
      taskSpec:
        params:
        - name: param1
          type: string
        - name: param2
          type: string
        steps:
        - name: foo
          image: busybox
          script: 'exit 0'
  conditions:
  - message: "Invalid task result reference: Could not find result with name result2 for task task1"
    reason: InvalidTaskResultReference
    status: "False"
    type: Succeeded
  childReferences:
  - apiVersion: tekton.dev/v1
    kind: TaskRun
    name: test-pipeline-missing-results-task1
    pipelineTaskName: task1
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
	d := test.Data{
		PipelineRuns: prs,
		TaskRuns:     trs,
	}
	prt := newPipelineRunTest(t, d)
	defer prt.Cancel()

	reconciledRun, clients := prt.reconcileRun("foo", "test-pipeline-missing-results", []string{}, true)
	if reconciledRun.Status.CompletionTime == nil {
		t.Errorf("Expected a CompletionTime on invalid PipelineRun but was nil")
	}

	// The PipelineRun should be marked as failed due to InvalidTaskResultReference.
	if d := cmp.Diff(reconciledRun, expectedPipelineRun, ignoreResourceVersion, ignoreLastTransitionTime, ignoreTypeMeta, ignoreStartTime, ignoreCompletionTime); d != "" {
		t.Errorf("Expected to see PipelineRun run marked as failed with the reason: InvalidTaskResultReference. Diff %s", diff.PrintWantGot(d))
	}

	// Check that the expected TaskRun was created
	taskRuns := getTaskRunsForPipelineRun(prt.TestAssets.Ctx, t, clients, "foo", "test-pipeline-missing-results")

	// We expect only 1 TaskRun to be created, since the PipelineRun should fail before creating the 2nd TaskRun due to the InvalidTaskResultReference
	validateTaskRunsCount(t, taskRuns, 1)
}

func TestReconcile_InvalidPipelineRunNames(t *testing.T) {
	// TestReconcile_InvalidPipelineRunNames runs "Reconcile" on several PipelineRuns that have invalid names.
	// It verifies that reconcile fails, how it fails and which events are triggered.
	// Note that the code tested here is part of the genreconciler.
	invalidNames := []string{
		"foo/test-pipeline-run-doesnot-exist",
		"test/invalidformat/t",
	}
	tcs := []struct {
		name        string
		pipelineRun string
	}{
		{
			name:        "invalid-pipeline-run-shd-stop-reconciling",
			pipelineRun: invalidNames[0],
		}, {
			name:        "invalid-pipeline-run-name-shd-stop-reconciling",
			pipelineRun: invalidNames[1],
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			testAssets, cancel := getPipelineRunController(t, test.Data{})
			defer cancel()
			c := testAssets.Controller

			err := c.Reconciler.Reconcile(testAssets.Ctx, tc.pipelineRun)
			// No reason to keep reconciling something that doesnt or can't exist
			if err != nil {
				t.Errorf("Did not expect to see error when reconciling invalid PipelineRun but saw %q", err)
			}
		})
	}
}

func TestReconcileOnCompletedPipelineRun(t *testing.T) {
	// TestReconcileOnCompletedPipelineRun runs "Reconcile" on a PipelineRun that already reached completion
	// and that does not have the latest status from TaskRuns yet. It checks that the TaskRun status is updated
	// in the PipelineRun status, that the completion status is not altered, that not error is returned and
	// a successful event is triggered
	namespace := "foo"
	taskRunName := "test-pipeline-run-completed-hello-world-task-run"
	customRunName := "test-pipeline-run-completed-hello-world-custom-run"
	pipelineRunName := "test-pipeline-run-completed"
	crs := []*v1beta1.CustomRun{parse.MustParseCustomRun(t, fmt.Sprintf(`
metadata:
  name: %s
spec: {}
status:
  conditions:
  - status: "False"
  type: Succeeded
`, customRunName))}
	prs := []*v1.PipelineRun{parse.MustParseV1PipelineRun(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: foo
spec:
  pipelineRef:
    name: test-pipeline
  taskRunTemplate:
    serviceAccountName: test-sa
status:
  conditions:
  - lastTransitionTime: null
    message: All Tasks have completed executing
    reason: Succeeded
    status: "True"
    type: Succeeded
  childReferences:
    - name: test-pipeline-run-completed-hello-world-task-run
      pipelineTaskName: hello-world-1
      kind: TaskRun
      apiVersion: tekton.dev/v1
    - name: test-pipeline-run-completed-hello-world-custom-run
      pipelineTaskName: hello-world-1
      kind: CustomRun
      apiVersion: tekton.dev/v1
`, pipelineRunName))}
	ps := []*v1.Pipeline{simpleHelloWorldPipeline}
	ts := []*v1.Task{simpleHelloWorldTask}
	trs := []*v1.TaskRun{createHelloWorldTaskRunWithStatus(t, taskRunName, "foo",
		pipelineRunName, "test-pipeline", "",
		apis.Condition{
			Type: apis.ConditionSucceeded,
		})}

	expectedChildReferences := []v1.ChildStatusReference{{
		TypeMeta: runtime.TypeMeta{
			APIVersion: v1.SchemeGroupVersion.String(),
			Kind:       "TaskRun",
		},
		Name:             taskRunName,
		PipelineTaskName: "hello-world-1",
	}, {
		TypeMeta: runtime.TypeMeta{
			APIVersion: v1.SchemeGroupVersion.String(),
			Kind:       customRun,
		},
		Name:             customRunName,
		PipelineTaskName: "hello-world-1",
	}}

	expectedTaskRunsStatus := make(map[string]*v1.PipelineRunTaskRunStatus)
	expectedTaskRunsStatus[taskRunName] = &v1.PipelineRunTaskRunStatus{
		PipelineTaskName: "hello-world-1",
		Status: &v1.TaskRunStatus{
			Status: duckv1.Status{
				Conditions: []apis.Condition{{Type: apis.ConditionSucceeded}},
			},
		},
	}

	d := test.Data{
		PipelineRuns: prs,
		Pipelines:    ps,
		Tasks:        ts,
		TaskRuns:     trs,
		CustomRuns:   crs,
	}
	prt := newPipelineRunTest(t, d)
	defer prt.Cancel()

	wantEvents := []string{
		"Normal Succeeded All Tasks have completed executing",
	}
	reconciledRun, clients := prt.reconcileRun(namespace, pipelineRunName, wantEvents, false)

	taskRuns := getTaskRunsForPipelineRun(prt.TestAssets.Ctx, t, clients, namespace, pipelineRunName)
	validateTaskRunsCount(t, taskRuns, 1)

	// This PipelineRun should still be complete and the status should reflect that
	if reconciledRun.Status.GetCondition(apis.ConditionSucceeded).IsUnknown() {
		t.Errorf("Expected PipelineRun status to be complete, but was %v", reconciledRun.Status.GetCondition(apis.ConditionSucceeded))
	}

	if d := cmp.Diff(reconciledRun.Status.ChildReferences, expectedChildReferences); d != "" {
		t.Fatalf("Expected PipelineRun status to match ChildReference(s) status, but got a mismatch %s", diff.PrintWantGot(d))
	}

	checkTaskRunStatusFromChildRefs(prt.TestAssets.Ctx, t, "foo", clients, reconciledRun.Status.ChildReferences, expectedTaskRunsStatus)
}

func newFeatureFlagsConfigMap() *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Namespace: system.Namespace(), Name: config.GetFeatureFlagsConfigName()},
		Data:       make(map[string]string),
	}
}

func newDefaultsConfigMap() *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: config.GetDefaultsConfigName(), Namespace: system.Namespace()},
		Data:       make(map[string]string),
	}
}

func withEnabledAlphaAPIFields(cm *corev1.ConfigMap) *corev1.ConfigMap {
	newCM := cm.DeepCopy()
	newCM.Data[apiFieldsFeatureFlag] = config.AlphaAPIFields
	return newCM
}

func withMaxMatrixCombinationsCount(cm *corev1.ConfigMap, count int) *corev1.ConfigMap {
	newCM := cm.DeepCopy()
	newCM.Data[maxMatrixCombinationsCountFlag] = strconv.Itoa(count)
	return newCM
}

func withoutAffinityAssistant(cm *corev1.ConfigMap) *corev1.ConfigMap {
	newCM := cm.DeepCopy()
	newCM.Data[disableAffinityAssistantFlag] = "true"
	return newCM
}

// TestReconcileOnCancelledPipelineRun runs "Reconcile" on a PipelineRun that
// has been cancelled.  It verifies that reconcile is successful, the pipeline
// status updated and events generated.
func TestReconcileOnCancelledPipelineRun(t *testing.T) {
	testCases := []struct {
		name       string
		specStatus v1.PipelineRunSpecStatus
		reason     string
	}{
		{
			name:       "cancelled",
			specStatus: v1.PipelineRunSpecStatusCancelled,
			reason:     ReasonCancelled,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			prs := []*v1.PipelineRun{createCancelledPipelineRun(t, "test-pipeline-run-cancelled", tc.specStatus)}
			ps := []*v1.Pipeline{simpleHelloWorldPipeline}
			ts := []*v1.Task{simpleHelloWorldTask}
			trs := []*v1.TaskRun{createHelloWorldTaskRun(t, "test-pipeline-run-cancelled-hello-world", "foo",
				"test-pipeline-run-cancelled", "test-pipeline")}
			cms := []*corev1.ConfigMap{newFeatureFlagsConfigMap()}

			d := test.Data{
				PipelineRuns: prs,
				Pipelines:    ps,
				Tasks:        ts,
				TaskRuns:     trs,
				ConfigMaps:   cms,
			}
			prt := newPipelineRunTest(t, d)
			defer prt.Cancel()

			wantEvents := []string{
				"Warning Failed PipelineRun \"test-pipeline-run-cancelled\" was cancelled",
			}
			reconciledRun, clients := prt.reconcileRun("foo", "test-pipeline-run-cancelled", wantEvents, false)
			actions := clients.Pipeline.Actions()

			if reconciledRun.Status.CompletionTime == nil {
				t.Errorf("Expected a CompletionTime on invalid PipelineRun but was nil")
			}

			// This PipelineRun should still be complete and false, and the status should reflect that
			checkPipelineRunConditionStatusAndReason(t, reconciledRun, corev1.ConditionFalse, tc.reason)

			// Check that no TaskRun is created or run
			for _, action := range actions {
				actionType := fmt.Sprintf("%T", action)
				if !(actionType == "testing.UpdateActionImpl" || actionType == "testing.GetActionImpl") {
					t.Errorf("Expected a TaskRun to be get/updated, but it was %s", actionType)
				}
			}
		})
	}
}

func TestReconcileForCustomTaskWithPipelineTaskTimedOut(t *testing.T) {
	names.TestingSeed()
	// TestReconcileForCustomTaskWithPipelineTaskTimedOut runs "Reconcile" on a PipelineRun.
	// It verifies that reconcile is successful, and the individual
	// custom task which has timed out, is patched as cancelled.
	ps := []*v1.Pipeline{parse.MustParseV1Pipeline(t, `
metadata:
  name: test-pipeline
  namespace: test
spec:
  tasks:
  - name: hello-world-1
    taskRef:
      apiVersion: example.dev/v0
      kind: Example
`)}
	prName := "test-pipeline-run-custom-task-with-timeout"
	prs := []*v1.PipelineRun{parse.MustParseV1PipelineRun(t, `
metadata:
  name: test-pipeline-run-custom-task-with-timeout
  namespace: test
spec:
  pipelineRef:
    name: test-pipeline
  taskRunTemplate:
    serviceAccountName: test-sa
`)}
	runs := []*v1beta1.CustomRun{mustParseCustomRunWithObjectMeta(t,
		taskRunObjectMeta("test-pipeline-run-custom-task-with-timeout-hello-world-1", "test", "test-pipeline-run-custom-task-with-timeout",
			"test-pipeline", "hello-world-1", true),
		`
spec:
  customRef:
    apiVersion: example.dev/v0
    kind: Example
  timeout: 1m0s
status:
  conditions:
  - status: Unknown
    type: Succeeded
  startTime: "2021-12-31T23:58:59Z"
`)}
	cms := []*corev1.ConfigMap{newFeatureFlagsConfigMap()}
	d := test.Data{
		PipelineRuns: prs,
		Pipelines:    ps,
		ConfigMaps:   cms,
		CustomRuns:   runs,
	}
	prt := newPipelineRunTest(t, d)
	defer prt.Cancel()

	wantEvents := []string{
		"Normal Started",
		"Normal Running Tasks Completed: 0 \\(Failed: 0, Cancelled 0\\), Incomplete: 1, Skipped: 0",
	}
	_, clients := prt.reconcileRun("test", prName, wantEvents, false)

	actions := clients.Pipeline.Actions()
	if len(actions) < 2 {
		t.Fatalf("Expected client to have at least two action implementation but it has %d", len(actions))
	}

	// The patch operation to cancel the CustomRun must be executed.
	var got []jsonpatch.Operation
	for _, a := range actions {
		if action, ok := a.(ktesting.PatchAction); ok {
			if a.(ktesting.PatchAction).Matches("patch", "customruns") {
				err := json.Unmarshal(action.GetPatch(), &got)
				if err != nil {
					t.Fatalf("Expected to get a patch operation for cancel,"+
						" but got error: %v\n", err)
				}
				break
			}
		}
	}
	var want []jsonpatch.JsonPatchOperation
	if d := cmp.Diff(got, want); d != "" {
		t.Fatalf("Expected no operation, but got %v", got)
	}
}

func TestReconcileForCustomTaskWithPipelineRunTimedOut(t *testing.T) {
	names.TestingSeed()
	// TestReconcileForCustomTaskWithPipelineRunTimedOut runs "Reconcile" on a
	// PipelineRun that has timed out.
	// It verifies that reconcile is successful, and the custom task has also timed
	// out and patched as cancelled.
	for _, tc := range []struct {
		name     string
		timeout  *metav1.Duration
		timeouts *v1.TimeoutFields
	}{{
		name:     "spec.Timeouts.Pipeline",
		timeouts: &v1.TimeoutFields{Pipeline: &metav1.Duration{Duration: 12 * time.Hour}},
	}} {
		t.Run(tc.name, func(*testing.T) {
			ps := []*v1.Pipeline{parse.MustParseV1Pipeline(t, `
metadata:
  name: test-pipeline
  namespace: test
spec:
  tasks:
  - name: hello-world-1
    taskRef:
      apiVersion: example.dev/v0
      kind: Example
`)}

			prName := "test-pipeline-run-custom-task"
			prs := []*v1.PipelineRun{parse.MustParseV1PipelineRun(t, `
metadata:
  name: test-pipeline-run-custom-task
  namespace: test
spec:
  pipelineRef:
    name: test-pipeline
  taskRunTemplate:
    serviceAccountName: test-sa
status:
  conditions:
  - message: running...
    reason: Running
    status: Unknown
    type: Succeeded
  startTime: "2021-12-31T11:00:00Z"
  childReferences:
  - name: test-pipeline-run-custom-task-hello-world-1
    pipelineTaskName: hello-world-1
    kind: CustomRun
    apiVersion: example.dev/v0
`)}

			prs[0].Spec.Timeouts = tc.timeouts

			customRuns := []*v1beta1.CustomRun{mustParseCustomRunWithObjectMeta(t,
				taskRunObjectMeta("test-pipeline-run-custom-task-hello-world-1", "test", "test-pipeline-run-custom-task",
					"test-pipeline", "hello-world-1", true),
				`
spec:
  customRef:
    apiVersion: example.dev/v0
    kind: Example
status:
  conditions:
  - status: Unknown
    type: Succeeded
  startTime: "2021-12-31T11:58:59Z"
  creationTime: "2021-12-31T11:58:58Z"
`)}

			cms := []*corev1.ConfigMap{newFeatureFlagsConfigMap()}
			d := test.Data{
				PipelineRuns: prs,
				Pipelines:    ps,
				ConfigMaps:   cms,
				CustomRuns:   customRuns,
			}
			prt := newPipelineRunTest(t, d)
			defer prt.Cancel()

			wantEvents := []string{
				fmt.Sprintf("Warning Failed PipelineRun \"%s\" failed to finish within \"12h0m0s\"", prName),
			}

			reconciledRun, clients := prt.reconcileRun("test", prName, wantEvents, false)

			if reconciledRun.Status.CompletionTime == nil {
				t.Errorf("Expected a CompletionTime on already timedout PipelineRun but was nil")
			}

			// The PipelineRun should be timed out.
			if reconciledRun.Status.GetCondition(apis.ConditionSucceeded).Reason != "PipelineRunTimeout" {
				t.Errorf("Expected PipelineRun to be timed out, but condition reason is %s",
					reconciledRun.Status.GetCondition(apis.ConditionSucceeded))
			}

			actions := clients.Pipeline.Actions()
			if len(actions) < 2 {
				t.Fatalf("Expected client to have at least two action implementation but it has %d", len(actions))
			}

			// The patch operation to cancel the CustomRun must be executed.
			var got []jsonpatch.Operation
			for _, a := range actions {
				if action, ok := a.(ktesting.PatchAction); ok {
					if a.(ktesting.PatchAction).Matches("patch", "customruns") {
						err := json.Unmarshal(action.GetPatch(), &got)
						if err != nil {
							t.Fatalf("Expected to get a patch operation for cancel,"+
								" but got error: %v\n", err)
						}
						break
					}
				}
			}
			want := []jsonpatch.JsonPatchOperation{{
				Operation: "add",
				Path:      "/spec/status",
				Value:     string(v1beta1.CustomRunSpecStatusCancelled),
			}, {
				Operation: "add",
				Path:      "/spec/statusMessage",
				Value:     string(v1beta1.CustomRunCancelledByPipelineTimeoutMsg),
			}}
			if d := cmp.Diff(want, got); d != "" {
				t.Fatalf("Expected CustomRunCancelled patch operation, but got a mismatch %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestReconcileOnCancelledRunFinallyPipelineRun(t *testing.T) {
	// TestReconcileOnCancelledRunFinallyPipelineRun runs "Reconcile" on a PipelineRun that has been gracefully cancelled.
	// It verifies that reconcile is successful, the pipeline status updated and events generated.
	prs := []*v1.PipelineRun{createCancelledPipelineRun(t, "test-pipeline-run-cancelled-run-finally", v1.PipelineRunSpecStatusCancelledRunFinally)}
	ps := []*v1.Pipeline{helloWorldPipelineWithRunAfter(t)}
	ts := []*v1.Task{simpleHelloWorldTask}
	cms := []*corev1.ConfigMap{newFeatureFlagsConfigMap()}
	d := test.Data{
		PipelineRuns: prs,
		Pipelines:    ps,
		Tasks:        ts,
		ConfigMaps:   cms,
	}
	prt := newPipelineRunTest(t, d)
	defer prt.Cancel()

	wantEvents := []string{
		"Warning Failed PipelineRun \"test-pipeline-run-cancelled-run-finally\" was cancelled",
	}
	reconciledRun, _ := prt.reconcileRun("foo", "test-pipeline-run-cancelled-run-finally", wantEvents, false)

	if reconciledRun.Status.CompletionTime == nil {
		t.Errorf("Expected a CompletionTime on invalid PipelineRun but was nil")
	}

	// This PipelineRun should still be complete and false, and the status should reflect that
	if !reconciledRun.Status.GetCondition(apis.ConditionSucceeded).IsFalse() {
		t.Errorf("Expected PipelineRun status to be complete and false, but was %v", reconciledRun.Status.GetCondition(apis.ConditionSucceeded))
	}

	// There should be no task runs triggered for the pipeline tasks
	verifyTaskRunStatusesCount(t, reconciledRun.Status, 0)

	expectedSkippedTasks := []v1.SkippedTask{{
		Name:   "hello-world-1",
		Reason: v1.GracefullyCancelledSkip,
	}, {
		Name:   "hello-world-2",
		Reason: v1.GracefullyCancelledSkip,
	}}

	if d := cmp.Diff(expectedSkippedTasks, reconciledRun.Status.SkippedTasks); d != "" {
		t.Fatalf("Didn't get the expected list of skipped tasks. Diff: %s", diff.PrintWantGot(d))
	}
}

// TestReconcileOnCancelledRunFinallyPipelineRunWithFinalTask runs
// "Reconcile" on a PipelineRun that has been gracefully cancelled.  It verifies
// that reconcile is successful, final tasks run, the pipeline status updated
// and events generated.
func TestReconcileOnCancelledRunFinallyPipelineRunWithFinalTask(t *testing.T) {
	prs := []*v1.PipelineRun{createCancelledPipelineRun(t, "test-pipeline-run-cancelled-run-finally", v1.PipelineRunSpecStatusCancelledRunFinally)}
	ps := []*v1.Pipeline{
		parse.MustParseV1Pipeline(t, `
metadata:
  name: test-pipeline
  namespace: foo
spec:
  tasks:
    - name: hello-world-1
      taskRef:
        name: hello-world
    - name: hello-world-2
      taskRef:
        name: hello-world
      runAfter:
        - hello-world-1
  finally:
    - name: final-task-1
      taskRef:
        name: some-task
`),
	}
	ts := []*v1.Task{
		simpleHelloWorldTask,
		simpleSomeTask,
	}
	cms := []*corev1.ConfigMap{newFeatureFlagsConfigMap()}

	d := test.Data{
		PipelineRuns: prs,
		Pipelines:    ps,
		Tasks:        ts,
		ConfigMaps:   cms,
	}
	prt := newPipelineRunTest(t, d)
	defer prt.Cancel()

	wantEvents := []string{
		"Normal Started",
	}
	reconciledRun, _ := prt.reconcileRun("foo", "test-pipeline-run-cancelled-run-finally", wantEvents, false)

	if reconciledRun.Status.CompletionTime != nil {
		t.Errorf("Expected a CompletionTime to be nil on incomplete PipelineRun but was %v", reconciledRun.Status.CompletionTime)
	}

	// This PipelineRun should still be complete and false, and the status should reflect that
	if !reconciledRun.Status.GetCondition(apis.ConditionSucceeded).IsUnknown() {
		t.Errorf("Expected PipelineRun status to be complete and unknown, but was %v", reconciledRun.Status.GetCondition(apis.ConditionSucceeded))
	}

	// There should be exactly one task run triggered for the "final-task-1" final task
	verifyTaskRunStatusesCount(t, reconciledRun.Status, 1)
	verifyTaskRunStatusesNames(t, reconciledRun.Status, "test-pipeline-run-cancelled-run-finally-final-task-1")
}

func TestReconcileOnCancelledRunFinallyPipelineRunWithRunningFinalTask(t *testing.T) {
	// TestReconcileOnCancelledRunFinallyPipelineRunWithRunningFinalTask runs "Reconcile" on a PipelineRun that has been gracefully cancelled.
	// It verifies that reconcile is successful and completed tasks and running final tasks are left untouched.
	prs := []*v1.PipelineRun{parse.MustParseV1PipelineRun(t, `
metadata:
  name: test-pipeline-run-cancelled-run-finally
  namespace: foo
spec:
  pipelineRef:
    name: test-pipeline
  taskRunTemplate:
    serviceAccountName: test-sa
  status: CancelledRunFinally
status:
  startTime: "2022-01-01T00:00:00Z"
  childReferences:
  - apiVersion: tekton.dev/v1
    kind: TaskRun
    name: test-pipeline-run-cancelled-run-finally-final-task
    pipelineTaskName: final-task-1
  - apiVersion: tekton.dev/v1
    kind: TaskRun
    name: test-pipeline-run-cancelled-run-finally-hello-world
    pipelineTaskName: hello-world-1
    status:
        conditions:
        - lastTransitionTime: null
          status: "True"
          type: Succeeded
`)}
	ps := []*v1.Pipeline{parse.MustParseV1Pipeline(t, `
metadata:
  name: test-pipeline
  namespace: foo
spec:
  finally:
  - name: final-task-1
    taskRef:
      name: some-task
  tasks:
  - name: hello-world-1
    taskRef:
      name: hello-world
`)}
	ts := []*v1.Task{
		simpleHelloWorldTask,
		simpleSomeTask,
	}
	trs := []*v1.TaskRun{
		createHelloWorldTaskRunWithStatus(t, "test-pipeline-run-cancelled-run-finally-hello-world", "foo",
			"test-pipeline-run-cancelled-run-finally", "test-pipeline", "my-pod-name",
			apis.Condition{
				Type:   apis.ConditionSucceeded,
				Status: corev1.ConditionTrue,
			}),
		createHelloWorldTaskRun(t, "test-pipeline-run-cancelled-run-finally-final-task", "foo",
			"test-pipeline-run-cancelled-run-finally", "test-pipeline"),
	}
	d := test.Data{
		PipelineRuns: prs,
		Pipelines:    ps,
		Tasks:        ts,
		TaskRuns:     trs,
	}
	prt := newPipelineRunTest(t, d)
	defer prt.Cancel()

	wantEvents := []string{
		"Normal Started",
	}
	reconciledRun, clients := prt.reconcileRun("foo", "test-pipeline-run-cancelled-run-finally", wantEvents, false)

	if reconciledRun.Status.CompletionTime != nil {
		t.Errorf("Expected a CompletionTime to be nil on incomplete PipelineRun but was %v", reconciledRun.Status.CompletionTime)
	}

	// This PipelineRun should still be complete and unknown, and the status should reflect that
	if !reconciledRun.Status.GetCondition(apis.ConditionSucceeded).IsUnknown() {
		t.Errorf("Expected PipelineRun status to be complete and unknown, but was %v", reconciledRun.Status.GetCondition(apis.ConditionSucceeded))
	}

	// There should be 2 task runs, one for already completed "hello-world-1" task and one for the "final-task-1" final task
	if len(reconciledRun.Status.ChildReferences) != 2 {
		t.Errorf("Expected PipelineRun status to have 2 child references, but was %v", len(reconciledRun.Status.ChildReferences))
	}

	actions := clients.Pipeline.Actions()
	patchActions := make([]ktesting.PatchAction, 0)
	for _, action := range actions {
		if patchAction, ok := action.(ktesting.PatchAction); ok {
			patchActions = append(patchActions, patchAction)
		}
	}
	if len(patchActions) != 0 {
		t.Errorf("Expected no patch actions, but was %v", len(patchActions))
	}
}

func TestReconcileOnCancelledRunFinallyPipelineRunWithFinalTaskAndRetries(t *testing.T) {
	// TestReconcileOnCancelledRunFinallyPipelineRunWithFinalTaskAndRetries runs "Reconcile" on a PipelineRun that has
	// been gracefully cancelled. It verifies that reconcile is successful, the pipeline status updated and events generated.
	// Pipeline has a DAG task "hello-world-1" and Finally task "hello-world-2"
	ps := []*v1.Pipeline{{
		ObjectMeta: baseObjectMeta("test-pipeline", "foo"),
		Spec: v1.PipelineSpec{
			Tasks: []v1.PipelineTask{{
				Name: "hello-world-1",
				TaskRef: &v1.TaskRef{
					Name: "hello-world",
				},
				Retries: 2,
			}},
			Finally: []v1.PipelineTask{{
				Name: "hello-world-2",
				TaskRef: &v1.TaskRef{
					Name: "hello-world",
				},
			}},
		},
	}}

	// PipelineRun has been gracefully cancelled, and it has a TaskRun for DAG task "hello-world-1" that has failed
	// with reason of cancellation
	prs := []*v1.PipelineRun{{
		ObjectMeta: baseObjectMeta("test-pipeline-run-cancelled-run-finally", "foo"),
		Spec: v1.PipelineRunSpec{
			PipelineRef: &v1.PipelineRef{Name: "test-pipeline"},
			TaskRunTemplate: v1.PipelineTaskRunTemplate{
				ServiceAccountName: "test-sa",
			},
			Status: v1.PipelineRunSpecStatusCancelledRunFinally,
		},
		Status: v1.PipelineRunStatus{
			PipelineRunStatusFields: v1.PipelineRunStatusFields{},
		},
	}}

	prs[0].Status.ChildReferences = append(prs[0].Status.ChildReferences, v1.ChildStatusReference{
		TypeMeta: runtime.TypeMeta{
			APIVersion: v1.SchemeGroupVersion.String(),
			Kind:       "TaskRun",
		},
		Name:             "test-pipeline-run-cancelled-run-finally-hello-world",
		PipelineTaskName: "hello-world-1",
	})

	// TaskRun exists for DAG task "hello-world-1" that has failed with reason of cancellation
	trs := []*v1.TaskRun{createHelloWorldTaskRunWithStatus(t, "test-pipeline-run-cancelled-run-finally-hello-world", "foo",
		"test-pipeline-run-cancelled-run-finally", "test-pipeline", "my-pod-name",
		apis.Condition{
			Type:   apis.ConditionSucceeded,
			Status: corev1.ConditionFalse,
			Reason: v1.TaskRunSpecStatusCancelled,
		})}

	ts := []*v1.Task{simpleHelloWorldTask}
	cms := []*corev1.ConfigMap{newFeatureFlagsConfigMap()}

	d := test.Data{
		PipelineRuns: prs,
		Pipelines:    ps,
		TaskRuns:     trs,
		Tasks:        ts,
		ConfigMaps:   cms,
	}
	prt := newPipelineRunTest(t, d)
	defer prt.Cancel()

	wantEvents := []string{
		"Normal Started",
		"Normal CancelledRunningFinally Tasks Completed: 1 \\(Failed: 0, Cancelled 1\\), Incomplete: 1, Skipped: 0",
	}
	reconciledRun, _ := prt.reconcileRun("foo", "test-pipeline-run-cancelled-run-finally", wantEvents, false)

	// This PipelineRun should still be running to execute the finally task, and the status should reflect that
	if !reconciledRun.Status.GetCondition(apis.ConditionSucceeded).IsUnknown() {
		t.Errorf("Expected PipelineRun status to be running to execute the finally task, but was %v",
			reconciledRun.Status.GetCondition(apis.ConditionSucceeded))
	}

	// There should be two task runs (failed dag task and one triggered for the finally task)
	verifyTaskRunStatusesCount(t, reconciledRun.Status, 2)
}

func TestReconcileTaskResolutionError(t *testing.T) {
	ts := []*v1.Task{
		simpleHelloWorldTask,
	}
	ptName := "hello-world-1"
	prName := "test-pipeline-fails-task-resolution"
	prs := []*v1.PipelineRun{
		parse.MustParseV1PipelineRun(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: foo
spec:
  pipelineRef:
    name: test-pipeline
  status: %s
status:
  conditions:
  - type: Succeeded
    status: Unknown
    reason: Running
    message: "running..."
  childReferences:
  - name: %s%s
    pipelineTaskName: %s
    kind: TaskRun
`, prName, v1.PipelineRunSpecStatusCancelledRunFinally, ptName, prName, ptName)),
	}

	trs := []*v1.TaskRun{
		getTaskRun(
			t,
			"test-pipeline-fails-task-resolutionhello-world-1",
			prName,
			"test-pipeline",
			"hello-world",
			corev1.ConditionUnknown,
		),
	}
	d := test.Data{
		PipelineRuns: prs,
		Pipelines:    []*v1.Pipeline{simpleHelloWorldPipeline},
		Tasks:        ts,
		TaskRuns:     trs,
		ConfigMaps:   []*corev1.ConfigMap{},
	}
	testAssets, cancel := getPipelineRunController(t, d)
	defer cancel()
	c := testAssets.Controller
	clients := testAssets.Clients

	failingReactorActivated := true
	// Create an error when the Pipeline client attempts to resolve the Task
	clients.Pipeline.PrependReactor("*", "tasks", func(action ktesting.Action) (bool, runtime.Object, error) {
		return failingReactorActivated, nil, errors.New("etcdserver: leader changed")
	})
	err := c.Reconciler.Reconcile(testAssets.Ctx, "foo/test-pipeline-fails-task-resolution")
	if err == nil {
		t.Errorf("Expected error returned from reconcile after failing to cancel TaskRun but saw none!")
	}

	if controller.IsPermanentError(err) {
		t.Errorf("Expected non-permanent error reconciling PipelineRun but got permanent error: %v", err)
	}

	// Check that the PipelineRun is still running with correct error message
	reconciledRun, err := clients.Pipeline.TektonV1().PipelineRuns("foo").Get(testAssets.Ctx, "test-pipeline-fails-task-resolution", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Somehow had error getting reconciled run out of fake client: %s", err)
	}

	// The PipelineRun should not be cancelled since the error was retryable
	condition := reconciledRun.Status.GetCondition(apis.ConditionSucceeded)
	if !condition.IsUnknown() {
		t.Errorf("Expected PipelineRun to still be running but succeeded condition is %v", condition.Status)
	}

	failingReactorActivated = false
	err = c.Reconciler.Reconcile(testAssets.Ctx, "foo/test-pipeline-fails-task-resolution")
	if err != nil {
		if ok, _ := controller.IsRequeueKey(err); !ok {
			t.Errorf("unexpected error in PipelineRun reconciliation: %v", err)
		}
	}
	condition = reconciledRun.Status.GetCondition(apis.ConditionSucceeded)
	if !condition.IsUnknown() {
		t.Errorf("Expected PipelineRun to still be running but succeeded condition is %v", condition.Status)
	}
}

func TestReconcileOnStoppedPipelineRun(t *testing.T) {
	basePRYAML := fmt.Sprintf(`
metadata:
  name: "test-pipeline-run-stopped-run-finally"
  namespace: foo
spec:
  pipelineRef:
    name: test-pipeline
  taskRunTemplate:
    serviceAccountName: test-sa
  status: %s
status:
  startTime: %s`, v1.PipelineRunSpecStatusStoppedRunFinally, now.Format(time.RFC3339))

	testCases := []struct {
		name                   string
		pipeline               *v1.Pipeline
		taskRuns               []*v1.TaskRun
		initialChildReferences []v1.ChildStatusReference
		expectedEvents         []string
		hasNilCompletionTime   bool
		isFailed               bool
		childRefInStatusCount  int
		skippedTasks           []v1.SkippedTask
	}{
		{
			name:                   "stopped PipelineRun",
			pipeline:               simpleHelloWorldPipeline,
			taskRuns:               nil,
			initialChildReferences: nil,
			expectedEvents:         []string{"Warning Failed PipelineRun \"test-pipeline-run-stopped-run-finally\" was cancelled"},
			hasNilCompletionTime:   false,
			isFailed:               true,
			childRefInStatusCount:  0,
			skippedTasks:           []v1.SkippedTask{{Name: "hello-world-1", Reason: v1.GracefullyStoppedSkip}},
		}, {
			name:     "with running task",
			pipeline: simpleHelloWorldPipeline,
			taskRuns: []*v1.TaskRun{getTaskRun(
				t,
				"test-pipeline-run-stopped-run-finally-hello-world",
				"test-pipeline-run-stopped-run-finally",
				"test-pipeline",
				"hello-world",
				corev1.ConditionUnknown,
			)},
			initialChildReferences: []v1.ChildStatusReference{{
				TypeMeta: runtime.TypeMeta{
					APIVersion: v1.SchemeGroupVersion.String(),
					Kind:       "TaskRun",
				},
				Name:             "test-pipeline-run-stopped-run-finally-hello-world",
				PipelineTaskName: "hello-world-1",
			}},
			expectedEvents:        []string{"Normal Started"},
			hasNilCompletionTime:  true,
			isFailed:              false,
			childRefInStatusCount: 1,
			skippedTasks:          nil,
		}, {
			name:     "with completed task",
			pipeline: helloWorldPipelineWithRunAfter(t),
			taskRuns: []*v1.TaskRun{getTaskRun(
				t,
				"test-pipeline-run-stopped-run-finally-hello-world",
				"test-pipeline-run-stopped-run-finally",
				"test-pipeline",
				"hello-world",
				corev1.ConditionTrue,
			)},
			initialChildReferences: []v1.ChildStatusReference{{
				TypeMeta: runtime.TypeMeta{
					APIVersion: v1.SchemeGroupVersion.String(),
					Kind:       "TaskRun",
				},
				Name:             "test-pipeline-run-stopped-run-finally-hello-world",
				PipelineTaskName: "hello-world-1",
			},
			},
			expectedEvents:        []string{"Warning Failed PipelineRun \"test-pipeline-run-stopped-run-finally\" was cancelled"},
			hasNilCompletionTime:  false,
			isFailed:              true,
			childRefInStatusCount: 1,
			skippedTasks:          []v1.SkippedTask{{Name: "hello-world-2", Reason: v1.GracefullyStoppedSkip}},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			pr := parse.MustParseV1PipelineRun(t, basePRYAML)
			if tc.initialChildReferences != nil {
				pr.Status.ChildReferences = tc.initialChildReferences
			}
			ps := []*v1.Pipeline{tc.pipeline}
			ts := []*v1.Task{simpleHelloWorldTask}
			d := test.Data{
				PipelineRuns: []*v1.PipelineRun{pr},
				Pipelines:    ps,
				Tasks:        ts,
				TaskRuns:     tc.taskRuns,
			}
			prt := newPipelineRunTest(t, d)
			defer prt.Cancel()

			wantEvents := append([]string{}, tc.expectedEvents...)

			reconciledRun, clients := prt.reconcileRun("foo", "test-pipeline-run-stopped-run-finally", wantEvents, false)

			if (reconciledRun.Status.CompletionTime == nil) != tc.hasNilCompletionTime {
				t.Errorf("Expected CompletionTime == nil to be %t on invalid PipelineRun but was %t", tc.hasNilCompletionTime, reconciledRun.Status.CompletionTime == nil)
			}

			if tc.isFailed && !reconciledRun.Status.GetCondition(apis.ConditionSucceeded).IsFalse() {
				t.Errorf("Expected PipelineRun status to be complete and false, but was %v", reconciledRun.Status.GetCondition(apis.ConditionSucceeded))
			} else if !tc.isFailed && !reconciledRun.Status.GetCondition(apis.ConditionSucceeded).IsUnknown() {
				t.Errorf("Expected PipelineRun status to be complete and unknown, but was %v", reconciledRun.Status.GetCondition(apis.ConditionSucceeded))
			}

			if len(reconciledRun.Status.ChildReferences) != tc.childRefInStatusCount {
				t.Fatalf("Expected %d ChildRerences in status but got %d", tc.childRefInStatusCount, len(reconciledRun.Status.ChildReferences))
			}

			if d := cmp.Diff(tc.skippedTasks, reconciledRun.Status.SkippedTasks); d != "" {
				t.Fatalf("Didn't get the expected list of skipped tasks. Diff: %s", diff.PrintWantGot(d))
			}

			actions := clients.Pipeline.Actions()
			patchCount := 0
			for _, action := range actions {
				if _, ok := action.(ktesting.PatchAction); ok {
					patchCount++
				}
			}
			if patchCount != 0 {
				t.Errorf("Expected no patch action, but was %v", patchCount)
			}
		})
	}
}

func TestReconcileOnPendingPipelineRun(t *testing.T) {
	// TestReconcileOnPendingPipelineRun runs "Reconcile" on a PipelineRun that is pending.
	// It verifies that reconcile is successful, the pipeline status updated and events generated.
	prs := []*v1.PipelineRun{parse.MustParseV1PipelineRun(t, `
metadata:
  name: test-pipeline-run-pending
  namespace: foo
spec:
  pipelineRef:
    name: test-pipeline
  taskRunTemplate:
    serviceAccountName: test-sa
  status: PipelineRunPending
`)}
	ps := []*v1.Pipeline{simpleHelloWorldPipeline}
	var ts []*v1.Task
	var trs []*v1.TaskRun

	d := test.Data{
		PipelineRuns: prs,
		Pipelines:    ps,
		Tasks:        ts,
		TaskRuns:     trs,
	}
	prt := newPipelineRunTest(t, d)
	defer prt.Cancel()

	var wantEvents []string
	reconciledRun, _ := prt.reconcileRun("foo", "test-pipeline-run-pending", wantEvents, false)

	checkPipelineRunConditionStatusAndReason(t, reconciledRun, corev1.ConditionUnknown, v1.PipelineRunReasonPending.String())

	if reconciledRun.Status.StartTime != nil {
		t.Errorf("Start time should be nil, not: %s", reconciledRun.Status.StartTime)
	}
}

func TestReconcileWithTimeouts_Pipeline(t *testing.T) {
	// TestReconcileWithTimeouts_Pipeline runs "Reconcile" on a PipelineRun that has timed out.
	// It verifies that reconcile is successful, no TaskRun is created, the PipelineTask is marked as skipped, and the
	// pipeline status updated and events generated.
	ps := []*v1.Pipeline{parse.MustParseV1Pipeline(t, `
metadata:
  name: test-pipeline
  namespace: foo
spec:
  tasks:
  - name: hello-world-1
    taskRef:
      name: hello-world
  - name: hello-world-2
    taskRef:
      name: hello-world
`)}
	prs := []*v1.PipelineRun{parse.MustParseV1PipelineRun(t, `
metadata:
  name: test-pipeline-run-with-timeout
  namespace: foo
spec:
  pipelineRef:
    name: test-pipeline
  taskRunTemplate:
    serviceAccountName: test-sa
  timeouts:
    pipeline: 12h0m0s
status:
  startTime: "2021-12-31T11:00:00Z"
  childReferences:
  - name: test-pipeline-run-with-timeout-hello-world-1
    pipelineTaskName: hello-world-1
    kind: TaskRun
`)}
	ts := []*v1.Task{simpleHelloWorldTask}

	trs := []*v1.TaskRun{mustParseTaskRunWithObjectMeta(t, taskRunObjectMeta("test-pipeline-run-with-timeout-hello-world-1", "foo", "test-pipeline-run-with-timeout",
		"test-pipeline", "hello-world-1", false), `
spec:
  serviceAccountName: test-sa
  taskRef:
    name: hello-world
    kind: Task
`)}

	d := test.Data{
		PipelineRuns: prs,
		Pipelines:    ps,
		Tasks:        ts,
		TaskRuns:     trs,
	}
	prt := newPipelineRunTest(t, d)
	defer prt.Cancel()

	wantEvents := []string{
		"Warning Failed PipelineRun \"test-pipeline-run-with-timeout\" failed to finish within \"12h0m0s\"",
	}
	reconciledRun, clients := prt.reconcileRun("foo", "test-pipeline-run-with-timeout", wantEvents, false)

	if reconciledRun.Status.CompletionTime == nil {
		t.Errorf("Expected a CompletionTime on invalid PipelineRun but was nil")
	}

	// The PipelineRun should be timed out.
	if reconciledRun.Status.GetCondition(apis.ConditionSucceeded).Reason != "PipelineRunTimeout" {
		t.Errorf("Expected PipelineRun to be timed out, but condition reason is %s", reconciledRun.Status.GetCondition(apis.ConditionSucceeded))
	}

	// Check that there is a skipped task for the expected reason
	if len(reconciledRun.Status.SkippedTasks) != 1 {
		t.Errorf("expected one skipped task, found %d", len(reconciledRun.Status.SkippedTasks))
	} else if reconciledRun.Status.SkippedTasks[0].Reason != v1.PipelineTimedOutSkip {
		t.Errorf("expected skipped reason to be '%s', but was '%s", v1.PipelineTimedOutSkip, reconciledRun.Status.SkippedTasks[0].Reason)
	}

	updatedTaskRun, err := clients.Pipeline.TektonV1().TaskRuns("foo").Get(context.Background(), trs[0].Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("error getting updated TaskRun: %#v", err)
	}

	if updatedTaskRun.Spec.Status != v1.TaskRunSpecStatusCancelled {
		t.Errorf("expected existing TaskRun Spec.Status to be set to %s, but was %s", v1.TaskRunSpecStatusCancelled, updatedTaskRun.Spec.Status)
	}
	if updatedTaskRun.Spec.StatusMessage != v1.TaskRunCancelledByPipelineTimeoutMsg {
		t.Errorf("expected existing TaskRun Spec.StatusMessage to be set to %s, but was %s", v1.TaskRunCancelledByPipelineTimeoutMsg, updatedTaskRun.Spec.StatusMessage)
	}
}

func TestReconcileWithTimeoutGreaterThan24h(t *testing.T) {
	testCases := []struct {
		name      string
		timeout   time.Duration
		wantError error
	}{
		{
			name:      "pipeline timeout is 24h",
			timeout:   24 * time.Hour,
			wantError: errors.New("PipelineRun has timed out for a long time"),
		},
		{
			name:      "pipeline timeout is way longer than 24h",
			timeout:   360 * time.Hour,
			wantError: errors.New("PipelineRun has timed out for a long time"),
		},
	}

	for _, tc := range testCases {
		startTime := time.Date(2022, time.January, 1, 0, 0, 0, 0, time.UTC).Add(-3 * tc.timeout)
		t.Run(tc.name, func(t *testing.T) {
			ps := []*v1.Pipeline{parse.MustParseV1Pipeline(t, `
metadata:
  name: test-pipeline
  namespace: foo
spec:
  tasks:
  - name: hello-world-1
    taskRef:
      name: hello-world
  - name: hello-world-2
    taskRef:
      name: hello-world
`)}
			prs := []*v1.PipelineRun{parse.MustParseV1PipelineRun(t, `
metadata:
  name: test-pipeline-run-with-timeout
  namespace: foo
spec:
  pipelineRef:
    name: test-pipeline
  taskRunTemplate:
    serviceAccountName: test-sa
  timeouts:
    pipeline: 30h0m0s
status:
  startTime: "2021-12-30T00:00:00Z"
`)}
			ts := []*v1.Task{simpleHelloWorldTask}

			trs := []*v1.TaskRun{mustParseTaskRunWithObjectMeta(t, taskRunObjectMeta("test-pipeline-run-with-timeout-hello-world-1", "foo", "test-pipeline-run-with-timeout",
				"test-pipeline", "hello-world-1", false), `
spec:
  serviceAccountName: test-sa
  taskRef:
    name: hello-world
    kind: Task
`)}
			start := metav1.NewTime(startTime)
			prs[0].Status.StartTime = &start

			d := test.Data{
				PipelineRuns: prs,
				Pipelines:    ps,
				Tasks:        ts,
				TaskRuns:     trs,
			}
			prt := newPipelineRunTest(t, d)
			defer prt.Cancel()

			c := prt.TestAssets.Controller
			clients := prt.TestAssets.Clients
			reconcileError := c.Reconciler.Reconcile(prt.TestAssets.Ctx, "foo/test-pipeline-run-with-timeout")
			if reconcileError == nil {
				t.Fatalf("expected error %q, but got nil", tc.wantError.Error())
			}
			if reconcileError.Error() != tc.wantError.Error() {
				t.Fatalf("Expected error: %s Got: %s", tc.wantError, reconcileError)
			}
			prt.Test.Logf("Getting reconciled run")
			reconciledRun, err := clients.Pipeline.TektonV1().PipelineRuns("foo").Get(prt.TestAssets.Ctx, "test-pipeline-run-with-timeout", metav1.GetOptions{})
			if err != nil {
				prt.Test.Fatalf("Somehow had error getting reconciled run out of fake client: %s", err)
			}
			if reconciledRun.Status.GetCondition(apis.ConditionSucceeded).Reason != "PipelineRunTimeout" {
				t.Errorf("Expected PipelineRun to be timed out, but condition reason is %s", reconciledRun.Status.GetCondition(apis.ConditionSucceeded))
			}
		})
	}
}

func TestReconcileWithTimeoutForALongTimeAndEtcdLimit_Pipeline(t *testing.T) {
	timeout := 12 * time.Hour
	testCases := []struct {
		name      string
		startTime time.Time
		wantError error
	}{
		{
			name:      "pipelinerun has timed out for way too much time",
			startTime: time.Date(2022, time.January, 1, 0, 0, 0, 0, time.UTC).Add(-3 * timeout),
			wantError: errors.New("PipelineRun has timed out for a long time"),
		},
		{
			name:      "pipelinerun has timed out for a long time",
			startTime: time.Date(2022, time.January, 1, 0, 0, 0, 0, time.UTC).Add(-2 * timeout),
			wantError: errors.New("PipelineRun has timed out for a long time"),
		},
		{
			name:      "pipelinerun has timed out for a while",
			startTime: time.Date(2022, time.January, 1, 0, 0, 0, 0, time.UTC).Add(-(3 / 2) * timeout),
			wantError: errors.New("etcdserver: request too large"),
		},
		{
			name:      "pipelinerun has just timed out",
			startTime: time.Date(2022, time.January, 1, 0, 0, 0, 0, time.UTC).Add(-timeout),
			wantError: errors.New("etcdserver: request too large"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ps := []*v1.Pipeline{parse.MustParseV1Pipeline(t, `
metadata:
  name: test-pipeline
  namespace: foo
spec:
  tasks:
  - name: hello-world-1
    taskRef:
      name: hello-world
  - name: hello-world-2
    taskRef:
      name: hello-world
`)}
			prs := []*v1.PipelineRun{parse.MustParseV1PipelineRun(t, `
metadata:
  name: test-pipeline-run-with-timeout
  namespace: foo
spec:
  pipelineRef:
    name: test-pipeline
  taskRunTemplate:
    serviceAccountName: test-sa
  timeouts:
    pipeline: 12h0m0s
status:
  startTime: "2021-12-30T00:00:00Z"
`)}
			ts := []*v1.Task{simpleHelloWorldTask}

			trs := []*v1.TaskRun{mustParseTaskRunWithObjectMeta(t, taskRunObjectMeta("test-pipeline-run-with-timeout-hello-world-1", "foo", "test-pipeline-run-with-timeout",
				"test-pipeline", "hello-world-1", false), `
spec:
  resources: {}
  serviceAccountName: test-sa
  taskRef:
    name: hello-world
    kind: Task
`)}
			start := metav1.NewTime(tc.startTime)
			prs[0].Status.StartTime = &start

			d := test.Data{
				PipelineRuns: prs,
				Pipelines:    ps,
				Tasks:        ts,
				TaskRuns:     trs,
			}
			prt := newPipelineRunTest(t, d)
			defer prt.Cancel()

			wantEvents := []string{
				"Warning Failed PipelineRun \"test-pipeline-run-with-timeout\" failed to finish within \"12h0m0s\"",
			}

			// this limit is just enough to set the timeout condition, but not enough for extra metadata.
			etcdRequestSizeLimit := 650
			prt.TestAssets.Clients.Pipeline.PrependReactor("update", "pipelineruns", withEtcdRequestSizeLimit(t, etcdRequestSizeLimit))

			c := prt.TestAssets.Controller
			clients := prt.TestAssets.Clients
			reconcileError := c.Reconciler.Reconcile(prt.TestAssets.Ctx, "foo/test-pipeline-run-with-timeout")
			if tc.wantError != nil {
				if reconcileError == nil {
					t.Fatalf("expected error %q, but got nil", tc.wantError.Error())
				}
				if reconcileError.Error() != tc.wantError.Error() {
					t.Fatalf("Expected error: %s Got: %s", tc.wantError, reconcileError)
				}
				return
			}
			if reconcileError != nil {
				t.Fatalf("Reconcile error: %s", reconcileError)
			}
			prt.Test.Logf("Getting reconciled run")
			reconciledRun, err := clients.Pipeline.TektonV1beta1().PipelineRuns("foo").Get(prt.TestAssets.Ctx, "test-pipeline-run-with-timeout", metav1.GetOptions{})
			if err != nil {
				prt.Test.Fatalf("Somehow had error getting reconciled run out of fake client: %s", err)
			}
			prt.Test.Logf("Getting events")
			// Check generated events match what's expected
			if err := k8sevent.CheckEventsOrdered(prt.Test, prt.TestAssets.Recorder.Events, "test-pipeline-run-with-timeout", wantEvents); err != nil {
				prt.Test.Errorf(err.Error())
			}

			// The PipelineRun should be timed out.
			if reconciledRun.Status.GetCondition(apis.ConditionSucceeded).Reason != "PipelineRunTimeout" {
				t.Errorf("Expected PipelineRun to be timed out, but condition reason is %s", reconciledRun.Status.GetCondition(apis.ConditionSucceeded))
			}
		})
	}
}

// withEtcdRequestSizeLimit calculates the yaml marshal of the payload and gives an `etcdserver: request too large` when
// the limit is reached
func withEtcdRequestSizeLimit(t *testing.T, limitBytes int) ktesting.ReactionFunc {
	t.Helper()
	return func(action ktesting.Action) (handled bool, ret runtime.Object, err error) {
		obj := action.(ktesting.UpdateAction).GetObject()
		bytes, err := yaml.Marshal(obj)
		if err != nil {
			t.Fatalf("returned a unserializable status: %+v", obj)
		}

		if len(bytes) > limitBytes {
			t.Logf("request size: %d\nrequest limit: %d\n", len(bytes), limitBytes)
			t.Logf("payload:\n%s\n", string(bytes))
			return true, nil, errors.New("etcdserver: request too large")
		}
		return false, nil, nil
	}
}

func TestReconcileWithTimeouts_Tasks(t *testing.T) {
	// TestReconcileWithTimeouts_Tasks runs "Reconcile" on a PipelineRun with timeouts.tasks configured.
	// It verifies that reconcile is successful, no TaskRun is created, the PipelineTask is marked as skipped, and the
	// pipeline status updated and events generated.
	ps := []*v1.Pipeline{parse.MustParseV1Pipeline(t, `
metadata:
  name: test-pipeline
  namespace: foo
spec:
  tasks:
  - name: hello-world-1
    taskRef:
      name: hello-world
  - name: hello-world-2
    taskRef:
      name: hello-world
`)}
	prs := []*v1.PipelineRun{parse.MustParseV1PipelineRun(t, `
metadata:
  name: test-pipeline-run-with-timeout
  namespace: foo
spec:
  pipelineRef:
    name: test-pipeline
  taskRunTemplate:
    serviceAccountName: test-sa
  timeouts:
    tasks: 2m
status:
  startTime: "2021-12-31T23:55:00Z"
  childReferences:
  - name: test-pipeline-run-with-timeout-hello-world-1
    pipelineTaskName: hello-world-1
    kind: TaskRun
`)}
	ts := []*v1.Task{simpleHelloWorldTask}

	trs := []*v1.TaskRun{mustParseTaskRunWithObjectMeta(t, taskRunObjectMeta("test-pipeline-run-with-timeout-hello-world-1", "foo", "test-pipeline-run-with-timeout",
		"test-pipeline", "hello-world-1", false), `
spec:
  serviceAccountName: test-sa
  taskRef:
    name: hello-world
    kind: Task
status:
  conditions:
  - lastTransitionTime: null
    status: "Unknown"
    type: Succeeded
`)}

	d := test.Data{
		PipelineRuns: prs,
		Pipelines:    ps,
		Tasks:        ts,
		TaskRuns:     trs,
	}
	prt := newPipelineRunTest(t, d)
	defer prt.Cancel()

	wantEvents := []string{
		"Normal Started",
	}
	reconciledRun, clients := prt.reconcileRun("foo", "test-pipeline-run-with-timeout", wantEvents, false)

	if reconciledRun.Status.CompletionTime != nil {
		t.Errorf("Expected nil CompletionTime on PipelineRun but was %s", reconciledRun.Status.CompletionTime)
	}

	// The PipelineRun should be running.
	if reconciledRun.Status.GetCondition(apis.ConditionSucceeded).Reason != v1.PipelineRunReasonRunning.String() {
		t.Errorf("Expected PipelineRun to be running, but condition reason is %s", reconciledRun.Status.GetCondition(apis.ConditionSucceeded).Reason)
	}

	// Check that there is a skipped task for the expected reason
	if len(reconciledRun.Status.SkippedTasks) != 1 {
		t.Errorf("expected one skipped task, found %d", len(reconciledRun.Status.SkippedTasks))
	} else if reconciledRun.Status.SkippedTasks[0].Reason != v1.TasksTimedOutSkip {
		t.Errorf("expected skipped reason to be '%s', but was '%s", v1.TasksTimedOutSkip, reconciledRun.Status.SkippedTasks[0].Reason)
	}

	updatedTaskRun, err := clients.Pipeline.TektonV1().TaskRuns("foo").Get(context.Background(), trs[0].Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("error getting updated TaskRun: %#v", err)
	}

	if updatedTaskRun.Spec.Status != v1.TaskRunSpecStatusCancelled {
		t.Errorf("expected existing TaskRun Spec.Status to be set to %s, but was %s", v1.TaskRunSpecStatusCancelled, updatedTaskRun.Spec.Status)
	}
	if updatedTaskRun.Spec.StatusMessage != v1.TaskRunCancelledByPipelineTimeoutMsg {
		t.Errorf("expected existing TaskRun Spec.StatusMessage to be set to %s, but was %s", v1.TaskRunCancelledByPipelineTimeoutMsg, updatedTaskRun.Spec.StatusMessage)
	}
}

func TestReconcileWithTimeouts_Finally(t *testing.T) {
	// TestReconcileWithTimeouts_Finally runs "Reconcile" on a PipelineRun with timeouts.finally configured.
	// It verifies that reconcile is successful, no TaskRun is created, the PipelineTask is marked as skipped, and the
	// pipeline status updated and events generated.
	ps := []*v1.Pipeline{parse.MustParseV1Pipeline(t, `
metadata:
  name: test-pipeline-with-finally
  namespace: foo
spec:
  finally:
  - name: finaltask-1
    taskRef:
      name: hello-world
  - name: finaltask-2
    taskRef:
      name: hello-world
  tasks:
  - name: task1
    taskRef:
      name: hello-world
`)}
	prs := []*v1.PipelineRun{parse.MustParseV1PipelineRun(t, `
metadata:
  name: test-pipeline-run-with-timeout
  namespace: foo
spec:
  pipelineRef:
    name: test-pipeline-with-finally
  taskRunTemplate:
    serviceAccountName: test-sa
  timeouts:
    finally: 15m
status:
  finallyStartTime: "2021-12-31T23:44:59Z"
  startTime: "2021-12-31T23:40:00Z"
  childReferences:
  - name: test-pipeline-run-with-timeout-hello-world
    apiVersion: tekton.dev/v1
    kind: TaskRun
    pipelineTaskName: task1
    status:
      conditions:
      - lastTransitionTime: null
        status: "True"
        type: Succeeded
  - name: test-pipeline-run-with-timeout-finaltask-1
    apiVersion: tekton.dev/v1
    kind: TaskRun
    pipelineTaskName: finaltask-1
    status:
      conditions:
      - lastTransitionTime: null
        status: "Unknown"
        type: Succeeded
`)}
	ts := []*v1.Task{simpleHelloWorldTask}
	trs := []*v1.TaskRun{
		getTaskRun(
			t,
			"test-pipeline-run-with-timeout-hello-world",
			prs[0].Name,
			ps[0].Name,
			"hello-world",
			corev1.ConditionTrue,
		),
		mustParseTaskRunWithObjectMeta(t, taskRunObjectMeta("test-pipeline-run-with-timeout-finaltask-1", "foo", "test-pipeline-run-with-timeout",
			"test-pipeline", "finaltask-1", false), `
spec:
  serviceAccountName: test-sa
  taskRef:
    name: hello-world
    kind: Task
`)}

	d := test.Data{
		PipelineRuns: prs,
		Pipelines:    ps,
		Tasks:        ts,
		TaskRuns:     trs,
	}
	prt := newPipelineRunTest(t, d)
	defer prt.Cancel()

	wantEvents := []string{
		"Normal Started",
	}
	reconciledRun, clients := prt.reconcileRun("foo", "test-pipeline-run-with-timeout", wantEvents, false)

	if reconciledRun.Status.CompletionTime != nil {
		t.Errorf("Expected a nil CompletionTime on running PipelineRun but was %s", reconciledRun.Status.CompletionTime.String())
	}

	// The PipelineRun should be running.
	if reconciledRun.Status.GetCondition(apis.ConditionSucceeded).Reason != v1.PipelineRunReasonRunning.String() {
		t.Errorf("Expected PipelineRun to be running, but condition reason is %s", reconciledRun.Status.GetCondition(apis.ConditionSucceeded).Reason)
	}

	// Check that there is a skipped task for the expected reason
	if len(reconciledRun.Status.SkippedTasks) != 1 {
		t.Errorf("expected one skipped task, found %d", len(reconciledRun.Status.SkippedTasks))
	} else if reconciledRun.Status.SkippedTasks[0].Reason != v1.FinallyTimedOutSkip {
		t.Errorf("expected skipped reason to be '%s', but was '%s", v1.FinallyTimedOutSkip, reconciledRun.Status.SkippedTasks[0].Reason)
	}

	updatedTaskRun, err := clients.Pipeline.TektonV1().TaskRuns("foo").Get(context.Background(), trs[1].Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("error getting updated TaskRun: %#v", err)
	}

	if updatedTaskRun.Spec.Status != v1.TaskRunSpecStatusCancelled {
		t.Errorf("expected existing TaskRun Spec.Status to be set to %s, but was %s", v1.TaskRunSpecStatusCancelled, updatedTaskRun.Spec.Status)
	}
	if updatedTaskRun.Spec.StatusMessage != v1.TaskRunCancelledByPipelineTimeoutMsg {
		t.Errorf("expected existing TaskRun Spec.StatusMessage to be set to %s, but was %s", v1.TaskRunCancelledByPipelineTimeoutMsg, updatedTaskRun.Spec.StatusMessage)
	}
}

func TestReconcileWithoutPVC(t *testing.T) {
	// TestReconcileWithoutPVC runs "Reconcile" on a PipelineRun that has two unrelated tasks.
	// It verifies that reconcile is successful and that no PVC is created
	ps := []*v1.Pipeline{parse.MustParseV1Pipeline(t, `
metadata:
  name: test-pipeline
  namespace: foo
spec:
  tasks:
  - name: hello-world-1
    taskRef:
      name: hello-world
  - name: hello-world-2
    taskRef:
      name: hello-world
`)}

	prs := []*v1.PipelineRun{parse.MustParseV1PipelineRun(t, `
metadata:
  name: test-pipeline-run
  namespace: foo
spec:
  pipelineRef:
    name: test-pipeline
`)}
	ts := []*v1.Task{simpleHelloWorldTask}

	d := test.Data{
		PipelineRuns: prs,
		Pipelines:    ps,
		Tasks:        ts,
	}
	prt := newPipelineRunTest(t, d)
	defer prt.Cancel()

	reconciledRun, clients := prt.reconcileRun("foo", "test-pipeline-run", []string{}, false)
	actions := clients.Pipeline.Actions()

	// Check that the expected TaskRun was created
	for _, a := range actions {
		if ca, ok := a.(ktesting.CreateAction); ok {
			obj := ca.GetObject()
			if pvc, ok := obj.(*corev1.PersistentVolumeClaim); ok {
				t.Errorf("Did not expect to see a PVC created when no resources are linked. %s was created", pvc)
			}
		}
	}

	if !reconciledRun.Status.GetCondition(apis.ConditionSucceeded).IsUnknown() {
		t.Errorf("Expected PipelineRun to be running, but condition status is %s", reconciledRun.Status.GetCondition(apis.ConditionSucceeded))
	}
}

func TestReconcileCancelledFailsTaskRunCancellation(t *testing.T) {
	prName := "test-pipeline-fails-to-cancel"

	testCases := []struct {
		name       string
		specStatus v1.PipelineRunSpecStatus
	}{
		{
			name:       "cancelled",
			specStatus: v1.PipelineRunSpecStatusCancelled,
		},
		{
			name:       "cancelled run finally",
			specStatus: v1.PipelineRunSpecStatusCancelledRunFinally,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// TestReconcileCancelledFailsTaskRunCancellation runs "Reconcile" on a PipelineRun with a single TaskRun.
			// The TaskRun cannot be cancelled. Check that the pipelinerun cancel fails, that reconcile fails and
			// an event is generated
			names.TestingSeed()
			prs := []*v1.PipelineRun{parse.MustParseV1PipelineRun(t, fmt.Sprintf(`
metadata:
  name: test-pipeline-fails-to-cancel
  namespace: foo
spec:
  pipelineRef:
    name: test-pipeline
  status: %s
status:
  conditions:
  - message: running...
    reason: Running
    status: Unknown
    type: Succeeded
  startTime: "2022-01-01T00:00:00Z"
  childReferences:
  - apiVersion: tekton.dev/v1
    kind: TaskRun
    name: test-pipeline-fails-to-cancelhello-world-1
    pipelineTaskName: hello-world-1
`, tc.specStatus))}
			ps := []*v1.Pipeline{parse.MustParseV1Pipeline(t, `
metadata:
  name: test-pipeline
  namespace: foo
spec:
  tasks:
  - name: hello-world-1
    taskRef:
      name: hello-world
  - name: hello-world-2
    taskRef:
      name: hello-world
`)}
			tasks := []*v1.Task{simpleHelloWorldTask}
			taskRuns := []*v1.TaskRun{
				getTaskRun(
					t,
					"test-pipeline-fails-to-cancelhello-world-1",
					prName,
					"test-pipeline",
					"hello-world",
					corev1.ConditionUnknown,
				),
			}

			cms := []*corev1.ConfigMap{withEnabledAlphaAPIFields(newFeatureFlagsConfigMap())}

			d := test.Data{
				PipelineRuns: prs,
				Pipelines:    ps,
				Tasks:        tasks,
				TaskRuns:     taskRuns,
				ConfigMaps:   cms,
			}

			testAssets, cancel := getPipelineRunController(t, d)
			defer cancel()
			c := testAssets.Controller
			clients := testAssets.Clients
			failingReactorActivated := true

			// Make the patch call fail, i.e. make it so that the controller fails to cancel the TaskRun
			clients.Pipeline.PrependReactor("patch", "taskruns", func(action ktesting.Action) (bool, runtime.Object, error) {
				return failingReactorActivated, nil, fmt.Errorf("i'm sorry Dave, i'm afraid i can't do that")
			})

			err := c.Reconciler.Reconcile(testAssets.Ctx, "foo/test-pipeline-fails-to-cancel")
			if err == nil {
				t.Errorf("Expected to see error returned from reconcile after failing to cancel TaskRun but saw none!")
			}

			// Check that the PipelineRun is still running with correct error message
			reconciledRun, err := clients.Pipeline.TektonV1().PipelineRuns("foo").Get(testAssets.Ctx, "test-pipeline-fails-to-cancel", metav1.GetOptions{})
			if err != nil {
				t.Fatalf("Somehow had error getting reconciled run out of fake client: %s", err)
			}

			if val, ok := reconciledRun.GetLabels()[pipeline.PipelineLabelKey]; !ok {
				t.Fatalf("expected pipeline label")
			} else if d := cmp.Diff("test-pipeline", val); d != "" {
				t.Errorf("expected to see pipeline label. Diff %s", diff.PrintWantGot(d))
			}

			// The PipelineRun should not be cancelled b/c we couldn't cancel the TaskRun
			checkPipelineRunConditionStatusAndReason(t, reconciledRun, corev1.ConditionUnknown, ReasonCouldntCancel)
			// The event here is "Normal" because in case we fail to cancel we leave the condition to unknown
			// Further reconcile might converge then the status of the pipeline.
			// See https://github.com/tektoncd/pipeline/issues/2647 for further details.
			wantEvents := []string{
				"Normal PipelineRunCouldntCancel PipelineRun \"test-pipeline-fails-to-cancel\" was cancelled but had errors trying to cancel TaskRuns",
				"Warning InternalError 1 error occurred",
			}
			err = k8sevent.CheckEventsOrdered(t, testAssets.Recorder.Events, prName, wantEvents)
			if err != nil {
				t.Errorf(err.Error())
			}

			// Turn off failing reactor and retry reconciliation
			failingReactorActivated = false

			err = c.Reconciler.Reconcile(testAssets.Ctx, "foo/test-pipeline-fails-to-cancel")
			if err == nil {
				// No error is ok
			} else if ok, _ := controller.IsRequeueKey(err); !ok { // Requeue is also fine.
				t.Errorf("Expected to cancel TaskRun successfully!")
			}
		})
	}
}

func TestReconcileFailsTaskRunTimeOut(t *testing.T) {
	prName := "test-pipeline-fails-to-timeout"

	// TestReconcileFailsTaskRunTimeOut runs "Reconcile" on a PipelineRun with a single TaskRun.
	// The TaskRun cannot be timed out. Check that the pipelinerun timeout fails, that reconcile fails and
	// an event is generated
	names.TestingSeed()
	prs := []*v1.PipelineRun{parse.MustParseV1PipelineRun(t, `
metadata:
  name: test-pipeline-fails-to-timeout
  namespace: foo
spec:
  pipelineRef:
    name: test-pipeline
  timeout: 1h0m0s
status:
  conditions:
  - message: running...
    reason: Running
    status: Unknown
    type: Succeeded
  startTime: "2021-12-31T22:59:00Z"
  childReferences:
  - apiVersion: tekton.dev/v1
    kind: TaskRun
    name: test-pipeline-fails-to-timeouthello-world-1
    pipelineTaskName: hello-world-1
`)}
	ps := []*v1.Pipeline{parse.MustParseV1Pipeline(t, `
metadata:
  name: test-pipeline
  namespace: foo
spec:
  tasks:
  - name: hello-world-1
    taskRef:
      name: hello-world
  - name: hello-world-2
    taskRef:
      name: hello-world
`)}
	tasks := []*v1.Task{simpleHelloWorldTask}
	taskRuns := []*v1.TaskRun{
		getTaskRun(
			t,
			"test-pipeline-fails-to-timeouthello-world-1",
			prName,
			"test-pipeline",
			"hello-world",
			corev1.ConditionUnknown,
		),
	}

	cms := []*corev1.ConfigMap{withEnabledAlphaAPIFields(newFeatureFlagsConfigMap())}

	d := test.Data{
		PipelineRuns: prs,
		Pipelines:    ps,
		Tasks:        tasks,
		TaskRuns:     taskRuns,
		ConfigMaps:   cms,
	}

	testAssets, cancel := getPipelineRunController(t, d)
	defer cancel()
	c := testAssets.Controller
	clients := testAssets.Clients
	failingReactorActivated := true

	// Make the patch call fail, i.e. make it so that the controller fails to cancel the TaskRun
	clients.Pipeline.PrependReactor("patch", "taskruns", func(action ktesting.Action) (bool, runtime.Object, error) {
		return failingReactorActivated, nil, fmt.Errorf("i'm sorry Dave, i'm afraid i can't do that")
	})

	err := c.Reconciler.Reconcile(testAssets.Ctx, "foo/test-pipeline-fails-to-timeout")
	if err == nil {
		t.Errorf("Expected to see error returned from reconcile after failing to timeout TaskRun but saw none!")
	}

	// Check that the PipelineRun is still running with correct error message
	reconciledRun, err := clients.Pipeline.TektonV1().PipelineRuns("foo").Get(testAssets.Ctx, "test-pipeline-fails-to-timeout", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Somehow had error getting reconciled run out of fake client: %s", err)
	}

	if val, ok := reconciledRun.GetLabels()[pipeline.PipelineLabelKey]; !ok {
		t.Fatalf("expected pipeline label")
	} else if d := cmp.Diff("test-pipeline", val); d != "" {
		t.Errorf("expected to see pipeline label. Diff %s", diff.PrintWantGot(d))
	}

	// The PipelineRun should not be timed out b/c we couldn't timeout the TaskRun
	checkPipelineRunConditionStatusAndReason(t, reconciledRun, corev1.ConditionUnknown, ReasonCouldntTimeOut)
	// The event here is "Normal" because in case we fail to timeout we leave the condition to unknown
	// Further reconcile might converge then the status of the pipeline.
	// See https://github.com/tektoncd/pipeline/issues/2647 for further details.
	wantEvents := []string{
		"Normal PipelineRunCouldntTimeOut PipelineRun \"test-pipeline-fails-to-timeout\" was timed out but had errors trying to time out TaskRuns and/or Runs",
		"Warning InternalError 1 error occurred",
	}
	err = k8sevent.CheckEventsOrdered(t, testAssets.Recorder.Events, prName, wantEvents)
	if err != nil {
		t.Errorf(err.Error())
	}

	// Turn off failing reactor and retry reconciliation
	failingReactorActivated = false

	err = c.Reconciler.Reconcile(testAssets.Ctx, "foo/test-pipeline-fails-to-timeout")
	if err == nil {
		// No error is ok
	} else if ok, _ := controller.IsRequeueKey(err); !ok { // Requeue is also fine.
		t.Errorf("Expected to timeout TaskRun successfully!")
	}
}

func TestReconcilePropagateLabelsAndAnnotations(t *testing.T) {
	names.TestingSeed()

	namespace := "foo"
	prName := "test-pipeline-run-with-labels"
	trName := "test-pipeline-run-with-labels-hello-world-1"

	ps := []*v1.Pipeline{simpleHelloWorldPipeline}
	prs := []*v1.PipelineRun{parse.MustParseV1PipelineRun(t, `
metadata:
  annotations:
    PipelineRunAnnotation: PipelineRunValue
  labels:
    PipelineRunLabel: PipelineRunValue
    tekton.dev/pipeline: WillNotBeUsed
  name: test-pipeline-run-with-labels
  namespace: foo
spec:
  pipelineRef:
    name: test-pipeline
  taskRunTemplate:
    serviceAccountName: test-sa
`)}
	ts := []*v1.Task{simpleHelloWorldTask}

	expectedObjectMeta := taskRunObjectMeta(trName, "foo", "test-pipeline-run-with-labels",
		"test-pipeline", "hello-world-1", false)
	expectedObjectMeta.Labels["PipelineRunLabel"] = "PipelineRunValue"
	expectedObjectMeta.Annotations["PipelineRunAnnotation"] = "PipelineRunValue"
	expected := mustParseTaskRunWithObjectMeta(t, expectedObjectMeta, `
spec:
  serviceAccountName: test-sa
  taskRef:
    name: hello-world
    kind: Task
`)

	d := test.Data{
		PipelineRuns: prs,
		Pipelines:    ps,
		Tasks:        ts,
	}
	prt := newPipelineRunTest(t, d)
	defer prt.Cancel()

	_, clients := prt.reconcileRun(namespace, prName, []string{}, false)

	// Check that the expected TaskRun was created
	taskRuns := getTaskRunsForPipelineRun(prt.TestAssets.Ctx, t, clients, namespace, prName)
	validateTaskRunsCount(t, taskRuns, 1)

	actual := getTaskRunByName(t, taskRuns, trName)
	// We're ignoring TypeMeta here because parse.MustParseV1TaskRun populates that, but ktesting does not, so actual does not have it.
	if d := cmp.Diff(expected, actual, ignoreTypeMeta, ignoreResourceVersion); d != "" {
		t.Errorf("expected to see TaskRun %v created. Diff %s", expected, diff.PrintWantGot(d))
	}
}

func TestReconcilePropagateLabelsWithSpecStatus(t *testing.T) {
	testCases := []struct {
		name       string
		specStatus v1.PipelineRunSpecStatus
	}{
		{
			name:       "pending",
			specStatus: v1.PipelineRunSpecStatusPending,
		}, {
			name:       "cancelled",
			specStatus: v1.PipelineRunSpecStatusCancelled,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			names.TestingSeed()

			ps := []*v1.Pipeline{simpleHelloWorldPipeline}
			prs := []*v1.PipelineRun{parse.MustParseV1PipelineRun(t, fmt.Sprintf(`
metadata:
  annotations:
    PipelineRunAnnotation: PipelineRunValue
  labels:
    PipelineRunLabel: PipelineRunValue
    tekton.dev/pipeline: WillNotBeUsed
  name: test-pipeline-run-with-labels
  namespace: foo
spec:
  pipelineRef:
    name: test-pipeline
  taskRunTemplate:
    serviceAccountName: test-sa
  status: %s
`, tc.specStatus))}

			ts := []*v1.Task{simpleHelloWorldTask}

			d := test.Data{
				PipelineRuns: prs,
				Pipelines:    ps,
				Tasks:        ts,
			}
			prt := newPipelineRunTest(t, d)
			defer prt.Cancel()

			_, clients := prt.reconcileRun("foo", "test-pipeline-run-with-labels", []string{}, false)

			reconciledRun, err := clients.Pipeline.TektonV1().PipelineRuns("foo").Get(prt.TestAssets.Ctx, "test-pipeline-run-with-labels", metav1.GetOptions{})
			if err != nil {
				t.Fatalf("unexpected error when updating status: %v", err)
			}

			want := "test-pipeline"
			got := reconciledRun.ObjectMeta.Labels["tekton.dev/pipeline"]
			if d := cmp.Diff(want, got); d != "" {
				t.Errorf("expected to see label %v created. Diff %s", want, diff.PrintWantGot(d))
			}
		})
	}
}

func TestReconcileWithDifferentServiceAccounts(t *testing.T) {
	names.TestingSeed()

	ps := []*v1.Pipeline{parse.MustParseV1Pipeline(t, `
metadata:
  name: test-pipeline
  namespace: foo
spec:
  tasks:
  - name: hello-world-0
    taskRef:
      name: hello-world-task
  - name: hello-world-1
    taskRef:
      name: hello-world-task
`)}
	prs := []*v1.PipelineRun{parse.MustParseV1PipelineRun(t, `
metadata:
  name: test-pipeline-run-different-service-accs
  namespace: foo
spec:
  pipelineRef:
    name: test-pipeline
  taskRunTemplate:
    serviceAccountName: test-sa-0
  taskRunSpecs:
  - serviceAccountName: test-sa-1
    pipelineTaskName: hello-world-1
`)}
	ts := []*v1.Task{parse.MustParseV1Task(t, `
metadata:
  name: hello-world-task
  namespace: foo
`)}

	d := test.Data{
		PipelineRuns: prs,
		Pipelines:    ps,
		Tasks:        ts,
	}
	prt := newPipelineRunTest(t, d)
	defer prt.Cancel()

	_, clients := prt.reconcileRun("foo", "test-pipeline-run-different-service-accs", []string{}, false)

	taskRunNames := []string{"test-pipeline-run-different-service-accs-hello-world-0", "test-pipeline-run-different-service-accs-hello-world-1"}

	expectedTaskRuns := []*v1.TaskRun{
		mustParseTaskRunWithObjectMeta(t,
			taskRunObjectMeta(taskRunNames[0], "foo", "test-pipeline-run-different-service-accs", "test-pipeline", "hello-world-0", false),
			`
spec:
  serviceAccountName: test-sa-0
  taskRef:
    name: hello-world-task
    kind: Task
`),
		mustParseTaskRunWithObjectMeta(t,
			taskRunObjectMeta(taskRunNames[1], "foo", "test-pipeline-run-different-service-accs", "test-pipeline", "hello-world-1", false),
			`
spec:
  serviceAccountName: test-sa-1
  taskRef:
    name: hello-world-task
    kind: Task
`),
	}

	for i := range ps[0].Spec.Tasks {
		// Check that the expected TaskRun was created
		actual, err := clients.Pipeline.TektonV1().TaskRuns("foo").Get(prt.TestAssets.Ctx, taskRunNames[i], metav1.GetOptions{})
		if err != nil {
			t.Fatalf("Expected a TaskRun to be created, but it wasn't: %s", err)
		}
		if d := cmp.Diff(actual, expectedTaskRuns[i], ignoreResourceVersion, ignoreTypeMeta); d != "" {
			t.Errorf("expected to see TaskRun %v created. Diff %s", expectedTaskRuns[i], diff.PrintWantGot(d))
		}
	}
}

func TestReconcileCustomTasksWithDifferentServiceAccounts(t *testing.T) {
	names.TestingSeed()

	ps := []*v1.Pipeline{parse.MustParseV1Pipeline(t, `
metadata:
  name: test-pipeline
  namespace: foo
spec:
  tasks:
  - name: hello-world-0
    taskRef:
      apiVersion: example.dev/v0
      kind: Example
  - name: hello-world-1
    taskRef:
      apiVersion: example.dev/v0
      kind: Example
`)}

	prs := []*v1.PipelineRun{parse.MustParseV1PipelineRun(t, `
metadata:
  name: test-pipeline-run-different-service-accs
  namespace: foo
spec:
  pipelineRef:
    name: test-pipeline
  taskRunTemplate:
    serviceAccountName: test-sa-0
  taskRunSpecs:
  - serviceAccountName: test-sa-1
    pipelineTaskName: hello-world-1
`)}

	cms := []*corev1.ConfigMap{newFeatureFlagsConfigMap()}
	d := test.Data{
		PipelineRuns: prs,
		Pipelines:    ps,
		ConfigMaps:   cms,
	}
	prt := newPipelineRunTest(t, d)
	defer prt.Cancel()

	_, clients := prt.reconcileRun("foo", "test-pipeline-run-different-service-accs", []string{}, false)

	customRunNames := []string{"test-pipeline-run-different-service-accs-hello-world-0", "test-pipeline-run-different-service-accs-hello-world-1"}
	expectedSANames := []string{"test-sa-0", "test-sa-1"}

	for i := range ps[0].Spec.Tasks {
		actual, err := clients.Pipeline.TektonV1beta1().CustomRuns("foo").Get(prt.TestAssets.Ctx, customRunNames[i], metav1.GetOptions{})
		if err != nil {
			t.Errorf("Expected a CustomRun %s to be created but it wasn't: %s", customRunNames[i], err)
			continue
		}
		if actual.Spec.ServiceAccountName != expectedSANames[i] {
			t.Errorf("Expected CustomRun %s to have service account %s but it was %s", customRunNames[i], expectedSANames[i], actual.Spec.ServiceAccountName)
		}
	}
}

// TestReconcileAndPropagateCustomPipelineTaskRunSpec tests that custom PipelineTaskRunSpec declared
// in PipelineRun is propagated to created TaskRuns
func TestReconcileAndPropagateCustomPipelineTaskRunSpec(t *testing.T) {
	names.TestingSeed()

	namespace := "foo"
	prName := "test-pipeline-run"
	trName := "test-pipeline-run-hello-world-1"

	ps := []*v1.Pipeline{simpleHelloWorldPipeline}
	prs := []*v1.PipelineRun{parse.MustParseV1PipelineRun(t, `
metadata:
  annotations:
    PipelineRunAnnotation: PipelineRunValue
  name: test-pipeline-run
  namespace: foo
spec:
  pipelineRef:
    name: test-pipeline
  taskRunTemplate:
    serviceAccountName: test-sa
  taskRunSpecs:
  - pipelineTaskName: hello-world-1
    sidecarOverrides:
    - name: bar
    stepOverrides:
    - name: foo
    podTemplate:
      nodeSelector:
        workloadtype: tekton
    serviceAccountName: custom-sa
`)}
	ts := []*v1.Task{simpleHelloWorldTask}

	d := test.Data{
		PipelineRuns: prs,
		Pipelines:    ps,
		Tasks:        ts,
	}
	prt := newPipelineRunTest(t, d)
	defer prt.Cancel()

	_, clients := prt.reconcileRun("foo", prName, []string{}, false)

	// Check that the expected TaskRun was created
	taskRuns := getTaskRunsForPipelineRun(prt.TestAssets.Ctx, t, clients, namespace, prName)
	validateTaskRunsCount(t, taskRuns, 1)

	actual := getTaskRunByName(t, taskRuns, trName)
	expectedTaskRunObjectMeta := taskRunObjectMeta(trName, namespace, prName, "test-pipeline", "hello-world-1", false)
	expectedTaskRunObjectMeta.Annotations["PipelineRunAnnotation"] = "PipelineRunValue"
	expectedTaskRun := mustParseTaskRunWithObjectMeta(t, expectedTaskRunObjectMeta, `
spec:
  podTemplate:
    nodeSelector:
      workloadtype: tekton
  serviceAccountName: custom-sa
  sidecarOverrides:
  - name: bar
  stepOverrides:
  - name: foo
  taskRef:
    name: hello-world
    kind: Task
`)

	if d := cmp.Diff(expectedTaskRun, actual, ignoreTypeMeta, ignoreResourceVersion); d != "" {
		t.Errorf("expected to see propagated custom ServiceAccountName and PodTemplate in TaskRun %v created. Diff %s", expectedTaskRun, diff.PrintWantGot(d))
	}
}

func TestReconcileCustomTasksWithTaskRunSpec(t *testing.T) {
	names.TestingSeed()
	prName := "test-pipeline-run"
	ps := []*v1.Pipeline{parse.MustParseV1Pipeline(t, `
metadata:
  name: test-pipeline
  namespace: foo
spec:
  tasks:
  - name: hello-world-1
    taskRef:
      apiVersion: example.dev/v0
      kind: Example
`)}

	serviceAccount := "custom-sa"
	prs := []*v1.PipelineRun{parse.MustParseV1PipelineRun(t, `
metadata:
  name: test-pipeline-run
  namespace: foo
spec:
  pipelineRef:
    name: test-pipeline
  taskRunTemplate:
    serviceAccountName: test-sa
  taskRunSpecs:
  - pipelineTaskName: hello-world-1
    podTemplate:
      nodeSelector:
        workloadtype: tekton
    serviceAccountName: custom-sa
`)}

	cms := []*corev1.ConfigMap{newFeatureFlagsConfigMap()}
	d := test.Data{
		PipelineRuns: prs,
		Pipelines:    ps,
		ConfigMaps:   cms,
	}
	prt := newPipelineRunTest(t, d)
	defer prt.Cancel()

	_, clients := prt.reconcileRun("foo", prName, []string{}, false)

	customRunName := "test-pipeline-run-hello-world-1"

	actual, err := clients.Pipeline.TektonV1beta1().CustomRuns("foo").Get(prt.TestAssets.Ctx, customRunName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Expected a customRun %s to be created but it wasn't: %s", customRunName, err)
	}
	if actual.Spec.ServiceAccountName != serviceAccount {
		t.Errorf("Expected customRun %s to have service account %s but it was %s", customRunName, serviceAccount, actual.Spec.ServiceAccountName)
	}
}

func TestReconcileWithWhenExpressionsWithTaskResultsAndParams(t *testing.T) {
	names.TestingSeed()
	ps := []*v1.Pipeline{parse.MustParseV1Pipeline(t, `
metadata:
  name: test-pipeline
  namespace: foo
spec:
  params:
  - name: run
    type: string
  tasks:
  - name: a-task
    taskRef:
      name: a-task
  - name: b-task
    taskRef:
      name: b-task
    when:
    - input: $(tasks.a-task.results.aResult)
      operator: in
      values:
      - aResultValue
    - input: aResultValue
      operator: in
      values:
      - $(tasks.a-task.results.aResult)
    - input: $(params.run)
      operator: in
      values:
      - "yes"
  - name: c-task
    taskRef:
      name: c-task
    when:
    - input: $(tasks.a-task.results.aResult)
      operator: in
      values:
      - missing
    - input: $(params.run)
      operator: notin
      values:
      - "yes"
  - name: d-task
    runAfter:
    - c-task
    taskRef:
      name: d-task
`)}
	prs := []*v1.PipelineRun{parse.MustParseV1PipelineRun(t, `
metadata:
  name: test-pipeline-run-different-service-accs
  namespace: foo
spec:
  params:
  - name: run
    value: "yes"
  pipelineRef:
    name: test-pipeline
  taskRunTemplate:
    serviceAccountName: test-sa-0
`)}
	ts := []*v1.Task{
		{ObjectMeta: baseObjectMeta("a-task", "foo")},
		{ObjectMeta: baseObjectMeta("b-task", "foo")},
		{ObjectMeta: baseObjectMeta("c-task", "foo")},
		{ObjectMeta: baseObjectMeta("d-task", "foo")},
	}
	trs := []*v1.TaskRun{mustParseTaskRunWithObjectMeta(t,
		taskRunObjectMeta("test-pipeline-run-different-service-accs-a-task-xxyyy", "foo", "test-pipeline-run-different-service-accs",
			"test-pipeline", "a-task", true),
		`
spec:
  serviceAccountName: test-sa
  taskRef:
    name: hello-world
  timeout: 1h0m0s
status:
  conditions:
  - status: "True"
    type: Succeeded
  results:
  - name: aResult
    value: aResultValue
`)}

	d := test.Data{
		PipelineRuns: prs,
		Pipelines:    ps,
		Tasks:        ts,
		TaskRuns:     trs,
		ConfigMaps:   []*corev1.ConfigMap{newFeatureFlagsConfigMap()},
	}
	prt := newPipelineRunTest(t, d)
	defer prt.Cancel()

	wantEvents := []string{
		"Normal Started",
		"Normal Running Tasks Completed: 1 \\(Failed: 0, Cancelled 0\\), Incomplete: 2, Skipped: 1",
	}
	pipelineRun, clients := prt.reconcileRun("foo", "test-pipeline-run-different-service-accs", wantEvents, false)

	expectedTaskRunName := "test-pipeline-run-different-service-accs-b-task"
	expectedTaskRun := mustParseTaskRunWithObjectMeta(t,
		taskRunObjectMeta(expectedTaskRunName, "foo", "test-pipeline-run-different-service-accs", "test-pipeline", "b-task", false),
		`
spec:
  serviceAccountName: test-sa-0
  taskRef:
    name: b-task
    kind: Task
`)
	// Check that the expected TaskRun was created
	actual, err := clients.Pipeline.TektonV1().TaskRuns("foo").List(prt.TestAssets.Ctx, metav1.ListOptions{
		LabelSelector: "tekton.dev/pipelineTask=b-task,tekton.dev/pipelineRun=test-pipeline-run-different-service-accs",
		Limit:         1,
	})

	if err != nil {
		t.Fatalf("Failure to list TaskRun's %s", err)
	}
	if len(actual.Items) != 1 {
		t.Fatalf("Expected 1 TaskRuns got %d", len(actual.Items))
	}
	actualTaskRun := actual.Items[0]
	if d := cmp.Diff(expectedTaskRun, &actualTaskRun, ignoreResourceVersion, ignoreTypeMeta); d != "" {
		t.Errorf("expected to see TaskRun %v created. Diff %s", expectedTaskRunName, diff.PrintWantGot(d))
	}

	expectedWhenExpressionsInTaskRun := []v1.WhenExpression{{
		Input:    "aResultValue",
		Operator: "in",
		Values:   []string{"aResultValue"},
	}, {
		Input:    "aResultValue",
		Operator: "in",
		Values:   []string{"aResultValue"},
	}, {
		Input:    "yes",
		Operator: "in",
		Values:   []string{"yes"},
	}}
	verifyTaskRunStatusesWhenExpressions(t, pipelineRun.Status, expectedTaskRunName, expectedWhenExpressionsInTaskRun)

	actualSkippedTasks := pipelineRun.Status.SkippedTasks
	expectedSkippedTasks := []v1.SkippedTask{{
		Name:   "c-task",
		Reason: v1.WhenExpressionsSkip,
		WhenExpressions: v1.WhenExpressions{{
			Input:    "aResultValue",
			Operator: "in",
			Values:   []string{"missing"},
		}, {
			Input:    "yes",
			Operator: "notin",
			Values:   []string{"yes"},
		}},
	}}
	if d := cmp.Diff(actualSkippedTasks, expectedSkippedTasks); d != "" {
		t.Errorf("expected to find Skipped Tasks %v. Diff %s", expectedSkippedTasks, diff.PrintWantGot(d))
	}

	skippedTasks := []string{"c-task"}
	for _, skippedTask := range skippedTasks {
		labelSelector := fmt.Sprintf("tekton.dev/pipelineTask=%s,tekton.dev/pipelineRun=test-pipeline-run-different-service-accs", skippedTask)
		actualSkippedTask, err := clients.Pipeline.TektonV1().TaskRuns("foo").List(prt.TestAssets.Ctx, metav1.ListOptions{
			LabelSelector: labelSelector,
			Limit:         1,
		})
		if err != nil {
			t.Fatalf("Failure to list TaskRun's %s", err)
		}
		if len(actualSkippedTask.Items) != 0 {
			t.Fatalf("Expected 0 TaskRuns got %d", len(actualSkippedTask.Items))
		}
	}
}

func TestReconcileWithWhenExpressions(t *testing.T) {
	//		(b)
	//		/
	//	(a) ———— (c) ———— (d)
	//		\
	//		(e) ———— (f)
	names.TestingSeed()
	ps := []*v1.Pipeline{parse.MustParseV1Pipeline(t, `
metadata:
  name: test-pipeline
  namespace: foo
spec:
  tasks:
# a-task is skipped because its when expressions evaluate to false
  - name: a-task
    taskRef:
      name: a-task
    when:
    - input: foo
      operator: in
      values:
      - bar
# b-task is executed regardless of running after skipped a-task because when expressions are scoped to task
  - name: b-task
    runAfter:
    - a-task
    taskRef:
      name: b-task
# c-task is skipped because its when expressions evaluate to false (not because it's parent a-task is skipped)
  - name: c-task
    runAfter:
    - a-task
    taskRef:
      name: c-task
    when:
    - input: foo
      operator: in
      values:
      - bar
# d-task is executed regardless of running after skipped parent c-task (and skipped grandparent a-task)
# because when expressions are scoped to task
  - name: d-task
    runAfter:
    - c-task
    taskRef:
      name: d-task
# e-task is attempted regardless of running after skipped a-task because when expressions are scoped to task
# but then get skipped because of missing result references from a-task
  - name: e-task
    taskRef:
      name: e-task
    when:
    - input: $(tasks.a-task.results.aResult)
      operator: in
      values:
      - aResultValue
# f-task is skipped because its parent task e-task is skipped because of missing result reference from a-task
  - name: f-task
    runAfter:
    - e-task
    taskRef:
      name: f-task
`)}
	// initialize the pipelinerun with the skipped a-task
	prs := []*v1.PipelineRun{parse.MustParseV1PipelineRun(t, `
metadata:
  name: test-pipeline-run-different-service-accs
  namespace: foo
spec:
  pipelineRef:
    name: test-pipeline
  taskRunTemplate:
    serviceAccountName: test-sa-0
status:
  skippedTasks:
  - name: a-task
    whenExpressions:
    - input: foo
      operator: in
      values:
      - bar
`)}
	// initialize the tasks used in the pipeline
	ts := []*v1.Task{
		parse.MustParseV1Task(t, `
metadata:
  name: a-task
  namespace: foo
spec:
  results:
  - description: a result
    name: aResult
`),
		{ObjectMeta: baseObjectMeta("b-task", "foo")},
		{ObjectMeta: baseObjectMeta("c-task", "foo")},
		{ObjectMeta: baseObjectMeta("d-task", "foo")},
		{ObjectMeta: baseObjectMeta("e-task", "foo")},
		{ObjectMeta: baseObjectMeta("f-task", "foo")},
	}

	d := test.Data{
		PipelineRuns: prs,
		Pipelines:    ps,
		Tasks:        ts,
	}
	prt := newPipelineRunTest(t, d)
	defer prt.Cancel()

	wantEvents := []string{
		"Normal Started",
		"Normal Running Tasks Completed: 0 \\(Failed: 0, Cancelled 0\\), Incomplete: 2, Skipped: 4",
	}
	pipelineRun, clients := prt.reconcileRun("foo", "test-pipeline-run-different-service-accs", wantEvents, false)

	taskRunExists := func(taskName string, taskRunName string) {
		expectedTaskRun := mustParseTaskRunWithObjectMeta(t,
			taskRunObjectMeta(taskRunName, "foo", "test-pipeline-run-different-service-accs",
				"test-pipeline", taskName, false),
			fmt.Sprintf(`
spec:
  serviceAccountName: test-sa-0
  taskRef:
    name: %s
    kind: Task
`, taskName))

		actual, err := clients.Pipeline.TektonV1().TaskRuns("foo").List(prt.TestAssets.Ctx, metav1.ListOptions{
			LabelSelector: fmt.Sprintf("tekton.dev/pipelineTask=%s,tekton.dev/pipelineRun=test-pipeline-run-different-service-accs", taskName),
			Limit:         1,
		})

		if err != nil {
			t.Fatalf("Failure to list TaskRuns %s", err)
		}
		if len(actual.Items) != 1 {
			t.Fatalf("Expected 1 TaskRun got %d", len(actual.Items))
		}
		actualTaskRun := actual.Items[0]
		if d := cmp.Diff(expectedTaskRun, &actualTaskRun, ignoreResourceVersion, ignoreTypeMeta); d != "" {
			t.Errorf("expected to see TaskRun %v created. Diff %s", taskRunName, diff.PrintWantGot(d))
		}
	}

	taskRunExists("b-task", "test-pipeline-run-different-service-accs-b-task")
	taskRunExists("d-task", "test-pipeline-run-different-service-accs-d-task")

	actualSkippedTasks := pipelineRun.Status.SkippedTasks
	expectedSkippedTasks := []v1.SkippedTask{{
		// its when expressions evaluate to false
		Name:   "a-task",
		Reason: v1.WhenExpressionsSkip,
		WhenExpressions: v1.WhenExpressions{{
			Input:    "foo",
			Operator: "in",
			Values:   []string{"bar"},
		}},
	}, {
		// its when expressions evaluate to false
		Name:   "c-task",
		Reason: v1.WhenExpressionsSkip,
		WhenExpressions: v1.WhenExpressions{{
			Input:    "foo",
			Operator: "in",
			Values:   []string{"bar"},
		}},
	}, {
		// was attempted, but has missing results references
		Name:   "e-task",
		Reason: v1.MissingResultsSkip,
		WhenExpressions: v1.WhenExpressions{{
			Input:    "$(tasks.a-task.results.aResult)",
			Operator: "in",
			Values:   []string{"aResultValue"},
		}},
	}, {
		Name:   "f-task",
		Reason: v1.ParentTasksSkip,
	}}
	if d := cmp.Diff(expectedSkippedTasks, actualSkippedTasks); d != "" {
		t.Errorf("expected to find Skipped Tasks %v. Diff %s", expectedSkippedTasks, diff.PrintWantGot(d))
	}

	// confirm that there are no taskruns created for the skipped tasks
	skippedTasks := []string{"a-task", "c-task", "e-task", "f-task"}
	for _, skippedTask := range skippedTasks {
		labelSelector := fmt.Sprintf("tekton.dev/pipelineTask=%s,tekton.dev/pipelineRun=test-pipeline-run-different-service-accs", skippedTask)
		actualSkippedTask, err := clients.Pipeline.TektonV1().TaskRuns("foo").List(prt.TestAssets.Ctx, metav1.ListOptions{
			LabelSelector: labelSelector,
			Limit:         1,
		})
		if err != nil {
			t.Fatalf("Failure to list TaskRun's %s", err)
		}
		if len(actualSkippedTask.Items) != 0 {
			t.Fatalf("Expected 0 TaskRuns got %d", len(actualSkippedTask.Items))
		}
	}
}

func TestReconcileWithWhenExpressionsWithResultRefs(t *testing.T) {
	names.TestingSeed()
	ps := []*v1.Pipeline{parse.MustParseV1Pipeline(t, `
metadata:
  name: test-pipeline
  namespace: foo
spec:
  tasks:
# a-task is executed and produces a result aResult with value aResultValue
  - name: a-task
    taskRef:
      name: a-task
# b-task is skipped because it has when expressions, with result reference to a-task, that evaluate to false
  - name: b-task
    taskRef:
      name: b-task
    when:
    - input: $(tasks.a-task.results.aResult)
      operator: in
      values:
      - notResultValue
# c-task is executed regardless of running after skipped b-task because when expressions are scoped to task
  - name: c-task
    runAfter:
    - b-task
    taskRef:
      name: c-task
`)}
	prs := []*v1.PipelineRun{parse.MustParseV1PipelineRun(t, `
metadata:
  name: test-pipeline-run-different-service-accs
  namespace: foo
spec:
  pipelineRef:
    name: test-pipeline
  taskRunTemplate:
    serviceAccountName: test-sa-0
`)}
	ts := []*v1.Task{
		parse.MustParseV1Task(t, `
metadata:
  name: a-task
  namespace: foo
spec:
  results:
  - description: a result
    name: aResult
`),
		{ObjectMeta: baseObjectMeta("b-task", "foo")},
		{ObjectMeta: baseObjectMeta("c-task", "foo")},
	}
	trs := []*v1.TaskRun{mustParseTaskRunWithObjectMeta(t,
		taskRunObjectMeta("test-pipeline-run-different-service-accs-a-task-xxyyy", "foo",
			"test-pipeline-run-different-service-accs", "test-pipeline", "a-task",
			true),
		`
spec:
  serviceAccountName: test-sa
  taskRef:
    name: hello-world
  timeout: 1h0m0s
status:
  conditions:
  - status: "True"
    type: Succeeded
  results:
  - name: aResult
    value: aResultValue
`)}

	d := test.Data{
		PipelineRuns: prs,
		Pipelines:    ps,
		Tasks:        ts,
		TaskRuns:     trs,
	}
	prt := newPipelineRunTest(t, d)
	defer prt.Cancel()

	wantEvents := []string{
		"Normal Started",
		"Normal Running Tasks Completed: 1 \\(Failed: 0, Cancelled 0\\), Incomplete: 1, Skipped: 1",
	}
	pipelineRun, clients := prt.reconcileRun("foo", "test-pipeline-run-different-service-accs", wantEvents, false)

	actual, err := clients.Pipeline.TektonV1().TaskRuns("foo").List(prt.TestAssets.Ctx, metav1.ListOptions{
		LabelSelector: "tekton.dev/pipelineTask=c-task,tekton.dev/pipelineRun=test-pipeline-run-different-service-accs",
		Limit:         1,
	})

	if err != nil {
		t.Fatalf("Failure to list TaskRuns %s", err)
	}
	if len(actual.Items) != 1 {
		t.Fatalf("Expected 1 TaskRun got %d", len(actual.Items))
	}

	actualSkippedTasks := pipelineRun.Status.SkippedTasks
	expectedSkippedTasks := []v1.SkippedTask{{
		// its when expressions evaluate to false
		Name:   "b-task",
		Reason: v1.WhenExpressionsSkip,
		WhenExpressions: v1.WhenExpressions{{
			Input:    "aResultValue",
			Operator: "in",
			Values:   []string{"notResultValue"},
		}},
	}}
	if d := cmp.Diff(expectedSkippedTasks, actualSkippedTasks); d != "" {
		t.Errorf("expected to find Skipped Tasks %v. Diff %s", expectedSkippedTasks, diff.PrintWantGot(d))
	}

	// confirm that there are no taskruns created for the skipped tasks
	skippedTasks := []string{"b-task"}
	for _, skippedTask := range skippedTasks {
		labelSelector := fmt.Sprintf("tekton.dev/pipelineTask=%s,tekton.dev/pipelineRun=test-pipeline-run-different-service-accs", skippedTask)
		actualSkippedTask, err := clients.Pipeline.TektonV1().TaskRuns("foo").List(prt.TestAssets.Ctx, metav1.ListOptions{
			LabelSelector: labelSelector,
			Limit:         1,
		})
		if err != nil {
			t.Fatalf("Failure to list TaskRun's %s", err)
		}
		if len(actualSkippedTask.Items) != 0 {
			t.Fatalf("Expected 0 TaskRuns got %d", len(actualSkippedTask.Items))
		}
	}
}

// TestReconcileWithAffinityAssistantStatefulSet tests that given a pipelineRun with workspaces,
// an Affinity Assistant StatefulSet is created for each PVC workspace and
// that the Affinity Assistant names is propagated to TaskRuns.
func TestReconcileWithAffinityAssistantStatefulSet(t *testing.T) {
	workspaceName := "ws1"
	workspaceName2 := "ws2"
	pipelineRunName := "test-pipeline-run"
	ps := []*v1.Pipeline{parse.MustParseV1Pipeline(t, `
metadata:
  name: test-pipeline
  namespace: foo
spec:
  tasks:
  - name: hello-world-1
    taskRef:
      name: hello-world
    workspaces:
    - name: taskWorkspaceName
      workspace: ws1
  - name: hello-world-2
    taskRef:
      name: hello-world
    workspaces:
    - name: taskWorkspaceName
      workspace: ws2
  - name: hello-world-3
    taskRef:
      name: hello-world
    workspaces:
    - name: taskWorkspaceName
      workspace: emptyDirWorkspace
  workspaces:
  - name: ws1
  - name: ws2
  - name: emptyDirWorkspace
`)}

	prs := []*v1.PipelineRun{parse.MustParseV1PipelineRun(t, `
metadata:
  name: test-pipeline-run
  namespace: foo
spec:
  pipelineRef:
    name: test-pipeline
  workspaces:
  - name: ws1
    volumeClaimTemplate:
      metadata:
        creationTimestamp: null
        name: myclaim
  - name: ws2
    volumeClaimTemplate:
      metadata:
        creationTimestamp: null
        name: myclaim2
  - emptyDir: {}
    name: emptyDirWorkspace
`)}
	ts := []*v1.Task{simpleHelloWorldTask}

	d := test.Data{
		PipelineRuns: prs,
		Pipelines:    ps,
		Tasks:        ts,
	}
	prt := newPipelineRunTest(t, d)
	defer prt.Cancel()

	reconciledRun, clients := prt.reconcileRun("foo", pipelineRunName, []string{}, false)

	// Check that the expected StatefulSet was created
	stsNames := make([]string, 0)
	for _, a := range clients.Kube.Actions() {
		if ca, ok := a.(ktesting.CreateAction); ok {
			obj := ca.GetObject()
			if sts, ok := obj.(*appsv1.StatefulSet); ok {
				stsNames = append(stsNames, sts.Name)
			}
		}
	}

	if len(stsNames) != 2 {
		t.Fatalf("expected one StatefulSet created. %d was created", len(stsNames))
	}

	expectedAffinityAssistantName1 := getAffinityAssistantName(workspaceName, pipelineRunName)
	expectedAffinityAssistantName2 := getAffinityAssistantName(workspaceName2, pipelineRunName)
	expectedAffinityAssistantStsNames := make(map[string]bool)
	expectedAffinityAssistantStsNames[expectedAffinityAssistantName1] = true
	expectedAffinityAssistantStsNames[expectedAffinityAssistantName2] = true
	for _, stsName := range stsNames {
		_, found := expectedAffinityAssistantStsNames[stsName]
		if !found {
			t.Errorf("unexpected StatefulSet created, named %s", stsName)
		}
	}

	taskRuns, err := clients.Pipeline.TektonV1().TaskRuns("foo").List(prt.TestAssets.Ctx, metav1.ListOptions{})
	if err != nil {
		t.Fatalf("unexpected error when listing TaskRuns: %v", err)
	}

	if len(taskRuns.Items) != 3 {
		t.Errorf("expected two TaskRuns created. %d was created", len(taskRuns.Items))
	}

	taskRunsWithPropagatedAffinityAssistantName := 0
	for _, tr := range taskRuns.Items {
		for _, ws := range tr.Spec.Workspaces {
			propagatedAffinityAssistantName := tr.Annotations["pipeline.tekton.dev/affinity-assistant"]
			if ws.PersistentVolumeClaim != nil {
				if propagatedAffinityAssistantName != expectedAffinityAssistantName1 && propagatedAffinityAssistantName != expectedAffinityAssistantName2 {
					t.Fatalf("found taskRun with PVC workspace, but with unexpected AffinityAssistantAnnotation value; expected %s or %s, got %s", expectedAffinityAssistantName1, expectedAffinityAssistantName2, propagatedAffinityAssistantName)
				}
				taskRunsWithPropagatedAffinityAssistantName++
			}

			if ws.PersistentVolumeClaim == nil {
				if propagatedAffinityAssistantName != "" {
					t.Fatalf("found taskRun workspace that is not PVC workspace, but with unexpected AffinityAssistantAnnotation; expected NO AffinityAssistantAnnotation, got %s", propagatedAffinityAssistantName)
				}
			}
		}
	}

	if taskRunsWithPropagatedAffinityAssistantName != 2 {
		t.Errorf("expected only one of two TaskRuns to have Affinity Assistant affinity. %d was detected", taskRunsWithPropagatedAffinityAssistantName)
	}

	if !reconciledRun.Status.GetCondition(apis.ConditionSucceeded).IsUnknown() {
		t.Errorf("Expected PipelineRun to be running, but condition status is %s", reconciledRun.Status.GetCondition(apis.ConditionSucceeded))
	}
}

// TestReconcileWithVolumeClaimTemplateWorkspace tests that given a pipeline with volumeClaimTemplate workspace,
// a PVC is created and that the workspace appears as a PersistentVolumeClaim workspace for TaskRuns.
func TestReconcileWithVolumeClaimTemplateWorkspace(t *testing.T) {
	pipelineRunName := "test-pipeline-run"
	ps := []*v1.Pipeline{parse.MustParseV1Pipeline(t, `
metadata:
  name: test-pipeline
  namespace: foo
spec:
  tasks:
  - name: hello-world-1
    taskRef:
      name: hello-world
    workspaces:
    - name: taskWorkspaceName
      workspace: ws1
  - name: hello-world-2
    taskRef:
      name: hello-world
  workspaces:
  - name: ws1
`)}

	prs := []*v1.PipelineRun{parse.MustParseV1PipelineRun(t, `
metadata:
  name: test-pipeline-run
  namespace: foo
spec:
  pipelineRef:
    name: test-pipeline
  workspaces:
  - name: ws1
    volumeClaimTemplate:
      metadata:
        creationTimestamp: null
        name: myclaim
`)}
	ts := []*v1.Task{simpleHelloWorldTask}
	cms := []*corev1.ConfigMap{withoutAffinityAssistant(newFeatureFlagsConfigMap())}

	d := test.Data{
		PipelineRuns: prs,
		Pipelines:    ps,
		Tasks:        ts,
		ConfigMaps:   cms,
	}
	prt := newPipelineRunTest(t, d)
	defer prt.Cancel()

	reconciledRun, clients := prt.reconcileRun("foo", pipelineRunName, []string{}, false)

	// Check that the expected PVC was created
	pvcNames := make([]string, 0)
	for _, a := range clients.Kube.Actions() {
		if ca, ok := a.(ktesting.CreateAction); ok {
			obj := ca.GetObject()
			if pvc, ok := obj.(*corev1.PersistentVolumeClaim); ok {
				pvcNames = append(pvcNames, pvc.Name)
			}
		}
	}

	if len(pvcNames) != 1 {
		t.Errorf("expected one PVC created. %d was created", len(pvcNames))
	}

	for _, pr := range prs {
		for _, w := range pr.Spec.Workspaces {
			expectedPVCName := volumeclaim.GetPVCNameWithoutAffinityAssistant(w.VolumeClaimTemplate.Name, w, *kmeta.NewControllerRef(pr))
			_, err := clients.Kube.CoreV1().PersistentVolumeClaims(pr.Namespace).Get(prt.TestAssets.Ctx, expectedPVCName, metav1.GetOptions{})
			if err != nil {
				t.Fatalf("expected PVC %s to exist but instead got error when getting it: %v", expectedPVCName, err)
			}
		}
	}

	taskRuns, err := clients.Pipeline.TektonV1().TaskRuns("foo").List(prt.TestAssets.Ctx, metav1.ListOptions{})
	if err != nil {
		t.Fatalf("unexpected error when listing TaskRuns: %v", err)
	}

	for _, tr := range taskRuns.Items {
		for _, ws := range tr.Spec.Workspaces {
			if ws.VolumeClaimTemplate != nil {
				t.Fatalf("found volumeClaimTemplate workspace. Did not expect to find any taskruns with volumeClaimTemplate workspaces")
			}

			if ws.PersistentVolumeClaim == nil {
				t.Fatalf("found taskRun workspace that is not PersistentVolumeClaim workspace. Did only expect PersistentVolumeClaims workspaces")
			}
		}
	}

	if !reconciledRun.Status.GetCondition(apis.ConditionSucceeded).IsUnknown() {
		t.Errorf("Expected PipelineRun to be running, but condition status is %s", reconciledRun.Status.GetCondition(apis.ConditionSucceeded))
	}
}

// TestReconcileWithVolumeClaimTemplateWorkspaceUsingSubPaths tests that given a pipeline with volumeClaimTemplate workspace and
// multiple instances of the same task, but using different subPaths in the volume - is seen as taskRuns with expected subPaths.
func TestReconcileWithVolumeClaimTemplateWorkspaceUsingSubPaths(t *testing.T) {
	subPath1 := "customdirectory"
	subPath2 := "otherdirecory"
	pipelineRunWsSubPath := "mypath"
	pipelineRunName := "test-pipeline-run"
	ps := []*v1.Pipeline{parse.MustParseV1Pipeline(t, `
metadata:
  name: test-pipeline
  namespace: foo
spec:
  tasks:
  - name: hello-world-1
    taskRef:
      name: hello-world
    workspaces:
    - name: taskWorkspaceName
      subPath: customdirectory
      workspace: ws1
  - name: hello-world-2
    taskRef:
      name: hello-world
    workspaces:
    - name: taskWorkspaceName
      subPath: otherdirecory
      workspace: ws1
  - name: hello-world-3
    taskRef:
      name: hello-world
    workspaces:
    - name: taskWorkspaceName
      workspace: ws1
  - name: hello-world-4
    taskRef:
      name: hello-world
    workspaces:
    - name: taskWorkspaceName
      workspace: ws2
  - name: hello-world-5
    taskRef:
      name: hello-world
    workspaces:
    - name: taskWorkspaceName
      subPath: customdirectory
      workspace: ws2
  - name: hello-world-6
    taskRef:
      name: hello-world
    workspaces:
    - name: taskWorkspaceName
      workspace: ws3
  - name: hello-world-7
    taskRef:
      name: hello-world
    workspaces:
    - name: taskWorkspaceName
      subPath: customdirectory
      workspace: ws3
  workspaces:
  - name: ws1
  - name: ws2
  - name: ws3
`)}

	prs := []*v1.PipelineRun{parse.MustParseV1PipelineRun(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: foo
spec:
  pipelineRef:
    name: test-pipeline
  workspaces:
  - name: ws1
    volumeClaimTemplate:
      metadata:
        creationTimestamp: null
        name: myclaim
  - name: ws2
    subPath: mypath
    volumeClaimTemplate:
      metadata:
        creationTimestamp: null
        name: myclaim
  - name: ws3
    subPath: $(context.pipelineRun.name)
    volumeClaimTemplate:
      metadata:
        creationTimestamp: null
        name: myclaim
`, pipelineRunName))}
	ts := []*v1.Task{simpleHelloWorldTask}

	d := test.Data{
		PipelineRuns: prs,
		Pipelines:    ps,
		Tasks:        ts,
	}
	prt := newPipelineRunTest(t, d)
	defer prt.Cancel()

	reconciledRun, clients := prt.reconcileRun("foo", pipelineRunName, []string{}, false)

	taskRuns, err := clients.Pipeline.TektonV1().TaskRuns("foo").List(prt.TestAssets.Ctx, metav1.ListOptions{})
	if err != nil {
		t.Fatalf("unexpected error when listing TaskRuns: %v", err)
	}

	if len(taskRuns.Items) != 7 {
		t.Fatalf("unexpected number of taskRuns found, expected 7, but found %d", len(taskRuns.Items))
	}

	hasSeenWorkspaceWithPipelineTaskSubPath1 := false
	hasSeenWorkspaceWithPipelineTaskSubPath2 := false
	hasSeenWorkspaceWithEmptyPipelineTaskSubPath := false
	hasSeenWorkspaceWithRunSubPathAndEmptyPipelineTaskSubPath := false
	hasSeenWorkspaceWithRunSubPathAndPipelineTaskSubPath1 := false
	hasSeenWorkspaceWithRunSubPathContextAndEmptyPipelineTaskSubPath := false
	hasSeenWorkspaceWithRunSubPathContextAndPipelineTaskSubPath := false

	for _, tr := range taskRuns.Items {
		for _, ws := range tr.Spec.Workspaces {
			if ws.PersistentVolumeClaim == nil {
				t.Fatalf("found taskRun workspace that is not PersistentVolumeClaim workspace. Did only expect PersistentVolumeClaims workspaces")
			}

			if ws.SubPath == subPath1 {
				hasSeenWorkspaceWithPipelineTaskSubPath1 = true
			}

			if ws.SubPath == subPath2 {
				hasSeenWorkspaceWithPipelineTaskSubPath2 = true
			}

			if ws.SubPath == "" {
				hasSeenWorkspaceWithEmptyPipelineTaskSubPath = true
			}

			if ws.SubPath == pipelineRunWsSubPath {
				hasSeenWorkspaceWithRunSubPathAndEmptyPipelineTaskSubPath = true
			}

			if ws.SubPath == fmt.Sprintf("%s/%s", pipelineRunWsSubPath, subPath1) {
				hasSeenWorkspaceWithRunSubPathAndPipelineTaskSubPath1 = true
			}

			if ws.SubPath == pipelineRunName {
				hasSeenWorkspaceWithRunSubPathContextAndEmptyPipelineTaskSubPath = true
			}
			if ws.SubPath == fmt.Sprintf("%s/%s", pipelineRunName, subPath1) {
				hasSeenWorkspaceWithRunSubPathContextAndPipelineTaskSubPath = true
			}
		}
	}

	if !hasSeenWorkspaceWithPipelineTaskSubPath1 {
		t.Fatalf("did not see a taskRun with a workspace using pipelineTask subPath1")
	}

	if !hasSeenWorkspaceWithPipelineTaskSubPath2 {
		t.Fatalf("did not see a taskRun with a workspace using pipelineTask subPath2")
	}

	if !hasSeenWorkspaceWithEmptyPipelineTaskSubPath {
		t.Fatalf("did not see a taskRun with a workspace using empty pipelineTask subPath")
	}

	if !hasSeenWorkspaceWithRunSubPathAndEmptyPipelineTaskSubPath {
		t.Fatalf("did not see a taskRun with workspace using empty pipelineTask subPath and a subPath from pipelineRun")
	}

	if !hasSeenWorkspaceWithRunSubPathAndPipelineTaskSubPath1 {
		t.Fatalf("did not see a taskRun with workspace using pipelineTaks subPath1 and a subPath from pipelineRun")
	}

	if !hasSeenWorkspaceWithRunSubPathContextAndEmptyPipelineTaskSubPath {
		t.Fatalf("did not see a taskRun with workspace using empty pipelineTaks subPath and a subPath from pipelineRun using context variables")
	}

	if !hasSeenWorkspaceWithRunSubPathContextAndPipelineTaskSubPath {
		t.Fatalf("did not see a taskRun with workspace using pipelineTaks subPath1 and a subPath from pipelineRun using context variables")
	}

	if !reconciledRun.Status.GetCondition(apis.ConditionSucceeded).IsUnknown() {
		t.Errorf("Expected PipelineRun to be running, but condition status is %s", reconciledRun.Status.GetCondition(apis.ConditionSucceeded))
	}
}

func TestReconcileWithTaskResults(t *testing.T) {
	names.TestingSeed()
	ps := []*v1.Pipeline{parse.MustParseV1Pipeline(t, `
metadata:
  name: test-pipeline
  namespace: foo
spec:
  tasks:
  - name: a-task
    taskRef:
      name: a-task
  - name: b-task
    params:
    - name: bParam
      value: $(tasks.a-task.results.aResult)
    taskRef:
      name: b-task
`)}
	prs := []*v1.PipelineRun{parse.MustParseV1PipelineRun(t, `
metadata:
  name: test-pipeline-run-different-service-accs
  namespace: foo
spec:
  pipelineRef:
    name: test-pipeline
  taskRunTemplate:
    serviceAccountName: test-sa-0
`)}
	ts := []*v1.Task{
		parse.MustParseV1Task(t, `
metadata:
  name: a-task
  namespace: foo
spec: {}
`),
		parse.MustParseV1Task(t, `
metadata:
  name: b-task
  namespace: foo
spec:
  params:
  - name: bParam
    type: string
`),
	}
	trs := []*v1.TaskRun{mustParseTaskRunWithObjectMeta(t,
		taskRunObjectMeta("test-pipeline-run-different-service-accs-a-task-xxyyy", "foo",
			"test-pipeline-run-different-service-accs", "test-pipeline", "a-task", true),
		`
spec:
  serviceAccountName: test-sa
  taskRef:
    name: hello-world
  timeout: 1h0m0s
status:
  conditions:
  - lastTransitionTime: null
    status: "True"
    type: Succeeded
  results:
  - name: aResult
    value: aResultValue
`)}

	d := test.Data{
		PipelineRuns: prs,
		Pipelines:    ps,
		Tasks:        ts,
		TaskRuns:     trs,
	}
	prt := newPipelineRunTest(t, d)
	defer prt.Cancel()

	_, clients := prt.reconcileRun("foo", "test-pipeline-run-different-service-accs", []string{}, false)

	expectedTaskRunName := "test-pipeline-run-different-service-accs-b-task"
	expectedTaskRun := mustParseTaskRunWithObjectMeta(t,
		taskRunObjectMeta("test-pipeline-run-different-service-accs-b-task", "foo",
			"test-pipeline-run-different-service-accs", "test-pipeline", "b-task", false),
		`
spec:
  params:
  - name: bParam
    value: aResultValue
  serviceAccountName: test-sa-0
  taskRef:
    name: b-task
    kind: Task
`)
	// Check that the expected TaskRun was created
	actual, err := clients.Pipeline.TektonV1().TaskRuns("foo").List(prt.TestAssets.Ctx, metav1.ListOptions{
		LabelSelector: "tekton.dev/pipelineTask=b-task,tekton.dev/pipelineRun=test-pipeline-run-different-service-accs",
		Limit:         1,
	})

	if err != nil {
		t.Fatalf("Failure to list TaskRun's %s", err)
	}
	if len(actual.Items) != 1 {
		t.Fatalf("Expected 1 TaskRuns got %d", len(actual.Items))
	}
	actualTaskRun := actual.Items[0]
	if d := cmp.Diff(expectedTaskRun, &actualTaskRun, ignoreResourceVersion, ignoreTypeMeta); d != "" {
		t.Errorf("expected to see TaskRun %v created. Diff %s", expectedTaskRunName, diff.PrintWantGot(d))
	}
}

func TestReconcileWithTaskResultsEmbeddedNoneStarted(t *testing.T) {
	names.TestingSeed()
	prs := []*v1.PipelineRun{parse.MustParseV1PipelineRun(t, `
metadata:
  name: test-pipeline-run-different-service-accs
  namespace: foo
spec:
  params:
  - name: foo
    value: bar
  pipelineSpec:
    params:
    - name: foo
      type: string
    tasks:
    - name: a-task
      taskRef:
        name: a-task
    - name: b-task
      params:
      - name: bParam
        value: $(params.foo)/baz@$(tasks.a-task.results.A_RESULT)
      taskRef:
        name: b-task
  taskRunTemplate:
    serviceAccountName: test-sa-0
`)}
	ts := []*v1.Task{
		parse.MustParseV1Task(t, `
metadata:
  name: a-task
  namespace: foo
spec:
  results:
  - name: A_RESULT
`),
		parse.MustParseV1Task(t, `
metadata:
  name: b-task
  namespace: foo
spec:
  params:
  - name: bParam
    type: string
`),
	}

	d := test.Data{
		PipelineRuns: prs,
		Tasks:        ts,
	}
	prt := newPipelineRunTest(t, d)
	defer prt.Cancel()

	reconciledRun, clients := prt.reconcileRun("foo", "test-pipeline-run-different-service-accs", []string{}, false)

	if !reconciledRun.Status.GetCondition(apis.ConditionSucceeded).IsUnknown() {
		t.Errorf("Expected PipelineRun to be running, but condition status is %s", reconciledRun.Status.GetCondition(apis.ConditionSucceeded))
	}

	// Since b-task is dependent on a-task, via the results, only a-task should run
	expectedTaskRun := mustParseTaskRunWithObjectMeta(t,
		taskRunObjectMeta("test-pipeline-run-different-service-accs-a-task", "foo",
			"test-pipeline-run-different-service-accs", "test-pipeline-run-different-service-accs", "a-task", false),
		`
spec:
  serviceAccountName: test-sa-0
  taskRef:
    kind: Task
    name: a-task
`)
	// Check that the expected TaskRun was created (only)
	actual, err := clients.Pipeline.TektonV1().TaskRuns("foo").List(prt.TestAssets.Ctx, metav1.ListOptions{})
	if err != nil {
		t.Fatalf("Failure to list TaskRun's %s", err)
	}
	if len(actual.Items) != 1 {
		t.Fatalf("Expected 1 TaskRuns got %d", len(actual.Items))
	}
	actualTaskRun := actual.Items[0]
	if d := cmp.Diff(expectedTaskRun, &actualTaskRun, ignoreResourceVersion, ignoreTypeMeta); d != "" {
		t.Errorf("expected to see TaskRun %v created. Diff %s", expectedTaskRun, diff.PrintWantGot(d))
	}
}

func TestReconcileWithFinallyResults(t *testing.T) {
	names.TestingSeed()
	ps := []*v1.Pipeline{parse.MustParseV1Pipeline(t, `
metadata:
  name: test-pipeline
  namespace: foo
spec:
  results:
    - description: pipeline result
      name: result
      value: $(finally.a-task.results.a-Result)
    - description: custom task pipeline result
      name: custom-result
      value: $(finally.b-task.results.b-Result)
  tasks:
    - name: c-task
      taskRef:
        name: c-task
  finally:
    - name: a-task
      taskRef:
        name: a-task
    - name: b-task
      taskRef:
        apiVersion: example.dev/v0
        kind: Example
        name: b-task
`)}
	trs := []*v1.TaskRun{mustParseTaskRunWithObjectMeta(t,
		taskRunObjectMeta("test-pipeline-run-finally-results-task-run-a", "foo",
			"test-pipeline-run-finally-results", "test-pipeline", "a-task", true),
		`
spec:
  taskRef:
    name: hello-world
status:
  conditions:
  - status: "True"
    type: Succeeded
  results:
  - name: a-Result
    value: aResultValue
`), mustParseTaskRunWithObjectMeta(t,
		taskRunObjectMeta("test-pipeline-run-finally-results-task-run-c", "foo",
			"test-pipeline-run-finally-results", "test-pipeline", "c-task", true),
		`
spec:
  taskRef:
    name: hello-world
status:
  conditions:
  - status: "True"
    type: Succeeded
`)}
	crs := []*v1beta1.CustomRun{mustParseCustomRunWithObjectMeta(t,
		taskRunObjectMeta("test-pipeline-run-finally-results-task-run-b", "foo",
			"test-pipeline-run-finally-results", "test-pipeline", "b-task", true),
		`
spec:
  customRef:
    apiVersion: example.dev/v0
    kind: Example
status:
  conditions:
  - status: "True"
    type: Succeeded
  results:
  - name: b-Result
    value: bResultValue
`)}
	prs := []*v1.PipelineRun{parse.MustParseV1PipelineRun(t, `
metadata:
  name: test-pipeline-run-finally-results
  namespace: foo
spec:
  pipelineRef:
    name: test-pipeline
status:
  conditions:
  - status: "Unknown"
    type: Succeeded
    reason: Succeeded
`)}
	ts := []*v1.Task{
		parse.MustParseV1Task(t, `
metadata:
  name: a-task
  namespace: foo
spec:
  results:
  - name: a-Result
`),
		parse.MustParseV1Task(t, `
metadata:
  name: c-task
  namespace: foo
spec:
  steps:
  - image: ubuntu
    script: |
      #!/usr/bin/env bash
      echo "Hello from bash!"

`),
	}

	d := test.Data{
		PipelineRuns: prs,
		Pipelines:    ps,
		Tasks:        ts,
		TaskRuns:     trs,
		CustomRuns:   crs,
		ConfigMaps:   []*corev1.ConfigMap{newFeatureFlagsConfigMap()},
	}
	prt := newPipelineRunTest(t, d)
	defer prt.Cancel()
	reconciledRun, _ := prt.reconcileRun("foo", "test-pipeline-run-finally-results", []string{}, false)

	expectedPrStatus := parse.MustParseV1PipelineRun(t, `
metadata:
  name: test-pipeline-run-finally-results
  namespace: foo
  labels:
    tekton.dev/pipeline: test-pipeline
  annotations: {}
spec:
  pipelineRef:
    name: test-pipeline
status:
  pipelineSpec:
    results:
    - description: pipeline result
      name: result
      value: $(finally.a-task.results.a-Result)
    - description: custom task pipeline result
      name: custom-result
      value: $(finally.b-task.results.b-Result)
    tasks:
    - name: c-task
      taskRef:
        name: c-task
        kind: Task
    finally:
    - name: a-task
      taskRef:
        name: a-task
        kind: Task
    - name: b-task
      taskRef:
        name: b-task
        apiVersion: example.dev/v0
        kind: Example
  conditions:
  - status: "True"
    type: Succeeded
    reason: Succeeded
    message: "Tasks Completed: 3 (Failed: 0, Cancelled 0), Skipped: 0"
  results:
  - name: result
    value: aResultValue
  - name: custom-result
    value: bResultValue
  childReferences:
  - apiVersion: tekton.dev/v1
    kind: TaskRun
    name: test-pipeline-run-finally-results-task-run-c
    pipelineTaskName: c-task
  - apiVersion: tekton.dev/v1
    kind: TaskRun
    name: test-pipeline-run-finally-results-task-run-a
    pipelineTaskName: a-task
  - apiVersion: tekton.dev/v1beta1
    kind: CustomRun
    name: test-pipeline-run-finally-results-task-run-b
    pipelineTaskName: b-task
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

	expectedPr := expectedPrStatus

	if d := cmp.Diff(expectedPr, reconciledRun, ignoreResourceVersion, ignoreLastTransitionTime, ignoreCompletionTime, ignoreStartTime, cmpopts.EquateEmpty()); d != "" {
		t.Errorf("expected to see pipeline run results created. Diff %s", diff.PrintWantGot(d))
	}
}

func TestReconcileWithPipelineResults(t *testing.T) {
	names.TestingSeed()
	ps := []*v1.Pipeline{parse.MustParseV1Pipeline(t, `
metadata:
  name: test-pipeline
  namespace: foo
spec:
  results:
    - description: pipeline result
      name: result
      value: $(tasks.a-task.results.a-Result)
    - description: custom task pipeline result
      name: custom-result
      value: $(tasks.b-task.results.b-Result)
  tasks:
  - name: b-task
    taskRef:
      apiVersion: example.dev/v0
      kind: Example
      name: b-task
  - name: a-task
    taskRef:
      name: a-task
`)}
	trs := []*v1.TaskRun{mustParseTaskRunWithObjectMeta(t,
		taskRunObjectMeta("test-pipeline-run-results-task-run-a", "foo",
			"test-pipeline-run-results", "test-pipeline", "a-task", true),
		`
spec:
  taskRef:
    name: hello-world
status:
  conditions:
  - status: "True"
    type: Succeeded
  results:
  - name: a-Result
    value: aResultValue
`)}
	rs := []*v1beta1.CustomRun{mustParseCustomRunWithObjectMeta(t,
		taskRunObjectMeta("test-pipeline-run-results-task-run-b", "foo",
			"test-pipeline-run-results", "test-pipeline", "b-task", true),
		`
spec:
  customRef:
    apiVersion: example.dev/v0
    kind: Example
status:
  conditions:
  - status: "True"
    type: Succeeded
  results:
  - name: b-Result
    value: bResultValue
`)}
	prs := []*v1.PipelineRun{parse.MustParseV1PipelineRun(t, `
metadata:
  name: test-pipeline-run-results
  namespace: foo
spec:
  pipelineRef:
    name: test-pipeline
status:
  conditions:
  - status: "Unknown"
    type: Succeeded
`)}
	ts := []*v1.Task{
		parse.MustParseV1Task(t, `
metadata:
  name: a-task
  namespace: foo
spec:
  results:
  - name: a-Result
`),
	}

	d := test.Data{
		PipelineRuns: prs,
		Pipelines:    ps,
		Tasks:        ts,
		TaskRuns:     trs,
		CustomRuns:   rs,
		ConfigMaps:   []*corev1.ConfigMap{newFeatureFlagsConfigMap()},
	}

	prt := newPipelineRunTest(t, d)
	defer prt.Cancel()
	reconciledRun, _ := prt.reconcileRun("foo", "test-pipeline-run-results", []string{}, false)

	expectedPrStatus := parse.MustParseV1PipelineRun(t, `
metadata:
  name: test-pipeline-run-results
  namespace: foo
  labels:
    tekton.dev/pipeline: test-pipeline
  annotations: {}
spec:
  pipelineRef:
    name: test-pipeline
status:
  pipelineSpec:
    results:
    - description: pipeline result
      name: result
      value: $(tasks.a-task.results.a-Result)
    - description: custom task pipeline result
      name: custom-result
      value: $(tasks.b-task.results.b-Result)
    tasks:
    - name: b-task
      taskRef:
        name: b-task
        apiVersion: example.dev/v0
        kind: Example
    - name: a-task
      taskRef:
        name: a-task
        kind: Task
  conditions:
  - status: "True"
    type: Succeeded
    reason: Succeeded
    message: "Tasks Completed: 2 (Failed: 0, Cancelled 0), Skipped: 0"
  results:
  - name: result
    value: aResultValue
  - name: custom-result
    value: bResultValue
  childReferences:
  - apiVersion: tekton.dev/v1beta1
    kind: CustomRun
    name: test-pipeline-run-results-task-run-b
    pipelineTaskName: b-task
  - apiVersion: tekton.dev/v1
    kind: TaskRun
    name: test-pipeline-run-results-task-run-a
    pipelineTaskName: a-task
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

	expectedPr := expectedPrStatus

	if d := cmp.Diff(expectedPr, reconciledRun, ignoreResourceVersion, ignoreLastTransitionTime, ignoreCompletionTime, ignoreStartTime, cmpopts.EquateEmpty()); d != "" {
		t.Errorf("expected to see pipeline run results created. Diff %s", diff.PrintWantGot(d))
	}
}

func TestReconcileWithPipelineResults_OnFailedPipelineRun(t *testing.T) {
	names.TestingSeed()
	ps := []*v1.Pipeline{parse.MustParseV1Pipeline(t, `
metadata:
  name: test-pipeline
  namespace: foo
spec:
  results:
  - description: pipeline result
    name: result
    value: $(tasks.a-task.results.aResult)
  tasks:
  - name: a-task
    taskRef:
      name: a-task
  - name: b-task
    params:
    - name: bParam
      value: $(tasks.a-task.results.aResult)
    taskRef:
      name: b-task
`)}
	trs := []*v1.TaskRun{mustParseTaskRunWithObjectMeta(t,
		taskRunObjectMeta("test-failed-pr-with-task-results-a-task", "foo",
			"test-failed-pr-with-task-results", "test-pipeline", "a-task", true),
		`
spec:
  serviceAccountName: test-sa
  taskRef:
    name: hello-world
  timeout: 1h0m0s
status:
  conditions:
  - status: "True"
    type: Succeeded
  results:
  - name: aResult
    value: aResultValue
`), mustParseTaskRunWithObjectMeta(t,
		taskRunObjectMeta("test-failed-pr-with-task-results-b-task", "foo",
			"test-failed-pr-with-task-results", "test-pipeline", "b-task", true),
		`
spec:
  serviceAccountName: test-sa
  taskRef:
     name: hello-world
  timeout: 1h0m0s
status:
  conditions:
  - status: "False"
    type: Succeeded
  results:
  - name: bResult
    value: bResultValue
`)}
	prs := []*v1.PipelineRun{parse.MustParseV1PipelineRun(t, `
metadata:
  name: test-failed-pr-with-task-results
  namespace: foo
spec:
  pipelineRef:
    name: test-pipeline
  taskRunTemplate:
    serviceAccountName: test-sa-0
  timeouts:
    pipeline: "0"
status:
  conditions:
  - message: Message
    reason: Running
    status: "Unknown"
    type: Succeeded
  startTime: "2021-12-31T00:00:00Z"
`)}
	ts := []*v1.Task{
		{ObjectMeta: baseObjectMeta("a-task", "foo")},
		parse.MustParseV1Task(t, `
metadata:
  name: b-task
  namespace: foo
spec:
  params:
  - name: bParam
    type: string
`),
	}
	wantPrs := []*v1.PipelineRun{parse.MustParseV1PipelineRun(t, `
metadata:
  name: test-failed-pr-with-task-results
  namespace: foo
spec:
  pipelineRef:
    name: test-pipeline
  taskRunTemplate:
    serviceAccountName: test-sa-0
  timeouts:
    pipeline: "0"
status:
  conditions:
  - message: "Tasks Completed: 2 (Failed: 1, Cancelled 0), Skipped: 0"
    reason: Failed
    status: "False"
    type: Succeeded
  results:
    - name: result
      value: aResultValue
`)}

	d := test.Data{
		PipelineRuns: prs,
		Pipelines:    ps,
		Tasks:        ts,
		TaskRuns:     trs,
		ConfigMaps:   []*corev1.ConfigMap{newFeatureFlagsConfigMap()},
	}
	prt := newPipelineRunTest(t, d)
	defer prt.Cancel()

	reconciledRun, _ := prt.reconcileRun("foo", "test-failed-pr-with-task-results", []string{}, false)

	if d := cmp.Diff(reconciledRun.Status.Conditions, wantPrs[0].Status.Conditions, ignoreResourceVersion, ignoreLastTransitionTime); d != "" {
		t.Errorf("expected to see pipeline run marked as failed. Diff %s", diff.PrintWantGot(d))
	}
	if d := cmp.Diff(reconciledRun.Status.Results, wantPrs[0].Status.Results, ignoreResourceVersion, ignoreLastTransitionTime); d != "" {
		t.Errorf("expected to see pipeline run results created. Diff %s", diff.PrintWantGot(d))
	}
}

func Test_storePipelineSpecAndRefSource(t *testing.T) {
	pr := parse.MustParseV1PipelineRun(t, `
metadata:
  name: test-pipeline-run-success
  labels:
    lbl: value
  annotations:
    io.annotation: value
`)
	refSource := &v1.RefSource{
		URI: "abc.com",
		Digest: map[string]string{
			"sha1": "a123",
		},
		EntryPoint: "foo/bar",
	}

	ps := v1.PipelineSpec{Description: "foo-pipeline"}
	ps1 := v1.PipelineSpec{Description: "bar-pipeline"}

	want := pr.DeepCopy()
	want.Status = v1.PipelineRunStatus{
		PipelineRunStatusFields: v1.PipelineRunStatusFields{
			PipelineSpec: ps.DeepCopy(),
			Provenance: &v1.Provenance{
				RefSource:    refSource.DeepCopy(),
				FeatureFlags: config.DefaultFeatureFlags.DeepCopy(),
			},
		},
	}
	want.ObjectMeta.Labels["tekton.dev/pipeline"] = pr.ObjectMeta.Name

	type args struct {
		pipelineSpec       *v1.PipelineSpec
		resolvedObjectMeta *resolutionutil.ResolvedObjectMeta
	}

	var tests = []struct {
		name            string
		reconcile1Args  *args
		reconcile2Args  *args
		wantPipelineRun *v1.PipelineRun
	}{
		{
			name: "spec and source are available in the same reconcile",
			reconcile1Args: &args{
				pipelineSpec: &ps,
				resolvedObjectMeta: &resolutionutil.ResolvedObjectMeta{
					ObjectMeta: &pr.ObjectMeta,
					RefSource:  refSource.DeepCopy(),
				},
			},
			reconcile2Args: &args{
				pipelineSpec:       &ps1,
				resolvedObjectMeta: &resolutionutil.ResolvedObjectMeta{},
			},
			wantPipelineRun: want,
		},
		{
			name: "spec comes in the first reconcile and source comes in next reconcile",
			reconcile1Args: &args{
				pipelineSpec: &ps,
				resolvedObjectMeta: &resolutionutil.ResolvedObjectMeta{
					ObjectMeta: &pr.ObjectMeta,
				},
			},
			reconcile2Args: &args{
				pipelineSpec: &ps,
				resolvedObjectMeta: &resolutionutil.ResolvedObjectMeta{
					RefSource: refSource.DeepCopy(),
				},
			},
			wantPipelineRun: want,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// mock first reconcile
			if err := storePipelineSpecAndMergeMeta(context.Background(), pr, tc.reconcile1Args.pipelineSpec, tc.reconcile1Args.resolvedObjectMeta); err != nil {
				t.Errorf("storePipelineSpec() error = %v", err)
			}
			if d := cmp.Diff(pr, tc.wantPipelineRun); d != "" {
				t.Fatalf(diff.PrintWantGot(d))
			}

			// mock second reconcile
			if err := storePipelineSpecAndMergeMeta(context.Background(), pr, tc.reconcile2Args.pipelineSpec, tc.reconcile2Args.resolvedObjectMeta); err != nil {
				t.Errorf("storePipelineSpec() error = %v", err)
			}
			if d := cmp.Diff(pr, tc.wantPipelineRun); d != "" {
				t.Fatalf(diff.PrintWantGot(d))
			}
		})
	}
}

func Test_storePipelineSpec_metadata(t *testing.T) {
	pipelinerunlabels := map[string]string{"lbl1": "value1", "lbl2": "value2"}
	pipelinerunannotations := map[string]string{"io.annotation.1": "value1", "io.annotation.2": "value2"}
	pipelinelabels := map[string]string{"lbl1": "another value", "lbl2": "another value", "lbl3": "value3"}
	pipelineannotations := map[string]string{"io.annotation.1": "another value", "io.annotation.2": "another value", "io.annotation.3": "value3", "kubectl.kubernetes.io/last-applied-configuration": "foo-is-bar"}
	wantedlabels := map[string]string{"lbl1": "value1", "lbl2": "value2", "lbl3": "value3", pipeline.PipelineLabelKey: "bar"}
	wantedannotations := map[string]string{"io.annotation.1": "value1", "io.annotation.2": "value2", "io.annotation.3": "value3"}

	pr := &v1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{Name: "foo", Labels: pipelinerunlabels, Annotations: pipelinerunannotations},
	}
	meta := metav1.ObjectMeta{Name: "bar", Labels: pipelinelabels, Annotations: pipelineannotations}
	if err := storePipelineSpecAndMergeMeta(context.Background(), pr, &v1.PipelineSpec{}, &resolutionutil.ResolvedObjectMeta{
		ObjectMeta: &meta,
	}); err != nil {
		t.Errorf("storePipelineSpecAndMergeMeta error = %v", err)
	}
	if d := cmp.Diff(pr.ObjectMeta.Labels, wantedlabels); d != "" {
		t.Fatalf(diff.PrintWantGot(d))
	}
	if d := cmp.Diff(pr.ObjectMeta.Annotations, wantedannotations); d != "" {
		t.Fatalf(diff.PrintWantGot(d))
	}
}

func TestReconcileOutOfSyncPipelineRun(t *testing.T) {
	// It may happen that a PipelineRun creates one or more TaskRuns during reconcile
	// but it fails to sync the update on the status back. This test verifies that
	// the reconciler is able to coverge back to a consistent state with the orphaned
	// TaskRuns back in the PipelineRun status.
	// For more details, see https://github.com/tektoncd/pipeline/issues/2558
	ctx := context.Background()

	namespace := "foo"
	prOutOfSyncName := "test-pipeline-run-out-of-sync"
	helloWorldTask := simpleHelloWorldTask

	testPipeline := parse.MustParseV1Pipeline(t, `
metadata:
  name: test-pipeline
  namespace: foo
spec:
  tasks:
  - name: hello-world-1
    taskRef:
      name: hello-world
  - name: hello-world-2
    taskRef:
      name: hello-world
  - name: hello-world-5
    taskRef:
      apiVersion: example.dev/v0
      kind: Example
`)

	// This taskrun is in the pipelinerun status. It completed successfully.
	taskRunDone := mustParseTaskRunWithObjectMeta(t,
		taskRunObjectMeta("test-pipeline-run-out-of-sync-hello-world-1", "foo", prOutOfSyncName, testPipeline.Name, "hello-world-1", false),
		`
spec:
  taskRef:
    name: hello-world
status:
  conditions:
  - status: "True"
    type: Succeeded
`)

	// This taskrun is *not* in the pipelinerun status. It's still running.
	taskRunOrphaned := mustParseTaskRunWithObjectMeta(t,
		taskRunObjectMeta("test-pipeline-run-out-of-sync-hello-world-2", "foo", prOutOfSyncName, testPipeline.Name, "hello-world-2", false),
		`
spec:
  taskRef:
    name: hello-world
status:
  conditions:
  - status: Unknown
    type: Succeeded
`)

	orphanedCustomRun := mustParseCustomRunWithObjectMeta(t,
		taskRunObjectMeta("test-pipeline-run-out-of-sync-hello-world-5", "foo", prOutOfSyncName, testPipeline.Name,
			"hello-world-5", true),
		`
spec:
  customRef:
    apiVersion: example.dev/v0
    kind: Example
status:
  conditions:
  - status: Unknown
    type: Succeeded
`)

	prOutOfSync := parse.MustParseV1PipelineRun(t, `
metadata:
  name: test-pipeline-run-out-of-sync
  namespace: foo
spec:
  pipelineRef:
    name: test-pipeline
  serviceAccountName: test-sa
status:
  conditions:
  - status: Unknown
    type: Succeeded
`)

	prOutOfSync.Status.ChildReferences = []v1.ChildStatusReference{
		{
			TypeMeta: runtime.TypeMeta{
				APIVersion: v1.SchemeGroupVersion.String(),
				Kind:       "TaskRun",
			},
			Name:             taskRunDone.Name,
			PipelineTaskName: "hello-world-1",
		},
	}

	prs := []*v1.PipelineRun{prOutOfSync}
	ps := []*v1.Pipeline{testPipeline}
	ts := []*v1.Task{helloWorldTask}
	trs := []*v1.TaskRun{taskRunDone, taskRunOrphaned}
	customRuns := []*v1beta1.CustomRun{orphanedCustomRun}

	cms := []*corev1.ConfigMap{newFeatureFlagsConfigMap()}

	d := test.Data{
		PipelineRuns: prs,
		Pipelines:    ps,
		Tasks:        ts,
		TaskRuns:     trs,
		CustomRuns:   customRuns,
		ConfigMaps:   cms,
	}
	prt := newPipelineRunTest(t, d)
	defer prt.Cancel()

	reconciledRun, clients := prt.reconcileRun(namespace, prOutOfSyncName, []string{}, false)
	taskRuns := getTaskRunsForPipelineRun(prt.TestAssets.Ctx, t, clients, namespace, prOutOfSyncName)
	validateTaskRunsCount(t, taskRuns, 2)

	// This PipelineRun should still be running and the status should reflect that
	if !reconciledRun.Status.GetCondition(apis.ConditionSucceeded).IsUnknown() {
		t.Errorf("Expected PipelineRun status to be running, but was %v", reconciledRun.Status.GetCondition(apis.ConditionSucceeded))
	}

	expectedTaskRunsStatus := make(map[string]*v1.PipelineRunTaskRunStatus)
	expectedRunsStatus := make(map[string]*v1.PipelineRunRunStatus)
	// taskRunDone did not change
	expectedTaskRunsStatus[taskRunDone.Name] = &v1.PipelineRunTaskRunStatus{
		PipelineTaskName: "hello-world-1",
		Status: &v1.TaskRunStatus{
			Status: duckv1.Status{
				Conditions: []apis.Condition{
					{
						Type:   apis.ConditionSucceeded,
						Status: corev1.ConditionTrue,
					},
				},
			},
		},
	}
	// taskRunOrphaned was recovered into the status
	expectedTaskRunsStatus[taskRunOrphaned.Name] = &v1.PipelineRunTaskRunStatus{
		PipelineTaskName: "hello-world-2",
		Status: &v1.TaskRunStatus{
			Status: duckv1.Status{
				Conditions: []apis.Condition{
					{
						Type:   apis.ConditionSucceeded,
						Status: corev1.ConditionUnknown,
					},
				},
			},
		},
	}
	// orphanedCustomRun was recovered into the status
	expectedRunsStatus[orphanedCustomRun.Name] = &v1.PipelineRunRunStatus{
		PipelineTaskName: "hello-world-5",
		Status: &v1beta1.CustomRunStatus{
			Status: duckv1.Status{
				Conditions: []apis.Condition{
					{
						Type:   apis.ConditionSucceeded,
						Status: corev1.ConditionUnknown,
					},
				},
			},
		},
	}

	taskRunsStatus := make(map[string]*v1.PipelineRunTaskRunStatus)
	runsStatus := make(map[string]*v1.PipelineRunRunStatus)

	for _, cr := range reconciledRun.Status.ChildReferences {
		if cr.Kind == taskRun {
			trStatusForPipelineRun := &v1.PipelineRunTaskRunStatus{
				PipelineTaskName: cr.PipelineTaskName,
				WhenExpressions:  cr.WhenExpressions,
			}

			tr, _ := clients.Pipeline.TektonV1().TaskRuns("foo").Get(ctx, cr.Name, metav1.GetOptions{})
			if tr != nil {
				trStatusForPipelineRun.Status = &tr.Status
			}

			taskRunsStatus[cr.Name] = trStatusForPipelineRun
		} else if cr.Kind == "CustomRun" {
			rStatusForPipelineRun := &v1.PipelineRunRunStatus{
				PipelineTaskName: cr.PipelineTaskName,
				WhenExpressions:  cr.WhenExpressions,
			}

			r, _ := clients.Pipeline.TektonV1beta1().CustomRuns("foo").Get(ctx, cr.Name, metav1.GetOptions{})
			rStatusForPipelineRun.Status = &r.Status

			runsStatus[cr.Name] = rStatusForPipelineRun
		}
	}
	if d := cmp.Diff(expectedTaskRunsStatus, taskRunsStatus); d != "" {
		t.Fatalf("Expected PipelineRun status to match TaskRun(s) status, but got a mismatch: %s", d)
	}
	if d := cmp.Diff(expectedRunsStatus, runsStatus); d != "" {
		t.Fatalf("Expected PipelineRun status to match Run(s) status, but got a mismatch: %s", d)
	}
}

func TestUpdatePipelineRunStatusFromInformer(t *testing.T) {
	names.TestingSeed()

	pr := parse.MustParseV1PipelineRun(t, `
metadata:
  labels:
    mylabel: myvale
  name: test-pipeline-run
  namespace: foo
spec:
  pipelineSpec:
    tasks:
    - name: unit-test-task-spec
      taskSpec:
        steps:
        - image: myimage
          name: mystep
    - name: custom-task-ref
      taskRef:
        apiVersion: example.dev/v0
        kind: Example
        name: some-custom-task
`)

	cms := []*corev1.ConfigMap{newFeatureFlagsConfigMap()}

	d := test.Data{
		PipelineRuns: []*v1.PipelineRun{pr},
		ConfigMaps:   cms,
	}
	prt := newPipelineRunTest(t, d)
	defer prt.Cancel()

	wantEvents := []string{
		"Normal Started",
		"Normal Running Tasks Completed: 0",
	}

	// Reconcile the PipelineRun.  This creates a Taskrun.
	reconciledRun, clients := prt.reconcileRun("foo", "test-pipeline-run", wantEvents, false)

	// Save the name of the TaskRun and CustomRun that were created.
	taskRunName := ""
	customRunName := ""

	if len(reconciledRun.Status.ChildReferences) != 2 {
		t.Fatalf("Expected 2 ChildReferences but got %d", len(reconciledRun.Status.ChildReferences))
	}
	for _, cr := range reconciledRun.Status.ChildReferences {
		if cr.Kind == taskRun {
			taskRunName = cr.Name
		}
		if cr.Kind == customRun {
			customRunName = cr.Name
		}
	}

	if taskRunName == "" {
		t.Fatal("expected to find a TaskRun name, but didn't")
	}
	if customRunName == "" {
		t.Fatal("expected to find a Run name, but didn't")
	}

	// Add a label to the PipelineRun.  This tests a scenario in issue 3126 which could prevent the reconciler
	// from finding TaskRuns that are missing from the status.
	reconciledRun.ObjectMeta.Labels["bah"] = "humbug"
	reconciledRun, err := clients.Pipeline.TektonV1().PipelineRuns("foo").Update(prt.TestAssets.Ctx, reconciledRun, metav1.UpdateOptions{})
	if err != nil {
		t.Fatalf("unexpected error when updating status: %v", err)
	}

	// The label update triggers another reconcile.  Depending on timing, the PipelineRun passed to the reconcile may or may not
	// have the updated status with the name of the created TaskRun.  Clear the status because we want to test the case where the
	// status does not have the TaskRun.
	reconciledRun.Status = v1.PipelineRunStatus{}
	if _, err := clients.Pipeline.TektonV1().PipelineRuns("foo").UpdateStatus(prt.TestAssets.Ctx, reconciledRun, metav1.UpdateOptions{}); err != nil {
		t.Fatalf("unexpected error when updating status: %v", err)
	}

	reconciledRun, _ = prt.reconcileRun("foo", "test-pipeline-run", wantEvents, false)

	// Verify that the reconciler found the existing TaskRun and Run instead of creating new ones.
	verifyTaskRunStatusesCount(t, reconciledRun.Status, 1)
	verifyTaskRunStatusesNames(t, reconciledRun.Status, taskRunName)
	verifyCustomRunOrRunStatusesCount(t, customRun, reconciledRun.Status, 1)
	verifyCustomRunOrRunStatusesNames(t, customRun, reconciledRun.Status, customRunName)
}

func TestReconcilePipeline_FinalTasks(t *testing.T) {
	tests := []struct {
		name                     string
		pipelineRunName          string
		prs                      []*v1.PipelineRun
		ps                       []*v1.Pipeline
		ts                       []*v1.Task
		trs                      []*v1.TaskRun
		expectedTaskRuns         map[string]*v1.PipelineRunTaskRunStatus
		expectedChildReferences  []v1.ChildStatusReference
		pipelineRunStatusUnknown bool
		pipelineRunStatusFalse   bool
	}{{
		// pipeline run should result in error when a dag task is executed and resulted in failure but final task is executed successfully

		// pipelineRunName - "pipeline-run-dag-task-failing"
		// pipelineName - "pipeline-dag-task-failing"
		// pipelineTasks - "dag-task-1" and "final-task-1"
		// taskRunNames - "task-run-dag-task" and "task-run-final-task"
		// taskName - "hello-world"

		name: "Test 01 - Pipeline run should result in error when a dag task fails but final task is executed successfully.",

		pipelineRunName: "pipeline-run-dag-task-failing",

		prs: getPipelineRun(
			"pipeline-run-dag-task-failing",
			"pipeline-dag-task-failing",
			corev1.ConditionFalse,
			v1.PipelineRunReasonFailed.String(),
			"Tasks Completed: 2 (Failed: 1, Cancelled 0), Skipped: 0",
			map[string]string{
				"dag-task-1":   "task-run-dag-task",
				"final-task-1": "task-run-final-task",
			},
		),

		ps: getPipeline(
			"pipeline-dag-task-failing",
			v1.PipelineSpec{
				Tasks: []v1.PipelineTask{{
					Name:    "dag-task-1",
					TaskRef: &v1.TaskRef{Name: "hello-world"},
				}},
				Finally: []v1.PipelineTask{{
					Name:    "final-task-1",
					TaskRef: &v1.TaskRef{Name: "hello-world"},
				}},
			}),

		ts: []*v1.Task{simpleHelloWorldTask},

		trs: []*v1.TaskRun{
			getTaskRun(
				t,
				"task-run-dag-task",
				"pipeline-run-dag-task-failing",
				"pipeline-dag-task-failing",
				"dag-task-1",
				corev1.ConditionFalse,
			),
			getTaskRun(
				t,
				"task-run-final-task",
				"pipeline-run-dag-task-failing",
				"pipeline-dag-task-failing",
				"final-task-1",
				"",
			),
		},

		expectedTaskRuns: map[string]*v1.PipelineRunTaskRunStatus{
			"task-run-dag-task":   getTaskRunStatus("dag-task-1", corev1.ConditionFalse),
			"task-run-final-task": getTaskRunStatus("final-task-1", ""),
		},

		expectedChildReferences: []v1.ChildStatusReference{{
			TypeMeta: runtime.TypeMeta{
				APIVersion: v1.SchemeGroupVersion.String(),
				Kind:       "TaskRun",
			},
			Name:             "task-run-dag-task",
			PipelineTaskName: "dag-task-1",
		}, {
			TypeMeta: runtime.TypeMeta{
				APIVersion: v1.SchemeGroupVersion.String(),
				Kind:       "TaskRun",
			},
			Name:             "task-run-final-task",
			PipelineTaskName: "final-task-1",
		}},
	}, {
		// pipeline run should result in error when a dag task is successful but the final task fails

		// pipelineRunName - "pipeline-run-with-dag-successful-but-final-failing"
		// pipelineName - "pipeline-with-dag-successful-but-final-failing"
		// pipelineTasks - "dag-task-1" and "final-task-1"
		// taskRunNames - "task-run-dag-task" and "task-run-final-task"
		// taskName - "hello-world"

		name: "Test 02 - Pipeline run should result in error when a dag task is successful but final task fails.",

		pipelineRunName: "pipeline-run-with-dag-successful-but-final-failing",

		prs: getPipelineRun(
			"pipeline-run-with-dag-successful-but-final-failing",
			"pipeline-with-dag-successful-but-final-failing",
			corev1.ConditionFalse,
			v1.PipelineRunReasonFailed.String(),
			"Tasks Completed: 2 (Failed: 1, Cancelled 0), Skipped: 0",
			map[string]string{
				"dag-task-1":   "task-run-dag-task",
				"final-task-1": "task-run-final-task",
			},
		),

		ps: getPipeline(
			"pipeline-with-dag-successful-but-final-failing",
			v1.PipelineSpec{
				Tasks: []v1.PipelineTask{{
					Name:    "dag-task-1",
					TaskRef: &v1.TaskRef{Name: "hello-world"},
				}},
				Finally: []v1.PipelineTask{{
					Name:    "final-task-1",
					TaskRef: &v1.TaskRef{Name: "hello-world"},
				}},
			}),

		ts: []*v1.Task{simpleHelloWorldTask},

		trs: []*v1.TaskRun{
			getTaskRun(
				t,
				"task-run-dag-task",
				"pipeline-run-with-dag-successful-but-final-failing",
				"pipeline-with-dag-successful-but-final-failing",
				"dag-task-1",
				"",
			),
			getTaskRun(
				t,
				"task-run-final-task",
				"pipeline-run-with-dag-successful-but-final-failing",
				"pipeline-with-dag-successful-but-final-failing",
				"final-task-1",
				corev1.ConditionFalse,
			),
		},

		expectedTaskRuns: map[string]*v1.PipelineRunTaskRunStatus{
			"task-run-dag-task":   getTaskRunStatus("dag-task-1", ""),
			"task-run-final-task": getTaskRunStatus("final-task-1", corev1.ConditionFalse),
		},

		expectedChildReferences: []v1.ChildStatusReference{{
			TypeMeta: runtime.TypeMeta{
				APIVersion: v1.SchemeGroupVersion.String(),
				Kind:       "TaskRun",
			},
			Name:             "task-run-dag-task",
			PipelineTaskName: "dag-task-1",
		}, {
			TypeMeta: runtime.TypeMeta{
				APIVersion: v1.SchemeGroupVersion.String(),
				Kind:       "TaskRun",
			},
			Name:             "task-run-final-task",
			PipelineTaskName: "final-task-1",
		}},

		pipelineRunStatusFalse: true,
	}, {
		// pipeline run should result in error when a dag task and final task both are executed and resulted in failure

		// pipelineRunName - "pipeline-run-with-dag-and-final-failing"
		// pipelineName - "pipeline-with-dag-and-final-failing"
		// pipelineTasks - "dag-task-1" and "final-task-1"
		// taskRunNames - "task-run-dag-task" and "task-run-final-task"
		// taskName - "hello-world"

		name: "Test 03 - Pipeline run should result in error when both dag task and final task fail.",

		pipelineRunName: "pipeline-run-with-dag-and-final-failing",

		prs: getPipelineRun(
			"pipeline-run-with-dag-and-final-failing",
			"pipeline-with-dag-and-final-failing",
			corev1.ConditionFalse,
			v1.PipelineRunReasonFailed.String(),
			"Tasks Completed: 2 (Failed: 2, Cancelled 0), Skipped: 0",
			map[string]string{
				"dag-task-1":   "task-run-dag-task",
				"final-task-1": "task-run-final-task",
			},
		),

		ps: getPipeline(
			"pipeline-with-dag-and-final-failing",
			v1.PipelineSpec{
				Tasks: []v1.PipelineTask{{
					Name:    "dag-task-1",
					TaskRef: &v1.TaskRef{Name: "hello-world"},
				}},
				Finally: []v1.PipelineTask{{
					Name:    "final-task-1",
					TaskRef: &v1.TaskRef{Name: "hello-world"},
				}},
			}),

		ts: []*v1.Task{simpleHelloWorldTask},

		trs: []*v1.TaskRun{
			getTaskRun(
				t,
				"task-run-dag-task",
				"pipeline-run-with-dag-and-final-failing",
				"pipeline-with-dag-and-final-failing",
				"dag-task-1",
				corev1.ConditionFalse,
			),
			getTaskRun(
				t,
				"task-run-final-task",
				"pipeline-run-with-dag-and-final-failing",
				"pipeline-with-dag-and-final-failing",
				"final-task-1",
				corev1.ConditionFalse,
			),
		},

		expectedTaskRuns: map[string]*v1.PipelineRunTaskRunStatus{
			"task-run-dag-task":   getTaskRunStatus("dag-task-1", corev1.ConditionFalse),
			"task-run-final-task": getTaskRunStatus("final-task-1", corev1.ConditionFalse),
		},

		expectedChildReferences: []v1.ChildStatusReference{{
			TypeMeta: runtime.TypeMeta{
				APIVersion: v1.SchemeGroupVersion.String(),
				Kind:       "TaskRun",
			},
			Name:             "task-run-dag-task",
			PipelineTaskName: "dag-task-1",
		}, {
			TypeMeta: runtime.TypeMeta{
				APIVersion: v1.SchemeGroupVersion.String(),
				Kind:       "TaskRun",
			},
			Name:             "task-run-final-task",
			PipelineTaskName: "final-task-1",
		}},

		pipelineRunStatusFalse: true,
	}, {
		// pipeline run should not schedule final tasks until dag tasks are done i.e.
		// dag task 1 fails but dag task 2 is still running, pipeline run should not schedule and create task run for final task

		// pipelineRunName - "pipeline-run-with-dag-running"
		// pipelineName - "pipeline-with-dag-running"
		// pipelineTasks - "dag-task-1", "dag-task-2" and "final-task-1"
		// taskRunNames - "task-run-dag-task-1" and "task-run-dag-task-2" - no task run for final task
		// taskName - "hello-world"

		name: "Test 04 - Pipeline run should not schedule final tasks while dag tasks are still running.",

		pipelineRunName: "pipeline-run-with-dag-running",

		prs: getPipelineRun(
			"pipeline-run-with-dag-running",
			"pipeline-with-dag-running",
			corev1.ConditionUnknown,
			v1.PipelineRunReasonRunning.String(),
			"Tasks Completed: 1 (Failed: 1, Cancelled 0), Incomplete: 2, Skipped: 0",
			map[string]string{
				"dag-task-1": "task-run-dag-task-1",
				"dag-task-2": "task-run-dag-task-2",
			},
		),

		ps: getPipeline(
			"pipeline-with-dag-running",
			v1.PipelineSpec{
				Tasks: []v1.PipelineTask{
					{
						Name:    "dag-task-1",
						TaskRef: &v1.TaskRef{Name: "hello-world"},
					},
					{
						Name:    "dag-task-2",
						TaskRef: &v1.TaskRef{Name: "hello-world"},
					},
				},
				Finally: []v1.PipelineTask{{
					Name:    "final-task-1",
					TaskRef: &v1.TaskRef{Name: "hello-world"},
				}},
			}),

		ts: []*v1.Task{simpleHelloWorldTask},

		trs: []*v1.TaskRun{
			getTaskRun(
				t,
				"task-run-dag-task-1",
				"pipeline-run-with-dag-running",
				"pipeline-with-dag-running",
				"dag-task-1",
				corev1.ConditionFalse,
			),
			getTaskRun(
				t,
				"task-run-dag-task-2",
				"pipeline-run-with-dag-running",
				"pipeline-with-dag-running",
				"dag-task-2",
				corev1.ConditionUnknown,
			),
		},

		expectedChildReferences: []v1.ChildStatusReference{{
			TypeMeta: runtime.TypeMeta{
				APIVersion: v1.SchemeGroupVersion.String(),
				Kind:       "TaskRun",
			},
			Name:             "task-run-dag-task-1",
			PipelineTaskName: "dag-task-1",
		}, {
			TypeMeta: runtime.TypeMeta{
				APIVersion: v1.SchemeGroupVersion.String(),
				Kind:       "TaskRun",
			},
			Name:             "task-run-dag-task-2",
			PipelineTaskName: "dag-task-2",
		}},

		expectedTaskRuns: map[string]*v1.PipelineRunTaskRunStatus{
			"task-run-dag-task-1": getTaskRunStatus("dag-task-1", corev1.ConditionFalse),
			"task-run-dag-task-2": getTaskRunStatus("dag-task-2", corev1.ConditionUnknown),
		},

		pipelineRunStatusUnknown: true,
	}, {
		// pipeline run should not schedule final tasks until dag tasks are done i.e.
		// dag task is still running and no other dag task available to schedule,
		// pipeline run should not schedule and create task run for final task

		// pipelineRunName - "pipeline-run-dag-task-running"
		// pipelineName - "pipeline-dag-task-running"
		// pipelineTasks - "dag-task-1" and "final-task-1"
		// taskRunNames - "task-run-dag-task-1" - no task run for final task
		// taskName - "hello-world"

		name: "Test 05 - Pipeline run should not schedule final tasks while dag tasks are still running and no other dag task available to schedule.",

		pipelineRunName: "pipeline-run-dag-task-running",

		prs: getPipelineRun(
			"pipeline-run-dag-task-running",
			"pipeline-dag-task-running",
			corev1.ConditionUnknown,
			v1.PipelineRunReasonRunning.String(),
			"Tasks Completed: 0 (Failed: 0, Cancelled 0), Incomplete: 1, Skipped: 0",
			map[string]string{
				"dag-task-1": "task-run-dag-task-1",
			},
		),

		ps: getPipeline(
			"pipeline-dag-task-running",
			v1.PipelineSpec{
				Tasks: []v1.PipelineTask{{
					Name:    "dag-task-1",
					TaskRef: &v1.TaskRef{Name: "hello-world"},
				}},
				Finally: []v1.PipelineTask{{
					Name:    "final-task-1",
					TaskRef: &v1.TaskRef{Name: "hello-world"},
				}},
			}),

		ts: []*v1.Task{simpleHelloWorldTask},

		trs: []*v1.TaskRun{
			getTaskRun(
				t,
				"task-run-dag-task-1",
				"pipeline-run-dag-task-running",
				"pipeline-dag-task-running",
				"dag-task-1",
				corev1.ConditionUnknown,
			),
		},

		expectedChildReferences: []v1.ChildStatusReference{{
			TypeMeta: runtime.TypeMeta{
				APIVersion: v1.SchemeGroupVersion.String(),
				Kind:       "TaskRun",
			},
			Name:             "task-run-dag-task-1",
			PipelineTaskName: "dag-task-1",
		}},

		expectedTaskRuns: map[string]*v1.PipelineRunTaskRunStatus{
			"task-run-dag-task-1": getTaskRunStatus("dag-task-1", corev1.ConditionUnknown),
		},

		pipelineRunStatusUnknown: true,
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			withOwnerReference(tt.trs, tt.prs[0].Name)

			d := test.Data{
				PipelineRuns: tt.prs,
				Pipelines:    tt.ps,
				Tasks:        tt.ts,
				TaskRuns:     tt.trs,
			}
			namespace := "foo"
			prt := newPipelineRunTest(t, d)
			defer prt.Cancel()

			reconciledRun, clients := prt.reconcileRun(namespace, tt.pipelineRunName, []string{}, false)

			actions := clients.Pipeline.Actions()
			if len(actions) < 2 {
				t.Fatalf("Expected client to have at least two action implementation but it has %d", len(actions))
			}

			// The first update action should be updating the PipelineRun.
			var actual *v1.PipelineRun
			for _, action := range actions {
				if actualPrime, ok := action.(ktesting.UpdateAction); ok {
					actual = actualPrime.GetObject().(*v1.PipelineRun)
					break
				}
			}

			if actual == nil {
				t.Errorf("Expected a PipelineRun to be updated, but it wasn't for %s", tt.name)
			}

			for _, action := range actions {
				if action != nil {
					resource := action.GetResource().Resource
					if resource == "taskruns" {
						t.Fatalf("Expected client to not have created a TaskRun for the PipelineRun, but it did for %s", tt.name)
					}
				}
			}

			if tt.pipelineRunStatusFalse {
				// This PipelineRun should still be failed and the status should reflect that
				if !reconciledRun.Status.GetCondition(apis.ConditionSucceeded).IsFalse() {
					t.Errorf("Expected PipelineRun status to be failed, but was %v for %s",
						reconciledRun.Status.GetCondition(apis.ConditionSucceeded), tt.name)
				}
			} else if tt.pipelineRunStatusUnknown {
				// This PipelineRun should still be running and the status should reflect that
				if !reconciledRun.Status.GetCondition(apis.ConditionSucceeded).IsUnknown() {
					t.Errorf("Expected PipelineRun status to be unknown (running), but was %v for %s",
						reconciledRun.Status.GetCondition(apis.ConditionSucceeded), tt.name)
				}
			}

			if d := cmp.Diff(reconciledRun.Status.ChildReferences, tt.expectedChildReferences, cmpopts.SortSlices(lessChildReferences)); d != "" {
				t.Fatalf("Expected PipelineRunTaskRun status to match ChildReferences status, but got a mismatch for %s: %s", tt.name, d)
			}

			checkTaskRunStatusFromChildRefs(prt.TestAssets.Ctx, t, namespace, clients, reconciledRun.Status.ChildReferences, tt.expectedTaskRuns)
		})
	}
}

// checkTaskRunStatusFromChildRefs checks the status of taskruns from ChildReferences to be expected.
func checkTaskRunStatusFromChildRefs(ctx context.Context, t *testing.T, namespace string, clients test.Clients,
	childRefs []v1.ChildStatusReference, expectedTaskRuns map[string]*v1.PipelineRunTaskRunStatus) {
	t.Helper()
	taskrunsToCheck := len(expectedTaskRuns)
	if taskrunsToCheck == 0 {
		return
	}

	for _, childRef := range childRefs {
		if childRef.Kind != taskRun {
			continue
		}
		trName := childRef.Name

		trFromChildRef, err := clients.Pipeline.TektonV1().TaskRuns(namespace).Get(ctx, trName, metav1.GetOptions{})
		if err != nil {
			t.Fatalf("Failure to get TaskRun from ChildReference %s: %s", childRef.Name, err)
		}

		if expectedTaskRunStatus, ok := expectedTaskRuns[trName]; !ok {
			t.Fatalf("Expected not to have TaskRun status from ChildReferences %s", trName)
		} else if d := cmp.Diff(*expectedTaskRunStatus.Status, trFromChildRef.Status); d != "" {
			t.Fatalf("Expected TaskRun Status to match ChildReferences TaskRun Status, but got a mismatch %s", diff.PrintWantGot(d))
		}
		taskrunsToCheck--
	}

	if taskrunsToCheck != 0 {
		t.Fatalf("Expected ChildReferences to match all the TaskRun Status, but there are TaskRuns that did not: %v", expectedTaskRuns)
	}
}

func getPipelineRun(pr, p string, status corev1.ConditionStatus, reason string, m string, tr map[string]string) []*v1.PipelineRun {
	pRun := &v1.PipelineRun{
		ObjectMeta: baseObjectMeta(pr, "foo"),
		Spec: v1.PipelineRunSpec{
			PipelineRef: &v1.PipelineRef{Name: p},
			TaskRunTemplate: v1.PipelineTaskRunTemplate{
				ServiceAccountName: "test-sa",
			},
		},
		Status: v1.PipelineRunStatus{
			Status: duckv1.Status{
				Conditions: duckv1.Conditions{
					apis.Condition{
						Type:    apis.ConditionSucceeded,
						Status:  status,
						Reason:  reason,
						Message: m,
					},
				},
			},
		},
	}
	for k, v := range tr {
		pRun.Status.ChildReferences = append(pRun.Status.ChildReferences, v1.ChildStatusReference{
			PipelineTaskName: k,
			Name:             v,
			TypeMeta: runtime.TypeMeta{
				Kind:       "TaskRun",
				APIVersion: "tekton.dev/v1",
			},
		})
	}
	return []*v1.PipelineRun{pRun}
}

// withOwnerReference adds the PipelineRun name to each TaskRun as their OwnerReference
// TODO: This shall be removed along with the refactor of `getTaskRun` to populate matched
// OwnerReference with the PipelineRun at https://github.com/tektoncd/pipeline/issues/6008
func withOwnerReference(trs []*v1.TaskRun, prName string) {
	for _, tr := range trs {
		tr.OwnerReferences = []metav1.OwnerReference{{Name: prName}}
	}
}

func getPipeline(p string, spec v1.PipelineSpec) []*v1.Pipeline {
	ps := []*v1.Pipeline{{
		ObjectMeta: baseObjectMeta(p, "foo"),
		Spec:       spec,
	}}
	return ps
}

func getTaskRun(t *testing.T, tr, pr, p, tl string, status corev1.ConditionStatus) *v1.TaskRun {
	t.Helper()
	return createHelloWorldTaskRunWithStatusTaskLabel(t, tr, "foo", pr, p, "", tl,
		apis.Condition{
			Type:   apis.ConditionSucceeded,
			Status: status,
		})
}

func getTaskRunStatus(t string, status corev1.ConditionStatus) *v1.PipelineRunTaskRunStatus {
	return &v1.PipelineRunTaskRunStatus{
		PipelineTaskName: t,
		Status: &v1.TaskRunStatus{
			Status: duckv1.Status{
				Conditions: []apis.Condition{
					{Type: apis.ConditionSucceeded, Status: status},
				},
			},
		},
	}
}

// TestReconcile_CloudEvents runs reconcile with a cloud event sink configured
// to ensure that events are sent in different cases
func TestReconcile_CloudEvents(t *testing.T) {
	names.TestingSeed()

	prs := []*v1.PipelineRun{
		parse.MustParseV1PipelineRun(t, `
metadata:
  name: test-pipelinerun
  namespace: foo
  selfLink: /pipeline/1234
spec:
  pipelineRef:
    name: test-pipeline
`),
	}
	ps := []*v1.Pipeline{
		parse.MustParseV1Pipeline(t, `
metadata:
  name: test-pipeline
  namespace: foo
spec:
  tasks:
    - name: test-1
      taskRef:
        name: test-task
`),
	}
	ts := []*v1.Task{
		parse.MustParseV1Task(t, `
metadata:
  name: test-task
  namespace: foo
spec:
  steps:
    - name: simple-step
      image: foo
      command: ["/mycmd"]
      env:
       - name: foo
         value: bar
`),
	}
	cms := []*corev1.ConfigMap{
		{
			ObjectMeta: metav1.ObjectMeta{Name: config.GetDefaultsConfigName(), Namespace: system.Namespace()},
			Data: map[string]string{
				"default-cloud-events-sink": "http://synk:8080",
			},
		},
	}
	t.Logf("config maps: %s", cms)

	wantEvents := []string{
		"Normal Started",
		"Normal Running Tasks Completed: 0",
	}

	d := test.Data{
		PipelineRuns:            prs,
		Pipelines:               ps,
		Tasks:                   ts,
		ConfigMaps:              cms,
		ExpectedCloudEventCount: len(wantEvents),
	}
	prt := newPipelineRunTest(t, d)
	defer prt.Cancel()

	reconciledRun, clients := prt.reconcileRun("foo", "test-pipelinerun", wantEvents, false)

	// This PipelineRun is in progress now and the status should reflect that
	checkPipelineRunConditionStatusAndReason(t, reconciledRun, corev1.ConditionUnknown, v1.PipelineRunReasonRunning.String())

	verifyTaskRunStatusesCount(t, reconciledRun.Status, 1)

	wantCloudEvents := []string{
		`(?s)dev.tekton.event.pipelinerun.started.v1.*test-pipelinerun`,
		`(?s)dev.tekton.event.pipelinerun.running.v1.*test-pipelinerun`,
	}
	ceClient := clients.CloudEvents.(cloudevent.FakeClient)
	ceClient.CheckCloudEventsUnordered(t, "reconcile-cloud-events", wantCloudEvents)
}

// this test validates taskSpec metadata is embedded into task run
func TestReconcilePipeline_TaskSpecMetadata(t *testing.T) {
	names.TestingSeed()

	prs := []*v1.PipelineRun{
		parse.MustParseV1PipelineRun(t, `
metadata:
  name: test-pipeline-run-success
  namespace: foo
spec:
  pipelineRef:
    name: test-pipeline
`),
	}

	ps := []*v1.Pipeline{
		parse.MustParseV1Pipeline(t, `
metadata:
  name: test-pipeline
  namespace: foo
spec:
  tasks:
    - name: task-without-metadata
      taskSpec:
        steps:
          - name: mystep
            image: myimage
    - name: task-with-metadata
      taskSpec:
        steps:
          - name: mystep
            image: myimage
        metadata:
          labels:
            label1: labelvalue1
            label2: labelvalue2
          annotations:
            annotation1: value1
            annotation2: value2
`),
	}

	d := test.Data{
		PipelineRuns: prs,
		Pipelines:    ps,
		ConfigMaps:   []*corev1.ConfigMap{newFeatureFlagsConfigMap()},
	}
	prt := newPipelineRunTest(t, d)
	defer prt.Cancel()

	reconciledRun, clients := prt.reconcileRun("foo", "test-pipeline-run-success", []string{}, false)

	actions := clients.Pipeline.Actions()
	if len(actions) == 0 {
		t.Fatalf("Expected client to have been used to create a TaskRun but it wasn't")
	}

	actualTaskRun := make(map[string]*v1.TaskRun)
	for _, a := range actions {
		if a.GetResource().Resource == "taskruns" {
			t := a.(ktesting.CreateAction).GetObject().(*v1.TaskRun)
			actualTaskRun[t.Name] = t
		}
	}

	// Check that the expected TaskRun was created
	if len(actualTaskRun) != 2 {
		t.Errorf("Expected two TaskRuns to be created, but found %d TaskRuns.", len(actualTaskRun))
	}

	expectedTaskRun := make(map[string]*v1.TaskRun)
	expectedTaskRun["test-pipeline-run-success-task-with-metadata"] = getTaskRunWithTaskSpec(
		"test-pipeline-run-success-task-with-metadata",
		"test-pipeline-run-success",
		"test-pipeline",
		"task-with-metadata",
		map[string]string{"label1": "labelvalue1", "label2": "labelvalue2"},
		map[string]string{"annotation1": "value1", "annotation2": "value2"},
	)

	expectedTaskRun["test-pipeline-run-success-task-without-metadata"] = getTaskRunWithTaskSpec(
		"test-pipeline-run-success-task-without-metadata",
		"test-pipeline-run-success",
		"test-pipeline",
		"task-without-metadata",
		map[string]string{},
		map[string]string{},
	)

	if d := cmp.Diff(expectedTaskRun, actualTaskRun); d != "" {
		t.Fatalf("Expected TaskRuns to match, but got a mismatch: %s", d)
	}

	verifyTaskRunStatusesCount(t, reconciledRun.Status, 2)
}

func TestReconciler_ReconcileKind_PipelineTaskContext(t *testing.T) {
	names.TestingSeed()

	pipelineName := "p-pipelinetask-status"
	pipelineRunName := "pr-pipelinetask-status"

	ps := []*v1.Pipeline{parse.MustParseV1Pipeline(t, `
metadata:
  name: p-pipelinetask-status
  namespace: foo
spec:
  finally:
  - name: finaltask
    params:
    - name: pipelineRun-tasks-task1
      value: $(tasks.task1.status)
    taskRef:
      name: finaltask
  tasks:
  - name: task1
    taskRef:
      name: mytask
`)}

	prs := []*v1.PipelineRun{parse.MustParseV1PipelineRun(t, `
metadata:
  name: pr-pipelinetask-status
  namespace: foo
spec:
  pipelineRef:
    name: p-pipelinetask-status
  taskRunTemplate:
    serviceAccountName: test-sa
`)}

	ts := []*v1.Task{
		{ObjectMeta: baseObjectMeta("mytask", "foo")},
		parse.MustParseV1Task(t, `
metadata:
  name: finaltask
  namespace: foo
spec:
  params:
  - name: pipelineRun-tasks-task1
    type: string
`)}

	trs := []*v1.TaskRun{mustParseTaskRunWithObjectMeta(t,
		taskRunObjectMeta(pipelineRunName+"-task1-xxyy", "foo", pipelineRunName, pipelineName, "task1", false),
		`
spec:
  serviceAccountName: test-sa
  taskRef:
    name: mytask
  timeout: 1h0m0s
status:
  conditions:
  - reason: Failed
    status: "False"
    type: Succeeded
`)}

	d := test.Data{
		PipelineRuns: prs,
		Pipelines:    ps,
		Tasks:        ts,
		TaskRuns:     trs,
	}
	prt := newPipelineRunTest(t, d)
	defer prt.Cancel()

	_, clients := prt.reconcileRun("foo", pipelineRunName, []string{}, false)

	expectedTaskRunName := pipelineRunName + "-finaltask"
	expectedTaskRunObjectMeta := taskRunObjectMeta(expectedTaskRunName, "foo", pipelineRunName, pipelineName, "finaltask", false)
	expectedTaskRunObjectMeta.Labels[pipeline.MemberOfLabelKey] = v1.PipelineFinallyTasks
	expectedTaskRun := mustParseTaskRunWithObjectMeta(t, expectedTaskRunObjectMeta, `
spec:
  params:
  - name: pipelineRun-tasks-task1
    value: Failed
  serviceAccountName: test-sa
  taskRef:
    name: finaltask
    kind: Task
`)
	// Check that the expected TaskRun was created
	actual, err := clients.Pipeline.TektonV1().TaskRuns("foo").List(prt.TestAssets.Ctx, metav1.ListOptions{
		LabelSelector: "tekton.dev/pipelineTask=finaltask,tekton.dev/pipelineRun=" + pipelineRunName,
		Limit:         1,
	})

	if err != nil {
		t.Fatalf("Failure to list TaskRun's %s", err)
	}
	if len(actual.Items) != 1 {
		t.Fatalf("Expected 1 TaskRuns got %d", len(actual.Items))
	}
	actualTaskRun := actual.Items[0]
	if d := cmp.Diff(expectedTaskRun, &actualTaskRun, ignoreResourceVersion, ignoreTypeMeta); d != "" {
		t.Errorf("expected to see TaskRun %v created. Diff %s", expectedTaskRunName, diff.PrintWantGot(d))
	}
}

func TestReconcileWithTaskResultsInFinalTasks(t *testing.T) {
	names.TestingSeed()

	ps := []*v1.Pipeline{parse.MustParseV1Pipeline(t, `
metadata:
  name: test-pipeline
  namespace: foo
spec:
  finally:
  - name: final-task-1
    params:
    - name: finalParam
      value: $(tasks.dag-task-1.results.aResult)
    taskRef:
      name: final-task
  - name: final-task-2
    params:
    - name: finalParam
      value: $(tasks.dag-task-2.results.aResult)
    taskRef:
      name: final-task
  - name: final-task-3
    params:
    - name: finalParam
      value: param
    taskRef:
      name: final-task
    when:
    - input: $(tasks.dag-task-1.results.aResult)
      operator: notin
      values:
      - aResultValue
  - name: final-task-4
    params:
    - name: finalParam
      value: param
    taskRef:
      name: final-task
    when:
    - input: $(tasks.dag-task-1.results.aResult)
      operator: in
      values:
      - aResultValue
  - name: final-task-5
    params:
    - name: finalParam
      value: param
    taskRef:
      name: final-task
    when:
    - input: $(tasks.dag-task-2.results.aResult)
      operator: notin
      values:
      - aResultValue
  - name: final-task-6
    params:
    - name: finalParam
      value: $(tasks.dag-task-2.results.aResult)
    taskRef:
      name: final-task
    when:
    - input: $(tasks.dag-task-1.results.aResult)
      operator: in
      values:
      - aResultValue
  - name: final-task-7
    params:
    - name: finalParam
      value: $(tasks.dag-task-3.results.aResult)
    taskRef:
      name: final-task
  tasks:
  - name: dag-task-1
    taskRef:
      name: dag-task
  - name: dag-task-2
    taskRef:
      name: dag-task
  - name: dag-task-3
    taskRef:
      name: dag-task
`)}

	prs := []*v1.PipelineRun{parse.MustParseV1PipelineRun(t, `
metadata:
  name: test-pipeline-run-final-task-results
  namespace: foo
spec:
  pipelineRef:
    name: test-pipeline
  taskRunTemplate:
    serviceAccountName: test-sa-0
`)}

	ts := []*v1.Task{
		parse.MustParseV1Task(t, `
metadata:
  name: dag-task
  namespace: foo
`),
		parse.MustParseV1Task(t, `
metadata:
  name: final-task
  namespace: foo
spec:
  params:
  - name: finalParam
    type: string
`),
	}

	trs := []*v1.TaskRun{
		mustParseTaskRunWithObjectMeta(t,
			taskRunObjectMeta("test-pipeline-run-final-task-results-dag-task-1-xxyyy", "foo",
				"test-pipeline-run-final-task-results", "test-pipeline", "dag-task-1", false),
			`
spec:
  serviceAccountName: test-sa
  taskRef:
    name: hello-world
status:
  conditions:
  - reason: Succeeded
    status: "True"
    type: Succeeded
  results:
  - name: aResult
    value: aResultValue
`),
		mustParseTaskRunWithObjectMeta(t,
			taskRunObjectMeta("test-pipeline-run-final-task-results-dag-task-2-xxyyy", "foo",
				"test-pipeline-run-final-task-results", "test-pipeline", "dag-task-2", false),
			`
spec:
  serviceAccountName: test-sa
  taskRef:
    name: hello-world
status:
  conditions:
  - lastTransitionTime: null
    status: "False"
    type: Succeeded
`),
		mustParseTaskRunWithObjectMeta(t,
			taskRunObjectMeta("test-pipeline-run-final-task-results-dag-task-3-xxyyy", "foo",
				"test-pipeline-run-final-task-results", "test-pipeline", "dag-task-3", false),
			`
spec:
  serviceAccountName: test-sa
  taskRef:
    name: hello-world
status:
  conditions:
  - lastTransitionTime: null
    status: "False"
    type: Succeeded
  results:
  - name: aResult
    value: aResultValue
`),
	}

	d := test.Data{
		PipelineRuns: prs,
		Pipelines:    ps,
		Tasks:        ts,
		TaskRuns:     trs,
	}
	prt := newPipelineRunTest(t, d)
	defer prt.Cancel()

	reconciledRun, clients := prt.reconcileRun("foo", "test-pipeline-run-final-task-results", []string{}, false)

	expectedTaskRunName := "test-pipeline-run-final-task-results-final-task-1"
	expectedTaskRunObjectMeta := taskRunObjectMeta("test-pipeline-run-final-task-results-final-task-1", "foo",
		"test-pipeline-run-final-task-results", "test-pipeline", "final-task-1", true)
	expectedTaskRunObjectMeta.Labels[pipeline.MemberOfLabelKey] = v1.PipelineFinallyTasks
	expectedTaskRun := mustParseTaskRunWithObjectMeta(t, expectedTaskRunObjectMeta, `
spec:
  params:
  - name: finalParam
    value: aResultValue
  serviceAccountName: test-sa-0
  taskRef:
    name: final-task
    kind: Task
`)

	// Check that the expected TaskRun was created
	actual, err := clients.Pipeline.TektonV1().TaskRuns("foo").List(prt.TestAssets.Ctx, metav1.ListOptions{
		LabelSelector: "tekton.dev/pipelineTask=final-task-1,tekton.dev/pipelineRun=test-pipeline-run-final-task-results",
		Limit:         1,
	})

	if err != nil {
		t.Fatalf("Failure to list TaskRun's %s", err)
	}
	if len(actual.Items) != 1 {
		t.Fatalf("Expected 1 TaskRuns got %d", len(actual.Items))
	}
	actualTaskRun := actual.Items[0]
	if d := cmp.Diff(*expectedTaskRun, actualTaskRun, ignoreResourceVersion, ignoreTypeMeta); d != "" {
		t.Errorf("expected to see TaskRun %v created. Diff %s", expectedTaskRunName, diff.PrintWantGot(d))
	}
	expectedSkippedTasks := []v1.SkippedTask{{
		Name:   "final-task-2",
		Reason: v1.MissingResultsSkip,
	}, {
		Name:   "final-task-3",
		Reason: v1.WhenExpressionsSkip,
		WhenExpressions: v1.WhenExpressions{{
			Input:    "aResultValue",
			Operator: "notin",
			Values:   []string{"aResultValue"},
		}},
	}, {
		Name:   "final-task-5",
		Reason: v1.MissingResultsSkip,
	}, {
		Name:   "final-task-6",
		Reason: v1.MissingResultsSkip,
	}}

	if d := cmp.Diff(expectedSkippedTasks, reconciledRun.Status.SkippedTasks); d != "" {
		t.Fatalf("Didn't get the expected list of skipped tasks. Diff: %s", diff.PrintWantGot(d))
	}
}

// newPipelineRunTest returns PipelineRunTest with a new PipelineRun controller created with specified state through data
// This PipelineRunTest can be reused for multiple PipelineRuns by calling reconcileRun for each pipelineRun
func newPipelineRunTest(t *testing.T, data test.Data) *PipelineRunTest {
	t.Helper()
	testAssets, cancel := getPipelineRunController(t, data)
	return &PipelineRunTest{
		Data:       data,
		Test:       t,
		TestAssets: testAssets,
		Cancel:     cancel,
	}
}

func (prt PipelineRunTest) reconcileRun(namespace, pipelineRunName string, wantEvents []string, permanentError bool) (*v1.PipelineRun, test.Clients) {
	prt.Test.Helper()
	c := prt.TestAssets.Controller
	clients := prt.TestAssets.Clients
	reconcileError := c.Reconciler.Reconcile(prt.TestAssets.Ctx, namespace+"/"+pipelineRunName)
	if permanentError {
		// When a PipelineRun is invalid and can't run, we expect a permanent error that will
		// tell the Reconciler to not keep trying to reconcile.
		if reconcileError == nil {
			prt.Test.Fatalf("Expected an error to be returned by Reconcile, got nil instead")
		}
		if controller.IsPermanentError(reconcileError) != permanentError {
			prt.Test.Fatalf("Expected the error to be permanent: %v but got: %v", permanentError, controller.IsPermanentError(reconcileError))
		}
	} else if ok, _ := controller.IsRequeueKey(reconcileError); ok {
		// This is normal, it happens for timeouts when we otherwise successfully reconcile.
	} else if reconcileError != nil {
		prt.Test.Fatalf("Error reconciling: %s", reconcileError)
	}
	prt.Test.Logf("Getting reconciled run")
	// Check that the PipelineRun was reconciled correctly
	reconciledRun, err := clients.Pipeline.TektonV1().PipelineRuns(namespace).Get(prt.TestAssets.Ctx, pipelineRunName, metav1.GetOptions{})
	if err != nil {
		prt.Test.Fatalf("Somehow had error getting reconciled run out of fake client: %s", err)
	}
	prt.Test.Logf("Getting events")

	// Check generated events match what's expected
	if len(wantEvents) > 0 {
		if err := k8sevent.CheckEventsOrdered(prt.Test, prt.TestAssets.Recorder.Events, pipelineRunName, wantEvents); err != nil {
			prt.Test.Fatalf("falal added: %s:", err.Error())
			prt.Test.Errorf(err.Error())
		}
	}

	return reconciledRun, clients
}

func TestReconcile_RemotePipelineRef(t *testing.T) {
	names.TestingSeed()

	namespace := "foo"
	prName := "test-pipeline-run-success"
	trName := "test-pipeline-run-success-unit-test-1"

	prs := []*v1.PipelineRun{parse.MustParseV1PipelineRun(t, `
metadata:
  name: test-pipeline-run-success
  namespace: foo
spec:
  pipelineRef:
    resolver: bar
  taskRunTemplate:
    serviceAccountName: test-sa
  timeout: 1h0m0s
`)}
	ps := parse.MustParseV1Pipeline(t, `
metadata:
  name: test-pipeline
  namespace: foo
spec:
  tasks:
  - name: unit-test-1
    taskRef:
      resolver: bar
`)

	remoteTask := parse.MustParseV1Task(t, `
metadata:
  name: unit-test-task
  namespace: foo
`)

	pipelineBytes, err := yaml.Marshal(ps)
	if err != nil {
		t.Fatal("fail to marshal task", err)
	}

	taskBytes, err := yaml.Marshal(remoteTask)
	if err != nil {
		t.Fatal("fail to marshal task", err)
	}

	pipelineReq := getResolvedResolutionRequest(t, "bar", pipelineBytes, "foo", prName)
	taskReq := getResolvedResolutionRequest(t, "bar", taskBytes, "foo", trName)

	// Unlike the tests above, we do *not* locally define our pipeline or unit-test task.
	d := test.Data{
		PipelineRuns: prs,
		ServiceAccounts: []*corev1.ServiceAccount{{
			ObjectMeta: metav1.ObjectMeta{Name: prs[0].Spec.TaskRunTemplate.ServiceAccountName, Namespace: namespace},
		}},
		ConfigMaps: []*corev1.ConfigMap{
			{
				ObjectMeta: metav1.ObjectMeta{Name: config.GetFeatureFlagsConfigName(), Namespace: system.Namespace()},
				Data: map[string]string{
					"enable-api-fields": "beta",
				},
			},
		},
		ResolutionRequests: []*resolutionv1beta1.ResolutionRequest{&taskReq, &pipelineReq},
	}

	prt := newPipelineRunTest(t, d)
	defer prt.Cancel()

	wantEvents := []string{
		"Normal Started",
		"Normal Running Tasks Completed: 0",
	}
	reconciledRun, clients := prt.reconcileRun(namespace, prName, wantEvents, false)

	// Check that the expected TaskRun was created
	taskRuns := getTaskRunsForPipelineRun(prt.TestAssets.Ctx, t, clients, namespace, prName)
	validateTaskRunsCount(t, taskRuns, 1)

	actual := getTaskRunByName(t, taskRuns, trName)
	expectedTaskRun := mustParseTaskRunWithObjectMeta(t,
		taskRunObjectMeta(trName, namespace, prName,
			"test-pipeline", "unit-test-1", false), `
spec:
  serviceAccountName: test-sa
  taskRef:
    kind: Task
    resolver: bar
`)

	if d := cmp.Diff(expectedTaskRun, actual, ignoreTypeMeta, ignoreResourceVersion); d != "" {
		t.Errorf("expected to see TaskRun %v created. Diff %s", expectedTaskRun, diff.PrintWantGot(d))
	}

	// This PipelineRun is in progress now and the status should reflect that
	checkPipelineRunConditionStatusAndReason(t, reconciledRun, corev1.ConditionUnknown, v1beta1.PipelineRunReasonRunning.String())

	verifyTaskRunStatusesCount(t, reconciledRun.Status, 1)
	verifyTaskRunStatusesNames(t, reconciledRun.Status, "test-pipeline-run-success-unit-test-1")
}

// TestReconcile_OptionalWorkspacesOmitted checks that an optional workspace declared by
// a Task and a Pipeline can be omitted by a PipelineRun and the run will still start
// successfully without an error.
func TestReconcile_OptionalWorkspacesOmitted(t *testing.T) {
	names.TestingSeed()

	namespace := "foo"
	prName := "test-pipeline-run-success"
	trName := "test-pipeline-run-success-unit-test-1"

	ctx := context.Background()
	cfg := config.NewStore(logtesting.TestLogger(t))
	ctx = cfg.ToContext(ctx)

	prs := []*v1.PipelineRun{parse.MustParseV1PipelineRun(t, `
metadata:
  name: test-pipeline-run-success
  namespace: foo
spec:
  pipelineSpec:
    tasks:
    - name: unit-test-1
      taskSpec:
        steps:
        - image: foo:latest
        workspaces:
        - name: ws
          optional: true
      workspaces:
      - name: ws
        workspace: optional-workspace
    workspaces:
    - name: optional-workspace
      optional: true
  taskRunTemplate:
    serviceAccountName: test-sa
`)}

	// Unlike the tests above, we do *not* locally define our pipeline or unit-test task.
	d := test.Data{
		PipelineRuns: prs,
		ServiceAccounts: []*corev1.ServiceAccount{{
			ObjectMeta: metav1.ObjectMeta{Name: prs[0].Spec.TaskRunTemplate.ServiceAccountName, Namespace: namespace},
		}},
		ConfigMaps: []*corev1.ConfigMap{newFeatureFlagsConfigMap()},
	}

	prt := newPipelineRunTest(t, d)
	defer prt.Cancel()

	reconciledRun, clients := prt.reconcileRun(namespace, prName, nil, false)

	// Check that the expected TaskRun was created
	taskRuns := getTaskRunsForPipelineRun(prt.TestAssets.Ctx, t, clients, namespace, prName)
	validateTaskRunsCount(t, taskRuns, 1)

	actual := getTaskRunByName(t, taskRuns, trName)
	expectedTaskRun := mustParseTaskRunWithObjectMeta(t,
		taskRunObjectMeta(trName, namespace, prName,
			"test-pipeline-run-success", "unit-test-1", false),
		`
spec:
  serviceAccountName: test-sa
  taskSpec:
    steps:
    - image: foo:latest
    workspaces:
    - name: ws
      optional: true
`)

	if d := cmp.Diff(expectedTaskRun, actual, ignoreTypeMeta, ignoreResourceVersion); d != "" {
		t.Errorf("expected to see TaskRun %v created. Diff %s", expectedTaskRun, diff.PrintWantGot(d))
	}

	// This PipelineRun is in progress now and the status should reflect that
	checkPipelineRunConditionStatusAndReason(t, reconciledRun, corev1.ConditionUnknown, v1.PipelineRunReasonRunning.String())

	verifyTaskRunStatusesCount(t, reconciledRun.Status, 1)
	verifyTaskRunStatusesNames(t, reconciledRun.Status, "test-pipeline-run-success-unit-test-1")
}

func TestReconcile_DependencyValidationsImmediatelyFailPipelineRun(t *testing.T) {
	names.TestingSeed()

	ctx := context.Background()
	cfg := config.NewStore(logtesting.TestLogger(t))
	ctx = cfg.ToContext(ctx)

	prs := []*v1.PipelineRun{
		parse.MustParseV1PipelineRun(t, `
metadata:
  name: pipelinerun-param-invalid-result-variable
  namespace: foo
spec:
  pipelineSpec:
    tasks:
    - name: pt0
      taskSpec:
        steps:
        - image: foo:latest
    - name: pt1
      params:
      - name: p
        value: $(tasks.pt0.results.r1)
      taskSpec:
        params:
        - name: p
        steps:
        - image: foo:latest
  serviceAccountName: test-sa
`),
		parse.MustParseV1PipelineRun(t, `
metadata:
  name: pipelinerun-pipeline-result-invalid-result-variable
  namespace: foo
spec:
  pipelineSpec:
    results:
    - name: pr
      value: $(tasks.pt0.results.r)
    tasks:
    - name: pt0
      taskSpec:
        steps:
        - image: foo:latest
    - name: pt1
      taskSpec:
        steps:
        - image: foo:latest
  serviceAccountName: test-sa
`),
		parse.MustParseV1PipelineRun(t, `
metadata:
  name: pipelinerun-with-optional-workspace-validation
  namespace: foo
spec:
  pipelineSpec:
    tasks:
    - name: unit-test-1
      taskSpec:
        steps:
        - image: foo:latest
        workspaces:
        - name: ws
      workspaces:
      - name: ws
        workspace: optional-workspace
    workspaces:
    - name: optional-workspace
      optional: true
  serviceAccountName: test-sa
`),
		parse.MustParseV1PipelineRun(t, `
metadata:
  name: pipelinerun-matrix-param-invalid-type
  namespace: foo
spec:
  pipelineSpec:
   tasks:
    - name: mytask
      params:
       - name: platform
      taskSpec:
        steps:
         - name: echo
           image: alpine
           script: |
             echo "$(params.platform)"
    - name: b-task
      taskRef:
        name: mytask
      matrix:
        params:
          - name: platform
            value: linux
`),
	}

	cms := []*corev1.ConfigMap{withEnabledAlphaAPIFields(newFeatureFlagsConfigMap())}
	d := test.Data{
		ConfigMaps:   cms,
		PipelineRuns: prs,
		ServiceAccounts: []*corev1.ServiceAccount{{
			ObjectMeta: metav1.ObjectMeta{Name: prs[0].Spec.TaskRunTemplate.ServiceAccountName, Namespace: "foo"},
		}},
	}

	prt := newPipelineRunTest(t, d)
	defer prt.Cancel()

	testCases := []struct {
		name   string
		reason string
	}{
		{
			name:   "pipelinerun-param-invalid-result-variable",
			reason: ReasonInvalidTaskResultReference,
		}, {
			name:   "pipelinerun-pipeline-result-invalid-result-variable",
			reason: ReasonInvalidTaskResultReference,
		}, {
			name:   "pipelinerun-with-optional-workspace-validation",
			reason: ReasonRequiredWorkspaceMarkedOptional,
		}, {
			name:   "pipelinerun-matrix-param-invalid-type",
			reason: ReasonInvalidMatrixParameterTypes,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			run, _ := prt.reconcileRun("foo", tc.name, nil, true)
			if run == nil {
				t.Fatalf("Could not reconcile run %s", tc.name)
			}
			c := run.Status.GetCondition(apis.ConditionSucceeded)
			if c.Status != corev1.ConditionFalse {
				t.Errorf("expected Succeeded/False condition but saw: %v", c)
			}
			if c.Reason != tc.reason {
				t.Errorf("want reason %s but got %s", tc.reason, c.Reason)
			}
		})
	}
}

// TestReconcileWithResolver checks that a PipelineRun with a populated Resolver
// field creates a ResolutionRequest object for that Resolver's type, and
// that when the request is successfully resolved the PipelineRun begins running.
func TestReconcileWithResolver(t *testing.T) {
	resolverName := "foobar"
	pr := parse.MustParseV1PipelineRun(t, `
metadata:
  name: pr
  namespace: default
spec:
  pipelineRef:
    resolver: foobar
  serviceAccountName: default
`)

	d := test.Data{
		PipelineRuns: []*v1.PipelineRun{pr},
		ServiceAccounts: []*corev1.ServiceAccount{{
			ObjectMeta: metav1.ObjectMeta{Name: pr.Spec.TaskRunTemplate.ServiceAccountName, Namespace: "foo"},
		}},
	}

	prt := newPipelineRunTest(t, d)
	defer prt.Cancel()

	wantEvents := []string(nil)
	pipelinerun, _ := prt.reconcileRun(pr.Namespace, pr.Name, wantEvents, false)
	checkPipelineRunConditionStatusAndReason(t, pipelinerun, corev1.ConditionUnknown, ReasonResolvingPipelineRef)

	client := prt.TestAssets.Clients.ResolutionRequests.ResolutionV1beta1().ResolutionRequests("default")
	resolutionrequests, err := client.List(prt.TestAssets.Ctx, metav1.ListOptions{})
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
	var pipelineBytes = []byte(`
          kind: Pipeline
          apiVersion: tekton.dev/v1
          metadata:
            name: foo
          spec:
            tasks:
            - name: task1
              taskSpec:
                steps:
                - name: step1
                  image: ubuntu
                  script: |
                    echo "hello world!"
        `)
	resreq.Status.ResolutionRequestStatusFields.Data = base64.StdEncoding.Strict().EncodeToString(pipelineBytes)
	resreq.Status.MarkSucceeded()
	resreq, err = client.UpdateStatus(prt.TestAssets.Ctx, resreq, metav1.UpdateOptions{})
	if err != nil {
		t.Fatalf("unexpected error updating resource request with resolved pipeline data: %v", err)
	}

	// Check that the resolved pipeline was recognized by the
	// PipelineRun reconciler and that the PipelineRun has now
	// started executing.
	updatedPipelineRun, _ := prt.reconcileRun("default", "pr", nil, false)
	checkPipelineRunConditionStatusAndReason(t, updatedPipelineRun, corev1.ConditionUnknown, v1.PipelineRunReasonRunning.String())
}

// TestReconcileWithFailingResolver checks that a PipelineRun with a failing Resolver
// field creates a ResolutionRequest object for that Resolver's type, and
// that when the request fails, the PipelineRun fails.
func TestReconcileWithFailingResolver(t *testing.T) {
	resolverName := "does-not-exist"
	pr := parse.MustParseV1PipelineRun(t, `
metadata:
  name: pr
  namespace: default
spec:
  pipelineRef:
    resolver: does-not-exist
  serviceAccountName: default
`)

	d := test.Data{
		PipelineRuns: []*v1.PipelineRun{pr},
		ServiceAccounts: []*corev1.ServiceAccount{{
			ObjectMeta: metav1.ObjectMeta{Name: pr.Spec.TaskRunTemplate.ServiceAccountName, Namespace: "foo"},
		}},
	}

	prt := newPipelineRunTest(t, d)
	defer prt.Cancel()

	wantEvents := []string(nil)
	pipelinerun, _ := prt.reconcileRun(pr.Namespace, pr.Name, wantEvents, false)
	checkPipelineRunConditionStatusAndReason(t, pipelinerun, corev1.ConditionUnknown, ReasonResolvingPipelineRef)

	client := prt.TestAssets.Clients.ResolutionRequests.ResolutionV1beta1().ResolutionRequests("default")
	resolutionrequests, err := client.List(prt.TestAssets.Ctx, metav1.ListOptions{})
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
	resreq, err = client.UpdateStatus(prt.TestAssets.Ctx, resreq, metav1.UpdateOptions{})
	if err != nil {
		t.Fatalf("unexpected error updating resource request with resolved pipeline data: %v", err)
	}

	// Check that the pipeline fails with the appropriate reason.
	updatedPipelineRun, _ := prt.reconcileRun("default", "pr", nil, true)
	checkPipelineRunConditionStatusAndReason(t, updatedPipelineRun, corev1.ConditionFalse, ReasonCouldntGetPipeline)
}

// TestReconcileWithFailingTaskResolver checks that a PipelineRun with a failing Resolver
// field for a Task creates a ResolutionRequest object for that Resolver's type, and
// that when the request fails, the PipelineRun fails.
func TestReconcileWithFailingTaskResolver(t *testing.T) {
	resolverName := "foobar"
	pr := parse.MustParseV1PipelineRun(t, `
metadata:
  name: pr
  namespace: default
spec:
  pipelineSpec:
    tasks:
    - name: some-task
      taskRef:
        resolver: foobar
  taskRunTemplate:
    serviceAccountName: default
`)

	d := test.Data{
		PipelineRuns: []*v1.PipelineRun{pr},
		ServiceAccounts: []*corev1.ServiceAccount{{
			ObjectMeta: metav1.ObjectMeta{Name: pr.Spec.TaskRunTemplate.ServiceAccountName, Namespace: "foo"},
		}},
		ConfigMaps: []*corev1.ConfigMap{withEnabledAlphaAPIFields(newFeatureFlagsConfigMap())},
	}

	prt := newPipelineRunTest(t, d)
	defer prt.Cancel()

	wantEvents := []string(nil)
	pipelinerun, _ := prt.reconcileRun(pr.Namespace, pr.Name, wantEvents, false)
	checkPipelineRunConditionStatusAndReason(t, pipelinerun, corev1.ConditionUnknown, v1.TaskRunReasonResolvingTaskRef)

	client := prt.TestAssets.Clients.ResolutionRequests.ResolutionV1beta1().ResolutionRequests("default")
	resolutionrequests, err := client.List(prt.TestAssets.Ctx, metav1.ListOptions{})
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
	resreq, err = client.UpdateStatus(prt.TestAssets.Ctx, resreq, metav1.UpdateOptions{})
	if err != nil {
		t.Fatalf("unexpected error updating resource request with resolved pipeline data: %v", err)
	}

	// Check that the pipeline fails.
	updatedPipelineRun, _ := prt.reconcileRun("default", "pr", nil, true)
	checkPipelineRunConditionStatusAndReason(t, updatedPipelineRun, corev1.ConditionFalse, ReasonCouldntGetTask)
}

// TestReconcileWithTaskResolver checks that a PipelineRun with a populated Resolver
// field for a Task creates a ResolutionRequest object for that Resolver's type, and
// that when the request is successfully resolved the PipelineRun begins running.
func TestReconcileWithTaskResolver(t *testing.T) {
	resolverName := "foobar"
	pr := parse.MustParseV1PipelineRun(t, `
metadata:
  name: pr
  namespace: default
spec:
  pipelineSpec:
    tasks:
    - name: some-task
      taskRef:
        resolver: foobar
        params:
        - name: foo
          value: bar
  taskRunTemplate:
    serviceAccountName: default
`)

	d := test.Data{
		PipelineRuns: []*v1.PipelineRun{pr},
		ServiceAccounts: []*corev1.ServiceAccount{{
			ObjectMeta: metav1.ObjectMeta{Name: pr.Spec.TaskRunTemplate.ServiceAccountName, Namespace: "foo"},
		}},
		ConfigMaps: []*corev1.ConfigMap{withEnabledAlphaAPIFields(newFeatureFlagsConfigMap())},
	}

	prt := newPipelineRunTest(t, d)
	defer prt.Cancel()

	wantEvents := []string(nil)
	pipelinerun, _ := prt.reconcileRun(pr.Namespace, pr.Name, wantEvents, false)
	checkPipelineRunConditionStatusAndReason(t, pipelinerun, corev1.ConditionUnknown, v1.TaskRunReasonResolvingTaskRef)

	client := prt.TestAssets.Clients.ResolutionRequests.ResolutionV1beta1().ResolutionRequests("default")
	resolutionrequests, err := client.List(prt.TestAssets.Ctx, metav1.ListOptions{})
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
	fooVal := ""
	for _, p := range resreq.Spec.Params {
		if p.Name == "foo" {
			fooVal = p.Value.StringVal
		}
	}
	if fooVal != "bar" {
		t.Fatalf("expected resource request parameter 'bar', but got '%s'", fooVal)
	}

	taskBytes := []byte(`
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
	resreq, err = client.UpdateStatus(prt.TestAssets.Ctx, resreq, metav1.UpdateOptions{})
	if err != nil {
		t.Fatalf("unexpected error updating resource request with resolved pipeline data: %v", err)
	}

	// Check that the resolved pipeline was recognized by the
	// PipelineRun reconciler and that the PipelineRun has now
	// started executing.
	updatedPipelineRun, _ := prt.reconcileRun("default", "pr", nil, false)
	checkPipelineRunConditionStatusAndReason(t, updatedPipelineRun, corev1.ConditionUnknown, v1.PipelineRunReasonRunning.String())
}

func getTaskRunWithTaskSpec(tr, pr, p, t string, labels, annotations map[string]string) *v1.TaskRun {
	om := taskRunObjectMeta(tr, "foo", pr, p, t, false)
	for k, v := range labels {
		om.Labels[k] = v
	}
	om.Annotations = annotations

	return &v1.TaskRun{
		ObjectMeta: om,
		Spec: v1.TaskRunSpec{
			TaskSpec: &v1.TaskSpec{
				Steps: []v1.Step{{
					Name:  "mystep",
					Image: "myimage",
				}},
			},
			ServiceAccountName: config.DefaultServiceAccountValue,
		},
	}
}

func baseObjectMeta(name, ns string) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:        name,
		Namespace:   ns,
		Labels:      map[string]string{},
		Annotations: map[string]string{},
	}
}

func taskRunObjectMeta(trName, ns, prName, pipelineName, pipelineTaskName string, skipMemberOfLabel bool) metav1.ObjectMeta {
	om := metav1.ObjectMeta{
		Name:      trName,
		Namespace: ns,
		OwnerReferences: []metav1.OwnerReference{{
			Kind:               "PipelineRun",
			Name:               prName,
			APIVersion:         "tekton.dev/v1",
			Controller:         &trueb,
			BlockOwnerDeletion: &trueb,
		}},
		Labels: map[string]string{
			pipeline.PipelineLabelKey:     pipelineName,
			pipeline.PipelineRunLabelKey:  prName,
			pipeline.PipelineTaskLabelKey: pipelineTaskName,
		},
		Annotations: map[string]string{},
	}
	if !skipMemberOfLabel {
		om.Labels[pipeline.MemberOfLabelKey] = v1.PipelineTasks
	}
	return om
}

func taskRunObjectMetaWithAnnotations(trName, ns, prName, pipelineName, pipelineTaskName string, skipMemberOfLabel bool, annotations map[string]string) metav1.ObjectMeta {
	om := taskRunObjectMeta(trName, ns, prName, pipelineName, pipelineTaskName, skipMemberOfLabel)
	for k, v := range annotations {
		om.Annotations[k] = v
	}
	return om
}

func createHelloWorldTaskRunWithStatus(
	t *testing.T,
	trName, ns, prName, pName, podName string,
	condition apis.Condition,
) *v1.TaskRun {
	t.Helper()
	p := createHelloWorldTaskRun(t, trName, ns, prName, pName)
	p.Status = v1.TaskRunStatus{
		Status: duckv1.Status{
			Conditions: duckv1.Conditions{condition},
		},
		TaskRunStatusFields: v1.TaskRunStatusFields{
			PodName: podName,
		},
	}
	return p
}

func createHelloWorldTaskRunWithStatusTaskLabel(
	t *testing.T,
	trName, ns, prName, pName, podName, taskLabel string,
	condition apis.Condition,
) *v1.TaskRun {
	t.Helper()
	p := createHelloWorldTaskRunWithStatus(t, trName, ns, prName, pName, podName, condition)
	p.Labels[pipeline.PipelineTaskLabelKey] = taskLabel

	return p
}

func createHelloWorldTaskRun(t *testing.T, trName, ns, prName, pName string) *v1.TaskRun {
	t.Helper()
	return parse.MustParseV1TaskRun(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
  labels:
    %s: %s
    %s: %s
spec:
  taskRef:
    name: hello-world
  serviceAccountName: test-sa
`, trName, ns, pipeline.PipelineLabelKey, pName, pipeline.PipelineRunLabelKey, prName))
}

func createCancelledPipelineRun(t *testing.T, prName string, specStatus v1.PipelineRunSpecStatus) *v1.PipelineRun {
	t.Helper()
	return parse.MustParseV1PipelineRun(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: foo
spec:
  pipelineRef:
    name: test-pipeline
  serviceAccountName: test-sa
  status: %s
status:
  startTime: %s`, prName, specStatus, now.Format(time.RFC3339)))
}
func verifyTaskRunStatusesCount(t *testing.T, prStatus v1.PipelineRunStatus, taskCount int) {
	t.Helper()

	if len(filterChildRefsForKind(prStatus.ChildReferences, taskRun)) != taskCount {
		t.Errorf("Expected PipelineRun status ChildReferences to have %d tasks, but was %d", taskCount, len(filterChildRefsForKind(prStatus.ChildReferences, taskRun)))
	}
}
func verifyTaskRunStatusesNames(t *testing.T, prStatus v1.PipelineRunStatus, taskNames ...string) {
	t.Helper()

	tnMap := make(map[string]struct{})
	for _, cr := range filterChildRefsForKind(prStatus.ChildReferences, taskRun) {
		tnMap[cr.Name] = struct{}{}
	}

	for _, tn := range taskNames {
		if _, ok := tnMap[tn]; !ok {
			t.Errorf("Expected PipelineRun status to include TaskRun status for %s but was %v", tn, prStatus.ChildReferences)
		}
	}
}

func verifyCustomRunOrRunStatusesCount(t *testing.T, kind string, prStatus v1.PipelineRunStatus, runCount int) {
	t.Helper()
	if len(filterChildRefsForKind(prStatus.ChildReferences, kind)) != runCount {
		t.Errorf("Expected PipelineRun status ChildReferences to have %d %ss, but was %d", runCount, kind, len(filterChildRefsForKind(prStatus.ChildReferences, kind)))
	}
}

func verifyCustomRunOrRunStatusesNames(t *testing.T, kind string, prStatus v1.PipelineRunStatus, runNames ...string) {
	t.Helper()

	rnMap := make(map[string]struct{})
	for _, cr := range filterChildRefsForKind(prStatus.ChildReferences, kind) {
		rnMap[cr.Name] = struct{}{}
	}

	for _, rn := range runNames {
		if _, ok := rnMap[rn]; !ok {
			t.Errorf("Expected PipelineRun status to include %s status for %s but was %v", kind, rn, prStatus.ChildReferences)
		}
	}
}

func verifyTaskRunStatusesWhenExpressions(t *testing.T, prStatus v1.PipelineRunStatus, trName string, expectedWhen []v1.WhenExpression) {
	t.Helper()
	var actualWhenExpressionsInTaskRun []v1.WhenExpression
	for _, cr := range prStatus.ChildReferences {
		if cr.Name == trName {
			actualWhenExpressionsInTaskRun = append(actualWhenExpressionsInTaskRun, cr.WhenExpressions...)
		}
	}
	if d := cmp.Diff(expectedWhen, actualWhenExpressionsInTaskRun); d != "" {
		t.Errorf("expected to see When Expressions %v created. Diff %s", trName, diff.PrintWantGot(d))
	}
}

func filterChildRefsForKind(childRefs []v1.ChildStatusReference, kind string) []v1.ChildStatusReference {
	var filtered []v1.ChildStatusReference
	for _, cr := range childRefs {
		if cr.Kind == kind {
			filtered = append(filtered, cr)
		}
	}
	return filtered
}

func mustParseTaskRunWithObjectMeta(t *testing.T, objectMeta metav1.ObjectMeta, asYAML string) *v1.TaskRun {
	t.Helper()
	tr := parse.MustParseV1TaskRun(t, asYAML)
	tr.ObjectMeta = objectMeta
	return tr
}

func mustParseCustomRunWithObjectMeta(t *testing.T, objectMeta metav1.ObjectMeta, asYAML string) *v1beta1.CustomRun {
	t.Helper()
	r := parse.MustParseCustomRun(t, asYAML)
	r.ObjectMeta = objectMeta
	return r
}

func helloWorldPipelineWithRunAfter(t *testing.T) *v1.Pipeline {
	t.Helper()
	return parse.MustParseV1Pipeline(t, `
metadata:
  name: test-pipeline
  namespace: foo
spec:
  tasks:
    - name: hello-world-1
      taskRef:
        name: hello-world
    - name: hello-world-2
      taskRef:
        name: hello-world
      runAfter:
        - hello-world-1
`)
}

func checkPipelineRunConditionStatusAndReason(t *testing.T, reconciledRun *v1.PipelineRun, conditionStatus corev1.ConditionStatus, conditionReason string) {
	t.Helper()

	condition := reconciledRun.Status.GetCondition(apis.ConditionSucceeded)
	if condition == nil {
		t.Fatalf("want condition, got nil")
	}
	if condition.Status != conditionStatus {
		t.Errorf("want status %v, got %v", conditionStatus, condition.Status)
	}
	if condition.Reason != conditionReason {
		t.Errorf("want reason %v, got %v", conditionReason, condition.Reason)
	}
}

func TestGetTaskrunWorkspaces_Failure(t *testing.T) {
	tests := []struct {
		name          string
		pr            *v1.PipelineRun
		expectedError string
	}{{
		name: "failure declaring workspace with different name",
		pr: parse.MustParseV1PipelineRun(t, `
metadata:
  name: pipeline
spec:
  workspaces:
    - name: source
  pipelineSpec:
    workspaces:
      - name: source
    tasks:
      - name: resolved-pipelinetask
        workspaces:
          - name: not-source
`),
		expectedError: `expected workspace "not-source" to be provided by pipelinerun for pipeline task "resolved-pipelinetask"`,
	},
		{
			name: "failure mapping workspace with different name",
			pr: parse.MustParseV1PipelineRun(t, `
metadata:
  name: pipeline
spec:
  workspaces:
    - name: source
  pipelineSpec:
    workspaces:
      - name: source
    tasks:
      - name: resolved-pipelinetask
        workspaces:
          - name: not-source
            workspace: ""
`),
			expectedError: `expected workspace "not-source" to be provided by pipelinerun for pipeline task "resolved-pipelinetask"`,
		},
		{
			name: "failure propagating workspaces using scripts",
			pr: parse.MustParseV1PipelineRun(t, `
metadata:
  name: pipeline
spec:
  workspaces:
    - name: source
  pipelineSpec:
    workspaces:
      - name: source
    tasks:
      - name: resolved-pipelinetask
        taskSpec:
          steps:
            - name: mystep
              image: myimage
              script: echo $(workspaces.not-source.path)
`),
			expectedError: `expected workspace "not-source" to be provided by pipelinerun for pipeline task "resolved-pipelinetask"`,
		},
		{
			name: "failure propagating workspaces using args",
			pr: parse.MustParseV1PipelineRun(t, `
metadata:
  name: pipeline
spec:
  workspaces:
    - name: source
  pipelineSpec:
    workspaces:
      - name: source
    tasks:
      - name: resolved-pipelinetask
        taskSpec:
          steps:
            - name: mystep
              image: myimage
              command:
                - /mycmd
              args:
                - echo $(workspaces.not-source.path)
        workspaces:
          - name: source
`),
			expectedError: `expected workspace "not-source" to be provided by pipelinerun for pipeline task "resolved-pipelinetask"`,
		},
	}
	ctx := config.EnableAlphaAPIFields(context.Background())
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := Reconciler{
				KubeClientSet: fakek8s.NewSimpleClientset(),
			}
			rprt := &resources.ResolvedPipelineTask{PipelineTask: &tt.pr.Spec.PipelineSpec.Tasks[0]}
			_, _, err := c.getTaskrunWorkspaces(ctx, tt.pr, rprt)
			if err == nil {
				t.Errorf("Pipeline.getTaskrunWorkspaces() did not return error for invalid workspace")
			} else if d := cmp.Diff(tt.expectedError, err.Error(), cmpopts.IgnoreUnexported(apis.FieldError{})); d != "" {
				t.Errorf("Pipeline.getTaskrunWorkspaces() errors diff %s", diff.PrintWantGot(d))
			} else if !controller.IsPermanentError(err) {
				t.Errorf("Pipeline.getTaskrunWorkspaces() not returned a permanent error for invalid workspace")
			}
		})
	}
}

func TestGetTaskrunWorkspaces_Success(t *testing.T) {
	tests := []struct {
		name string
		pr   *v1.PipelineRun
		rprt *resources.ResolvedPipelineTask
	}{{
		name: "valid declaration of workspace names",
		pr: parse.MustParseV1PipelineRun(t, `
metadata:
  name: pipeline
spec:
  workspaces:
    - name: source`),
		rprt: &resources.ResolvedPipelineTask{
			PipelineTask: &v1.PipelineTask{
				Name: "resolved-pipelinetask",
				Workspaces: []v1.WorkspacePipelineTaskBinding{{
					Name:      "my-task-workspace",
					Workspace: "source",
				}},
			},
		},
	},
		{
			name: "valid mapping with same workspace names",
			pr: parse.MustParseV1PipelineRun(t, `
metadata:
  name: pipeline
spec:
  workspaces:
    - name: source`),
			rprt: &resources.ResolvedPipelineTask{
				PipelineTask: &v1.PipelineTask{
					Name: "resolved-pipelinetask",
					Workspaces: []v1.WorkspacePipelineTaskBinding{{
						Name:      "source",
						Workspace: "",
					}},
				},
			},
		},
		{
			name: "propagating workspaces using scripts",
			pr: parse.MustParseV1PipelineRun(t, `
metadata:
  name: pipeline
spec:
  workspaces:
    - name: source`),
			rprt: &resources.ResolvedPipelineTask{
				PipelineTask: &v1.PipelineTask{
					Name: "resolved-pipelinetask",
					TaskSpec: &v1.EmbeddedTask{
						TaskSpec: v1.TaskSpec{
							Steps: []v1.Step{{
								Name:   "mystep",
								Image:  "myimage",
								Script: "echo $(workspaces.source.path)",
							}},
						},
					},
				},
			},
		},
		{
			name: "propagating workspaces using args",
			pr: parse.MustParseV1PipelineRun(t, `
metadata:
  name: pipeline
spec:
  workspaces:
    - name: source`),
			rprt: &resources.ResolvedPipelineTask{
				PipelineTask: &v1.PipelineTask{
					Name: "resolved-pipelinetask",
					TaskSpec: &v1.EmbeddedTask{
						TaskSpec: v1.TaskSpec{
							Steps: []v1.Step{{
								Name:    "mystep",
								Image:   "myimage",
								Command: []string{"/mycmd"},
								Args:    []string{"echo $(workspaces.source.path)"},
							}},
						},
					},
					Workspaces: []v1.WorkspacePipelineTaskBinding{{
						Name: "source",
					}},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := Reconciler{
				KubeClientSet: fakek8s.NewSimpleClientset(),
			}
			_, _, err := c.getTaskrunWorkspaces(context.Background(), tt.pr, tt.rprt)

			if err != nil {
				t.Errorf("Pipeline.getTaskrunWorkspaces() returned error for valid pipeline: %v", err)
			}
		})
	}
}

func Test_taskWorkspaceByWorkspaceVolumeSource(t *testing.T) {
	testPr := &v1beta1.PipelineRun{}
	tests := []struct {
		name, taskWorkspaceName, pipelineWorkspaceName, prName string
		wb                                                     v1.WorkspaceBinding
		expectedBinding                                        v1.WorkspaceBinding
		disableAffinityAssistant                               bool
	}{
		{
			name:                  "PVC Workspace with Affinity Assistant",
			prName:                "test-pipeline-run",
			taskWorkspaceName:     "task-workspace",
			pipelineWorkspaceName: "pipeline-workspace",
			wb: v1.WorkspaceBinding{
				Name:                "foo",
				VolumeClaimTemplate: &corev1.PersistentVolumeClaim{},
			},
			expectedBinding: v1.WorkspaceBinding{
				Name: "task-workspace",
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: "pvc-2c26b46b68-affinity-assistant-e011a5ef79-0",
				},
			},
		},
		{
			name:              "PVC Workspace without Affinity Assistant",
			prName:            "test-pipeline-run",
			taskWorkspaceName: "task-workspace",
			wb: v1.WorkspaceBinding{
				Name:                "foo",
				VolumeClaimTemplate: &corev1.PersistentVolumeClaim{},
			},
			expectedBinding: v1.WorkspaceBinding{
				Name: "task-workspace",
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: "pvc-2c26b46b68",
				},
			},
			disableAffinityAssistant: true,
		},
		{
			name:              "non-PVC Workspace",
			taskWorkspaceName: "task-workspace",
			wb: v1.WorkspaceBinding{
				Name:     "foo",
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
			expectedBinding: v1.WorkspaceBinding{
				Name:     "task-workspace",
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := Reconciler{}
			ctx := context.Background()
			if tc.disableAffinityAssistant {
				featureFlags, err := config.NewFeatureFlagsFromMap(map[string]string{
					"disable-affinity-assistant": "true",
				})
				if err != nil {
					t.Fatalf("error creating feature flag disable-affinity-assistant from map: %v", err)
				}
				cfg := &config.Config{
					FeatureFlags: featureFlags,
				}
				ctx = config.ToContext(context.Background(), cfg)
			}

			binding := c.taskWorkspaceByWorkspaceVolumeSource(ctx, tc.pipelineWorkspaceName, tc.prName, tc.wb, tc.taskWorkspaceName, "", *kmeta.NewControllerRef(testPr))
			if d := cmp.Diff(tc.expectedBinding, binding); d != "" {
				t.Errorf("WorkspaceBinding diff: %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestReconcile_PropagatePipelineTaskRunSpecMetadata(t *testing.T) {
	names.TestingSeed()

	namespace := "foo"
	prName := "test-pipeline-run"
	trName := "test-pipeline-run-hello-world-1"

	ps := []*v1.Pipeline{simpleHelloWorldPipeline}
	prs := []*v1.PipelineRun{parse.MustParseV1PipelineRun(t, `
metadata:
  name: test-pipeline-run
  namespace: foo
spec:
  pipelineRef:
    name: test-pipeline
  taskRunSpecs:
  - pipelineTaskName: hello-world-1
    metadata:
      labels:
        PipelineTaskRunSpecLabel: PipelineTaskRunSpecValue
      annotations:
        PipelineTaskRunSpecAnnotation: PipelineTaskRunSpecValue
    serviceAccountName: custom-sa
`)}
	ts := []*v1.Task{simpleHelloWorldTask}

	d := test.Data{
		PipelineRuns: prs,
		Pipelines:    ps,
		Tasks:        ts,
	}
	prt := newPipelineRunTest(t, d)
	defer prt.Cancel()

	_, clients := prt.reconcileRun(namespace, prName, []string{}, false)

	taskRuns := getTaskRunsForPipelineRun(prt.TestAssets.Ctx, t, clients, namespace, prName)
	validateTaskRunsCount(t, taskRuns, 1)

	actual := getTaskRunByName(t, taskRuns, trName)
	expectedTaskRunObjectMeta := taskRunObjectMeta(trName, namespace, prName, "test-pipeline", "hello-world-1", false)
	expectedTaskRunObjectMeta.Labels["PipelineTaskRunSpecLabel"] = "PipelineTaskRunSpecValue"
	expectedTaskRunObjectMeta.Annotations["PipelineTaskRunSpecAnnotation"] = "PipelineTaskRunSpecValue"
	expectedTaskRun := mustParseTaskRunWithObjectMeta(t, expectedTaskRunObjectMeta, `
spec:
  serviceAccountName: custom-sa
  taskRef:
    name: hello-world
    kind: Task
`)

	if d := cmp.Diff(expectedTaskRun, actual, ignoreTypeMeta, ignoreResourceVersion); d != "" {
		t.Errorf("expected to see propagated metadata from PipelineTaskRunSpec in TaskRun %v created. Diff %s", expectedTaskRun, diff.PrintWantGot(d))
	}
}

func TestReconcile_AddMetadataByPrecedence(t *testing.T) {
	names.TestingSeed()

	namespace := "foo"
	prName := "test-pipeline-run"
	trName := "test-pipeline-run-hello-world-1"

	ps := []*v1.Pipeline{parse.MustParseV1Pipeline(t, `
metadata:
  name: test-pipeline
  namespace: foo
spec:
  tasks:
    - name: hello-world-1
      taskSpec:
        steps:
          - name: foo-step
            image: foo-image
        metadata:
          labels:
            TestPrecedenceLabel: PipelineTaskSpecValue
          annotations:
            TestPrecedenceAnnotation: PipelineTaskSpecValue
`)}
	prs := []*v1.PipelineRun{parse.MustParseV1PipelineRun(t, `
metadata:
  name: test-pipeline-run
  namespace: foo
  metadata:
    labels:
      TestPrecedenceLabel: PipelineRunValue
    annotations:
      TestPrecedenceAnnotation: PipelineRunValue
spec:
  pipelineRef:
    name: test-pipeline
  taskRunSpecs:
  - pipelineTaskName: hello-world-1
    metadata:
      labels:
        TestPrecedenceLabel: PipelineTaskRunSpecValue
      annotations:
        TestPrecedenceAnnotation: PipelineTaskRunSpecValue
    serviceAccountName: custom-sa
`)}
	ts := []*v1.Task{simpleHelloWorldTask}

	d := test.Data{
		PipelineRuns: prs,
		Pipelines:    ps,
		Tasks:        ts,
	}
	prt := newPipelineRunTest(t, d)
	defer prt.Cancel()

	_, clients := prt.reconcileRun(namespace, prName, []string{}, false)

	taskRuns := getTaskRunsForPipelineRun(prt.TestAssets.Ctx, t, clients, namespace, prName)
	validateTaskRunsCount(t, taskRuns, 1)

	actual := getTaskRunByName(t, taskRuns, trName)
	expectedTaskRunObjectMeta := taskRunObjectMeta(trName, namespace, prName, "test-pipeline", "hello-world-1", false)
	expectedTaskRunObjectMeta.Labels["TestPrecedenceLabel"] = "PipelineTaskRunSpecValue"
	expectedTaskRunObjectMeta.Annotations["TestPrecedenceAnnotation"] = "PipelineTaskRunSpecValue"
	expectedTaskRun := mustParseTaskRunWithObjectMeta(t, expectedTaskRunObjectMeta, `
spec:
  serviceAccountName: custom-sa
  taskSpec:
    steps:
      - name: foo-step
        image: foo-image
`)

	if d := cmp.Diff(expectedTaskRun, actual, ignoreTypeMeta, ignoreResourceVersion); d != "" {
		t.Errorf("expected to see propagated metadata by the precedence from PipelineTaskRunSpec in TaskRun %v created. Diff %s", expectedTaskRun, diff.PrintWantGot(d))
	}
}

func TestReconciler_PipelineTaskMatrix(t *testing.T) {
	names.TestingSeed()

	task := parse.MustParseV1Task(t, `
metadata:
  name: mytask
  namespace: foo
spec:
  params:
    - name: platform
    - name: browser
    - name: version
  steps:
    - name: echo
      image: alpine
      script: |
        echo "$(params.platform) and $(params.browser) and $(params.version)"
`)

	expectedTaskRuns := []*v1.TaskRun{
		mustParseTaskRunWithObjectMeta(t,
			taskRunObjectMeta("pr-platforms-and-browsers-0", "foo",
				"pr", "p", "platforms-and-browsers", false),
			`
spec:
  params:
  - name: browser
    value: chrome
  - name: platform
    value: linux
  - name: version
    value: v0.33.0
  serviceAccountName: test-sa
  taskRef:
    name: mytask
    kind: Task
`),
		mustParseTaskRunWithObjectMeta(t,
			taskRunObjectMeta("pr-platforms-and-browsers-1", "foo",
				"pr", "p", "platforms-and-browsers", false),
			`
spec:
  params:
  - name: browser
    value: chrome
  - name: platform
    value: mac
  - name: version
    value: v0.33.0
  serviceAccountName: test-sa
  taskRef:
    name: mytask
    kind: Task
`),
		mustParseTaskRunWithObjectMeta(t,
			taskRunObjectMeta("pr-platforms-and-browsers-2", "foo",
				"pr", "p", "platforms-and-browsers", false),
			`
spec:
  params:
  - name: browser
    value: chrome
  - name: platform
    value: windows
  - name: version
    value: v0.33.0
  serviceAccountName: test-sa
  taskRef:
    name: mytask
    kind: Task
`),
		mustParseTaskRunWithObjectMeta(t,
			taskRunObjectMeta("pr-platforms-and-browsers-3", "foo",
				"pr", "p", "platforms-and-browsers", false),
			`
spec:
  params:
  - name: browser
    value: safari
  - name: platform
    value: linux
  - name: version
    value: v0.33.0
  serviceAccountName: test-sa
  taskRef:
    name: mytask
    kind: Task
`),
		mustParseTaskRunWithObjectMeta(t,
			taskRunObjectMeta("pr-platforms-and-browsers-4", "foo",
				"pr", "p", "platforms-and-browsers", false),
			`
spec:
  params:
  - name: browser
    value: safari
  - name: platform
    value: mac
  - name: version
    value: v0.33.0
  serviceAccountName: test-sa
  taskRef:
    name: mytask
    kind: Task
`),
		mustParseTaskRunWithObjectMeta(t,
			taskRunObjectMeta("pr-platforms-and-browsers-5", "foo",
				"pr", "p", "platforms-and-browsers", false),
			`
spec:
  params:
  - name: browser
    value: safari
  - name: platform
    value: windows
  - name: version
    value: v0.33.0
  serviceAccountName: test-sa
  taskRef:
    name: mytask
    kind: Task
`),
		mustParseTaskRunWithObjectMeta(t,
			taskRunObjectMeta("pr-platforms-and-browsers-6", "foo",
				"pr", "p", "platforms-and-browsers", false),
			`
spec:
  params:
  - name: browser
    value: firefox
  - name: platform
    value: linux
  - name: version
    value: v0.33.0
  serviceAccountName: test-sa
  taskRef:
    name: mytask
    kind: Task
`),
		mustParseTaskRunWithObjectMeta(t,
			taskRunObjectMeta("pr-platforms-and-browsers-7", "foo",
				"pr", "p", "platforms-and-browsers", false),
			`
spec:
  params:
  - name: browser
    value: firefox
  - name: platform
    value: mac
  - name: version
    value: v0.33.0
  serviceAccountName: test-sa
  taskRef:
    name: mytask
    kind: Task
`),
		mustParseTaskRunWithObjectMeta(t,
			taskRunObjectMeta("pr-platforms-and-browsers-8", "foo",
				"pr", "p", "platforms-and-browsers", false),
			`
spec:
  params:
  - name: browser
    value: firefox
  - name: platform
    value: windows
  - name: version
    value: v0.33.0
  serviceAccountName: test-sa
  taskRef:
    name: mytask
    kind: Task
`),
	}
	cms := []*corev1.ConfigMap{withEnabledAlphaAPIFields(newFeatureFlagsConfigMap())}
	cms = append(cms, withMaxMatrixCombinationsCount(newDefaultsConfigMap(), 10))

	tests := []struct {
		name                string
		memberOf            string
		p                   *v1.Pipeline
		tr                  *v1.TaskRun
		expectedPipelineRun *v1.PipelineRun
	}{{
		name:     "p-dag",
		memberOf: "tasks",
		p: parse.MustParseV1Pipeline(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: foo
spec:
  tasks:
    - name: platforms-and-browsers
      taskRef:
        name: mytask
      matrix:
        params:
          - name: platform
            value:
              - linux
              - mac
              - windows
          - name: browser
            value:
              - chrome
              - safari
              - firefox
      params:
        - name: version
          value: v0.33.0
`, "p-dag")),
		expectedPipelineRun: parse.MustParseV1PipelineRun(t, `
metadata:
  name: pr
  namespace: foo
  annotations: {}
  labels:
    tekton.dev/pipeline: p-dag
spec:
  taskRunTemplate:
    serviceAccountName: test-sa
  pipelineRef:
    name: p-dag
status:
  pipelineSpec:
    tasks:
    - name: platforms-and-browsers
      taskRef:
        name: mytask
        kind: Task
      matrix:
        params:
          - name: platform
            value:
              - linux
              - mac
              - windows
          - name: browser
            value:
              - chrome
              - safari
              - firefox
      params:
        - name: version
          value: v0.33.0
  conditions:
  - type: Succeeded
    status: "Unknown"
    reason: "Running"
    message: "Tasks Completed: 0 (Failed: 0, Cancelled 0), Incomplete: 1, Skipped: 0"
  childReferences:
  - apiVersion: tekton.dev/v1
    kind: TaskRun
    name: pr-platforms-and-browsers-0
    pipelineTaskName: platforms-and-browsers
  - apiVersion: tekton.dev/v1
    kind: TaskRun
    name: pr-platforms-and-browsers-1
    pipelineTaskName: platforms-and-browsers
  - apiVersion: tekton.dev/v1
    kind: TaskRun
    name: pr-platforms-and-browsers-2
    pipelineTaskName: platforms-and-browsers
  - apiVersion: tekton.dev/v1
    kind: TaskRun
    name: pr-platforms-and-browsers-3
    pipelineTaskName: platforms-and-browsers
  - apiVersion: tekton.dev/v1
    kind: TaskRun
    name: pr-platforms-and-browsers-4
    pipelineTaskName: platforms-and-browsers
  - apiVersion: tekton.dev/v1
    kind: TaskRun
    name: pr-platforms-and-browsers-5
    pipelineTaskName: platforms-and-browsers
  - apiVersion: tekton.dev/v1
    kind: TaskRun
    name: pr-platforms-and-browsers-6
    pipelineTaskName: platforms-and-browsers
  - apiVersion: tekton.dev/v1
    kind: TaskRun
    name: pr-platforms-and-browsers-7
    pipelineTaskName: platforms-and-browsers
  - apiVersion: tekton.dev/v1
    kind: TaskRun
    name: pr-platforms-and-browsers-8
    pipelineTaskName: platforms-and-browsers
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
`),
	}, {
		name:     "p-finally",
		memberOf: "finally",
		p: parse.MustParseV1Pipeline(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: foo
spec:
  tasks:
    - name: unmatrixed-pt
      params:
        - name: platform
          value: linux
        - name: browser
          value: chrome
        - name: version
          value: v0.22.0
      taskRef:
        name: mytask
  finally:
    - name: platforms-and-browsers
      taskRef:
        name: mytask
      matrix:
        params:
          - name: platform
            value:
              - linux
              - mac
              - windows
          - name: browser
            value:
              - chrome
              - safari
              - firefox
      params:
        - name: version
          value: v0.33.0
`, "p-finally")),
		tr: mustParseTaskRunWithObjectMeta(t,
			taskRunObjectMeta("pr-unmatrixed-pt", "foo",
				"pr", "p-finally", "unmatrixed-pt", false),
			`
spec:
  params:
  - name: platform
    value: linux
  - name: browser
    value: chrome
  - name: version
    value: v0.22.0
  serviceAccountName: test-sa
  taskRef:
    name: mytask
    kind: Task
status:
 conditions:
  - type: Succeeded
    status: "True"
    reason: Succeeded
    message: All Tasks have completed executing
`),
		expectedPipelineRun: parse.MustParseV1PipelineRun(t, `
metadata:
  name: pr
  namespace: foo
  annotations: {}
  labels:
    tekton.dev/pipeline: p-finally
spec:
  taskRunTemplate:
    serviceAccountName: test-sa
  pipelineRef:
    name: p-finally
status:
  pipelineSpec:
    tasks:
    - name: unmatrixed-pt
      params:
        - name: platform
          value: linux
        - name: browser
          value: chrome
        - name: version
          value: v0.22.0
      taskRef:
        name: mytask
        kind: Task
    finally:
    - name: platforms-and-browsers
      taskRef:
        name: mytask
        kind: Task
      matrix:
        params:
          - name: platform
            value:
              - linux
              - mac
              - windows
          - name: browser
            value:
              - chrome
              - safari
              - firefox
      params:
        - name: version
          value: v0.33.0
  conditions:
  - type: Succeeded
    status: "Unknown"
    reason: "Running"
    message: "Tasks Completed: 1 (Failed: 0, Cancelled 0), Incomplete: 1, Skipped: 0"
  childReferences:
  - apiVersion: tekton.dev/v1
    kind: TaskRun
    name: pr-unmatrixed-pt
    pipelineTaskName: unmatrixed-pt
  - apiVersion: tekton.dev/v1
    kind: TaskRun
    name: pr-platforms-and-browsers-0
    pipelineTaskName: platforms-and-browsers
  - apiVersion: tekton.dev/v1
    kind: TaskRun
    name: pr-platforms-and-browsers-1
    pipelineTaskName: platforms-and-browsers
  - apiVersion: tekton.dev/v1
    kind: TaskRun
    name: pr-platforms-and-browsers-2
    pipelineTaskName: platforms-and-browsers
  - apiVersion: tekton.dev/v1
    kind: TaskRun
    name: pr-platforms-and-browsers-3
    pipelineTaskName: platforms-and-browsers
  - apiVersion: tekton.dev/v1
    kind: TaskRun
    name: pr-platforms-and-browsers-4
    pipelineTaskName: platforms-and-browsers
  - apiVersion: tekton.dev/v1
    kind: TaskRun
    name: pr-platforms-and-browsers-5
    pipelineTaskName: platforms-and-browsers
  - apiVersion: tekton.dev/v1
    kind: TaskRun
    name: pr-platforms-and-browsers-6
    pipelineTaskName: platforms-and-browsers
  - apiVersion: tekton.dev/v1
    kind: TaskRun
    name: pr-platforms-and-browsers-7
    pipelineTaskName: platforms-and-browsers
  - apiVersion: tekton.dev/v1
    kind: TaskRun
    name: pr-platforms-and-browsers-8
    pipelineTaskName: platforms-and-browsers
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
`),
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pr := parse.MustParseV1PipelineRun(t, fmt.Sprintf(`
metadata:
  name: pr
  namespace: foo
spec:
  taskRunTemplate:
    serviceAccountName: test-sa
  pipelineRef:
    name: %s
`, tt.name))
			d := test.Data{
				PipelineRuns: []*v1.PipelineRun{pr},
				Pipelines:    []*v1.Pipeline{tt.p},
				Tasks:        []*v1.Task{task},
				ConfigMaps:   cms,
			}
			if tt.tr != nil {
				d.TaskRuns = []*v1.TaskRun{tt.tr}
			}
			prt := newPipelineRunTest(t, d)
			defer prt.Cancel()

			_, clients := prt.reconcileRun("foo", "pr", []string{}, false)
			taskRuns, err := clients.Pipeline.TektonV1().TaskRuns("foo").List(prt.TestAssets.Ctx, metav1.ListOptions{
				LabelSelector: fmt.Sprintf("tekton.dev/pipelineRun=pr,tekton.dev/pipeline=%s,tekton.dev/pipelineTask=platforms-and-browsers", tt.name),
				Limit:         1,
			})
			if err != nil {
				t.Fatalf("Failure to list TaskRun's %s", err)
			}

			if len(taskRuns.Items) != 9 {
				t.Fatalf("Expected 9 TaskRuns got %d", len(taskRuns.Items))
			}

			for i := range taskRuns.Items {
				expectedTaskRun := expectedTaskRuns[i]
				expectedTaskRun.Labels["tekton.dev/pipeline"] = tt.name
				expectedTaskRun.Labels["tekton.dev/memberOf"] = tt.memberOf
				if d := cmp.Diff(expectedTaskRun, &taskRuns.Items[i], ignoreResourceVersion, ignoreTypeMeta); d != "" {
					t.Errorf("expected to see TaskRun %v created. Diff %s", expectedTaskRuns[i].Name, diff.PrintWantGot(d))
				}
			}

			pipelineRun, err := clients.Pipeline.TektonV1().PipelineRuns("foo").Get(prt.TestAssets.Ctx, "pr", metav1.GetOptions{})
			if err != nil {
				t.Fatalf("Got an error getting reconciled run out of fake client: %s", err)
			}
			if d := cmp.Diff(tt.expectedPipelineRun, pipelineRun, ignoreResourceVersion, ignoreTypeMeta, ignoreLastTransitionTime, ignoreStartTime, ignoreFinallyStartTime, cmpopts.EquateEmpty()); d != "" {
				t.Errorf("expected PipelineRun was not created. Diff %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestReconciler_PipelineTaskMatrixWithArrayReferences(t *testing.T) {
	names.TestingSeed()

	task := parse.MustParseV1Task(t, `
metadata:
  name: mytask
  namespace: foo
spec:
  params:
    - name: platform
    - name: browser
  steps:
    - name: echo
      image: alpine
      script: |
        echo "$(params.platform) and $(params.browser)"
`)

	cms := []*corev1.ConfigMap{withEnabledAlphaAPIFields(newFeatureFlagsConfigMap())}
	cms = append(cms, withMaxMatrixCombinationsCount(newDefaultsConfigMap(), 10))

	tests := []struct {
		name                string
		memberOf            string
		p                   *v1.Pipeline
		tr                  *v1.TaskRun
		expectedPipelineRun *v1.PipelineRun
	}{{
		name: "p-dag",
		p: parse.MustParseV1Pipeline(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: foo
spec:
  params:
    - name: platforms
      type: array
    - name: browsers
      type: array
  tasks:
    - name: platforms-and-browsers
      taskRef:
        name: mytask
      matrix:
        params:
          - name: platform
            value: $(params.platforms[*])
          - name: browser
            value: $(params.browsers[*])

`, "p-dag")),
		expectedPipelineRun: parse.MustParseV1PipelineRun(t, `
metadata:
  name: pr
  namespace: foo
  annotations: {}
  labels:
    tekton.dev/pipeline: p-dag
spec:
  params:
   - name: platforms
     value:
      - linux
      - mac
      - windows
   - name: browsers
     value:
      - chrome
      - safari
      - firefox
  taskRunTemplate:
    serviceAccountName: test-sa
  pipelineRef:
    name: p-dag
status:
  pipelineSpec:
    params:
     - name: platforms
       type: array
     - name: browsers
       type: array
    tasks:
      - name: platforms-and-browsers
        matrix:
          params:
            - name: platform
              value:
               - linux
               -  mac
               -  windows
            - name: browser
              value:
               - chrome
               - safari
               - firefox
        taskRef:
          name: mytask
          kind: Task
  conditions:
  - type: Succeeded
    status: "Unknown"
    reason: "Running"
    message: "Tasks Completed: 0 (Failed: 0, Cancelled 0), Incomplete: 1, Skipped: 0"
  childReferences:
  - apiVersion: tekton.dev/v1
    kind: TaskRun
    name: pr-platforms-and-browsers-0
    pipelineTaskName: platforms-and-browsers
  - apiVersion: tekton.dev/v1
    kind: TaskRun
    name: pr-platforms-and-browsers-1
    pipelineTaskName: platforms-and-browsers
  - apiVersion: tekton.dev/v1
    kind: TaskRun
    name: pr-platforms-and-browsers-2
    pipelineTaskName: platforms-and-browsers
  - apiVersion: tekton.dev/v1
    kind: TaskRun
    name: pr-platforms-and-browsers-3
    pipelineTaskName: platforms-and-browsers
  - apiVersion: tekton.dev/v1
    kind: TaskRun
    name: pr-platforms-and-browsers-4
    pipelineTaskName: platforms-and-browsers
  - apiVersion: tekton.dev/v1
    kind: TaskRun
    name: pr-platforms-and-browsers-5
    pipelineTaskName: platforms-and-browsers
  - apiVersion: tekton.dev/v1
    kind: TaskRun
    name: pr-platforms-and-browsers-6
    pipelineTaskName: platforms-and-browsers
  - apiVersion: tekton.dev/v1
    kind: TaskRun
    name: pr-platforms-and-browsers-7
    pipelineTaskName: platforms-and-browsers
  - apiVersion: tekton.dev/v1
    kind: TaskRun
    name: pr-platforms-and-browsers-8
    pipelineTaskName: platforms-and-browsers
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
`),
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pr := parse.MustParseV1PipelineRun(t, fmt.Sprintf(`
metadata:
  name: pr
  namespace: foo
spec:
  taskRunTemplate:
    serviceAccountName: test-sa
  params:
  - name: platforms
    value:
     - linux
     - mac
     - windows
  - name: browsers
    value:
     - chrome
     - safari
     - firefox
  pipelineRef:
    name: %s
`, tt.name))
			d := test.Data{
				PipelineRuns: []*v1.PipelineRun{pr},
				Pipelines:    []*v1.Pipeline{tt.p},
				Tasks:        []*v1.Task{task},
				ConfigMaps:   cms,
			}
			if tt.tr != nil {
				d.TaskRuns = []*v1.TaskRun{tt.tr}
			}

			expectedTaskRunsData := []struct {
				browser  string
				platform string
			}{{
				browser:  "chrome",
				platform: "linux",
			}, {
				browser:  "chrome",
				platform: "mac",
			}, {
				browser:  "chrome",
				platform: "windows",
			}, {
				browser:  "safari",
				platform: "linux",
			}, {
				browser:  "safari",
				platform: "mac",
			}, {
				browser:  "safari",
				platform: "windows",
			}, {
				browser:  "firefox",
				platform: "linux",
			}, {
				browser:  "firefox",
				platform: "mac",
			}, {
				browser:  "firefox",
				platform: "windows",
			}}

			expectedTaskRuns := []*v1.TaskRun{}
			for i, trd := range expectedTaskRunsData {
				trName := "pr-platforms-and-browsers-" + strconv.Itoa(i)
				expectedTaskRuns = append(expectedTaskRuns, mustParseTaskRunWithObjectMeta(t,
					taskRunObjectMeta(trName, "foo", "pr", "p-dag", "platforms-and-browsers", false),
					fmt.Sprintf(`
spec:
  params:
  - name: browser
    value: %s
  - name: platform
    value: %s
  serviceAccountName: test-sa
  taskRef:
    name: mytask
    kind: Task
labels:
  tekton.dev/memberOf: tasks
  tekton.dev/pipeline: p-dag
`, trd.browser, trd.platform)))
			}

			prt := newPipelineRunTest(t, d)
			defer prt.Cancel()

			pipelineRun, clients := prt.reconcileRun(pr.Namespace, pr.Name /*wantEvents*/, []string{} /*permanentError*/, false)
			taskRuns := getTaskRunsForPipelineRun(prt.TestAssets.Ctx, t, clients, pr.Namespace, pr.Name)
			validateTaskRunsCount(t, taskRuns, len(expectedTaskRuns))

			for i, expectedTaskRun := range expectedTaskRuns {
				trName := expectedTaskRun.Name
				actual := getTaskRunByName(t, taskRuns, trName)
				if d := cmp.Diff(expectedTaskRun, actual, ignoreResourceVersion, ignoreTypeMeta); d != "" {
					t.Errorf("expected to see TaskRun %v created. Diff %s", expectedTaskRuns[i].Name, diff.PrintWantGot(d))
				}
			}

			if d := cmp.Diff(tt.expectedPipelineRun, pipelineRun, ignoreResourceVersion, ignoreTypeMeta, ignoreLastTransitionTime, ignoreStartTime, ignoreFinallyStartTime, cmpopts.EquateEmpty()); d != "" {
				t.Errorf("found PipelineRun does not match expected PipelineRun. Diff %s", diff.PrintWantGot(d))
			}
		})
	}
}
func TestReconciler_PipelineTaskIncludeParams(t *testing.T) {
	names.TestingSeed()

	task := parse.MustParseV1Task(t, `
metadata:
  name: mytask
  namespace: foo
spec:
  params:
    - name: GOARCH
    - name: version
    - name: flags
      default: ""
    - name: context
      default: ""
    - name: package
      default: ""
  steps:
    - name: echo
      image: alpine
      script: |
        echo "$(params.GOARCH) and $(params.version)"
`)

	expectedTaskRuns := []*v1.TaskRun{
		mustParseTaskRunWithObjectMeta(t,
			taskRunObjectMeta("pr-matrix-include-0", "foo",
				"pr", "p", "matrix-include", false),
			`
spec:
  params:
  - name: GOARCH
    value: linux/amd64
  - name: context
    value: path/to/go117/context
  - name: package
    value: path/to/common/package/
  - name: version
    value: go1.17
  serviceAccountName: test-sa
  taskRef:
    name: mytask
    kind: Task
`),
		mustParseTaskRunWithObjectMeta(t,
			taskRunObjectMeta("pr-matrix-include-1", "foo",
				"pr", "p", "matrix-include", false),
			`
spec:
  params:
  - name: GOARCH
    value: linux/ppc64le
  - name: context
    value: path/to/go117/context
  - name: package
    value: path/to/common/package/
  - name: version
    value: go1.17
  serviceAccountName: test-sa
  taskRef:
    name: mytask
    kind: Task
`),
		mustParseTaskRunWithObjectMeta(t,
			taskRunObjectMeta("pr-matrix-include-2", "foo",
				"pr", "p", "matrix-include", false),
			`
spec:
  params:
  - name: GOARCH
    value: linux/s390x
  - name: context
    value: path/to/go117/context
  - name: flags
    value: -cover -v
  - name: package
    value: path/to/common/package/
  - name: version
    value: go1.17
  serviceAccountName: test-sa
  taskRef:
    name: mytask
    kind: Task
`),
		mustParseTaskRunWithObjectMeta(t,
			taskRunObjectMeta("pr-matrix-include-3", "foo",
				"pr", "p", "matrix-include", false),
			`
spec:
  params:
  - name: GOARCH
    value: linux/amd64
  - name: package
    value: path/to/common/package/
  - name: version
    value: go1.18.1
  serviceAccountName: test-sa
  taskRef:
    name: mytask
    kind: Task
`),
		mustParseTaskRunWithObjectMeta(t,
			taskRunObjectMeta("pr-matrix-include-4", "foo",
				"pr", "p", "matrix-include", false),
			`
spec:
  params:
  - name: GOARCH
    value: linux/ppc64le
  - name: package
    value: path/to/common/package/
  - name: version
    value: go1.18.1
  serviceAccountName: test-sa
  taskRef:
    name: mytask
    kind: Task
`),
		mustParseTaskRunWithObjectMeta(t,
			taskRunObjectMeta("pr-matrix-include-5", "foo",
				"pr", "p", "matrix-include", false),
			`
spec:
  params:
  - name: GOARCH
    value: linux/s390x
  - name: flags
    value: -cover -v
  - name: package
    value: path/to/common/package/
  - name: version
    value: go1.18.1
  serviceAccountName: test-sa
  taskRef:
    name: mytask
    kind: Task
`),
		mustParseTaskRunWithObjectMeta(t,
			taskRunObjectMeta("pr-matrix-include-6", "foo",
				"pr", "p", "matrix-include", false),
			`
spec:
  params:
  - name: GOARCH
    value: I-do-not-exist
  serviceAccountName: test-sa
  taskRef:
    name: mytask
    kind: Task
`),
	}
	cms := []*corev1.ConfigMap{withEnabledAlphaAPIFields(newFeatureFlagsConfigMap())}
	cms = append(cms, withMaxMatrixCombinationsCount(newDefaultsConfigMap(), 10))

	tests := []struct {
		name                string
		memberOf            string
		p                   *v1.Pipeline
		tr                  *v1.TaskRun
		expectedPipelineRun *v1.PipelineRun
	}{{
		name:     "p-dag",
		memberOf: "tasks",
		p: parse.MustParseV1Pipeline(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: foo
spec:
  tasks:
    - name: matrix-include
      taskRef:
        name: mytask
      matrix:
        params:
          - name: GOARCH
            value:
              - linux/amd64
              - linux/ppc64le
              - linux/s390x
          - name: version
            value:
              - go1.17
              - go1.18.1
        include:
         - name: common-package
           params:
            - name: package
              value: path/to/common/package/
         - name: s390x-no-race
           params:
           - name: GOARCH
             value: linux/s390x
           - name: flags
             value: '-cover -v'
         - name: go117-context
           params:
            - name: version
              value: go1.17
            - name: context
              value: path/to/go117/context
         - name: non-existent-arch
           params:
            - name: GOARCH
              value: I-do-not-exist
`, "p-dag")),
		expectedPipelineRun: parse.MustParseV1PipelineRun(t, `
metadata:
  name: pr
  namespace: foo
  annotations: {}
  labels:
    tekton.dev/pipeline: p-dag
spec:
  taskRunTemplate:
    serviceAccountName: test-sa
  pipelineRef:
    name: p-dag
status:
  pipelineSpec:
    tasks:
    - name: matrix-include
      taskRef:
        name: mytask
        kind: Task
      matrix:
        params:
          - name: GOARCH
            value:
              - linux/amd64
              - linux/ppc64le
              - linux/s390x
          - name: version
            value:
              - go1.17
              - go1.18.1
        include:
         - name: common-package
           params:
            - name: package
              value: path/to/common/package/
         - name: s390x-no-race
           params:
           - name: GOARCH
             value: linux/s390x
           - name: flags
             value: '-cover -v'
         - name: go117-context
           params:
            - name: version
              value: go1.17
            - name: context
              value: path/to/go117/context
         - name: non-existent-arch
           params:
            - name: GOARCH
              value: I-do-not-exist
  conditions:
  - type: Succeeded
    status: "Unknown"
    reason: "Running"
    message: "Tasks Completed: 0 (Failed: 0, Cancelled 0), Incomplete: 1, Skipped: 0"
  childReferences:
  - apiVersion: tekton.dev/v1
    kind: TaskRun
    name: pr-matrix-include-0
    pipelineTaskName: matrix-include
  - apiVersion: tekton.dev/v1
    kind: TaskRun
    name: pr-matrix-include-1
    pipelineTaskName: matrix-include
  - apiVersion: tekton.dev/v1
    kind: TaskRun
    name: pr-matrix-include-2
    pipelineTaskName: matrix-include
  - apiVersion: tekton.dev/v1
    kind: TaskRun
    name: pr-matrix-include-3
    pipelineTaskName: matrix-include
  - apiVersion: tekton.dev/v1
    kind: TaskRun
    name: pr-matrix-include-4
    pipelineTaskName: matrix-include
  - apiVersion: tekton.dev/v1
    kind: TaskRun
    name: pr-matrix-include-5
    pipelineTaskName: matrix-include
  - apiVersion: tekton.dev/v1
    kind: TaskRun
    name: pr-matrix-include-6
    pipelineTaskName: matrix-include
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
`),
	}, {
		name:     "p-finally",
		memberOf: "finally",
		p: parse.MustParseV1Pipeline(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: foo
spec:
  tasks:
    - name: unmatrixed-pt
      params:
      - name: GOARCH
        value: "test"
      - name: version
        value: "test"
      - name: flags
        value: "test"
      - name: context
        value: "test"
      - name: package
        value: "test"
      taskRef:
        name: mytask
  finally:
    - name: matrix-include
      taskRef:
        name: mytask
      matrix:
        params:
          - name: GOARCH
            value:
              - linux/amd64
              - linux/ppc64le
              - linux/s390x
          - name: version
            value:
              - go1.17
              - go1.18.1
        include:
         - name: common-package
           params:
            - name: package
              value: path/to/common/package/
         - name: s390x-no-race
           params:
           - name: GOARCH
             value: linux/s390x
           - name: flags
             value: '-cover -v'
         - name: go117-context
           params:
            - name: version
              value: go1.17
            - name: context
              value: path/to/go117/context
         - name: non-existent-arch
           params:
            - name: GOARCH
              value: I-do-not-exist
`, "p-finally")),
		tr: mustParseTaskRunWithObjectMeta(t,
			taskRunObjectMeta("pr-unmatrixed-pt", "foo",
				"pr", "p-finally", "unmatrixed-pt", false),
			`
spec:
  params:
  - name: GOARCH
    value: "test"
  - name: version
    value: "test"
  - name: flags
    value: "test"
  - name: context
    value: "test"
  - name: package
    value: "test"
  serviceAccountName: test-sa
  taskRef:
    name: mytask
    kind: Task
status:
 conditions:
  - type: Succeeded
    status: "True"
    reason: Succeeded
    message: All Tasks have completed executing
`),
		expectedPipelineRun: parse.MustParseV1PipelineRun(t, `
metadata:
  name: pr
  namespace: foo
  annotations: {}
  labels:
    tekton.dev/pipeline: p-finally
spec:
  taskRunTemplate:
    serviceAccountName: test-sa
  pipelineRef:
    name: p-finally
status:
  pipelineSpec:
    tasks:
    - name: unmatrixed-pt
      params:
       - name: GOARCH
         value: "test"
       - name: version
         value: "test"
       - name: flags
         value: "test"
       - name: context
         value: "test"
       - name: package
         value: "test"
      taskRef:
        name: mytask
        kind: Task
    finally:
    - name: matrix-include
      taskRef:
        name: mytask
        kind: Task
      matrix:
        params:
          - name: GOARCH
            value:
              - linux/amd64
              - linux/ppc64le
              - linux/s390x
          - name: version
            value:
              - go1.17
              - go1.18.1
        include:
         - name: common-package
           params:
            - name: package
              value: path/to/common/package/
         - name: s390x-no-race
           params:
           - name: GOARCH
             value: linux/s390x
           - name: flags
             value: '-cover -v'
         - name: go117-context
           params:
            - name: version
              value: go1.17
            - name: context
              value: path/to/go117/context
         - name: non-existent-arch
           params:
            - name: GOARCH
              value: I-do-not-exist
  conditions:
  - type: Succeeded
    status: "Unknown"
    reason: "Running"
    message: "Tasks Completed: 1 (Failed: 0, Cancelled 0), Incomplete: 1, Skipped: 0"
  childReferences:
  - apiVersion: tekton.dev/v1
    kind: TaskRun
    name: pr-unmatrixed-pt
    pipelineTaskName: unmatrixed-pt
  - apiVersion: tekton.dev/v1
    kind: TaskRun
    name: pr-matrix-include-0
    pipelineTaskName: matrix-include
  - apiVersion: tekton.dev/v1
    kind: TaskRun
    name: pr-matrix-include-1
    pipelineTaskName: matrix-include
  - apiVersion: tekton.dev/v1
    kind: TaskRun
    name: pr-matrix-include-2
    pipelineTaskName: matrix-include
  - apiVersion: tekton.dev/v1
    kind: TaskRun
    name: pr-matrix-include-3
    pipelineTaskName: matrix-include
  - apiVersion: tekton.dev/v1
    kind: TaskRun
    name: pr-matrix-include-4
    pipelineTaskName: matrix-include
  - apiVersion: tekton.dev/v1
    kind: TaskRun
    name: pr-matrix-include-5
    pipelineTaskName: matrix-include
  - apiVersion: tekton.dev/v1
    kind: TaskRun
    name: pr-matrix-include-6
    pipelineTaskName: matrix-include
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
`),
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pr := parse.MustParseV1PipelineRun(t, fmt.Sprintf(`
metadata:
  name: pr
  namespace: foo
spec:
  taskRunTemplate:
    serviceAccountName: test-sa
  pipelineRef:
    name: %s
`, tt.name))
			d := test.Data{
				PipelineRuns: []*v1.PipelineRun{pr},
				Pipelines:    []*v1.Pipeline{tt.p},
				Tasks:        []*v1.Task{task},
				ConfigMaps:   cms,
			}
			if tt.tr != nil {
				d.TaskRuns = []*v1.TaskRun{tt.tr}
			}
			prt := newPipelineRunTest(t, d)
			defer prt.Cancel()

			_, clients := prt.reconcileRun("foo", "pr", []string{}, false)
			taskRuns, err := clients.Pipeline.TektonV1().TaskRuns("foo").List(prt.TestAssets.Ctx, metav1.ListOptions{
				LabelSelector: fmt.Sprintf("tekton.dev/pipelineRun=pr,tekton.dev/pipeline=%s,tekton.dev/pipelineTask=matrix-include", tt.name),
				Limit:         1,
			})
			if err != nil {
				t.Fatalf("Failure to list TaskRun's %s", err)
			}

			if len(taskRuns.Items) != 7 {
				t.Fatalf("Expected 7 TaskRuns got %d", len(taskRuns.Items))
			}

			for i := range taskRuns.Items {
				expectedTaskRun := expectedTaskRuns[i]
				expectedTaskRun.Labels["tekton.dev/pipeline"] = tt.name
				expectedTaskRun.Labels["tekton.dev/memberOf"] = tt.memberOf
				if d := cmp.Diff(expectedTaskRun, &taskRuns.Items[i], ignoreResourceVersion, ignoreTypeMeta); d != "" {
					t.Errorf("expected to see TaskRun %v created. Diff %s", expectedTaskRuns[i].Name, diff.PrintWantGot(d))
				}
			}

			pipelineRun, err := clients.Pipeline.TektonV1().PipelineRuns("foo").Get(prt.TestAssets.Ctx, "pr", metav1.GetOptions{})
			if err != nil {
				t.Fatalf("Got an error getting reconciled run out of fake client: %s", err)
			}

			if d := cmp.Diff(tt.expectedPipelineRun, pipelineRun, ignoreResourceVersion, ignoreTypeMeta, ignoreLastTransitionTime, ignoreStartTime, ignoreFinallyStartTime, cmpopts.EquateEmpty()); d != "" {
				t.Errorf("expected PipelineRun was not created. Diff %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestReconciler_PipelineTaskExplicitCombos(t *testing.T) {
	names.TestingSeed()

	task := parse.MustParseV1Task(t, `
metadata:
  name: mytask
  namespace: foo
spec:
  params:
    - name: IMAGE
    - name: DOCKERFILE
  steps:
    - name: echo
      image: alpine
      script: |
        echo "$(params.IMAGE) and $(params.DOCKERFILE)"
`)

	expectedTaskRuns := []*v1.TaskRun{
		mustParseTaskRunWithObjectMeta(t,
			taskRunObjectMeta("pr-matrix-include-0", "foo",
				"pr", "p", "matrix-include", false),
			`
spec:
  params:
  - name: DOCKERFILE
    value: path/to/Dockerfile1
  - name: IMAGE
    value: image-1
  serviceAccountName: test-sa
  taskRef:
    name: mytask
    kind: Task
`),
		mustParseTaskRunWithObjectMeta(t,
			taskRunObjectMeta("pr-matrix-include-1", "foo",
				"pr", "p", "matrix-include", false),
			`
spec:
  params:
  - name: DOCKERFILE
    value: path/to/Dockerfile2
  - name: IMAGE
    value: image-2
  serviceAccountName: test-sa
  taskRef:
    name: mytask
    kind: Task
`),
		mustParseTaskRunWithObjectMeta(t,
			taskRunObjectMeta("pr-matrix-include-2", "foo",
				"pr", "p", "matrix-include", false),
			`
spec:
  params:
  - name: DOCKERFILE
    value: path/to/Dockerfile3
  - name: IMAGE
    value: image-3
  serviceAccountName: test-sa
  taskRef:
    name: mytask
    kind: Task
`),
	}
	cms := []*corev1.ConfigMap{withEnabledAlphaAPIFields(newFeatureFlagsConfigMap())}
	cms = append(cms, withMaxMatrixCombinationsCount(newDefaultsConfigMap(), 10))

	tests := []struct {
		name                string
		memberOf            string
		p                   *v1.Pipeline
		tr                  *v1.TaskRun
		expectedPipelineRun *v1.PipelineRun
	}{{
		name:     "p-dag",
		memberOf: "tasks",
		p: parse.MustParseV1Pipeline(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: foo
spec:
  tasks:
    - name: matrix-include
      taskRef:
        name: mytask
      matrix:
        include:
         - name: build-1
           params:
            - name: IMAGE
              value: image-1
            - name: DOCKERFILE
              value: path/to/Dockerfile1
         - name: build-2
           params:
            - name: IMAGE
              value: image-2
            - name: DOCKERFILE
              value: path/to/Dockerfile2
         - name: build-3
           params:
            - name: IMAGE
              value: image-3
            - name: DOCKERFILE
              value: path/to/Dockerfile3
`, "p-dag")),
		expectedPipelineRun: parse.MustParseV1PipelineRun(t, `
metadata:
  name: pr
  namespace: foo
  annotations: {}
  labels:
    tekton.dev/pipeline: p-dag
spec:
  taskRunTemplate:
    serviceAccountName: test-sa
  pipelineRef:
    name: p-dag
status:
  pipelineSpec:
    tasks:
      - name: matrix-include
        taskRef:
          name: mytask
          kind: Task
        matrix:
          include:
           - name: build-1
             params:
              - name: IMAGE
                value: image-1
              - name: DOCKERFILE
                value: path/to/Dockerfile1
           - name: build-2
             params:
              - name: IMAGE
                value: image-2
              - name: DOCKERFILE
                value: path/to/Dockerfile2
           - name: build-3
             params:
              - name: IMAGE
                value: image-3
              - name: DOCKERFILE
                value: path/to/Dockerfile3
  conditions:
  - type: Succeeded
    status: "Unknown"
    reason: "Running"
    message: "Tasks Completed: 0 (Failed: 0, Cancelled 0), Incomplete: 1, Skipped: 0"
  childReferences:
  - apiVersion: tekton.dev/v1
    kind: TaskRun
    name: pr-matrix-include-0
    pipelineTaskName: matrix-include
  - apiVersion: tekton.dev/v1
    kind: TaskRun
    name: pr-matrix-include-1
    pipelineTaskName: matrix-include
  - apiVersion: tekton.dev/v1
    kind: TaskRun
    name: pr-matrix-include-2
    pipelineTaskName: matrix-include
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
`),
	},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pr := parse.MustParseV1PipelineRun(t, fmt.Sprintf(`
metadata:
  name: pr
  namespace: foo
spec:
  taskRunTemplate:
    serviceAccountName: test-sa
  pipelineRef:
    name: %s
`, tt.name))
			d := test.Data{
				PipelineRuns: []*v1.PipelineRun{pr},
				Pipelines:    []*v1.Pipeline{tt.p},
				Tasks:        []*v1.Task{task},
				ConfigMaps:   cms,
			}
			if tt.tr != nil {
				d.TaskRuns = []*v1.TaskRun{tt.tr}
			}
			prt := newPipelineRunTest(t, d)
			defer prt.Cancel()

			_, clients := prt.reconcileRun("foo", "pr", []string{}, false)
			taskRuns, err := clients.Pipeline.TektonV1().TaskRuns("foo").List(prt.TestAssets.Ctx, metav1.ListOptions{
				LabelSelector: fmt.Sprintf("tekton.dev/pipelineRun=pr,tekton.dev/pipeline=%s,tekton.dev/pipelineTask=matrix-include", tt.name),
				Limit:         1,
			})
			if err != nil {
				t.Fatalf("Failure to list TaskRun's %s", err)
			}

			if len(taskRuns.Items) != 3 {
				t.Fatalf("Expected 3 TaskRuns got %d", len(taskRuns.Items))
			}

			for i := range taskRuns.Items {
				expectedTaskRun := expectedTaskRuns[i]
				expectedTaskRun.Labels["tekton.dev/pipeline"] = tt.name
				expectedTaskRun.Labels["tekton.dev/memberOf"] = tt.memberOf
				if d := cmp.Diff(expectedTaskRun, &taskRuns.Items[i], ignoreResourceVersion, ignoreTypeMeta); d != "" {
					t.Errorf("expected to see TaskRun %v created. Diff %s", expectedTaskRuns[i].Name, diff.PrintWantGot(d))
				}
			}

			pipelineRun, err := clients.Pipeline.TektonV1().PipelineRuns("foo").Get(prt.TestAssets.Ctx, "pr", metav1.GetOptions{})
			if err != nil {
				t.Fatalf("Got an error getting reconciled run out of fake client: %s", err)
			}

			if d := cmp.Diff(tt.expectedPipelineRun, pipelineRun, ignoreResourceVersion, ignoreTypeMeta, ignoreLastTransitionTime, ignoreStartTime, ignoreFinallyStartTime, cmpopts.EquateEmpty()); d != "" {
				t.Errorf("expected PipelineRun was not created. Diff %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestReconciler_PipelineTaskMatrixWithResults(t *testing.T) {
	names.TestingSeed()

	task := parse.MustParseV1Task(t, `
metadata:
  name: mytask
  namespace: foo
spec:
  params:
    - name: platform
    - name: browser
    - name: version
  steps:
    - name: echo
      image: alpine
      script: |
        echo "$(params.platform) and $(params.browser) and $(params.version)"
`)

	taskwithresults := parse.MustParseV1Task(t, `
metadata:
  name: taskwithresults
  namespace: foo
spec:
  results:
   - name: platform-1
   - name: platform-2
   - name: platform-3
   - name: browser-1
   - name: browser-2
   - name: browser-3
   - name: version
  steps:
    - name: echo
      image: alpine
      script: |
        printf linux | tee /tekton/results/platform-1
        printf mac | tee /tekton/results/platform-2
        printf windows | tee /tekton/results/platform-3
        printf chrome | tee /tekton/results/browser-1
        printf safari | tee /tekton/results/browser-2
        printf firefox | tee /tekton/results/browser-3
        printf v0.33.0 | tee /tekton/results/version
`)

	expectedTaskRuns := []*v1.TaskRun{
		mustParseTaskRunWithObjectMeta(t,
			taskRunObjectMeta("pr-platforms-and-browsers-0", "foo",
				"pr", "p", "platforms-and-browsers", false),
			`
spec:
  params:
  - name: browser
    value: chrome
  - name: platform
    value: linux
  - name: version
    value: v0.33.0
  serviceAccountName: test-sa
  taskRef:
    name: mytask
    kind: Task
`),
		mustParseTaskRunWithObjectMeta(t,
			taskRunObjectMeta("pr-platforms-and-browsers-1", "foo",
				"pr", "p", "platforms-and-browsers", false),
			`
spec:
  params:
  - name: browser
    value: chrome
  - name: platform
    value: mac
  - name: version
    value: v0.33.0
  serviceAccountName: test-sa
  taskRef:
    name: mytask
    kind: Task
`),
		mustParseTaskRunWithObjectMeta(t,
			taskRunObjectMeta("pr-platforms-and-browsers-2", "foo",
				"pr", "p", "platforms-and-browsers", false),
			`
spec:
  params:
  - name: browser
    value: chrome
  - name: platform
    value: windows
  - name: version
    value: v0.33.0
  serviceAccountName: test-sa
  taskRef:
    name: mytask
    kind: Task
`),
		mustParseTaskRunWithObjectMeta(t,
			taskRunObjectMeta("pr-platforms-and-browsers-3", "foo",
				"pr", "p", "platforms-and-browsers", false),
			`
spec:
  params:
  - name: browser
    value: safari
  - name: platform
    value: linux
  - name: version
    value: v0.33.0
  serviceAccountName: test-sa
  taskRef:
    name: mytask
    kind: Task
`),
		mustParseTaskRunWithObjectMeta(t,
			taskRunObjectMeta("pr-platforms-and-browsers-4", "foo",
				"pr", "p", "platforms-and-browsers", false),
			`
spec:
  params:
  - name: browser
    value: safari
  - name: platform
    value: mac
  - name: version
    value: v0.33.0
  serviceAccountName: test-sa
  taskRef:
    name: mytask
    kind: Task
`),
		mustParseTaskRunWithObjectMeta(t,
			taskRunObjectMeta("pr-platforms-and-browsers-5", "foo",
				"pr", "p", "platforms-and-browsers", false),
			`
spec:
  params:
  - name: browser
    value: safari
  - name: platform
    value: windows
  - name: version
    value: v0.33.0
  serviceAccountName: test-sa
  taskRef:
    name: mytask
    kind: Task
`),
		mustParseTaskRunWithObjectMeta(t,
			taskRunObjectMeta("pr-platforms-and-browsers-6", "foo",
				"pr", "p", "platforms-and-browsers", false),
			`
spec:
  params:
  - name: browser
    value: firefox
  - name: platform
    value: linux
  - name: version
    value: v0.33.0
  serviceAccountName: test-sa
  taskRef:
    name: mytask
    kind: Task
`),
		mustParseTaskRunWithObjectMeta(t,
			taskRunObjectMeta("pr-platforms-and-browsers-7", "foo",
				"pr", "p", "platforms-and-browsers", false),
			`
spec:
  params:
  - name: browser
    value: firefox
  - name: platform
    value: mac
  - name: version
    value: v0.33.0
  serviceAccountName: test-sa
  taskRef:
    name: mytask
    kind: Task
`),
		mustParseTaskRunWithObjectMeta(t,
			taskRunObjectMeta("pr-platforms-and-browsers-8", "foo",
				"pr", "p", "platforms-and-browsers", false),
			`
spec:
  params:
  - name: browser
    value: firefox
  - name: platform
    value: windows
  - name: version
    value: v0.33.0
  serviceAccountName: test-sa
  taskRef:
    name: mytask
    kind: Task
`),
	}
	cms := []*corev1.ConfigMap{withEnabledAlphaAPIFields(newFeatureFlagsConfigMap())}
	cms = append(cms, withMaxMatrixCombinationsCount(newDefaultsConfigMap(), 10))

	tests := []struct {
		name                string
		memberOf            string
		p                   *v1.Pipeline
		tr                  *v1.TaskRun
		expectedPipelineRun *v1.PipelineRun
	}{{
		name:     "p-dag",
		memberOf: "tasks",
		p: parse.MustParseV1Pipeline(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: foo
spec:
  tasks:
    - name: pt-with-result
      params:
        - name: platform
          value: linux
        - name: browser
          value: chrome
        - name: version
          value: v0.22.0
      taskRef:
        name: taskwithresults
        kind: Task
    - name: platforms-and-browsers
      taskRef:
        name: mytask
        kind: Task
      matrix:
        params:
          - name: platform
            value:
              - $(tasks.pt-with-result.results.platform-1)
              - $(tasks.pt-with-result.results.platform-2)
              - $(tasks.pt-with-result.results.platform-3)
          - name: browser
            value:
              - $(tasks.pt-with-result.results.browser-1)
              - $(tasks.pt-with-result.results.browser-2)
              - $(tasks.pt-with-result.results.browser-3)
      params:
        - name: version
          value: $(tasks.pt-with-result.results.version)
`, "p-dag")),
		tr: mustParseTaskRunWithObjectMeta(t,
			taskRunObjectMeta("pr-pt-with-result", "foo",
				"pr", "p-dag", "pt-with-result", false),
			`
spec:
  serviceAccountName: test-sa
  taskRef:
    name: taskwithresults
  timeout: 1h0m0s
status:
 conditions:
  - type: Succeeded
    status: "True"
    reason: Succeeded
    message: All Tasks have completed executing
 results:
  - name: platform-1
    value: linux
  - name: platform-2
    value: mac
  - name: platform-3
    value: windows
  - name: browser-1
    value: chrome
  - name: browser-2
    value: safari
  - name: browser-3
    value: firefox
  - name: version
    value: v0.33.0
`),
		expectedPipelineRun: parse.MustParseV1PipelineRun(t, `
metadata:
  name: pr
  namespace: foo
  annotations: {}
  labels:
    tekton.dev/pipeline: p-dag
spec:
  taskRunTemplate:
    serviceAccountName: test-sa
  pipelineRef:
    name: p-dag
status:
  pipelineSpec:
    tasks:
    - name: pt-with-result
      params:
        - name: platform
          value: linux
        - name: browser
          value: chrome
        - name: version
          value: v0.22.0
      taskRef:
        name: taskwithresults
        kind: Task
    - name: platforms-and-browsers
      taskRef:
        name: mytask
        kind: Task
      matrix:
        params:
          - name: platform
            value:
              - $(tasks.pt-with-result.results.platform-1)
              - $(tasks.pt-with-result.results.platform-2)
              - $(tasks.pt-with-result.results.platform-3)
          - name: browser
            value:
              - $(tasks.pt-with-result.results.browser-1)
              - $(tasks.pt-with-result.results.browser-2)
              - $(tasks.pt-with-result.results.browser-3)
      params:
        - name: version
          value: $(tasks.pt-with-result.results.version)
  conditions:
  - type: Succeeded
    status: "Unknown"
    reason: "Running"
    message: "Tasks Completed: 1 (Failed: 0, Cancelled 0), Incomplete: 1, Skipped: 0"
  childReferences:
  - apiVersion: tekton.dev/v1
    kind: TaskRun
    name: pr-pt-with-result
    pipelineTaskName: pt-with-result
  - apiVersion: tekton.dev/v1
    kind: TaskRun
    name: pr-platforms-and-browsers-0
    pipelineTaskName: platforms-and-browsers
  - apiVersion: tekton.dev/v1
    kind: TaskRun
    name: pr-platforms-and-browsers-1
    pipelineTaskName: platforms-and-browsers
  - apiVersion: tekton.dev/v1
    kind: TaskRun
    name: pr-platforms-and-browsers-2
    pipelineTaskName: platforms-and-browsers
  - apiVersion: tekton.dev/v1
    kind: TaskRun
    name: pr-platforms-and-browsers-3
    pipelineTaskName: platforms-and-browsers
  - apiVersion: tekton.dev/v1
    kind: TaskRun
    name: pr-platforms-and-browsers-4
    pipelineTaskName: platforms-and-browsers
  - apiVersion: tekton.dev/v1
    kind: TaskRun
    name: pr-platforms-and-browsers-5
    pipelineTaskName: platforms-and-browsers
  - apiVersion: tekton.dev/v1
    kind: TaskRun
    name: pr-platforms-and-browsers-6
    pipelineTaskName: platforms-and-browsers
  - apiVersion: tekton.dev/v1
    kind: TaskRun
    name: pr-platforms-and-browsers-7
    pipelineTaskName: platforms-and-browsers
  - apiVersion: tekton.dev/v1
    kind: TaskRun
    name: pr-platforms-and-browsers-8
    pipelineTaskName: platforms-and-browsers
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
`),
	}, {
		name:     "p-finally",
		memberOf: "finally",
		p: parse.MustParseV1Pipeline(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: foo
spec:
  tasks:
    - name: pt-with-result
      params:
        - name: platform
          value: linux
        - name: browser
          value: chrome
        - name: version
          value: v0.22.0
      taskRef:
        name: taskwithresults
        kind: Task
  finally:
    - name: platforms-and-browsers
      taskRef:
        name: mytask
        kind: Task
      matrix:
        params:
          - name: platform
            value:
              - $(tasks.pt-with-result.results.platform-1)
              - $(tasks.pt-with-result.results.platform-2)
              - $(tasks.pt-with-result.results.platform-3)
          - name: browser
            value:
              - $(tasks.pt-with-result.results.browser-1)
              - $(tasks.pt-with-result.results.browser-2)
              - $(tasks.pt-with-result.results.browser-3)
      params:
        - name: version
          value: $(tasks.pt-with-result.results.version)
`, "p-finally")),
		tr: mustParseTaskRunWithObjectMeta(t,
			taskRunObjectMeta("pr-pt-with-result", "foo",
				"pr", "p-finally", "pt-with-result", false),
			`
spec:
  serviceAccountName: test-sa
  taskRef:
    name: taskwithresults
    kind: Task
  timeout: 1h0m0s
status:
 conditions:
  - type: Succeeded
    status: "True"
    reason: Succeeded
    message: All Tasks have completed executing
 results:
  - name: platform-1
    value: linux
  - name: platform-2
    value: mac
  - name: platform-3
    value: windows
  - name: browser-1
    value: chrome
  - name: browser-2
    value: safari
  - name: browser-3
    value: firefox
  - name: version
    value: v0.33.0
`),
		expectedPipelineRun: parse.MustParseV1PipelineRun(t, `
metadata:
  name: pr
  namespace: foo
  annotations: {}
  labels:
    tekton.dev/pipeline: p-finally
spec:
  taskRunTemplate:
    serviceAccountName: test-sa
  pipelineRef:
    name: p-finally
status:
  pipelineSpec:
    tasks:
    - name: pt-with-result
      params:
        - name: platform
          value: linux
        - name: browser
          value: chrome
        - name: version
          value: v0.22.0
      taskRef:
        name: taskwithresults
        kind: Task
    finally:
    - name: platforms-and-browsers
      taskRef:
        name: mytask
        kind: Task
      matrix:
        params:
          - name: platform
            value:
              - $(tasks.pt-with-result.results.platform-1)
              - $(tasks.pt-with-result.results.platform-2)
              - $(tasks.pt-with-result.results.platform-3)
          - name: browser
            value:
              - $(tasks.pt-with-result.results.browser-1)
              - $(tasks.pt-with-result.results.browser-2)
              - $(tasks.pt-with-result.results.browser-3)
      params:
        - name: version
          value: $(tasks.pt-with-result.results.version)
  conditions:
  - type: Succeeded
    status: "Unknown"
    reason: "Running"
    message: "Tasks Completed: 1 (Failed: 0, Cancelled 0), Incomplete: 1, Skipped: 0"
  childReferences:
  - apiVersion: tekton.dev/v1
    kind: TaskRun
    name: pr-pt-with-result
    pipelineTaskName: pt-with-result
  - apiVersion: tekton.dev/v1
    kind: TaskRun
    name: pr-platforms-and-browsers-0
    pipelineTaskName: platforms-and-browsers
  - apiVersion: tekton.dev/v1
    kind: TaskRun
    name: pr-platforms-and-browsers-1
    pipelineTaskName: platforms-and-browsers
  - apiVersion: tekton.dev/v1
    kind: TaskRun
    name: pr-platforms-and-browsers-2
    pipelineTaskName: platforms-and-browsers
  - apiVersion: tekton.dev/v1
    kind: TaskRun
    name: pr-platforms-and-browsers-3
    pipelineTaskName: platforms-and-browsers
  - apiVersion: tekton.dev/v1
    kind: TaskRun
    name: pr-platforms-and-browsers-4
    pipelineTaskName: platforms-and-browsers
  - apiVersion: tekton.dev/v1
    kind: TaskRun
    name: pr-platforms-and-browsers-5
    pipelineTaskName: platforms-and-browsers
  - apiVersion: tekton.dev/v1
    kind: TaskRun
    name: pr-platforms-and-browsers-6
    pipelineTaskName: platforms-and-browsers
  - apiVersion: tekton.dev/v1
    kind: TaskRun
    name: pr-platforms-and-browsers-7
    pipelineTaskName: platforms-and-browsers
  - apiVersion: tekton.dev/v1
    kind: TaskRun
    name: pr-platforms-and-browsers-8
    pipelineTaskName: platforms-and-browsers
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
`),
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pr := parse.MustParseV1PipelineRun(t, fmt.Sprintf(`
metadata:
  name: pr
  namespace: foo
spec:
  taskRunTemplate:
    serviceAccountName: test-sa
  pipelineRef:
    name: %s
`, tt.name))
			d := test.Data{
				PipelineRuns: []*v1.PipelineRun{pr},
				Pipelines:    []*v1.Pipeline{tt.p},
				Tasks:        []*v1.Task{task, taskwithresults},
				ConfigMaps:   cms,
			}
			if tt.tr != nil {
				d.TaskRuns = []*v1.TaskRun{tt.tr}
			}
			prt := newPipelineRunTest(t, d)
			defer prt.Cancel()

			_, clients := prt.reconcileRun("foo", "pr", []string{}, false)
			taskRuns, err := clients.Pipeline.TektonV1().TaskRuns("foo").List(prt.TestAssets.Ctx, metav1.ListOptions{
				LabelSelector: fmt.Sprintf("tekton.dev/pipelineRun=pr,tekton.dev/pipeline=%s,tekton.dev/pipelineTask=platforms-and-browsers", tt.name),
				Limit:         1,
			})
			if err != nil {
				t.Fatalf("Failure to list TaskRun's %s", err)
			}

			if len(taskRuns.Items) != 9 {
				t.Fatalf("Expected 9 TaskRuns got %d", len(taskRuns.Items))
			}

			for i := range taskRuns.Items {
				expectedTaskRun := expectedTaskRuns[i]
				expectedTaskRun.Labels["tekton.dev/pipeline"] = tt.name
				expectedTaskRun.Labels["tekton.dev/memberOf"] = tt.memberOf
				if d := cmp.Diff(expectedTaskRun, &taskRuns.Items[i], ignoreResourceVersion, ignoreTypeMeta); d != "" {
					t.Errorf("expected to see TaskRun %v created. Diff %s", expectedTaskRuns[i].Name, diff.PrintWantGot(d))
				}
			}

			pipelineRun, err := clients.Pipeline.TektonV1().PipelineRuns("foo").Get(prt.TestAssets.Ctx, "pr", metav1.GetOptions{})
			if err != nil {
				t.Fatalf("Got an error getting reconciled run out of fake client: %s", err)
			}

			if d := cmp.Diff(tt.expectedPipelineRun, pipelineRun, ignoreResourceVersion, ignoreTypeMeta, ignoreLastTransitionTime, ignoreStartTime, ignoreFinallyStartTime, cmpopts.EquateEmpty()); d != "" {
				t.Errorf("expected PipelineRun was not created. Diff %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestReconciler_PipelineTaskMatrixResultsWithArrays(t *testing.T) {
	names.TestingSeed()
	task := parse.MustParseV1Task(t, `
metadata:
  name: mytask
  namespace: foo
spec:
  params:
    - name: platform
      default: mac
  steps:
    - name: echo
      image: alpine
      script: |
        echo "$(params.platform)"
`)
	taskwithresults := parse.MustParseV1Task(t, `
metadata:
  name: taskwithresults
  namespace: foo
spec:
  results:
    - name: platforms
      type: array
  steps:
    - name: produce-a-list-of-platforms
      image: bash:latest
      script: |
        #!/usr/bin/env bash
        echo -n "[\"linux\",\"mac\",\"windows\"]" | tee $(results.platforms.path)
`)

	cms := []*corev1.ConfigMap{withEnabledAlphaAPIFields(newFeatureFlagsConfigMap())}
	cms = append(cms, withMaxMatrixCombinationsCount(newDefaultsConfigMap(), 10))
	tests := []struct {
		name                string
		pName               string
		p                   *v1.Pipeline
		tr                  *v1.TaskRun
		expectedTaskRuns    []*v1.TaskRun
		expectedPipelineRun *v1.PipelineRun
	}{{
		name:  "indexing results in params",
		pName: "p-dag",
		p: parse.MustParseV1Pipeline(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: foo
spec:
  tasks:
    - name: pt-with-result
      params:
       - name: platforms
         type: array
      taskRef:
        name: taskwithresults
    - name: echo-platforms
      params:
        - name: platforms
          value:
            - $(tasks.pt-with-result.results.platforms[0])
            - $(tasks.pt-with-result.results.platforms[1])
            - $(tasks.pt-with-result.results.platforms[2])
      taskRef:
        name: mytask
`, "p-dag")),
		tr: mustParseTaskRunWithObjectMeta(t,
			taskRunObjectMeta("pr-pt-with-result", "foo",
				"pr", "p-dag", "pt-with-result", false),
			`
spec:
  serviceAccountName: test-sa
  taskRef:
    name: taskwithresults
status:
 conditions:
  - type: Succeeded
    status: "True"
    reason: Succeeded
    message: All Tasks have completed executing
 results:
  - name: platforms
    value:
     - linux
     - mac
     - windows
`),
		expectedTaskRuns: []*v1.TaskRun{
			mustParseTaskRunWithObjectMeta(t,
				taskRunObjectMeta("pr-echo-platforms", "foo",
					"pr", "p-dag", "echo-platforms", false),
				`
spec:
  params:
  - name: platforms
    value:
      - linux
      - mac
      - windows
  serviceAccountName: test-sa
  taskRef:
    name: mytask
    kind: Task
labels:
    tekton.dev/memberOf: tasks
    tekton.dev/pipeline: p-dag
`),
		},
		expectedPipelineRun: parse.MustParseV1PipelineRun(t, `
metadata:
  name: pr
  namespace: foo
  annotations: {}
  labels:
    tekton.dev/pipeline: p-dag
spec:
  taskRunTemplate:
    serviceAccountName: test-sa
  pipelineRef:
    name: p-dag
status:
  pipelineSpec:
    tasks:
    - name: pt-with-result
      params:
        - name: platforms
          type: array
      taskRef:
        name: taskwithresults
        kind: Task
    - name: echo-platforms
      taskRef:
        name: mytask
        kind: Task
      params:
        - name: platforms
          value:
            - $(tasks.pt-with-result.results.platforms[0])
            - $(tasks.pt-with-result.results.platforms[1])
            - $(tasks.pt-with-result.results.platforms[2])
  conditions:
  - type: Succeeded
    status: "Unknown"
    reason: "Running"
    message: "Tasks Completed: 1 (Failed: 0, Cancelled 0), Incomplete: 1, Skipped: 0"
  childReferences:
  - apiVersion: tekton.dev/v1
    kind: TaskRun
    name: pr-pt-with-result
    pipelineTaskName: pt-with-result
  - apiVersion: tekton.dev/v1
    kind: TaskRun
    name: pr-echo-platforms
    pipelineTaskName: echo-platforms
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
`),
	}, {
		name:  "indexing results in matrix.params",
		pName: "p-dag-2",
		p: parse.MustParseV1Pipeline(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: foo
spec:
  tasks:
    - name: pt-with-result
      params:
       - name: platforms
         type: array
      taskRef:
        name: taskwithresults
    - name: echo-platforms
      matrix:
        params:
          - name: platform
            value:
              - $(tasks.pt-with-result.results.platforms[0])
              - $(tasks.pt-with-result.results.platforms[1])
              - $(tasks.pt-with-result.results.platforms[2])
      taskRef:
        name: mytask
`, "p-dag-2")),
		tr: mustParseTaskRunWithObjectMeta(t,
			taskRunObjectMeta("pr-pt-with-result", "foo",
				"pr", "p-dag-2", "pt-with-result", false),
			`
spec:
  serviceAccountName: test-sa
  taskRef:
    name: taskwithresults
status:
 conditions:
  - type: Succeeded
    status: "True"
    reason: Succeeded
    message: All Tasks have completed executing
 results:
  - name: platforms
    value:
     - linux
     - mac
     - windows
`),
		expectedTaskRuns: []*v1.TaskRun{
			mustParseTaskRunWithObjectMeta(t,
				taskRunObjectMeta("pr-echo-platforms-0", "foo",
					"pr", "p-dag-2", "echo-platforms", false),
				`
spec:
  params:
  - name: platform
    value: linux
  serviceAccountName: test-sa
  taskRef:
    name: mytask
    kind: Task
labels:
    tekton.dev/memberOf: tasks
    tekton.dev/pipeline: p-dag-2
`),
			mustParseTaskRunWithObjectMeta(t,
				taskRunObjectMeta("pr-echo-platforms-1", "foo",
					"pr", "p-dag-2", "echo-platforms", false),
				`
spec:
  params:
  - name: platform
    value: mac
  serviceAccountName: test-sa
  taskRef:
    name: mytask
    kind: Task
labels:
    tekton.dev/memberOf: tasks
    tekton.dev/pipeline: p-dag-2
`),
			mustParseTaskRunWithObjectMeta(t,
				taskRunObjectMeta("pr-echo-platforms-2", "foo",
					"pr", "p-dag-2", "echo-platforms", false),
				`
spec:
  params:
  - name: platform
    value: windows
  serviceAccountName: test-sa
  taskRef:
    name: mytask
    kind: Task
labels:
    tekton.dev/memberOf: tasks
    tekton.dev/pipeline: p-dag-2
`),
		},
		expectedPipelineRun: parse.MustParseV1PipelineRun(t, `
metadata:
  name: pr
  namespace: foo
  annotations: {}
  labels:
    tekton.dev/pipeline: p-dag-2
spec:
  taskRunTemplate:
    serviceAccountName: test-sa
  pipelineRef:
    name: p-dag-2
status:
  pipelineSpec:
    tasks:
    - name: pt-with-result
      params:
        - name: platforms
          type: array
      taskRef:
        name: taskwithresults
        kind: Task
    - name: echo-platforms
      taskRef:
        name: mytask
        kind: Task
      matrix:
        params:
          - name: platform
            value:
              - $(tasks.pt-with-result.results.platforms[0])
              - $(tasks.pt-with-result.results.platforms[1])
              - $(tasks.pt-with-result.results.platforms[2])
  conditions:
  - type: Succeeded
    status: "Unknown"
    reason: "Running"
    message: "Tasks Completed: 1 (Failed: 0, Cancelled 0), Incomplete: 1, Skipped: 0"
  childReferences:
  - apiVersion: tekton.dev/v1
    kind: TaskRun
    name: pr-pt-with-result
    pipelineTaskName: pt-with-result
  - apiVersion: tekton.dev/v1
    kind: TaskRun
    name: pr-echo-platforms-0
    pipelineTaskName: echo-platforms
  - apiVersion: tekton.dev/v1
    kind: TaskRun
    name: pr-echo-platforms-1
    pipelineTaskName: echo-platforms
  - apiVersion: tekton.dev/v1
    kind: TaskRun
    name: pr-echo-platforms-2
    pipelineTaskName: echo-platforms
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
`),
	}, {
		name:  "whole array result replacements in matrix.params",
		pName: "p-dag-3",
		p: parse.MustParseV1Pipeline(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: foo
spec:
  tasks:
    - name: pt-with-result
      params:
       - name: platforms
         type: array
      taskRef:
        name: taskwithresults
    - name: echo-platforms
      matrix:
        params:
          - name: platform
            value: $(tasks.pt-with-result.results.platforms[*])
      taskRef:
        name: mytask
`, "p-dag-3")),
		tr: mustParseTaskRunWithObjectMeta(t,
			taskRunObjectMeta("pr-pt-with-result", "foo",
				"pr", "p-dag-3", "pt-with-result", false),
			`
spec:
  serviceAccountName: test-sa
  taskRef:
    name: taskwithresults
status:
 conditions:
  - type: Succeeded
    status: "True"
    reason: Succeeded
    message: All Tasks have completed executing
 results:
  - name: platforms
    value:
     - linux
     - mac
     - windows
`),
		expectedTaskRuns: []*v1.TaskRun{
			mustParseTaskRunWithObjectMeta(t,
				taskRunObjectMeta("pr-echo-platforms-0", "foo",
					"pr", "p-dag-3", "echo-platforms", false),
				`
spec:
  params:
  - name: platform
    value: linux
  serviceAccountName: test-sa
  taskRef:
    name: mytask
    kind: Task
labels:
    tekton.dev/memberOf: tasks
    tekton.dev/pipeline: p-dag-3
`),
			mustParseTaskRunWithObjectMeta(t,
				taskRunObjectMeta("pr-echo-platforms-1", "foo",
					"pr", "p-dag-3", "echo-platforms", false),
				`
spec:
  params:
  - name: platform
    value: mac
  serviceAccountName: test-sa
  taskRef:
    name: mytask
    kind: Task
labels:
    tekton.dev/memberOf: tasks
    tekton.dev/pipeline: p-dag-3
`),
			mustParseTaskRunWithObjectMeta(t,
				taskRunObjectMeta("pr-echo-platforms-2", "foo",
					"pr", "p-dag-3", "echo-platforms", false),
				`
spec:
  params:
  - name: platform
    value: windows
  serviceAccountName: test-sa
  taskRef:
    name: mytask
    kind: Task
labels:
    tekton.dev/memberOf: tasks
    tekton.dev/pipeline: p-dag-3
`),
		},
		expectedPipelineRun: parse.MustParseV1PipelineRun(t, `
metadata:
  name: pr
  namespace: foo
  annotations: {}
  labels:
    tekton.dev/pipeline: p-dag-3
spec:
  taskRunTemplate:
    serviceAccountName: test-sa
  pipelineRef:
    name: p-dag-3
status:
  pipelineSpec:
    tasks:
    - name: pt-with-result
      params:
        - name: platforms
          type: array
      taskRef:
        name: taskwithresults
        kind: Task
    - name: echo-platforms
      taskRef:
        name: mytask
        kind: Task
      matrix:
        params:
          - name: platform
            value: $(tasks.pt-with-result.results.platforms[*])
  conditions:
  - type: Succeeded
    status: "Unknown"
    reason: "Running"
    message: "Tasks Completed: 1 (Failed: 0, Cancelled 0), Incomplete: 1, Skipped: 0"
  childReferences:
  - apiVersion: tekton.dev/v1
    kind: TaskRun
    name: pr-pt-with-result
    pipelineTaskName: pt-with-result
  - apiVersion: tekton.dev/v1
    kind: TaskRun
    name: pr-echo-platforms-0
    pipelineTaskName: echo-platforms
  - apiVersion: tekton.dev/v1
    kind: TaskRun
    name: pr-echo-platforms-1
    pipelineTaskName: echo-platforms
  - apiVersion: tekton.dev/v1
    kind: TaskRun
    name: pr-echo-platforms-2
    pipelineTaskName: echo-platforms
`),
	}}
	for _, tt := range tests {
		t.Run(tt.pName, func(t *testing.T) {
			pr := parse.MustParseV1PipelineRun(t, fmt.Sprintf(`
metadata:
  name: pr
  namespace: foo
spec:
  taskRunTemplate:
    serviceAccountName: test-sa
  pipelineRef:
    name: %s
`, tt.pName))
			d := test.Data{
				PipelineRuns: []*v1.PipelineRun{pr},
				Pipelines:    []*v1.Pipeline{tt.p},
				Tasks:        []*v1.Task{task, taskwithresults},
				ConfigMaps:   cms,
			}
			if tt.tr != nil {
				d.TaskRuns = []*v1.TaskRun{tt.tr}
			}
			prt := newPipelineRunTest(t, d)
			defer prt.Cancel()
			pipelineRun, clients := prt.reconcileRun(pr.Namespace, pr.Name, []string{} /* wantEvents*/, false /* permanentError*/)

			taskRuns := getTaskRunsForPipelineTask(prt.TestAssets.Ctx, t, clients, pr.Namespace, pr.Name, "echo-platforms")
			validateTaskRunsCount(t, taskRuns, len(tt.expectedTaskRuns))
			for _, expectedTaskRun := range tt.expectedTaskRuns {
				trName := expectedTaskRun.Name
				actual := getTaskRunByName(t, taskRuns, trName)
				if d := cmp.Diff(expectedTaskRun, actual, ignoreResourceVersion, ignoreTypeMeta); d != "" {
					t.Errorf("expected to see TaskRun %v created. Diff %s", expectedTaskRun.Name, diff.PrintWantGot(d))
				}
			}

			if d := cmp.Diff(tt.expectedPipelineRun, pipelineRun, ignoreResourceVersion, ignoreTypeMeta, ignoreLastTransitionTime, ignoreStartTime, ignoreFinallyStartTime, ignoreProvenance, cmpopts.EquateEmpty()); d != "" {
				t.Errorf("expected PipelineRun was not created. Diff %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestReconciler_CustomPipelineTaskMatrixResultsWholeArray(t *testing.T) {
	names.TestingSeed()
	taskwithresults := parse.MustParseV1Task(t, `
metadata:
  name: taskwithresults
  namespace: foo
spec:
  results:
    - name: platforms
      type: array
  steps:
    - name: produce-a-list-of-platforms
      image: bash:latest
      script: |
        #!/usr/bin/env bash
        echo -n "[\"linux\",\"mac\",\"windows\"]" | tee $(results.platforms.path)
`)
	expectedCustomRuns := []*v1beta1.CustomRun{
		mustParseCustomRunWithObjectMeta(t,
			taskRunObjectMeta("pr-pt-matrix-custom-task-0", "foo",
				"pr", "p-dag", "pt-matrix-custom-task", false),
			`
spec:
  customRef:
    apiVersion: example.dev/v0
    kind: Example
  params:
  - name: platform
    value: linux
  serviceAccountName: test-sa
  taskRef:
    name: mytask
    kind: Task
  labels:
    tekton.dev/memberOf: tasks
    tekton.dev/pipeline: p-dag
`),
		mustParseCustomRunWithObjectMeta(t,
			taskRunObjectMeta("pr-pt-matrix-custom-task-1", "foo",
				"pr", "p-dag", "pt-matrix-custom-task", false),
			`
spec:
  customRef:
    apiVersion: example.dev/v0
    kind: Example
  params:
  - name: platform
    value: mac
  serviceAccountName: test-sa
  taskRef:
    name: mytask
    kind: Task
  labels:
    tekton.dev/memberOf: tasks
    tekton.dev/pipeline: p-dag
`),
		mustParseCustomRunWithObjectMeta(t,
			taskRunObjectMeta("pr-pt-matrix-custom-task-2", "foo",
				"pr", "p-dag", "pt-matrix-custom-task", false),
			`
spec:
  customRef:
    apiVersion: example.dev/v0
    kind: Example
  params:
  - name: platform
    value: windows
  serviceAccountName: test-sa
  taskRef:
    name: mytask
    kind: Task
  labels:
    tekton.dev/memberOf: tasks
    tekton.dev/pipeline: p-dag
`),
	}
	cms := []*corev1.ConfigMap{withEnabledAlphaAPIFields(newFeatureFlagsConfigMap())}
	cms = append(cms, withMaxMatrixCombinationsCount(newDefaultsConfigMap(), 10))
	tests := []struct {
		name                string
		pName               string
		p                   *v1.Pipeline
		tr                  *v1.TaskRun
		expectedPipelineRun *v1.PipelineRun
	}{{
		name:  "whole array results replacements in a matrix",
		pName: "p-dag",
		p: parse.MustParseV1Pipeline(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: foo
spec:
  tasks:
    - name: pt-with-result
      params:
       - name: platforms
         type: array
      taskRef:
        name: taskwithresults
    - name: pt-matrix-custom-task
      matrix:
        params:
          - name: platform
            value: $(tasks.pt-with-result.results.platforms[*])
      taskRef:
        apiVersion: example.dev/v0
        kind: Example
`, "p-dag")),
		tr: mustParseTaskRunWithObjectMeta(t,
			taskRunObjectMeta("pr-pt-with-result", "foo",
				"pr", "p-dag", "pt-with-result", false),
			`
spec:
  serviceAccountName: test-sa
  taskRef:
    name: taskwithresults
status:
 conditions:
  - type: Succeeded
    status: "True"
    reason: Succeeded
    message: All Tasks have completed executing
 results:
  - name: platforms
    value:
     - linux
     - mac
     - windows
`),
		expectedPipelineRun: parse.MustParseV1PipelineRun(t, `
metadata:
  name: pr
  namespace: foo
  annotations: {}
  labels:
    tekton.dev/pipeline: p-dag
spec:
  serviceAccountName: test-sa
  pipelineRef:
    name: p-dag
  taskRunTemplate:
    serviceAccountName: test-sa
status:
  pipelineSpec:
    tasks:
    - name: pt-with-result
      params:
        - name: platforms
          type: array
      taskRef:
        name: taskwithresults
        kind: Task
    - name: pt-matrix-custom-task
      taskRef:
        apiVersion: example.dev/v0
        kind: Example
      matrix:
        params:
          - name: platform
            value: $(tasks.pt-with-result.results.platforms[*])
  conditions:
  - type: Succeeded
    status: "Unknown"
    reason: "Running"
    message: "Tasks Completed: 1 (Failed: 0, Cancelled 0), Incomplete: 1, Skipped: 0"
  childReferences:
  - apiVersion: tekton.dev/v1
    kind: TaskRun
    name: pr-pt-with-result
    pipelineTaskName: pt-with-result
  - apiVersion: tekton.dev/v1beta1
    kind: CustomRun
    name:  pr-pt-matrix-custom-task-0
    pipelineTaskName: pt-matrix-custom-task
  - apiVersion: tekton.dev/v1beta1
    kind: CustomRun
    name:  pr-pt-matrix-custom-task-1
    pipelineTaskName: pt-matrix-custom-task
  - apiVersion: tekton.dev/v1beta1
    kind: CustomRun
    name:  pr-pt-matrix-custom-task-2
    pipelineTaskName: pt-matrix-custom-task
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
`),
	}}
	for _, tt := range tests {
		t.Run(tt.pName, func(t *testing.T) {
			pr := parse.MustParseV1PipelineRun(t, fmt.Sprintf(`
metadata:
  name: pr
  namespace: foo
spec:
  taskRunTemplate:
    serviceAccountName: test-sa
  pipelineRef:
    name: %s
`, tt.pName))
			d := test.Data{
				PipelineRuns: []*v1.PipelineRun{pr},
				Pipelines:    []*v1.Pipeline{tt.p},
				Tasks:        []*v1.Task{taskwithresults},
				ConfigMaps:   cms,
			}
			if tt.tr != nil {
				d.TaskRuns = []*v1.TaskRun{tt.tr}
			}

			prt := newPipelineRunTest(t, d)
			defer prt.Cancel()

			pipelineRun, clients := prt.reconcileRun("foo", "pr", []string{}, false)
			customRuns, err := clients.Pipeline.TektonV1beta1().CustomRuns("foo").List(prt.TestAssets.Ctx, metav1.ListOptions{})
			if err != nil {
				t.Fatalf("Failure to list customRuns's %s", err)
			}

			if len(customRuns.Items) != 3 {
				t.Fatalf("Expected 3 customRuns got %d", len(customRuns.Items))
			}

			for i := range customRuns.Items {
				expectedCustomRun := expectedCustomRuns[i]
				if d := cmp.Diff(expectedCustomRun, &customRuns.Items[i], ignoreResourceVersion, ignoreTypeMeta, cmpopts.EquateEmpty()); d != "" {
					t.Errorf("expected to see CustomRun %v created. Diff %s", expectedCustomRun.Name, diff.PrintWantGot(d))
				}
			}

			if d := cmp.Diff(tt.expectedPipelineRun, pipelineRun, ignoreResourceVersion, ignoreTypeMeta, ignoreLastTransitionTime, ignoreStartTime, ignoreFinallyStartTime, cmpopts.EquateEmpty()); d != "" {
				t.Errorf("expected PipelineRun was not created. Diff %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestReconciler_PipelineTaskMatrixWithRetries(t *testing.T) {
	names.TestingSeed()

	task := parse.MustParseV1Task(t, `
metadata:
  name: mytask
  namespace: foo
spec:
  params:
    - name: platform
    - name: browser
  steps:
    - name: echo
      image: alpine
      script: |
        echo "$(params.platform) and $(params.browser)"
        exit 1
`)

	cms := []*corev1.ConfigMap{withEnabledAlphaAPIFields(newFeatureFlagsConfigMap())}
	cms = append(cms, withMaxMatrixCombinationsCount(newDefaultsConfigMap(), 10))

	tests := []struct {
		name                string
		trs                 []*v1.TaskRun
		prs                 []*v1.PipelineRun
		expectedPipelineRun *v1.PipelineRun
		expectedTaskRuns    []*v1.TaskRun
	}{{
		name: "matrixed pipelinetask with retries, where one taskrun has failed and another one is running",
		trs: []*v1.TaskRun{
			mustParseTaskRunWithObjectMeta(t,
				taskRunObjectMeta("pr-platforms-and-browsers-0", "foo",
					"pr", "p", "platforms-and-browsers", false),
				`
spec:
  retries: 1
  params:
  - name: platform
    value: linux
  - name: browser
    value: chrome
  serviceAccountName: test-sa
  taskRef:
    name: mytask
  timeout: 1h0m0s
status:
  conditions:
  - type: Succeeded
    status: "False"
  retriesStatus:
  - conditions:
    - status: "False"
      type: Succeeded
`),
			mustParseTaskRunWithObjectMeta(t,
				taskRunObjectMeta("pr-platforms-and-browsers-1", "foo",
					"pr", "p", "platforms-and-browsers", false),
				`
spec:
  retries: 1
  params:
  - name: platform
    value: mac
  - name: browser
    value: chrome
  serviceAccountName: test-sa
  taskRef:
    name: mytask
  timeout: 1h0m0s
status:
  conditions:
  - type: Succeeded
    status: "Unknown"
`),
		},
		prs: []*v1.PipelineRun{
			parse.MustParseV1PipelineRun(t, `
metadata:
  name: pr
  namespace: foo
  annotations: {}
  labels:
    tekton.dev/pipeline: p
spec:
  serviceAccountName: test-sa
  pipelineRef:
    name: p
status:
  pipelineSpec:
    tasks:
    - name: platforms-and-browsers
      retries: 1
      taskRef:
        name: mytask
        kind: Task
      matrix:
        params:
          - name: platform
            value:
              - linux
              - mac
      params:
        - name: browser
          value: chrome
  conditions:
  - type: Succeeded
    status: "Unknown"
    reason: "Running"
    message: "Tasks Completed: 0 (Failed: 0, Cancelled 0), Incomplete: 1, Skipped: 0"
  childReferences:
  - apiVersion: tekton.dev/v1
    kind: TaskRun
    name: pr-platforms-and-browsers-0
    pipelineTaskName: platforms-and-browsers
  - apiVersion: tekton.dev/v1
    kind: TaskRun
    name: pr-platforms-and-browsers-1
    pipelineTaskName: platforms-and-browsers
`),
		},
		expectedPipelineRun: parse.MustParseV1PipelineRun(t, `
metadata:
  name: pr
  namespace: foo
  annotations: {}
  labels:
    tekton.dev/pipeline: p
spec:
  serviceAccountName: test-sa
  pipelineRef:
    name: p
status:
  pipelineSpec:
    tasks:
    - name: platforms-and-browsers
      retries: 1
      taskRef:
        name: mytask
        kind: Task
      matrix:
        params:
          - name: platform
            value:
              - linux
              - mac
      params:
        - name: browser
          value: chrome
  conditions:
  - type: Succeeded
    status: "Unknown"
    reason: "Running"
    message: "Tasks Completed: 0 (Failed: 0, Cancelled 0), Incomplete: 1, Skipped: 0"
  childReferences:
  - apiVersion: tekton.dev/v1
    kind: TaskRun
    name: pr-platforms-and-browsers-0
    pipelineTaskName: platforms-and-browsers
  - apiVersion: tekton.dev/v1
    kind: TaskRun
    name: pr-platforms-and-browsers-1
    pipelineTaskName: platforms-and-browsers
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
`),
		expectedTaskRuns: []*v1.TaskRun{
			mustParseTaskRunWithObjectMeta(t,
				taskRunObjectMeta("pr-platforms-and-browsers-0", "foo",
					"pr", "p", "platforms-and-browsers", false),
				`
spec:
  retries: 1
  params:
  - name: platform
    value: linux
  - name: browser
    value: chrome
  serviceAccountName: test-sa
  taskRef:
    name: mytask
  timeout: 1h0m0s
status:
  conditions:
  - type: Succeeded
    status: "False"
  retriesStatus:
  - conditions:
    - status: "False"
      type: Succeeded
`),
			mustParseTaskRunWithObjectMeta(t,
				taskRunObjectMeta("pr-platforms-and-browsers-1", "foo",
					"pr", "p", "platforms-and-browsers", false),
				`
spec:
  retries: 1
  params:
  - name: platform
    value: mac
  - name: browser
    value: chrome
  serviceAccountName: test-sa
  taskRef:
    name: mytask
  timeout: 1h0m0s
status:
  conditions:
  - type: Succeeded
    status: "Unknown"
`),
		},
	}, {
		name: "matrixed pipelinetask with retries, where both taskruns have failed",
		trs: []*v1.TaskRun{
			mustParseTaskRunWithObjectMeta(t,
				taskRunObjectMeta("pr-platforms-and-browsers-0", "foo",
					"pr", "p", "platforms-and-browsers", false),
				`
spec:
  retries: 1
  params:
  - name: platform
    value: linux
  - name: browser
    value: chrome
  serviceAccountName: test-sa
  taskRef:
    name: mytask
  timeout: 1h0m0s
status:
  conditions:
  - type: Succeeded
    status: "Unknown"
  retriesStatus:
  - conditions:
    - status: "False"
      type: Succeeded
`),
			mustParseTaskRunWithObjectMeta(t,
				taskRunObjectMeta("pr-platforms-and-browsers-1", "foo",
					"pr", "p", "platforms-and-browsers", false),
				`
spec:
  retries: 1
  params:
  - name: platform
    value: mac
  - name: browser
    value: chrome
  serviceAccountName: test-sa
  taskRef:
    name: mytask
  timeout: 1h0m0s
status:
  conditions:
  - type: Succeeded
    status: "Unknown"
  retriesStatus:
  - conditions:
    - status: "False"
      type: Succeeded
`),
		},
		prs: []*v1.PipelineRun{
			parse.MustParseV1PipelineRun(t, `
metadata:
  name: pr
  namespace: foo
  annotations: {}
  labels:
    tekton.dev/pipeline: p
spec:
  serviceAccountName: test-sa
  pipelineRef:
    name: p
status:
  pipelineSpec:
    tasks:
    - name: platforms-and-browsers
      retries: 1
      taskRef:
        name: mytask
        kind: Task
      matrix:
        params:
          - name: platform
            value:
              - linux
              - mac
      params:
        - name: browser
          value: chrome
  conditions:
  - type: Succeeded
    status: "Unknown"
    reason: "Running"
    message: "Tasks Completed: 0 (Failed: 0, Cancelled 0), Incomplete: 1, Skipped: 0"
  childReferences:
  - apiVersion: tekton.dev/v1
    kind: TaskRun
    name: pr-platforms-and-browsers-0
    pipelineTaskName: platforms-and-browsers
  - apiVersion: tekton.dev/v1
    kind: TaskRun
    name: pr-platforms-and-browsers-1
    pipelineTaskName: platforms-and-browsers
`),
		},
		expectedPipelineRun: parse.MustParseV1PipelineRun(t, `
metadata:
  name: pr
  namespace: foo
  annotations: {}
  labels:
    tekton.dev/pipeline: p
spec:
  serviceAccountName: test-sa
  pipelineRef:
    name: p
status:
  pipelineSpec:
    tasks:
    - name: platforms-and-browsers
      retries: 1
      taskRef:
        name: mytask
        kind: Task
      matrix:
        params:
          - name: platform
            value:
              - linux
              - mac
      params:
        - name: browser
          value: chrome
  conditions:
  - type: Succeeded
    status: "Unknown"
    reason: "Running"
    message: "Tasks Completed: 0 (Failed: 0, Cancelled 0), Incomplete: 1, Skipped: 0"
  childReferences:
  - apiVersion: tekton.dev/v1
    kind: TaskRun
    name: pr-platforms-and-browsers-0
    pipelineTaskName: platforms-and-browsers
  - apiVersion: tekton.dev/v1
    kind: TaskRun
    name: pr-platforms-and-browsers-1
    pipelineTaskName: platforms-and-browsers
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
`),
		expectedTaskRuns: []*v1.TaskRun{
			mustParseTaskRunWithObjectMeta(t,
				taskRunObjectMeta("pr-platforms-and-browsers-0", "foo",
					"pr", "p", "platforms-and-browsers", false),
				`
spec:
  retries: 1
  params:
  - name: platform
    value: linux
  - name: browser
    value: chrome
  serviceAccountName: test-sa
  taskRef:
    name: mytask
  timeout: 1h0m0s
status:
  conditions:
  - type: Succeeded
    status: "Unknown"
  retriesStatus:
  - conditions:
    - status: "False"
      type: Succeeded
`),
			mustParseTaskRunWithObjectMeta(t,
				taskRunObjectMeta("pr-platforms-and-browsers-1", "foo",
					"pr", "p", "platforms-and-browsers", false),
				`
spec:
  retries: 1
  params:
  - name: platform
    value: mac
  - name: browser
    value: chrome
  serviceAccountName: test-sa
  taskRef:
    name: mytask
  timeout: 1h0m0s
status:
  conditions:
  - type: Succeeded
    status: "Unknown"
  retriesStatus:
  - conditions:
    - status: "False"
      type: Succeeded
`),
		},
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := test.Data{
				Tasks:        []*v1.Task{task},
				TaskRuns:     tt.trs,
				PipelineRuns: tt.prs,
				ConfigMaps:   cms,
			}
			prt := newPipelineRunTest(t, d)
			defer prt.Cancel()

			_, clients := prt.reconcileRun("foo", "pr", []string{}, false)
			taskRuns, err := clients.Pipeline.TektonV1().TaskRuns("foo").List(prt.TestAssets.Ctx, metav1.ListOptions{
				LabelSelector: "tekton.dev/pipelineRun=pr,tekton.dev/pipelineTask=platforms-and-browsers",
				Limit:         1,
			})
			if err != nil {
				t.Fatalf("Failure to list TaskRun's %s", err)
			}

			if len(taskRuns.Items) != 2 {
				t.Fatalf("Expected 2 TaskRuns got %d", len(taskRuns.Items))
			}

			for i := range taskRuns.Items {
				expectedTaskRun := tt.expectedTaskRuns[i]
				if d := cmp.Diff(expectedTaskRun, &taskRuns.Items[i], ignoreResourceVersion, ignoreTypeMeta, ignoreLastTransitionTime, ignoreStartTime); d != "" {
					t.Errorf("expected to see TaskRun %v created. Diff %s", tt.expectedTaskRuns[i].Name, diff.PrintWantGot(d))
				}
			}

			pipelineRun, err := clients.Pipeline.TektonV1().PipelineRuns("foo").Get(prt.TestAssets.Ctx, "pr", metav1.GetOptions{})
			if err != nil {
				t.Fatalf("Got an error getting reconciled run out of fake client: %s", err)
			}

			if d := cmp.Diff(tt.expectedPipelineRun, pipelineRun, ignoreResourceVersion, ignoreTypeMeta, ignoreLastTransitionTime, ignoreStartTime, cmpopts.SortSlices(lessChildReferences), cmpopts.EquateEmpty()); d != "" {
				t.Errorf("expected PipelineRun was not created. Diff %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestReconciler_PipelineTaskMatrixWithCustomTask(t *testing.T) {
	names.TestingSeed()

	task := parse.MustParseV1Task(t, `
metadata:
  name: mytask
  namespace: foo
spec:
  ref:
    apiVersion: example.dev/v0
    kind: Example
  params:
    - name: platform
    - name: browser
    - name: version
  steps:
    - name: echo
      image: alpine
      script: |
        echo "$(params.platform) and $(params.browser)" and $(params.version)"
`)

	expectedCustomRuns := []*v1beta1.CustomRun{
		mustParseCustomRunWithObjectMeta(t,
			taskRunObjectMeta("pr-platforms-and-browsers-0", "foo",
				"pr", "p", "platforms-and-browsers", false),
			`
spec:
  customRef:
    apiVersion: example.dev/v0
    kind: Example
  params:
  - name: browser
    value: chrome
  - name: platform
    value: linux
  - name: version
    value: v0.1
  serviceAccountName: test-sa
  taskRef:
    name: mytask
`),
		mustParseCustomRunWithObjectMeta(t,
			taskRunObjectMeta("pr-platforms-and-browsers-1", "foo",
				"pr", "p", "platforms-and-browsers", false),
			`
spec:
  customRef:
    apiVersion: example.dev/v0
    kind: Example
  params:
  - name: browser
    value: chrome
  - name: platform
    value: mac
  - name: version
    value: v0.1
  serviceAccountName: test-sa
  taskRef:
    name: mytask
`),
		mustParseCustomRunWithObjectMeta(t,
			taskRunObjectMeta("pr-platforms-and-browsers-2", "foo",
				"pr", "p", "platforms-and-browsers", false),
			`
spec:
  customRef:
    apiVersion: example.dev/v0
    kind: Example
  params:
  - name: browser
    value: chrome
  - name: platform
    value: windows
  - name: version
    value: v0.1
  serviceAccountName: test-sa
  taskRef:
    name: mytask
`),
		mustParseCustomRunWithObjectMeta(t,
			taskRunObjectMeta("pr-platforms-and-browsers-3", "foo",
				"pr", "p", "platforms-and-browsers", false),
			`
spec:
  customRef:
    apiVersion: example.dev/v0
    kind: Example
  params:
  - name: browser
    value: safari
  - name: platform
    value: linux
  - name: version
    value: v0.1
  serviceAccountName: test-sa
  taskRef:
    name: mytask
`),
		mustParseCustomRunWithObjectMeta(t,
			taskRunObjectMeta("pr-platforms-and-browsers-4", "foo",
				"pr", "p", "platforms-and-browsers", false),
			`
spec:
  customRef:
    apiVersion: example.dev/v0
    kind: Example
  params:
  - name: browser
    value: safari
  - name: platform
    value: mac
  - name: version
    value: v0.1
  serviceAccountName: test-sa
  taskRef:
    name: mytask
`),
		mustParseCustomRunWithObjectMeta(t,
			taskRunObjectMeta("pr-platforms-and-browsers-5", "foo",
				"pr", "p", "platforms-and-browsers", false),
			`
spec:
  customRef:
    apiVersion: example.dev/v0
    kind: Example
  params:
  - name: browser
    value: safari
  - name: platform
    value: windows
  - name: version
    value: v0.1
  serviceAccountName: test-sa
  taskRef:
    name: mytask
`),
		mustParseCustomRunWithObjectMeta(t,
			taskRunObjectMeta("pr-platforms-and-browsers-6", "foo",
				"pr", "p", "platforms-and-browsers", false),
			`
spec:
  customRef:
    apiVersion: example.dev/v0
    kind: Example
  params:
  - name: browser
    value: firefox
  - name: platform
    value: linux
  - name: version
    value: v0.1
  serviceAccountName: test-sa
  taskRef:
    name: mytask
`),
		mustParseCustomRunWithObjectMeta(t,
			taskRunObjectMeta("pr-platforms-and-browsers-7", "foo",
				"pr", "p", "platforms-and-browsers", false),
			`
spec:
  customRef:
    apiVersion: example.dev/v0
    kind: Example
  params:
  - name: browser
    value: firefox
  - name: platform
    value: mac
  - name: version
    value: v0.1
  serviceAccountName: test-sa
  taskRef:
    name: mytask
`),
		mustParseCustomRunWithObjectMeta(t,
			taskRunObjectMeta("pr-platforms-and-browsers-8", "foo",
				"pr", "p", "platforms-and-browsers", false),
			`
spec:
  customRef:
    apiVersion: example.dev/v0
    kind: Example
  params:
  - name: browser
    value: firefox
  - name: platform
    value: windows
  - name: version
    value: v0.1
  serviceAccountName: test-sa
  taskRef:
    name: mytask
`),
	}
	cms := []*corev1.ConfigMap{withEnabledAlphaAPIFields(newFeatureFlagsConfigMap())}

	tests := []struct {
		name                string
		memberOf            string
		p                   *v1.Pipeline
		tr                  *v1.TaskRun
		expectedPipelineRun *v1.PipelineRun
	}{{
		name:     "p-dag",
		memberOf: "tasks",
		p: parse.MustParseV1Pipeline(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: foo
spec:
  tasks:
    - name: platforms-and-browsers
      taskRef:
        apiVersion: example.dev/v0
        kind: Example
      matrix:
        params:
          - name: platform
            value:
              - linux
              - mac
              - windows
          - name: browser
            value:
              - chrome
              - safari
              - firefox
      params:
        - name: version
          value: v0.1
`, "p-dag")),
		expectedPipelineRun: parse.MustParseV1PipelineRun(t, `
metadata:
  name: pr
  namespace: foo
  annotations: {}
  labels:
    tekton.dev/pipeline: p-dag
spec:
  taskRunTemplate:
    serviceAccountName: test-sa
  pipelineRef:
    name: p-dag
status:
  pipelineSpec:
    tasks:
    - name: platforms-and-browsers
      taskRef:
        apiVersion: example.dev/v0
        kind: Example
      matrix:
        params:
          - name: platform
            value:
              - linux
              - mac
              - windows
          - name: browser
            value:
              - chrome
              - safari
              - firefox
      params:
        - name: version
          value: v0.1
  conditions:
  - type: Succeeded
    status: "Unknown"
    reason: "Running"
    message: "Tasks Completed: 0 (Failed: 0, Cancelled 0), Incomplete: 1, Skipped: 0"
  childReferences:
  - apiVersion: tekton.dev/v1beta1
    kind: CustomRun
    name: pr-platforms-and-browsers-0
    pipelineTaskName: platforms-and-browsers
  - apiVersion: tekton.dev/v1beta1
    kind: CustomRun
    name: pr-platforms-and-browsers-1
    pipelineTaskName: platforms-and-browsers
  - apiVersion: tekton.dev/v1beta1
    kind: CustomRun
    name: pr-platforms-and-browsers-2
    pipelineTaskName: platforms-and-browsers
  - apiVersion: tekton.dev/v1beta1
    kind: CustomRun
    name: pr-platforms-and-browsers-3
    pipelineTaskName: platforms-and-browsers
  - apiVersion: tekton.dev/v1beta1
    kind: CustomRun
    name: pr-platforms-and-browsers-4
    pipelineTaskName: platforms-and-browsers
  - apiVersion: tekton.dev/v1beta1
    kind: CustomRun
    name: pr-platforms-and-browsers-5
    pipelineTaskName: platforms-and-browsers
  - apiVersion: tekton.dev/v1beta1
    kind: CustomRun
    name: pr-platforms-and-browsers-6
    pipelineTaskName: platforms-and-browsers
  - apiVersion: tekton.dev/v1beta1
    kind: CustomRun
    name: pr-platforms-and-browsers-7
    pipelineTaskName: platforms-and-browsers
  - apiVersion: tekton.dev/v1beta1
    kind: CustomRun
    name: pr-platforms-and-browsers-8
    pipelineTaskName: platforms-and-browsers
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
`),
	}, {
		name:     "p-finally",
		memberOf: "finally",
		p: parse.MustParseV1Pipeline(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: foo
spec:
  tasks:
    - name: unmatrixed-pt
      params:
        - name: platform
          value: linux
        - name: browser
          value: chrome
        - name: version
          value: v0.0
      taskRef:
        name: mytask
  finally:
    - name: platforms-and-browsers
      taskRef:
        apiVersion: example.dev/v0
        kind: Example
      matrix:
        params:
          - name: platform
            value:
              - linux
              - mac
              - windows
          - name: browser
            value:
              - chrome
              - safari
              - firefox
      params:
        - name: version
          value: v0.1
`, "p-finally")),
		tr: mustParseTaskRunWithObjectMeta(t,
			taskRunObjectMeta("pr-unmatrixed-pt", "foo",
				"pr", "p-finally", "unmatrixed-pt", false),
			`
spec:
  params:
  - name: platform
    value: linux
  - name: browser
    value: chrome
  - name: version
    value: v0.0
  serviceAccountName: test-sa
  taskRef:
    name: mytask
  timeout: 1h0m0s
status:
 conditions:
  - type: Succeeded
    status: "True"
    reason: Succeeded
    message: All Tasks have completed executing
`),
		expectedPipelineRun: parse.MustParseV1PipelineRun(t, `
metadata:
  name: pr
  namespace: foo
  annotations: {}
  labels:
    tekton.dev/pipeline: p-finally
spec:
  taskRunTemplate:
    serviceAccountName: test-sa
  pipelineRef:
    name: p-finally
status:
  pipelineSpec:
    tasks:
    - name: unmatrixed-pt
      params:
        - name: platform
          value: linux
        - name: browser
          value: chrome
        - name: version
          value: v0.0
      taskRef:
        name: mytask
        kind: Task
    finally:
    - name: platforms-and-browsers
      taskRef:
        apiVersion: example.dev/v0
        kind: Example
      matrix:
        params:
          - name: platform
            value:
              - linux
              - mac
              - windows
          - name: browser
            value:
              - chrome
              - safari
              - firefox
      params:
        - name: version
          value: v0.1
  conditions:
  - type: Succeeded
    status: "Unknown"
    reason: "Running"
    message: "Tasks Completed: 1 (Failed: 0, Cancelled 0), Incomplete: 1, Skipped: 0"
  childReferences:
  - apiVersion: tekton.dev/v1
    kind: TaskRun
    name: pr-unmatrixed-pt
    pipelineTaskName: unmatrixed-pt
  - apiVersion: tekton.dev/v1beta1
    kind: CustomRun
    name: pr-platforms-and-browsers-0
    pipelineTaskName: platforms-and-browsers
  - apiVersion: tekton.dev/v1beta1
    kind: CustomRun
    name: pr-platforms-and-browsers-1
    pipelineTaskName: platforms-and-browsers
  - apiVersion: tekton.dev/v1beta1
    kind: CustomRun
    name: pr-platforms-and-browsers-2
    pipelineTaskName: platforms-and-browsers
  - apiVersion: tekton.dev/v1beta1
    kind: CustomRun
    name: pr-platforms-and-browsers-3
    pipelineTaskName: platforms-and-browsers
  - apiVersion: tekton.dev/v1beta1
    kind: CustomRun
    name: pr-platforms-and-browsers-4
    pipelineTaskName: platforms-and-browsers
  - apiVersion: tekton.dev/v1beta1
    kind: CustomRun
    name: pr-platforms-and-browsers-5
    pipelineTaskName: platforms-and-browsers
  - apiVersion: tekton.dev/v1beta1
    kind: CustomRun
    name: pr-platforms-and-browsers-6
    pipelineTaskName: platforms-and-browsers
  - apiVersion: tekton.dev/v1beta1
    kind: CustomRun
    name: pr-platforms-and-browsers-7
    pipelineTaskName: platforms-and-browsers
  - apiVersion: tekton.dev/v1beta1
    kind: CustomRun
    name: pr-platforms-and-browsers-8
    pipelineTaskName: platforms-and-browsers
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
`),
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pr := parse.MustParseV1PipelineRun(t, fmt.Sprintf(`
metadata:
  name: pr
  namespace: foo
spec:
  taskRunTemplate:
    serviceAccountName: test-sa
  pipelineRef:
    name: %s
`, tt.name))
			d := test.Data{
				PipelineRuns: []*v1.PipelineRun{pr},
				Pipelines:    []*v1.Pipeline{tt.p},
				Tasks:        []*v1.Task{task},
				ConfigMaps:   cms,
			}
			if tt.tr != nil {
				d.TaskRuns = []*v1.TaskRun{tt.tr}
			}
			prt := newPipelineRunTest(t, d)
			defer prt.Cancel()

			_, clients := prt.reconcileRun("foo", "pr", []string{}, false)
			customRuns, err := clients.Pipeline.TektonV1beta1().CustomRuns("foo").List(prt.TestAssets.Ctx, metav1.ListOptions{
				LabelSelector: fmt.Sprintf("tekton.dev/pipelineRun=pr,tekton.dev/pipeline=%s,tekton.dev/pipelineTask=platforms-and-browsers", tt.name),
				Limit:         1,
			})
			if err != nil {
				t.Fatalf("Failure to list TaskRun's %s", err)
			}

			if len(customRuns.Items) != 9 {
				t.Fatalf("Expected 9 TaskRuns got %d", len(customRuns.Items))
			}

			for i := range customRuns.Items {
				expectedTaskRun := expectedCustomRuns[i]
				expectedTaskRun.Labels["tekton.dev/pipeline"] = tt.name
				expectedTaskRun.Labels["tekton.dev/memberOf"] = tt.memberOf
				if d := cmp.Diff(expectedTaskRun, &customRuns.Items[i], ignoreResourceVersion, ignoreTypeMeta, cmpopts.EquateEmpty()); d != "" {
					t.Errorf("expected to see TaskRun %v created. Diff %s", expectedCustomRuns[i].Name, diff.PrintWantGot(d))
				}
			}

			pipelineRun, err := clients.Pipeline.TektonV1().PipelineRuns("foo").Get(prt.TestAssets.Ctx, "pr", metav1.GetOptions{})
			if err != nil {
				t.Fatalf("Got an error getting reconciled run out of fake client: %s", err)
			}

			if d := cmp.Diff(tt.expectedPipelineRun, pipelineRun, ignoreResourceVersion, ignoreTypeMeta, ignoreLastTransitionTime, ignoreStartTime, ignoreFinallyStartTime, cmpopts.EquateEmpty()); d != "" {
				t.Errorf("expected PipelineRun was not created. Diff %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestReconciler_FailedPipelineTaskMatrixIdxResultsOutOfBounds(t *testing.T) {
	names.TestingSeed()
	task := parse.MustParseV1Task(t, `
metadata:
  name: mytask
  namespace: foo
spec:
  params:
    - name: platform
      default: mac
  steps:
    - name: echo
      image: alpine
      script: |
        echo "$(params.platform)"
`)
	taskwithresults := parse.MustParseV1Task(t, `
metadata:
  name: taskwithresults
  namespace: foo
spec:
  results:
    - name: platforms
      type: array
  steps:
    - name: produce-a-list-of-platforms
      image: bash:latest
      script: |
        #!/usr/bin/env bash
        echo -n "[\"linux\",\"mac\",\"windows\"]" | tee $(results.platforms.path)
`)

	cms := []*corev1.ConfigMap{withEnabledAlphaAPIFields(newFeatureFlagsConfigMap())}
	cms = append(cms, withMaxMatrixCombinationsCount(newDefaultsConfigMap(), 10))
	tests := []struct {
		name  string
		pName string
		p     *v1.Pipeline
		tr    *v1.TaskRun
	}{{
		name:  "out of bounds array indexing for matrixed pipelinetask",
		pName: "p-dag",
		p: parse.MustParseV1Pipeline(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: foo
spec:
  tasks:
    - name: pt-with-result
      params:
       - name: platforms
         type: array
      taskRef:
        name: taskwithresults
    - name: echo-platforms
      params:
        - name: platforms
          value:
            - $(tasks.pt-with-result.results.platforms[0])
            - $(tasks.pt-with-result.results.platforms[3])
      taskRef:
        name: mytask
`, "p-dag")),
		tr: mustParseTaskRunWithObjectMeta(t,
			taskRunObjectMeta("pr-pt-with-result", "foo",
				"pr", "p-dag", "pt-with-result", false),
			`
spec:
  serviceAccountName: test-sa
  taskRef:
    name: taskwithresults
status:
 conditions:
  - type: Succeeded
    status: "True"
    reason: Succeeded
    message: All Tasks have completed executing
 results:
  - name: platforms
    value:
     - linux
     - mac
     - windows
`),
	}}
	for _, tt := range tests {
		t.Run(tt.pName, func(t *testing.T) {
			pr := parse.MustParseV1PipelineRun(t, fmt.Sprintf(`
metadata:
  name: pr
  namespace: foo
spec:
  taskRunTemplate:
    serviceAccountName: test-sa
  pipelineRef:
    name: %s
`, tt.pName))
			d := test.Data{
				PipelineRuns: []*v1.PipelineRun{pr},
				Pipelines:    []*v1.Pipeline{tt.p},
				Tasks:        []*v1.Task{task, taskwithresults},
				ConfigMaps:   cms,
			}
			if tt.tr != nil {
				d.TaskRuns = []*v1.TaskRun{tt.tr}
			}
			wantEvents :=
				[]string{
					"Normal Started",
					"Warning Failed PipelineRun foo/pr can't be Run; couldn't resolve all references: Array Result Index 3 for Task pt-with-result Result platforms is out of bound of size 3",
					"Warning InternalError 1 error occurred:",
				}
			prt := newPipelineRunTest(t, d)
			defer prt.Cancel()
			pipelineRun, clients := prt.reconcileRun(pr.Namespace, pr.Name, wantEvents /* wantEvents*/, true /* permanentError*/)
			// Validate the PR failed due to out of bounds array index reference
			checkPipelineRunConditionStatusAndReason(t, pipelineRun, corev1.ConditionFalse, "PipelineValidationFailed")
			taskRuns := getTaskRunsForPipelineTask(prt.TestAssets.Ctx, t, clients, pr.Namespace, pr.Name, "echo-platforms")
			// Validate no TaskRuns were created
			validateTaskRunsCount(t, taskRuns, 0)
		})
	}
}

func TestReconcile_SetDefaults(t *testing.T) {
	names.TestingSeed()

	namespace := "foo"
	prName := "test-pipeline-run-success"
	trName := "test-pipeline-run-success-unit-test-1"

	prs := []*v1.PipelineRun{parse.MustParseV1PipelineRun(t, `
metadata:
  name: test-pipeline-run-success
  namespace: foo
spec:
  params:
  - name: bar
    value: somethingmorefun
  pipelineRef:
    name: test-pipeline
  taskRunTemplate:
    serviceAccountName: test-sa
`)}
	ps := []*v1.Pipeline{parse.MustParseV1Pipeline(t, `
metadata:
  name: test-pipeline
  namespace: foo
spec:
  params:
  - default: somethingdifferent
    name: pipeline-param
  - default: revision
    name: rev-param
  - name: bar
  tasks:
  - name: unit-test-1
    params:
    - name: foo
      value: somethingfun
    - name: bar
      value: $(params.bar)
    taskRef:
      name: unit-test-task
  - name: unit-test-cluster-task
    params:
    - name: foo
      value: somethingfun
    - name: bar
      value: $(params.bar)
    taskRef:
      kind: ClusterTask
      name: unit-test-cluster-task
`)}
	ts := []*v1.Task{
		parse.MustParseV1Task(t, `
metadata:
  name: unit-test-task
  namespace: foo
spec:
  params:
  - name: foo
  - name: bar
`),
	}
	clusterTasks := []*v1beta1.ClusterTask{
		parse.MustParseClusterTask(t, `
metadata:
  name: unit-test-cluster-task
spec:
  params:
  - name: foo
  - name: bar
`),
	}

	d := test.Data{
		PipelineRuns: prs,
		Pipelines:    ps,
		Tasks:        ts,
		ClusterTasks: clusterTasks,
		ConfigMaps:   []*corev1.ConfigMap{newFeatureFlagsConfigMap()},
	}
	prt := newPipelineRunTest(t, d)
	defer prt.Cancel()

	wantEvents := []string{
		"Normal Started",
		"Normal Running Tasks Completed: 0",
	}
	reconciledRun, clients := prt.reconcileRun(namespace, prName, wantEvents, false)

	// Check that the expected TaskRun was created
	taskRuns := getTaskRunsForPipelineRun(prt.TestAssets.Ctx, t, clients, namespace, prName)
	// Ensure that there are 2 TaskRuns associated with this PipelineRun
	validateTaskRunsCount(t, taskRuns, 2)

	actual := getTaskRunByName(t, taskRuns, trName)
	expectedTaskRun := mustParseTaskRunWithObjectMeta(t,
		taskRunObjectMeta(trName, namespace, prName,
			"test-pipeline", "unit-test-1", false),
		`
spec:
  params:
  - name: foo
    value: somethingfun
  - name: bar
    value: somethingmorefun
  serviceAccountName: test-sa
  taskRef:
    name: unit-test-task
    kind: Task
`)
	// ignore IgnoreUnexported ignore both after and before steps fields
	if d := cmp.Diff(expectedTaskRun, actual, ignoreTypeMeta, ignoreResourceVersion); d != "" {
		t.Errorf("expected to see TaskRun %v created. Diff %s", expectedTaskRun, diff.PrintWantGot(d))
	}

	// This PipelineRun is in progress now and the status should reflect that
	checkPipelineRunConditionStatusAndReason(t, reconciledRun, corev1.ConditionUnknown, v1.PipelineRunReasonRunning.String())

	tr1Name := "test-pipeline-run-success-unit-test-1"
	tr2Name := "test-pipeline-run-success-unit-test-cluster-task"

	verifyTaskRunStatusesCount(t, reconciledRun.Status, 2)
	verifyTaskRunStatusesNames(t, reconciledRun.Status, tr1Name, tr2Name)
}

func TestReconcile_CreateTaskRunWithComputeResources(t *testing.T) {
	simplePipeline := parse.MustParseV1Pipeline(t, `
metadata:
  name: foo-pipeline
  namespace: default
spec:
  tasks:
  - name: 1st-task
    taskSpec:
      steps:
      - name: 1st-step
        image: foo-image
        command:
        - /foo-cmd
      - name: 2nd-step
        image: foo-image
        command:
        - /foo-cmd
`)

	testCases := []struct {
		name                     string
		pipeline                 *v1.Pipeline
		pipelineRun              *v1.PipelineRun
		expectedComputeResources []corev1.ResourceRequirements
	}{{
		name:     "only with requests",
		pipeline: simplePipeline,
		pipelineRun: parse.MustParseV1PipelineRun(t, `
metadata:
  name: foo-pipeline-run
  namespace: default
spec:
  pipelineRef:
    name: foo-pipeline
  taskRunSpecs:
  - pipelineTaskName: 1st-task
    computeResources:
      requests:
        cpu: "500m"
`),
		expectedComputeResources: []corev1.ResourceRequirements{{
			Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("500m")},
		}},
	}, {
		name:     "only with limits",
		pipeline: simplePipeline,
		pipelineRun: parse.MustParseV1PipelineRun(t, `
metadata:
  name: foo-pipeline-run
  namespace: default
spec:
  pipelineRef:
    name: foo-pipeline
  taskRunSpecs:
  - pipelineTaskName: 1st-task
    computeResources:
      limits:
        cpu: "2"
`),
		expectedComputeResources: []corev1.ResourceRequirements{{
			Limits: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("2")},
		}},
	}, {
		name:     "both with requests and limits",
		pipeline: simplePipeline,
		pipelineRun: parse.MustParseV1PipelineRun(t, `
metadata:
  name: foo-pipeline-run
  namespace: default
spec:
  pipelineRef:
    name: foo-pipeline
  taskRunSpecs:
  - pipelineTaskName: 1st-task
    computeResources:
      requests:
        cpu: "500m"
      limits:
        cpu: "2"
`),
		expectedComputeResources: []corev1.ResourceRequirements{{
			Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("500m")},
			Limits:   corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("2")},
		}},
	}, {
		name:     "both with cpu and memory",
		pipeline: simplePipeline,
		pipelineRun: parse.MustParseV1PipelineRun(t, `
metadata:
  name: foo-pipeline-run
  namespace: default
spec:
  pipelineRef:
    name: foo-pipeline
  taskRunSpecs:
  - pipelineTaskName: 1st-task
    computeResources:
      requests:
        cpu: "1"
        memory: "1Gi"
      limits:
        cpu: "2"
`),
		expectedComputeResources: []corev1.ResourceRequirements{{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("1"),
				corev1.ResourceMemory: resource.MustParse("1Gi")},
			Limits: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("2")},
		}},
	}}

	// verifyTaskLevelComputeResources verifies that the created TaskRuns have the expected compute resources
	verifyTaskLevelComputeResources := func(expectedComputeResources []corev1.ResourceRequirements, taskRuns []v1.TaskRun) error {
		if len(expectedComputeResources) != len(taskRuns) {
			return fmt.Errorf("expected %d compute resource requirements, got %d", len(expectedComputeResources), len(taskRuns))
		}
		for i, r := range expectedComputeResources {
			if d := cmp.Diff(r, *taskRuns[i].Spec.ComputeResources); d != "" {
				return fmt.Errorf("TaskRun #%d resource requirements don't match %s", i, diff.PrintWantGot(d))
			}
		}
		return nil
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			d := test.Data{
				PipelineRuns: []*v1.PipelineRun{tc.pipelineRun},
				Pipelines:    []*v1.Pipeline{tc.pipeline},
			}
			prt := newPipelineRunTest(t, d)
			defer prt.Cancel()

			reconciledRun, clients := prt.reconcileRun("default", tc.pipelineRun.Name, []string{}, false)

			if reconciledRun.Status.CompletionTime != nil {
				t.Errorf("Expected a CompletionTime on valid PipelineRun, but got nil")
			}

			TaskRunList, err := clients.Pipeline.TektonV1().TaskRuns("default").List(prt.TestAssets.Ctx, metav1.ListOptions{})
			if err != nil {
				t.Fatalf("Failure to list TaskRun's %s", err)
			}

			if err := verifyTaskLevelComputeResources(tc.expectedComputeResources, TaskRunList.Items); err != nil {
				t.Errorf("TaskRun \"%s\" failed to verify task-level compute resource requirements, because: %v", tc.name, err)
			}
		})
	}
}

func TestReconcile_FilterLabels(t *testing.T) {
	prs := []*v1.PipelineRun{parse.MustParseV1PipelineRun(t, `metadata:
  name: test-annotations
  namespace: foo
  annotations:
    chains.tekton.dev/signed: "true"
    results.tekton.dev/foo: "bar"
    tekton.dev/foo: "bar"
    foo: bar
spec:
  pipelineSpec:
    tasks:
      - name: hello
        taskSpec:
          steps:
          - image: busybox
            script: |
              echo "hello"
`)}

	ts := []*v1.Task{}
	cms := []*corev1.ConfigMap{newFeatureFlagsConfigMap()}

	d := test.Data{
		PipelineRuns: prs,
		Tasks:        ts,
		ConfigMaps:   cms,
	}
	prt := newPipelineRunTest(t, d)
	defer prt.Cancel()

	_, clients := prt.reconcileRun("foo", "test-annotations", []string{}, false)

	taskRunList, err := clients.Pipeline.TektonV1().TaskRuns("foo").List(prt.TestAssets.Ctx, metav1.ListOptions{})
	if err != nil {
		t.Fatalf("Failure to list TaskRun's %s", err)
	}
	if len(taskRunList.Items) != 1 {
		t.Fatalf("Should have only one TaskRun, got %d", len(taskRunList.Items))
	}
	tr := taskRunList.Items[0]
	vFoo, okFoo := tr.ObjectMeta.Annotations["foo"]
	vTektonFoo, okTektonFoo := tr.ObjectMeta.Annotations["tekton.dev/foo"]
	if len(tr.ObjectMeta.Annotations) != 2 || !okFoo || !okTektonFoo || vFoo != "bar" || vTektonFoo != "bar" {
		t.Fatalf("Should have filtered chains.tekton.dev/* and results.tekton.dev/* annotations and only have one annotations, got %v", tr.ObjectMeta.Annotations)
	}
}
func TestReconcile_CancelUnscheduled(t *testing.T) {
	pipelineRunName := "cancel-test-run"
	prs := []*v1.PipelineRun{parse.MustParseV1PipelineRun(t, `metadata:
  name: cancel-test-run
  namespace: foo
spec:
  pipelineSpec:
    tasks:
      - name: wait-1
        taskSpec:
          apiVersion: example.dev/v0
          kind: Wait
          params:
            - name: duration
              value: 1h
      - name: wait-2
        runAfter:
          - wait-1
        taskSpec:
          apiVersion: example.dev/v0
          kind: Wait
          params:
            - name: duration
              value: 10s
      - name: wait-3
        runAfter:
          - wait-1
        taskRef:
          name: hello-world
`)}

	ts := []*v1.Task{simpleHelloWorldTask}

	cms := []*corev1.ConfigMap{newFeatureFlagsConfigMap()}

	d := test.Data{
		PipelineRuns: prs,
		Tasks:        ts,
		ConfigMaps:   cms,
	}
	prt := newPipelineRunTest(t, d)
	defer prt.Cancel()

	pr, clients := prt.reconcileRun("foo", pipelineRunName, []string{}, false)

	var taskrunsCount, customrunsCount int
	for _, childRef := range pr.Status.ChildReferences {
		switch childRef.Kind {
		case taskRun:
			taskrunsCount++
		case customRun:
			customrunsCount++
		}
	}
	if taskrunsCount > 0 {
		t.Errorf("Expected no TaskRuns in status, but found %d", taskrunsCount)
	}
	if customrunsCount > 1 {
		t.Errorf("Expected one CustomRuns in status, but found %d", customrunsCount)
	}

	err := cancelPipelineRun(prt.TestAssets.Ctx, logtesting.TestLogger(t), pr, clients.Pipeline)
	if err != nil {
		t.Fatalf("Error found: %v", err)
	}
}

func TestReconcile_verifyResolved_V1beta1Pipeline_NoError(t *testing.T) {
	resolverName := "foobar"

	ts := parse.MustParseV1beta1Task(t, `
metadata:
  name: test-task
  namespace: foo
spec:
  steps:
    - name: simple-step
      image: foo
      command: ["/mycmd"]
      env:
       - name: foo
         value: bar
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

	ps := parse.MustParseV1beta1Pipeline(t, fmt.Sprintf(`
metadata:
  name: test-pipeline
  namespace: foo
spec:
  tasks:
    - name: test-1
      taskRef:
        resolver: %s
`, resolverName))

	signedPipeline, err := test.GetSignedV1beta1Pipeline(ps, signer, "test-pipeline")
	if err != nil {
		t.Fatal("fail to sign pipeline", err)
	}
	signedPipelineBytes, err := yaml.Marshal(signedPipeline)
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

	prs := parse.MustParseV1PipelineRun(t, fmt.Sprintf(`
metadata:
  name: test-pipelinerun
  namespace: foo
  selfLink: /pipeline/1234
spec:
  pipelineRef:
    resolver: %s
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

			pipelineReq := getResolvedResolutionRequest(t, resolverName, signedPipelineBytes, prs.Namespace, prs.Name)
			taskReq := getResolvedResolutionRequest(t, resolverName, signedTaskBytes, prs.Namespace, prs.Name+"-"+ps.Spec.Tasks[0].Name)

			d := test.Data{
				PipelineRuns:         []*v1.PipelineRun{prs},
				VerificationPolicies: tc.verificationPolicies,
				ConfigMaps:           cms,
				ResolutionRequests:   []*resolutionv1beta1.ResolutionRequest{&pipelineReq, &taskReq},
			}
			prt := newPipelineRunTest(t, d)
			defer prt.Cancel()

			reconciledRun, _ := prt.reconcileRun("foo", "test-pipelinerun", []string{}, false)

			checkPipelineRunConditionStatusAndReason(t, reconciledRun, corev1.ConditionUnknown, v1beta1.PipelineRunReasonRunning.String())
			gotVerificationCondition := reconciledRun.Status.GetCondition(trustedresources.ConditionTrustedResourcesVerified)
			if d := cmp.Diff(tc.wantTrustedResourcesCondition, gotVerificationCondition, ignoreLastTransitionTime); d != "" {
				t.Error(diff.PrintWantGot(d))
			}
		})
	}
}

func TestReconcile_verifyResolved_V1beta1Pipeline_Error(t *testing.T) {
	resolverName := "foobar"

	// Case1: unsigned Pipeline refers to unsigned Task
	unsignedTask := parse.MustParseV1beta1Task(t, `
metadata:
  name: test-task
  namespace: foo
spec:
  steps:
    - name: simple-step
      image: foo
      command: ["/mycmd"]
      env:
       - name: foo
         value: bar
`)
	unsignedTaskBytes, err := yaml.Marshal(unsignedTask)
	if err != nil {
		t.Fatal("fail to marshal task", err)
	}

	unsignedPipeline := parse.MustParseV1beta1Pipeline(t, fmt.Sprintf(`
metadata:
  name: test-pipeline
  namespace: foo
spec:
  tasks:
    - name: test-1
      taskRef:
        resolver: %s
`, resolverName))
	unsignedPipelineBytes, err := yaml.Marshal(unsignedPipeline)
	if err != nil {
		t.Fatal("fail to marshal task", err)
	}

	// Case2: signed Pipeline refers to unsigned Task
	signer, _, vps := test.SetupMatchAllVerificationPolicies(t, unsignedTask.Namespace)
	signedPipelineWithUnsignedTask, err := test.GetSignedV1beta1Pipeline(unsignedPipeline, signer, "test-pipeline")
	if err != nil {
		t.Fatal("fail to sign pipeline", err)
	}
	signedPipelineWithUnsignedTaskBytes, err := yaml.Marshal(signedPipelineWithUnsignedTask)
	if err != nil {
		t.Fatal("fail to marshal task", err)
	}

	// Case3: signed Pipeline refers to modified Task
	signedTask, err := test.GetSignedV1beta1Task(unsignedTask, signer, "test-task")
	if err != nil {
		t.Fatal("fail to sign task", err)
	}
	signedTaskBytes, err := yaml.Marshal(signedTask)
	if err != nil {
		t.Fatal("fail to marshal task", err)
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

	ps := parse.MustParseV1beta1Pipeline(t, fmt.Sprintf(`
metadata:
  name: test-pipeline
  namespace: foo
spec:
  tasks:
    - name: test-1
      taskRef:
        resolver: %s
`, resolverName))
	signedPipelineWithModifiedTask, err := test.GetSignedV1beta1Pipeline(ps, signer, "test-pipeline")
	if err != nil {
		t.Fatal("fail to sign pipeline", err)
	}
	signedPipelineWithModifiedTaskBytes, err := yaml.Marshal(signedPipelineWithModifiedTask)
	if err != nil {
		t.Fatal("fail to marshal task", err)
	}

	// Case4: modified Pipeline refers to signed Task
	ps = parse.MustParseV1beta1Pipeline(t, fmt.Sprintf(`
metadata:
  name: test-pipeline
  namespace: foo
spec:
  tasks:
    - name: test-1
      taskRef:
        resolver: %s
`, resolverName))
	signedPipeline, err := test.GetSignedV1beta1Pipeline(ps, signer, "test-pipeline")
	if err != nil {
		t.Fatal("fail to sign pipeline", err)
	}
	modifiedPipeline := signedPipeline.DeepCopy()
	if modifiedPipeline.Annotations == nil {
		modifiedPipeline.Annotations = make(map[string]string)
	}
	modifiedPipeline.Annotations["random"] = "attack"
	modifiedPipelineBytes, err := yaml.Marshal(modifiedPipeline)
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

	pr := parse.MustParseV1PipelineRun(t, fmt.Sprintf(`
metadata:
  name: test-pipelinerun
  namespace: foo
  selfLink: /pipeline/1234
spec:
  pipelineRef:
    resolver: %s
  serviceAccountName: default
`, resolverName))

	testCases := []struct {
		name          string
		pipelinerun   []*v1beta1.PipelineRun
		pipelineBytes []byte
		taskBytes     []byte
	}{
		{
			name:          "unsigned pipeline fails verification",
			pipelineBytes: unsignedPipelineBytes,
			taskBytes:     unsignedTaskBytes,
		},
		{
			name:          "signed pipeline with unsigned task fails verification",
			pipelineBytes: signedPipelineWithUnsignedTaskBytes,
			taskBytes:     unsignedTaskBytes,
		},
		{
			name:          "signed pipeline with modified task fails verification",
			pipelineBytes: signedPipelineWithModifiedTaskBytes,
			taskBytes:     modifiedTaskBytes,
		},
		{
			name:          "modified pipeline with signed task fails verification",
			pipelineBytes: modifiedPipelineBytes,
			taskBytes:     signedTaskBytes,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			pipelineReq := getResolvedResolutionRequest(t, resolverName, tc.pipelineBytes, pr.Namespace, pr.Name)
			taskReq := getResolvedResolutionRequest(t, resolverName, tc.taskBytes, pr.Namespace, pr.Name+"-"+ps.Spec.Tasks[0].Name)

			d := test.Data{
				PipelineRuns:         []*v1.PipelineRun{pr},
				ConfigMaps:           cms,
				VerificationPolicies: vps,
				ResolutionRequests:   []*resolutionv1beta1.ResolutionRequest{&pipelineReq, &taskReq},
			}
			prt := newPipelineRunTest(t, d)
			defer prt.Cancel()

			reconciledRun, _ := prt.reconcileRun("foo", "test-pipelinerun", []string{}, true)
			checkPipelineRunConditionStatusAndReason(t, reconciledRun, corev1.ConditionFalse, ReasonResourceVerificationFailed)
			gotVerificationCondition := reconciledRun.Status.GetCondition(trustedresources.ConditionTrustedResourcesVerified)
			if gotVerificationCondition == nil || gotVerificationCondition.Status != corev1.ConditionFalse {
				t.Errorf("Expected to have false condition, but had %v", gotVerificationCondition)
			}
		})
	}
}

func TestReconcile_verifyResolved_V1Pipeline_NoError(t *testing.T) {
	resolverName := "foobar"

	ts := parse.MustParseV1Task(t, `
metadata:
  name: test-task
  namespace: foo
spec:
  steps:
    - name: simple-step
      image: foo
      command: ["/mycmd"]
      env:
       - name: foo
         value: bar
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

	ps := parse.MustParseV1Pipeline(t, fmt.Sprintf(`
metadata:
  name: test-pipeline
  namespace: foo
spec:
  tasks:
    - name: test-1
      taskRef:
        resolver: %s
`, resolverName))

	signedPipeline, err := getSignedV1Pipeline(ps, signer, "test-pipeline")
	if err != nil {
		t.Fatal("fail to sign pipeline", err)
	}
	signedPipelineBytes, err := yaml.Marshal(signedPipeline)
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

	prs := parse.MustParseV1PipelineRun(t, fmt.Sprintf(`
metadata:
  name: test-pipelinerun
  namespace: foo
  selfLink: /pipeline/1234
spec:
  pipelineRef:
    resolver: %s
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
						"enable-api-fields": config.BetaAPIFields,
					},
				},
			}

			pipelineReq := getResolvedResolutionRequest(t, resolverName, signedPipelineBytes, prs.Namespace, prs.Name)
			taskReq := getResolvedResolutionRequest(t, resolverName, signedTaskBytes, prs.Namespace, prs.Name+"-"+ps.Spec.Tasks[0].Name)

			d := test.Data{
				PipelineRuns:         []*v1.PipelineRun{prs},
				VerificationPolicies: tc.verificationPolicies,
				ConfigMaps:           cms,
				ResolutionRequests:   []*resolutionv1beta1.ResolutionRequest{&pipelineReq, &taskReq},
			}
			prt := newPipelineRunTest(t, d)
			defer prt.Cancel()

			reconciledRun, _ := prt.reconcileRun("foo", "test-pipelinerun", []string{}, false)

			checkPipelineRunConditionStatusAndReason(t, reconciledRun, corev1.ConditionUnknown, v1beta1.PipelineRunReasonRunning.String())
			gotVerificationCondition := reconciledRun.Status.GetCondition(trustedresources.ConditionTrustedResourcesVerified)
			if d := cmp.Diff(tc.wantTrustedResourcesCondition, gotVerificationCondition, ignoreLastTransitionTime); d != "" {
				t.Error(diff.PrintWantGot(d))
			}
		})
	}
}

func TestReconcile_verifyResolved_V1Pipeline_Error(t *testing.T) {
	resolverName := "foobar"

	// Case1: unsigned Pipeline refers to unsigned Task
	unsignedTask := parse.MustParseV1beta1Task(t, `
metadata:
  name: test-task
  namespace: foo
spec:
  steps:
    - name: simple-step
      image: foo
      command: ["/mycmd"]
      env:
       - name: foo
         value: bar
`)
	unsignedTaskBytes, err := yaml.Marshal(unsignedTask)
	if err != nil {
		t.Fatal("fail to marshal task", err)
	}

	unsignedPipeline := parse.MustParseV1beta1Pipeline(t, fmt.Sprintf(`
metadata:
  name: test-pipeline
  namespace: foo
spec:
  tasks:
    - name: test-1
      taskRef:
        resolver: %s
`, resolverName))
	unsignedPipelineBytes, err := yaml.Marshal(unsignedPipeline)
	if err != nil {
		t.Fatal("fail to marshal task", err)
	}

	// Case2: signed Pipeline refers to unsigned Task
	signer, _, vps := test.SetupMatchAllVerificationPolicies(t, unsignedTask.Namespace)
	signedPipelineWithUnsignedTask, err := test.GetSignedV1beta1Pipeline(unsignedPipeline, signer, "test-pipeline")
	if err != nil {
		t.Fatal("fail to sign pipeline", err)
	}
	signedPipelineWithUnsignedTaskBytes, err := yaml.Marshal(signedPipelineWithUnsignedTask)
	if err != nil {
		t.Fatal("fail to marshal task", err)
	}

	// Case3: signed Pipeline refers to modified Task
	signedTask, err := test.GetSignedV1beta1Task(unsignedTask, signer, "test-task")
	if err != nil {
		t.Fatal("fail to sign task", err)
	}
	signedTaskBytes, err := yaml.Marshal(signedTask)
	if err != nil {
		t.Fatal("fail to marshal task", err)
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

	ps := parse.MustParseV1beta1Pipeline(t, fmt.Sprintf(`
metadata:
  name: test-pipeline
  namespace: foo
spec:
  tasks:
    - name: test-1
      taskRef:
        resolver: %s
`, resolverName))
	signedPipelineWithModifiedTask, err := test.GetSignedV1beta1Pipeline(ps, signer, "test-pipeline")
	if err != nil {
		t.Fatal("fail to sign pipeline", err)
	}
	signedPipelineWithModifiedTaskBytes, err := yaml.Marshal(signedPipelineWithModifiedTask)
	if err != nil {
		t.Fatal("fail to marshal task", err)
	}

	// Case4: modified Pipeline refers to signed Task
	ps = parse.MustParseV1beta1Pipeline(t, fmt.Sprintf(`
metadata:
  name: test-pipeline
  namespace: foo
spec:
  tasks:
    - name: test-1
      taskRef:
        resolver: %s
`, resolverName))
	signedPipeline, err := test.GetSignedV1beta1Pipeline(ps, signer, "test-pipeline")
	if err != nil {
		t.Fatal("fail to sign pipeline", err)
	}
	modifiedPipeline := signedPipeline.DeepCopy()
	if modifiedPipeline.Annotations == nil {
		modifiedPipeline.Annotations = make(map[string]string)
	}
	modifiedPipeline.Annotations["random"] = "attack"
	modifiedPipelineBytes, err := yaml.Marshal(modifiedPipeline)
	if err != nil {
		t.Fatal("fail to marshal task", err)
	}

	cms := []*corev1.ConfigMap{
		{
			ObjectMeta: metav1.ObjectMeta{Name: config.GetFeatureFlagsConfigName(), Namespace: system.Namespace()},
			Data: map[string]string{
				"trusted-resources-verification-no-match-policy": config.FailNoMatchPolicy,
				"enable-api-fields": config.BetaAPIFields,
			},
		},
	}

	pr := parse.MustParseV1PipelineRun(t, fmt.Sprintf(`
metadata:
  name: test-pipelinerun
  namespace: foo
  selfLink: /pipeline/1234
spec:
  pipelineRef:
    resolver: %s
  serviceAccountName: default
`, resolverName))

	testCases := []struct {
		name          string
		pipelinerun   []*v1beta1.PipelineRun
		pipelineBytes []byte
		taskBytes     []byte
	}{
		{
			name:          "unsigned pipeline fails verification",
			pipelineBytes: unsignedPipelineBytes,
			taskBytes:     unsignedTaskBytes,
		},
		{
			name:          "signed pipeline with unsigned task fails verification",
			pipelineBytes: signedPipelineWithUnsignedTaskBytes,
			taskBytes:     unsignedTaskBytes,
		},
		{
			name:          "signed pipeline with modified task fails verification",
			pipelineBytes: signedPipelineWithModifiedTaskBytes,
			taskBytes:     modifiedTaskBytes,
		},
		{
			name:          "modified pipeline with signed task fails verification",
			pipelineBytes: modifiedPipelineBytes,
			taskBytes:     signedTaskBytes,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			pipelineReq := getResolvedResolutionRequest(t, resolverName, tc.pipelineBytes, pr.Namespace, pr.Name)
			taskReq := getResolvedResolutionRequest(t, resolverName, tc.taskBytes, pr.Namespace, pr.Name+"-"+ps.Spec.Tasks[0].Name)

			d := test.Data{
				PipelineRuns:         []*v1.PipelineRun{pr},
				ConfigMaps:           cms,
				VerificationPolicies: vps,
				ResolutionRequests:   []*resolutionv1beta1.ResolutionRequest{&pipelineReq, &taskReq},
			}
			prt := newPipelineRunTest(t, d)
			defer prt.Cancel()

			reconciledRun, _ := prt.reconcileRun("foo", "test-pipelinerun", []string{}, true)
			checkPipelineRunConditionStatusAndReason(t, reconciledRun, corev1.ConditionFalse, ReasonResourceVerificationFailed)
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

func TestReconcileForPipelineRunCreateRunFailed(t *testing.T) {
	// TestReconcileForPipelineRunCreateRunFailed runs "Reconcile" on a PipelineRun that has permanent error.
	// It verifies that reconcile is successful, no TaskRun is created, the PipelineTask is marked as CreateRunFailed, and the
	// pipeline status updated and events generated.
	ps := []*v1.Pipeline{}
	prs := []*v1.PipelineRun{parse.MustParseV1PipelineRun(t, `
metadata:
  name: test-pipeline-run-with-create-run-failed
  namespace: foo
spec:
  pipelineSpec:
    tasks:
    - name: hello-world
      taskRef:
        name: unit-test-task
      workspaces:
      - name: source
`)}

	ts := []*v1.Task{
		parse.MustParseV1Task(t, `
metadata:
  name: unit-test-task
  namespace: foo
spec:
  workspaces:
  - name: source
  steps:
  - image: alpine:latest
    name: test
    script: |
      echo "hello world"
    workspaces:
    - name: source
`)}

	trs := []*v1.TaskRun{}

	d := test.Data{
		PipelineRuns: prs,
		Pipelines:    ps,
		Tasks:        ts,
		TaskRuns:     trs,
	}
	prt := newPipelineRunTest(t, d)
	defer prt.Cancel()

	wantEvents := []string{
		"Normal Started",
		"Warning TaskRunsCreationFailed",
		"error creating TaskRuns called \\[test-pipeline-run-with-create-run-failed-hello-world]\\ for PipelineTask hello-world from PipelineRun test-pipeline-run-with-create-run-failed: expected workspace \"source\" to be provided by pipelinerun for pipeline task \"hello-world\"",
		"error creating TaskRuns called \\[test-pipeline-run-with-create-run-failed-hello-world]\\ for PipelineTask hello-world from PipelineRun test-pipeline-run-with-create-run-failed: expected workspace \"source\" to be provided by pipelinerun for pipeline task \"hello-world\"",
	}
	reconciledRun, _ := prt.reconcileRun("foo", "test-pipeline-run-with-create-run-failed", wantEvents, true)

	if reconciledRun.Status.CompletionTime == nil {
		t.Errorf("Expected a CompletionTime on invalid PipelineRun but was nil")
	}

	// The PipelineRun should be create run failed.
	if reconciledRun.Status.GetCondition(apis.ConditionSucceeded).Reason != "CreateRunFailed" {
		t.Errorf("Expected PipelineRun to be create run failed, but condition reason is %s", reconciledRun.Status.GetCondition(apis.ConditionSucceeded))
	}
}

func TestReconcileWithTimeoutsOfCompletedPipelineRun(t *testing.T) {
	// TestReconcileWithTimeoutsOfCompletedPipelineRun runs "Reconcile" on a PipelineRun that has completed and
	// which has a passed timeout.
	// It verifies that reconcile is successful, and the PipelineRun is not marked as timed out.
	ps := []*v1.Pipeline{parse.MustParseV1Pipeline(t, `
metadata:
  name: test-pipeline
  namespace: foo
spec:
  tasks:
  - name: hello-world-1
    taskRef:
      name: hello-world
`)}
	prs := []*v1.PipelineRun{parse.MustParseV1PipelineRun(t, `
metadata:
  name: test-pipeline-run-with-timeout
  namespace: foo
spec:
  pipelineRef:
    name: test-pipeline
  serviceAccountName: test-sa
  timeouts:
    pipeline: 2m
status:
  conditions:
  - lastTransitionTime: "2021-12-31T11:01:00Z"
    message: 'Tasks Completed: 1 (Failed: 0, Cancelled 0), Skipped: 0'
    reason: Succeeded
    status: "True"
    type: Succeeded
  completionTime: "2021-12-31T11:01:00Z"
  startTime: "2021-12-31T11:00:00Z"
  childReferences:
  - name: test-pipeline-run-with-timeout-hello-world-1
    pipelineTaskName: hello-world-1
    kind: TaskRun
`)}
	ts := []*v1.Task{simpleHelloWorldTask}

	trs := []*v1.TaskRun{mustParseTaskRunWithObjectMeta(t, taskRunObjectMeta("test-pipeline-run-with-timeout-hello-world-1", "foo", "test-pipeline-run-with-timeout",
		"test-pipeline", "hello-world-1", false), `
spec:
  serviceAccountName: test-sa
  taskRef:
    name: hello-world
    kind: Task
`)}

	d := test.Data{
		PipelineRuns: prs,
		Pipelines:    ps,
		Tasks:        ts,
		TaskRuns:     trs,
	}
	prt := newPipelineRunTest(t, d)
	defer prt.Cancel()

	wantEvents := []string{}
	reconciledRun, _ := prt.reconcileRun("foo", "test-pipeline-run-with-timeout", wantEvents, false)

	// The PipelineRun should not be timed out.
	if reconciledRun.Status.GetCondition(apis.ConditionSucceeded).Reason != "Succeeded" {
		t.Errorf("Expected PipelineRun to still be Succeeded, but reason is %s", reconciledRun.Status.GetCondition(apis.ConditionSucceeded))
	}
}

func getSignedV1Pipeline(unsigned *pipelinev1.Pipeline, signer signature.Signer, name string) (*pipelinev1.Pipeline, error) {
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

func getSignedV1Task(unsigned *pipelinev1.Task, signer signature.Signer, name string) (*pipelinev1.Task, error) {
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
