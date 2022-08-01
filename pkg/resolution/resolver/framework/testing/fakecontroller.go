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

package testing

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/tektoncd/pipeline/pkg/apis/resolution/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/resolution/resolver/framework"
	"github.com/tektoncd/pipeline/test"
	"github.com/tektoncd/pipeline/test/diff"
	"github.com/tektoncd/pipeline/test/names"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/clock"
	"k8s.io/client-go/tools/record"
	"knative.dev/pkg/apis"
	cminformer "knative.dev/pkg/configmap/informer"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"
	pkgreconciler "knative.dev/pkg/reconciler"
	"knative.dev/pkg/system"
)

var (
	now                      = time.Date(2022, time.January, 1, 0, 0, 0, 0, time.UTC)
	testClock                = clock.NewFakePassiveClock(now)
	ignoreLastTransitionTime = cmpopts.IgnoreFields(apis.Condition{}, "LastTransitionTime.Inner.Time")
)

// RunResolverReconcileTest takes data to seed clients and informers, a Resolver, a ResolutionRequest, and the expected
// ResolutionRequestStatus and error, both of which can be nil. It instantiates a controller for that resolver and
// reconciles the given request. It then checks for the expected error, if any, and compares the resulting status with
// the expected status.
func RunResolverReconcileTest(ctx context.Context, t *testing.T, d test.Data, resolver framework.Resolver, request *v1alpha1.ResolutionRequest,
	expectedStatus *v1alpha1.ResolutionRequestStatus, expectedErr error) {
	t.Helper()

	testAssets, cancel := GetResolverFrameworkController(ctx, t, d, resolver, setClockOnReconciler)
	defer cancel()

	err := testAssets.Controller.Reconciler.Reconcile(testAssets.Ctx, getRequestName(request))
	if expectedErr != nil {
		if err == nil {
			t.Fatalf("expected to get error %v, but got nothing", expectedErr)
		}
		if expectedErr.Error() != err.Error() {
			t.Fatalf("expected to get error %v, but got %v", expectedErr, err)
		}
	} else if err != nil {
		if ok, _ := controller.IsRequeueKey(err); !ok {
			t.Fatalf("did not expect an error, but got %v", err)
		}
	}

	c := testAssets.Clients.ResolutionRequests.ResolutionV1alpha1()
	reconciledRR, err := c.ResolutionRequests(request.Namespace).Get(testAssets.Ctx, request.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("getting updated ResolutionRequest: %v", err)
	}
	if expectedStatus != nil {
		if d := cmp.Diff(*expectedStatus, reconciledRR.Status, ignoreLastTransitionTime); d != "" {
			t.Errorf("ResolutionRequest status doesn't match %s", diff.PrintWantGot(d))
		}
	}
}

// GetResolverFrameworkController returns an instance of the resolver framework controller/reconciler using the given resolver,
// seeded with d, where d represents the state of the system (existing resources) needed for the test.
func GetResolverFrameworkController(ctx context.Context, t *testing.T, d test.Data, resolver framework.Resolver, modifiers ...framework.ReconcilerModifier) (test.Assets, func()) {
	t.Helper()
	names.TestingSeed()
	return initializeResolverFrameworkControllerAssets(ctx, t, d, resolver, modifiers...)
}

func initializeResolverFrameworkControllerAssets(ctx context.Context, t *testing.T, d test.Data, resolver framework.Resolver, modifiers ...framework.ReconcilerModifier) (test.Assets, func()) {
	t.Helper()
	ctx, cancel := context.WithCancel(ctx)
	c, informers := test.SeedTestData(t, ctx, d)
	configMapWatcher := cminformer.NewInformedWatcher(c.Kube, system.Namespace())
	ctl := framework.NewController(ctx, resolver, modifiers...)(ctx, configMapWatcher)
	if err := configMapWatcher.Start(ctx.Done()); err != nil {
		t.Fatalf("error starting configmap watcher: %v", err)
	}

	if la, ok := ctl.Reconciler.(pkgreconciler.LeaderAware); ok {
		_ = la.Promote(pkgreconciler.UniversalBucket(), func(pkgreconciler.Bucket, types.NamespacedName) {})
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

func getRequestName(rr *v1alpha1.ResolutionRequest) string {
	return strings.Join([]string{rr.Namespace, rr.Name}, "/")
}

func setClockOnReconciler(r *framework.Reconciler) {
	if r.Clock == nil {
		r.Clock = testClock
	}
}
