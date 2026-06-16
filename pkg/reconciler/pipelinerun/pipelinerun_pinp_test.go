package pipelinerun

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	listers "github.com/tektoncd/pipeline/pkg/client/listers/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/reconciler/pipelinerun/resources"
	th "github.com/tektoncd/pipeline/pkg/reconciler/testing"
	"github.com/tektoncd/pipeline/test"
	"github.com/tektoncd/pipeline/test/diff"
	"github.com/tektoncd/pipeline/test/names"
	"go.opentelemetry.io/otel/trace"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ktesting "k8s.io/client-go/testing"
)

// TestReconcile_ChildPipelineRunPipelineSpec verifies the reconciliation logic for PipelineRuns that create child
// PipelineRuns from PipelineSpecs. It tests scenarios with one or more child PipelineRuns (with mixed TaskSpec and
// TaskRef), ensuring that:
//   - The parent PipelineRun is correctly marked as running after reconciliation.
//   - The correct number of child PipelineRuns are created and referenced in the parent status.
//   - The actual child PipelineRuns match the expected specifications.
//   - The expected events are emitted during reconciliation.
func TestReconcile_ChildPipelineRunPipelineSpec(t *testing.T) {
	names.TestingSeed()
	// GIVEN
	namespace := "foo"
	parentPipelineRunName := "parent-pipeline-run"
	parentPipeline1,
		parentPipelineRun1,
		expectedChildPipelineRun1 := th.OnePipelineInPipeline(t, namespace, parentPipelineRunName)
	_, parentPipeline2,
		parentPipelineRun2,
		expectedChildPipelineRun1And2 := th.TwoPipelinesInPipelineMixedTasks(t, namespace, parentPipelineRunName)
	expectedEvents := []string{
		"Normal Started",
		"Normal Running Tasks Completed: 0",
	}
	testCases := []struct {
		name                      string
		parentPipeline            *v1.Pipeline
		parentPipelineRun         *v1.PipelineRun
		expectedChildPipelineRuns []*v1.PipelineRun
	}{
		{
			name:                      "one child PipelineRun from PipelineSpec",
			parentPipeline:            parentPipeline1,
			parentPipelineRun:         parentPipelineRun1,
			expectedChildPipelineRuns: []*v1.PipelineRun{expectedChildPipelineRun1},
		},
		{
			name:                      "two child PipelineRuns from PipelineSpecs, one with TaskSpec and one with TaskRef",
			parentPipeline:            parentPipeline2,
			parentPipelineRun:         parentPipelineRun2,
			expectedChildPipelineRuns: expectedChildPipelineRun1And2,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			testData := test.Data{
				PipelineRuns: []*v1.PipelineRun{tc.parentPipelineRun},
				Pipelines:    []*v1.Pipeline{tc.parentPipeline},
				ConfigMaps:   th.NewAlphaFeatureFlagsConfigMapInSlice(),
			}

			// WHEN
			reconciledRun, childPipelineRuns := reconcileOncePinP(
				t,
				testData,
				namespace,
				tc.parentPipelineRun.Name,
				expectedEvents,
			)

			// THEN
			validatePinP(
				t,
				reconciledRun.Status,
				reconciledRun.Name,
				childPipelineRuns,
				tc.expectedChildPipelineRuns,
			)
		})
	}
}

func reconcileOncePinP(
	t *testing.T,
	testData test.Data,
	namespace,
	parentPipelineRunName string,
	expectedEvents []string,
) (*v1.PipelineRun, map[string]*v1.PipelineRun) {
	t.Helper()

	prt := newPipelineRunTest(t, testData)
	defer prt.Cancel()

	// reconcile once given parent PipelineRun
	reconciledRun, clients := prt.reconcileRun(
		namespace,
		parentPipelineRunName,
		expectedEvents,
		false,
	)

	// fetch created child PipelineRun(s)
	childPipelineRuns := getChildPipelineRunsForPipelineRun(
		prt.TestAssets.Ctx,
		t,
		clients,
		namespace,
		parentPipelineRunName,
	)

	return reconciledRun, childPipelineRuns
}

func getChildPipelineRunsForPipelineRun(
	ctx context.Context,
	t *testing.T,
	clients test.Clients,
	namespace, parentPipelineRunName string,
) map[string]*v1.PipelineRun {
	t.Helper()

	opt := metav1.ListOptions{
		LabelSelector: pipeline.PipelineRunLabelKey + "=" + parentPipelineRunName,
	}

	pipelineRunList, err := clients.
		Pipeline.
		TektonV1().
		PipelineRuns(namespace).
		List(ctx, opt)
	if err != nil {
		t.Fatalf("failed to list child PipelineRuns: %v", err)
	}

	result := make(map[string]*v1.PipelineRun)
	for _, pipelineRun := range pipelineRunList.Items {
		result[pipelineRun.Name] = &pipelineRun
	}

	return result
}

func validatePinP(
	t *testing.T,
	reconciledRunStatus v1.PipelineRunStatus,
	reconciledRunName string,
	childPipelineRuns map[string]*v1.PipelineRun,
	expectedChildPipelineRuns []*v1.PipelineRun,
) {
	t.Helper()

	// validate parent PipelineRun is in progress; the status should reflect that
	th.CheckPipelineRunConditionStatusAndReason(
		t,
		reconciledRunStatus,
		corev1.ConditionUnknown,
		v1.PipelineRunReasonRunning.String(),
	)

	// validate there is the correct number of child references with the correct names of the child PipelineRuns
	th.VerifyChildPipelineRunStatusesCount(t, reconciledRunStatus, len(expectedChildPipelineRuns))
	var expectedNames []string
	for _, cpr := range expectedChildPipelineRuns {
		expectedNames = append(expectedNames, cpr.Name)
	}
	th.VerifyChildPipelineRunStatusesNames(t, reconciledRunStatus, expectedNames...)

	validateChildPipelineRunCount(t, childPipelineRuns, len(expectedChildPipelineRuns))

	// validate the actual child PipelineRuns are as expected
	for _, expectedChild := range expectedChildPipelineRuns {
		actualChild := getChildPipelineRunByName(t, childPipelineRuns, expectedChild.Name)
		if d := cmp.Diff(expectedChild, actualChild, ignoreTypeMeta, ignoreResourceVersion); d != "" {
			t.Errorf("expected to see child PipelineRun %v created. Diff %s", expectedChild, diff.PrintWantGot(d))
		}

		// validate correct owner reference
		if len(actualChild.OwnerReferences) != 1 || actualChild.OwnerReferences[0].Name != reconciledRunName {
			t.Errorf("Child PipelineRun should be owned by parent %s", reconciledRunName)
		}
	}
}

