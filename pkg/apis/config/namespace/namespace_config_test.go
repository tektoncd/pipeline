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

package namespace_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/pkg/apis/config"
	namespaceconfig "github.com/tektoncd/pipeline/pkg/apis/config/namespace"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/pod"
	"go.uber.org/zap/zaptest"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	fakek8s "k8s.io/client-go/kubernetes/fake"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/system"
	_ "knative.dev/pkg/system/testing"
)

func newPerNamespaceConfig(t *testing.T, configMaps ...*corev1.ConfigMap) (*namespaceconfig.PerNamespaceConfig, *fakek8s.Clientset, context.Context) {
	t.Helper()
	objects := make([]runtime.Object, len(configMaps))
	for i, configMap := range configMaps {
		objects[i] = configMap
	}
	client := fakek8s.NewSimpleClientset(objects...)
	ctx, cancel := context.WithCancel(logging.WithLogger(t.Context(), zaptest.NewLogger(t).Sugar()))
	t.Cleanup(cancel)
	return namespaceconfig.NewPerNamespaceConfig(ctx, client), client, ctx
}

func labeledConfigMap(namespace, name string, data map[string]string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				"app.kubernetes.io/part-of":  "tekton-pipelines",
				"tekton.dev/pipeline-config": "true",
			},
		},
		Data: data,
	}
}

func enabledConfigContext(ctx context.Context) (context.Context, *config.Config) {
	cfg := &config.Config{
		Defaults:     config.DefaultConfig.DeepCopy(),
		FeatureFlags: config.DefaultFeatureFlags.DeepCopy(),
	}
	cfg.FeatureFlags.PerNamespaceConfiguration = true
	return config.ToContext(ctx, cfg), cfg
}

func TestMergeGlobalConfigWithLocal(t *testing.T) {
	defaults := labeledConfigMap("team-a", "tekton-config-defaults", map[string]string{
		"default-timeout-minutes": "120",
		"default-service-account": "team-a-sa",
		"default-resolver-type":   "ignored",
		"unknown-default":         "ignored",
	})
	flags := labeledConfigMap("team-a", "tekton-feature-flags", map[string]string{
		"enable-cel-in-whenexpression": "true",
		"coschedule":                   "isolate-pipelinerun",
		"set-security-context":         "true",
		"unknown-flag":                 "true",
	})
	perNamespaceConfig, _, baseCtx := newPerNamespaceConfig(t, defaults, flags)
	ctx, cfg := enabledConfigContext(baseCtx)
	cfg.FeatureFlags.NonOverridableFields = " coschedule "
	cfg.FeatureFlags.EnableTerminationMessageCompression = true
	cfg.Defaults.DefaultTaskRunWorkspaceBinding = "emptyDir: {}"
	cfg.Defaults.DefaultForbiddenEnv = []string{"FORBIDDEN_ENV"}
	cfg.Defaults.DefaultPodTemplate = &pod.Template{NodeSelector: map[string]string{"disk": "ssd"}}
	cfg.Defaults.DefaultAAPodTemplate = &pod.AffinityAssistantTemplate{NodeSelector: map[string]string{"zone": "west"}}
	cfg.Defaults.DefaultContainerResourceRequirements = map[string]corev1.ResourceRequirements{
		config.ResourceRequirementDefaultContainerKey: {
			Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("100m")},
		},
	}

	mergedCtx, err := perNamespaceConfig.MergeGlobalConfigWithLocal(ctx, "team-a")
	if err != nil {
		t.Fatal(err)
	}
	merged := config.FromContext(mergedCtx)
	if merged.Defaults.DefaultTimeoutMinutes != 120 || merged.Defaults.DefaultServiceAccount != "team-a-sa" {
		t.Fatalf("namespace defaults were not applied: %#v", merged.Defaults)
	}
	if !merged.FeatureFlags.EnableCELInWhenExpression {
		t.Error("namespace feature flag was not applied")
	}
	if merged.FeatureFlags.Coschedule != cfg.FeatureFlags.Coschedule {
		t.Error("operator-locked field was overridden")
	}
	if merged.FeatureFlags.SetSecurityContext != cfg.FeatureFlags.SetSecurityContext {
		t.Error("blocked security field was overridden")
	}
	if merged.Defaults.DefaultResolverType != cfg.Defaults.DefaultResolverType {
		t.Error("unsupported default was overridden")
	}
	if diff := cmp.Diff(cfg.Defaults.DefaultPodTemplate, merged.Defaults.DefaultPodTemplate); diff != "" {
		t.Errorf("pod template changed (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(cfg.Defaults.DefaultAAPodTemplate, merged.Defaults.DefaultAAPodTemplate); diff != "" {
		t.Errorf("affinity assistant template changed (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(cfg.Defaults.DefaultContainerResourceRequirements, merged.Defaults.DefaultContainerResourceRequirements); diff != "" {
		t.Errorf("container resources changed (-want +got):\n%s", diff)
	}
	if merged.Defaults.DefaultTaskRunWorkspaceBinding != cfg.Defaults.DefaultTaskRunWorkspaceBinding ||
		!cmp.Equal(merged.Defaults.DefaultForbiddenEnv, cfg.Defaults.DefaultForbiddenEnv) {
		t.Error("optional cluster defaults were not preserved")
	}
	if !merged.FeatureFlags.EnableTerminationMessageCompression || !merged.FeatureFlags.PerNamespaceConfiguration || merged.FeatureFlags.NonOverridableFields != " coschedule " {
		t.Error("cluster feature flags were not preserved")
	}
}

func TestMergeGlobalConfigWithLocalNoOp(t *testing.T) {
	valid := labeledConfigMap("team-a", "tekton-config-defaults", map[string]string{"default-timeout-minutes": "120"})
	missingPartOf := valid.DeepCopy()
	missingPartOf.Namespace = "missing-label"
	delete(missingPartOf.Labels, "app.kubernetes.io/part-of")
	missingSelector := valid.DeepCopy()
	missingSelector.Namespace = "missing-selector"
	delete(missingSelector.Labels, "tekton.dev/pipeline-config")
	systemConfig := valid.DeepCopy()
	systemConfig.Namespace = system.Namespace()
	example := labeledConfigMap("example", "tekton-config-defaults", map[string]string{"_example": "ignored"})
	perNamespaceConfig, _, baseCtx := newPerNamespaceConfig(t, valid, missingPartOf, missingSelector, systemConfig, example)

	tests := []struct {
		name      string
		namespace string
		enabled   bool
	}{
		{name: "disabled", namespace: "team-a"},
		{name: "missing config", namespace: "missing", enabled: true},
		{name: "missing part-of label", namespace: "missing-label", enabled: true},
		{name: "missing selector label", namespace: "missing-selector", enabled: true},
		{name: "example key", namespace: "example", enabled: true},
		{name: "system namespace", namespace: system.Namespace(), enabled: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{Defaults: config.DefaultConfig.DeepCopy(), FeatureFlags: config.DefaultFeatureFlags.DeepCopy()}
			cfg.FeatureFlags.PerNamespaceConfiguration = tt.enabled
			ctx := config.ToContext(baseCtx, cfg)
			mergedCtx, err := perNamespaceConfig.MergeGlobalConfigWithLocal(ctx, tt.namespace)
			if err != nil {
				t.Fatal(err)
			}
			if got := config.FromContext(mergedCtx).Defaults.DefaultTimeoutMinutes; got != config.DefaultTimeoutMinutes {
				t.Fatalf("timeout = %d, want %d", got, config.DefaultTimeoutMinutes)
			}
		})
	}

	ctx, _ := enabledConfigContext(baseCtx)
	mergedCtx, err := (&namespaceconfig.PerNamespaceConfig{}).MergeGlobalConfigWithLocal(ctx, "team-a")
	if err != nil {
		t.Fatal(err)
	}
	if mergedCtx != ctx {
		t.Error("zero-value namespace config changed the context")
	}
}

