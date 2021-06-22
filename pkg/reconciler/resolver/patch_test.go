package resolver

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/test"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ktesting "k8s.io/client-go/testing"
)

func TestPatchResolvedTaskRunSuccess(t *testing.T) {
	tr := &v1beta1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tr",
			Namespace: "default",
		},
		Spec: v1beta1.TaskRunSpec{
			TaskRef: &v1beta1.TaskRef{
				Name: simpleTask.Name,
			},
		},
		Status: v1beta1.TaskRunStatus{
			TaskRunStatusFields: v1beta1.TaskRunStatusFields{
				TaskSpec: simpleTask.Spec.DeepCopy(),
			},
		},
	}

	assets, cancel := getClients(t, test.Data{
		Tasks:    []*v1beta1.Task{simpleTask.DeepCopy()},
		TaskRuns: []*v1beta1.TaskRun{tr},
	})

	defer cancel()

	tr, err := PatchResolvedTaskRun(assets.Ctx, assets.Clients.Kube, assets.Clients.Pipeline, tr)

	if tr == nil || err != nil {
		t.Fatalf("unexpected return result")
	}

	patchedMetadata := false
	patchedStatus := false
	for _, a := range getPatchActions(assets.Clients.Pipeline.Actions(), "taskruns") {
		if a.GetSubresource() == "status" {
			got := map[string]struct {
				TaskSpec *v1beta1.TaskSpec `json:"taskSpec"`
			}{}
			if err := json.Unmarshal(a.GetPatch(), &got); err != nil {
				t.Fatalf("invalid status patch json: %v", err)
			}
			if _, hasStatusPatch := got["status"]; hasStatusPatch {
				patchedStatus = true
			}
		} else {
			got := map[string]metav1.ObjectMeta{}
			if err := json.Unmarshal(a.GetPatch(), &got); err != nil {
				t.Fatalf("invalid metadata patch json: %v", err)
			}
			if _, hasMetadataPatch := got["metadata"]; hasMetadataPatch {
				patchedMetadata = true
			}
		}
	}

	if !patchedMetadata {
		t.Errorf("expected metadata patch")
	}
	if !patchedStatus {
		t.Errorf("expected status patch")
	}
}

func TestPatchResolvedPipelineRunSuccess(t *testing.T) {
	pr := &v1beta1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tr",
			Namespace: simplePipeline.Namespace,
		},
		Spec: v1beta1.PipelineRunSpec{
			PipelineRef: &v1beta1.PipelineRef{
				Name: simplePipeline.Name,
			},
		},
		Status: v1beta1.PipelineRunStatus{
			PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{
				PipelineSpec: simplePipeline.Spec.DeepCopy(),
			},
		},
	}

	assets, cancel := getClients(t, test.Data{
		Pipelines:    []*v1beta1.Pipeline{simplePipeline.DeepCopy()},
		PipelineRuns: []*v1beta1.PipelineRun{pr},
	})

	defer cancel()

	pr, err := PatchResolvedPipelineRun(assets.Ctx, assets.Clients.Kube, assets.Clients.Pipeline, pr)

	if pr == nil || err != nil {
		t.Fatalf("unexpected return result")
	}

	patchedMetadata := false
	patchedStatus := false

	for _, a := range getPatchActions(assets.Clients.Pipeline.Actions(), "pipelineruns") {
		if a.GetSubresource() == "status" {
			got := map[string]struct {
				PipelineSpec *v1beta1.PipelineSpec `json:"pipelineSpec"`
			}{}
			if err := json.Unmarshal(a.GetPatch(), &got); err != nil {
				t.Fatalf("invalid status patch json: %v", err)
			}
			if _, hasStatusPatch := got["status"]; hasStatusPatch {
				patchedStatus = true
			}
		} else {
			got := map[string]metav1.ObjectMeta{}
			if err := json.Unmarshal(a.GetPatch(), &got); err != nil {
				t.Fatalf("invalid metadata patch json: %v", err)
			}
			if _, hasMetadataPatch := got["metadata"]; hasMetadataPatch {
				patchedMetadata = true
			}
		}
	}

	if !patchedMetadata {
		t.Errorf("expected metadata patch")
	}
	if !patchedStatus {
		t.Errorf("expected status patch")
	}
}

