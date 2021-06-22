package resolver

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/test"
	"github.com/tektoncd/pipeline/test/diff"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ktesting "k8s.io/client-go/testing"
	"knative.dev/pkg/controller"
)

func TestPipelineRunResolver_ReconcileMissingPipelineRun(t *testing.T) {
	assets, cancel := getPipelineRunResolverController(t, test.Data{
		PipelineRuns: []*v1beta1.PipelineRun{},
	})

	defer cancel()

	err := assets.Controller.Reconciler.Reconcile(assets.Ctx, "test/foo")

	e := &ErrorGettingResource{}
	if !errors.As(err, &e) {
		t.Errorf("received incorrect error: %v", err)
	}
}

func TestPipelineRunResolver_ReconcileInvalidResourceKey(t *testing.T) {
	assets, cancel := getPipelineRunResolverController(t, test.Data{})

	defer cancel()

	err := assets.Controller.Reconciler.Reconcile(assets.Ctx, "i/n/v/a/l/i/d/k/e/y")

	e := &ErrorInvalidResourceKey{}
	if !errors.As(err, &e) {
		t.Errorf("received incorrect error: %v", err)
	}

	if !controller.IsPermanentError(err) {
		t.Errorf("expected permanent controller error for invalid key")
	}
}

func TestPipelineRunResolver_ReconcileAlreadyResolved(t *testing.T) {
	pr := &v1beta1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "foo",
			Namespace: simplePipeline.Namespace,
		},
		Spec: v1beta1.PipelineRunSpec{
			PipelineSpec: simplePipeline.Spec.DeepCopy(),
		},
		Status: v1beta1.PipelineRunStatus{
			PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{
				PipelineSpec: simplePipeline.Spec.DeepCopy(),
			},
		},
	}
	assets, cancel := getPipelineRunResolverController(t, test.Data{
		PipelineRuns: []*v1beta1.PipelineRun{pr},
	})

	defer cancel()

	key := fmt.Sprintf("%s/%s", pr.Namespace, pr.Name)
	err := assets.Controller.Reconciler.Reconcile(assets.Ctx, key)

	if err != nil {
		t.Errorf("expected nil error, received: %v", err)
	}
}

func TestPipelineRunResolver_PatchesResourceStatus(t *testing.T) {
	pr := &v1beta1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "foo",
			Namespace: simplePipeline.Namespace,
		},
		Spec: v1beta1.PipelineRunSpec{
			PipelineSpec: simplePipeline.Spec.DeepCopy(),
		},
	}

	assets, cancel := getPipelineRunResolverController(t, test.Data{
		PipelineRuns: []*v1beta1.PipelineRun{pr},
	})

	defer cancel()

	key := fmt.Sprintf("%s/%s", pr.Namespace, pr.Name)
	err := assets.Controller.Reconciler.Reconcile(assets.Ctx, key)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	actions := assets.Clients.Pipeline.Actions()
	for _, a := range getPatchActions(actions, "pipelineruns") {
		got := map[string]struct {
			PipelineSpec *v1beta1.PipelineSpec `json:"pipelineSpec"`
		}{}
		if err := json.Unmarshal(a.GetPatch(), &got); err == nil {
			if _, hasStatus := got["status"]; hasStatus {
				if d := cmp.Diff(&simplePipeline.Spec, got["status"].PipelineSpec); d != "" {
					t.Errorf(diff.PrintWantGot(d))
				}
				return
			}
		}
	}
	t.Fatalf("no status patch found")
}

func TestPipelineRunResolver_PatchesResourceMetadata(t *testing.T) {
	pr := &v1beta1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "foo",
			Namespace: simplePipeline.Namespace,
		},
		Spec: v1beta1.PipelineRunSpec{
			PipelineRef: &v1beta1.PipelineRef{
				Name: simplePipeline.Name,
			},
		},
	}

	assets, cancel := getPipelineRunResolverController(t, test.Data{
		Pipelines:    []*v1beta1.Pipeline{simplePipeline.DeepCopy()},
		PipelineRuns: []*v1beta1.PipelineRun{pr},
	})

	defer cancel()

	key := fmt.Sprintf("%s/%s", pr.Namespace, pr.Name)
	err := assets.Controller.Reconciler.Reconcile(assets.Ctx, key)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedMetadata := &metav1.ObjectMeta{
		Labels: map[string]string{
			"tekton.dev/pipeline": "test-pipeline",
		},
		Annotations: map[string]string{},
	}
	for key, val := range simplePipeline.Labels {
		expectedMetadata.Labels[key] = val
	}
	for key, val := range simplePipeline.Annotations {
		expectedMetadata.Annotations[key] = val
	}

	actions := assets.Clients.Pipeline.Actions()
	for _, a := range getPatchActions(actions, "pipelineruns") {
		got := map[string]*metav1.ObjectMeta{}
		if err := json.Unmarshal(a.GetPatch(), &got); err == nil {
			if _, hasMetadata := got["metadata"]; hasMetadata {
				if d := cmp.Diff(expectedMetadata, got["metadata"]); d != "" {
					t.Errorf(diff.PrintWantGot(d))
				}
				return
			}
		}
	}
	t.Fatalf("no metadata patch found")
}

