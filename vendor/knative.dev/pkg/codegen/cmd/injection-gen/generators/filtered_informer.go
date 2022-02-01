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

	"k8s.io/code-generator/cmd/client-gen/generators/util"
	clientgentypes "k8s.io/code-generator/cmd/client-gen/types"
	"k8s.io/gengo/generator"
	"k8s.io/gengo/namer"
	"k8s.io/gengo/types"
	"k8s.io/klog/v2"
)

// injectionTestGenerator produces a file of listers for a given GroupVersion and
// type.
type filteredInjectionGenerator struct {
	generator.DefaultGen
	outputPackage               string
	groupVersion                clientgentypes.GroupVersion
	groupGoName                 string
	typeToGenerate              *types.Type
	imports                     namer.ImportTracker
	typedInformerPackage        string
	groupInformerFactoryPackage string
	injectionClientSetPackage   string
	clientSetPackage            string
	listerPkg                   string
}

var _ generator.Generator = (*filteredInjectionGenerator)(nil)

func (g *filteredInjectionGenerator) Filter(c *generator.Context, t *types.Type) bool {
	// Only process the type for this informer generator.
	return t == g.typeToGenerate
}

func (g *filteredInjectionGenerator) Namers(c *generator.Context) namer.NameSystems {
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

func (g *filteredInjectionGenerator) Imports(c *generator.Context) (imports []string) {
	imports = append(imports, g.imports.ImportLines()...)
	return
}

func (g *filteredInjectionGenerator) GenerateType(c *generator.Context, t *types.Type, w io.Writer) error {
	sw := generator.NewSnippetWriter(w, c, "{{", "}}")

	klog.V(5).Info("processing type ", t)

	tags, err := util.ParseClientGenTags(append(g.typeToGenerate.SecondClosestCommentLines, g.typeToGenerate.CommentLines...))
	if err != nil {
		return err
	}

	m := map[string]interface{}{
		"clientGet":                          c.Universe.Type(types.Name{Package: g.injectionClientSetPackage, Name: "Get"}),
		"clientSetInterface":                 c.Universe.Type(types.Name{Package: g.clientSetPackage, Name: "Interface"}),
		"resourceLister":                     c.Universe.Type(types.Name{Name: g.typeToGenerate.Name.Name + "Lister", Package: g.listerPkg}),
		"resourceNamespaceLister":            c.Universe.Type(types.Name{Name: g.typeToGenerate.Name.Name + "NamespaceLister", Package: g.listerPkg}),
		"groupGoName":                        namer.IC(g.groupGoName),
		"versionGoName":                      namer.IC(g.groupVersion.Version.String()),
		"group":                              namer.IC(g.groupGoName),
		"type":                               t,
		"version":                            namer.IC(g.groupVersion.Version.String()),
		"injectionRegisterFilteredInformers": c.Universe.Type(types.Name{Package: "knative.dev/pkg/injection", Name: "Default.RegisterFilteredInformers"}),
		"injectionRegisterDynamicInformer":   c.Universe.Type(types.Name{Package: "knative.dev/pkg/injection", Name: "Dynamic.RegisterDynamicInformer"}),
		"controllerInformer":                 c.Universe.Type(types.Name{Package: "knative.dev/pkg/controller", Name: "Informer"}),
		"informersTypedInformer":             c.Universe.Type(types.Name{Package: g.typedInformerPackage, Name: t.Name.Name + "Informer"}),
		"factoryLabelKey":                    c.Universe.Type(types.Name{Package: g.groupInformerFactoryPackage, Name: "LabelKey"}),
		"factoryGet":                         c.Universe.Function(types.Name{Package: g.groupInformerFactoryPackage, Name: "Get"}),
		"loggingFromContext": c.Universe.Function(types.Name{
			Package: "knative.dev/pkg/logging",
			Name:    "FromContext",
		}),
		"contextContext": c.Universe.Type(types.Name{
			Package: "context",
			Name:    "Context",
		}),
		"contextWithValue": c.Universe.Function(types.Name{
			Package: "context",
			Name:    "WithValue",
		}),
		"schemaGVR": c.Universe.Type(types.Name{
			Package: "k8s.io/apimachinery/pkg/runtime/schema",
			Name:    "GroupVersionResource",
		}),
		"labelsSelector": c.Universe.Type(types.Name{
			Package: "k8s.io/apimachinery/pkg/labels",
			Name:    "Selector",
		}),
		"labelsParseToRequirements": c.Universe.Type(types.Name{
			Package: "k8s.io/apimachinery/pkg/labels",
			Name:    "ParseToRequirements",
		}),
		"contextTODO": c.Universe.Function(types.Name{
			Package: "context",
			Name:    "TODO",
		}),
		"metav1GetOptions": c.Universe.Type(types.Name{
			Package: "k8s.io/apimachinery/pkg/apis/meta/v1",
			Name:    "GetOptions",
		}),
		"metav1ListOptions": c.Universe.Type(types.Name{
			Package: "k8s.io/apimachinery/pkg/apis/meta/v1",
			Name:    "ListOptions",
		}),
		"cacheSharedIndexInformer": c.Universe.Type(types.Name{
			Package: "k8s.io/client-go/tools/cache",
			Name:    "SharedIndexInformer",
		}),
		"cacheNewSharedIndexInformer": c.Universe.Function(types.Name{
			Package: "k8s.io/client-go/tools/cache",
			Name:    "NewSharedIndexInformer",
		}),
		"Namespaced": !tags.NonNamespaced,
	}

	sw.Do(filteredInjectionInformer, m)

	return sw.Error()
}

var filteredInjectionInformer = `
func init() {
	{{.injectionRegisterFilteredInformers|raw}}(withInformer)
	{{.injectionRegisterDynamicInformer|raw}}(withDynamicInformer)
}

// Key is used for associating the Informer inside the context.Context.
type Key struct{
	Selector string
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
		ctx = {{ .contextWithValue|raw }}(ctx, Key{Selector: selector}, inf)
		infs = append(infs, inf.Informer())
	}
	return ctx, infs
}

func withDynamicInformer(ctx {{.contextContext|raw}}) {{.contextContext|raw}} {
	untyped := ctx.Value({{.factoryLabelKey|raw}}{})
	if untyped == nil {
		{{.loggingFromContext|raw}}(ctx).Panic(
			"Unable to fetch labelkey from context.")
	}
	labelSelectors := untyped.([]string)
	for _, selector := range labelSelectors {
		inf := &wrapper{client: {{ .clientGet|raw }}(ctx), selector: selector}
		ctx = {{ .contextWithValue|raw }}(ctx, Key{Selector: selector}, inf)
	}
	return ctx
}

// Get extracts the typed informer from the context.
func Get(ctx {{.contextContext|raw}}, selector string) {{.informersTypedInformer|raw}} {
	untyped := ctx.Value(Key{Selector: selector})
	if untyped == nil {
		{{.loggingFromContext|raw}}(ctx).Panicf(
			"Unable to fetch {{.informersTypedInformer}} with selector %s from context.", selector)
	}
	return untyped.({{.informersTypedInformer|raw}})
}

type wrapper struct {
	client    {{.clientSetInterface|raw}}
{{ if .Namespaced }}
	namespace string
{{ end }}
	selector string
}

var _ {{.informersTypedInformer|raw}} = (*wrapper)(nil)
var _ {{.resourceLister|raw}} = (*wrapper)(nil)

func (w *wrapper) Informer() {{ .cacheSharedIndexInformer|raw }} {
	return {{ .cacheNewSharedIndexInformer|raw }}(nil, &{{ .type|raw }}{}, 0, nil)
}

func (w *wrapper) Lister() {{ .resourceLister|raw }} {
	return w
}

{{if .Namespaced}}
func (w *wrapper) {{ .type|publicPlural }}(namespace string) {{ .resourceNamespaceLister|raw }} {
	return &wrapper{client: w.client, namespace: namespace, selector: w.selector}
}
{{end}}

func (w *wrapper) List(selector {{ .labelsSelector|raw }}) (ret []*{{ .type|raw }}, err error) {
	reqs, err := {{ .labelsParseToRequirements|raw }}(w.selector)
	if err != nil {
		return nil, err
	}
	selector = selector.Add(reqs...)
	lo, err := w.client.{{.groupGoName}}{{.versionGoName}}().{{.type|publicPlural}}({{if .Namespaced}}w.namespace{{end}}).List({{ .contextTODO|raw }}(), {{ .metav1ListOptions|raw }}{
		LabelSelector: selector.String(),
		// TODO(mattmoor): Incorporate resourceVersion bounds based on staleness criteria.
	})
	if err != nil {
		return nil, err
	}
	for idx := range lo.Items {
		ret = append(ret, &lo.Items[idx])
	}
	return ret, nil
}

func (w *wrapper) Get(name string) (*{{ .type|raw }}, error) {
	// TODO(mattmoor): Check that the fetched object matches the selector.
	return w.client.{{.groupGoName}}{{.versionGoName}}().{{.type|publicPlural}}({{if .Namespaced}}w.namespace{{end}}).Get({{ .contextTODO|raw }}(), name, {{ .metav1GetOptions|raw }}{
		// TODO(mattmoor): Incorporate resourceVersion bounds based on staleness criteria.
	})
}
`
