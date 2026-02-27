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
	"testing"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	th "github.com/tektoncd/pipeline/pkg/reconciler/testing"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
		if cpr.Spec.PipelineSpec.Tasks[0].PipelineSpec == nil {
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
	checkLabelPropagationToChildPipelineRun(ctx, t, c, directParentPrName, actualCpr)
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