func validateChildPipelineRunCount(t *testing.T, pipelineRuns map[string]*v1.PipelineRun, expectedCount int) {
	t.Helper()

	actualCount := len(pipelineRuns)
	if actualCount != expectedCount {
		t.Fatalf("Expected %d child PipelineRuns, got %d", expectedCount, actualCount)
	}
}

func getChildPipelineRunByName(t *testing.T, pipelineRuns map[string]*v1.PipelineRun, expectedName string) *v1.PipelineRun {
	t.Helper()

	pr, exist := pipelineRuns[expectedName]
	if !exist {
		t.Fatalf("Expected pipelinerun %s does not exist", expectedName)
	}

	return pr
}

// TestReconcile_NestedChildPipelineRuns verifies the reconciliation logic for multi-level nested PipelineRuns.
// It tests a parent pipeline that creates a child pipeline, which itself creates a grandchild pipeline.
// This test requires multiple reconciliation cycles:
//   - First reconciliation: Parent creates child pipeline
//   - Second reconciliation: Child creates grandchild pipeline
func TestReconcile_NestedChildPipelineRuns(t *testing.T) {
	names.TestingSeed()
	// GIVEN
	namespace := "foo"
	parentPipelineRunName := "parent-pipeline-run"
	parentPipeline,
		parentPipelineRun,
		expectedChildPipelineRun,
		expectedGrandchildPipelineRun := th.NestedPipelinesInPipeline(t, namespace, parentPipelineRunName)
	expectedEvents := []string{
		"Normal Started",
		"Normal Running Tasks Completed: 0",
	}
	testData := test.Data{
		PipelineRuns: []*v1.PipelineRun{parentPipelineRun},
		Pipelines:    []*v1.Pipeline{parentPipeline},
		ConfigMaps:   th.NewAlphaFeatureFlagsConfigMapInSlice(),
	}

	// WHEN
	// first reconcile parent PipelineRun once which creates the child
	reconciledRunParent, childPipelineRuns := reconcileOncePinP(
		t,
		testData,
		namespace,
		parentPipelineRun.Name,
		expectedEvents,
	)

	// THEN
	validatePinP(
		t,
		reconciledRunParent.Status,
		reconciledRunParent.Name,
		childPipelineRuns,
		[]*v1.PipelineRun{expectedChildPipelineRun},
	)

	// GIVEN
	// use the child from previous reconcile
	childPipelineRun := getChildPipelineRunByName(t, childPipelineRuns, expectedChildPipelineRun.Name)
	childTestData := test.Data{
		PipelineRuns: []*v1.PipelineRun{childPipelineRun},
		ConfigMaps:   th.NewAlphaFeatureFlagsConfigMapInSlice(),
	}

	// WHEN
	// second reconcile child PipelineRun which creates the grandchild
	reconciledRunChild, grandchildPipelineRuns := reconcileOncePinP(
		t,
		childTestData,
		namespace,
		childPipelineRun.Name,
		expectedEvents,
	)

	// THEN
	validatePinP(
		t,
		reconciledRunChild.Status,
		reconciledRunChild.Name,
		grandchildPipelineRuns,
		[]*v1.PipelineRun{expectedGrandchildPipelineRun},
	)
}

func TestReconcile_PropagateLabelsAndAnnotationsToChildPipelineRun(t *testing.T) {
	names.TestingSeed()
	// GIVEN
	namespace := "foo"
	parentPipeline,
		parentPipelineRun,
		expectedChildPipelineRun := th.OnePipelineInPipeline(t, namespace, "parent-pipeline-run")
	expectedChildPipelineRun = th.WithAnnotationAndLabel(expectedChildPipelineRun, false)
	testData := test.Data{
		PipelineRuns: []*v1.PipelineRun{th.WithAnnotationAndLabel(parentPipelineRun, true)},
		Pipelines:    []*v1.Pipeline{parentPipeline},
		ConfigMaps:   th.NewAlphaFeatureFlagsConfigMapInSlice(),
	}

	// WHEN
	reconciledRun, childPipelineRuns := reconcileOncePinP(
		t,
		testData,
		namespace,
		parentPipelineRun.Name,
		[]string{},
	)

	// THEN
	validatePinP(
		t,
		reconciledRun.Status,
		reconciledRun.Name,
		childPipelineRuns,
		[]*v1.PipelineRun{expectedChildPipelineRun},
	)
}

func TestReconcile_ChildPipelineRunHasDefaultLabels(t *testing.T) {
	names.TestingSeed()
	// GIVEN
	namespace := "foo"
	parentPipeline,
		parentPipelineRun,
		expectedChildPipelineRun := th.OnePipelineInPipeline(t, namespace, "parent-pipeline-run")
	expectedLabels := map[string]string{
		pipeline.PipelineRunLabelKey:    parentPipelineRun.Name,
		pipeline.PipelineLabelKey:       parentPipelineRun.Spec.PipelineRef.Name,
		pipeline.PipelineRunUIDLabelKey: string(parentPipelineRun.UID),
		pipeline.PipelineTaskLabelKey:   parentPipeline.Spec.Tasks[0].Name,
		pipeline.MemberOfLabelKey:       v1.PipelineTasks,
	}
	testData := test.Data{
		PipelineRuns: []*v1.PipelineRun{parentPipelineRun},
		Pipelines:    []*v1.Pipeline{parentPipeline},
		ConfigMaps:   th.NewAlphaFeatureFlagsConfigMapInSlice(),
	}

	// WHEN
	_, childPipelineRuns := reconcileOncePinP(
		t,
		testData,
		namespace,
		parentPipelineRun.Name,
		[]string{},
	)

	// THEN
	validateChildPipelineRunCount(t, childPipelineRuns, 1)

	actualLabels := childPipelineRuns[expectedChildPipelineRun.Name].Labels
	for k, v := range expectedLabels {
		if actualLabels[k] != v {
			t.Errorf("Expected label %q=%q on child PipelineRun, got %q", k, v, actualLabels[k])
		}
	}
}

