/*
Copyright 2022 The Tekton Authors

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

package resolutionrequest

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/tektoncd/pipeline/pkg/apis/config"
	pipelinev1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/apis/resolution/v1beta1"
	th "github.com/tektoncd/pipeline/pkg/reconciler/testing"
	ttesting "github.com/tektoncd/pipeline/pkg/reconciler/testing"
	resolutioncommon "github.com/tektoncd/pipeline/pkg/resolution/common"
	"github.com/tektoncd/pipeline/test"
	"github.com/tektoncd/pipeline/test/diff"
	"github.com/tektoncd/pipeline/test/names"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ktesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/record"
	clock "k8s.io/utils/clock/testing"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	cminformer "knative.dev/pkg/configmap/informer"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"
	pkgreconciler "knative.dev/pkg/reconciler"
	"knative.dev/pkg/system"
	_ "knative.dev/pkg/system/testing" // Setup system.Namespace()
)

var (
	now                      = time.Date(2022, time.January, 1, 0, 0, 0, 0, time.UTC)
	testClock                = clock.NewFakePassiveClock(now)
	ignoreLastTransitionTime = cmpopts.IgnoreFields(apis.Condition{}, "LastTransitionTime.Inner.Time")
)

// getResolutionRequestController returns an instance of the ResolutionRequest controller/reconciler that has been seeded with
// d, where d represents the state of the system (existing resources) needed for the test.
func getResolutionRequestController(t *testing.T, d test.Data) (test.Assets, func()) {
	t.Helper()
	names.TestingSeed()
	return initializeResolutionRequestControllerAssets(t, d)
}

func initializeResolutionRequestControllerAssets(t *testing.T, d test.Data) (test.Assets, func()) {
	t.Helper()
	ctx, _ := ttesting.SetupFakeContext(t)
	ctx, cancel := context.WithCancel(ctx)
	test.EnsureConfigurationConfigMapsExist(&d)
	c, informers := test.SeedTestData(t, ctx, d)
	configMapWatcher := cminformer.NewInformedWatcher(c.Kube, system.Namespace())
	ctl := NewController(testClock)(ctx, configMapWatcher)
	if err := configMapWatcher.Start(ctx.Done()); err != nil {
		t.Fatalf("error starting configmap watcher: %v", err)
	}

	if la, ok := ctl.Reconciler.(pkgreconciler.LeaderAware); ok {
		la.Promote(pkgreconciler.UniversalBucket(), func(pkgreconciler.Bucket, types.NamespacedName) {})
	}

	return test.Assets{
		Logger:     logging.FromContext(ctx),
		Controller: ctl,
		Clients:    c,
		Informers:  informers,
		Recorder:   controller.GetEventRecorder(ctx).(*record.FakeRecorder),
		Ctx:        ctx,
	}, cancel
}

func TestReconcile(t *testing.T) {
	testCases := []struct {
		name           string
		input          *v1beta1.ResolutionRequest
		expectedStatus *v1beta1.ResolutionRequestStatus
	}{
		{
			name: "new request",
			input: &v1beta1.ResolutionRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "rr",
					Namespace:         "foo",
					CreationTimestamp: metav1.Time{Time: time.Now()},
				},
				Spec:   v1beta1.ResolutionRequestSpec{},
				Status: v1beta1.ResolutionRequestStatus{},
			},
			expectedStatus: &v1beta1.ResolutionRequestStatus{
				Status: duckv1.Status{
					Conditions: duckv1.Conditions{{
						Type:    apis.ConditionSucceeded,
						Status:  corev1.ConditionUnknown,
						Reason:  resolutioncommon.ReasonResolutionInProgress,
						Message: resolutioncommon.MessageWaitingForResolver,
					}},
				},
				ResolutionRequestStatusFields: v1beta1.ResolutionRequestStatusFields{},
			},
		}, {
			name: "timed out request",
			input: &v1beta1.ResolutionRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "rr",
					Namespace:         "foo",
					CreationTimestamp: metav1.Time{Time: time.Now().Add(-2 * time.Minute)},
				},
				Spec:   v1beta1.ResolutionRequestSpec{},
				Status: v1beta1.ResolutionRequestStatus{},
			},
			expectedStatus: &v1beta1.ResolutionRequestStatus{
				Status: duckv1.Status{
					Conditions: duckv1.Conditions{{
						Type:    apis.ConditionSucceeded,
						Status:  corev1.ConditionFalse,
						Reason:  resolutioncommon.ReasonResolutionTimedOut,
						Message: timeoutMessage(config.FromContextOrDefaults(t.Context()).Defaults.DefaultMaximumResolutionTimeout),
					}},
				},
				ResolutionRequestStatusFields: v1beta1.ResolutionRequestStatusFields{},
			},
		}, {
			name: "populated request",
			input: &v1beta1.ResolutionRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "rr",
					Namespace:         "foo",
					CreationTimestamp: metav1.Time{Time: time.Now()},
				},
				Spec: v1beta1.ResolutionRequestSpec{},
				Status: v1beta1.ResolutionRequestStatus{
					ResolutionRequestStatusFields: v1beta1.ResolutionRequestStatusFields{
						Data: "some data",
					},
				},
			},
			expectedStatus: &v1beta1.ResolutionRequestStatus{
				Status: duckv1.Status{
					Conditions: duckv1.Conditions{{
						Type:   apis.ConditionSucceeded,
						Status: corev1.ConditionTrue,
					}},
				},
				ResolutionRequestStatusFields: v1beta1.ResolutionRequestStatusFields{
					Data: "some data",
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			d := test.Data{
				ResolutionRequests: []*v1beta1.ResolutionRequest{tc.input},
				ConfigMaps:         th.NewDefaultsConfigMapInSlice(),
			}

			testAssets, cancel := getResolutionRequestController(t, d)
			defer cancel()

			err := testAssets.Controller.Reconciler.Reconcile(testAssets.Ctx, getRequestName(tc.input))
			if err != nil {
				if ok, _ := controller.IsRequeueKey(err); !ok {
					t.Fatalf("did not expect an error, but got %v", err)
				}
			}
			reconciledRR, err := testAssets.Clients.ResolutionRequests.ResolutionV1beta1().ResolutionRequests(tc.input.Namespace).Get(testAssets.Ctx, tc.input.Name, metav1.GetOptions{})
			if err != nil {
				t.Fatalf("getting updated ResolutionRequest: %v", err)
			}
			if d := cmp.Diff(*tc.expectedStatus, reconciledRR.Status, ignoreLastTransitionTime); d != "" {
				t.Errorf("ResolutionRequest status doesn't match %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestReconcileWrapsLifecycleStatusError(t *testing.T) {
	input := &v1beta1.ResolutionRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "rr",
			Namespace:         "foo",
			CreationTimestamp: metav1.Now(),
		},
	}
	d := test.Data{
		ResolutionRequests: []*v1beta1.ResolutionRequest{input},
		ConfigMaps:         th.NewDefaultsConfigMapInSlice(),
	}
	testAssets, cancel := getResolutionRequestController(t, d)
	defer cancel()

	testAssets.Clients.ResolutionRequests.PrependReactor("patch", "resolutionrequests", func(ktesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("write failed")
	})
	if err := testAssets.Controller.Reconciler.Reconcile(testAssets.Ctx, getRequestName(input)); err == nil || err.Error() != "failed to update resource lifecycle status: write failed" {
		t.Fatalf("Reconcile() error = %v, want wrapped lifecycle status error", err)
	}
}

func TestReconcilePreservesResolverStatusOnConflict(t *testing.T) {
	wantFields := v1beta1.ResolutionRequestStatusFields{
		Data: "resolved data",
		Source: &pipelinev1.RefSource{
			URI: "source",
		},
		RefSource: &pipelinev1.RefSource{
			URI: "ref-source",
		},
	}
	wantAnnotations := map[string]string{"resolver.example/cache": "hit"}
	testCases := []struct {
		name                string
		concurrentUpdate    func(*v1beta1.ResolutionRequest)
		assertPreserved     func(*testing.T, *v1beta1.ResolutionRequest)
		wantConditionStatus corev1.ConditionStatus
	}{
		{
			name: "resolver payload",
			concurrentUpdate: func(latest *v1beta1.ResolutionRequest) {
				latest.Status.ResolutionRequestStatusFields = wantFields
				latest.Status.Annotations = wantAnnotations
			},
			assertPreserved: func(t *testing.T, latest *v1beta1.ResolutionRequest) {
				t.Helper()
				assertResolverStatus(t, latest, wantFields, wantAnnotations)
			},
			wantConditionStatus: corev1.ConditionTrue,
		},
		{
			name: "resolver failure",
			concurrentUpdate: func(latest *v1beta1.ResolutionRequest) {
				latest.Status.ObservedGeneration = latest.Generation
				latest.Status.InitializeConditions()
				latest.Status.MarkFailed("ResolverFailed", "resolver failed")
			},
			assertPreserved:     assertResolverFailure,
			wantConditionStatus: corev1.ConditionFalse,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			input := &v1beta1.ResolutionRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "rr",
					Namespace:         "foo",
					CreationTimestamp: metav1.Now(),
					Generation:        1,
					ResourceVersion:   "1",
				},
			}
			d := test.Data{
				ResolutionRequests: []*v1beta1.ResolutionRequest{input},
				ConfigMaps:         th.NewDefaultsConfigMapInSlice(),
			}
			testAssets, cancel := getResolutionRequestController(t, d)
			defer cancel()

			prependConcurrentStatusUpdate(t, testAssets, input, tc.concurrentUpdate)
			err := testAssets.Controller.Reconciler.Reconcile(testAssets.Ctx, getRequestName(input))
			if ok, delay := controller.IsRequeueKey(err); !ok || delay != 0 {
				t.Fatalf("first reconcile result = %v, want immediate requeue", err)
			}

			latest, err := testAssets.Clients.ResolutionRequests.ResolutionV1beta1().ResolutionRequests(input.Namespace).Get(testAssets.Ctx, input.Name, metav1.GetOptions{})
			if err != nil {
				t.Fatalf("get ResolutionRequest after conflict: %v", err)
			}
			tc.assertPreserved(t, latest)
			if err := testAssets.Informers.ResolutionRequest.Informer().GetIndexer().Update(latest.DeepCopy()); err != nil {
				t.Fatalf("update ResolutionRequest informer: %v", err)
			}

			if err := testAssets.Controller.Reconciler.Reconcile(testAssets.Ctx, getRequestName(input)); err != nil {
				t.Fatalf("second reconcile: %v", err)
			}
			latest, err = testAssets.Clients.ResolutionRequests.ResolutionV1beta1().ResolutionRequests(input.Namespace).Get(testAssets.Ctx, input.Name, metav1.GetOptions{})
			if err != nil {
				t.Fatalf("get reconciled ResolutionRequest: %v", err)
			}
			tc.assertPreserved(t, latest)
			if condition := latest.Status.GetCondition(apis.ConditionSucceeded); condition == nil || condition.Status != tc.wantConditionStatus {
				t.Fatalf("Succeeded condition = %#v, want %s", condition, tc.wantConditionStatus)
			}
		})
	}
}

func prependConcurrentStatusUpdate(t *testing.T, testAssets test.Assets, input *v1beta1.ResolutionRequest, update func(*v1beta1.ResolutionRequest)) {
	t.Helper()
	gvr := v1beta1.SchemeGroupVersion.WithResource("resolutionrequests")
	injected := false
	testAssets.Clients.ResolutionRequests.PrependReactor("patch", "resolutionrequests", func(action ktesting.Action) (bool, runtime.Object, error) {
		patchAction := action.(ktesting.PatchAction)
		if got := patchAction.GetSubresource(); got != "status" {
			t.Fatalf("patch subresource = %q, want status", got)
		}

		var patch struct {
			Metadata map[string]json.RawMessage `json:"metadata"`
			Status   map[string]json.RawMessage `json:"status"`
		}
		if err := json.Unmarshal(patchAction.GetPatch(), &patch); err != nil {
			t.Fatalf("unmarshal status patch: %v", err)
		}
		if len(patch.Metadata) != 1 || patch.Metadata["resourceVersion"] == nil {
			t.Fatalf("metadata patch = %s, want only resourceVersion", patchAction.GetPatch())
		}
		if len(patch.Status) != 2 || patch.Status["conditions"] == nil || patch.Status["observedGeneration"] == nil {
			t.Fatalf("status patch = %s, want only conditions and observedGeneration", patchAction.GetPatch())
		}
		var patchResourceVersion string
		if err := json.Unmarshal(patch.Metadata["resourceVersion"], &patchResourceVersion); err != nil {
			t.Fatalf("unmarshal patch resourceVersion: %v", err)
		}

		if !injected {
			injected = true
			obj, err := testAssets.Clients.ResolutionRequests.Tracker().Get(gvr, input.Namespace, input.Name)
			if err != nil {
				t.Fatalf("get tracked ResolutionRequest: %v", err)
			}
			latest := obj.(*v1beta1.ResolutionRequest).DeepCopy()
			latest.ResourceVersion = "2"
			update(latest)
			if err := testAssets.Clients.ResolutionRequests.Tracker().Update(gvr, latest, input.Namespace); err != nil {
				t.Fatalf("inject resolver status: %v", err)
			}
		}

		obj, err := testAssets.Clients.ResolutionRequests.Tracker().Get(gvr, input.Namespace, input.Name)
		if err != nil {
			t.Fatalf("get current ResolutionRequest: %v", err)
		}
		if currentResourceVersion := obj.(*v1beta1.ResolutionRequest).ResourceVersion; patchResourceVersion != currentResourceVersion {
			return true, nil, apierrors.NewConflict(gvr.GroupResource(), input.Name, errors.New("resourceVersion changed"))
		}
		return false, nil, nil
	})
}

func assertResolverFailure(t *testing.T, rr *v1beta1.ResolutionRequest) {
	t.Helper()
	condition := rr.Status.GetCondition(apis.ConditionSucceeded)
	if condition == nil || condition.Status != corev1.ConditionFalse || condition.Reason != "ResolverFailed" || condition.Message != "resolver failed" {
		t.Errorf("Succeeded condition = %#v, want resolver failure", condition)
	}
}

func assertResolverStatus(t *testing.T, rr *v1beta1.ResolutionRequest, wantFields v1beta1.ResolutionRequestStatusFields, wantAnnotations map[string]string) {
	t.Helper()
	if d := cmp.Diff(wantFields, rr.Status.ResolutionRequestStatusFields); d != "" {
		t.Errorf("resolver status fields changed (-want, +got): %s", d)
	}
	if d := cmp.Diff(wantAnnotations, rr.Status.Annotations); d != "" {
		t.Errorf("resolver status annotations changed (-want, +got): %s", d)
	}
}

func getRequestName(rr *v1beta1.ResolutionRequest) string {
	return strings.Join([]string{rr.Namespace, rr.Name}, "/")
}
