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
	"github.com/tektoncd/pipeline/pkg/remoteresolution/resolver/framework"
	resolutioncommon "github.com/tektoncd/pipeline/pkg/resolution/common"
	resolutionframework "github.com/tektoncd/pipeline/pkg/resolution/resolver/framework"
	"github.com/tektoncd/pipeline/test"
	"github.com/tektoncd/pipeline/test/diff"
	"github.com/tektoncd/pipeline/test/names"
	"go.uber.org/goleak"
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

func TestReconcile(t *testing.T) {
	testCases := []struct {
		name              string
		inputRequest      *v1beta1.ResolutionRequest
		paramMap          map[string]*resolutionframework.FakeResolvedResource
		reconcilerTimeout time.Duration
		expectedStatus    *v1beta1.ResolutionRequestStatus
		expectedErr       error
		transient         bool
	}{
		{
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
						resolutioncommon.LabelKeyResolverType: resolutionframework.LabelValueFakeResolverType,
					},
				},
				Spec: v1beta1.ResolutionRequestSpec{
					Params: []pipelinev1.Param{{
						Name:  resolutionframework.FakeParamName,
						Value: *pipelinev1.NewStructuredValues("bar"),
					}},
				},
				Status: v1beta1.ResolutionRequestStatus{},
			},
			paramMap: map[string]*resolutionframework.FakeResolvedResource{
				"bar": {
					Content:       "{\"apiVersion\": \"tekton.dev/v1\", \"kind\": \"Pipeline\"}",
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
					Data: base64.StdEncoding.Strict().EncodeToString([]byte("{\"apiVersion\": \"tekton.dev/v1\", \"kind\": \"Pipeline\"}")),
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
			name: "known invalid value",
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
						resolutioncommon.LabelKeyResolverType: resolutionframework.LabelValueFakeResolverType,
					},
				},
				Spec: v1beta1.ResolutionRequestSpec{
					Params: []pipelinev1.Param{{
						Name:  resolutionframework.FakeParamName,
						Value: *pipelinev1.NewStructuredValues("bar"),
					}},
				},
				Status: v1beta1.ResolutionRequestStatus{},
			},
			paramMap: map[string]*resolutionframework.FakeResolvedResource{
				"bar": {
					Content:       "foo: bar\nbax: baz",
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
			expectedErr: errors.New("error getting \"Fake\" \"foo/rr\": resolved resource validation error: resolved data is not of a supported type, must be of Group: tekton.dev, Kinds: [PipelineRun Pipeline TaskRun Task Run CustomRun StepAction]"),
		}, {
			name: "known value unknown type",
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
						resolutioncommon.LabelKeyResolverType: resolutionframework.LabelValueFakeResolverType,
					},
				},
				Spec: v1beta1.ResolutionRequestSpec{
					Params: []pipelinev1.Param{{
						Name:  resolutionframework.FakeParamName,
						Value: *pipelinev1.NewStructuredValues("bar"),
					}},
				},
				Status: v1beta1.ResolutionRequestStatus{},
			},
			paramMap: map[string]*resolutionframework.FakeResolvedResource{
				"bar": {
					Content:       "{\"apiVersion\": \"other/type\", \"kind\": \"NonTekton\"}",
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
			expectedErr: errors.New("error getting \"Fake\" \"foo/rr\": resolved resource validation error: resolved data is not of a supported type, must be of Group: tekton.dev, Kinds: [PipelineRun Pipeline TaskRun Task Run CustomRun StepAction]"),
		}, {
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
						resolutioncommon.LabelKeyResolverType: resolutionframework.LabelValueFakeResolverType,
					},
				},
				Spec: v1beta1.ResolutionRequestSpec{
					Params: []pipelinev1.Param{{
						Name:  resolutionframework.FakeParamName,
						Value: *pipelinev1.NewStructuredValues("bar"),
					}},
				},
				Status: v1beta1.ResolutionRequestStatus{},
			},
			expectedErr: errors.New("error getting \"Fake\" \"foo/rr\": couldn't find resource for param value bar"),
		}, {
			name: "unknown url",
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
						resolutioncommon.LabelKeyResolverType: resolutionframework.LabelValueFakeResolverType,
					},
				},
				Spec: v1beta1.ResolutionRequestSpec{
					URL: "dne://does-not-exist",
				},
				Status: v1beta1.ResolutionRequestStatus{},
			},
			expectedErr: errors.New("invalid resource request \"foo/rr\": Wrong url. Expected: fake://url,  Got: dne://does-not-exist"),
		}, {
			name: "valid url",
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
						resolutioncommon.LabelKeyResolverType: resolutionframework.LabelValueFakeResolverType,
					},
				},
				Spec: v1beta1.ResolutionRequestSpec{
					URL: framework.FakeUrl,
				},
				Status: v1beta1.ResolutionRequestStatus{},
			},
			paramMap: map[string]*resolutionframework.FakeResolvedResource{
				framework.FakeUrl: {
					Content:       "{\"apiVersion\": \"tekton.dev/v1\", \"kind\": \"Pipeline\"}",
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
					Data: base64.StdEncoding.Strict().EncodeToString([]byte("{\"apiVersion\": \"tekton.dev/v1\", \"kind\": \"Pipeline\"}")),
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
			name: "resource not found for url",
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
						resolutioncommon.LabelKeyResolverType: resolutionframework.LabelValueFakeResolverType,
					},
				},
				Spec: v1beta1.ResolutionRequestSpec{
					URL: framework.FakeUrl,
				},
				Status: v1beta1.ResolutionRequestStatus{},
			},
			paramMap: map[string]*resolutionframework.FakeResolvedResource{
				"other://resource": {
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
			expectedErr: errors.New("error getting \"Fake\" \"foo/rr\": couldn't find resource for url fake://url"),
		}, {
			name: "invalid params",
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
						resolutioncommon.LabelKeyResolverType: resolutionframework.LabelValueFakeResolverType,
					},
				},
				Spec: v1beta1.ResolutionRequestSpec{
					Params: []pipelinev1.Param{{
						Name:  "not-a-fake-param",
						Value: *pipelinev1.NewStructuredValues("bar"),
					}},
				},
				Status: v1beta1.ResolutionRequestStatus{},
			},
			expectedErr: errors.New(`invalid resource request "foo/rr": missing fake-key`),
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
						resolutioncommon.LabelKeyResolverType: resolutionframework.LabelValueFakeResolverType,
					},
				},
				Spec: v1beta1.ResolutionRequestSpec{
					Params: []pipelinev1.Param{{
						Name:  resolutionframework.FakeParamName,
						Value: *pipelinev1.NewStructuredValues("bar"),
					}},
				},
				Status: v1beta1.ResolutionRequestStatus{},
			},
			paramMap: map[string]*resolutionframework.FakeResolvedResource{
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
						resolutioncommon.LabelKeyResolverType: resolutionframework.LabelValueFakeResolverType,
					},
				},
				Spec: v1beta1.ResolutionRequestSpec{
					Params: []pipelinev1.Param{{
						Name:  resolutionframework.FakeParamName,
						Value: *pipelinev1.NewStructuredValues("bar"),
					}},
				},
				Status: v1beta1.ResolutionRequestStatus{},
			},
			paramMap: map[string]*resolutionframework.FakeResolvedResource{
				"bar": {
					WaitFor: 1100 * time.Millisecond,
				},
			},
			reconcilerTimeout: 1 * time.Second,
			expectedErr:       errors.New("context deadline exceeded"),
			transient:         true,
		}, {
			name: "resolved but not yet done should skip re-resolution",
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
						resolutioncommon.LabelKeyResolverType: resolutionframework.LabelValueFakeResolverType,
					},
				},
				Spec: v1beta1.ResolutionRequestSpec{
					Params: []pipelinev1.Param{{
						Name:  resolutionframework.FakeParamName,
						Value: *pipelinev1.NewStructuredValues("bar"),
					}},
				},
				Status: v1beta1.ResolutionRequestStatus{
					Status: duckv1.Status{
						Conditions: duckv1.Conditions{{
							Type:   apis.ConditionSucceeded,
							Status: corev1.ConditionUnknown,
						}},
					},
					ResolutionRequestStatusFields: v1beta1.ResolutionRequestStatusFields{
						Data: base64.StdEncoding.Strict().EncodeToString(
							[]byte(`{"apiVersion": "tekton.dev/v1", "kind": "Pipeline"}`),
						),
					},
				},
			},
			paramMap: map[string]*resolutionframework.FakeResolvedResource{
				"bar": {
					ErrorWith: "resolver should not have been called",
				},
			},
			expectedStatus: &v1beta1.ResolutionRequestStatus{
				Status: duckv1.Status{
					Conditions: duckv1.Conditions{{
						Type:   apis.ConditionSucceeded,
						Status: corev1.ConditionUnknown,
					}},
				},
				ResolutionRequestStatusFields: v1beta1.ResolutionRequestStatusFields{
					Data: base64.StdEncoding.Strict().EncodeToString(
						[]byte(`{"apiVersion": "tekton.dev/v1", "kind": "Pipeline"}`),
					),
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			d := test.Data{
				ResolutionRequests: []*v1beta1.ResolutionRequest{tc.inputRequest},
				ConfigMaps: []*corev1.ConfigMap{{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "resolver-cache-config",
						Namespace: system.Namespace(),
					},
					Data: map[string]string{},
				}},
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
				if tc.transient && controller.IsPermanentError(err) {
					t.Fatalf("exepected error to not be wrapped as permanent %v", err)
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

// controlledResolver is a fake resolver whose Resolve blocks on an explicit
// signal so a test can deterministically exercise the timeout-vs-publish race
// without depending on the noisy, process-global runtime.NumGoroutine() count.
// Resolve closes started when it begins and waits on release before returning,
// letting the reconciler time out first; the test then confirms the worker
// goroutine exits instead of blocking forever on its result channel.
type controlledResolver struct {
	timeout time.Duration
	started chan struct{}
	release chan struct{}
}

var (
	_ framework.Resolver                  = &controlledResolver{}
	_ resolutionframework.TimedResolution = &controlledResolver{}
)

func (r *controlledResolver) Initialize(context.Context) error { return nil }
func (r *controlledResolver) GetName(context.Context) string   { return "controlled" }
func (r *controlledResolver) Validate(context.Context, *v1beta1.ResolutionRequestSpec) error {
	return nil
}

func (r *controlledResolver) GetSelector(context.Context) map[string]string {
	return map[string]string{resolutioncommon.LabelKeyResolverType: resolutionframework.LabelValueFakeResolverType}
}

func (r *controlledResolver) GetResolutionTimeout(_ context.Context, def time.Duration, _ map[string]string) (time.Duration, error) {
	if r.timeout > 0 {
		return r.timeout, nil
	}
	return def, nil
}

func (r *controlledResolver) Resolve(context.Context, *v1beta1.ResolutionRequestSpec) (resolutionframework.ResolvedResource, error) {
	close(r.started)
	<-r.release
	return &resolutionframework.FakeResolvedResource{Content: `{"apiVersion": "tekton.dev/v1", "kind": "Pipeline"}`}, nil
}

// TestResolveGoroutineLeak asserts the contract directly: when resolution times
// out the reconciler returns, and when the resolver later finishes the worker
// goroutine must not block forever publishing its result. The buffered (cap-1)
// errChan/resourceChan in resolve() let the worker deposit its value and exit;
// reverting them to unbuffered makes the worker block, which goleak catches.
func TestResolveGoroutineLeak(t *testing.T) {
	resolver := &controlledResolver{
		timeout: 10 * time.Millisecond,
		started: make(chan struct{}),
		release: make(chan struct{}),
	}

	request := &v1beta1.ResolutionRequest{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "resolution.tekton.dev/v1beta1",
			Kind:       "ResolutionRequest",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:              "rr",
			Namespace:         "foo",
			CreationTimestamp: metav1.Time{Time: time.Now()},
			Labels: map[string]string{
				resolutioncommon.LabelKeyResolverType: resolutionframework.LabelValueFakeResolverType,
			},
		},
		Spec: v1beta1.ResolutionRequestSpec{
			Params: []pipelinev1.Param{{
				Name:  resolutionframework.FakeParamName,
				Value: *pipelinev1.NewStructuredValues("bar"),
			}},
		},
	}

	d := test.Data{
		ResolutionRequests: []*v1beta1.ResolutionRequest{request},
		ConfigMaps: []*corev1.ConfigMap{{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "resolver-cache-config",
				Namespace: system.Namespace(),
			},
			Data: map[string]string{},
		}},
	}

	ctx, _ := ttesting.SetupFakeContext(t)
	testAssets, cancel := getResolverFrameworkController(ctx, t, d, resolver, setClockOnReconciler)
	defer cancel()
	ignoreCurrent := goleak.IgnoreCurrent()

	// Reconcile must return once the worker outlasts the short timeout, while
	// the worker is still blocked inside Resolve waiting for release.
	err := testAssets.Controller.Reconciler.Reconcile(testAssets.Ctx, getRequestName(request))
	if err == nil || !strings.Contains(err.Error(), "context deadline exceeded") {
		t.Fatalf("expected timeout error, got %v", err)
	}
	<-resolver.started

	// Let the worker finish after the reconciler has already returned. With the
	// buffered channel it deposits its result and exits; unbuffered, it blocks
	// forever on the send and the resolve goroutine never disappears.
	close(resolver.release)
	if err := goleak.Find(ignoreCurrent); err != nil {
		t.Fatalf("resolver worker goroutine did not exit after timeout: %v", err)
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