func TestReconcile_ChildPipelineRunCreationError(t *testing.T) {
	names.TestingSeed()
	// GIVEN
	namespace := "foo"
	parentPipeline,
		parentPipelineRun,
		expectedChildPipelineRun := th.OnePipelineInPipeline(t, namespace, "parent-pipeline-run")
	testData := test.Data{
		PipelineRuns: []*v1.PipelineRun{parentPipelineRun},
		Pipelines:    []*v1.Pipeline{parentPipeline},
		ConfigMaps:   th.NewAlphaFeatureFlagsConfigMapInSlice(),
	}
	testCases := []struct {
		name        string
		creationErr clientError
	}{
		{
			name: "invalid",
			creationErr: clientError{
				verb:     "create",
				resource: "pipelineruns",
				actualError: apierrors.NewInvalid(
					schema.GroupKind{},
					expectedChildPipelineRun.Name,
					field.ErrorList{}),
			},
		},
		{
			name: "bad request",
			creationErr: clientError{
				verb:        "create",
				resource:    "pipelineruns",
				actualError: apierrors.NewBadRequest("bad request"),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// WHEN
			reconciledRun := reconcileWithError(
				t,
				testData,
				namespace,
				parentPipelineRun.Name,
				tc.creationErr,
			)

			// THEN
			th.CheckPipelineRunConditionStatusAndReason(
				t,
				reconciledRun.Status,
				corev1.ConditionFalse,
				"CreateRunFailed",
			)

			if reconciledRun.Status.CompletionTime == nil {
				t.Errorf("Expected a CompletionTime on invalid PipelineRun but was nil")
			}
		})
	}
}

type clientError struct {
	verb,
	resource string
	actualError error
}

func reconcileWithError(
	t *testing.T,
	testData test.Data,
	namespace,
	pipelineRunName string,
	clientErr clientError,
) *v1.PipelineRun {
	t.Helper()

	prt := newPipelineRunTest(t, testData)
	defer prt.Cancel()

	// simulate error when creating child resource
	prt.TestAssets.Clients.
		Pipeline.
		PrependReactor(
			clientErr.verb,
			clientErr.resource,
			func(_ ktesting.Action) (bool, runtime.Object, error) {
				return true, nil, clientErr.actualError
			},
		)

	reconciledRun, _ := prt.reconcileRun(
		namespace,
		pipelineRunName,
		[]string{},
		true,
	)

	return reconciledRun
}

// TestReconcile_NestedChildPipelineRunsWithPipelineRef verifies the reconciliation logic for
// multi-level nested PipelineRuns using PipelineRef. It tests a parent pipeline (A) that
// references child pipeline (B) via PipelineRef, which itself references grandchild pipeline (C)
// via PipelineRef. This test requires multiple reconciliation cycles:
//   - First reconciliation: Parent creates child PipelineRun with PipelineRef to B
//   - Second reconciliation: Child creates grandchild PipelineRun with PipelineRef to C
func TestReconcile_NestedChildPipelineRunsWithPipelineRef(t *testing.T) {
	names.TestingSeed()
	namespace := "foo"
	parentPipelineRunName := "parent-pipeline-run"
	parentPipeline,
		childPipeline,
		grandchildPipeline,
		parentPipelineRun,
		expectedChildPipelineRun,
		expectedGrandchildPipelineRun := th.NestedPipelineRefsInPipeline(t, namespace, parentPipelineRunName)
	expectedEvents := []string{
		"Normal Started",
		"Normal Running Tasks Completed: 0",
	}
	testData := test.Data{
		PipelineRuns: []*v1.PipelineRun{parentPipelineRun},
		Pipelines:    []*v1.Pipeline{parentPipeline, childPipeline, grandchildPipeline},
		ConfigMaps:   th.NewAlphaFeatureFlagsConfigMapInSlice(),
	}

	// first reconcile parent PipelineRun once which creates the child
	reconciledRunParent, childPipelineRuns := reconcileOncePinP(
		t,
		testData,
		namespace,
		parentPipelineRun.Name,
		expectedEvents,
	)

	validatePinP(
		t,
		reconciledRunParent.Status,
		reconciledRunParent.Name,
		childPipelineRuns,
		[]*v1.PipelineRun{expectedChildPipelineRun},
	)

	// use the child from previous reconcile
	childPipelineRun := getChildPipelineRunByName(t, childPipelineRuns, expectedChildPipelineRun.Name)
	childTestData := test.Data{
		PipelineRuns: []*v1.PipelineRun{childPipelineRun},
		Pipelines:    []*v1.Pipeline{childPipeline, grandchildPipeline},
		ConfigMaps:   th.NewAlphaFeatureFlagsConfigMapInSlice(),
	}

	// second reconcile child PipelineRun which creates the grandchild
	reconciledRunChild, grandchildPipelineRuns := reconcileOncePinP(
		t,
		childTestData,
		namespace,
		childPipelineRun.Name,
		expectedEvents,
	)

	validatePinP(
		t,
		reconciledRunChild.Status,
		reconciledRunChild.Name,
		grandchildPipelineRuns,
		[]*v1.PipelineRun{expectedGrandchildPipelineRun},
	)
}

