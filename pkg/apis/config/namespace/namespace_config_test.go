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
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/pkg/apis/config"
	"go.uber.org/zap/zaptest"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	fakek8s "k8s.io/client-go/kubernetes/fake"
)

// newTestCache creates a NamespaceConfigCache backed by a fake informer
// seeded with the given ConfigMaps. The informer is started and synced
// before returning.
func newTestCache(t *testing.T, cms ...*corev1.ConfigMap) *NamespaceConfigCache {
	t.Helper()

	runtimeObjs := make([]runtime.Object, len(cms))
	for i, cm := range cms {
		runtimeObjs[i] = cm
	}

	kubeClient := fakek8s.NewSimpleClientset(runtimeObjs...)

	factory, cmInformer := NewNamespaceConfigInformer(kubeClient, 0)
	stopCh := make(chan struct{})
	t.Cleanup(func() { close(stopCh) })
	factory.Start(stopCh)
	factory.WaitForCacheSync(stopCh)

	return NewNamespaceConfigCache(cmInformer.Lister())
}

func TestMergeConfigMaps(t *testing.T) {
	logger := zaptest.NewLogger(t).Sugar()

	tests := []struct {
		name              string
		globalData        map[string]string
		namespaceData     map[string]string
		overridableFields map[string]bool
		operatorLocked    map[string]bool
		expected          map[string]string
	}{
		{
			name:              "no namespace overrides",
			globalData:        map[string]string{"default-timeout-minutes": "60"},
			namespaceData:     map[string]string{},
			overridableFields: OverridableDefaultsFields,
			expected:          map[string]string{"default-timeout-minutes": "60"},
		},
		{
			name:              "override allowed field",
			globalData:        map[string]string{"default-timeout-minutes": "60", "default-service-account": "default"},
			namespaceData:     map[string]string{"default-timeout-minutes": "120"},
			overridableFields: OverridableDefaultsFields,
			expected:          map[string]string{"default-timeout-minutes": "120", "default-service-account": "default"},
		},
		{
			name:              "block non-overridable field",
			globalData:        map[string]string{"enforce-nonfalsifiability": "none"},
			namespaceData:     map[string]string{"enforce-nonfalsifiability": "spire"},
			overridableFields: OverridableFeatureFlagsFields,
			expected:          map[string]string{"enforce-nonfalsifiability": "none"},
		},
		{
			name:              "block operator-locked field",
			globalData:        map[string]string{"coschedule": "workspaces"},
			namespaceData:     map[string]string{"coschedule": "disabled"},
			overridableFields: OverridableFeatureFlagsFields,
			operatorLocked:    map[string]bool{"coschedule": true},
			expected:          map[string]string{"coschedule": "workspaces"},
		},
		{
			name:              "ignore unknown fields",
			globalData:        map[string]string{"default-timeout-minutes": "60"},
			namespaceData:     map[string]string{"unknown-field": "value"},
			overridableFields: OverridableDefaultsFields,
			expected:          map[string]string{"default-timeout-minutes": "60"},
		},
		{
			name:              "skip _example key",
			globalData:        map[string]string{"default-timeout-minutes": "60"},
			namespaceData:     map[string]string{"_example": "foo", "default-timeout-minutes": "120"},
			overridableFields: OverridableDefaultsFields,
			expected:          map[string]string{"default-timeout-minutes": "120"},
		},
		{
			name: "multiple overrides",
			globalData: map[string]string{
				"default-timeout-minutes": "60",
				"default-service-account": "default",
				"default-resolver-type":   "",
			},
			namespaceData: map[string]string{
				"default-timeout-minutes": "120",
				"default-service-account": "team-sa",
			},
			overridableFields: OverridableDefaultsFields,
			expected: map[string]string{
				"default-timeout-minutes": "120",
				"default-service-account": "team-sa",
				"default-resolver-type":   "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MergeConfigMaps(tt.globalData, tt.namespaceData, tt.overridableFields, tt.operatorLocked, logger)
			if diff := cmp.Diff(tt.expected, got); diff != "" {
				t.Errorf("MergeConfigMaps() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestParseOperatorLockedFields(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected map[string]bool
	}{
		{
			name:     "empty string",
			input:    "",
			expected: nil,
		},
		{
			name:  "single field",
			input: "coschedule",
			expected: map[string]bool{
				"coschedule": true,
			},
		},
		{
			name:  "multiple fields",
			input: "coschedule,enable-step-actions,max-result-size",
			expected: map[string]bool{
				"coschedule":          true,
				"enable-step-actions": true,
				"max-result-size":     true,
			},
		},
		{
			name:  "fields with spaces",
			input: "coschedule , enable-step-actions",
			expected: map[string]bool{
				"coschedule":          true,
				"enable-step-actions": true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseOperatorLockedFields(tt.input)
			if diff := cmp.Diff(tt.expected, got); diff != "" {
				t.Errorf("ParseOperatorLockedFields() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestNamespaceConfigCache(t *testing.T) {
	ctx := context.Background()

	cm := &corev1.ConfigMap{
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
		},
	}

	cache := newTestCache(t, cm)

	// First access (lister lookup)
	nsConfig := cache.Get(ctx, "team-alpha")
	if nsConfig == nil {
		t.Fatal("expected non-nil namespace config")
	}
	if nsConfig.RawFlags["enable-cel-in-whenexpression"] != "true" {
		t.Errorf("expected enable-cel-in-whenexpression=true, got %q", nsConfig.RawFlags["enable-cel-in-whenexpression"])
	}

	// System namespace
	nsConfig3 := cache.Get(ctx, "tekton-pipelines")
	if nsConfig3 != nil {
		t.Error("expected nil for system namespace")
	}

	// Namespace with no config
	nsConfig4 := cache.Get(ctx, "no-config-ns")
	if nsConfig4 != nil {
		t.Error("expected nil for namespace without config")
	}
}

func TestNamespaceConfigCacheInformerUpdate(t *testing.T) {
	ctx := context.Background()

	cm := &corev1.ConfigMap{
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
		},
	}

	kubeClient := fakek8s.NewSimpleClientset(cm)
	factory, cmInformer := NewNamespaceConfigInformer(kubeClient, 0)
	stopCh := make(chan struct{})
	defer close(stopCh)
	factory.Start(stopCh)
	factory.WaitForCacheSync(stopCh)

	cache := NewNamespaceConfigCache(cmInformer.Lister())

	// Verify initial state
	nsConfig := cache.Get(ctx, "team-alpha")
	if nsConfig == nil || nsConfig.RawFlags["enable-cel-in-whenexpression"] != "true" {
		t.Fatal("expected enable-cel-in-whenexpression=true initially")
	}

	// Update the ConfigMap via the client
	cm.Data["enable-cel-in-whenexpression"] = "false"
	if _, err := kubeClient.CoreV1().ConfigMaps("team-alpha").Update(ctx, cm, metav1.UpdateOptions{}); err != nil {
		t.Fatalf("failed to update ConfigMap: %v", err)
	}

	// Wait for informer to sync (with a short timeout)
	deadline := time.After(5 * time.Second)
	for {
		nsConfig = cache.Get(ctx, "team-alpha")
		if nsConfig != nil && nsConfig.RawFlags["enable-cel-in-whenexpression"] == "false" {
			break
		}
		select {
		case <-deadline:
			t.Fatal("timed out waiting for informer to sync updated ConfigMap")
		case <-time.After(100 * time.Millisecond):
			// retry
		}
	}
}

func TestWithNamespaceConfig(t *testing.T) {
	logger := zaptest.NewLogger(t).Sugar()

	cm := &corev1.ConfigMap{
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

	cache := newTestCache(t, cm)

	// Create context with per-namespace-configuration enabled
	cfg := &config.Config{
		Defaults:     config.DefaultConfig.DeepCopy(),
		FeatureFlags: config.DefaultFeatureFlags.DeepCopy(),
	}
	cfg.FeatureFlags.PerNamespaceConfiguration = true
	ctx := config.ToContext(context.Background(), cfg)

	// Apply namespace config
	ctx = WithNamespaceConfig(ctx, cache, "team-alpha", logger)

	// Verify merged config
	mergedCfg := config.FromContext(ctx)
	if mergedCfg == nil {
		t.Fatal("expected non-nil config from context")
	}
	if mergedCfg.Defaults.DefaultTimeoutMinutes != 120 {
		t.Errorf("expected DefaultTimeoutMinutes=120, got %d", mergedCfg.Defaults.DefaultTimeoutMinutes)
	}
	if mergedCfg.Defaults.DefaultServiceAccount != "team-alpha-sa" {
		t.Errorf("expected DefaultServiceAccount=team-alpha-sa, got %q", mergedCfg.Defaults.DefaultServiceAccount)
	}
}

func TestWithNamespaceConfigDisabled(t *testing.T) {
	logger := zaptest.NewLogger(t).Sugar()

	cm := &corev1.ConfigMap{
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

	cache := newTestCache(t, cm)

	// Create context with per-namespace-configuration DISABLED (default)
	cfg := &config.Config{
		Defaults:     config.DefaultConfig.DeepCopy(),
		FeatureFlags: config.DefaultFeatureFlags.DeepCopy(),
	}
	ctx := config.ToContext(context.Background(), cfg)

	// Apply namespace config - should be a no-op
	ctx = WithNamespaceConfig(ctx, cache, "team-alpha", logger)

	mergedCfg := config.FromContext(ctx)
	if mergedCfg.Defaults.DefaultTimeoutMinutes != 60 {
		t.Errorf("expected DefaultTimeoutMinutes=60 (not overridden), got %d", mergedCfg.Defaults.DefaultTimeoutMinutes)
	}
}

func TestWithNamespaceConfigBlocksSecurityFields(t *testing.T) {
	logger := zaptest.NewLogger(t).Sugar()

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      NamespaceFeatureFlagsConfigMapName,
			Namespace: "team-alpha",
			Labels: map[string]string{
				PartOfLabel:          PartOfValue,
				NamespaceConfigLabel: "true",
			},
		},
		Data: map[string]string{
			"enforce-nonfalsifiability":    "spire",
			"set-security-context":         "true",
			"enable-cel-in-whenexpression": "true",
		},
	}

	cache := newTestCache(t, cm)

	cfg := &config.Config{
		Defaults:     config.DefaultConfig.DeepCopy(),
		FeatureFlags: config.DefaultFeatureFlags.DeepCopy(),
	}
	cfg.FeatureFlags.PerNamespaceConfiguration = true
	ctx := config.ToContext(context.Background(), cfg)

	ctx = WithNamespaceConfig(ctx, cache, "team-alpha", logger)

	mergedCfg := config.FromContext(ctx)
	// Security fields should NOT be overridden
	if mergedCfg.FeatureFlags.EnforceNonfalsifiability != "none" {
		t.Errorf("expected enforce-nonfalsifiability=none (not overridden), got %q", mergedCfg.FeatureFlags.EnforceNonfalsifiability)
	}
	if mergedCfg.FeatureFlags.SetSecurityContext != false {
		t.Error("expected set-security-context=false (not overridden)")
	}
	// Allowed field should be overridden
	if mergedCfg.FeatureFlags.EnableCELInWhenExpression != true {
		t.Error("expected enable-cel-in-whenexpression=true (overridden)")
	}
}

func TestConfigMapWithoutLabelsIgnored(t *testing.T) {
	ctx := context.Background()

	// ConfigMap without required labels - won't be picked up by the filtered informer
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      NamespaceFeatureFlagsConfigMapName,
			Namespace: "team-alpha",
			// No labels - informer filter won't include this
		},
		Data: map[string]string{
			"enable-cel-in-whenexpression": "true",
		},
	}

	// The filtered informer only watches CMs with NamespaceConfigLabel=true,
	// so this CM won't appear in the lister even though the fake client has it.
	cache := newTestCache(t, cm)

	nsConfig := cache.Get(ctx, "team-alpha")
	if nsConfig != nil {
		t.Error("expected nil for ConfigMap without required labels")
	}
}

func TestConfigMapWithoutPartOfLabelIgnored(t *testing.T) {
	ctx := context.Background()

	// ConfigMap with pipeline-config label but missing part-of label
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      NamespaceFeatureFlagsConfigMapName,
			Namespace: "team-alpha",
			Labels: map[string]string{
				NamespaceConfigLabel: "true",
				// Missing PartOfLabel
			},
		},
		Data: map[string]string{
			"enable-cel-in-whenexpression": "true",
		},
	}

	cache := newTestCache(t, cm)

	nsConfig := cache.Get(ctx, "team-alpha")
	if nsConfig != nil {
		t.Error("expected nil for ConfigMap without part-of label")
	}
}