func TestMergeGlobalConfigWithLocalWithNilGlobalDefaults(t *testing.T) {
	defaults := labeledConfigMap("team-a", "tekton-config-defaults", map[string]string{"default-timeout-minutes": "120"})
	perNamespaceConfig, _, baseCtx := newPerNamespaceConfig(t, defaults)
	ctx, cfg := enabledConfigContext(baseCtx)
	cfg.Defaults = nil

	mergedCtx, err := perNamespaceConfig.MergeGlobalConfigWithLocal(ctx, "team-a")
	if err != nil {
		t.Fatal(err)
	}
	if got := config.FromContext(mergedCtx).Defaults.DefaultTimeoutMinutes; got != 120 {
		t.Fatalf("timeout = %d, want 120", got)
	}
}

func TestMergeGlobalConfigWithLocalReturnsParseErrors(t *testing.T) {
	tests := []struct {
		name      string
		configMap *corev1.ConfigMap
	}{
		{
			name: "defaults",
			configMap: labeledConfigMap("team-a", "tekton-config-defaults", map[string]string{
				"default-timeout-minutes": "not-an-int",
			}),
		},
		{
			name: "feature flags",
			configMap: labeledConfigMap("team-a", "tekton-feature-flags", map[string]string{
				"enable-cel-in-whenexpression": "not-a-bool",
			}),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			perNamespaceConfig, _, baseCtx := newPerNamespaceConfig(t, tt.configMap)
			ctx, cfg := enabledConfigContext(baseCtx)
			mergedCtx, err := perNamespaceConfig.MergeGlobalConfigWithLocal(ctx, "team-a")
			if err == nil {
				t.Fatal("expected parse error")
			}
			if config.FromContext(mergedCtx) != cfg {
				t.Error("parse error returned a partially merged config")
			}
		})
	}
}

func TestPerNamespaceConfigTracksUpdatesAndNamespaces(t *testing.T) {
	alpha := labeledConfigMap("team-alpha", "tekton-config-defaults", map[string]string{"default-timeout-minutes": "90"})
	beta := labeledConfigMap("team-beta", "tekton-config-defaults", map[string]string{
		"default-timeout-minutes": "240",
		"default-service-account": "beta-sa",
	})
	perNamespaceConfig, client, baseCtx := newPerNamespaceConfig(t, alpha, beta)

	assertConfig := func(namespace string, timeout int, serviceAccount string) bool {
		ctx, _ := enabledConfigContext(baseCtx)
		mergedCtx, err := perNamespaceConfig.MergeGlobalConfigWithLocal(ctx, namespace)
		if err != nil {
			return false
		}
		merged := config.FromContext(mergedCtx)
		return merged.Defaults.DefaultTimeoutMinutes == timeout && merged.Defaults.DefaultServiceAccount == serviceAccount
	}
	if !assertConfig("team-alpha", 90, config.DefaultServiceAccountValue) || !assertConfig("team-beta", 240, "beta-sa") {
		t.Fatal("namespace configs were not isolated")
	}

	updated := alpha.DeepCopy()
	updated.Data["default-timeout-minutes"] = "200"
	if _, err := client.CoreV1().ConfigMaps("team-alpha").Update(baseCtx, updated, metav1.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for !assertConfig("team-alpha", 200, config.DefaultServiceAccountValue) {
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for ConfigMap update")
		}
		time.Sleep(10 * time.Millisecond)
	}
}
