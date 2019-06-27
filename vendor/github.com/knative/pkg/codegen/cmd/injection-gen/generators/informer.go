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
	"k8s.io/klog"
)

// injectionTestGenerator produces a file of listers for a given GroupVersion and
// type.
type injectionGenerator struct {
	generator.DefaultGen
	outputPackage               string
	groupVersion                clientgentypes.GroupVersion
	groupGoName                 string
	typeToGenerate              *types.Type
	imports                     namer.ImportTracker
	typedInformerPackage        string
	groupInformerFactoryPackage string
}

var _ generator.Generator = (*injectionGenerator)(nil)

func (g *injectionGenerator) Filter(c *generator.Context, t *types.Type) bool {
	// Only process the type for this informer generator.
	return t == g.typeToGenerate
}

func (g *injectionGenerator) Namers(c *generator.Context) namer.NameSystems {
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

func (g *injectionGenerator) Imports(c *generator.Context) (imports []string) {
	imports = append(imports, g.imports.ImportLines()...)
	return
}

func (g *injectionGenerator) GenerateType(c *generator.Context, t *types.Type, w io.Writer) error {
	sw := generator.NewSnippetWriter(w, c, "{{", "}}")

	klog.V(5).Infof("processing type %v", t)

	m := map[string]interface{}{
		"group":                     namer.IC(g.groupGoName),
		"type":                      t,
		"version":                   namer.IC(g.groupVersion.Version.String()),
		"injectionRegisterInformer": c.Universe.Type(types.Name{Package: "github.com/knative/pkg/injection", Name: "Default.RegisterInformer"}),
		"controllerInformer":        c.Universe.Type(types.Name{Package: "github.com/knative/pkg/controller", Name: "Informer"}),
		"informersTypedInformer":    c.Universe.Type(types.Name{Package: g.typedInformerPackage, Name: t.Name.Name + "Informer"}),
		"factoryGet":                c.Universe.Type(types.Name{Package: g.groupInformerFactoryPackage, Name: "Get"}),
		"loggingFromContext": c.Universe.Function(types.Name{
			Package: "github.com/knative/pkg/logging",
			Name:    "FromContext",
		}),
	}

	sw.Do(injectionInformer, m)

	return sw.Error()
}

var injectionInformer = `
func init() {
	{{.injectionRegisterInformer|raw}}(withInformer)
}

// Key is used for associating the Informer inside the context.Context.
type Key struct{}

func withInformer(ctx context.Context) (context.Context, {{.controllerInformer|raw}}) {
	f := {{.factoryGet|raw}}(ctx)
	inf := f.{{.group}}().{{.version}}().{{.type|publicPlural}}()
	return context.WithValue(ctx, Key{}, inf), inf.Informer()
}

// Get extracts the typed informer from the context.
func Get(ctx context.Context) {{.informersTypedInformer|raw}} {
	untyped := ctx.Value(Key{})
	if untyped == nil {
		{{.loggingFromContext|raw}}(ctx).Fatalf(
			"Unable to fetch %T from context.", ({{.informersTypedInformer|raw}})(nil))
	}
	return untyped.({{.informersTypedInformer|raw}})
}
`
