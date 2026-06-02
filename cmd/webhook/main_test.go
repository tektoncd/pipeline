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
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	defaultconfig "github.com/tektoncd/pipeline/pkg/apis/config"
	nsconfig "github.com/tektoncd/pipeline/pkg/apis/config/namespace"
	"go.uber.org/zap/zaptest"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	fakek8s "k8s.io/client-go/kubernetes/fake"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/logging"
)

func TestAdmissionRequestNamespaceFromContext(t *testing.T) {
	req := admissionReviewHTTPRequest(t, "team-alpha")
	ctx := apis.WithHTTPRequest(t.Context(), req)

	if got, want := admissionRequestNamespaceFromContext(ctx), "team-alpha"; got != want {
		t.Fatalf("admissionRequestNamespaceFromContext() = %q, want %q", got, want)
	}

	// The helper must restore the body so the underlying admission controller can
	// still read it after the context hook runs.
	review := &admissionv1.AdmissionReview{}
	if err := json.NewDecoder(req.Body).Decode(review); err != nil {
		t.Fatalf("AdmissionReview body was not restored: %v", err)
	}
	if got, want := review.Request.Namespace, "team-alpha"; got != want {
		t.Fatalf("restored AdmissionReview namespace = %q, want %q", got, want)
	}
}

func TestWithNamespaceConfigForAdmissionAppliesValidationFeatureFlagOverrides(t *testing.T) {
	cache := newTestNamespaceConfigCache(t, &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nsconfig.NamespaceFeatureFlagsConfigMapName,
			Namespace: "team-alpha",
			Labels: map[string]string{
				nsconfig.NamespaceConfigLabel: "true",
				nsconfig.PartOfLabel:          nsconfig.PartOfValue,
			},
		},
		Data: map[string]string{
			defaultconfig.EnableCELInWhenExpression: "true",
		},
	})

	cfg := defaultconfig.FromContextOrDefaults(t.Context()).DeepCopy()
	cfg.FeatureFlags.PerNamespaceConfiguration = true
	cfg.FeatureFlags.EnableCELInWhenExpression = false

	ctx := logging.WithLogger(t.Context(), zaptest.NewLogger(t).Sugar())
	ctx = defaultconfig.ToContext(ctx, cfg)
	ctx = apis.WithHTTPRequest(ctx, admissionReviewHTTPRequest(t, "team-alpha"))

	ctx = withNamespaceConfigForAdmission(ctx, cache)

	got := defaultconfig.FromContext(ctx).FeatureFlags
	if !got.EnableCELInWhenExpression {
		t.Fatalf("EnableCELInWhenExpression = false, want namespace override to enable it")
	}
}

func TestConfigValidationConstructorsValidateNamespaceConfigMaps(t *testing.T) {
	constructors := configValidationConstructors()

	defaultsConstructor, ok := constructors[nsconfig.NamespaceDefaultsConfigMapName].(func(*corev1.ConfigMap) (*defaultconfig.Defaults, error))
	if !ok {
		t.Fatalf("missing namespace defaults constructor")
	}
	if _, err := defaultsConstructor(&corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: nsconfig.NamespaceDefaultsConfigMapName},
		Data: map[string]string{
			"default-timeout-minutes": "not-an-int",
		},
	}); err == nil {
		t.Fatalf("expected invalid namespace defaults ConfigMap to fail validation")
	}

	featureFlagsConstructor, ok := constructors[nsconfig.NamespaceFeatureFlagsConfigMapName].(func(*corev1.ConfigMap) (*defaultconfig.FeatureFlags, error))
	if !ok {
		t.Fatalf("missing namespace feature flags constructor")
	}
	if _, err := featureFlagsConstructor(&corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: nsconfig.NamespaceFeatureFlagsConfigMapName},
		Data: map[string]string{
			defaultconfig.EnableCELInWhenExpression: "not-a-bool",
		},
	}); err == nil {
		t.Fatalf("expected invalid namespace feature flags ConfigMap to fail validation")
	}
}

func admissionReviewHTTPRequest(t *testing.T, namespace string) *http.Request {
	t.Helper()
	review := &admissionv1.AdmissionReview{
		Request: &admissionv1.AdmissionRequest{
			Namespace: namespace,
		},
	}
	body, err := json.Marshal(review)
	if err != nil {
		t.Fatalf("Marshal AdmissionReview: %v", err)
	}
	return httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", bytes.NewReader(body))
}

func newTestNamespaceConfigCache(t *testing.T, cms ...*corev1.ConfigMap) *nsconfig.NamespaceConfigCache {
	t.Helper()

	objects := make([]runtime.Object, len(cms))
	for i, cm := range cms {
		objects[i] = cm
	}

	kubeClient := fakek8s.NewSimpleClientset(objects...)
	factory, cmInformer := nsconfig.NewNamespaceConfigInformer(kubeClient, 0)
	stopCh := make(chan struct{})
	t.Cleanup(func() { close(stopCh) })
	factory.Start(stopCh)
	factory.WaitForCacheSync(stopCh)

	return nsconfig.NewNamespaceConfigCache(cmInformer.Lister())
}
