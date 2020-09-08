/*
Copyright 2020 The Knative Authors.

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

package generators

import (
	"io"

	"k8s.io/gengo/generator"
	"k8s.io/gengo/namer"
	"k8s.io/gengo/types"
	"k8s.io/klog"
)

// reconcilerStateGenerator produces a reconciler state object to manage reconciliation runs.
type reconcilerStateGenerator struct {
	generator.DefaultGen
	outputPackage  string
	imports        namer.ImportTracker
	typeToGenerate *types.Type
}

var _ generator.Generator = (*reconcilerStateGenerator)(nil)

func (g *reconcilerStateGenerator) Filter(c *generator.Context, t *types.Type) bool {
	// Only process the type for this generator.
	return t == g.typeToGenerate
}

func (g *reconcilerStateGenerator) Namers(c *generator.Context) namer.NameSystems {
	return namer.NameSystems{
		"raw": namer.NewRawNamer(g.outputPackage, g.imports),
	}
}

func (g *reconcilerStateGenerator) Imports(c *generator.Context) (imports []string) {
	imports = append(imports, g.imports.ImportLines()...)
	return
}

func (g *reconcilerStateGenerator) GenerateType(c *generator.Context, t *types.Type, w io.Writer) error {
	sw := generator.NewSnippetWriter(w, c, "{{", "}}")

	klog.V(5).Infof("processing type %v", t)

	m := map[string]interface{}{
		"type": t,
		// methods
		"cacheSplitMetaNamespaceKey": c.Universe.Function(types.Name{
			Package: "k8s.io/client-go/tools/cache",
			Name:    "SplitMetaNamespaceKey",
		}),
		"fmtErrorf": c.Universe.Package("fmt").Function("Errorf"),
		"reconcilerLeaderAware": c.Universe.Type(types.Name{
			Package: "knative.dev/pkg/reconciler",
			Name:    "LeaderAware",
		}),
		"typesNamespacedName": c.Universe.Type(types.Name{
			Package: "k8s.io/apimachinery/pkg/types",
			Name:    "NamespacedName",
		}),
		"doReconcileKind": c.Universe.Type(types.Name{
			Package: "knative.dev/pkg/reconciler",
			Name:    "DoReconcileKind",
		}),
		"doObserveKind": c.Universe.Type(types.Name{
			Package: "knative.dev/pkg/reconciler",
			Name:    "DoObserveKind",
		}),
		"doFinalizeKind": c.Universe.Type(types.Name{
			Package: "knative.dev/pkg/reconciler",
			Name:    "DoFinalizeKind",
		}),
		"doObserveFinalizeKind": c.Universe.Type(types.Name{
			Package: "knative.dev/pkg/reconciler",
			Name:    "DoObserveFinalizeKind",
		}),
	}

	sw.Do(reconcilerStateType, m)
	sw.Do(reconcilerStateMethods, m)

	return sw.Error()
}

var reconcilerStateType = `
// state is used to track the state of a reconciler in a single run.
type state struct {
	// Key is the original reconciliation key from the queue.
	key string
	// Namespace is the namespace split from the reconciliation key.
	namespace string
	// Namespace is the name split from the reconciliation key.
	name string
	// reconciler is the reconciler.
	reconciler Interface
	// rof is the read only interface cast of the reconciler.
	roi ReadOnlyInterface
	// IsROI (Read Only Interface) the reconciler only observes reconciliation.
	isROI bool
	// rof is the read only finalizer cast of the reconciler.
	rof ReadOnlyFinalizer
	// IsROF (Read Only Finalizer) the reconciler only observes finalize.
	isROF bool
	// IsLeader the instance of the reconciler is the elected leader.
	isLeader bool
}

func newState(key string, r *reconcilerImpl) (*state, error) {
	// Convert the namespace/name string into a distinct namespace and name
	namespace, name, err := {{.cacheSplitMetaNamespaceKey|raw}}(key)
	if err != nil {
		return nil, {{.fmtErrorf|raw}}("invalid resource key: %s", key)
	}

	roi, isROI := r.reconciler.(ReadOnlyInterface)
	rof, isROF := r.reconciler.(ReadOnlyFinalizer)

	isLeader := r.IsLeaderFor({{.typesNamespacedName|raw}}{
		Namespace: namespace,
		Name:      name,
	})

	return &state{
		key:        key,
		namespace:  namespace,
		name:       name,
		reconciler: r.reconciler,
		roi:        roi,
		isROI:      isROI,
		rof:        rof,
		isROF:      isROF,
		isLeader:   isLeader,
	}, nil
}
`

var reconcilerStateMethods = `
// isNotLeaderNorObserver checks to see if this reconciler with the current
// state is enabled to do any work or not.
// isNotLeaderNorObserver returns true when there is no work possible for the
// reconciler.
func (s *state) isNotLeaderNorObserver() bool {
	if !s.isLeader && !s.isROI && !s.isROF {
		// If we are not the leader, and we don't implement either ReadOnly
		// interface, then take a fast-path out.
		return true
	}
	return false
}

func (s *state) reconcileMethodFor(o *{{.type|raw}}) (string, doReconcile) {
	if o.GetDeletionTimestamp().IsZero() {
		if s.isLeader {
			return {{.doReconcileKind|raw}}, s.reconciler.ReconcileKind
		} else if s.isROI {
			return {{.doObserveKind|raw}}, s.roi.ObserveKind
		}
	} else if fin, ok := s.reconciler.(Finalizer); s.isLeader && ok {
		return {{.doFinalizeKind|raw}}, fin.FinalizeKind
	} else if !s.isLeader && s.isROF {
		return {{.doObserveFinalizeKind|raw}}, s.rof.ObserveFinalizeKind
	}
	return "unknown", nil
}
`