func TestPatchResolvedTaskRunMetadataError(t *testing.T) {
	tr := &v1beta1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tr",
			Namespace: "default",
		},
		Spec: v1beta1.TaskRunSpec{
			TaskRef: &v1beta1.TaskRef{
				Name: simpleTask.Name,
			},
		},
		Status: v1beta1.TaskRunStatus{
			TaskRunStatusFields: v1beta1.TaskRunStatusFields{
				TaskSpec: simpleTask.Spec.DeepCopy(),
			},
		},
	}

	assets, cancel := getClients(t, test.Data{
		Tasks:    []*v1beta1.Task{simpleTask.DeepCopy()},
		TaskRuns: []*v1beta1.TaskRun{tr},
	})

	defer cancel()

	testError := errors.New("this error should be returned when patching")

	// Return error when resolver tries to patch TaskRun metadata.
	assets.Clients.Pipeline.PrependReactor("patch", "taskruns", func(action ktesting.Action) (bool, runtime.Object, error) {
		if action.GetSubresource() != "" {
			t.Fatalf("unexpected patch of subresource %q", action.GetSubresource())
		}
		return true, nil, testError
	})

	tr, err := PatchResolvedTaskRun(assets.Ctx, assets.Clients.Kube, assets.Clients.Pipeline, tr)

	if !errors.Is(err, testError) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPatchResolvedPipelineRunMetadataError(t *testing.T) {
	pr := &v1beta1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tr",
			Namespace: simplePipeline.Namespace,
		},
		Spec: v1beta1.PipelineRunSpec{
			PipelineRef: &v1beta1.PipelineRef{
				Name: simplePipeline.Name,
			},
		},
		Status: v1beta1.PipelineRunStatus{
			PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{
				PipelineSpec: simplePipeline.Spec.DeepCopy(),
			},
		},
	}

	assets, cancel := getClients(t, test.Data{
		Pipelines:    []*v1beta1.Pipeline{simplePipeline.DeepCopy()},
		PipelineRuns: []*v1beta1.PipelineRun{pr},
	})

	defer cancel()

	testError := errors.New("this error should be returned when patching")

	// Return error when resolver tries to patch PipelineRun metadata.
	assets.Clients.Pipeline.PrependReactor("patch", "pipelineruns", func(action ktesting.Action) (bool, runtime.Object, error) {
		if action.GetSubresource() != "" {
			t.Fatalf("unexpected patch of subresource %q", action.GetSubresource())
		}
		return true, nil, testError
	})

	pr, err := PatchResolvedPipelineRun(assets.Ctx, assets.Clients.Kube, assets.Clients.Pipeline, pr)

	if pr != nil {
		t.Errorf("non-nil pipelinerun returned by patcher")
	}

	if !errors.Is(err, testError) {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestPatchResolvedTaskRunStatusSpecError(t *testing.T) {
	tr := &v1beta1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tr",
			Namespace: "default",
		},
		Spec: v1beta1.TaskRunSpec{
			TaskRef: &v1beta1.TaskRef{
				Name: simpleTask.Name,
			},
		},
		Status: v1beta1.TaskRunStatus{
			TaskRunStatusFields: v1beta1.TaskRunStatusFields{
				TaskSpec: simpleTask.Spec.DeepCopy(),
			},
		},
	}

	assets, cancel := getClients(t, test.Data{
		Tasks:    []*v1beta1.Task{simpleTask.DeepCopy()},
		TaskRuns: []*v1beta1.TaskRun{tr},
	})

	defer cancel()

	testError := errors.New("this error should be returned when patching")

	// Return error when resolver tries to patch PipelineRun status.pipelineSpec.
	assets.Clients.Pipeline.PrependReactor("patch", "taskruns", func(action ktesting.Action) (bool, runtime.Object, error) {
		if action.GetSubresource() == "status" {
			return true, nil, testError
		}
		return false, nil, nil
	})

	tr, err := PatchResolvedTaskRun(assets.Ctx, assets.Clients.Kube, assets.Clients.Pipeline, tr)

	if tr != nil {
		t.Errorf("non-nil taskrun returned by patcher")
	}

	if !errors.Is(err, testError) {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestPatchResolvedPipelineRunStatusSpecError(t *testing.T) {
	pr := &v1beta1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tr",
			Namespace: simplePipeline.Namespace,
		},
		Spec: v1beta1.PipelineRunSpec{
			PipelineRef: &v1beta1.PipelineRef{
				Name: simplePipeline.Name,
			},
		},
		Status: v1beta1.PipelineRunStatus{
			PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{
				PipelineSpec: simplePipeline.Spec.DeepCopy(),
			},
		},
	}

	assets, cancel := getClients(t, test.Data{
		Pipelines:    []*v1beta1.Pipeline{simplePipeline.DeepCopy()},
		PipelineRuns: []*v1beta1.PipelineRun{pr},
	})

	defer cancel()

	testError := errors.New("this error should be returned when patching")

	// Return error when resolver tries to patch TaskRun metadata.
	assets.Clients.Pipeline.PrependReactor("patch", "pipelineruns", func(action ktesting.Action) (bool, runtime.Object, error) {
		if action.GetSubresource() == "status" {
			return true, nil, testError
		}
		return false, nil, nil
	})

	pr, err := PatchResolvedPipelineRun(assets.Ctx, assets.Clients.Kube, assets.Clients.Pipeline, pr)

	if pr != nil {
		t.Errorf("non-nil pipelinerun returned by patcher")
	}

	if !errors.Is(err, testError) {
		t.Errorf("unexpected error: %v", err)
	}
}
