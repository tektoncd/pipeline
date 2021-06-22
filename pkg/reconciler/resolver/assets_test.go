package resolver

import (
	"context"
	"testing"

	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	ttesting "github.com/tektoncd/pipeline/pkg/reconciler/testing"
	"github.com/tektoncd/pipeline/test"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ktesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/record"
	cminformer "knative.dev/pkg/configmap/informer"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"
	pkgreconciler "knative.dev/pkg/reconciler"
	"knative.dev/pkg/system"
)

// simpleTask is a reusable Task resource.
var simpleTask = v1beta1.Task{
	TypeMeta: metav1.TypeMeta{
		Kind:       "Task",
		APIVersion: "tekton.dev/v1beta1",
	},
	ObjectMeta: metav1.ObjectMeta{
		Name:        "test-task",
		Namespace:   "default",
		Labels:      map[string]string{"red-balloons": "99"},
		Annotations: map[string]string{"green-balloons": "0"},
	},
	Spec: v1beta1.TaskSpec{
		Steps: []v1beta1.Step{{
			Container: corev1.Container{
				Name:    "step-1",
				Command: []string{"echo"},
				Args:    []string{"hello", "world"},
			},
		}},
	},
}

var simplePipeline = v1beta1.Pipeline{
	TypeMeta: metav1.TypeMeta{
		Kind:       "Pipeline",
		APIVersion: "tekton.dev/v1beta1",
	},
	ObjectMeta: metav1.ObjectMeta{
		Name:        "test-pipeline",
		Namespace:   "default",
		Labels:      map[string]string{"foo": "bar"},
		Annotations: map[string]string{"baz": "quux"},
	},
	Spec: v1beta1.PipelineSpec{
		Tasks: []v1beta1.PipelineTask{{
			TaskRef: &v1beta1.TaskRef{
				Name: simpleTask.Name,
			},
		}},
	},
}

// getPatchActions filters a list of Actions down to just the PatchActions for
// a given resource (e.g. "taskruns" or "pipelineruns").
func getPatchActions(actions []ktesting.Action, resourceKind string) []ktesting.PatchAction {
	ret := []ktesting.PatchAction{}
	for _, a := range actions {
		if action, ok := a.(ktesting.PatchAction); ok {
			if action.Matches("patch", resourceKind) {
				ret = append(ret, action)
			}
		}
	}
	return ret
}

// getClients is a test helper to construct the fake clients
// needed for testing resource patching.
func getClients(t *testing.T, d test.Data) (test.Assets, func()) {
	ctx, _ := ttesting.SetupFakeContext(t)
	ctx, cancel := context.WithCancel(ctx)

	clients, informers := test.SeedTestData(t, ctx, d)

	return test.Assets{
		Ctx:       ctx,
		Clients:   clients,
		Informers: informers,
	}, cancel
}

// getTaskRunResolverController is a helper to return a
// test.Assets object populated with the resolver reconciler
// controller.
func getTaskRunResolverController(t *testing.T, d test.Data) (test.Assets, func()) {
	ctx, _ := ttesting.SetupFakeContext(t)
	ctx, cancel := context.WithCancel(ctx)
	ensureConfigurationConfigMapsExist(&d)
	c, informers := test.SeedTestData(t, ctx, d)
	configMapWatcher := cminformer.NewInformedWatcher(c.Kube, system.Namespace())

	ctl := NewTaskRunResolverController()(ctx, configMapWatcher)
	if err := configMapWatcher.Start(ctx.Done()); err != nil {
		t.Fatalf("error starting configmap watcher: %v", err)
	}

	if la, ok := ctl.Reconciler.(pkgreconciler.LeaderAware); ok {
		la.Promote(pkgreconciler.UniversalBucket(), func(pkgreconciler.Bucket, types.NamespacedName) {})
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

// getPipelineRunResolverController is a helper to return a
// test.Assets object populated with the resolver reconciler
// controller.
func getPipelineRunResolverController(t *testing.T, d test.Data) (test.Assets, func()) {
	ctx, _ := ttesting.SetupFakeContext(t)
	ctx, cancel := context.WithCancel(ctx)
	ensureConfigurationConfigMapsExist(&d)
	c, informers := test.SeedTestData(t, ctx, d)
	configMapWatcher := cminformer.NewInformedWatcher(c.Kube, system.Namespace())

	ctl := NewPipelineRunResolverController()(ctx, configMapWatcher)
	if err := configMapWatcher.Start(ctx.Done()); err != nil {
		t.Fatalf("error starting configmap watcher: %v", err)
	}

	if la, ok := ctl.Reconciler.(pkgreconciler.LeaderAware); ok {
		la.Promote(pkgreconciler.UniversalBucket(), func(pkgreconciler.Bucket, types.NamespacedName) {})
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

// ensureConfigurationConfigMapsExist sets up the expected minimum defaults for
// configmaps passed to a resolver reconciler controller.
func ensureConfigurationConfigMapsExist(d *test.Data) {
	var defaultsExists, featureFlagsExists, artifactBucketExists, artifactPVCExists bool
	for _, cm := range d.ConfigMaps {
		if cm.Name == config.GetDefaultsConfigName() {
			defaultsExists = true
		}
		if cm.Name == config.GetFeatureFlagsConfigName() {
			featureFlagsExists = true
		}
		if cm.Name == config.GetArtifactBucketConfigName() {
			artifactBucketExists = true
		}
		if cm.Name == config.GetArtifactPVCConfigName() {
			artifactPVCExists = true
		}
	}
	if !defaultsExists {
		d.ConfigMaps = append(d.ConfigMaps, &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: config.GetDefaultsConfigName(), Namespace: system.Namespace()},
			Data:       map[string]string{},
		})
	}
	if !featureFlagsExists {
		d.ConfigMaps = append(d.ConfigMaps, &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: config.GetFeatureFlagsConfigName(), Namespace: system.Namespace()},
			Data:       map[string]string{},
		})
	}
	if !artifactBucketExists {
		d.ConfigMaps = append(d.ConfigMaps, &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: config.GetArtifactBucketConfigName(), Namespace: system.Namespace()},
			Data:       map[string]string{},
		})
	}
	if !artifactPVCExists {
		d.ConfigMaps = append(d.ConfigMaps, &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: config.GetArtifactPVCConfigName(), Namespace: system.Namespace()},
			Data:       map[string]string{},
		})
	}
}
