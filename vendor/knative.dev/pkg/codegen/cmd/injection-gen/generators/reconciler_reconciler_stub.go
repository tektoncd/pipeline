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

// reconcilerReconcilerStubGenerator produces a file of the stub of how to
// implement the reconciler.
type reconcilerReconcilerStubGenerator struct {
	generator.DefaultGen
	outputPackage  string
	imports        namer.ImportTracker
	typeToGenerate *types.Type

	reconcilerPkg string
}

var _ generator.Generator = (*reconcilerReconcilerStubGenerator)(nil)

func (g *reconcilerReconcilerStubGenerator) Filter(c *generator.Context, t *types.Type) bool {
	// Only process the type for this generator.
	return t == g.typeToGenerate
}

func (g *reconcilerReconcilerStubGenerator) Namers(c *generator.Context) namer.NameSystems {
	return namer.NameSystems{
		"raw": namer.NewRawNamer(g.outputPackage, g.imports),
	}
}

func (g *reconcilerReconcilerStubGenerator) Imports(c *generator.Context) (imports []string) {
	imports = append(imports, g.imports.ImportLines()...)
	return
}

func (g *reconcilerReconcilerStubGenerator) GenerateType(c *generator.Context, t *types.Type, w io.Writer) error {
	sw := generator.NewSnippetWriter(w, c, "{{", "}}")

	klog.V(5).Infof("processing type %v", t)

	m := map[string]interface{}{
		"type": t,
		"reconcilerEvent": c.Universe.Type(types.Name{
			Package: "knative.dev/pkg/reconciler",
			Name:    "Event",
		}),
		"reconcilerNewEvent": c.Universe.Function(types.Name{
			Package: "knative.dev/pkg/reconciler",
			Name:    "NewEvent",
		}),
		"reconcilerInterface": c.Universe.Type(types.Name{
			Package: g.reconcilerPkg,
			Name:    "Interface",
		}),
		"reconcilerFinalizer": c.Universe.Type(types.Name{
			Package: g.reconcilerPkg,
			Name:    "Finalizer",
		}),
		"corev1EventTypeNormal": c.Universe.Type(types.Name{
			Package: "k8s.io/api/core/v1",
			Name:    "EventTypeNormal",
		}),
		"contextContext": c.Universe.Type(types.Name{
			Package: "context",
			Name:    "Context",
		}),
	}

	sw.Do(reconcilerReconcilerStub, m)

	return sw.Error()
}

var reconcilerReconcilerStub = `
// TODO: PLEASE COPY AND MODIFY THIS FILE AS A STARTING POINT

// newReconciledNormal makes a new reconciler event with event type Normal, and
// reason {{.type|public}}Reconciled.
func newReconciledNormal(namespace, name string) reconciler.Event {
	return {{.reconcilerNewEvent|raw}}({{.corev1EventTypeNormal|raw}}, "{{.type|public}}Reconciled", "{{.type|public}} reconciled: \"%s/%s\"", namespace, name)
}

// Reconciler implements controller.Reconciler for {{.type|public}} resources.
type Reconciler struct {
	// TODO: add additional requirements here.
}

// Check that our Reconciler implements Interface
var _ {{.reconcilerInterface|raw}} = (*Reconciler)(nil)

// Optionally check that our Reconciler implements Finalizer
//var _ {{.reconcilerFinalizer|raw}} = (*Reconciler)(nil)


// ReconcileKind implements Interface.ReconcileKind.
func (r *Reconciler) ReconcileKind(ctx {{.contextContext|raw}}, o *{{.type|raw}}) {{.reconcilerEvent|raw}} {
    // TODO: use this if the resource implements InitializeConditions.
	// o.Status.InitializeConditions()

	// TODO: add custom reconciliation logic here.

	// TODO: use this if the object has .status.ObservedGeneration.
    // o.Status.ObservedGeneration = o.Generation
	return newReconciledNormal(o.Namespace, o.Name)
}

// Optionally, use FinalizeKind to add finalizers. FinalizeKind will be called
// when the resource is deleted.
//func (r *Reconciler) FinalizeKind(ctx {{.contextContext|raw}}, o *{{.type|raw}}) {{.reconcilerEvent|raw}} {
//	// TODO: add custom finalization logic here.
//	return nil
//}
`
