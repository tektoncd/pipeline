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
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	th "github.com/tektoncd/pipeline/pkg/reconciler/testing"
	"github.com/tektoncd/pipeline/test/parse"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
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
	expectedKinds := createKindsMap(parentPipelineRun, []*v1.PipelineRun{expectedChildPipelineRun})
	expectedEventsAmount := 3

	// WHEN
	createResourcesAndWaitForPipelineRun(ctx, t, c, namespace, parentPipeline, parentPipelineRun, nil)

	// THEN
	assertPinP(ctx, t, c, namespace, expectedChildPipelineRun)
	assertEvents(ctx, t, expectedEventsAmount, expectedKinds, c, namespace)
}

func createKindsMap(parentPipelineRun *v1.PipelineRun, childPipelineRuns []*v1.PipelineRun) map[string][]string {
	prNames := []string{parentPipelineRun.Name}
	for _, cpr := range childPipelineRuns {
		prNames = append(prNames, cpr.Name)
	}

	var trNames []string
	for _, cpr := range childPipelineRuns {
		// collect names of TaskRuns; ignore nested PipelineRuns
		if cpr.Spec.PipelineSpec != nil && cpr.Spec.PipelineSpec.Tasks[0].PipelineSpec == nil {
			trNames = append(trNames, cpr.Name+"-"+cpr.Spec.PipelineSpec.Tasks[0].Name)
		}
	}

	return map[string][]string{
		"PipelineRun": prNames,
		"TaskRun":     trNames,
	}
}

func assertPinP(
	ctx context.Context,
	t *testing.T,
	c *clients,
	namespace string,
	childPipelineRun *v1.PipelineRun,
) {
	t.Helper()

	t.Logf("Making sure the expected child PipelineRun %q was created", childPipelineRun.Name)
	actualCpr, err := c.V1PipelineRunClient.Get(ctx, childPipelineRun.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Error listing child PipelineRuns for PipelineRun %s: %s", childPipelineRun.Name, err)
	}
	th.CheckPipelineRunConditionStatusAndReason(
		t,
		actualCpr.Status,
		corev1.ConditionTrue,
		v1.PipelineRunReasonSuccessful.String(),
	)
	t.Logf("Checking that labels were propagated correctly for child PipelineRun %q", actualCpr.Name)
	directParentPrName := actualCpr.OwnerReferences[0].Name
	checkLabelPropagationToChildPipelineRun(ctx, t, c, namespace, directParentPrName, actualCpr)
	t.Logf("Checking that annotations were propagated correctly for child PipelineRun %q", actualCpr.Name)
	checkAnnotationPropagationToChildPipelineRun(ctx, t, c, namespace, directParentPrName, actualCpr)
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
	expectedKinds := createKindsMap(parentPipelineRun, expectedChildPipelineRuns)
	expectedEventsAmount := 5

	// WHEN
	createResourcesAndWaitForPipelineRun(ctx, t, c, namespace, parentPipeline, parentPipelineRun, task)

	// THEN
	assertPinP(ctx, t, c, namespace, expectedChildPipelineRuns[0])
	assertPinP(ctx, t, c, namespace, expectedChildPipelineRuns[1])
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
		})
	expectedEventsAmount := 4

	// WHEN
	createResourcesAndWaitForPipelineRun(ctx, t, c, namespace, parentPipeline, parentPipelineRun, nil)

	// THEN
	assertPinP(ctx, t, c, namespace, expectedChildPipelineRun)
	assertPinP(ctx, t, c, namespace, expectedGrandchildPipelineRun)
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

	// WHEN
	createPipelinesAndWaitForPipelineRun(ctx, t, c, namespace, []*v1.Pipeline{parentPipeline, childPipeline}, parentPipelineRun)

	// THEN
	assertPinPRefAndSpecMatch(ctx, t, c, namespace, expectedChildPipelineRun)
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

	// WHEN
	createPipelinesAndWaitForPipelineRun(ctx, t, c, namespace, []*v1.Pipeline{parentPipeline, childPipeline, grandchildPipeline}, parentPipelineRun)

	// THEN
	assertPinPRefAndSpecMatch(ctx, t, c, namespace, expectedChildPipelineRun)
	assertPinPRefAndSpecMatch(ctx, t, c, namespace, expectedGrandchildPipelineRun)
}

