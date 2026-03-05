/*
Copyright 2026 The Tekton Authors

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

package namespace

import (
	"context"
	"testing"

	"github.com/tektoncd/pipeline/pkg/apis/config"
	"go.uber.org/zap/zaptest"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	fakek8s "k8s.io/client-go/kubernetes/fake"
)

// TestIntegrationNamespaceConfigOverrideFlow tests the full end-to-end flow:
// 1. Creates namespace ConfigMaps with proper labels
// 2. Enables per-namespace-configuration in cluster config
// 3. Verifies that WithNamespaceConfig merges namespace overrides
// 4. Verifies cluster-level defaults are preserved for non-overridden fields
// 5. Verifies security fields cannot be overridden
// 6. Verifies operator-locked fields cannot be overridden
func TestIntegrationNamespaceConfigOverrideFlow(t *testing.T) {
	logger := zaptest.NewLogger(t).Sugar()

	// Create namespace ConfigMaps with the required labels
	nsDefaultsCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      NamespaceDefaultsConfigMapName,
			Namespace: "team-alpha",
			Labels: map[string]string{
				PartOfLabel:          PartOfValue,
				NamespaceConfigLabel: "true",
			},
		},
		Data: map[string]string{
			"default-timeout-minutes": "120",
			"default-service-account": "team-alpha-sa",
		},
	}
	nsFlagsCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      NamespaceFeatureFlagsConfigMapName,
			Namespace: "team-alpha",
			Labels: map[string]string{
				PartOfLabel:          PartOfValue,
				NamespaceConfigLabel: "true",
			},
		},
		Data: map[string]string{
			"enable-cel-in-whenexpression": "true",
			"coschedule":                   "isolate-pipelinerun",
			// Attempt to override security fields (should be blocked)
			"enforce-nonfalsifiability": "spire",
			"set-security-context":      "true",
		},
	}

	cache := newTestCache(t, nsDefaultsCM, nsFlagsCM)

	// Create cluster-level config with per-namespace-configuration enabled
	// and operator-locked fields
	cfg := &config.Config{
		Defaults:     config.DefaultConfig.DeepCopy(),
		FeatureFlags: config.DefaultFeatureFlags.DeepCopy(),
	}
	cfg.FeatureFlags.PerNamespaceConfiguration = true
	cfg.FeatureFlags.NonOverridableFields = "coschedule" // operator locks coschedule
	ctx := config.ToContext(context.Background(), cfg)

	// Apply namespace config
	ctx = WithNamespaceConfig(ctx, cache, "team-alpha", logger)

	// Verify the merged config
	mergedCfg := config.FromContext(ctx)
	if mergedCfg == nil {
		t.Fatal("expected non-nil config from context")
	}

	// 1. Overridden defaults fields should reflect namespace values
	if mergedCfg.Defaults.DefaultTimeoutMinutes != 120 {
		t.Errorf("expected DefaultTimeoutMinutes=120, got %d", mergedCfg.Defaults.DefaultTimeoutMinutes)
	}
	if mergedCfg.Defaults.DefaultServiceAccount != "team-alpha-sa" {
		t.Errorf("expected DefaultServiceAccount=team-alpha-sa, got %q", mergedCfg.Defaults.DefaultServiceAccount)
	}

	// 2. Non-overridden defaults should keep cluster values
	if mergedCfg.Defaults.DefaultManagedByLabelValue != "tekton-pipelines" {
		t.Errorf("expected DefaultManagedByLabelValue=tekton-pipelines (cluster default), got %q", mergedCfg.Defaults.DefaultManagedByLabelValue)
	}

	// 3. Overridable feature flag should be applied
	if !mergedCfg.FeatureFlags.EnableCELInWhenExpression {
		t.Error("expected EnableCELInWhenExpression=true (namespace override)")
	}

	// 4. Security fields should NOT be overridden
	if mergedCfg.FeatureFlags.EnforceNonfalsifiability != "none" {
		t.Errorf("expected EnforceNonfalsifiability=none (security field not overridable), got %q", mergedCfg.FeatureFlags.EnforceNonfalsifiability)
	}
	if mergedCfg.FeatureFlags.SetSecurityContext != false {
		t.Error("expected SetSecurityContext=false (security field not overridable)")
	}

	// 5. Operator-locked field should NOT be overridden
	if mergedCfg.FeatureFlags.Coschedule != config.DefaultCoschedule {
		t.Errorf("expected Coschedule=%q (operator-locked), got %q", config.DefaultCoschedule, mergedCfg.FeatureFlags.Coschedule)
	}

	// 6. per-namespace-configuration meta-fields preserved from cluster config
	if !mergedCfg.FeatureFlags.PerNamespaceConfiguration {
		t.Error("expected PerNamespaceConfiguration=true (preserved from cluster)")
	}
	if mergedCfg.FeatureFlags.NonOverridableFields != "coschedule" {
		t.Errorf("expected NonOverridableFields=coschedule (preserved from cluster), got %q", mergedCfg.FeatureFlags.NonOverridableFields)
	}
}

// TestIntegrationNamespaceConfigContextKey tests that the namespace config cache
// can be stored and retrieved from context, simulating the webhook flow.
func TestIntegrationNamespaceConfigContextKey(t *testing.T) {
	logger := zaptest.NewLogger(t).Sugar()

	nsDefaultsCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      NamespaceDefaultsConfigMapName,
			Namespace: "team-beta",
			Labels: map[string]string{
				PartOfLabel:          PartOfValue,
				NamespaceConfigLabel: "true",
			},
		},
		Data: map[string]string{
			"default-timeout-minutes": "180",
		},
	}

	cache := newTestCache(t, nsDefaultsCM)

	// Simulate webhook flow: store cache in context
	cfg := &config.Config{
		Defaults:     config.DefaultConfig.DeepCopy(),
		FeatureFlags: config.DefaultFeatureFlags.DeepCopy(),
	}
	cfg.FeatureFlags.PerNamespaceConfiguration = true
	ctx := config.ToContext(context.Background(), cfg)
	ctx = ToContext(ctx, cache)

	// Retrieve from context (as SetDefaults would do)
	retrievedCache := FromContext(ctx)
	if retrievedCache == nil {
		t.Fatal("expected non-nil cache from context")
	}

	// Use it to merge namespace config
	ctx = WithNamespaceConfig(ctx, retrievedCache, "team-beta", logger)
	mergedCfg := config.FromContext(ctx)
	if mergedCfg.Defaults.DefaultTimeoutMinutes != 180 {
		t.Errorf("expected DefaultTimeoutMinutes=180, got %d", mergedCfg.Defaults.DefaultTimeoutMinutes)
	}
}

// TestIntegrationCacheInvalidation tests that the informer-backed cache
// automatically reflects ConfigMap updates without manual invalidation.
func TestIntegrationCacheInvalidation(t *testing.T) {
	logger := zaptest.NewLogger(t).Sugar()
	ctx := context.Background()

	nsDefaultsCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      NamespaceDefaultsConfigMapName,
			Namespace: "team-gamma",
			Labels: map[string]string{
				PartOfLabel:          PartOfValue,
				NamespaceConfigLabel: "true",
			},
		},
		Data: map[string]string{
			"default-timeout-minutes": "90",
		},
	}

	kubeClient := fakek8s.NewSimpleClientset(nsDefaultsCM)
	factory, cmInformer := NewNamespaceConfigInformer(kubeClient, 0)
	stopCh := make(chan struct{})
	defer close(stopCh)
	factory.Start(stopCh)
	factory.WaitForCacheSync(stopCh)

	cache := NewNamespaceConfigCache(cmInformer.Lister())

	// First access - reads from informer cache
	cfg := &config.Config{
		Defaults:     config.DefaultConfig.DeepCopy(),
		FeatureFlags: config.DefaultFeatureFlags.DeepCopy(),
	}
	cfg.FeatureFlags.PerNamespaceConfiguration = true
	cfgCtx := config.ToContext(ctx, cfg)

	cfgCtx = WithNamespaceConfig(cfgCtx, cache, "team-gamma", logger)
	mergedCfg := config.FromContext(cfgCtx)
	if mergedCfg.Defaults.DefaultTimeoutMinutes != 90 {
		t.Errorf("expected DefaultTimeoutMinutes=90, got %d", mergedCfg.Defaults.DefaultTimeoutMinutes)
	}

	// Update the ConfigMap and sync the informer store directly
	// (fake clients don't auto-trigger informer events)
	updatedCM := nsDefaultsCM.DeepCopy()
	updatedCM.Data["default-timeout-minutes"] = "200"
	cmInformer.Informer().GetStore().Update(updatedCM)

	// Re-read from cache (informer store is now updated)
	cfgCtx = config.ToContext(ctx, cfg)
	cfgCtx = WithNamespaceConfig(cfgCtx, cache, "team-gamma", logger)
	mergedCfg = config.FromContext(cfgCtx)
	if mergedCfg.Defaults.DefaultTimeoutMinutes != 200 {
		t.Fatalf("expected DefaultTimeoutMinutes=200 after update, got %d", mergedCfg.Defaults.DefaultTimeoutMinutes)
	}
}

// TestIntegrationMultipleNamespaces tests that different namespaces can have
// independent configuration overrides.
func TestIntegrationMultipleNamespaces(t *testing.T) {
	logger := zaptest.NewLogger(t).Sugar()

	nsAlphaCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      NamespaceDefaultsConfigMapName,
			Namespace: "team-alpha",
			Labels: map[string]string{
				PartOfLabel:          PartOfValue,
				NamespaceConfigLabel: "true",
			},
		},
		Data: map[string]string{
			"default-timeout-minutes": "120",
		},
	}
	nsBetaCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      NamespaceDefaultsConfigMapName,
			Namespace: "team-beta",
			Labels: map[string]string{
				PartOfLabel:          PartOfValue,
				NamespaceConfigLabel: "true",
			},
		},
		Data: map[string]string{
			"default-timeout-minutes": "240",
			"default-service-account": "team-beta-sa",
		},
	}

	cache := newTestCache(t, nsAlphaCM, nsBetaCM)

	cfg := &config.Config{
		Defaults:     config.DefaultConfig.DeepCopy(),
		FeatureFlags: config.DefaultFeatureFlags.DeepCopy(),
	}
	cfg.FeatureFlags.PerNamespaceConfiguration = true

	// team-alpha namespace
	ctxAlpha := config.ToContext(context.Background(), cfg)
	ctxAlpha = WithNamespaceConfig(ctxAlpha, cache, "team-alpha", logger)
	alphaConfig := config.FromContext(ctxAlpha)

	if alphaConfig.Defaults.DefaultTimeoutMinutes != 120 {
		t.Errorf("team-alpha: expected timeout=120, got %d", alphaConfig.Defaults.DefaultTimeoutMinutes)
	}
	if alphaConfig.Defaults.DefaultServiceAccount != "default" {
		t.Errorf("team-alpha: expected sa=default, got %q", alphaConfig.Defaults.DefaultServiceAccount)
	}

	// team-beta namespace
	ctxBeta := config.ToContext(context.Background(), cfg)
	ctxBeta = WithNamespaceConfig(ctxBeta, cache, "team-beta", logger)
	betaConfig := config.FromContext(ctxBeta)

	if betaConfig.Defaults.DefaultTimeoutMinutes != 240 {
		t.Errorf("team-beta: expected timeout=240, got %d", betaConfig.Defaults.DefaultTimeoutMinutes)
	}
	if betaConfig.Defaults.DefaultServiceAccount != "team-beta-sa" {
		t.Errorf("team-beta: expected sa=team-beta-sa, got %q", betaConfig.Defaults.DefaultServiceAccount)
	}

	// Namespace without config (should get cluster defaults)
	ctxGamma := config.ToContext(context.Background(), cfg)
	ctxGamma = WithNamespaceConfig(ctxGamma, cache, "team-gamma", logger)
	gammaConfig := config.FromContext(ctxGamma)

	if gammaConfig.Defaults.DefaultTimeoutMinutes != 60 {
		t.Errorf("team-gamma: expected timeout=60 (cluster default), got %d", gammaConfig.Defaults.DefaultTimeoutMinutes)
	}
}
