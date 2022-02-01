/*
Copyright 2021 The Knative Authors

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
	"k8s.io/klog/v2"
)

// factoryTestGenerator produces a file of factory injection of a given type.
type filteredFactoryGenerator struct {
	generator.DefaultGen
	outputPackage                string
	imports                      namer.ImportTracker
	cachingClientSetPackage      string
	sharedInformerFactoryPackage string
	filtered                     bool
}

var _ generator.Generator = (*filteredFactoryGenerator)(nil)

func (g *filteredFactoryGenerator) Filter(c *generator.Context, t *types.Type) bool {
	// We generate a single factory, so return true once.
	if !g.filtered {
		g.filtered = true
		return true
	}
	return false
}

func (g *filteredFactoryGenerator) Namers(c *generator.Context) namer.NameSystems {
	return namer.NameSystems{
		"raw": namer.NewRawNamer(g.outputPackage, g.imports),
	}
}

func (g *filteredFactoryGenerator) Imports(c *generator.Context) (imports []string) {
	imports = append(imports, g.imports.ImportLines()...)
	return
}

func (g *filteredFactoryGenerator) GenerateType(c *generator.Context, t *types.Type, w io.Writer) error {
	sw := generator.NewSnippetWriter(w, c, "{{", "}}")

	klog.V(5).Info("processing type ", t)

	m := map[string]interface{}{
		"cachingClientGet": c.Universe.Type(types.Name{Package: g.cachingClientSetPackage, Name: "Get"}),
		"informersNewSharedInformerFactoryWithOptions": c.Universe.Function(types.Name{Package: g.sharedInformerFactoryPackage, Name: "NewSharedInformerFactoryWithOptions"}),
		"informersSharedInformerOption":                c.Universe.Function(types.Name{Package: g.sharedInformerFactoryPackage, Name: "SharedInformerOption"}),
		"informersWithNamespace":                       c.Universe.Function(types.Name{Package: g.sharedInformerFactoryPackage, Name: "WithNamespace"}),
		"informersWithTweakListOptions":                c.Universe.Function(types.Name{Package: g.sharedInformerFactoryPackage, Name: "WithTweakListOptions"}),
		"informersSharedInformerFactory":               c.Universe.Function(types.Name{Package: g.sharedInformerFactoryPackage, Name: "SharedInformerFactory"}),
		"injectionRegisterInformerFactory":             c.Universe.Type(types.Name{Package: "knative.dev/pkg/injection", Name: "Default.RegisterInformerFactory"}),
		"injectionHasNamespace":                        c.Universe.Type(types.Name{Package: "knative.dev/pkg/injection", Name: "HasNamespaceScope"}),
		"injectionGetNamespace":                        c.Universe.Type(types.Name{Package: "knative.dev/pkg/injection", Name: "GetNamespaceScope"}),
		"controllerGetResyncPeriod":                    c.Universe.Type(types.Name{Package: "knative.dev/pkg/controller", Name: "GetResyncPeriod"}),
		"loggingFromContext": c.Universe.Function(types.Name{
			Package: "knative.dev/pkg/logging",
			Name:    "FromContext",
		}),
		"contextContext": c.Universe.Type(types.Name{
			Package: "context",
			Name:    "Context",
		}),
		"metav1ListOptions": c.Universe.Type(types.Name{
			Package: "k8s.io/apimachinery/pkg/apis/meta/v1",
			Name:    "ListOptions",
		}),
	}

	sw.Do(injectionFilteredFactory, m)

	return sw.Error()
}

var injectionFilteredFactory = `
func init() {
	{{.injectionRegisterInformerFactory|raw}}(withInformerFactory)
}

// Key is used as the key for associating information with a context.Context.
type Key struct{
	Selector string
}

type LabelKey struct{}

func WithSelectors(ctx {{.contextContext|raw}}, selector ...string) context.Context {
	return context.WithValue(ctx, LabelKey{}, selector)
}

func withInformerFactory(ctx {{.contextContext|raw}}) {{.contextContext|raw}} {
	c := {{.cachingClientGet|raw}}(ctx)
	untyped := ctx.Value(LabelKey{})
	if untyped == nil {
		{{.loggingFromContext|raw}}(ctx).Panic(
			"Unable to fetch labelkey from context.")
	}
	labelSelectors := untyped.([]string)
	for _, selector := range labelSelectors {
		opts := []{{.informersSharedInformerOption|raw}}{}
		if {{.injectionHasNamespace|raw}}(ctx) {
			opts = append(opts, {{.informersWithNamespace|raw}}({{.injectionGetNamespace|raw}}(ctx)))
		}	
		opts = append(opts, {{.informersWithTweakListOptions|raw}}(func(l *{{.metav1ListOptions|raw}}) {
			l.LabelSelector = selector
		}))
		ctx = context.WithValue(ctx, Key{Selector: selector},
			{{.informersNewSharedInformerFactoryWithOptions|raw}}(c, {{.controllerGetResyncPeriod|raw}}(ctx), opts...))
	}
	return ctx
}

// Get extracts the InformerFactory from the context.
func Get(ctx {{.contextContext|raw}}, selector string) {{.informersSharedInformerFactory|raw}} {
	untyped := ctx.Value(Key{Selector: selector})
	if untyped == nil {
		{{.loggingFromContext|raw}}(ctx).Panicf(
			"Unable to fetch {{.informersSharedInformerFactory}} with selector %s from context.", selector)
	}
	return untyped.({{.informersSharedInformerFactory|raw}})
}
`
