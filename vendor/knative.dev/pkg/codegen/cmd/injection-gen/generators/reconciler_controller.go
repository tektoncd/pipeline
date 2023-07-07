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
	"k8s.io/klog/v2"
)

// reconcilerControllerGenerator produces a file for setting up the reconciler
// with injection.
type reconcilerControllerGenerator struct {
	generator.DefaultGen
	outputPackage  string
	imports        namer.ImportTracker
	typeToGenerate *types.Type

	groupName           string
	clientPkg           string
	schemePkg           string
	informerPackagePath string

	reconcilerClasses  []string
	hasReconcilerClass bool
	hasStatus          bool
}

var _ generator.Generator = (*reconcilerControllerGenerator)(nil)

func (g *reconcilerControllerGenerator) Filter(c *generator.Context, t *types.Type) bool {
	// Only process the type for this generator.
	return t == g.typeToGenerate
}

func (g *reconcilerControllerGenerator) Namers(c *generator.Context) namer.NameSystems {
	return namer.NameSystems{
		"raw": namer.NewRawNamer(g.outputPackage, g.imports),
	}
}

func (g *reconcilerControllerGenerator) Imports(c *generator.Context) (imports []string) {
	imports = append(imports, g.imports.ImportLines()...)
	return
}

func (g *reconcilerControllerGenerator) GenerateType(c *generator.Context, t *types.Type, w io.Writer) error {
	sw := generator.NewSnippetWriter(w, c, "{{", "}}")

	klog.V(5).Info("processing type ", t)

	m := map[string]interface{}{
		"type":      t,
		"group":     g.groupName,
		"classes":   g.reconcilerClasses,
		"hasClass":  g.hasReconcilerClass,
		"hasStatus": g.hasStatus,
		"controllerImpl": c.Universe.Type(types.Name{
			Package: "knative.dev/pkg/controller",
			Name:    "Impl",
		}),
		"controllerReconciler": c.Universe.Type(types.Name{
			Package: "knative.dev/pkg/controller",
			Name:    "Reconciler",
		}),
		"controllerNewContext": c.Universe.Function(types.Name{
			Package: "knative.dev/pkg/controller",
			Name:    "NewContext",
		}),
		"loggingFromContext": c.Universe.Function(types.Name{
			Package: "knative.dev/pkg/logging",
			Name:    "FromContext",
		}),
		"ptrString": c.Universe.Function(types.Name{
			Package: "knative.dev/pkg/ptr",
			Name:    "String",
		}),
		"corev1EventSource": c.Universe.Function(types.Name{
			Package: "k8s.io/api/core/v1",
			Name:    "EventSource",
		}),
		"clientGet": c.Universe.Function(types.Name{
			Package: g.clientPkg,
			Name:    "Get",
		}),
		"informerGet": c.Universe.Function(types.Name{
			Package: g.informerPackagePath,
			Name:    "Get",
		}),
		"schemeScheme": c.Universe.Function(types.Name{
			Package: "k8s.io/client-go/kubernetes/scheme",
			Name:    "Scheme",
		}),
		"schemeAddToScheme": c.Universe.Function(types.Name{
			Package: g.schemePkg,
			Name:    "AddToScheme",
		}),
		"kubeclientGet": c.Universe.Function(types.Name{
			Package: "knative.dev/pkg/client/injection/kube/client",
			Name:    "Get",
		}),
		"typedcorev1EventSinkImpl": c.Universe.Function(types.Name{
			Package: "k8s.io/client-go/kubernetes/typed/core/v1",
			Name:    "EventSinkImpl",
		}),
		"recordNewBroadcaster": c.Universe.Function(types.Name{
			Package: "k8s.io/client-go/tools/record",
			Name:    "NewBroadcaster",
		}),
		"watchInterface": c.Universe.Type(types.Name{
			Package: "k8s.io/apimachinery/pkg/watch",
			Name:    "Interface",
		}),
		"controllerGetEventRecorder": c.Universe.Function(types.Name{
			Package: "knative.dev/pkg/controller",
			Name:    "GetEventRecorder",
		}),
		"controllerOptions": c.Universe.Type(types.Name{
			Package: "knative.dev/pkg/controller",
			Name:    "ControllerOptions",
		}),
		"controllerOptionsFn": c.Universe.Type(types.Name{
			Package: "knative.dev/pkg/controller",
			Name:    "OptionsFn",
		}),
		"contextContext": c.Universe.Type(types.Name{
			Package: "context",
			Name:    "Context",
		}),
		"reconcilerLeaderAwareFuncs": c.Universe.Type(types.Name{
			Package: "knative.dev/pkg/reconciler",
			Name:    "LeaderAwareFuncs",
		}),
		"reconcilerBucket": c.Universe.Type(types.Name{
			Package: "knative.dev/pkg/reconciler",
			Name:    "Bucket",
		}),
		"typesNamespacedName": c.Universe.Type(types.Name{
			Package: "k8s.io/apimachinery/pkg/types",
			Name:    "NamespacedName",
		}),
		"labelsEverything": c.Universe.Function(types.Name{
			Package: "k8s.io/apimachinery/pkg/labels",
			Name:    "Everything",
		}),
		"stringsReplaceAll": c.Universe.Function(types.Name{
			Package: "strings",
			Name:    "ReplaceAll",
		}),
		"reflectTypeOf": c.Universe.Function(types.Name{
			Package: "reflect",
			Name:    "TypeOf",
		}),
		"fmtSprintf": c.Universe.Function(types.Name{
			Package: "fmt",
			Name:    "Sprintf",
		}),
		"logkeyControllerType": c.Universe.Constant(types.Name{
			Package: "knative.dev/pkg/logging/logkey",
			Name:    "ControllerType",
		}),
		"logkeyControllerKind": c.Universe.Constant(types.Name{
			Package: "knative.dev/pkg/logging/logkey",
			Name:    "Kind",
		}),
		"zapString": c.Universe.Function(types.Name{
			Package: "go.uber.org/zap",
			Name:    "String",
		}),
	}

	sw.Do(reconcilerControllerNewImpl, m)

	return sw.Error()
}

