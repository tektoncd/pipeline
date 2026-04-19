package pipelinerun

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	th "github.com/tektoncd/pipeline/pkg/reconciler/testing"
	"github.com/tektoncd/pipeline/test"
	"github.com/tektoncd/pipeline/test/diff"
	"github.com/tektoncd/pipeline/test/names"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

// TestReconcile_ChildPipelineRunPipelineRefCycleDetection verifies that a self-referencing
// pipeline cycle (A -> A) is detected via ownerReferences walk-up and the PipelineRun is
// marked as failed.
func TestReconcile_ChildPipelineRunPipelineRefCycleDetection(t *testing.T) {
	names.TestingSeed()
	namespace := "foo"

	// Pipeline A references itself — a self-referencing cycle
	pipelineA := &v1.Pipeline{
		ObjectMeta: metav1.ObjectMeta{Name: "pipeline-a", Namespace: namespace},
		Spec: v1.PipelineSpec{
			Tasks: []v1.PipelineTask{{
				Name:        "ref-self",
				PipelineRef: &v1.PipelineRef{Name: "pipeline-a"},
			}},
		},
	}

	pr := &v1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cycle-pr",
			Namespace: namespace,
			Labels:    map[string]string{pipeline.PipelineLabelKey: "pipeline-a"},
		},
		Spec: v1.PipelineRunSpec{
			PipelineRef: &v1.PipelineRef{Name: "pipeline-a"},
		},
	}

	testData := test.Data{
		PipelineRuns: []*v1.PipelineRun{pr},
		Pipelines:    []*v1.Pipeline{pipelineA},
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

	// Pipeline A references Pipeline B, and Pipeline B references Pipeline A
	pipelineA := &v1.Pipeline{
		ObjectMeta: metav1.ObjectMeta{Name: "pipeline-a", Namespace: namespace},
		Spec: v1.PipelineSpec{
			Tasks: []v1.PipelineTask{{
				Name:        "ref-b",
				PipelineRef: &v1.PipelineRef{Name: "pipeline-b"},
			}},
		},
	}
	pipelineB := &v1.Pipeline{
		ObjectMeta: metav1.ObjectMeta{Name: "pipeline-b", Namespace: namespace},
		Spec: v1.PipelineSpec{
			Tasks: []v1.PipelineTask{{
				Name:        "ref-a",
				PipelineRef: &v1.PipelineRef{Name: "pipeline-a"},
			}},
		},
	}

	parentPR := &v1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "parent-pr",
			Namespace: namespace,
			Labels:    map[string]string{pipeline.PipelineLabelKey: "pipeline-a"},
		},
		Spec: v1.PipelineRunSpec{
			PipelineRef: &v1.PipelineRef{Name: "pipeline-a"},
		},
	}

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

	parentPipeline := &v1.Pipeline{
		ObjectMeta: metav1.ObjectMeta{Name: "parent-pipeline", Namespace: namespace},
		Spec: v1.PipelineSpec{
			Tasks: []v1.PipelineTask{{
				Name:        "ref-nonexistent",
				PipelineRef: &v1.PipelineRef{Name: "nonexistent-pipeline"},
			}},
		},
	}

	pr := &v1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "not-found-pr",
			Namespace: namespace,
		},
		Spec: v1.PipelineRunSpec{
			PipelineRef: &v1.PipelineRef{Name: "parent-pipeline"},
		},
	}

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
