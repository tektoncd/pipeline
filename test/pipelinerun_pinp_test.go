//go:build e2e

/*
Copyright 2025 The Tekton Authors

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
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	th "github.com/tektoncd/pipeline/pkg/reconciler/testing"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/kmeta"
	knativetest "knative.dev/pkg/test"
)

// @test:execution=parallel
func TestPipelineRun_PinP_OneChildPipelineRunFromPipelineSpec(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	c, namespace := setup(ctx, t, requireAnyGate(map[string]string{
		"enable-api-fields": "alpha",
	}))
	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	// GIVEN
	t.Logf("Setting up test resources for one child PipelineRun from PipelineSpec in namespace %q", namespace)
	parentPipeline,
		parentPipelineRun,
		expectedChildPipelineRun := th.OnePipelineInPipeline(t, namespace, "parent-pipeline-run")
	parentPipelineRun = th.WithAnnotationAndLabel(parentPipelineRun, false)
	expectedKinds := createKindsMap(parentPipelineRun, []*v1.PipelineRun{expectedChildPipelineRun}, []*v1.Pipeline{parentPipeline})
	expectedEventsAmount := 3

	// WHEN
	createResourcesAndWaitForPipelineRunSuccess(ctx, t, c, namespace, []*v1.Pipeline{parentPipeline}, parentPipelineRun, nil)

	// THEN
	assertPinP(ctx, t, c, expectedChildPipelineRun)
	assertEvents(ctx, t, expectedEventsAmount, expectedKinds, c, namespace)
}

// createKindsMap returns the expected "PipelineRun" and "TaskRun" event source
// names for a PinP test: the parent PipelineRun, every child PipelineRun, and
// each leaf-task TaskRun reachable from the parent and child PipelineRuns. A
// PipelineRun's spec is resolved either inline (pipelineSpec) or by looking up
// its pipelineRef in the supplied pipelines; pass every Pipeline involved
// (parent and children) so ref-based specs can be resolved. Tasks that are
// themselves nested Pipelines are skipped — only leaf Tasks produce TaskRuns.
func createKindsMap(
	parentPipelineRun *v1.PipelineRun,
	childPipelineRuns []*v1.PipelineRun,
	pipelines []*v1.Pipeline,
) map[string][]string {
	prNames := []string{parentPipelineRun.Name}
	for _, cpr := range childPipelineRuns {
		prNames = append(prNames, cpr.Name)
	}

	pipelineSpecByName := make(map[string]v1.PipelineSpec, len(pipelines))
	for _, p := range pipelines {
		pipelineSpecByName[p.Name] = p.Spec
	}

	var trNames []string
	allPRs := append([]*v1.PipelineRun{parentPipelineRun}, childPipelineRuns...)
	for _, pr := range allPRs {
		// Resolve the PipelineRun's spec: inline or via ref.
		var spec *v1.PipelineSpec
		switch {
		case pr.Spec.PipelineSpec != nil:
			spec = pr.Spec.PipelineSpec
		case pr.Spec.PipelineRef != nil:
			if s, ok := pipelineSpecByName[pr.Spec.PipelineRef.Name]; ok {
				spec = &s
			}
		}
		if spec == nil {
			continue
		}

		// Leaf tasks (a Task, not a nested Pipeline) produce TaskRuns. Use
		// kmeta.ChildName — the same helper the reconciler uses — so derived
		// names match even when they are hashed past the 63-char limit.
		tasks := append([]v1.PipelineTask{}, spec.Tasks...)
		tasks = append(tasks, spec.Finally...)
		for _, task := range tasks {
			if task.PipelineSpec == nil && task.PipelineRef == nil {
				trNames = append(trNames, kmeta.ChildName(pr.Name, "-"+task.Name))
			}
		}
	}

	return map[string][]string{
		"PipelineRun": prNames,
		"TaskRun":     trNames,
	}
}

// assertPinP verifies that the expected child PipelineRun exists, succeeded,
// has labels/annotations propagated correctly from its parent, and matches the
// expected spec. It handles both PipelineSpec children (where the
// tekton.dev/pipeline label is the child PipelineRun's own name) and PipelineRef
// children (where it is the resolved Pipeline's name).
func assertPinP(
	ctx context.Context,
	t *testing.T,
	c *clients,
	expected *v1.PipelineRun,
) {
	t.Helper()

	t.Logf("Making sure the expected child PipelineRun %q was created", expected.Name)
	actual, err := c.V1PipelineRunClient.Get(ctx, expected.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Error getting child PipelineRun %s: %s", expected.Name, err)
	}
	th.CheckPipelineRunConditionStatusAndReason(
		t,
		actual.Status,
		corev1.ConditionTrue,
		v1.PipelineRunReasonSuccessful.String(),
	)
	directParentPrName := actual.OwnerReferences[0].Name
	t.Logf("Checking that labels were propagated correctly for child PipelineRun %q", actual.Name)
	if actual.Spec.PipelineRef != nil {
		// The expected tekton.dev/pipeline label is the resolved child Pipeline
		// name. For a local name ref it equals the ref name; for a resolver ref
		// (whose Name is empty) the caller supplies it on the expected labels.
		expectedPipelineName := expected.Labels[pipeline.PipelineLabelKey]
		if expectedPipelineName == "" {
			expectedPipelineName = actual.Spec.PipelineRef.Name
		}
		checkLabelPropagationToChildPipelineRunRef(ctx, t, c, directParentPrName, actual, expectedPipelineName)
	} else {
		checkLabelPropagationToChildPipelineRun(ctx, t, c, directParentPrName, actual)
	}
	t.Logf("Checking that annotations were propagated correctly for child PipelineRun %q", actual.Name)
	checkAnnotationPropagationToChildPipelineRun(ctx, t, c, directParentPrName, actual)

	// Spec diff: only run when the caller provided a non-minimal expected.Spec
	// (i.e. it specifies how the child Pipeline is selected). For minimal
	// expected PipelineRuns (just an ObjectMeta), the label/annotation check above
	// is the contract — the controller adds spec details we don't model in tests.
	// Ignored fields:
	//   - ignoreSAPipelineRunSpec: test cluster default vs explicit SA.
	//   - PipelineRunSpec.Timeouts: only set on the parent.
	//   - PipelineRunSpec.Workspaces: the reconciler synthesises PVC claim names
	//     for volumeClaimTemplate-backed parent workspaces; hashes aren't worth
	//     reconstructing in tests.
	//   - TaskRef.Kind: the webhook defaults it to "Task" when unset; factories
	//     don't always pre-set it.
	if expected.Spec.PipelineRef == nil && expected.Spec.PipelineSpec == nil {
		return
	}
	opts := []cmp.Option{
		ignoreSAPipelineRunSpec,
		cmpopts.IgnoreFields(v1.PipelineRunSpec{}, "Timeouts", "Workspaces"),
		cmpopts.IgnoreFields(v1.TaskRef{}, "Kind"),
		cmpopts.EquateEmpty(),
	}
	if diff := cmp.Diff(expected.Spec, actual.Spec, opts...); diff != "" {
		t.Fatalf("Unexpected child PipelineRun spec (-want, +got): %s", diff)
	}
}

// @test:execution=parallel
func TestPipelineRun_PinP_TwoChildPipelineRunsMixedTasks(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	c, namespace := setup(ctx, t, requireAnyGate(map[string]string{
		"enable-api-fields": "alpha",
	}))
	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	// GIVEN
	t.Logf("Setting up test resources for two child PipelineRuns (mixed tasks) in namespace %q", namespace)
	task,
		parentPipeline,
		parentPipelineRun,
		expectedChildPipelineRuns := th.TwoPipelinesInPipelineMixedTasks(t, namespace, "parent-pipeline-mixed")
	parentPipelineRun = th.WithAnnotationAndLabel(parentPipelineRun, false)
	expectedKinds := createKindsMap(parentPipelineRun, expectedChildPipelineRuns, []*v1.Pipeline{parentPipeline})
	expectedEventsAmount := 5

	// WHEN
	createResourcesAndWaitForPipelineRunSuccess(ctx, t, c, namespace, []*v1.Pipeline{parentPipeline}, parentPipelineRun, task)

	// THEN
	assertPinP(ctx, t, c, expectedChildPipelineRuns[0])
	assertPinP(ctx, t, c, expectedChildPipelineRuns[1])
	assertEvents(ctx, t, expectedEventsAmount, expectedKinds, c, namespace)
}

// @test:execution=parallel
func TestPipelineRun_PinP_TwoLevelDeepNestedChildPipelineRuns(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	c, namespace := setup(ctx, t, requireAnyGate(map[string]string{
		"enable-api-fields": "alpha",
	}))
	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	// GIVEN
	t.Logf("Setting up test resources for two level deep nested child PipelineRuns in namespace %q", namespace)
	parentPipeline,
		parentPipelineRun,
		expectedChildPipelineRun,
		expectedGrandchildPipelineRun := th.NestedPipelinesInPipeline(t, namespace, "parent-pipeline-nested")
	parentPipelineRun = th.WithAnnotationAndLabel(parentPipelineRun, false)
	expectedKinds := createKindsMap(
		parentPipelineRun,
		[]*v1.PipelineRun{
			expectedChildPipelineRun,
			expectedGrandchildPipelineRun,
		},
		[]*v1.Pipeline{parentPipeline})
	expectedEventsAmount := 4

	// WHEN
	createResourcesAndWaitForPipelineRunSuccess(ctx, t, c, namespace, []*v1.Pipeline{parentPipeline}, parentPipelineRun, nil)

	// THEN
	assertPinP(ctx, t, c, expectedChildPipelineRun)
	assertPinP(ctx, t, c, expectedGrandchildPipelineRun)
	assertEvents(ctx, t, expectedEventsAmount, expectedKinds, c, namespace)
}

// @test:execution=parallel
func TestPipelineRun_PinP_OneChildPipelineRunFromPipelineRef(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	c, namespace := setup(ctx, t, requireAnyGate(map[string]string{
		"enable-api-fields": "alpha",
	}))
	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	// GIVEN
	t.Logf("Setting up test resources for one child PipelineRun from PipelineRef in namespace %q", namespace)
	parentPipeline, childPipeline, parentPipelineRun, expectedChildPipelineRun :=
		th.OnePipelineRefInPipeline(t, namespace, "parent-pipeline-run")
	parentPipelineRun = th.WithAnnotationAndLabel(parentPipelineRun, false)
	expectedKinds := createKindsMap(parentPipelineRun, []*v1.PipelineRun{expectedChildPipelineRun}, []*v1.Pipeline{parentPipeline, childPipeline})
	expectedEventsAmount := 3

	// WHEN
	createResourcesAndWaitForPipelineRunSuccess(ctx, t, c, namespace, []*v1.Pipeline{parentPipeline, childPipeline}, parentPipelineRun, nil)

	// THEN
	assertPinP(ctx, t, c, expectedChildPipelineRun)
	assertEvents(ctx, t, expectedEventsAmount, expectedKinds, c, namespace)
}

// TestPipelineRun_PinP_ChildPipelineRunInheritsServiceAccount verifies that the parent
// PipelineRun's taskRunTemplate.serviceAccountName propagates to the child PipelineRun and
// reaches execution on the child's TaskRun (TEP-0056, issue #10180).
// @test:execution=parallel
func TestPipelineRun_PinP_ChildPipelineRunInheritsServiceAccount(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	c, namespace := setup(ctx, t, requireAnyGate(map[string]string{
		"enable-api-fields": "alpha",
	}))
	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	const serviceAccountName = "pinp-sa"

	// GIVEN
	t.Logf("Setting up custom ServiceAccount %q and PinP resources in namespace %q", serviceAccountName, namespace)
	createServiceAccountWithSecret(ctx, t, c, namespace, serviceAccountName, "pinp-sa-secret")
	parentPipeline, childPipeline, parentPipelineRun, expectedChildPipelineRun :=
		th.OnePipelineRefInPipelineWithServiceAccount(t, namespace, "parent-pipeline-run-sa", serviceAccountName)
	parentPipelineRun = th.WithAnnotationAndLabel(parentPipelineRun, false)
	expectedKinds := createKindsMap(parentPipelineRun, []*v1.PipelineRun{expectedChildPipelineRun}, []*v1.Pipeline{parentPipeline, childPipeline})
	expectedEventsAmount := 3

	// WHEN
	createResourcesAndWaitForPipelineRunSuccess(ctx, t, c, namespace, []*v1.Pipeline{parentPipeline, childPipeline}, parentPipelineRun, nil)

	// THEN
	// assertPinP validates the child's labels/annotations/status, but it deliberately
	// ignores ServiceAccountName in its spec diff (ignoreSAPipelineRunSpec), so the
	// ServiceAccount propagation is verified explicitly below.
	assertPinP(ctx, t, c, expectedChildPipelineRun)
	checkServiceAccountPropagationToChild(ctx, t, c, expectedChildPipelineRun.Name, serviceAccountName)
	assertEvents(ctx, t, expectedEventsAmount, expectedKinds, c, namespace)
}

// @test:execution=parallel
func TestPipelineRun_PinP_TwoLevelDeepNestedChildPipelineRunsWithPipelineRef(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	c, namespace := setup(ctx, t, requireAnyGate(map[string]string{
		"enable-api-fields": "alpha",
	}))
	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	// GIVEN
	t.Logf("Setting up test resources for two level deep nested child PipelineRuns with PipelineRef in namespace %q", namespace)
	parentPipeline, childPipeline, grandchildPipeline,
		parentPipelineRun,
		expectedChildPipelineRun,
		expectedGrandchildPipelineRun := th.NestedPipelineRefsInPipeline(t, namespace, "parent-pipeline-nested-ref")
	parentPipelineRun = th.WithAnnotationAndLabel(parentPipelineRun, false)
	// Only the three PipelineRun "Succeeded" events are asserted here.
	// grandchildPipeline is deliberately left out of the createKindsMap lookup so
	// the deeply-nested grandchild TaskRun is not required — its name is hashed
	// past 63 chars and this test's intent is the nested PipelineRun creation, not
	// that leaf TaskRun. (createKindsMap itself now hashes via kmeta.ChildName, so
	// it would derive the correct name if the pipeline were included.)
	expectedKinds := createKindsMap(parentPipelineRun,
		[]*v1.PipelineRun{expectedChildPipelineRun, expectedGrandchildPipelineRun},
		[]*v1.Pipeline{parentPipeline, childPipeline})
	expectedEventsAmount := 3

	// WHEN
	createResourcesAndWaitForPipelineRunSuccess(ctx, t, c, namespace, []*v1.Pipeline{parentPipeline, childPipeline, grandchildPipeline}, parentPipelineRun, nil)

	// THEN
	assertPinP(ctx, t, c, expectedChildPipelineRun)
	assertPinP(ctx, t, c, expectedGrandchildPipelineRun)
	assertEvents(ctx, t, expectedEventsAmount, expectedKinds, c, namespace)
}

// assertFailureMessage fetches the named PipelineRun and asserts that its
// Succeeded condition is false and that the condition message contains the
// given substring.
func assertFailureMessage(ctx context.Context, t *testing.T, c *clients, prName, expectedMessage string) {
	t.Helper()
	pr, err := c.V1PipelineRunClient.Get(ctx, prName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Error getting PipelineRun %s: %s", prName, err)
	}
	condition := pr.Status.GetCondition(apis.ConditionSucceeded)
	if condition == nil || condition.Status != corev1.ConditionFalse || !strings.Contains(condition.Message, expectedMessage) {
		t.Fatalf("Expected PipelineRun %s to be failed with message containing %q; got condition: %+v", prName, expectedMessage, condition)
	}
}

// checkLabelPropagationToChildPipelineRunRef checks label propagation for PipelineRef children.
// Unlike PipelineSpec children (where tekton.dev/pipeline = childPr.Name), PipelineRef children
// have tekton.dev/pipeline set to the resolved Pipeline name, which the caller passes in
// expectedPipelineName.
func checkLabelPropagationToChildPipelineRunRef(
	ctx context.Context,
	t *testing.T,
	c *clients,
	parentPrName string,
	childPr *v1.PipelineRun,
	expectedPipelineName string,
) {
	t.Helper()

	labels := make(map[string]string)

	parentPr, err := c.V1PipelineRunClient.Get(ctx, parentPrName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Couldn't get expected PipelineRun for %s: %s", childPr.Name, err)
	}

	// Copy non-overwritten labels from parent
	for key, val := range parentPr.ObjectMeta.Labels {
		if key == pipeline.MemberOfLabelKey ||
			key == pipeline.PipelineLabelKey ||
			key == pipeline.PipelineRunLabelKey ||
			key == pipeline.PipelineRunUIDLabelKey ||
			key == pipeline.PipelineTaskLabelKey {
			continue
		}
		labels[key] = val
	}

	// Child references the parent PipelineRun
	labels[pipeline.PipelineRunLabelKey] = parentPr.Name

	// For PipelineRef children, tekton.dev/pipeline is set to the resolved child
	// Pipeline name (not the child PR name like for PipelineSpec children). Assert
	// the exact expected value.
	if childPr.Labels[pipeline.PipelineLabelKey] != expectedPipelineName {
		t.Errorf("Expected tekton.dev/pipeline=%q, got %q",
			expectedPipelineName, childPr.Labels[pipeline.PipelineLabelKey])
	}
	labels[pipeline.PipelineLabelKey] = expectedPipelineName

	assertLabelsMatch(t, labels, childPr.ObjectMeta.Labels)
	t.Logf("Labels propagated from parent PipelineRun to child PipelineRun (PipelineRef): %#v", labels)
}

// createResourcesAndWaitForPipelineRun creates the given pipelines (and optional
// task), creates the PipelineRun, and waits until it reaches the supplied
// inState (e.g. PipelineRunSucceed, PipelineRunFailed, or a Chain of accessors).
// It is the single creation helper used by every PinP e2e test; the
// createResourcesAndWaitForPipelineRunSuccess / ...Failed decorators wrap it for
// the common success and failure cases.
func createResourcesAndWaitForPipelineRun(
	ctx context.Context,
	t *testing.T,
	c *clients,
	namespace string,
	pipelines []*v1.Pipeline,
	pipelineRun *v1.PipelineRun,
	task *v1.Task,
	inState ConditionAccessorFn,
	stateName string,
) {
	t.Helper()

	for _, p := range pipelines {
		if _, err := c.V1PipelineClient.Create(ctx, p, metav1.CreateOptions{}); err != nil {
			t.Fatalf("Failed to create Pipeline `%s`: %s", p.Name, err)
		}
	}

	if task != nil {
		if _, err := c.V1TaskClient.Create(ctx, task, metav1.CreateOptions{}); err != nil {
			t.Fatalf("Failed to create Task `%s`: %s", task.Name, err)
		}
	}

	if _, err := c.V1PipelineRunClient.Create(ctx, pipelineRun, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create PipelineRun `%s`: %s", pipelineRun.Name, err)
	}

	t.Logf("Waiting for PipelineRun %q in namespace %q to reach %s", pipelineRun.Name, namespace, stateName)
	if err := WaitForPipelineRunState(
		ctx,
		c,
		pipelineRun.Name,
		timeout,
		inState,
		stateName,
		v1Version,
	); err != nil {
		t.Fatalf("Error waiting for PipelineRun %s to reach %s: %s", pipelineRun.Name, stateName, err)
	}
}

// createResourcesAndWaitForPipelineRunSuccess creates the resources and waits
// for the PipelineRun to succeed. It is the common-case decorator over
// createResourcesAndWaitForPipelineRun.
func createResourcesAndWaitForPipelineRunSuccess(
	ctx context.Context,
	t *testing.T,
	c *clients,
	namespace string,
	pipelines []*v1.Pipeline,
	pipelineRun *v1.PipelineRun,
	task *v1.Task,
) {
	t.Helper()

	createResourcesAndWaitForPipelineRun(
		ctx,
		t,
		c,
		namespace,
		pipelines,
		pipelineRun,
		task,
		PipelineRunSucceed(pipelineRun.Name),
		"PipelineRunSuccess",
	)
}

// createResourcesAndWaitForPipelineRunFailed creates the resources and waits for
// the PipelineRun to fail. It is the common-case decorator over
// createResourcesAndWaitForPipelineRun.
func createResourcesAndWaitForPipelineRunFailed(
	ctx context.Context,
	t *testing.T,
	c *clients,
	namespace string,
	pipelines []*v1.Pipeline,
	pipelineRun *v1.PipelineRun,
	task *v1.Task,
) {
	t.Helper()

	createResourcesAndWaitForPipelineRun(
		ctx,
		t,
		c,
		namespace,
		pipelines,
		pipelineRun,
		task,
		PipelineRunFailed(pipelineRun.Name),
		"PipelineRunFailed",
	)
}

func assertEvents(
	ctx context.Context,
	t *testing.T,
	expectedEventsAmount int,
	matchKinds map[string][]string,
	c *clients,
	namespace string,
) {
	t.Helper()

	t.Logf(
		"Making sure %d events were created from parent PipelineRun, child PipelineRun and TaskRun with kinds %v",
		expectedEventsAmount,
		matchKinds,
	)

	events, err := collectMatchingEvents(
		ctx,
		c.KubeClient,
		namespace,
		matchKinds,
		"Succeeded",
	)
	if err != nil {
		t.Fatalf("Failed to collect matching events: %q", err)
	}
	if len(events) != expectedEventsAmount {
		collectedEvents := ""
		for i, event := range events {
			collectedEvents += fmt.Sprintf("%#v", event)
			if i < (len(events) - 1) {
				collectedEvents += ", "
			}
		}
		t.Fatalf(
			"Expected %d number of successful events from parent PipelineRun, child PipelineRun and "+
				"TaskRun but got %d; list of received events: %#v",
			expectedEventsAmount,
			len(events),
			collectedEvents,
		)
	}
}

// checkLabelPropagationToChildPipelineRun checks that labels are correctly propagating from
// Pipelines and PipelineRuns to child/grandchild PipelineRuns.
func checkLabelPropagationToChildPipelineRun(
	ctx context.Context,
	t *testing.T,
	c *clients,
	parentPrName string,
	childPr *v1.PipelineRun,
) {
	t.Helper()

	labels := make(map[string]string)

	parentPr, err := c.V1PipelineRunClient.Get(ctx, parentPrName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Couldn't get expected PipelineRun for %s: %s", childPr.Name, err)
	}

	// Does the parent PipelineRun have an owner? If not its the initial PipelineRun
	// and we have to check for labels propagated from the Pipeline.
	if parentPr.OwnerReferences == nil {
		p, err := c.V1PipelineClient.Get(ctx, parentPr.Spec.PipelineRef.Name, metav1.GetOptions{})
		if err != nil {
			t.Fatalf("Couldn't get expected Pipeline for %s: %s", parentPr.Name, err)
		}
		// Extract every label the Pipeline has.
		for key, val := range p.ObjectMeta.Labels {
			labels[key] = val
		}
		// This label is added to every PipelineRun by the PipelineRun controller.
		labels[pipeline.PipelineLabelKey] = p.Name
		// Check label propagation from Pipeline to parent PipelineRun.
		assertLabelsMatch(t, labels, parentPr.ObjectMeta.Labels)
		t.Logf("Labels propagated from Pipeline to PipelineRun: %#v", labels)
	}

	// Check label propagation from parent PipelineRun to child PipelineRun.
	for key, val := range parentPr.ObjectMeta.Labels {
		// Skip overwritten labels.
		if key == pipeline.MemberOfLabelKey ||
			key == pipeline.PipelineLabelKey ||
			key == pipeline.PipelineRunLabelKey ||
			key == pipeline.PipelineRunUIDLabelKey ||
			key == pipeline.PipelineTaskLabelKey {
			continue
		}

		labels[key] = val
	}

	// Child always references the parent PipelineRun its labels.
	labels[pipeline.PipelineRunLabelKey] = parentPr.Name
	// The parent PipelineRun references an existing Pipeline via PipelineRef
	// the child PipelineRun does not reference any existing Pipeline but it uses the
	// PipelineSpec embedded field, that is why its label "tekton.dev/pipeline:" is
	// set to its own name. Refer to "propagatePipelineNameLabelToPipelineRun" for
	// more implementation details.
	labels[pipeline.PipelineLabelKey] = childPr.Name
	assertLabelsMatch(t, labels, childPr.ObjectMeta.Labels)
	t.Logf("Labels propagated from parent PipelineRun to child PipelineRun: %#v", labels)
}

// checkAnnotationPropagationToChildPipelineRun checks that annotations are correctly propagating from
// Pipelines and PipelineRuns to child PipelineRuns.
func checkAnnotationPropagationToChildPipelineRun(
	ctx context.Context,
	t *testing.T,
	c *clients,
	parentPrName string,
	childPr *v1.PipelineRun,
) {
	t.Helper()

	annotations := make(map[string]string)
	parentPr, err := c.V1PipelineRunClient.Get(ctx, parentPrName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Couldn't get expected PipelineRun for %s: %s", childPr.Name, err)
	}

	// Does the parent PipelineRun have an owner? If not its the initial PipelineRun
	// and we have to check for annotations propagated from the Pipeline.
	if parentPr.OwnerReferences == nil {
		p, err := c.V1PipelineClient.Get(ctx, parentPr.Spec.PipelineRef.Name, metav1.GetOptions{})
		if err != nil {
			t.Fatalf("Couldn't get expected Pipeline for %s: %s", parentPr.Name, err)
		}
		for key, val := range p.ObjectMeta.Annotations {
			annotations[key] = val
		}

		assertAnnotationsMatch(t, annotations, parentPr.ObjectMeta.Annotations)
	}

	// Check annotation propagation to child PipelineRuns.
	for key, val := range parentPr.ObjectMeta.Annotations {
		annotations[key] = val
	}
	assertAnnotationsMatch(t, annotations, childPr.ObjectMeta.Annotations)

	if len(annotations) > 0 {
		t.Logf("Propagated annotations: %#v", annotations)
	}
}

// createServiceAccountWithSecret creates an Opaque Secret and a ServiceAccount that
// references it, mirroring the Secret+ServiceAccount setup in serviceaccount_test.go.
func createServiceAccountWithSecret(ctx context.Context, t *testing.T, c *clients, namespace, serviceAccountName, secretName string) {
	t.Helper()

	if _, err := c.KubeClient.CoreV1().Secrets(namespace).Create(ctx, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: secretName},
		Type:       corev1.SecretTypeOpaque,
		Data:       map[string][]byte{"foo": []byte("bar")},
	}, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create Secret %q: %s", secretName, err)
	}
	if _, err := c.KubeClient.CoreV1().ServiceAccounts(namespace).Create(ctx, &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: serviceAccountName},
		Secrets:    []corev1.ObjectReference{{Name: secretName}},
	}, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create ServiceAccount %q: %s", serviceAccountName, err)
	}
}

// checkServiceAccountPropagationToChild verifies that the parent's ServiceAccount propagated to
// the child PipelineRun and reached execution. It asserts the ServiceAccountName at three layers:
// the child PipelineRun's taskRunTemplate, the child's TaskRun(s), and the executing pod(s). The
// child PipelineRun spec alone is the struct the reconciler copied; the TaskRun and pod prove the
// ServiceAccount reached the Kubernetes scheduling layer (mirrors serviceaccount_test.go).
func checkServiceAccountPropagationToChild(ctx context.Context, t *testing.T, c *clients, childPipelineRunName, expectedServiceAccountName string) {
	t.Helper()

	childPr, err := c.V1PipelineRunClient.Get(ctx, childPipelineRunName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Couldn't get child PipelineRun %q: %s", childPipelineRunName, err)
	}
	if got := childPr.Spec.TaskRunTemplate.ServiceAccountName; got != expectedServiceAccountName {
		t.Errorf("Child PipelineRun %q taskRunTemplate.serviceAccountName = %q, want %q", childPipelineRunName, got, expectedServiceAccountName)
	}

	taskRuns, err := c.V1TaskRunClient.List(ctx, metav1.ListOptions{
		LabelSelector: pipeline.PipelineRunLabelKey + "=" + childPipelineRunName,
	})
	if err != nil {
		t.Fatalf("Couldn't list TaskRuns for child PipelineRun %q: %s", childPipelineRunName, err)
	}
	if len(taskRuns.Items) == 0 {
		t.Fatalf("Expected at least one TaskRun for child PipelineRun %q, got none", childPipelineRunName)
	}
	for _, tr := range taskRuns.Items {
		if got := tr.Spec.ServiceAccountName; got != expectedServiceAccountName {
			t.Errorf("Child TaskRun %q ServiceAccountName = %q, want %q", tr.Name, got, expectedServiceAccountName)
		}

		// The pod is where execution actually happens — assert it runs under the SA too.
		pods, err := c.KubeClient.CoreV1().Pods(tr.Namespace).List(ctx, metav1.ListOptions{
			LabelSelector: pipeline.TaskRunLabelKey + "=" + tr.Name,
		})
		if err != nil {
			t.Fatalf("Couldn't list Pods for child TaskRun %q: %s", tr.Name, err)
		}
		if len(pods.Items) != 1 {
			t.Fatalf("Expected exactly 1 pod for child TaskRun %q, got %d", tr.Name, len(pods.Items))
		}
		if got := pods.Items[0].Spec.ServiceAccountName; got != expectedServiceAccountName {
			t.Errorf("Child TaskRun %q pod %q ServiceAccountName = %q, want %q", tr.Name, pods.Items[0].Name, got, expectedServiceAccountName)
		}
	}
}

// @test:execution=parallel
func TestPipelineRun_PinP_PipelineRefWithParamsAndWorkspaces(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	c, namespace := setup(ctx, t, requireAnyGate(map[string]string{
		"enable-api-fields": "alpha",
	}))
	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	// GIVEN
	parentPipeline, childPipeline, parentPipelineRun, expectedChildPipelineRun :=
		th.OnePipelineRefInPipelineWithParamsAndWorkspaces(t, namespace, "parent-pipeline-run")
	parentPipelineRun = th.WithAnnotationAndLabel(parentPipelineRun, false)
	expectedKinds := createKindsMap(parentPipelineRun, []*v1.PipelineRun{expectedChildPipelineRun}, []*v1.Pipeline{parentPipeline, childPipeline})
	expectedEventsAmount := 3

	// WHEN
	createResourcesAndWaitForPipelineRunSuccess(ctx, t, c, namespace, []*v1.Pipeline{parentPipeline, childPipeline}, parentPipelineRun, nil)

	// THEN
	assertPinP(ctx, t, c, expectedChildPipelineRun)
	assertEvents(ctx, t, expectedEventsAmount, expectedKinds, c, namespace)
}

// @test:execution=parallel
func TestPipelineRun_PinP_SelfReferencingPipelineRefCycleDetection(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	c, namespace := setup(ctx, t, requireAnyGate(map[string]string{
		"enable-api-fields": "alpha",
	}))
	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	// GIVEN: Pipeline references itself via PipelineRef — a self-referencing cycle.
	selfRefPipeline, pipelineRun := th.SelfReferencingPipelineRefCycle(t, namespace, "pr-cycle-self")

	// WHEN
	createResourcesAndWaitForPipelineRunFailed(ctx, t, c, namespace, []*v1.Pipeline{selfRefPipeline}, pipelineRun, nil)

	// THEN: PipelineRun fails due to cycle detection.
	assertFailureMessage(ctx, t, c, pipelineRun.Name, "detected cycle in pipeline-in-pipeline")
}

// @test:execution=parallel
func TestPipelineRun_PinP_TwoLevelPipelineRefCycleDetection(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	c, namespace := setup(ctx, t, requireAnyGate(map[string]string{
		"enable-api-fields": "alpha",
	}))
	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	// GIVEN: pipeline-a -> pipeline-b -> pipeline-a (two-level cycle).
	pipelineA, pipelineB, pipelineRun := th.TwoLevelPipelineRefCycle(t, namespace, "pr-cycle-two-level")
	childPipelineRunName := pipelineRun.Name + "-pipeline-b"

	// WHEN: wait for the top-level PipelineRun to fail; the cycle is detected
	// before the grandchild is created, so checking the child message is enough.
	createResourcesAndWaitForPipelineRunFailed(ctx, t, c, namespace, []*v1.Pipeline{pipelineA, pipelineB}, pipelineRun, nil)

	// THEN
	assertFailureMessage(ctx, t, c, childPipelineRunName, "detected cycle in pipeline-in-pipeline")
}

// @test:execution=parallel
func TestPipelineRun_PinP_PipelineRefNotFound(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	c, namespace := setup(ctx, t, requireAnyGate(map[string]string{
		"enable-api-fields": "alpha",
	}))
	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	// GIVEN: a parent Pipeline whose pipelineRef points at a non-existent Pipeline.
	parentPipeline, pipelineRun := th.OnePipelineRefMissing(t, namespace, "pr-ref-not-found")

	// WHEN: Create only the parent Pipeline (the referenced child is intentionally missing).
	createResourcesAndWaitForPipelineRun(ctx, t, c, namespace,
		[]*v1.Pipeline{parentPipeline}, pipelineRun, nil,
		Chain(
			FailedWithReason(v1.PipelineRunReasonCouldntGetPipeline.String(), pipelineRun.Name),
			FailedWithMessage("nonexistent-pipeline", pipelineRun.Name),
		),
		"PipelineRunFailed")
}

// @test:execution=parallel
func TestPipelineRun_PinP_MissingWorkspaceBinding(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	c, namespace := setup(ctx, t, requireAnyGate(map[string]string{
		"enable-api-fields": "alpha",
	}))
	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	// GIVEN: parent that omits the child Pipeline's required workspace binding.
	parentPipeline, childPipeline, parentPipelineRun := th.OnePipelineRefMissingWorkspace(t, namespace, "pr-pinp-missing-ws")
	childPipelineRunName := parentPipelineRun.Name + "-child-pipeline-workspace"

	// WHEN: parent fails after the child workspace validation fails.
	createResourcesAndWaitForPipelineRunFailed(ctx, t, c, namespace, []*v1.Pipeline{parentPipeline, childPipeline}, parentPipelineRun, nil)

	// THEN
	assertFailureMessage(ctx, t, c, childPipelineRunName,
		`pipeline requires workspace with name "child-ws" be provided by pipelinerun`)
}

// @test:execution=parallel
// TestPipelineRun_PinP_MultiplePipelineRefChildren verifies that a parent Pipeline
// with more than one pipelineRef PipelineTask creates and completes one child
// PipelineRun per task. The existing suite only covers multiple children via
// pipelineSpec; this exercises the multi-child path for resolved pipelineRefs.
func TestPipelineRun_PinP_MultiplePipelineRefChildren(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	c, namespace := setup(ctx, t, requireAnyGate(map[string]string{
		"enable-api-fields": "alpha",
	}))
	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	// GIVEN: two child Pipelines and a parent that references both via pipelineRef.
	parentPipeline, childPipelines, parentPipelineRun, expectedChildPRs :=
		th.MultiplePipelineRefsInPipeline(t, namespace, "pr-multi-ref")
	expectedKinds := createKindsMap(parentPipelineRun, expectedChildPRs, append([]*v1.Pipeline{parentPipeline}, childPipelines...))
	expectedEventsAmount := 5

	// WHEN
	createResourcesAndWaitForPipelineRunSuccess(ctx, t, c, namespace, append([]*v1.Pipeline{parentPipeline}, childPipelines...), parentPipelineRun, nil)

	// THEN: both child PipelineRuns were created and succeeded.
	for _, cpr := range expectedChildPRs {
		assertPinP(ctx, t, c, cpr)
	}
	assertEvents(ctx, t, expectedEventsAmount, expectedKinds, c, namespace)
}

// @test:execution=parallel
// TestPipelineRun_PinP_MixedPipelineRefSpecAndTaskRef verifies that a single parent
// Pipeline can mix a pipelineRef child, an inline pipelineSpec child, and a regular
// taskRef task: two child PipelineRuns and one TaskRun are created and all complete.
func TestPipelineRun_PinP_MixedPipelineRefSpecAndTaskRef(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	c, namespace := setup(ctx, t, requireAnyGate(map[string]string{
		"enable-api-fields": "alpha",
	}))
	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	// GIVEN
	task, parentPipeline, childRef, parentPipelineRun, expectedRefChild, expectedSpecChild :=
		th.MixedPipelineRefSpecAndTaskRefInPipeline(t, namespace, "pr-mixed")
	expectedKinds := createKindsMap(parentPipelineRun,
		[]*v1.PipelineRun{expectedRefChild, expectedSpecChild},
		[]*v1.Pipeline{parentPipeline, childRef})
	expectedEventsAmount := 6

	// WHEN
	createResourcesAndWaitForPipelineRunSuccess(ctx, t, c, namespace, []*v1.Pipeline{parentPipeline, childRef}, parentPipelineRun, task)

	// THEN: the pipelineRef child and the pipelineSpec child both succeeded.
	assertPinP(ctx, t, c, expectedRefChild)
	assertPinP(ctx, t, c, expectedSpecChild)
	assertEvents(ctx, t, expectedEventsAmount, expectedKinds, c, namespace)
}

// @test:execution=parallel
// TestPipelineRun_PinP_ChildPipelineViaClusterResolver verifies the headline
// "child pipeline via resolver" capability end-to-end: the parent PipelineTask
// resolves its child Pipeline through the cluster resolver (rather than a local
// name ref), and the resulting child PipelineRun runs and succeeds.
func TestPipelineRun_PinP_ChildPipelineViaClusterResolver(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	c, namespace := setup(ctx, t, requireAnyGate(map[string]string{
		"enable-api-fields": "alpha",
	}))
	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	// GIVEN: a child Pipeline resolved through the cluster resolver instead of a local ref.
	parentPipeline, resolvedChild, parentPipelineRun, expectedChildPipelineRun :=
		th.ChildPipelineViaClusterResolver(t, namespace, "pr-resolver")
	// The child PR's Spec.PipelineRef is the resolver ref (Name=""), so
	// createKindsMap can't auto-derive its TaskRun via lookup — build the map by hand.
	childPRName := expectedChildPipelineRun.Name
	expectedKinds := map[string][]string{
		"PipelineRun": {parentPipelineRun.Name, childPRName},
		"TaskRun":     {kmeta.ChildName(childPRName, "-run")},
	}
	expectedEventsAmount := 3

	// WHEN
	createResourcesAndWaitForPipelineRunSuccess(ctx, t, c, namespace, []*v1.Pipeline{parentPipeline, resolvedChild}, parentPipelineRun, nil)

	// THEN: the resolver-backed child PipelineRun was created, succeeded, and is
	// labelled with the resolved Pipeline name.
	assertPinP(ctx, t, c, expectedChildPipelineRun)
	assertEvents(ctx, t, expectedEventsAmount, expectedKinds, c, namespace)
}

// @test:execution=parallel
// TestPipelineRun_PinP_PipelineRefWithPVCWorkspace verifies that a PVC-backed
// (volumeClaimTemplate) workspace is genuinely shared from the parent into a
// child Pipeline at runtime: a parent task writes a file to the workspace, and
// a task in the child Pipeline (which receives the same workspace) reads it
// back. The parent only succeeds if the child sees the parent-written data,
// proving the underlying volume is the same across the parent->child boundary.
// The existing workspace e2e coverage uses emptyDir, which cannot prove sharing.
func TestPipelineRun_PinP_PipelineRefWithPVCWorkspace(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	c, namespace := setup(ctx, t, requireAnyGate(map[string]string{
		"enable-api-fields": "alpha",
	}))
	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	// GIVEN: a child Pipeline that reads a file from its workspace, and a parent
	// that first writes that file to a PVC-backed workspace, then maps the same
	// workspace into the child via pipelineRef.
	parentPipeline, childPipeline, parentPipelineRun, expectedChildPR :=
		th.PipelineRefWithPVCWorkspace(t, namespace, "pr-pvc-ws")
	// extras: the parent's writer task is a taskSpec on the parent (not a child PR).
	expectedKinds := createKindsMap(parentPipelineRun,
		[]*v1.PipelineRun{expectedChildPR},
		[]*v1.Pipeline{parentPipeline, childPipeline})
	expectedEventsAmount := 4

	// WHEN: createResourcesAndWaitForPipelineRunSuccess waits for the parent to succeed,
	// which only happens if the child's read (grep) of the parent-written file passes.
	createResourcesAndWaitForPipelineRunSuccess(ctx, t, c, namespace, []*v1.Pipeline{parentPipeline, childPipeline}, parentPipelineRun, nil)

	// THEN: the child PipelineRun that received the shared PVC workspace succeeded.
	assertPinP(ctx, t, c, expectedChildPR)
	assertEvents(ctx, t, expectedEventsAmount, expectedKinds, c, namespace)
}

// @test:execution=parallel
// TestPipelineRun_PinP_PipelineRefWhenExpressionSkipsChild verifies that a
// pipelineRef PipelineTask guarded by a when expression that evaluates to false
// is skipped: no child PipelineRun is created and the parent succeeds.
func TestPipelineRun_PinP_PipelineRefWhenExpressionSkipsChild(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	c, namespace := setup(ctx, t, requireAnyGate(map[string]string{
		"enable-api-fields": "alpha",
	}))
	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	// GIVEN: a parent whose only task references a child Pipeline but is guarded
	// by a when expression that is false ("run" is not in ["no"]).
	parentPipeline, childPipeline, parentPipelineRun :=
		th.PipelineRefInPipelineWhenSkipped(t, namespace, "pr-when-skip")

	// WHEN: the parent succeeds (a skipped-only-task pipeline completes successfully).
	createResourcesAndWaitForPipelineRunSuccess(ctx, t, c, namespace, []*v1.Pipeline{parentPipeline, childPipeline}, parentPipelineRun, nil)

	// THEN: no child PipelineRun was created for the skipped task.
	childPRName := parentPipelineRun.Name + "-maybe-child"
	if _, err := c.V1PipelineRunClient.Get(ctx, childPRName, metav1.GetOptions{}); err == nil {
		t.Fatalf("Expected child PipelineRun %s to be skipped (not created), but it exists", childPRName)
	} else if !apierrors.IsNotFound(err) {
		t.Fatalf("Unexpected error checking for skipped child PipelineRun %s: %s", childPRName, err)
	}
}

// @test:execution=parallel
// TestPipelineRun_PinP_PipelineRefInFinally verifies that a pipelineRef
// PipelineTask placed in a parent Pipeline's finally section runs as a child
// PipelineRun after the main tasks complete, and succeeds.
func TestPipelineRun_PinP_PipelineRefInFinally(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	c, namespace := setup(ctx, t, requireAnyGate(map[string]string{
		"enable-api-fields": "alpha",
	}))
	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	// GIVEN: a parent with a main task and a finally task that references a child Pipeline.
	parentPipeline, finallyChild, parentPipelineRun, expectedChildPR :=
		th.PipelineRefInFinally(t, namespace, "pr-finally")
	// extras: the parent's main task is a taskSpec on the parent (not a child PR).
	expectedKinds := createKindsMap(parentPipelineRun,
		[]*v1.PipelineRun{expectedChildPR},
		[]*v1.Pipeline{parentPipeline, finallyChild})
	expectedEventsAmount := 4

	// WHEN
	createResourcesAndWaitForPipelineRunSuccess(ctx, t, c, namespace, []*v1.Pipeline{parentPipeline, finallyChild}, parentPipelineRun, nil)

	// THEN: the finally child PipelineRun was created and succeeded.
	assertPinP(ctx, t, c, expectedChildPR)
	assertEvents(ctx, t, expectedEventsAmount, expectedKinds, c, namespace)
}
