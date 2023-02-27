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
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/tektoncd/pipeline/pkg/apis/resolution/v1beta1"
	ttesting "github.com/tektoncd/pipeline/pkg/reconciler/testing"
	resolutioncommon "github.com/tektoncd/pipeline/pkg/resolution/common"
	"github.com/tektoncd/pipeline/test"
	"github.com/tektoncd/pipeline/test/diff"
	"github.com/tektoncd/pipeline/test/names"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
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
						Message: timeoutMessage(),
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

func getRequestName(rr *v1beta1.ResolutionRequest) string {
	return strings.Join([]string{rr.Namespace, rr.Name}, "/")
}
