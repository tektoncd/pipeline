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

package generators

import (
	"io"

	clientgentypes "k8s.io/code-generator/cmd/client-gen/types"
	"k8s.io/gengo/generator"
	"k8s.io/gengo/namer"
	"k8s.io/gengo/types"
	"k8s.io/klog/v2"
)

// duckGenerator produces logic to register a duck.InformerFactory for a particular
// type onto context.
type duckGenerator struct {
	generator.DefaultGen
	outputPackage  string
	groupVersion   clientgentypes.GroupVersion
	groupGoName    string
	typeToGenerate *types.Type
	imports        namer.ImportTracker
}

var _ generator.Generator = (*duckGenerator)(nil)

func (g *duckGenerator) Filter(c *generator.Context, t *types.Type) bool {
	// Only process the type for this informer generator.
	return t == g.typeToGenerate
}

func (g *duckGenerator) Namers(c *generator.Context) namer.NameSystems {
	publicPluralNamer := &ExceptionNamer{
		Exceptions: map[string]string{
			// these exceptions are used to deconflict the generated code
			// you can put your fully qualified package like
			// to generate a name that doesn't conflict with your group.
			// "k8s.io/apis/events/v1beta1.Event": "EventResource"
		},
		KeyFunc: func(t *types.Type) string {
			return t.Name.Package + "." + t.Name.Name
		},
		Delegate: namer.NewPublicPluralNamer(map[string]string{
			"Endpoints": "Endpoints",
		}),
	}

	return namer.NameSystems{
		"raw":          namer.NewRawNamer(g.outputPackage, g.imports),
		"publicPlural": publicPluralNamer,
	}
}

func (g *duckGenerator) Imports(c *generator.Context) (imports []string) {
	imports = append(imports, g.imports.ImportLines()...)
	return
}

func (g *duckGenerator) GenerateType(c *generator.Context, t *types.Type, w io.Writer) error {
	sw := generator.NewSnippetWriter(w, c, "{{", "}}")

	klog.V(5).Info("processing type ", t)

	m := map[string]interface{}{
		"group":                     namer.IC(g.groupGoName),
		"type":                      t,
		"version":                   namer.IC(g.groupVersion.Version.String()),
		"injectionRegisterDuck":     c.Universe.Type(types.Name{Package: "knative.dev/pkg/injection", Name: "Default.RegisterDuck"}),
		"getResyncPeriod":           c.Universe.Type(types.Name{Package: "knative.dev/pkg/controller", Name: "GetResyncPeriod"}),
		"dynamicGet":                c.Universe.Type(types.Name{Package: "knative.dev/pkg/injection/clients/dynamicclient", Name: "Get"}),
		"duckTypedInformerFactory":  c.Universe.Type(types.Name{Package: "knative.dev/pkg/apis/duck", Name: "TypedInformerFactory"}),
		"duckCachedInformerFactory": c.Universe.Type(types.Name{Package: "knative.dev/pkg/apis/duck", Name: "CachedInformerFactory"}),
		"duckInformerFactory":       c.Universe.Type(types.Name{Package: "knative.dev/pkg/apis/duck", Name: "InformerFactory"}),
		"loggingFromContext": c.Universe.Function(types.Name{
			Package: "knative.dev/pkg/logging",
			Name:    "FromContext",
		}),
		"contextContext": c.Universe.Type(types.Name{
			Package: "context",
			Name:    "Context",
		}),
	}

	sw.Do(duckFactory, m)

	return sw.Error()
}

var duckFactory = `
func init() {
	{{.injectionRegisterDuck|raw}}(WithDuck)
}

// Key is used for associating the Informer inside the context.Context.
type Key struct{}

func WithDuck(ctx {{.contextContext|raw}}) {{.contextContext|raw}} {
	dc := {{.dynamicGet|raw}}(ctx)
	dif := &{{.duckCachedInformerFactory|raw}}{
		Delegate: &{{.duckTypedInformerFactory|raw}}{
			Client:       dc,
			Type:         (&{{.type|raw}}{}).GetFullType(),
			ResyncPeriod: {{.getResyncPeriod|raw}}(ctx),
			StopChannel:  ctx.Done(),
		},
	}
	return context.WithValue(ctx, Key{}, dif)
}

// Get extracts the typed informer from the context.
func Get(ctx {{.contextContext|raw}}) {{.duckInformerFactory|raw}} {
	untyped := ctx.Value(Key{})
	if untyped == nil {
		{{.loggingFromContext|raw}}(ctx).Panic(
			"Unable to fetch {{.duckInformerFactory}} from context.")
	}
	return untyped.({{.duckInformerFactory|raw}})
}
`