// TestReconcile_ChildPipelineRunPipelineRefNotFound verifies that when a PipelineTask
// references a nonexistent Pipeline, the PipelineRun is marked as failed.
func TestReconcile_ChildPipelineRunPipelineRefNotFound(t *testing.T) {
	names.TestingSeed()
	namespace := "foo"

	parentPipeline, pr := th.OnePipelineRefMissing(t, namespace, "not-found-pr")

	testData := test.Data{
		PipelineRuns: []*v1.PipelineRun{pr},
		Pipelines:    []*v1.Pipeline{parentPipeline},
		ConfigMaps:   th.NewAlphaFeatureFlagsConfigMapInSlice(),
	}

	prt := newPipelineRunTest(t, testData)
	defer prt.Cancel()

	reconciledRun, _ := prt.reconcileRun(namespace, pr.Name, []string{}, true)

	// The run should be marked as failed since the referenced pipeline doesn't exist
	th.CheckPipelineRunConditionStatusAndReason(
		t,
		reconciledRun.Status,
		corev1.ConditionFalse,
		v1.PipelineRunReasonCouldntGetPipeline.String(),
	)
}

// TestReconcile_ChildPipelineRunPipelineRef verifies that a PipelineTask with a pipelineRef (instead
// of inline pipelineSpec) is correctly resolved and creates a child PipelineRun with the resolved spec.
func TestReconcile_ChildPipelineRunPipelineRef(t *testing.T) {
	names.TestingSeed()
	namespace := "foo"
	parentPipelineRunName := "parent-pipeline-run"

	parentPipeline, childPipeline, parentPipelineRun, expectedChildPipelineRun :=
		th.OnePipelineRefInPipeline(t, namespace, parentPipelineRunName)

	testData := test.Data{
		PipelineRuns: []*v1.PipelineRun{parentPipelineRun},
		Pipelines:    []*v1.Pipeline{parentPipeline, childPipeline},
		ConfigMaps:   th.NewAlphaFeatureFlagsConfigMapInSlice(),
	}

	reconciledRun, childPipelineRuns := reconcileOncePinP(
		t,
		testData,
		namespace,
		parentPipelineRunName,
		[]string{"Normal Started", "Normal Running Tasks Completed: 0"},
	)

	validatePinP(
		t,
		reconciledRun.Status,
		reconciledRun.Name,
		childPipelineRuns,
		[]*v1.PipelineRun{expectedChildPipelineRun},
	)
}

// TestReconcile_ChildPipelineRunPipelineRefWithParams verifies that params from the
// PipelineTask are propagated to the child PipelineRun spec.
func TestReconcile_ChildPipelineRunPipelineRefWithParams(t *testing.T) {
	names.TestingSeed()
	namespace := "foo"
	parentPipelineRunName := "parent-pipeline-run"

	parentPipeline, childPipeline, parentPipelineRun, expectedChildPipelineRun :=
		th.OnePipelineRefInPipelineWithParams(t, namespace, parentPipelineRunName)

	testData := test.Data{
		PipelineRuns: []*v1.PipelineRun{parentPipelineRun},
		Pipelines:    []*v1.Pipeline{parentPipeline, childPipeline},
		ConfigMaps:   th.NewAlphaFeatureFlagsConfigMapInSlice(),
	}

	reconciledRun, childPipelineRuns := reconcileOncePinP(
		t,
		testData,
		namespace,
		parentPipelineRunName,
		[]string{"Normal Started", "Normal Running Tasks Completed: 0"},
	)

	validatePinP(
		t,
		reconciledRun.Status,
		reconciledRun.Name,
		childPipelineRuns,
		[]*v1.PipelineRun{expectedChildPipelineRun},
	)
}

// TestReconcile_ChildPipelineRunPipelineRefWithWorkspaces verifies that workspace bindings
// from the parent PipelineRun are mapped and propagated to the child PipelineRun spec.
func TestReconcile_ChildPipelineRunPipelineRefWithWorkspaces(t *testing.T) {
	names.TestingSeed()
	namespace := "foo"
	parentPipelineRunName := "parent-pipeline-run"

	parentPipeline, childPipeline, parentPipelineRun, expectedChildPipelineRun :=
		th.OnePipelineRefInPipelineWithWorkspaces(t, namespace, parentPipelineRunName)

	testData := test.Data{
		PipelineRuns: []*v1.PipelineRun{parentPipelineRun},
		Pipelines:    []*v1.Pipeline{parentPipeline, childPipeline},
		ConfigMaps:   th.NewAlphaFeatureFlagsConfigMapInSlice(),
	}

	reconciledRun, childPipelineRuns := reconcileOncePinP(
		t,
		testData,
		namespace,
		parentPipelineRunName,
		[]string{"Normal Started", "Normal Running Tasks Completed: 0"},
	)

	validatePinP(
		t,
		reconciledRun.Status,
		reconciledRun.Name,
		childPipelineRuns,
		[]*v1.PipelineRun{expectedChildPipelineRun},
	)
}

// TestReconcile_ChildPipelineRunPipelineRefWithParamsAndWorkspaces verifies that both params
// and workspaces are propagated to the child PipelineRun spec.
func TestReconcile_ChildPipelineRunPipelineRefWithParamsAndWorkspaces(t *testing.T) {
	names.TestingSeed()
	namespace := "foo"
	parentPipelineRunName := "parent-pipeline-run"

	parentPipeline, childPipeline, parentPipelineRun, expectedChildPipelineRun :=
		th.OnePipelineRefInPipelineWithParamsAndWorkspaces(t, namespace, parentPipelineRunName)

	testData := test.Data{
		PipelineRuns: []*v1.PipelineRun{parentPipelineRun},
		Pipelines:    []*v1.Pipeline{parentPipeline, childPipeline},
		ConfigMaps:   th.NewAlphaFeatureFlagsConfigMapInSlice(),
	}

	reconciledRun, childPipelineRuns := reconcileOncePinP(
		t,
		testData,
		namespace,
		parentPipelineRunName,
		[]string{"Normal Started", "Normal Running Tasks Completed: 0"},
	)

	validatePinP(
		t,
		reconciledRun.Status,
		reconciledRun.Name,
		childPipelineRuns,
		[]*v1.PipelineRun{expectedChildPipelineRun},
	)
}