// @test:execution=parallel
func TestPipelineRun_PinP_ThreeLevelDeepNestedChildPipelineRunsWithPipelineRef(t *testing.T) {
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
	t.Logf("Setting up test resources for three level deep nested child PipelineRuns with PipelineRef in namespace %q", namespace)
	parentPipeline, childPipeline, grandchildPipeline, greatGrandchildPipeline,
		parentPipelineRun,
		expectedChildPipelineRun,
		expectedGrandchildPipelineRun,
		expectedGreatGrandchildPipelineRun := th.ThreeLevelNestedPipelineRefsInPipeline(t, namespace, "parent-pipeline-deep-nested-ref")
	parentPipelineRun = th.WithAnnotationAndLabel(parentPipelineRun, false)

	// WHEN
	createPipelinesAndWaitForPipelineRun(
		ctx,
		t,
		c,
		namespace,
		[]*v1.Pipeline{parentPipeline, childPipeline, grandchildPipeline, greatGrandchildPipeline},
		parentPipelineRun,
	)

	// THEN
	assertPinPRefAndSpecMatch(ctx, t, c, namespace, expectedChildPipelineRun)
	assertPinPRefAndSpecMatch(ctx, t, c, namespace, expectedGrandchildPipelineRun)
	assertPinPRefAndSpecMatch(ctx, t, c, namespace, expectedGreatGrandchildPipelineRun)
}

// assertPinPRef is like assertPinP but for child PipelineRuns created from PipelineRef.
// It verifies the child completed successfully and that labels/annotations were propagated.
func assertPinPRef(
	ctx context.Context,
	t *testing.T,
	c *clients,
	namespace string,
	childPipelineRun *v1.PipelineRun,
) *v1.PipelineRun {
	t.Helper()

	t.Logf("Making sure the expected child PipelineRun %q was created", childPipelineRun.Name)
	actualCpr, err := c.V1PipelineRunClient.Get(ctx, childPipelineRun.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Error getting child PipelineRun %s: %s", childPipelineRun.Name, err)
	}
	th.CheckPipelineRunConditionStatusAndReason(
		t,
		actualCpr.Status,
		corev1.ConditionTrue,
		v1.PipelineRunReasonSuccessful.String(),
	)
	t.Logf("Checking that labels were propagated correctly for child PipelineRun %q", actualCpr.Name)
	directParentPrName := actualCpr.OwnerReferences[0].Name
	checkLabelPropagationToChildPipelineRunRef(ctx, t, c, namespace, directParentPrName, actualCpr)
	t.Logf("Checking that annotations were propagated correctly for child PipelineRun %q", actualCpr.Name)
	checkAnnotationPropagationToChildPipelineRun(ctx, t, c, namespace, directParentPrName, actualCpr)

	return actualCpr
}

func assertPipelineRunSpecMatchesExpected(t *testing.T, expected, actual *v1.PipelineRun) {
	t.Helper()

	ignoreTimeouts := cmpopts.IgnoreFields(v1.PipelineRunSpec{}, "Timeouts")

	if diff := cmp.Diff(expected.Spec, actual.Spec, ignoreSAPipelineRunSpec, ignoreTimeouts, cmpopts.EquateEmpty()); diff != "" {
		t.Fatalf("Unexpected child PipelineRun spec (-want, +got): %s", diff)
	}
}

func assertPinPRefAndSpecMatch(
	ctx context.Context,
	t *testing.T,
	c *clients,
	namespace string,
	expected *v1.PipelineRun,
) {
	t.Helper()

	actual := assertPinPRef(ctx, t, c, namespace, expected)
	assertPipelineRunSpecMatchesExpected(t, expected, actual)
}

