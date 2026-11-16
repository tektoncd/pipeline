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
	"strconv"
	"testing"
	"testing/synctest"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/pkg/apis/config"
	namespaceconfig "github.com/tektoncd/pipeline/pkg/apis/config/namespace"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/pod"
	th "github.com/tektoncd/pipeline/pkg/reconciler/testing"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	fakek8s "k8s.io/client-go/kubernetes/fake"
	"knative.dev/pkg/injection"
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

func enabledConfigContext(ctx context.Context) (context.Context, *config.Config) {
	cfg := &config.Config{
		Defaults:     config.DefaultConfig.DeepCopy(),
		FeatureFlags: config.DefaultFeatureFlags.DeepCopy(),
	}
	cfg.FeatureFlags.PerNamespaceConfiguration = true
	return config.ToContext(ctx, cfg), cfg
}

func TestNewPerNamespaceConfigSharesContextInstance(t *testing.T) {
	client := fakek8s.NewSimpleClientset()
	ctx, cancel := context.WithCancel(logging.WithLogger(t.Context(), zaptest.NewLogger(t).Sugar()))
	t.Cleanup(cancel)
	ctx = namespaceconfig.WithSharedPerNamespaceConfig(ctx)

	first := namespaceconfig.NewPerNamespaceConfig(ctx, client)
	if second := namespaceconfig.NewPerNamespaceConfig(ctx, client); second != first {
		t.Fatal("NewPerNamespaceConfig returned different instances for a shared context")
	}
}

func TestNewPerNamespaceConfigRespectsNamespaceScope(t *testing.T) {
	client := fakek8s.NewSimpleClientset(
		th.NewNamespaceDefaultsConfigMap("team-a", map[string]string{"default-timeout-minutes": "120"}),
		th.NewNamespaceDefaultsConfigMap("team-b", map[string]string{"default-timeout-minutes": "240"}),
	)
	ctx, cancel := context.WithCancel(logging.WithLogger(t.Context(), zaptest.NewLogger(t).Sugar()))
	t.Cleanup(cancel)
	ctx = injection.WithNamespaceScope(ctx, "team-a")
	ctx, cfg := enabledConfigContext(ctx)

	perNamespaceConfig := namespaceconfig.NewPerNamespaceConfig(ctx, client)

	merged, err := perNamespaceConfig.MergeGlobalConfigWithLocal(ctx, "team-a")
	if err != nil {
		t.Fatal(err)
	}
	if got := config.FromContext(merged).Defaults.DefaultTimeoutMinutes; got != 120 {
		t.Errorf("in-scope namespace: timeout = %d, want 120", got)
	}

	merged, err = perNamespaceConfig.MergeGlobalConfigWithLocal(ctx, "team-b")
	if err != nil {
		t.Fatal(err)
	}
	if got := config.FromContext(merged).Defaults.DefaultTimeoutMinutes; got != cfg.Defaults.DefaultTimeoutMinutes {
		t.Errorf("out-of-scope namespace: timeout = %d, want cluster default %d", got, cfg.Defaults.DefaultTimeoutMinutes)
	}
}

func TestMergeGlobalConfigWithLocalAppliesOverrides(t *testing.T) {
	defaults := th.NewNamespaceDefaultsConfigMap("team-a", map[string]string{
		"default-timeout-minutes": "120",
		"default-service-account": "team-a-sa",
	})
	flags := th.NewNamespaceFeatureFlagsConfigMap("team-a", map[string]string{
		"enable-cel-in-whenexpression": "true",
	})
	perNamespaceConfig, _, baseCtx := newPerNamespaceConfig(t, defaults, flags)
	ctx, cfg := enabledConfigContext(baseCtx)
	cfg.FeatureFlags.EnableCELInWhenExpression = false
	want := cfg.DeepCopy()
	want.Defaults.DefaultTimeoutMinutes = 120
	want.Defaults.DefaultServiceAccount = "team-a-sa"
	want.FeatureFlags.EnableCELInWhenExpression = true

	mergedCtx, err := perNamespaceConfig.MergeGlobalConfigWithLocal(ctx, "team-a")
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(want, config.FromContext(mergedCtx)); diff != "" {
		t.Errorf("namespace overrides (-want +got):\n%s", diff)
	}
}