// TestReconcile_ChildPipelineRunsMultiplePipelineRefs verifies that a parent
// Pipeline with more than one pipelineRef PipelineTask produces one child
// PipelineRun per task in a single reconcile — exercising the loop over
// multiple child-pipeline references.
func TestReconcile_ChildPipelineRunsMultiplePipelineRefs(t *testing.T) {
	names.TestingSeed()
	namespace := "foo"

	parentPipeline, childPipelines, parentPipelineRun, _ :=
		th.MultiplePipelineRefsInPipeline(t, namespace, "parent-pipeline-run")

	testData := test.Data{
		PipelineRuns: []*v1.PipelineRun{parentPipelineRun},
		Pipelines:    append([]*v1.Pipeline{parentPipeline}, childPipelines...),
		ConfigMaps:   th.NewAlphaFeatureFlagsConfigMapInSlice(),
	}

	reconciledRun, childPipelineRuns := reconcileOncePinP(
		t,
		testData,
		namespace,
		parentPipelineRun.Name,
		[]string{"Normal Started", "Normal Running Tasks Completed: 0"},
	)

	th.CheckPipelineRunConditionStatusAndReason(
		t,
		reconciledRun.Status,
		corev1.ConditionUnknown,
		v1.PipelineRunReasonRunning.String(),
	)
	validateChildPipelineRunCount(t, childPipelineRuns, 2)
}

// TestReconcile_ChildPipelineRunsMixedPipelineRefSpecAndTaskRef verifies that a parent
// Pipeline mixing a pipelineRef PipelineTask, an inline pipelineSpec PipelineTask, and a
// regular taskRef task resolves all three: a child PipelineRun is created for the
// pipelineRef and pipelineSpec tasks and a TaskRun for the direct taskRef task.
func TestReconcile_ChildPipelineRunsMixedPipelineRefSpecAndTaskRef(t *testing.T) {
	names.TestingSeed()
	namespace := "foo"

	task, parentPipeline, childRef, parentPipelineRun, _, _ :=
		th.MixedPipelineRefSpecAndTaskRefInPipeline(t, namespace, "parent-pipeline-run")

	testData := test.Data{
		PipelineRuns: []*v1.PipelineRun{parentPipelineRun},
		Pipelines:    []*v1.Pipeline{parentPipeline, childRef},
		Tasks:        []*v1.Task{task},
		ConfigMaps:   th.NewAlphaFeatureFlagsConfigMapInSlice(),
	}

	reconciledRun, childPipelineRuns := reconcileOncePinP(
		t,
		testData,
		namespace,
		parentPipelineRun.Name,
		[]string{"Normal Started", "Normal Running Tasks Completed: 0"},
	)

	th.CheckPipelineRunConditionStatusAndReason(
		t,
		reconciledRun.Status,
		corev1.ConditionUnknown,
		v1.PipelineRunReasonRunning.String(),
	)
	// Two child PipelineRuns (pipelineRef + pipelineSpec); the direct taskRef task
	// produces a TaskRun, not a child PipelineRun.
	validateChildPipelineRunCount(t, childPipelineRuns, 2)
}

// TestReconcile_ChildPipelineRunPipelineRefWhenSkipped verifies that a pipelineRef
// PipelineTask guarded by a when expression that evaluates to false is skipped:
// no child PipelineRun is created and the parent PipelineRun completes
// successfully (reason Completed, since a task was skipped).
func TestReconcile_ChildPipelineRunPipelineRefWhenSkipped(t *testing.T) {
	names.TestingSeed()
	namespace := "foo"

	parentPipeline, childPipeline, parentPipelineRun :=
		th.PipelineRefInPipelineWhenSkipped(t, namespace, "parent-pipeline-run")

	testData := test.Data{
		PipelineRuns: []*v1.PipelineRun{parentPipelineRun},
		Pipelines:    []*v1.Pipeline{parentPipeline, childPipeline},
		ConfigMaps:   th.NewAlphaFeatureFlagsConfigMapInSlice(),
	}

	reconciledRun, childPipelineRuns := reconcileOncePinP(
		t,
		testData,
		namespace,
		parentPipelineRun.Name,
		[]string{"Normal Started", "Normal Succeeded"},
	)

	// No child PipelineRun is created for the skipped task, and the run succeeds.
	validateChildPipelineRunCount(t, childPipelineRuns, 0)
	th.CheckPipelineRunConditionStatusAndReason(
		t,
		reconciledRun.Status,
		corev1.ConditionTrue,
		v1.PipelineRunReasonCompleted.String(),
	)
}