func waitForPipelineRunToExist(ctx context.Context, t *testing.T, c *clients, name string, polltimeout time.Duration) {
	t.Helper()

	ctx, cancel := context.WithTimeout(ctx, polltimeout)
	defer cancel()

	if err := pollImmediateWithContext(ctx, func() (bool, error) {
		_, err := c.V1PipelineRunClient.Get(ctx, name, metav1.GetOptions{})
		if err == nil {
			return true, nil
		}
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return true, err
	}); err != nil {
		t.Fatalf("Error waiting for child PipelineRun %s to be created: %s", name, err)
	}
}

func pipelineRunHasFailureMessage(pr *v1.PipelineRun, message string) bool {
	condition := pr.Status.GetCondition(apis.ConditionSucceeded)
	return condition != nil &&
		condition.Status == corev1.ConditionFalse &&
		strings.Contains(condition.Message, message)
}

// createPipelinesAndWaitForPipelineRun creates multiple pipelines and a PipelineRun, then waits for completion.
func createPipelinesAndWaitForPipelineRun(
	ctx context.Context,
	t *testing.T,
	c *clients,
	namespace string,
	pipelines []*v1.Pipeline,
	pipelineRun *v1.PipelineRun,
) {
	t.Helper()

	for _, p := range pipelines {
		if _, err := c.V1PipelineClient.Create(ctx, p, metav1.CreateOptions{}); err != nil {
			t.Fatalf("Failed to create Pipeline `%s`: %s", p.Name, err)
		}
	}

	if _, err := c.V1PipelineRunClient.Create(ctx, pipelineRun, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create PipelineRun `%s`: %s", pipelineRun.Name, err)
	}

	t.Logf("Waiting for PipelineRun %q in namespace %q to complete", pipelineRun.Name, namespace)
	if err := WaitForPipelineRunState(
		ctx,
		c,
		pipelineRun.Name,
		timeout,
		PipelineRunSucceed(pipelineRun.Name),
		"PipelineRunSuccess",
		v1Version,
	); err != nil {
		t.Fatalf("Error waiting for PipelineRun %s to finish: %s", pipelineRun.Name, err)
	}
}

// checkLabelPropagationToChildPipelineRunRef checks label propagation for PipelineRef children.
// Unlike PipelineSpec children (where tekton.dev/pipeline = childPr.Name), PipelineRef children
// have tekton.dev/pipeline set to the referenced pipeline name.
func checkLabelPropagationToChildPipelineRunRef(
	ctx context.Context,
	t *testing.T,
	c *clients,
	namespace string,
	parentPrName string,
	childPr *v1.PipelineRun,
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

	// For PipelineRef children, tekton.dev/pipeline is set to the referenced pipeline name
	// (not the child PR name like for PipelineSpec children). The reconciler sets this via
	// the PipelineRef resolution. Assert the exact expected value.
	expectedPipelineName := childPr.Spec.PipelineRef.Name
	if childPr.Labels[pipeline.PipelineLabelKey] != expectedPipelineName {
		t.Errorf("Expected tekton.dev/pipeline=%q, got %q",
			expectedPipelineName, childPr.Labels[pipeline.PipelineLabelKey])
	}
	labels[pipeline.PipelineLabelKey] = expectedPipelineName

	assertLabelsMatch(t, labels, childPr.ObjectMeta.Labels)
	t.Logf("Labels propagated from parent PipelineRun to child PipelineRun (PipelineRef): %#v", labels)
}

