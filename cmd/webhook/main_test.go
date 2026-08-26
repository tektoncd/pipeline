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
	_ "knative.dev/pkg/system/testing"
)

type errorReadCloser struct{}

func (errorReadCloser) Read([]byte) (int, error) { return 0, errors.New("read failed") }
func (errorReadCloser) Close() error             { return nil }

func TestWithPerNamespaceConfig(t *testing.T) {
	perNamespaceConfig := newTestPerNamespaceConfig(t, &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tekton-feature-flags",
			Namespace: "team-alpha",
			Labels: map[string]string{
				"tekton.dev/pipeline-config": "true",
				"app.kubernetes.io/part-of":  "tekton-pipelines",
			},
		},
		Data: map[string]string{
			defaultconfig.EnableCELInWhenExpression: "true",
		},
	})

	tests := []struct {
		name     string
		request  *http.Request
		override bool
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
		{name: taskRunKind, request: admissionReviewHTTPRequest(t, "team-alpha", taskRunKind, true), override: true},
		{name: pipelineRunKind, request: admissionReviewHTTPRequest(t, "team-alpha", pipelineRunKind, true), override: true},
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

			mergedCtx := withPerNamespaceConfig(ctx, perNamespaceConfig)
			if got := defaultconfig.FromContext(mergedCtx).FeatureFlags.EnableCELInWhenExpression; got != tt.override {
				t.Fatalf("EnableCELInWhenExpression = %t, want %t", got, tt.override)
			}
			if tt.override {
				review := &admissionv1.AdmissionReview{}
				if err := json.NewDecoder(tt.request.Body).Decode(review); err != nil {
					t.Fatalf("AdmissionReview body was not restored: %v", err)
				}
			}
		})
	}
}

func TestWithPerNamespaceConfigParseErrorKeepsGlobalConfig(t *testing.T) {
	perNamespaceConfig := newTestPerNamespaceConfig(t, &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tekton-feature-flags",
			Namespace: "team-alpha",
			Labels: map[string]string{
				"tekton.dev/pipeline-config": "true",
				"app.kubernetes.io/part-of":  "tekton-pipelines",
			},
		},
		Data: map[string]string{defaultconfig.EnableCELInWhenExpression: "invalid"},
	})
	cfg := defaultconfig.FromContextOrDefaults(t.Context()).DeepCopy()
	cfg.FeatureFlags.PerNamespaceConfiguration = true
	ctx := logging.WithLogger(t.Context(), zaptest.NewLogger(t).Sugar())
	ctx = defaultconfig.ToContext(ctx, cfg)
	ctx = apis.WithHTTPRequest(ctx, admissionReviewHTTPRequest(t, "team-alpha", taskRunKind, true))

	mergedCtx := withPerNamespaceConfig(ctx, perNamespaceConfig)
	if defaultconfig.FromContext(mergedCtx) != cfg {
		t.Error("parse error returned a partially merged config")
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