// TestGetChildPipelineRunWorkspaces_Success exercises every branch of
// getChildPipelineRunWorkspaces that produces workspace bindings: no workspaces,
// explicit and fallback name mapping, multiple workspaces, unbound workspaces,
// PVC sources, the PVC-then-non-PVC iteration order, SubPath propagation, and
// the edge cases of fan-out and fully-unbound child workspaces. It mirrors the
// direct table-driven style of the sibling TestGetTaskrunWorkspaces_Success.
func TestGetChildPipelineRunWorkspaces_Success(t *testing.T) {
	tests := []struct {
		name             string
		parentWorkspaces []v1.WorkspaceBinding
		childWorkspaces  []v1.WorkspacePipelineTaskBinding
		want             []v1.WorkspaceBinding
	}{{
		name:            "no workspaces returns nil",
		childWorkspaces: nil,
		want:            nil,
	}, {
		name:             "single emptyDir workspace with explicit mapping",
		parentWorkspaces: []v1.WorkspaceBinding{{Name: "parent-ws", EmptyDir: &corev1.EmptyDirVolumeSource{}}},
		childWorkspaces:  []v1.WorkspacePipelineTaskBinding{{Name: "child-ws", Workspace: "parent-ws"}},
		want:             []v1.WorkspaceBinding{{Name: "child-ws", EmptyDir: &corev1.EmptyDirVolumeSource{}}},
	}, {
		name: "multiple workspaces",
		parentWorkspaces: []v1.WorkspaceBinding{
			{Name: "p1", EmptyDir: &corev1.EmptyDirVolumeSource{}},
			{Name: "p2", EmptyDir: &corev1.EmptyDirVolumeSource{}},
		},
		childWorkspaces: []v1.WorkspacePipelineTaskBinding{
			{Name: "c1", Workspace: "p1"},
			{Name: "c2", Workspace: "p2"},
		},
		want: []v1.WorkspaceBinding{
			{Name: "c1", EmptyDir: &corev1.EmptyDirVolumeSource{}},
			{Name: "c2", EmptyDir: &corev1.EmptyDirVolumeSource{}},
		},
	}, {
		name:             "empty Workspace field falls back to child workspace name",
		parentWorkspaces: []v1.WorkspaceBinding{{Name: "shared", EmptyDir: &corev1.EmptyDirVolumeSource{}}},
		childWorkspaces:  []v1.WorkspacePipelineTaskBinding{{Name: "shared"}},
		want:             []v1.WorkspaceBinding{{Name: "shared", EmptyDir: &corev1.EmptyDirVolumeSource{}}},
	}, {
		name:             "unbound workspace is skipped",
		parentWorkspaces: []v1.WorkspaceBinding{{Name: "data", EmptyDir: &corev1.EmptyDirVolumeSource{}}},
		childWorkspaces: []v1.WorkspacePipelineTaskBinding{
			{Name: "data", Workspace: "data"},
			{Name: "missing", Workspace: "does-not-exist"},
		},
		want: []v1.WorkspaceBinding{{Name: "data", EmptyDir: &corev1.EmptyDirVolumeSource{}}},
	}, {
		name:             "PVC-source workspace",
		parentWorkspaces: []v1.WorkspaceBinding{{Name: "pvc-ws", PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: "my-pvc"}}},
		childWorkspaces:  []v1.WorkspacePipelineTaskBinding{{Name: "child-pvc", Workspace: "pvc-ws"}},
		want:             []v1.WorkspaceBinding{{Name: "child-pvc", PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: "my-pvc"}}},
	}, {
		name: "PVC workspace then non-PVC workspace",
		parentWorkspaces: []v1.WorkspaceBinding{
			{Name: "pvc-ws", PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: "my-pvc"}},
			{Name: "ed-ws", EmptyDir: &corev1.EmptyDirVolumeSource{}},
		},
		childWorkspaces: []v1.WorkspacePipelineTaskBinding{
			{Name: "c-pvc", Workspace: "pvc-ws"},
			{Name: "c-ed", Workspace: "ed-ws"},
		},
		want: []v1.WorkspaceBinding{
			{Name: "c-pvc", PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: "my-pvc"}},
			{Name: "c-ed", EmptyDir: &corev1.EmptyDirVolumeSource{}},
		},
	}, {
		name:             "SubPath is combined from parent binding and pipelineTask binding",
		parentWorkspaces: []v1.WorkspaceBinding{{Name: "ws", EmptyDir: &corev1.EmptyDirVolumeSource{}, SubPath: "parent-sub"}},
		childWorkspaces:  []v1.WorkspacePipelineTaskBinding{{Name: "ws", Workspace: "ws", SubPath: "child-sub"}},
		want:             []v1.WorkspaceBinding{{Name: "ws", EmptyDir: &corev1.EmptyDirVolumeSource{}, SubPath: "parent-sub/child-sub"}},
	}, {
		name:             "same parent workspace mapped to two child workspaces",
		parentWorkspaces: []v1.WorkspaceBinding{{Name: "shared-vol", PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: "my-pvc"}}},
		childWorkspaces: []v1.WorkspacePipelineTaskBinding{
			{Name: "source", Workspace: "shared-vol"},
			{Name: "cache", Workspace: "shared-vol"},
		},
		want: []v1.WorkspaceBinding{
			{Name: "source", PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: "my-pvc"}},
			{Name: "cache", PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: "my-pvc"}},
		},
	}, {
		name:             "no child workspace name matches any parent workspace",
		parentWorkspaces: []v1.WorkspaceBinding{{Name: "data", EmptyDir: &corev1.EmptyDirVolumeSource{}}},
		childWorkspaces: []v1.WorkspacePipelineTaskBinding{
			{Name: "src"},
			{Name: "artifacts"},
		},
		want: nil,
	}, {
		name:             "parent PipelineRun has no workspaces",
		parentWorkspaces: nil,
		childWorkspaces:  []v1.WorkspacePipelineTaskBinding{{Name: "source", Workspace: "shared-vol"}},
		want:             nil,
	}}

	c := Reconciler{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pr := &v1.PipelineRun{
				ObjectMeta: metav1.ObjectMeta{Name: "parent-pr", Namespace: "foo"},
				Spec:       v1.PipelineRunSpec{Workspaces: tt.parentWorkspaces},
			}
			rpt := &resources.ResolvedPipelineTask{
				PipelineTask: &v1.PipelineTask{Name: "child-task", Workspaces: tt.childWorkspaces},
			}

			got, err := c.getChildPipelineRunWorkspaces(t.Context(), pr, rpt)
			if err != nil {
				t.Fatalf("getChildPipelineRunWorkspaces() unexpected error: %v", err)
			}
			if d := cmp.Diff(tt.want, got); d != "" {
				t.Errorf("getChildPipelineRunWorkspaces() bindings diff %s", diff.PrintWantGot(d))
			}
		})
	}
}

// TestGetChildPipelineRunWorkspaces_Failure verifies that an error from
// GetAffinityAssistantBehavior — the only error path in the function — is
// propagated to the caller. An unknown "coschedule" feature-flag value makes
// GetAffinityAssistantBehavior fail.
func TestGetChildPipelineRunWorkspaces_Failure(t *testing.T) {
	cfg := config.FromContextOrDefaults(t.Context())
	cfg.FeatureFlags.Coschedule = "bogus-coschedule"
	ctx := config.ToContext(t.Context(), cfg)

	c := Reconciler{}
	pr := &v1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{Name: "parent-pr", Namespace: "foo"},
		Spec: v1.PipelineRunSpec{
			Workspaces: []v1.WorkspaceBinding{{Name: "ws", EmptyDir: &corev1.EmptyDirVolumeSource{}}},
		},
	}
	rpt := &resources.ResolvedPipelineTask{
		PipelineTask: &v1.PipelineTask{
			Name:       "child-task",
			Workspaces: []v1.WorkspacePipelineTaskBinding{{Name: "ws", Workspace: "ws"}},
		},
	}

	if _, err := c.getChildPipelineRunWorkspaces(ctx, pr, rpt); err == nil {
		t.Fatal("getChildPipelineRunWorkspaces() expected an error for an invalid coschedule flag, got nil")
	}
}