func createResourcesAndWaitForPipelineRun(
	ctx context.Context,
	t *testing.T,
	c *clients,
	namespace string,
	pipeline *v1.Pipeline,
	pipelineRun *v1.PipelineRun,
	task *v1.Task,
) {
	t.Helper()

	if _, err := c.V1PipelineClient.Create(ctx, pipeline, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create Pipeline `%s`: %s", pipeline.Name, err)
	}

	if task != nil {
		if _, err := c.V1TaskClient.Create(ctx, task, metav1.CreateOptions{}); err != nil {
			t.Fatalf("Failed to create Task `%s`: %s", task.Name, err)
		}
	}

	if _, err := c.V1PipelineRunClient.Create(ctx, pipelineRun, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create PipelineRun `%s`: %s", pipelineRun.Name, err)
	}

	t.Logf("Waiting for PipelineRun %q in namespace %q to complete", pipelineRun.Name, namespace)
	if err := WaitForPipelineRunState(
		ctx,
		c,
		pipelineRun.Name,
		timeout,
		PipelineRunSucceed(pipelineRun.Name),
		"PipelineRunSuccess",
		v1Version,
	); err != nil {
		t.Fatalf("Error waiting for PipelineRun %s to finish: %s", pipelineRun.Name, err)
	}
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
	namespace string,
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
	namespace string,
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

// @test:execution=parallel
func TestPipelineRun_PinP_PipelineRefWithParams(t *testing.T) {
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
		th.OnePipelineRefInPipelineWithParams(t, namespace, "parent-pipeline-run")
	parentPipelineRun = th.WithAnnotationAndLabel(parentPipelineRun, false)

	// WHEN
	createPipelinesAndWaitForPipelineRun(ctx, t, c, namespace,
		[]*v1.Pipeline{parentPipeline, childPipeline}, parentPipelineRun)

	// THEN
	assertPinPRefAndSpecMatch(ctx, t, c, namespace, expectedChildPipelineRun)
}

// @test:execution=parallel
func TestPipelineRun_PinP_PipelineRefWithWorkspaces(t *testing.T) {
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
		th.OnePipelineRefInPipelineWithWorkspaces(t, namespace, "parent-pipeline-run")
	parentPipelineRun = th.WithAnnotationAndLabel(parentPipelineRun, false)

	// WHEN
	createPipelinesAndWaitForPipelineRun(ctx, t, c, namespace,
		[]*v1.Pipeline{parentPipeline, childPipeline}, parentPipelineRun)

	// THEN
	assertPinPRefAndSpecMatch(ctx, t, c, namespace, expectedChildPipelineRun)
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

	// WHEN
	createPipelinesAndWaitForPipelineRun(ctx, t, c, namespace,
		[]*v1.Pipeline{parentPipeline, childPipeline}, parentPipelineRun)

	// THEN
	assertPinPRefAndSpecMatch(ctx, t, c, namespace, expectedChildPipelineRun)
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

	// GIVEN: Pipeline A references itself via PipelineRef — a self-referencing cycle
	pipelineName := "self-ref-pipeline"
	selfRefPipeline := &v1.Pipeline{
		ObjectMeta: metav1.ObjectMeta{Name: pipelineName, Namespace: namespace},
		Spec: v1.PipelineSpec{
			Tasks: []v1.PipelineTask{{
				Name:        "ref-self",
				PipelineRef: &v1.PipelineRef{Name: pipelineName},
			}},
		},
	}

	pipelineRun := &v1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{Name: "pr-cycle-self", Namespace: namespace},
		Spec: v1.PipelineRunSpec{
			PipelineRef: &v1.PipelineRef{Name: pipelineName},
		},
	}

	// WHEN
	if _, err := c.V1PipelineClient.Create(ctx, selfRefPipeline, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create Pipeline %s: %s", pipelineName, err)
	}
	if _, err := c.V1PipelineRunClient.Create(ctx, pipelineRun, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create PipelineRun %s: %s", pipelineRun.Name, err)
	}

	// THEN: PipelineRun should fail due to cycle detection
	t.Logf("Waiting for PipelineRun %q to fail with cycle detection", pipelineRun.Name)
	if err := WaitForPipelineRunState(ctx, c, pipelineRun.Name, timeout,
		Chain(
			PipelineRunFailed(pipelineRun.Name),
			FailedWithMessage("detected cycle in pipeline-in-pipeline", pipelineRun.Name),
		), "PipelineRunFailed", v1Version); err != nil {
		t.Fatalf("Error waiting for PipelineRun %s to fail: %s", pipelineRun.Name, err)
	}
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

	// GIVEN
	pipelineA := parse.MustParseV1Pipeline(t, fmt.Sprintf(`
metadata:
  name: pipeline-a
  namespace: %s
spec:
  tasks:
  - name: pipeline-b
    pipelineRef:
      name: pipeline-b
`, namespace))
	pipelineB := parse.MustParseV1Pipeline(t, fmt.Sprintf(`
metadata:
  name: pipeline-b
  namespace: %s
spec:
  tasks:
  - name: pipeline-a
    pipelineRef:
      name: pipeline-a
`, namespace))
	pipelineRun := parse.MustParseV1PipelineRun(t, fmt.Sprintf(`
metadata:
  name: pr-cycle-two-level
  namespace: %s
spec:
  pipelineRef:
    name: pipeline-a
`, namespace))
	childPipelineRunName := pipelineRun.Name + "-pipeline-b"
	grandchildPipelineRunName := childPipelineRunName + "-pipeline-a"

	// WHEN
	for _, pipeline := range []*v1.Pipeline{pipelineA, pipelineB} {
		if _, err := c.V1PipelineClient.Create(ctx, pipeline, metav1.CreateOptions{}); err != nil {
			t.Fatalf("Failed to create Pipeline %s: %s", pipeline.Name, err)
		}
	}
	if _, err := c.V1PipelineRunClient.Create(ctx, pipelineRun, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create PipelineRun %s: %s", pipelineRun.Name, err)
	}

	// THEN
	t.Logf("Waiting for child PipelineRun %q to be created before asserting failure", childPipelineRunName)
	waitForPipelineRunToExist(ctx, t, c, childPipelineRunName, timeout)

	t.Logf("Waiting for child PipelineRun %q to fail after grandchild cycle detection", childPipelineRunName)
	if err := WaitForPipelineRunState(ctx, c, childPipelineRunName, timeout,
		PipelineRunFailed(childPipelineRunName), "PipelineRunFailed", v1Version); err != nil {
		t.Fatalf("Error waiting for child PipelineRun %s to fail: %s", childPipelineRunName, err)
	}

	childPipelineRun, err := c.V1PipelineRunClient.Get(ctx, childPipelineRunName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Error getting child PipelineRun %s: %s", childPipelineRunName, err)
	}
	if !pipelineRunHasFailureMessage(childPipelineRun, "detected cycle in pipeline-in-pipeline") {
		t.Logf("Cycle message was not present on child PipelineRun %q, checking grandchild PipelineRun %q", childPipelineRunName, grandchildPipelineRunName)
		waitForPipelineRunToExist(ctx, t, c, grandchildPipelineRunName, timeout)
		if err := WaitForPipelineRunState(ctx, c, grandchildPipelineRunName, timeout,
			Chain(
				PipelineRunFailed(grandchildPipelineRunName),
				FailedWithMessage("detected cycle in pipeline-in-pipeline", grandchildPipelineRunName),
			), "PipelineRunFailed", v1Version); err != nil {
			t.Fatalf("Error waiting for grandchild PipelineRun %s to fail: %s", grandchildPipelineRunName, err)
		}
	}

	t.Logf("Waiting for top-level PipelineRun %q to fail after descendant cycle detection", pipelineRun.Name)
	if err := WaitForPipelineRunState(ctx, c, pipelineRun.Name, timeout,
		PipelineRunFailed(pipelineRun.Name), "PipelineRunFailed", v1Version); err != nil {
		t.Fatalf("Error waiting for PipelineRun %s to fail: %s", pipelineRun.Name, err)
	}
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

	// GIVEN: Pipeline A references non-existent Pipeline B
	parentPipelineName := "parent-ref-not-found"
	parentPipeline := &v1.Pipeline{
		ObjectMeta: metav1.ObjectMeta{Name: parentPipelineName, Namespace: namespace},
		Spec: v1.PipelineSpec{
			Tasks: []v1.PipelineTask{{
				Name:        "ref-nonexistent",
				PipelineRef: &v1.PipelineRef{Name: "nonexistent-pipeline"},
			}},
		},
	}

	pipelineRun := &v1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{Name: "pr-ref-not-found", Namespace: namespace},
		Spec: v1.PipelineRunSpec{
			PipelineRef: &v1.PipelineRef{Name: parentPipelineName},
		},
	}

	// WHEN: Create only Pipeline A (not B)
	if _, err := c.V1PipelineClient.Create(ctx, parentPipeline, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create Pipeline %s: %s", parentPipelineName, err)
	}
	if _, err := c.V1PipelineRunClient.Create(ctx, pipelineRun, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create PipelineRun %s: %s", pipelineRun.Name, err)
	}

	// THEN: PipelineRun should fail because referenced pipeline doesn't exist
	t.Logf("Waiting for PipelineRun %q to fail with CouldntGetPipeline", pipelineRun.Name)
	if err := WaitForPipelineRunState(ctx, c, pipelineRun.Name, timeout,
		Chain(
			FailedWithReason(v1.PipelineRunReasonCouldntGetPipeline.String(), pipelineRun.Name),
			FailedWithMessage("nonexistent-pipeline", pipelineRun.Name),
		),
		"PipelineRunFailed", v1Version); err != nil {
		t.Fatalf("Error waiting for PipelineRun %s to fail: %s", pipelineRun.Name, err)
	}
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

	// GIVEN
	childPipeline := parse.MustParseV1Pipeline(t, fmt.Sprintf(`
metadata:
  name: child-pipeline-workspace
  namespace: %s
spec:
  workspaces:
  - name: child-ws
  tasks:
  - name: use-workspace
    taskSpec:
      steps:
      - name: ls
        image: mirror.gcr.io/busybox
        script: |
          ls $(workspaces.shared.path)
      workspaces:
      - name: shared
    workspaces:
    - name: shared
      workspace: child-ws
`, namespace))
	parentPipeline := parse.MustParseV1Pipeline(t, fmt.Sprintf(`
metadata:
  name: parent-pipeline-missing-ws
  namespace: %s
spec:
  tasks:
  - name: child-pipeline-workspace
    pipelineRef:
      name: child-pipeline-workspace
`, namespace))
	parentPipelineRun := parse.MustParseV1PipelineRun(t, fmt.Sprintf(`
metadata:
  name: pr-pinp-missing-ws
  namespace: %s
spec:
  pipelineRef:
    name: parent-pipeline-missing-ws
`, namespace))
	childPipelineRunName := parentPipelineRun.Name + "-child-pipeline-workspace"

	// WHEN
	for _, pipeline := range []*v1.Pipeline{parentPipeline, childPipeline} {
		if _, err := c.V1PipelineClient.Create(ctx, pipeline, metav1.CreateOptions{}); err != nil {
			t.Fatalf("Failed to create Pipeline %s: %s", pipeline.Name, err)
		}
	}
	if _, err := c.V1PipelineRunClient.Create(ctx, parentPipelineRun, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create PipelineRun %s: %s", parentPipelineRun.Name, err)
	}

	// THEN
	t.Logf("Waiting for child PipelineRun %q to be created before asserting failure", childPipelineRunName)
	waitForPipelineRunToExist(ctx, t, c, childPipelineRunName, timeout)

	t.Logf("Waiting for child PipelineRun %q to fail because the workspace binding is missing", childPipelineRunName)
	if err := WaitForPipelineRunState(ctx, c, childPipelineRunName, timeout,
		Chain(
			PipelineRunFailed(childPipelineRunName),
			FailedWithMessage(`pipeline requires workspace with name "child-ws" be provided by pipelinerun`, childPipelineRunName),
		), "PipelineRunFailed", v1Version); err != nil {
		t.Fatalf("Error waiting for child PipelineRun %s to fail: %s", childPipelineRunName, err)
	}

	t.Logf("Waiting for top-level PipelineRun %q to fail after child workspace validation fails", parentPipelineRun.Name)
	if err := WaitForPipelineRunState(ctx, c, parentPipelineRun.Name, timeout,
		PipelineRunFailed(parentPipelineRun.Name), "PipelineRunFailed", v1Version); err != nil {
		t.Fatalf("Error waiting for PipelineRun %s to fail: %s", parentPipelineRun.Name, err)
	}
}