func TestMergeGlobalConfigWithLocalIgnoresRestrictedFields(t *testing.T) {
	defaults := th.NewNamespaceDefaultsConfigMap("team-a", map[string]string{
		"default-resolver-type": "ignored",
		"unknown-default":       "ignored",
	})
	flags := th.NewNamespaceFeatureFlagsConfigMap("team-a", map[string]string{
		"coschedule":           "isolate-pipelinerun",
		"set-security-context": "true",
		"unknown-flag":         "true",
	})
	perNamespaceConfig, _, baseCtx := newPerNamespaceConfig(t, defaults, flags)
	ctx, cfg := enabledConfigContext(baseCtx)
	cfg.FeatureFlags.NonOverridableFields = " coschedule "
	cfg.FeatureFlags.Coschedule = "workspaces"
	cfg.FeatureFlags.SetSecurityContext = false
	want := cfg.DeepCopy()

	mergedCtx, err := perNamespaceConfig.MergeGlobalConfigWithLocal(ctx, "team-a")
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(want, config.FromContext(mergedCtx)); diff != "" {
		t.Errorf("restricted fields changed config (-want +got):\n%s", diff)
	}
}

func TestMergeGlobalConfigWithLocalPreservesGlobalConfig(t *testing.T) {
	defaults := th.NewNamespaceDefaultsConfigMap("team-a", map[string]string{"default-timeout-minutes": "120"})
	flags := th.NewNamespaceFeatureFlagsConfigMap("team-a", map[string]string{"enable-cel-in-whenexpression": "true"})
	perNamespaceConfig, _, baseCtx := newPerNamespaceConfig(t, defaults, flags)
	ctx, cfg := enabledConfigContext(baseCtx)
	cfg.FeatureFlags.NonOverridableFields = " coschedule "
	cfg.FeatureFlags.EnableTerminationMessageCompression = true
	cfg.FeatureFlags.KeepStatusSpecDescriptions = true
	cfg.FeatureFlags.EnableSurfacePodEvents = true
	cfg.FeatureFlags.EnableCELInWhenExpression = false
	cfg.Defaults.DefaultTaskRunWorkspaceBinding = "emptyDir: {}"
	cfg.Defaults.DefaultForbiddenEnv = []string{"FORBIDDEN_ENV"}
	cfg.Defaults.DefaultPodTemplate = &pod.Template{NodeSelector: map[string]string{"disk": "ssd"}}
	cfg.Defaults.DefaultAAPodTemplate = &pod.AffinityAssistantTemplate{NodeSelector: map[string]string{"zone": "west"}}
	cfg.Defaults.DefaultContainerResourceRequirements = map[string]corev1.ResourceRequirements{
		config.ResourceRequirementDefaultContainerKey: {
			Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("100m")},
		},
	}
	original := cfg.DeepCopy()
	want := cfg.DeepCopy()
	want.Defaults.DefaultTimeoutMinutes = 120
	want.FeatureFlags.EnableCELInWhenExpression = true

	mergedCtx, err := perNamespaceConfig.MergeGlobalConfigWithLocal(ctx, "team-a")
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(want, config.FromContext(mergedCtx)); diff != "" {
		t.Errorf("merged config (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(original, cfg); diff != "" {
		t.Errorf("merge mutated global config (-want +got):\n%s", diff)
	}
}

