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

// reconcilerControllerStubGenerator produces a file of the stub of the
// controller for a custom impl with injection.
type reconcilerControllerStubGenerator struct {
	generator.DefaultGen
	outputPackage  string
	imports        namer.ImportTracker
	typeToGenerate *types.Type

	reconcilerPkg       string
	informerPackagePath string
	reconcilerClass     string
	hasReconcilerClass  bool
}

var _ generator.Generator = (*reconcilerControllerStubGenerator)(nil)

func (g *reconcilerControllerStubGenerator) Filter(c *generator.Context, t *types.Type) bool {
	// Only process the type for this generator.
	return t == g.typeToGenerate
}

func (g *reconcilerControllerStubGenerator) Namers(c *generator.Context) namer.NameSystems {
	return namer.NameSystems{
		"raw": namer.NewRawNamer(g.outputPackage, g.imports),
	}
}

func (g *reconcilerControllerStubGenerator) Imports(c *generator.Context) (imports []string) {
	imports = append(imports, g.imports.ImportLines()...)
	return
}

func (g *reconcilerControllerStubGenerator) GenerateType(c *generator.Context, t *types.Type, w io.Writer) error {
	sw := generator.NewSnippetWriter(w, c, "{{", "}}")

	klog.V(5).Infof("processing type %v", t)

	m := map[string]interface{}{
		"type":     t,
		"class":    g.reconcilerClass,
		"hasClass": g.hasReconcilerClass,
		"informerGet": c.Universe.Function(types.Name{
			Package: g.informerPackagePath,
			Name:    "Get",
		}),
		"controllerImpl": c.Universe.Type(types.Name{Package: "knative.dev/pkg/controller", Name: "Impl"}),
		"reconcilerNewImpl": c.Universe.Type(types.Name{
			Package: g.reconcilerPkg,
			Name:    "NewImpl",
		}),
		"loggingFromContext": c.Universe.Function(types.Name{
			Package: "knative.dev/pkg/logging",
			Name:    "FromContext",
		}),
		"contextContext": c.Universe.Type(types.Name{
			Package: "context",
			Name:    "Context",
		}),
		"configmapWatcher": c.Universe.Type(types.Name{
			Package: "knative.dev/pkg/configmap",
			Name:    "Watcher",
		}),
	}

	sw.Do(reconcilerControllerStub, m)

	return sw.Error()
}

var reconcilerControllerStub = `
// TODO: PLEASE COPY AND MODIFY THIS FILE AS A STARTING POINT

// NewController creates a Reconciler for {{.type|public}} and returns the result of NewImpl.
func NewController(
	ctx {{.contextContext|raw}},
	cmw {{.configmapWatcher|raw}},
) *{{.controllerImpl|raw}} {
	logger := {{.loggingFromContext|raw}}(ctx)

	{{.type|lowercaseSingular}}Informer := {{.informerGet|raw}}(ctx)

	// TODO: setup additional informers here.
	{{if .hasClass}}// TODO: pass in the expected value for the class annotation filter.{{end}}

	r := &Reconciler{}
	impl := {{.reconcilerNewImpl|raw}}(ctx, r{{if .hasClass}}, "default"{{end}})

	logger.Info("Setting up event handlers.")

	{{.type|lowercaseSingular}}Informer.Informer().AddEventHandler(controller.HandleAll(impl.Enqueue))

	// TODO: add additional informer event handlers here.

	return impl
}
`
