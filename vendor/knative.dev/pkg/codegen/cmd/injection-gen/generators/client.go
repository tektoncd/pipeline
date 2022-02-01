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
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"

	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/code-generator/cmd/client-gen/generators/util"
	clientgentypes "k8s.io/code-generator/cmd/client-gen/types"
	"k8s.io/gengo/generator"
	"k8s.io/gengo/namer"
	"k8s.io/gengo/types"
	"k8s.io/klog/v2"
)

// clientGenerator produces a file of listers for a given GroupVersion and
// type.
type clientGenerator struct {
	generator.DefaultGen

	groupVersions     map[string]clientgentypes.GroupVersions
	groupGoNames      map[string]string
	groupVersionTypes map[string]map[clientgentypes.Version][]*types.Type

	outputPackage    string
	imports          namer.ImportTracker
	clientSetPackage string
	filtered         bool
}

var _ generator.Generator = (*clientGenerator)(nil)

func (g *clientGenerator) Filter(c *generator.Context, t *types.Type) bool {
	// We generate a single client, so return true once.
	if !g.filtered {
		g.filtered = true
		return true
	}
	return false
}

var publicPluralNamer = &ExceptionNamer{
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

func (g *clientGenerator) Namers(c *generator.Context) namer.NameSystems {
	return namer.NameSystems{
		"raw":          namer.NewRawNamer(g.outputPackage, g.imports),
		"publicPlural": publicPluralNamer,
	}
}

func (g *clientGenerator) Imports(c *generator.Context) (imports []string) {
	imports = append(imports, g.imports.ImportLines()...)
	for gpn, group := range g.groupVersions {
		for _, version := range group.Versions {
			typedClientPath := filepath.Join(g.clientSetPackage, "typed", strings.ToLower(group.PackageName), strings.ToLower(version.NonEmpty()))
			imports = append(imports, fmt.Sprintf("%s \"%s\"", strings.ToLower("typed"+g.groupGoNames[gpn]+version.NonEmpty()), typedClientPath))
		}
	}
	imports = sets.NewString(imports...).List()
	return
}

func (g *clientGenerator) GenerateType(c *generator.Context, t *types.Type, w io.Writer) error {
	sw := generator.NewSnippetWriter(w, c, "{{", "}}")

	klog.V(5).Info("processing type ", t)

	m := map[string]interface{}{
		"clientSetNewForConfigOrDie":     c.Universe.Function(types.Name{Package: g.clientSetPackage, Name: "NewForConfigOrDie"}),
		"clientSetInterface":             c.Universe.Type(types.Name{Package: g.clientSetPackage, Name: "Interface"}),
		"injectionRegisterClient":        c.Universe.Function(types.Name{Package: "knative.dev/pkg/injection", Name: "Default.RegisterClient"}),
		"injectionRegisterDynamicClient": c.Universe.Function(types.Name{Package: "knative.dev/pkg/injection", Name: "Dynamic.RegisterDynamicClient"}),
		"injectionRegisterClientFetcher": c.Universe.Function(types.Name{
			Package: "knative.dev/pkg/injection",
			Name:    "Default.RegisterClientFetcher",
		}),
		"restConfig": c.Universe.Type(types.Name{Package: "k8s.io/client-go/rest", Name: "Config"}),
		"loggingFromContext": c.Universe.Function(types.Name{
			Package: "knative.dev/pkg/logging",
			Name:    "FromContext",
		}),
		"contextContext": c.Universe.Type(types.Name{
			Package: "context",
			Name:    "Context",
		}),
		"dynamicInterface": c.Universe.Type(types.Name{
			Package: "k8s.io/client-go/dynamic",
			Name:    "Interface",
		}),
		"dynamicclientGet": c.Universe.Function(types.Name{
			Package: "knative.dev/pkg/injection/clients/dynamicclient",
			Name:    "Get",
		}),
		"discoveryInterface": c.Universe.Type(types.Name{
			Package: "k8s.io/client-go/discovery",
			Name:    "DiscoveryInterface",
		}),
		"runtimeObject": c.Universe.Type(types.Name{
			Package: "k8s.io/apimachinery/pkg/runtime",
			Name:    "Object",
		}),
		"jsonMarshal": c.Universe.Function(types.Name{
			Package: "encoding/json",
			Name:    "Marshal",
		}),
		"jsonUnmarshal": c.Universe.Function(types.Name{
			Package: "encoding/json",
			Name:    "Unmarshal",
		}),
		"fmtErrorf": c.Universe.Function(types.Name{
			Package: "fmt",
			Name:    "Errorf",
		}),
	}

	sw.Do(injectionClient, m)

	gpns := make(sets.String, len(g.groupGoNames))
	for gpn := range g.groupGoNames {
		gpns.Insert(gpn)
	}

	for _, gpn := range gpns.List() {
		ggn := g.groupGoNames[gpn]
		gv := g.groupVersions[gpn]
		verTypes := g.groupVersionTypes[gpn]
		for _, version := range gv.Versions {
			vts := verTypes[version.Version]
			if len(vts) == 0 {
				// Skip things with zero types.
				continue
			}
			sw.Do(clientsetInterfaceImplTemplate, map[string]interface{}{
				"dynamicInterface": c.Universe.Type(types.Name{
					Package: "k8s.io/client-go/dynamic",
					Name:    "Interface",
				}),
				"restInterface": c.Universe.Type(types.Name{
					Package: "k8s.io/client-go/rest",
					Name:    "Interface",
				}),
				"GroupGoName":  ggn,
				"Version":      namer.IC(version.String()),
				"PackageAlias": strings.ToLower("typed" + g.groupGoNames[gpn] + version.NonEmpty()),
			})
			sort.Slice(vts, func(i, j int) bool {
				lhs, rhs := vts[i], vts[j]
				return lhs.String() < rhs.String()
			})
			for _, t := range vts {
				tags, err := util.ParseClientGenTags(append(t.SecondClosestCommentLines, t.CommentLines...))
				if err != nil {
					return err
				}

				// Don't use the literal group "core", these should be left blank.
				group := gv.Group
				if group == "core" {
					group = ""
				}

				sw.Do(typeImplTemplate, map[string]interface{}{
					"dynamicNamespaceableResourceInterface": c.Universe.Type(types.Name{
						Package: "k8s.io/client-go/dynamic",
						Name:    "NamespaceableResourceInterface",
					}),
					"schemaGroupVersionResource": c.Universe.Type(types.Name{
						Package: "k8s.io/apimachinery/pkg/runtime/schema",
						Name:    "GroupVersionResource",
					}),
					"contextContext": c.Universe.Type(types.Name{
						Package: "context",
						Name:    "Context",
					}),
					"GroupGoName":  ggn,
					"Version":      namer.IC(version.String()),
					"PackageAlias": strings.ToLower("typed" + g.groupGoNames[gpn] + version.NonEmpty()),
					"Type":         t,
					"Namespaced":   !tags.NonNamespaced,
					"Group":        group,
					"VersionLower": version,
					"Resource":     strings.ToLower(publicPluralNamer.Name(t)),
				})
				opts := map[string]interface{}{
					"contextContext": c.Universe.Type(types.Name{
						Package: "context",
						Name:    "Context",
					}),
					"unstructuredUnstructured": c.Universe.Type(types.Name{
						Package: "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured",
						Name:    "Unstructured",
					}),
					"metav1CreateOptions": c.Universe.Type(types.Name{
						Package: "k8s.io/apimachinery/pkg/apis/meta/v1",
						Name:    "CreateOptions",
					}),
					"metav1UpdateOptions": c.Universe.Type(types.Name{
						Package: "k8s.io/apimachinery/pkg/apis/meta/v1",
						Name:    "UpdateOptions",
					}),
					"metav1GetOptions": c.Universe.Type(types.Name{
						Package: "k8s.io/apimachinery/pkg/apis/meta/v1",
						Name:    "GetOptions",
					}),
					"metav1ListOptions": c.Universe.Type(types.Name{
						Package: "k8s.io/apimachinery/pkg/apis/meta/v1",
						Name:    "ListOptions",
					}),
					"metav1DeleteOptions": c.Universe.Type(types.Name{
						Package: "k8s.io/apimachinery/pkg/apis/meta/v1",
						Name:    "DeleteOptions",
					}),
					"metav1PatchOptions": c.Universe.Type(types.Name{
						Package: "k8s.io/apimachinery/pkg/apis/meta/v1",
						Name:    "PatchOptions",
					}),
					"metav1ApplyOptions": c.Universe.Type(types.Name{
						Package: "k8s.io/apimachinery/pkg/apis/meta/v1",
						Name:    "ApplyOptions",
					}),
					"typesPatchType": c.Universe.Type(types.Name{
						Package: "k8s.io/apimachinery/pkg/types",
						Name:    "PatchType",
					}),
					"watchInterface": c.Universe.Type(types.Name{
						Package: "k8s.io/apimachinery/pkg/watch",
						Name:    "Interface",
					}),
					"errorsNew": c.Universe.Function(types.Name{
						Package: "errors",
						Name:    "New",
					}),
					"GroupGoName": ggn,
					"Version":     namer.IC(version.String()),
					"Type":        t,
					"InputType":   t,
					"ResultType":  t,

					// TODO: Total hacks to get this to run at all.
					"ApplyType": c.Universe.Type(types.Name{
						Package: "k8s.io/client-go/applyconfigurations/" + strings.ReplaceAll(t.Name.Package, "k8s.io/api/", ""),
						Name:    t.Name.Name + "ApplyConfiguration",
					}),
					"generateApply": strings.HasPrefix(t.Name.Package, "k8s.io/api/"),

					"Namespaced":  !tags.NonNamespaced,
					"Subresource": "",
					"schemaGroupVersionKind": c.Universe.Type(types.Name{
						Package: "k8s.io/apimachinery/pkg/runtime/schema",
						Name:    "GroupVersionKind",
					}),
					"Group":        group,
					"VersionLower": version,
					"Kind":         t.Name.Name,
				}

				for _, v := range verbs.List() {
					tmpl := verbMap[v]
					if tags.NoVerbs || !tags.HasVerb(v) {
						continue
					}
					sw.Do(tmpl, opts)
				}
				for _, e := range tags.Extensions {
					for _, v := range extensionVerbs.List() {
						tmpl := extensionVerbMap[v]
						if !e.HasVerb(v) {
							continue
						}
						inputType := *t
						resultType := *t
						if len(e.InputTypeOverride) > 0 {
							if name, pkg := e.Input(); len(pkg) > 0 {
								// _, inputGVString = util.ParsePathGroupVersion(pkg)
								newType := c.Universe.Type(types.Name{Package: pkg, Name: name})
								inputType = *newType
							} else {
								inputType.Name.Name = e.InputTypeOverride
							}
						}
						if len(e.ResultTypeOverride) > 0 {
							if name, pkg := e.Result(); len(pkg) > 0 {
								newType := c.Universe.Type(types.Name{Package: pkg, Name: name})
								resultType = *newType
							} else {
								resultType.Name.Name = e.ResultTypeOverride
							}
						}
						// TODO: Total hacks to get this to run at all.
						if v == "apply" {
							inputType = *(c.Universe.Type(types.Name{
								Package: "k8s.io/client-go/applyconfigurations/" + strings.ReplaceAll(inputType.Name.Package, "k8s.io/api/", ""),
								Name:    inputType.Name.Name + "ApplyConfiguration",
							}))
						}
						opts["InputType"] = &inputType
						opts["ResultType"] = &resultType
						if e.IsSubresource() {
							opts["Subresource"] = e.SubResourcePath
						}
						sw.Do(strings.Replace(tmpl, " "+strings.Title(e.VerbType), " "+e.VerbName, -1), opts)
					}
				}
			}
		}
	}

	return sw.Error()
}

var injectionClient = `
func init() {
	{{.injectionRegisterClient|raw}}(withClientFromConfig)
	{{.injectionRegisterClientFetcher|raw}}(func(ctx context.Context) interface{} {
		return Get(ctx)
	})
	{{.injectionRegisterDynamicClient|raw}}(withClientFromDynamic)
}

// Key is used as the key for associating information with a context.Context.
type Key struct{}

func withClientFromConfig(ctx {{.contextContext|raw}}, cfg *{{.restConfig|raw}}) context.Context {
	return context.WithValue(ctx, Key{}, {{.clientSetNewForConfigOrDie|raw}}(cfg))
}

func withClientFromDynamic(ctx {{.contextContext|raw}}) context.Context {
	return context.WithValue(ctx, Key{}, &wrapClient{dyn: {{.dynamicclientGet|raw}}(ctx)})
}

// Get extracts the {{.clientSetInterface|raw}} client from the context.
func Get(ctx {{.contextContext|raw}}) {{.clientSetInterface|raw}} {
	untyped := ctx.Value(Key{})
	if untyped == nil {
		if injection.GetConfig(ctx) == nil {
		    {{.loggingFromContext|raw}}(ctx).Panic(
		    	    "Unable to fetch {{.clientSetInterface}} from context. This context is not the application context (which is typically given to constructors via sharedmain).")
		} else {
		    {{.loggingFromContext|raw}}(ctx).Panic(
		    	    "Unable to fetch {{.clientSetInterface}} from context.")
		}
	}
	return untyped.({{.clientSetInterface|raw}})
}

type wrapClient struct {
	dyn {{.dynamicInterface|raw}}
}

var _ {{.clientSetInterface|raw}} = (*wrapClient)(nil)

func (w *wrapClient) Discovery() {{.discoveryInterface|raw}} {
	panic("Discovery called on dynamic client!")
}


func convert(from interface{}, to {{ .runtimeObject|raw }}) error {
	bs, err := {{ .jsonMarshal|raw }}(from)
	if err != nil {
		return {{ .fmtErrorf|raw }}("Marshal() = %w", err)
	}
	if err := {{ .jsonUnmarshal|raw }}(bs, to); err != nil {
		return {{ .fmtErrorf|raw }}("Unmarshal() = %w", err)
	}
	return nil
}

`

var clientsetInterfaceImplTemplate = `
// {{.GroupGoName}}{{.Version}} retrieves the {{.GroupGoName}}{{.Version}}Client
func (w *wrapClient) {{.GroupGoName}}{{.Version}}() {{.PackageAlias}}.{{.GroupGoName}}{{.Version}}Interface {
	return &wrap{{.GroupGoName}}{{.Version}}{
		dyn: w.dyn,
	}
}

type wrap{{.GroupGoName}}{{.Version}} struct {
	dyn {{.dynamicInterface|raw}}
}

func (w *wrap{{.GroupGoName}}{{.Version}}) RESTClient() {{.restInterface|raw}} {
	panic("RESTClient called on dynamic client!")
}

`

var typeImplTemplate = `
func (w *wrap{{ .GroupGoName }}{{ .Version }}) {{ .Type|publicPlural }}({{if .Namespaced}}namespace string{{end}}) {{ .PackageAlias }}.{{ .Type.Name.Name }}Interface {
	return &wrap{{.GroupGoName}}{{.Version}}{{ .Type.Name.Name }}Impl{
		dyn: w.dyn.Resource({{ .schemaGroupVersionResource|raw }}{
			Group: "{{ .Group }}",
			Version: "{{ .VersionLower }}",
			Resource: "{{ .Resource }}",
		}),
{{if .Namespaced}}
		namespace: namespace,
{{end}}
	}
}

type wrap{{.GroupGoName}}{{.Version}}{{ .Type.Name.Name }}Impl struct {
	dyn {{.dynamicNamespaceableResourceInterface|raw}}
{{if .Namespaced}}
    namespace string
{{end}}}

var _ {{ .PackageAlias }}.{{ .Type.Name.Name }}Interface = (*wrap{{.GroupGoName}}{{.Version}}{{ .Type.Name.Name }}Impl)(nil)

`

var verbs = sets.NewString( /* Populated from verbMap during init */ )
var verbMap = map[string]string{
	"create": `
func (w *wrap{{.GroupGoName}}{{.Version}}{{ .Type.Name.Name }}Impl) Create(ctx {{ .contextContext|raw }}, {{ if .Subresource }}_ string, {{ end }}in *{{ .InputType|raw }}, opts {{ .metav1CreateOptions|raw }}) (*{{ .ResultType|raw }}, error) {
	in.SetGroupVersionKind({{ .schemaGroupVersionKind|raw }}{
		Group: "{{ .Group }}",
		Version: "{{ .VersionLower }}",
		Kind: "{{ .Kind }}",
	})
	uo := &{{ .unstructuredUnstructured|raw }}{}
	if err := convert(in, uo); err != nil {
		return nil, err
	}
	uo, err := w.dyn{{if .Namespaced}}{{if .Namespaced}}.Namespace(w.namespace){{end}}{{end}}.Create(ctx, uo, opts)
	if err != nil {
		return nil, err
	}
	out := &{{ .ResultType|raw }}{}
	if err := convert(uo, out); err != nil {
		return nil, err
	}
	return out, nil
}
`,

	"update": `
func (w *wrap{{.GroupGoName}}{{.Version}}{{ .Type.Name.Name }}Impl) Update(ctx {{ .contextContext|raw }}, {{ if .Subresource }}_ string, {{ end }}in *{{ .InputType|raw }}, opts {{ .metav1UpdateOptions|raw }}) (*{{ .ResultType|raw }}, error) {
	in.SetGroupVersionKind({{ .schemaGroupVersionKind|raw }}{
		Group: "{{ .Group }}",
		Version: "{{ .VersionLower }}",
		Kind: "{{ .Kind }}",
	})
	uo := &{{ .unstructuredUnstructured|raw }}{}
	if err := convert(in, uo); err != nil {
		return nil, err
	}
	uo, err := w.dyn{{if .Namespaced}}.Namespace(w.namespace){{end}}.Update(ctx, uo, opts)
	if err != nil {
		return nil, err
	}
	out := &{{ .ResultType|raw }}{}
	if err := convert(uo, out); err != nil {
		return nil, err
	}
	return out, nil
}
`,

	"updateStatus": `
func (w *wrap{{.GroupGoName}}{{.Version}}{{ .Type.Name.Name }}Impl) UpdateStatus(ctx {{ .contextContext|raw }}, in *{{ .InputType|raw }}, opts {{ .metav1UpdateOptions|raw }}) (*{{ .ResultType|raw }}, error) {
	in.SetGroupVersionKind({{ .schemaGroupVersionKind|raw }}{
		Group: "{{ .Group }}",
		Version: "{{ .VersionLower }}",
		Kind: "{{ .Kind }}",
	})
	uo := &{{ .unstructuredUnstructured|raw }}{}
	if err := convert(in, uo); err != nil {
		return nil, err
	}
	uo, err := w.dyn{{if .Namespaced}}.Namespace(w.namespace){{end}}.UpdateStatus(ctx, uo, opts)
	if err != nil {
		return nil, err
	}
	out := &{{ .ResultType|raw }}{}
	if err := convert(uo, out); err != nil {
		return nil, err
	}
	return out, nil
}
`,

	"delete": `
func (w *wrap{{.GroupGoName}}{{.Version}}{{ .Type.Name.Name }}Impl) Delete(ctx {{ .contextContext|raw }}, name string, opts {{ .metav1DeleteOptions|raw }}) error {
	return w.dyn{{if .Namespaced}}.Namespace(w.namespace){{end}}.Delete(ctx, name, opts)
}
`,

	"deleteCollection": `
func (w *wrap{{.GroupGoName}}{{.Version}}{{ .Type.Name.Name }}Impl) DeleteCollection(ctx {{ .contextContext|raw }}, opts {{ .metav1DeleteOptions|raw }}, listOpts {{ .metav1ListOptions|raw }}) error {
	return w.dyn{{if .Namespaced}}.Namespace(w.namespace){{end}}.DeleteCollection(ctx, opts, listOpts)
}
`,

	"get": `
func (w *wrap{{.GroupGoName}}{{.Version}}{{ .Type.Name.Name }}Impl) Get(ctx {{ .contextContext|raw }}, name string, opts {{ .metav1GetOptions|raw }}) (*{{ .ResultType|raw }}, error) {
	uo, err := w.dyn{{if .Namespaced}}.Namespace(w.namespace){{end}}.Get(ctx, name, opts)
	if err != nil {
		return nil, err
	}
	out := &{{ .ResultType|raw }}{}
	if err := convert(uo, out); err != nil {
		return nil, err
	}
	return out, nil
}
`,

	"list": `
func (w *wrap{{.GroupGoName}}{{.Version}}{{ .Type.Name.Name }}Impl) List(ctx {{ .contextContext|raw }}, opts {{ .metav1ListOptions|raw }}) (*{{ .ResultType|raw }}List, error) {
	uo, err := w.dyn{{if .Namespaced}}.Namespace(w.namespace){{end}}.List(ctx, opts)
	if err != nil {
		return nil, err
	}
	out := &{{ .ResultType|raw }}List{}
	if err := convert(uo, out); err != nil {
		return nil, err
	}
	return out, nil
}
`,

	"watch": `
func (w *wrap{{.GroupGoName}}{{.Version}}{{ .Type.Name.Name }}Impl) Watch(ctx {{ .contextContext|raw }}, opts {{ .metav1ListOptions|raw }}) ({{ .watchInterface|raw }}, error) {
	return nil, {{ .errorsNew|raw }}("NYI: Watch")
}
`,
	"patch": `
func (w *wrap{{.GroupGoName}}{{.Version}}{{ .Type.Name.Name }}Impl) Patch(ctx {{ .contextContext|raw }}, name string, pt {{ .typesPatchType|raw }}, data []byte, opts {{ .metav1PatchOptions|raw }}, subresources ...string) (result *{{ .ResultType|raw }}, err error) {
	uo, err := w.dyn{{if .Namespaced}}.Namespace(w.namespace){{end}}.Patch(ctx, name, pt, data, opts)
	if err != nil {
		return nil, err
	}
	out := &{{ .ResultType|raw }}{}
	if err := convert(uo, out); err != nil {
		return nil, err
	}
	return out, nil
}
`,
	"apply": `{{if .generateApply}}
func (w *wrap{{.GroupGoName}}{{.Version}}{{ .Type.Name.Name }}Impl) Apply(ctx {{ .contextContext|raw }}, in *{{ .ApplyType|raw }}, opts {{ .metav1ApplyOptions|raw }}) (result *{{ .ResultType|raw }}, err error) {
	panic("NYI")
}
{{end}}
`,
	"applyStatus": `{{if .generateApply}}
func (w *wrap{{.GroupGoName}}{{.Version}}{{ .Type.Name.Name }}Impl) ApplyStatus(ctx {{ .contextContext|raw }}, in *{{ .ApplyType|raw }}, opts {{ .metav1ApplyOptions|raw }}) (result *{{ .ResultType|raw }}, err error) {
	panic("NYI")
}
{{end}}
`,
}

var extensionVerbs = sets.NewString( /* Populated from extensionVerbMap during init */ )
var extensionVerbMap = map[string]string{
	"apply": `
func (w *wrap{{.GroupGoName}}{{.Version}}{{ .Type.Name.Name }}Impl) Apply(ctx {{ .contextContext|raw }}, name string, in *{{ .InputType|raw }}, opts {{ .metav1ApplyOptions|raw }}) (*{{ .ResultType|raw }}, error) {
	panic("NYI")
}
`,
	"create": `
func (w *wrap{{.GroupGoName}}{{.Version}}{{ .Type.Name.Name }}Impl) Create(ctx {{ .contextContext|raw }}, {{ if .Subresource }}_ string, {{ end }}in *{{ .InputType|raw }}, opts {{ .metav1CreateOptions|raw }}) (*{{ .ResultType|raw }}, error) {
	panic("NYI")
}
`,
	"update": `
func (w *wrap{{.GroupGoName}}{{.Version}}{{ .Type.Name.Name }}Impl) Update(ctx {{ .contextContext|raw }}, {{ if .Subresource }}_ string, {{ end }}in *{{ .InputType|raw }}, opts {{ .metav1UpdateOptions|raw }}) (*{{ .ResultType|raw }}, error) {
	panic("NYI")
}
`,
	"updateStatus": `
func (w *wrap{{.GroupGoName}}{{.Version}}{{ .Type.Name.Name }}Impl) UpdateStatus(ctx {{ .contextContext|raw }}, in *{{ .InputType|raw }}, opts {{ .metav1UpdateOptions|raw }}) (*{{ .ResultType|raw }}, error) {
	panic("NYI")
}
`,
	"delete": `
func (w *wrap{{.GroupGoName}}{{.Version}}{{ .Type.Name.Name }}Impl) Delete(ctx {{ .contextContext|raw }}, name string, opts {{ .metav1DeleteOptions|raw }}) error {
	panic("NYI")
}
`,
	"deleteCollection": `
func (w *wrap{{.GroupGoName}}{{.Version}}{{ .Type.Name.Name }}Impl) DeleteCollection(ctx {{ .contextContext|raw }}, opts {{ .metav1DeleteOptions|raw }}, listOpts {{ .metav1ListOptions|raw }}) error {
	panic("NYI")
}
`,
	"get": `
func (w *wrap{{.GroupGoName}}{{.Version}}{{ .Type.Name.Name }}Impl) Get(ctx {{ .contextContext|raw }}, name string, opts {{ .metav1GetOptions|raw }}) (*{{ .ResultType|raw }}, error) {
	panic("NYI")
}
`,
	"list": `
func (w *wrap{{.GroupGoName}}{{.Version}}{{ .Type.Name.Name }}Impl) List(ctx {{ .contextContext|raw }}, opts {{ .metav1ListOptions|raw }}) (*{{ .ResultType|raw }}List, error) {
	panic("NYI")
}
`,
	"watch": `
func (w *wrap{{.GroupGoName}}{{.Version}}{{ .Type.Name.Name }}Impl) Watch(ctx {{ .contextContext|raw }}, opts {{ .metav1ListOptions|raw }}) ({{ .watchInterface|raw }}, error) {
	panic("NYI")
}
`,
	"patch": `
func (w *wrap{{.GroupGoName}}{{.Version}}{{ .Type.Name.Name }}Impl) Patch(ctx {{ .contextContext|raw }}, name string, pt {{ .typesPatchType|raw }}, data []byte, opts {{ .metav1PatchOptions|raw }}, subresources ...string) (result *{{ .ResultType|raw }}, err error) {
	panic("NYI")
}
`,
}

func init() {
	for v := range verbMap {
		verbs.Insert(v)
	}
	for ev := range extensionVerbMap {
		extensionVerbs.Insert(ev)
	}
}