func TestPipelineRunResolveError_ErrorDuringPipelineRunGet(t *testing.T) {
	pr := &v1beta1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pr",
			Namespace: "default",
		},
		Spec: v1beta1.PipelineRunSpec{
			PipelineRef: &v1beta1.PipelineRef{
				Name: simplePipeline.Name,
			},
		},
	}

	assets, cancel := getPipelineRunResolverController(t, test.Data{
		Pipelines:    []*v1beta1.Pipeline{simplePipeline.DeepCopy()},
		PipelineRuns: []*v1beta1.PipelineRun{pr},
	})

	defer cancel()

	testError := errors.New("this error should be returned by the reconciler")

	// Return error when resolver tries to get PipelineRef'd Pipeline, leading
	// resolver to try and update the PipelineRun with that error detail.
	assets.Clients.Pipeline.PrependReactor("get", "pipelines", func(action ktesting.Action) (bool, runtime.Object, error) {
		return true, nil, testError
	})

	// Return error when resolver tries to get the PipelineRun to update it
	// with the error detail.
	assets.Clients.Pipeline.PrependReactor("get", "pipelineruns", func(action ktesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("GET PipelineRun disallowed for this test")
	})

	key := fmt.Sprintf("%s/%s", pr.Namespace, pr.Name)
	err := assets.Controller.Reconciler.Reconcile(assets.Ctx, key)

	if controller.IsPermanentError(err) {
		t.Errorf("controller is not supposed to make permanent error if PipelineRun GET fails")
	}
	if !errors.Is(err, testError) {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestPipelineRunResolveError_ErrorDuringPipelineRunUpdate(t *testing.T) {
	pr := &v1beta1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pr",
			Namespace: "default",
		},
		Spec: v1beta1.PipelineRunSpec{
			PipelineRef: &v1beta1.PipelineRef{
				Name: simplePipeline.Name,
			},
		},
	}

	assets, cancel := getPipelineRunResolverController(t, test.Data{
		Pipelines:    []*v1beta1.Pipeline{simplePipeline.DeepCopy()},
		PipelineRuns: []*v1beta1.PipelineRun{pr},
	})

	defer cancel()

	testError := errors.New("this error should be returned by the reconciler")

	// Return error when resolver tries to get PipelineRef'd Pipeline, leading
	// resolver to try and update the PipelineRun with that error detail.
	assets.Clients.Pipeline.PrependReactor("get", "pipelines", func(action ktesting.Action) (bool, runtime.Object, error) {
		return true, nil, testError
	})

	// Return error when resolver tries to update the PipelineRun status
	// with the error detail.
	assets.Clients.Pipeline.PrependReactor("update", "pipelineruns", func(action ktesting.Action) (bool, runtime.Object, error) {
		if action.GetSubresource() != "status" {
			t.Fatalf("expected update only on PipelineRun's status")
		}
		return true, nil, errors.New("UPDATE of PipelineRun status disallowed for this test")
	})

	key := fmt.Sprintf("%s/%s", pr.Namespace, pr.Name)
	err := assets.Controller.Reconciler.Reconcile(assets.Ctx, key)

	if controller.IsPermanentError(err) {
		t.Errorf("controller is not supposed to make permanent error if PipelineRun status UPDATE fails")
	}
	if !errors.Is(err, testError) {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestPipelineRunResolver_PatchMetadataError(t *testing.T) {
	pr := &v1beta1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pr",
			Namespace: "default",
		},
		Spec: v1beta1.PipelineRunSpec{
			PipelineRef: &v1beta1.PipelineRef{
				Name: simplePipeline.Name,
			},
		},
	}

	assets, cancel := getPipelineRunResolverController(t, test.Data{
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

	key := fmt.Sprintf("%s/%s", pr.Namespace, pr.Name)
	err := assets.Controller.Reconciler.Reconcile(assets.Ctx, key)

	if controller.IsPermanentError(err) {
		t.Errorf("controller is not supposed to make permanent error if TaksRun metadata PATCH fails")
	}
	if !errors.Is(err, testError) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPipelineRunResolver_PatchStatusSpecError(t *testing.T) {
	pr := &v1beta1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pr",
			Namespace: "default",
		},
		Spec: v1beta1.PipelineRunSpec{
			PipelineRef: &v1beta1.PipelineRef{
				Name: simplePipeline.Name,
			},
		},
	}

	assets, cancel := getPipelineRunResolverController(t, test.Data{
		Pipelines:    []*v1beta1.Pipeline{simplePipeline.DeepCopy()},
		PipelineRuns: []*v1beta1.PipelineRun{pr},
	})

	defer cancel()

	testError := errors.New("this error should be returned when patching")

	// Return error when resolver tries to patch PipelineRun metadata.
	assets.Clients.Pipeline.PrependReactor("patch", "pipelineruns", func(action ktesting.Action) (bool, runtime.Object, error) {
		if action.GetSubresource() == "status" {
			return true, nil, testError
		}
		return false, nil, nil
	})

	key := fmt.Sprintf("%s/%s", pr.Namespace, pr.Name)
	err := assets.Controller.Reconciler.Reconcile(assets.Ctx, key)

	if controller.IsPermanentError(err) {
		t.Errorf("controller is not supposed to make permanent error if TaksRun status.spec PATCH fails")
	}
	if !errors.Is(err, testError) {
		t.Fatalf("unexpected error: %v", err)
	}
}
