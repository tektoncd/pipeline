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

// fakeDuckGenerator produces a file of listers for a given GroupVersion and
// type.
type fakeDuckGenerator struct {
	generator.DefaultGen
	outputPackage string
	imports       namer.ImportTracker

	typeToGenerate   *types.Type
	groupVersion     clientgentypes.GroupVersion
	groupGoName      string
	duckInjectionPkg string
}

var _ generator.Generator = (*fakeDuckGenerator)(nil)

func (g *fakeDuckGenerator) Filter(c *generator.Context, t *types.Type) bool {
	// Only process the type for this duck generator.
	return t == g.typeToGenerate
}

func (g *fakeDuckGenerator) Namers(c *generator.Context) namer.NameSystems {
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

func (g *fakeDuckGenerator) Imports(c *generator.Context) (imports []string) {
	imports = append(imports, g.imports.ImportLines()...)
	return
}

func (g *fakeDuckGenerator) GenerateType(c *generator.Context, t *types.Type, w io.Writer) error {
	sw := generator.NewSnippetWriter(w, c, "{{", "}}")

	klog.V(5).Info("processing type ", t)

	m := map[string]interface{}{
		"withDuck": c.Universe.Type(types.Name{Package: g.duckInjectionPkg, Name: "WithDuck"}),
		"duckGet":  c.Universe.Function(types.Name{Package: g.duckInjectionPkg, Name: "Get"}),
		"group":    namer.IC(g.groupGoName),
		"type":     t,
		"version":  namer.IC(g.groupVersion.Version.String()),
		"injectionRegisterDuck": c.Universe.Function(types.Name{
			Package: "knative.dev/pkg/injection",
			Name:    "Fake.RegisterDuck",
		}),
	}

	sw.Do(injectionFakeDuck, m)

	return sw.Error()
}

var injectionFakeDuck = `
var Get = {{.duckGet|raw}}

func init() {
	{{.injectionRegisterDuck|raw}}({{.withDuck|raw}})
}
`