// TestCreateChildPipelineRun_WorkspaceResolutionError verifies that an error
// from child workspace resolution is propagated by createChildPipelineRun
// instead of a child PipelineRun being created. An invalid coschedule flag in
// the context makes getChildPipelineRunWorkspaces fail. A pipelineSpec child
// task is used so cycle detection is skipped and workspace resolution is the
// first failing step.
func TestCreateChildPipelineRun_WorkspaceResolutionError(t *testing.T) {
	cfg := config.FromContextOrDefaults(t.Context())
	cfg.FeatureFlags.Coschedule = "bogus-coschedule"
	ctx := config.ToContext(t.Context(), cfg)

	c := &Reconciler{tracerProvider: trace.NewNoopTracerProvider()}
	pr := &v1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{Name: "parent-pr", Namespace: "foo"},
		Spec: v1.PipelineRunSpec{
			Workspaces: []v1.WorkspaceBinding{{Name: "ws", EmptyDir: &corev1.EmptyDirVolumeSource{}}},
		},
	}
	rpt := &resources.ResolvedPipelineTask{
		PipelineTask: &v1.PipelineTask{
			Name:         "child-task",
			PipelineSpec: &v1.PipelineSpec{},
			Workspaces:   []v1.WorkspacePipelineTaskBinding{{Name: "ws", Workspace: "ws"}},
		},
		ResolvedPipeline: resources.ResolvedPipeline{PipelineSpec: &v1.PipelineSpec{}},
	}

	if _, err := c.createChildPipelineRun(ctx, "child-pr", rpt, pr, nil); err == nil {
		t.Fatal("createChildPipelineRun() expected a workspace resolution error, got nil")
	}
}

// errPipelineRunLister is a minimal stub of listers.PipelineRunLister whose
// only purpose is to make Get return a fixed non-NotFound error. The standard
// indexer-backed lister installed by test.SeedTestData can only return
// NotFound or success, so it cannot exercise detectPipelineRefCycle's
// "lister returned a transient error" branch.
type errPipelineRunLister struct{ err error }

func (l *errPipelineRunLister) List(labels.Selector) ([]*v1.PipelineRun, error) {
	return nil, l.err
}

func (l *errPipelineRunLister) PipelineRuns(string) listers.PipelineRunNamespaceLister {
	return errPipelineRunNamespaceLister{err: l.err}
}

type errPipelineRunNamespaceLister struct{ err error }

func (l errPipelineRunNamespaceLister) List(labels.Selector) ([]*v1.PipelineRun, error) {
	return nil, l.err
}

func (l errPipelineRunNamespaceLister) Get(string) (*v1.PipelineRun, error) {
	return nil, l.err
}

// TestReconcile_ChildPipelineRunPipelineRefCycleDetection verifies that a self-referencing
// pipeline cycle (A -> A) is detected via ownerReferences walk-up and the PipelineRun is
// marked as failed.
func TestReconcile_ChildPipelineRunPipelineRefCycleDetection(t *testing.T) {
	names.TestingSeed()
	namespace := "foo"

	selfRefPipeline, pr := th.SelfReferencingPipelineRefCycle(t, namespace, "cycle-pr")

	testData := test.Data{
		PipelineRuns: []*v1.PipelineRun{pr},
		Pipelines:    []*v1.Pipeline{selfRefPipeline},
		ConfigMaps:   th.NewAlphaFeatureFlagsConfigMapInSlice(),
	}

	prt := newPipelineRunTest(t, testData)
	defer prt.Cancel()

	reconciledRun, _ := prt.reconcileRun(namespace, pr.Name, []string{}, true)

	// The run should be marked as failed due to cycle detection
	th.CheckPipelineRunConditionStatusAndReason(
		t,
		reconciledRun.Status,
		corev1.ConditionFalse,
		"CreateRunFailed",
	)
}

// TestReconcile_ChildPipelineRunPipelineRefCycleDetection_TwoLevel verifies that a two-level
// pipeline cycle (A → B → A) is detected via ownerReferences walk-up. This requires two
// reconciliation steps:
//  1. Reconcile A's PipelineRun — creates child PipelineRun for B (no cycle yet)
//  2. Reconcile B's PipelineRun — walks up ownerRefs, finds A's label matches target "pipeline-a" → cycle detected
func TestReconcile_ChildPipelineRunPipelineRefCycleDetection_TwoLevel(t *testing.T) {
	names.TestingSeed()
	namespace := "foo"

	pipelineA, pipelineB, parentPR := th.TwoLevelPipelineRefCycle(t, namespace, "parent-pr")

	// Step 1: Reconcile A's PipelineRun — should create child for B (no cycle)
	testData := test.Data{
		PipelineRuns: []*v1.PipelineRun{parentPR},
		Pipelines:    []*v1.Pipeline{pipelineA, pipelineB},
		ConfigMaps:   th.NewAlphaFeatureFlagsConfigMapInSlice(),
	}
	expectedEvents := []string{
		"Normal Started",
		"Normal Running Tasks Completed: 0",
	}

	_, childPipelineRuns := reconcileOncePinP(
		t,
		testData,
		namespace,
		parentPR.Name,
		expectedEvents,
	)

	// Verify that the child PipelineRun for B was created
	validateChildPipelineRunCount(t, childPipelineRuns, 1)
	var childPR *v1.PipelineRun
	for _, cpr := range childPipelineRuns {
		childPR = cpr
	}

	// Step 2: Reconcile B's PipelineRun — should detect cycle (A is in ancestor chain)
	// Include both the parent and child in the test data so the lister can walk up ownerRefs.
	childTestData := test.Data{
		PipelineRuns: []*v1.PipelineRun{parentPR, childPR},
		Pipelines:    []*v1.Pipeline{pipelineA, pipelineB},
		ConfigMaps:   th.NewAlphaFeatureFlagsConfigMapInSlice(),
	}

	prt := newPipelineRunTest(t, childTestData)
	defer prt.Cancel()

	reconciledChild, _ := prt.reconcileRun(namespace, childPR.Name, []string{}, true)

	// The child run should be marked as failed due to cycle detection
	th.CheckPipelineRunConditionStatusAndReason(
		t,
		reconciledChild.Status,
		corev1.ConditionFalse,
		"CreateRunFailed",
	)
}

