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

	clientgentypes "k8s.io/code-generator/cmd/client-gen/types"
	"k8s.io/gengo/generator"
	"k8s.io/gengo/namer"
	"k8s.io/gengo/types"
	"k8s.io/klog/v2"
)

// fakeFilteredInformerGenerator produces a file of listers for a given GroupVersion and
// type.
type fakeFilteredInformerGenerator struct {
	generator.DefaultGen
	outputPackage string
	imports       namer.ImportTracker

	typeToGenerate          *types.Type
	groupVersion            clientgentypes.GroupVersion
	groupGoName             string
	informerInjectionPkg    string
	fakeFactoryInjectionPkg string
}

var _ generator.Generator = (*fakeFilteredInformerGenerator)(nil)

func (g *fakeFilteredInformerGenerator) Filter(c *generator.Context, t *types.Type) bool {
	// Only process the type for this informer generator.
	return t == g.typeToGenerate
}

func (g *fakeFilteredInformerGenerator) Namers(c *generator.Context) namer.NameSystems {
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

func (g *fakeFilteredInformerGenerator) Imports(c *generator.Context) (imports []string) {
	imports = append(imports, g.imports.ImportLines()...)
	return
}

func (g *fakeFilteredInformerGenerator) GenerateType(c *generator.Context, t *types.Type, w io.Writer) error {
	sw := generator.NewSnippetWriter(w, c, "{{", "}}")

	klog.V(5).Info("processing type ", t)

	m := map[string]interface{}{
		"informerKey":        c.Universe.Type(types.Name{Package: g.informerInjectionPkg, Name: "Key"}),
		"informerGet":        c.Universe.Function(types.Name{Package: g.informerInjectionPkg, Name: "Get"}),
		"factoryGet":         c.Universe.Function(types.Name{Package: g.fakeFactoryInjectionPkg, Name: "Get"}),
		"factoryLabelKey":    c.Universe.Type(types.Name{Package: g.fakeFactoryInjectionPkg, Name: "LabelKey"}),
		"group":              namer.IC(g.groupGoName),
		"type":               t,
		"version":            namer.IC(g.groupVersion.Version.String()),
		"controllerInformer": c.Universe.Type(types.Name{Package: "knative.dev/pkg/controller", Name: "Informer"}),
		"injectionRegisterFilteredInformers": c.Universe.Function(types.Name{
			Package: "knative.dev/pkg/injection",
			Name:    "Fake.RegisterFilteredInformers",
		}),
		"loggingFromContext": c.Universe.Function(types.Name{
			Package: "knative.dev/pkg/logging",
			Name:    "FromContext",
		}),
		"contextContext": c.Universe.Type(types.Name{
			Package: "context",
			Name:    "Context",
		}),
	}

	sw.Do(injectionFakeFilteredInformer, m)

	return sw.Error()
}

var injectionFakeFilteredInformer = `
var Get = {{.informerGet|raw}}

func init() {
	{{.injectionRegisterFilteredInformers|raw}}(withInformer)
}

func withInformer(ctx {{.contextContext|raw}}) ({{.contextContext|raw}}, []{{.controllerInformer|raw}}) {
	untyped := ctx.Value({{.factoryLabelKey|raw}}{})
	if untyped == nil {
		{{.loggingFromContext|raw}}(ctx).Panic(
			"Unable to fetch labelkey from context.")
	}
	labelSelectors := untyped.([]string)
	infs := []{{.controllerInformer|raw}}{}
	for _, selector := range labelSelectors {
		f := {{.factoryGet|raw}}(ctx, selector)
		inf := f.{{.group}}().{{.version}}().{{.type|publicPlural}}()
		ctx = context.WithValue(ctx, {{.informerKey|raw}}{Selector: selector}, inf)
		infs = append(infs, inf.Informer())
	}
	return ctx, infs
}
`
