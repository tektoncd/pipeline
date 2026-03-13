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

package notifications_test

import (
	"testing"

	"github.com/tektoncd/pipeline/pkg/reconciler/notifications"
	ntesting "github.com/tektoncd/pipeline/pkg/reconciler/notifications/testing"
	"github.com/tektoncd/pipeline/test"
	cminformer "knative.dev/pkg/configmap/informer"
	"knative.dev/pkg/system"
	_ "knative.dev/pkg/system/testing" // Setup system.Namespace()
)

func TestConfigStoreFromContext(t *testing.T) {
	testAssets, cancel := ntesting.InitializeTestAssets(t, &test.Data{})
	defer cancel()

	cmw := cminformer.NewInformedWatcher(testAssets.Clients.Kube, system.Namespace())
	store := notifications.ConfigStoreFromContext(testAssets.Ctx, cmw)
	if store == nil {
		t.Fatal("expected a non-nil config store")
	}
}

func TestControllerOptions(t *testing.T) {
	testAssets, cancel := ntesting.InitializeTestAssets(t, &test.Data{})
	defer cancel()

	cmw := cminformer.NewInformedWatcher(testAssets.Clients.Kube, system.Namespace())
	store := notifications.ConfigStoreFromContext(testAssets.Ctx, cmw)

	optsFn := notifications.ControllerOptions("TestAgent", store)
	opts := optsFn(nil)

	if opts.AgentName != "TestAgent" {
		t.Errorf("expected AgentName %q, got %q", "TestAgent", opts.AgentName)
	}
	if opts.ConfigStore != store {
		t.Error("expected ConfigStore to match the provided store")
	}
	if !opts.SkipStatusUpdates {
		t.Error("expected SkipStatusUpdates to be true")
	}
}
