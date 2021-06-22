/*
Copyright 2021 The Tekton Authors

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

	_ "knative.dev/pkg/system/testing" // Setup system.Namespace()
)

func TestTaskRunResolver_ReconcileMissingTaskRun(t *testing.T) {
	assets, cancel := getTaskRunResolverController(t, test.Data{
		TaskRuns: []*v1beta1.TaskRun{},
	})

	defer cancel()

	err := assets.Controller.Reconciler.Reconcile(assets.Ctx, "test/foo")

	e := &ErrorGettingResource{}
	if !errors.As(err, &e) {
		t.Errorf("received incorrect error: %v", err)
	}
}

func TestTaskRunResolver_ReconcileInvalidResourceKey(t *testing.T) {
	assets, cancel := getTaskRunResolverController(t, test.Data{})

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

func TestTaskRunResolver_ReconcileAlreadyResolved(t *testing.T) {
	tr := &v1beta1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tr",
			Namespace: "default",
		},
		Spec: v1beta1.TaskRunSpec{
			TaskRef: &v1beta1.TaskRef{Name: simpleTask.Name},
		},
		Status: v1beta1.TaskRunStatus{
			TaskRunStatusFields: v1beta1.TaskRunStatusFields{
				TaskSpec: &simpleTask.Spec,
			},
		},
	}

	assets, cancel := getTaskRunResolverController(t, test.Data{
		TaskRuns: []*v1beta1.TaskRun{tr},
	})

	defer cancel()

	key := fmt.Sprintf("%s/%s", tr.Namespace, tr.Name)
	err := assets.Controller.Reconciler.Reconcile(assets.Ctx, key)

	if err != nil {
		t.Errorf("expected nil error, received: %v", err)
	}
}

func TestTaskRunResolver_PatchesResourceStatus(t *testing.T) {
	tr := &v1beta1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tr",
			Namespace: "default",
		},
		Spec: v1beta1.TaskRunSpec{
			TaskSpec: simpleTask.Spec.DeepCopy(),
		},
	}

	assets, cancel := getTaskRunResolverController(t, test.Data{
		TaskRuns: []*v1beta1.TaskRun{tr},
	})

	defer cancel()

	key := fmt.Sprintf("%s/%s", tr.Namespace, tr.Name)
	err := assets.Controller.Reconciler.Reconcile(assets.Ctx, key)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	actions := assets.Clients.Pipeline.Actions()
	for _, a := range getPatchActions(actions, "taskruns") {
		got := map[string]struct {
			TaskSpec *v1beta1.TaskSpec `json:"taskSpec"`
		}{}
		if err := json.Unmarshal(a.GetPatch(), &got); err == nil {
			if _, hasStatus := got["status"]; hasStatus {
				if d := cmp.Diff(&simpleTask.Spec, got["status"].TaskSpec); d != "" {
					t.Errorf(diff.PrintWantGot(d))
				}
				return
			}
		}
	}
	t.Fatalf("no status patch found")
}

func TestTaskRunResolver_PatchesResourceMetadata(t *testing.T) {
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
	}

	assets, cancel := getTaskRunResolverController(t, test.Data{
		Tasks:    []*v1beta1.Task{simpleTask.DeepCopy()},
		TaskRuns: []*v1beta1.TaskRun{tr},
	})

	defer cancel()

	key := fmt.Sprintf("%s/%s", tr.Namespace, tr.Name)
	err := assets.Controller.Reconciler.Reconcile(assets.Ctx, key)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedMetadata := &metav1.ObjectMeta{
		Labels:      map[string]string{"tekton.dev/task": simpleTask.Name},
		Annotations: map[string]string{},
	}
	for key, val := range simpleTask.Labels {
		expectedMetadata.Labels[key] = val
	}
	for key, val := range simpleTask.Annotations {
		expectedMetadata.Annotations[key] = val
	}

	actions := assets.Clients.Pipeline.Actions()
	for _, a := range getPatchActions(actions, "taskruns") {
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

func TestTaskRunResolveError_ErrorDuringTaskRunGet(t *testing.T) {
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
	}

	assets, cancel := getTaskRunResolverController(t, test.Data{
		Tasks:    []*v1beta1.Task{simpleTask.DeepCopy()},
		TaskRuns: []*v1beta1.TaskRun{tr},
	})

	defer cancel()

	testError := errors.New("this error should be returned by the reconciler")

	// Return error when resolver tries to get taskRef'd Task, leading
	// resolver to try and update the TaskRun with that error detail.
	assets.Clients.Pipeline.PrependReactor("get", "tasks", func(action ktesting.Action) (bool, runtime.Object, error) {
		return true, nil, testError
	})

	// Return error when resolver tries to get the TaskRun to update it
	// with the error detail.
	assets.Clients.Pipeline.PrependReactor("get", "taskruns", func(action ktesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("GET taskrun disallowed for this test")
	})

	key := fmt.Sprintf("%s/%s", tr.Namespace, tr.Name)
	err := assets.Controller.Reconciler.Reconcile(assets.Ctx, key)

	if controller.IsPermanentError(err) {
		t.Errorf("controller is not supposed to make permanent error if TaskRun GET fails")
	}
	if !errors.Is(err, testError) {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestTaskRunResolveError_ErrorDuringTaskRunUpdate(t *testing.T) {
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
	}

	assets, cancel := getTaskRunResolverController(t, test.Data{
		Tasks:    []*v1beta1.Task{simpleTask.DeepCopy()},
		TaskRuns: []*v1beta1.TaskRun{tr},
	})

	defer cancel()

	testError := errors.New("this error should be returned by the reconciler")

	// Return error when resolver tries to get taskRef'd Task, leading
	// resolver to try and update the TaskRun with that error detail.
	assets.Clients.Pipeline.PrependReactor("get", "tasks", func(action ktesting.Action) (bool, runtime.Object, error) {
		return true, nil, testError
	})

	// Return error when resolver tries to update the TaskRun status
	// with the error detail.
	assets.Clients.Pipeline.PrependReactor("update", "taskruns", func(action ktesting.Action) (bool, runtime.Object, error) {
		if action.GetSubresource() != "status" {
			t.Fatalf("expected update only on taskrun's status")
		}
		return true, nil, errors.New("UPDATE of taskrun status disallowed for this test")
	})

	key := fmt.Sprintf("%s/%s", tr.Namespace, tr.Name)
	err := assets.Controller.Reconciler.Reconcile(assets.Ctx, key)

	if controller.IsPermanentError(err) {
		t.Errorf("controller is not supposed to make permanent error if TaskRun status UPDATE fails")
	}
	if !errors.Is(err, testError) {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestTaskRunResolver_PatchMetadataError(t *testing.T) {
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
	}

	assets, cancel := getTaskRunResolverController(t, test.Data{
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

	key := fmt.Sprintf("%s/%s", tr.Namespace, tr.Name)
	err := assets.Controller.Reconciler.Reconcile(assets.Ctx, key)

	if controller.IsPermanentError(err) {
		t.Errorf("controller is not supposed to make permanent error if TaksRun metadata PATCH fails")
	}
	if !errors.Is(err, testError) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTaskRunResolver_PatchStatusSpecError(t *testing.T) {
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
	}

	assets, cancel := getTaskRunResolverController(t, test.Data{
		Tasks:    []*v1beta1.Task{simpleTask.DeepCopy()},
		TaskRuns: []*v1beta1.TaskRun{tr},
	})

	defer cancel()

	testError := errors.New("this error should be returned when patching")

	// Return error when resolver tries to patch TaskRun metadata.
	assets.Clients.Pipeline.PrependReactor("patch", "taskruns", func(action ktesting.Action) (bool, runtime.Object, error) {
		if action.GetSubresource() == "status" {
			return true, nil, testError
		}
		return false, nil, nil
	})

	key := fmt.Sprintf("%s/%s", tr.Namespace, tr.Name)
	err := assets.Controller.Reconciler.Reconcile(assets.Ctx, key)

	if controller.IsPermanentError(err) {
		t.Errorf("controller is not supposed to make permanent error if TaksRun status.spec PATCH fails")
	}
	if !errors.Is(err, testError) {
		t.Fatalf("unexpected error: %v", err)
	}
}
