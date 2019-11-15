/*
Copyright 2019 The Knative Authors.

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
	"path"
	"reflect"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"go.uber.org/zap"

	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"
	clientgotesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"

	"knative.dev/pkg/controller"
	"knative.dev/pkg/kmeta"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/logging/logkey"
	_ "knative.dev/pkg/system/testing" // Setup system.Namespace()
)

// TableRow holds a single row of our table test.
type TableRow struct {
	// Name is a descriptive name for this test suitable as a first argument to t.Run()
	Name string

	// Ctx is the context to pass to Reconcile. Defaults to context.Background()
	Ctx context.Context

	// Objects holds the state of the world at the onset of reconciliation.
	Objects []runtime.Object

	// Key is the parameter to reconciliation.
	// This has the form "namespace/name".
	Key string

	// WantErr holds whether we should expect the reconciliation to result in an error.
	WantErr bool

	// WantCreates holds the ordered list of Create calls we expect during reconciliation.
	WantCreates []runtime.Object

	// WantUpdates holds the ordered list of Update calls we expect during reconciliation.
	WantUpdates []clientgotesting.UpdateActionImpl

	// WantStatusUpdates holds the ordered list of Update calls, with `status` subresource set,
	// that we expect during reconciliation.
	WantStatusUpdates []clientgotesting.UpdateActionImpl

	// WantDeletes holds the ordered list of Delete calls we expect during reconciliation.
	WantDeletes []clientgotesting.DeleteActionImpl

	// WantDeleteCollections holds the ordered list of DeleteCollection calls we expect during reconciliation.
	WantDeleteCollections []clientgotesting.DeleteCollectionActionImpl

	// WantPatches holds the ordered list of Patch calls we expect during reconciliation.
	WantPatches []clientgotesting.PatchActionImpl

	// WantEvents holds the ordered list of events we expect during reconciliation.
	WantEvents []string

	// WantServiceReadyStats holds the ServiceReady stats we exepect during reconciliation.
	WantServiceReadyStats map[string]int

	// WithReactors is a set of functions that are installed as Reactors for the execution
	// of this row of the table-driven-test.
	WithReactors []clientgotesting.ReactionFunc

	// For cluster-scoped resources like ClusterIngress, it does not have to be
	// in the same namespace with its child resources.
	SkipNamespaceValidation bool
}

func objKey(o runtime.Object) string {
	on := o.(kmeta.Accessor)
	// namespace + name is not unique, and the tests don't populate k8s kind
	// information, so use GoLang's type name as part of the key.
	return path.Join(reflect.TypeOf(o).String(), on.GetNamespace(), on.GetName())
}

// Factory returns a Reconciler.Interface to perform reconciliation in table test,
// ActionRecorderList/EventList to capture k8s actions/events produced during reconciliation
// and FakeStatsReporter to capture stats.
type Factory func(*testing.T, *TableRow) (controller.Reconciler, ActionRecorderList, EventList, *FakeStatsReporter)

// Test executes the single table test.
func (r *TableRow) Test(t *testing.T, factory Factory) {
	t.Helper()
	c, recorderList, eventList, statsReporter := factory(t, r)

	// Set context to not be nil.
	ctx := r.Ctx
	if ctx == nil {
		ctx = context.Background()
	} else {
		// If we have logger setup on the context, decorate it with the key, so that the logs
		// look like in prod.
		l := logging.FromContext(ctx)
		l = l.With(zap.String(logkey.Key, r.Key))
		ctx = logging.WithLogger(ctx, l)
	}

	// Run the Reconcile we're testing.
	if err := c.Reconcile(ctx, r.Key); (err != nil) != r.WantErr {
		t.Errorf("Reconcile() error = %v, WantErr %v", err, r.WantErr)
	}

	expectedNamespace, _, _ := cache.SplitMetaNamespaceKey(r.Key)

	actions, err := recorderList.ActionsByVerb()
	if err != nil {
		t.Errorf("Error capturing actions by verb: %q", err)
	}

	// Previous state is used to diff resource expected state for update requests that were missed.
	objPrevState := make(map[string]runtime.Object, len(r.Objects))
	for _, o := range r.Objects {
		objPrevState[objKey(o)] = o
	}

	for i, want := range r.WantCreates {
		if i >= len(actions.Creates) {
			t.Errorf("Missing create: %#v", want)
			continue
		}
		got := actions.Creates[i]
		obj := got.GetObject()
		objPrevState[objKey(obj)] = obj

		if !r.SkipNamespaceValidation && got.GetNamespace() != expectedNamespace {
			t.Errorf("Unexpected action[%d]: %#v", i, got)
		}

		if diff := cmp.Diff(want, obj, ignoreLastTransitionTime, safeDeployDiff, cmpopts.EquateEmpty()); diff != "" {
			t.Errorf("Unexpected create (-want, +got): %s", diff)
		}
	}
	if got, want := len(actions.Creates), len(r.WantCreates); got > want {
		for _, extra := range actions.Creates[want:] {
			t.Errorf("Extra create: %#v", extra.GetObject())
		}
	}

	updates := filterUpdatesWithSubresource("", actions.Updates)
	for i, want := range r.WantUpdates {
		if i >= len(updates) {
			wo := want.GetObject()
			key := objKey(wo)
			oldObj, ok := objPrevState[key]
			if !ok {
				t.Errorf("Object %s was never created: want: %#v", key, wo)
				continue
			}
			t.Errorf("Missing update for %s (-want, +prevState): %s", key,
				cmp.Diff(wo, oldObj, ignoreLastTransitionTime, safeDeployDiff, cmpopts.EquateEmpty()))
			continue
		}

		if want.GetSubresource() != "" {
			t.Errorf("Expectation was invalid - it should not include a subresource: %#v", want)
		}

		got := updates[i].GetObject()

		// Update the object state.
		objPrevState[objKey(got)] = got

		if diff := cmp.Diff(want.GetObject(), got, ignoreLastTransitionTime, safeDeployDiff, cmpopts.EquateEmpty()); diff != "" {
			t.Errorf("Unexpected update (-want, +got): %s", diff)
		}
	}
	if got, want := len(updates), len(r.WantUpdates); got > want {
		for _, extra := range updates[want:] {
			t.Errorf("Extra update: %#v", extra.GetObject())
		}
	}

	// TODO(#2843): refactor.
	statusUpdates := filterUpdatesWithSubresource("status", actions.Updates)
	for i, want := range r.WantStatusUpdates {
		if i >= len(statusUpdates) {
			wo := want.GetObject()
			key := objKey(wo)
			oldObj, ok := objPrevState[key]
			if !ok {
				t.Errorf("Object %s was never created: want: %#v", key, wo)
				continue
			}
			t.Errorf("Missing status update for %s (-want, +prevState): %s", key,
				cmp.Diff(wo, oldObj, ignoreLastTransitionTime, safeDeployDiff, cmpopts.EquateEmpty()))
			continue
		}

		got := statusUpdates[i].GetObject()

		// Update the object state.
		objPrevState[objKey(got)] = got

		if diff := cmp.Diff(want.GetObject(), got, ignoreLastTransitionTime, safeDeployDiff, cmpopts.EquateEmpty()); diff != "" {
			t.Errorf("Unexpected status update (-want, +got): %s\nFull: %v", diff, got)
		}
	}
	if got, want := len(statusUpdates), len(r.WantStatusUpdates); got > want {
		for _, extra := range statusUpdates[want:] {
			wo := extra.GetObject()
			key := objKey(wo)
			oldObj, ok := objPrevState[key]
			if !ok {
				t.Errorf("Object %s was never created: want: %#v", key, wo)
				continue
			}
			t.Errorf("Extra status update for %s (-extra, +prevState): %s", key,
				cmp.Diff(wo, oldObj, ignoreLastTransitionTime, safeDeployDiff, cmpopts.EquateEmpty()))
		}
	}

	if len(statusUpdates)+len(updates) != len(actions.Updates) {
		var unexpected []runtime.Object

		for _, update := range actions.Updates {
			if update.GetSubresource() != "status" && update.GetSubresource() != "" {
				unexpected = append(unexpected, update.GetObject())
			}
		}

		t.Errorf("Unexpected subresource updates occurred %#v", unexpected)
	}

	for i, want := range r.WantDeletes {
		if i >= len(actions.Deletes) {
			t.Errorf("Missing delete: %#v", want)
			continue
		}
		got := actions.Deletes[i]
		if got.GetName() != want.GetName() {
			t.Errorf("Unexpected delete[%d]: %#v", i, got)
		}
		if !r.SkipNamespaceValidation && got.GetNamespace() != expectedNamespace {
			t.Errorf("Unexpected delete[%d]: %#v", i, got)
		}
	}
	if got, want := len(actions.Deletes), len(r.WantDeletes); got > want {
		for _, extra := range actions.Deletes[want:] {
			t.Errorf("Extra delete: %s/%s", extra.GetNamespace(), extra.GetName())
		}
	}

	for i, want := range r.WantDeleteCollections {
		if i >= len(actions.DeleteCollections) {
			t.Errorf("Missing delete-collection: %#v", want)
			continue
		}
		got := actions.DeleteCollections[i]
		if got, want := got.GetListRestrictions().Labels, want.GetListRestrictions().Labels; (got != nil) != (want != nil) || got.String() != want.String() {
			t.Errorf("Unexpected delete-collection[%d].Labels = %v, wanted %v", i, got, want)
		}
		if got, want := got.GetListRestrictions().Fields, want.GetListRestrictions().Fields; (got != nil) != (want != nil) || got.String() != want.String() {
			t.Errorf("Unexpected delete-collection[%d].Fields = %v, wanted %v", i, got, want)
		}
		if !r.SkipNamespaceValidation && got.GetNamespace() != expectedNamespace {
			t.Errorf("Unexpected delete-collection[%d]: %#v, wanted %s", i, got, expectedNamespace)
		}
	}
	if got, want := len(actions.DeleteCollections), len(r.WantDeleteCollections); got > want {
		for _, extra := range actions.DeleteCollections[want:] {
			t.Errorf("Extra delete-collection: %#v", extra)
		}
	}

	for i, want := range r.WantPatches {
		if i >= len(actions.Patches) {
			t.Errorf("Missing patch: %#v; raw: %s", want, string(want.GetPatch()))
			continue
		}

		got := actions.Patches[i]
		if got.GetName() != want.GetName() {
			t.Errorf("Unexpected patch[%d]: %#v", i, got)
		}
		if !r.SkipNamespaceValidation && got.GetNamespace() != expectedNamespace {
			t.Errorf("Unexpected patch[%d]: %#v", i, got)
		}
		if diff := cmp.Diff(string(want.GetPatch()), string(got.GetPatch())); diff != "" {
			t.Errorf("Unexpected patch(-want, +got): %s", diff)
		}
	}
	if got, want := len(actions.Patches), len(r.WantPatches); got > want {
		for _, extra := range actions.Patches[want:] {
			t.Errorf("Extra patch: %#v; raw: %s", extra, string(extra.GetPatch()))
		}
	}

	gotEvents := eventList.Events()
	for i, want := range r.WantEvents {
		if i >= len(gotEvents) {
			t.Errorf("Missing event: %s", want)
			continue
		}

		if diff := cmp.Diff(want, gotEvents[i]); diff != "" {
			t.Errorf("unexpected event(-want, +got): %s", diff)
		}
	}
	if got, want := len(gotEvents), len(r.WantEvents); got > want {
		for _, extra := range gotEvents[want:] {
			t.Errorf("Extra event: %s", extra)
		}
	}

	gotStats := statsReporter.GetServiceReadyStats()
	if diff := cmp.Diff(r.WantServiceReadyStats, gotStats); diff != "" {
		t.Errorf("Unexpected service ready stats (-want, +got): %s", diff)
	}
}

func filterUpdatesWithSubresource(
	subresource string,
	actions []clientgotesting.UpdateAction) (result []clientgotesting.UpdateAction) {
	for _, action := range actions {
		if action.GetSubresource() == subresource {
			result = append(result, action)
		}
	}
	return
}

// TableTest represents a list of TableRow tests instances.
type TableTest []TableRow

// Test executes the whole suite of the table tests.
func (tt TableTest) Test(t *testing.T, factory Factory) {
	t.Helper()
	for _, test := range tt {
		// Record the original objects in table.
		originObjects := make([]runtime.Object, 0, len(test.Objects))
		for _, obj := range test.Objects {
			originObjects = append(originObjects, obj.DeepCopyObject())
		}
		t.Run(test.Name, func(t *testing.T) {
			t.Helper()
			test.Test(t, factory)
		})
		// Validate cached objects do not get soiled after controller loops
		if diff := cmp.Diff(originObjects, test.Objects, safeDeployDiff, cmpopts.EquateEmpty()); diff != "" {
			t.Errorf("Unexpected objects in test %s (-want, +got): %v", test.Name, diff)
		}
	}
}

var (
	ignoreLastTransitionTime = cmp.FilterPath(func(p cmp.Path) bool {
		return strings.HasSuffix(p.String(), "LastTransitionTime.Inner.Time")
	}, cmp.Ignore())

	safeDeployDiff = cmpopts.IgnoreUnexported(resource.Quantity{})
)
