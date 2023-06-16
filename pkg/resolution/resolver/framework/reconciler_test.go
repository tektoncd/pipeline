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

package framework_test

import (
	"context"
	"encoding/base64"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	pipelinev1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/apis/resolution/v1beta1"
	ttesting "github.com/tektoncd/pipeline/pkg/reconciler/testing"
	resolutioncommon "github.com/tektoncd/pipeline/pkg/resolution/common"
	framework "github.com/tektoncd/pipeline/pkg/resolution/resolver/framework"
	"github.com/tektoncd/pipeline/test"
	"github.com/tektoncd/pipeline/test/diff"
	"github.com/tektoncd/pipeline/test/names"
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

func TestReconcile(t *testing.T) {
	testCases := []struct {
		name              string
		inputRequest      *v1beta1.ResolutionRequest
		paramMap          map[string]*framework.FakeResolvedResource
		reconcilerTimeout time.Duration
		expectedStatus    *v1beta1.ResolutionRequestStatus
		expectedErr       error
	}{
		{
			name: "unknown value",
			inputRequest: &v1beta1.ResolutionRequest{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "resolution.tekton.dev/v1beta1",
					Kind:       "ResolutionRequest",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:              "rr",
					Namespace:         "foo",
					CreationTimestamp: metav1.Time{Time: time.Now()},
					Labels: map[string]string{
						resolutioncommon.LabelKeyResolverType: framework.LabelValueFakeResolverType,
					},
				},
				Spec: v1beta1.ResolutionRequestSpec{
					Params: []pipelinev1.Param{{
						Name:  framework.FakeParamName,
						Value: *pipelinev1.NewStructuredValues("bar"),
					}},
				},
				Status: v1beta1.ResolutionRequestStatus{},
			},
			expectedErr: errors.New("error getting \"Fake\" \"foo/rr\": couldn't find resource for param value bar"),
		}, {
			name: "known value",
			inputRequest: &v1beta1.ResolutionRequest{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "resolution.tekton.dev/v1beta1",
					Kind:       "ResolutionRequest",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:              "rr",
					Namespace:         "foo",
					CreationTimestamp: metav1.Time{Time: time.Now()},
					Labels: map[string]string{
						resolutioncommon.LabelKeyResolverType: framework.LabelValueFakeResolverType,
					},
				},
				Spec: v1beta1.ResolutionRequestSpec{
					Params: []pipelinev1.Param{{
						Name:  framework.FakeParamName,
						Value: *pipelinev1.NewStructuredValues("bar"),
					}},
				},
				Status: v1beta1.ResolutionRequestStatus{},
			},
			paramMap: map[string]*framework.FakeResolvedResource{
				"bar": {
					Content:       "some content",
					AnnotationMap: map[string]string{"foo": "bar"},
					ContentSource: &pipelinev1.RefSource{
						URI: "https://abc.com",
						Digest: map[string]string{
							"sha1": "xyz",
						},
						EntryPoint: "foo/bar",
					},
				},
			},
			expectedStatus: &v1beta1.ResolutionRequestStatus{
				Status: duckv1.Status{
					Annotations: map[string]string{
						"foo": "bar",
					},
				},
				ResolutionRequestStatusFields: v1beta1.ResolutionRequestStatusFields{
					Data: base64.StdEncoding.Strict().EncodeToString([]byte("some content")),
					RefSource: &pipelinev1.RefSource{
						URI: "https://abc.com",
						Digest: map[string]string{
							"sha1": "xyz",
						},
						EntryPoint: "foo/bar",
					},
					Source: &pipelinev1.RefSource{
						URI: "https://abc.com",
						Digest: map[string]string{
							"sha1": "xyz",
						},
						EntryPoint: "foo/bar",
					},
				},
			},
		}, {
			name: "error resolving",
			inputRequest: &v1beta1.ResolutionRequest{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "resolution.tekton.dev/v1beta1",
					Kind:       "ResolutionRequest",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:              "rr",
					Namespace:         "foo",
					CreationTimestamp: metav1.Time{Time: time.Now()},
					Labels: map[string]string{
						resolutioncommon.LabelKeyResolverType: framework.LabelValueFakeResolverType,
					},
				},
				Spec: v1beta1.ResolutionRequestSpec{
					Params: []pipelinev1.Param{{
						Name:  framework.FakeParamName,
						Value: *pipelinev1.NewStructuredValues("bar"),
					}},
				},
				Status: v1beta1.ResolutionRequestStatus{},
			},
			paramMap: map[string]*framework.FakeResolvedResource{
				"bar": {
					ErrorWith: "fake failure",
				},
			},
			expectedErr: errors.New(`error getting "Fake" "foo/rr": fake failure`),
		}, {
			name: "timeout",
			inputRequest: &v1beta1.ResolutionRequest{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "resolution.tekton.dev/v1beta1",
					Kind:       "ResolutionRequest",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:              "rr",
					Namespace:         "foo",
					CreationTimestamp: metav1.Time{Time: time.Now().Add(-59 * time.Second)}, // 1 second before default timeout
					Labels: map[string]string{
						resolutioncommon.LabelKeyResolverType: framework.LabelValueFakeResolverType,
					},
				},
				Spec: v1beta1.ResolutionRequestSpec{
					Params: []pipelinev1.Param{{
						Name:  framework.FakeParamName,
						Value: *pipelinev1.NewStructuredValues("bar"),
					}},
				},
				Status: v1beta1.ResolutionRequestStatus{},
			},
			paramMap: map[string]*framework.FakeResolvedResource{
				"bar": {
					WaitFor: 1100 * time.Millisecond,
				},
			},
			reconcilerTimeout: 1 * time.Second,
			expectedErr:       errors.New("context deadline exceeded"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			d := test.Data{
				ResolutionRequests: []*v1beta1.ResolutionRequest{tc.inputRequest},
			}

			fakeResolver := &framework.FakeResolver{ForParam: tc.paramMap}
			if tc.reconcilerTimeout > 0 {
				fakeResolver.Timeout = tc.reconcilerTimeout
			}

			ctx, _ := ttesting.SetupFakeContext(t)
			testAssets, cancel := getResolverFrameworkController(ctx, t, d, fakeResolver, setClockOnReconciler)
			defer cancel()

			err := testAssets.Controller.Reconciler.Reconcile(testAssets.Ctx, getRequestName(tc.inputRequest))
			if tc.expectedErr != nil {
				if err == nil {
					t.Fatalf("expected to get error %v, but got nothing", tc.expectedErr)
				}
				if tc.expectedErr.Error() != err.Error() {
					t.Fatalf("expected to get error %v, but got %v", tc.expectedErr, err)
				}
			} else {
				if err != nil {
					if ok, _ := controller.IsRequeueKey(err); !ok {
						t.Fatalf("did not expect an error, but got %v", err)
					}
				}

				c := testAssets.Clients.ResolutionRequests.ResolutionV1beta1()
				reconciledRR, err := c.ResolutionRequests(tc.inputRequest.Namespace).Get(testAssets.Ctx, tc.inputRequest.Name, metav1.GetOptions{})
				if err != nil {
					t.Fatalf("getting updated ResolutionRequest: %v", err)
				}
				if d := cmp.Diff(*tc.expectedStatus, reconciledRR.Status, ignoreLastTransitionTime); d != "" {
					t.Errorf("ResolutionRequest status doesn't match %s", diff.PrintWantGot(d))
				}
			}
		})
	}
}

func getResolverFrameworkController(ctx context.Context, t *testing.T, d test.Data, resolver framework.Resolver, modifiers ...framework.ReconcilerModifier) (test.Assets, func()) {
	t.Helper()
	names.TestingSeed()

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

func getRequestName(rr *v1beta1.ResolutionRequest) string {
	return strings.Join([]string{rr.Namespace, rr.Name}, "/")
}

func setClockOnReconciler(r *framework.Reconciler) {
	if r.Clock == nil {
		r.Clock = testClock
	}
}