func TestMergeGlobalConfigWithLocalNoOp(t *testing.T) {
	valid := th.NewNamespaceDefaultsConfigMap("team-a", map[string]string{"default-timeout-minutes": "120"})
	missingPartOf := valid.DeepCopy()
	missingPartOf.Namespace = "missing-label"
	delete(missingPartOf.Labels, "app.kubernetes.io/part-of")
	missingSelector := valid.DeepCopy()
	missingSelector.Namespace = "missing-selector"
	delete(missingSelector.Labels, "tekton.dev/pipeline-config")
	systemConfig := valid.DeepCopy()
	systemConfig.Namespace = system.Namespace()
	kubeSystemConfig := valid.DeepCopy()
	kubeSystemConfig.Namespace = "kube-system"
	kubePublicConfig := valid.DeepCopy()
	kubePublicConfig.Namespace = "kube-public"
	example := th.NewNamespaceDefaultsConfigMap("example", map[string]string{"_example": "ignored"})
	perNamespaceConfig, _, baseCtx := newPerNamespaceConfig(t,
		valid, missingPartOf, missingSelector, systemConfig, kubeSystemConfig, kubePublicConfig, example,
	)

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
		{name: "kube-system namespace", namespace: "kube-system", enabled: true},
		{name: "kube-public namespace", namespace: "kube-public", enabled: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.FromContextOrDefaults(baseCtx).DeepCopy()
			cfg.Defaults.DefaultTimeoutMinutes = 45
			cfg.Defaults.DefaultServiceAccount = "global-sa"
			cfg.FeatureFlags.PerNamespaceConfiguration = tt.enabled
			cfg.FeatureFlags.EnableCELInWhenExpression = true
			want := cfg.DeepCopy()
			ctx := config.ToContext(baseCtx, cfg)
			mergedCtx, err := perNamespaceConfig.MergeGlobalConfigWithLocal(ctx, tt.namespace)
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(want, config.FromContext(mergedCtx)); diff != "" {
				t.Errorf("no-op changed config (-want +got):\n%s", diff)
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
	defaults := th.NewNamespaceDefaultsConfigMap("team-a", map[string]string{"default-timeout-minutes": "120"})
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
			configMap: th.NewNamespaceDefaultsConfigMap("team-a", map[string]string{
				"default-timeout-minutes": "not-an-int",
			}),
		},
		{
			name: "feature flags",
			configMap: th.NewNamespaceFeatureFlagsConfigMap("team-a", map[string]string{
				"enable-cel-in-whenexpression": "not-a-bool",
			}),
		},
		{
			name: "negative timeout",
			configMap: th.NewNamespaceDefaultsConfigMap("team-a", map[string]string{
				"default-timeout-minutes": "-1",
			}),
		},
		{
			name: "overflowing timeout",
			configMap: th.NewNamespaceDefaultsConfigMap("team-a", map[string]string{
				"default-timeout-minutes": strconv.FormatInt(int64((1<<63-1)/time.Minute)+1, 10),
			}),
		},
		{
			name: "malformed workspace binding",
			configMap: th.NewNamespaceDefaultsConfigMap("team-a", map[string]string{
				"default-task-run-workspace-binding": "[",
			}),
		},
		{
			name: "invalid workspace field type",
			configMap: th.NewNamespaceDefaultsConfigMap("team-a", map[string]string{
				"default-task-run-workspace-binding": "emptyDir: []",
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

func TestMergeGlobalConfigWithLocalValidDefaults(t *testing.T) {
	for _, timeout := range []int{0, 120, int((1<<63 - 1) / time.Minute)} {
		t.Run(strconv.Itoa(timeout), func(t *testing.T) {
			defaults := th.NewNamespaceDefaultsConfigMap("team-a", map[string]string{
				"default-timeout-minutes":            strconv.Itoa(timeout),
				"default-task-run-workspace-binding": "emptyDir: {}",
			})
			perNamespaceConfig, _, baseCtx := newPerNamespaceConfig(t, defaults)
			ctx, cfg := enabledConfigContext(baseCtx)
			expected := cfg.DeepCopy()
			expected.Defaults.DefaultTimeoutMinutes = timeout
			expected.Defaults.DefaultTaskRunWorkspaceBinding = "emptyDir: {}"

			merged, err := perNamespaceConfig.MergeGlobalConfigWithLocal(ctx, "team-a")
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(expected, config.FromContext(merged)); diff != "" {
				t.Errorf("merged config (-want +got):\n%s", diff)
			}
		})
	}
}

func TestMergeGlobalConfigWithLocalIgnoresLockedInvalidDefaults(t *testing.T) {
	defaults := th.NewNamespaceDefaultsConfigMap("team-a", map[string]string{
		"default-timeout-minutes":            "-1",
		"default-task-run-workspace-binding": "[",
	})
	perNamespaceConfig, _, baseCtx := newPerNamespaceConfig(t, defaults)
	ctx, cfg := enabledConfigContext(baseCtx)
	cfg.FeatureFlags.NonOverridableFields = "default-timeout-minutes,default-task-run-workspace-binding"

	merged, err := perNamespaceConfig.MergeGlobalConfigWithLocal(ctx, "team-a")
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(cfg, config.FromContext(merged)); diff != "" {
		t.Errorf("locked config changed (-want +got):\n%s", diff)
	}
}

func TestMergeGlobalConfigWithLocalLogsOverridesAtDebugLevel(t *testing.T) {
	defaults := th.NewNamespaceDefaultsConfigMap("team-a", map[string]string{
		"default-timeout-minutes": "120",
		"unknown-default":         "ignored",
	})
	flags := th.NewNamespaceFeatureFlagsConfigMap("team-a", map[string]string{
		"coschedule":           "isolate-pipelinerun",
		"set-security-context": "true",
	})
	perNamespaceConfig, _, baseCtx := newPerNamespaceConfig(t, defaults, flags)

	var entries []zapcore.Entry
	logger := zaptest.NewLogger(t, zaptest.WrapOptions(zap.Hooks(func(entry zapcore.Entry) error {
		entries = append(entries, entry)
		return nil
	}))).Sugar()
	ctx, cfg := enabledConfigContext(logging.WithLogger(baseCtx, logger))
	cfg.FeatureFlags.NonOverridableFields = "coschedule"

	if _, err := perNamespaceConfig.MergeGlobalConfigWithLocal(ctx, "team-a"); err != nil {
		t.Fatal(err)
	}
	if len(entries) == 0 {
		t.Fatal("expected override diagnostics at debug level")
	}
	for _, entry := range entries {
		if entry.Level != zapcore.DebugLevel {
			t.Errorf("override diagnostic level = %v, want debug: %s", entry.Level, entry.Message)
		}
	}
}

func TestMergeGlobalConfigWithLocalLogsOnlyAfterSuccessfulMerge(t *testing.T) {
	defaults := th.NewNamespaceDefaultsConfigMap("team-a", map[string]string{"default-timeout-minutes": "120"})
	flags := th.NewNamespaceFeatureFlagsConfigMap("team-a", map[string]string{"enable-cel-in-whenexpression": "not-a-bool"})
	perNamespaceConfig, _, baseCtx := newPerNamespaceConfig(t, defaults, flags)

	var entries []zapcore.Entry
	logger := zaptest.NewLogger(t, zaptest.WrapOptions(zap.Hooks(func(entry zapcore.Entry) error {
		entries = append(entries, entry)
		return nil
	}))).Sugar()
	ctx, _ := enabledConfigContext(logging.WithLogger(baseCtx, logger))

	if _, err := perNamespaceConfig.MergeGlobalConfigWithLocal(ctx, "team-a"); err == nil {
		t.Fatal("expected parse error")
	}
	if len(entries) != 0 {
		t.Errorf("logged a failed merge: %v", entries)
	}
}

func TestPerNamespaceConfigTracksUpdatesAndNamespaces(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		alpha := th.NewNamespaceDefaultsConfigMap("team-alpha", map[string]string{"default-timeout-minutes": "90"})
		beta := th.NewNamespaceDefaultsConfigMap("team-beta", map[string]string{
			"default-timeout-minutes": "240",
			"default-service-account": "beta-sa",
		})
		perNamespaceConfig, client, baseCtx := newPerNamespaceConfig(t, alpha, beta)

		assertConfig := func(namespace string, timeout int, serviceAccount string) {
			t.Helper()
			ctx, _ := enabledConfigContext(baseCtx)
			mergedCtx, err := perNamespaceConfig.MergeGlobalConfigWithLocal(ctx, namespace)
			if err != nil {
				t.Fatalf("%s config merge: %v", namespace, err)
			}
			merged := config.FromContext(mergedCtx)
			if got := merged.Defaults.DefaultTimeoutMinutes; got != timeout {
				t.Errorf("%s timeout = %d, want %d", namespace, got, timeout)
			}
			if got := merged.Defaults.DefaultServiceAccount; got != serviceAccount {
				t.Errorf("%s service account = %q, want %q", namespace, got, serviceAccount)
			}
		}
		assertConfig("team-alpha", 90, config.DefaultServiceAccountValue)
		assertConfig("team-beta", 240, "beta-sa")

		updated := alpha.DeepCopy()
		updated.Data["default-timeout-minutes"] = "200"
		if _, err := client.CoreV1().ConfigMaps("team-alpha").Update(baseCtx, updated, metav1.UpdateOptions{}); err != nil {
			t.Fatal(err)
		}
		synctest.Wait()
		assertConfig("team-alpha", 200, config.DefaultServiceAccountValue)
		assertConfig("team-beta", 240, "beta-sa")
	})
}
