/*
Copyright 2026 The Tekton Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    https://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	defaultconfig "github.com/tektoncd/pipeline/pkg/apis/config"
	nsconfig "github.com/tektoncd/pipeline/pkg/apis/config/namespace"
	reconciletesting "github.com/tektoncd/pipeline/pkg/reconciler/testing"
	"go.uber.org/zap/zaptest"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/informers"
	fakek8s "k8s.io/client-go/kubernetes/fake"
	"knative.dev/pkg/apis"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	mwhinformer "knative.dev/pkg/client/injection/kube/informers/admissionregistration/v1/mutatingwebhookconfiguration"
	vwhinformer "knative.dev/pkg/client/injection/kube/informers/admissionregistration/v1/validatingwebhookconfiguration"
	"knative.dev/pkg/configmap"
	cminformer "knative.dev/pkg/configmap/informer"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/injection"
	secretinformer "knative.dev/pkg/injection/clients/namespacedkube/informers/core/v1/secret"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/system"
	_ "knative.dev/pkg/system/testing"
	"knative.dev/pkg/webhook"
)

type errorReadCloser struct{}

func (errorReadCloser) Read([]byte) (int, error) { return 0, errors.New("read failed") }
func (errorReadCloser) Close() error             { return nil }

func TestAdmissionControllersWatchNamespaceConfigClusterWide(t *testing.T) {
	for name, constructor := range map[string]func(context.Context, configmap.Watcher) *controller.Impl{
		"defaulting": newDefaultingAdmissionController("defaulting"),
		"validation": newValidationAdmissionController("validation"),
	} {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(logging.WithLogger(t.Context(), zaptest.NewLogger(t).Sugar()))
			t.Cleanup(cancel)
			ctx = injection.WithNamespaceScope(ctx, system.Namespace())
			ctx = nsconfig.WithSharedPerNamespaceConfig(ctx)
			ctx = webhook.WithOptions(ctx, webhook.Options{SecretName: "webhook-certs"})
			client := fakek8s.NewSimpleClientset(reconciletesting.NewNamespaceDefaultsConfigMap("team-alpha", map[string]string{
				"default-timeout-minutes": "120",
			}))
			factory := informers.NewSharedInformerFactory(client, 0)
			ctx = context.WithValue(ctx, kubeclient.Key{}, client)
			ctx = context.WithValue(ctx, mwhinformer.Key{}, factory.Admissionregistration().V1().MutatingWebhookConfigurations())
			ctx = context.WithValue(ctx, vwhinformer.Key{}, factory.Admissionregistration().V1().ValidatingWebhookConfigurations())
			ctx = context.WithValue(ctx, secretinformer.Key{}, factory.Core().V1().Secrets())

			impl := constructor(ctx, cminformer.NewInformedWatcher(client, system.Namespace()))
			t.Cleanup(impl.WorkQueue().ShutDown)

			// The cache initialized by admission must include namespaces outside the webhook's own namespace.
			cfg := defaultconfig.FromContextOrDefaults(ctx).DeepCopy()
			cfg.FeatureFlags.PerNamespaceConfiguration = true
			merged, err := nsconfig.NewPerNamespaceConfig(ctx, client).MergeGlobalConfigWithLocal(defaultconfig.ToContext(ctx, cfg), "team-alpha")
			if err != nil {
				t.Fatal(err)
			}
			if got := defaultconfig.FromContext(merged).Defaults.DefaultTimeoutMinutes; got != 120 {
				t.Errorf("admission namespace timeout = %d, want 120", got)
			}
			if got := injection.GetNamespaceScope(ctx); got != system.Namespace() {
				t.Errorf("webhook informer scope = %q, want %q", got, system.Namespace())
			}
		})
	}
}

func TestWithPerNamespaceConfig(t *testing.T) {
	perNamespaceConfig := newTestPerNamespaceConfig(t,
		reconciletesting.NewNamespaceDefaultsConfigMap("team-alpha", map[string]string{
			"default-timeout-minutes": "120",
		}),
		reconciletesting.NewNamespaceFeatureFlagsConfigMap("team-alpha", map[string]string{
			defaultconfig.EnableCELInWhenExpression: "true",
		}),
	)

	for _, kind := range []string{taskRunKind, pipelineRunKind} {
		t.Run(kind, func(t *testing.T) {
			cfg := defaultconfig.FromContextOrDefaults(t.Context()).DeepCopy()
			cfg.FeatureFlags.PerNamespaceConfiguration = true
			cfg.FeatureFlags.EnableCELInWhenExpression = false
			want := cfg.DeepCopy()
			want.Defaults.DefaultTimeoutMinutes = 120
			want.FeatureFlags.EnableCELInWhenExpression = true
			request := admissionReviewHTTPRequest(t, "team-alpha", kind, true)
			body, err := io.ReadAll(request.Body)
			if err != nil {
				t.Fatal(err)
			}
			request.Body = io.NopCloser(bytes.NewReader(body))
			ctx := logging.WithLogger(t.Context(), zaptest.NewLogger(t).Sugar())
			ctx = defaultconfig.ToContext(ctx, cfg)
			ctx = apis.WithHTTPRequest(ctx, request)

			mergedCtx, err := withPerNamespaceConfig(ctx, perNamespaceConfig)
			if err != nil {
				t.Fatalf("withPerNamespaceConfig() error = %v", err)
			}
			if diff := cmp.Diff(want, defaultconfig.FromContext(mergedCtx)); diff != "" {
				t.Errorf("config mismatch (-want +got):\n%s", diff)
			}
			restored, err := io.ReadAll(request.Body)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(body, restored) {
				t.Errorf("AdmissionReview body = %q, want %q", restored, body)
			}
		})
	}
}

func TestWithPerNamespaceConfigNoOp(t *testing.T) {
	perNamespaceConfig := newTestPerNamespaceConfig(t,
		reconciletesting.NewNamespaceFeatureFlagsConfigMap("team-alpha", map[string]string{
			defaultconfig.EnableCELInWhenExpression: "true",
		}),
	)

	tests := []struct {
		name    string
		request *http.Request
	}{
		{name: "missing request"},
		{name: "missing body", request: &http.Request{}},
		{name: "body read error", request: &http.Request{Body: errorReadCloser{}}},
		{name: "invalid JSON", request: httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", bytes.NewBufferString("{"))},
		{name: "missing admission request", request: admissionReviewHTTPRequest(t, "", "", false)},
		{name: "missing namespace", request: admissionReviewHTTPRequest(t, "", taskRunKind, true)},
		{name: "Task", request: admissionReviewHTTPRequest(t, "team-alpha", "Task", true)},
		{name: "Pipeline", request: admissionReviewHTTPRequest(t, "team-alpha", "Pipeline", true)},
		{name: "CustomRun", request: admissionReviewHTTPRequest(t, "team-alpha", "CustomRun", true)},
		{name: "StepAction", request: admissionReviewHTTPRequest(t, "team-alpha", "StepAction", true)},
		{name: "ResolutionRequest", request: admissionReviewHTTPRequest(t, "team-alpha", "ResolutionRequest", true)},
		{name: "no namespace config", request: admissionReviewHTTPRequest(t, "team-beta", taskRunKind, true)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := defaultconfig.FromContextOrDefaults(t.Context()).DeepCopy()
			cfg.FeatureFlags.PerNamespaceConfiguration = true
			cfg.FeatureFlags.EnableCELInWhenExpression = false
			ctx := logging.WithLogger(t.Context(), zaptest.NewLogger(t).Sugar())
			ctx = defaultconfig.ToContext(ctx, cfg)
			if tt.request != nil {
				ctx = apis.WithHTTPRequest(ctx, tt.request)
			}

			mergedCtx, err := withPerNamespaceConfig(ctx, perNamespaceConfig)
			if err != nil {
				t.Fatalf("withPerNamespaceConfig() error = %v", err)
			}
			if defaultconfig.FromContext(mergedCtx) != cfg {
				t.Error("no-op replaced the original config")
			}
		})
	}
}

func TestWithPerNamespaceConfigError(t *testing.T) {
	tests := []struct {
		name      string
		defaults  map[string]string
		flags     map[string]string
		wantError string
	}{
		{
			name:      "invalid timeout",
			defaults:  map[string]string{"default-timeout-minutes": "not-an-int"},
			wantError: `parse namespace defaults for "team-alpha"`,
		},
		{
			name:      "negative timeout",
			defaults:  map[string]string{"default-timeout-minutes": "-1"},
			wantError: `parse namespace defaults for "team-alpha"`,
		},
		{
			name:      "malformed workspace",
			defaults:  map[string]string{"default-task-run-workspace-binding": "["},
			wantError: `parse namespace defaults for "team-alpha"`,
		},
		{
			name:      "invalid feature flag",
			defaults:  map[string]string{"default-timeout-minutes": "120"},
			flags:     map[string]string{defaultconfig.EnableCELInWhenExpression: "invalid"},
			wantError: `parse namespace feature flags for "team-alpha"`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			perNamespaceConfig := newTestPerNamespaceConfig(t,
				reconciletesting.NewNamespaceDefaultsConfigMap("team-alpha", tt.defaults),
				reconciletesting.NewNamespaceFeatureFlagsConfigMap("team-alpha", tt.flags),
			)
			cfg := defaultconfig.FromContextOrDefaults(t.Context()).DeepCopy()
			cfg.FeatureFlags.PerNamespaceConfiguration = true
			want := cfg.DeepCopy()
			ctx := logging.WithLogger(t.Context(), zaptest.NewLogger(t).Sugar())
			ctx = defaultconfig.ToContext(ctx, cfg)
			ctx = apis.WithHTTPRequest(ctx, admissionReviewHTTPRequest(t, "team-alpha", taskRunKind, true))

			mergedCtx, err := withPerNamespaceConfig(ctx, perNamespaceConfig)
			if err == nil {
				t.Fatal("expected namespace config parse error")
			}
			if !strings.Contains(err.Error(), tt.wantError) {
				t.Errorf("error = %v, want %q to prove the namespace fixture was found", err, tt.wantError)
			}
			if defaultconfig.FromContext(mergedCtx) != cfg {
				t.Error("parse error replaced the original config")
			}
			if diff := cmp.Diff(want, defaultconfig.FromContext(mergedCtx)); diff != "" {
				t.Errorf("parse error changed config (-want +got):\n%s", diff)
			}
		})
	}
}

func admissionReviewHTTPRequest(t *testing.T, namespace, kind string, includeRequest bool) *http.Request {
	t.Helper()
	review := &admissionv1.AdmissionReview{}
	if includeRequest {
		review.Request = &admissionv1.AdmissionRequest{
			Namespace: namespace,
			Kind: metav1.GroupVersionKind{
				Group:   "tekton.dev",
				Version: "v1",
				Kind:    kind,
			},
		}
	}
	body, err := json.Marshal(review)
	if err != nil {
		t.Fatal(err)
	}
	return httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", bytes.NewReader(body))
}

func newTestPerNamespaceConfig(t *testing.T, configMaps ...*corev1.ConfigMap) *nsconfig.PerNamespaceConfig {
	t.Helper()
	objects := make([]runtime.Object, len(configMaps))
	for i, configMap := range configMaps {
		objects[i] = configMap
	}
	client := fakek8s.NewSimpleClientset(objects...)
	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)
	return nsconfig.NewPerNamespaceConfig(ctx, client)
}