// TestReconcile_ChildPipelineRunPipelineRefParentNotFound verifies that cycle
// detection terminates cleanly when a PipelineRun's ownerReference points at a
// parent that no longer exists (e.g. garbage-collected). Reconciling such a
// PipelineRun, the ancestor walk hits a NotFound from the lister, treats the
// chain as ended, and reconciliation proceeds normally: the child Pipeline is
// resolved and its PipelineRun created rather than the run being failed on a
// phantom cycle. This exercises the NotFound branch of detectPipelineRefCycle
// through the reconciler instead of calling it directly.
func TestReconcile_ChildPipelineRunPipelineRefParentNotFound(t *testing.T) {
	names.TestingSeed()
	namespace := "foo"

	// Reuse the standard one-level pipelineRef fixture: the "child-pr" PipelineRun
	// references parent-pipeline, whose pipelineRef task makes reconciling it run
	// cycle detection. The expected-child return value is unused here.
	parentPipeline, childPipeline, pr, _ := th.OnePipelineRefInPipeline(t, namespace, "child-pr")

	// Stamp the tekton.dev/pipeline label the reconciler would set on this run, so
	// the ancestor walk evaluates a real, non-target label (target is the child
	// "child-pipeline") before reaching the missing parent.
	pr.Labels = map[string]string{pipeline.PipelineLabelKey: "parent-pipeline"}
	// Own pr with a PipelineRun that is never seeded, so the ancestor walk hits a
	// NotFound from the lister and must terminate without reporting a cycle.
	controller := true
	pr.OwnerReferences = []metav1.OwnerReference{{
		APIVersion: "tekton.dev/v1",
		Kind:       "PipelineRun",
		Name:       "missing-parent",
		Controller: &controller,
	}}

	testData := test.Data{
		PipelineRuns: []*v1.PipelineRun{pr},
		Pipelines:    []*v1.Pipeline{parentPipeline, childPipeline},
		ConfigMaps:   th.NewAlphaFeatureFlagsConfigMapInSlice(),
	}

	// reconcileOncePinP expects the "Running" events and no permanent error;
	// a phantom cycle would instead fail the run with CreateRunFailed.
	reconciledRun, childPipelineRuns := reconcileOncePinP(
		t,
		testData,
		namespace,
		pr.Name,
		[]string{"Normal Started", "Normal Running Tasks Completed: 0"},
	)

	th.CheckPipelineRunConditionStatusAndReason(
		t,
		reconciledRun.Status,
		corev1.ConditionUnknown,
		v1.PipelineRunReasonRunning.String(),
	)
	validateChildPipelineRunCount(t, childPipelineRuns, 1)
}

// TestDetectPipelineRefCycle_OwnerNotPipelineRun verifies that the cycle walker
// returns nil when the controller-owner reference is set but its Kind is not
// "PipelineRun" — the second half of the `ownerRef == nil || ownerRef.Kind != "PipelineRun"`
// guard. The walker must break before consulting the lister; the sentinel
// errPipelineRunLister surfaces a regression if it ever does.
func TestDetectPipelineRefCycle_OwnerNotPipelineRun(t *testing.T) {
	namespace := "foo"

	// The 4th return value is the expected child PipelineRun: it already carries a
	// tekton.dev/pipeline label and a PipelineRun owner-ref, which is what the cycle
	// walker reads. Overriding the owner-ref to a non-PipelineRun (Job) kind makes
	// the walker break before it ever consults the lister.
	_, _, _, pr := th.OnePipelineRefInPipeline(t, namespace, "parent-pr")
	controller := true
	pr.OwnerReferences = []metav1.OwnerReference{{
		APIVersion: "batch/v1",
		Kind:       "Job",
		Name:       "some-job",
		Controller: &controller,
	}}

	r := &Reconciler{pipelineRunLister: &errPipelineRunLister{err: errors.New("lister must not be called")}}

	if err := r.detectPipelineRefCycle(pr, "some-other-pipeline"); err != nil {
		t.Fatalf("detectPipelineRefCycle returned unexpected error: %v", err)
	}
}

// TestDetectPipelineRefCycle_ParentLookupError verifies that a non-NotFound
// error from the lister is wrapped and returned to the caller so the
// PipelineRun is retried rather than misclassified as having no cycle.
// Indexer-backed listers cannot produce this error in normal test data, so we
// substitute a stub lister that always errors.
func TestDetectPipelineRefCycle_ParentLookupError(t *testing.T) {
	namespace := "foo"

	// The 4th return value is the expected child PipelineRun, whose PipelineRun
	// owner-ref points at "parent-pr"; the stub lister errors on that lookup, so
	// the walker must wrap and return the error.
	_, _, _, pr := th.OnePipelineRefInPipeline(t, namespace, "parent-pr")

	listerErr := errors.New("indexer not synced")
	r := &Reconciler{pipelineRunLister: &errPipelineRunLister{err: listerErr}}

	err := r.detectPipelineRefCycle(pr, "some-other-pipeline")
	if err == nil {
		t.Fatal("detectPipelineRefCycle returned nil; expected wrapped lister error")
	}
	if !errors.Is(err, listerErr) {
		t.Fatalf("returned error does not wrap the lister error: %v", err)
	}
	if !strings.Contains(err.Error(), "cycle detection in pipeline-in-pipeline") ||
		!strings.Contains(err.Error(), "parent-pr") {
		t.Fatalf("error message missing expected context: %v", err)
	}
}
