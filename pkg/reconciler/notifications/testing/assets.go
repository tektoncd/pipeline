/*
Copyright 2023 The Tekton Authors

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

package testing

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	bc "github.com/allegro/bigcache/v3"
	"github.com/tektoncd/pipeline/pkg/apis/config"
	fakepipelineclientset "github.com/tektoncd/pipeline/pkg/client/clientset/versioned/fake"
	"github.com/tektoncd/pipeline/pkg/reconciler/events/cloudevent"
	rtesting "github.com/tektoncd/pipeline/pkg/reconciler/testing"
	"github.com/tektoncd/pipeline/test"
	"github.com/tektoncd/pipeline/test/names"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	k8stesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"
)

// Reconciler implements the Reconciler interface from the notification package
type FakeReconciler struct {
	CloudEventClient cloudevent.CEClient
	CacheClient      *bc.BigCache
}

func (c *FakeReconciler) GetCloudEventsClient() cloudevent.CEClient {
	return c.CloudEventClient
}

func (c *FakeReconciler) GetCacheClient() *bc.BigCache {
	return c.CacheClient
}

func configFromConfigMap(d test.Data) config.Config {
	testConfig := config.Config{}
	for _, cm := range d.ConfigMaps {
		switch cm.Name {
		case config.GetDefaultsConfigName():
			testConfig.Defaults, _ = config.NewDefaultsFromConfigMap(cm)
		case config.GetFeatureFlagsConfigName():
			testConfig.FeatureFlags, _ = config.NewFeatureFlagsFromConfigMap(cm)
		case config.GetEventsConfigName():
			testConfig.Events, _ = config.NewEventsFromConfigMap(cm)
		}
	}
	return testConfig
}

// InitializeTestAssets sets up test assets to be used for direct testing
// of the ReconcileKind method (i.e. with no controller object)
// Config maps are loaded into the context and no config map watcher is setup
//
// Example usage:
//
//	testAssets, cancel := rtesting.InitializeTestAssets(t, &d)
//	defer cancel()
//	ceClient, cacheClient := controller.EventClientsFromContext(testAssets.Ctx)
//	reconciler := &rtesting.FakeReconciler{
//		CloudEventClient: ceClient,
//		CacheClient:      cacheClient,
//	}
//	notifications.ReconcileRunObject(testAssets.Ctx, reconciler, readOnlyRun)
func InitializeTestAssets(t *testing.T, d *test.Data) (test.Assets, func()) {
	t.Helper()
	names.TestingSeed()
	ctx, _ := rtesting.SetupFakeContext(t)
	ctx = rtesting.SetupFakeCloudClientContext(ctx, d.ExpectedCloudEventCount)
	ctx, cancel := context.WithCancel(ctx)
	// Ensure all cm exists before seeding the data
	test.EnsureConfigurationConfigMapsExist(d)
	c, informers := test.SeedTestData(t, ctx, *d)
	testConfig := configFromConfigMap(*d)
	ctx = config.ToContext(ctx, &testConfig)

	return test.Assets{
		Logger:    logging.FromContext(ctx),
		Clients:   c,
		Informers: informers,
		Recorder:  controller.GetEventRecorder(ctx).(*record.FakeRecorder),
		Ctx:       ctx,
	}, cancel
}

func GetTestResourceName(run metav1.ObjectMetaAccessor) string {
	return strings.Join([]string{run.GetObjectMeta().GetNamespace(), run.GetObjectMeta().GetName()}, "/")
}

// RunInformer starts the informer for resource and waits until both its
// initial list has synced and its watch is registered with the fake
// clientset, so that later writes through the clientset reach the informer
// as watch events, as they would from a real API server.
//
// The fake informers set up by InitializeTestAssets are otherwise never
// started, and test.SeedTestData installs reactors that write objects
// directly into the informer's indexer; those are bypassed for resource so
// that the informer sees genuine add and update notifications.
func RunInformer(t *testing.T, ctx context.Context, client *fakepipelineclientset.Clientset, resource string, informer cache.SharedIndexInformer) {
	t.Helper()
	client.PrependReactor("*", resource, k8stesting.ObjectReaction(client.Tracker()))
	watching := make(chan struct{})
	var once sync.Once
	client.PrependWatchReactor(resource, func(action k8stesting.Action) (bool, watch.Interface, error) {
		w, err := client.Tracker().Watch(action.GetResource(), action.GetNamespace())
		if err != nil {
			return true, nil, err
		}
		once.Do(func() { close(watching) })
		return true, w, nil
	})
	ctx, cancel := context.WithCancel(ctx)
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		informer.Run(ctx.Done())
	}()
	// Stop the informer before the test completes, so that its goroutines
	// do not log to a finished test.
	t.Cleanup(func() {
		cancel()
		<-stopped
	})
	if !cache.WaitForCacheSync(ctx.Done(), informer.HasSynced) {
		t.Fatal("timed out waiting for the informer to sync")
	}
	select {
	case <-watching:
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for the informer to watch")
	}
}

// NextQueueKey returns the next key added to the controller work queue and
// marks it done. It fails the test if no key arrives within 10 seconds.
func NextQueueKey(t *testing.T, impl *controller.Impl) types.NamespacedName {
	t.Helper()
	queue := impl.WorkQueue()
	keys := make(chan interface{}, 1)
	go func() {
		key, shutdown := queue.Get()
		if shutdown {
			return
		}
		queue.Done(key)
		keys <- key
	}()
	select {
	case key := <-keys:
		return key.(types.NamespacedName)
	case <-time.After(10 * time.Second):
		queue.ShutDown()
		t.Fatal("timed out waiting for a key in the work queue")
		return types.NamespacedName{}
	}
}