var reconcilerControllerNewImpl = `
const (
	defaultControllerAgentName = "{{.type|lowercaseSingular}}-controller"
	defaultFinalizerName       = "{{.type|allLowercasePlural}}.{{.group}}"
	{{if .hasClass }}
	// ClassAnnotationKey points to the annotation for the class of this resource.
	{{if gt (.classes | len) 1 }}// Deprecated: Use ClassAnnotationKeys given multiple keys exist
	{{end -}}
	ClassAnnotationKey = "{{ index .classes 0 }}"
	{{end}}
)

{{if gt (.classes | len) 1 }}
var (
	// ClassAnnotationKeys points to the annotation for the class of this resource.
	ClassAnnotationKeys = []string{
		{{range $class := .classes}}"{{$class}}",
		{{end}}
	}
)
{{end}}

// NewImpl returns a {{.controllerImpl|raw}} that handles queuing and feeding work from
// the queue through an implementation of {{.controllerReconciler|raw}}, delegating to
// the provided Interface and optional Finalizer methods. OptionsFn is used to return
// {{.controllerOptions|raw}} to be used by the internal reconciler.
func NewImpl(ctx {{.contextContext|raw}}, r Interface{{if .hasClass}}, classValue string{{end}}, optionsFns ...{{.controllerOptionsFn|raw}}) *{{.controllerImpl|raw}} {
	logger := {{.loggingFromContext|raw}}(ctx)

	// Check the options function input. It should be 0 or 1.
	if len(optionsFns) > 1 {
		logger.Fatal("Up to one options function is supported, found: ", len(optionsFns))
	}

	{{.type|lowercaseSingular}}Informer := {{.informerGet|raw}}(ctx)

	lister := {{.type|lowercaseSingular}}Informer.Lister()

	var promoteFilterFunc func(obj interface{}) bool
	var promoteFunc = func(bkt {{.reconcilerBucket|raw}}) {}

	rec := &reconcilerImpl{
		LeaderAwareFuncs: {{.reconcilerLeaderAwareFuncs|raw}}{
			PromoteFunc: func(bkt {{.reconcilerBucket|raw}}, enq func({{.reconcilerBucket|raw}}, {{.typesNamespacedName|raw}})) error {

				// Signal promotion event
				promoteFunc(bkt)

				all, err := lister.List({{.labelsEverything|raw}}())
				if err != nil {
					return err
				}
				for _, elt := range all {
					if promoteFilterFunc != nil {
						if ok := promoteFilterFunc(elt); !ok {
							continue
						}
					}
					enq(bkt, {{.typesNamespacedName|raw}}{
						Namespace: elt.GetNamespace(),
						Name: elt.GetName(),
					})
				}
				return nil
			},
		},
		Client:  {{.clientGet|raw}}(ctx),
		Lister:  lister,
		reconciler:    r,
		finalizerName: defaultFinalizerName,
		{{if .hasClass}}classValue: classValue,{{end}}
	}

	ctrType := {{.reflectTypeOf|raw}}(r).Elem()
	ctrTypeName := {{.fmtSprintf|raw}}("%s.%s", ctrType.PkgPath(), ctrType.Name())
	ctrTypeName = {{.stringsReplaceAll|raw}}(ctrTypeName, "/", ".")

	logger = logger.With(
			{{.zapString|raw}}({{.logkeyControllerType|raw}}, ctrTypeName),
			{{.zapString|raw}}({{.logkeyControllerKind|raw}}, "{{ printf "%s.%s" .group .type.Name.Name }}"),
	)


	impl := {{.controllerNewContext|raw}}(ctx, rec, {{ .controllerOptions|raw }}{WorkQueueName: ctrTypeName, Logger: logger})
	agentName := defaultControllerAgentName

	// Pass impl to the options. Save any optional results.
	for _, fn := range optionsFns {
		opts := fn(impl)
		if opts.ConfigStore != nil {
			rec.configStore = opts.ConfigStore
		}
		if opts.FinalizerName != "" {
			rec.finalizerName = opts.FinalizerName
		}
		if opts.AgentName != "" {
			agentName = opts.AgentName
		}
		{{- if .hasStatus}}
		if opts.SkipStatusUpdates {
			rec.skipStatusUpdates = true
		}
		{{- end}}
		if opts.DemoteFunc != nil {
			rec.DemoteFunc = opts.DemoteFunc
		}
		if opts.PromoteFilterFunc != nil {
			promoteFilterFunc = opts.PromoteFilterFunc
		}
		if opts.PromoteFunc != nil {
			promoteFunc = opts.PromoteFunc
		}
	}

	rec.Recorder = createRecorder(ctx, agentName)

	return impl
}

func createRecorder(ctx context.Context, agentName string) record.EventRecorder {
	logger := {{.loggingFromContext|raw}}(ctx)

	recorder := {{.controllerGetEventRecorder|raw}}(ctx)
	if recorder == nil {
		// Create event broadcaster
		logger.Debug("Creating event broadcaster")
		eventBroadcaster := {{.recordNewBroadcaster|raw}}()
		watches := []{{.watchInterface|raw}}{
			eventBroadcaster.StartLogging(logger.Named("event-broadcaster").Infof),
			eventBroadcaster.StartRecordingToSink(
				&{{.typedcorev1EventSinkImpl|raw}}{Interface: {{.kubeclientGet|raw}}(ctx).CoreV1().Events("")}),
		}
		recorder = eventBroadcaster.NewRecorder({{.schemeScheme|raw}}, {{.corev1EventSource|raw}}{Component: agentName})
		go func() {
			<-ctx.Done()
			for _, w := range watches {
				w.Stop()
			}
		}()
	}

	return recorder
}

func init() {
	{{.schemeAddToScheme|raw}}({{.schemeScheme|raw}})
}
`
