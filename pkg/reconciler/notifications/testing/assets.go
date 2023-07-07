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
	"testing"

	bc "github.com/allegro/bigcache/v3"
	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/reconciler/events/cloudevent"
	rtesting "github.com/tektoncd/pipeline/pkg/reconciler/testing"
	"github.com/tektoncd/pipeline/test"
	"github.com/tektoncd/pipeline/test/names"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"
)

// TestEmitCloudEventsParams matches the parameter of the `EmitCloudEvents`
// function, it is used to record invocations of the function
type TestEmitCloudEventsParams struct {
	Ctx    context.Context //nolint:containedctx
	Object runtime.Object
}

// Reconciler implements the Reconciler interface from the notification package
type FakeReconciler struct {
	cloudEventClient cloudevent.CEClient
	cacheClient      *bc.BigCache
}

func (c *FakeReconciler) GetCloudEventsClient() cloudevent.CEClient {
	return c.cloudEventClient
}

func (c *FakeReconciler) GetCacheClient() *bc.BigCache {
	return c.cacheClient
}

func (c *FakeReconciler) SetCloudEventsClient(client cloudevent.CEClient) {
	c.cloudEventClient = client
}

func (c *FakeReconciler) SetCacheClient(client *bc.BigCache) {
	c.cacheClient = client
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
//	reconciler := &rtesting.FakeReconciler{}
//	notifications.ReconcilerFromContext(testAssets.Ctx, reconciler)
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
