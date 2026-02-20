package testing

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/test/diff"
	corev1 "k8s.io/api/core/v1"
	"knative.dev/pkg/apis"
)

const (
	taskRun     = pipeline.TaskRunControllerName
	customRun   = pipeline.CustomRunControllerName
	pipelineRun = pipeline.PipelineRunControllerName
)

func CheckPipelineRunConditionStatusAndReason(
	t *testing.T,
	prStatus v1.PipelineRunStatus,
	expectedStatus corev1.ConditionStatus,
	expectedReason string,
) {
	t.Helper()

	actualCondition := prStatus.GetCondition(apis.ConditionSucceeded)
	if actualCondition == nil {
		t.Fatalf("want condition, got nil")
	}
	if actualCondition.Status != expectedStatus {
		t.Errorf("want status %v, got %v", expectedStatus, actualCondition.Status)
	}
	if actualCondition.Reason != expectedReason {
		t.Errorf("want reason %s, got %s", expectedReason, actualCondition.Reason)
	}
}

func VerifyTaskRunStatusesCount(t *testing.T, prStatus v1.PipelineRunStatus, expectedCount int) {
	t.Helper()
	verifyCount(t, prStatus, expectedCount, taskRun)
}

func verifyCount(t *testing.T, prStatus v1.PipelineRunStatus, expectedCount int, kind string) {
	t.Helper()

	actualCount := len(filterChildRefsForKind(prStatus.ChildReferences, kind))
	if actualCount != expectedCount {
		oneOrMany := kind
		if expectedCount > 1 {
			oneOrMany += "s"
		}
		t.Errorf("Expected PipelineRun status ChildReferences to have %d %s, but was %d", expectedCount, oneOrMany, actualCount)
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

func VerifyTaskRunStatusesNames(t *testing.T, prStatus v1.PipelineRunStatus, expectedNames ...string) {
	t.Helper()
	verifyNames(t, prStatus, expectedNames, taskRun)
}

func verifyNames(t *testing.T, prStatus v1.PipelineRunStatus, expectedNames []string, kind string) {
	t.Helper()

	actualNames := make(map[string]bool)
	for _, cr := range filterChildRefsForKind(prStatus.ChildReferences, kind) {
		actualNames[cr.Name] = true
	}

	for _, expectedName := range expectedNames {
		if actualNames[expectedName] {
			continue
		}

		t.Errorf("Expected PipelineRun status to include %s status for %s but was %v", kind, expectedName, prStatus.ChildReferences)
	}
}

func VerifyTaskRunStatusesWhenExpressions(t *testing.T, prStatus v1.PipelineRunStatus, trName string, expectedWhen []v1.WhenExpression) {
	t.Helper()

	var actualWhen []v1.WhenExpression
	for _, cr := range prStatus.ChildReferences {
		if cr.Name == trName {
			actualWhen = append(actualWhen, cr.WhenExpressions...)
		}
	}

	if d := cmp.Diff(expectedWhen, actualWhen); d != "" {
		t.Errorf("Expected to see When Expressions %v created. Diff %s", trName, diff.PrintWantGot(d))
	}
}

func VerifyCustomRunOrRunStatusesCount(t *testing.T, prStatus v1.PipelineRunStatus, expectedCount int) {
	t.Helper()
	verifyCount(t, prStatus, expectedCount, customRun)
}

func VerifyCustomRunOrRunStatusesNames(t *testing.T, prStatus v1.PipelineRunStatus, expectedNames ...string) {
	t.Helper()
	verifyNames(t, prStatus, expectedNames, customRun)
}

func VerifyChildPipelineRunStatusesCount(t *testing.T, prStatus v1.PipelineRunStatus, expectedCount int) {
	t.Helper()
	verifyCount(t, prStatus, expectedCount, pipelineRun)
}

func VerifyChildPipelineRunStatusesNames(t *testing.T, prStatus v1.PipelineRunStatus, expectedNames ...string) {
	t.Helper()
	verifyNames(t, prStatus, expectedNames, pipelineRun)
}
