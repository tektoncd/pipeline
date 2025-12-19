/*
Copyright 2020 The Knative Authors

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

package reconciler

import (
	"io"

	clientgentypes "k8s.io/code-generator/cmd/client-gen/types"

	"k8s.io/gengo/v2/generator"
	"k8s.io/gengo/v2/namer"
	"k8s.io/gengo/v2/types"
	"k8s.io/klog/v2"
)

// reconcilerReconcilerGenerator produces a reconciler struct for the given type.
type reconcilerReconcilerGenerator struct {
	generator.GoGenerator
	outputPackage  string
	imports        namer.ImportTracker
	typeToGenerate *types.Type
	clientsetPkg   string
	listerName     string
	listerPkg      string

	reconcilerClasses  []string
	hasReconcilerClass bool
	nonNamespaced      bool
	isKRShaped         bool
	hasStatus          bool

	groupGoName  string
	groupVersion clientgentypes.GroupVersion
}

var _ generator.Generator = (*reconcilerReconcilerGenerator)(nil)

func (g *reconcilerReconcilerGenerator) Filter(c *generator.Context, t *types.Type) bool {
	// Only process the type for this generator.
	return t == g.typeToGenerate
}

func (g *reconcilerReconcilerGenerator) Namers(c *generator.Context) namer.NameSystems {
	return namer.NameSystems{
		"raw": namer.NewRawNamer(g.outputPackage, g.imports),
	}
}

func (g *reconcilerReconcilerGenerator) Imports(c *generator.Context) (imports []string) {
	imports = append(imports, g.imports.ImportLines()...)
	return imports
}

func (g *reconcilerReconcilerGenerator) GenerateType(c *generator.Context, t *types.Type, w io.Writer) error {
	sw := generator.NewSnippetWriter(w, c, "{{", "}}")

	klog.V(5).Info("processing type ", t)

	m := map[string]interface{}{
		"type":          t,
		"group":         namer.IC(g.groupGoName),
		"version":       namer.IC(g.groupVersion.Version.String()),
		"classes":       g.reconcilerClasses,
		"hasClass":      g.hasReconcilerClass,
		"isKRShaped":    g.isKRShaped,
		"hasStatus":     g.hasStatus,
		"nonNamespaced": g.nonNamespaced,
		"controllerImpl": c.Universe.Type(types.Name{
			Package: "knative.dev/pkg/controller",
			Name:    "Impl",
		}),
		"controllerReconciler": c.Universe.Type(types.Name{
			Package: "knative.dev/pkg/controller",
			Name:    "Reconciler",
		}),
		"controllerWithEventRecorder": c.Universe.Type(types.Name{
			Package: "knative.dev/pkg/controller",
			Name:    "WithEventRecorder",
		}),
		"controllerNewSkipKey": c.Universe.Type(types.Name{
			Package: "knative.dev/pkg/controller",
			Name:    "NewSkipKey",
		}),
		"corev1EventSource": c.Universe.Function(types.Name{
			Package: "k8s.io/api/core/v1",
			Name:    "EventSource",
		}),
		"corev1EventTypeNormal": c.Universe.Type(types.Name{
			Package: "k8s.io/api/core/v1",
			Name:    "EventTypeNormal",
		}),
		"corev1EventTypeWarning": c.Universe.Type(types.Name{
			Package: "k8s.io/api/core/v1",
			Name:    "EventTypeWarning",
		}),
		"reconcilerEvent":                c.Universe.Type(types.Name{Package: "knative.dev/pkg/reconciler", Name: "Event"}),
		"reconcilerReconcilerEvent":      c.Universe.Type(types.Name{Package: "knative.dev/pkg/reconciler", Name: "ReconcilerEvent"}),
		"reconcilerRetryUpdateConflicts": c.Universe.Function(types.Name{Package: "knative.dev/pkg/reconciler", Name: "RetryUpdateConflicts"}),
		"reconcilerConfigStore":          c.Universe.Type(types.Name{Name: "ConfigStore", Package: "knative.dev/pkg/reconciler"}),
		"reconcilerOnDeletionInterface":  c.Universe.Type(types.Name{Package: "knative.dev/pkg/reconciler", Name: "OnDeletionInterface"}),
		// Deps
		"clientsetInterface": c.Universe.Type(types.Name{Name: "Interface", Package: g.clientsetPkg}),
		"resourceLister":     c.Universe.Type(types.Name{Name: g.listerName, Package: g.listerPkg}),
		// K8s types
		"recordEventRecorder": c.Universe.Type(types.Name{Name: "EventRecorder", Package: "k8s.io/client-go/tools/record"}),
		// methods
		"loggingFromContext": c.Universe.Function(types.Name{
			Package: "knative.dev/pkg/logging",
			Name:    "FromContext",
		}),
		"cacheSplitMetaNamespaceKey": c.Universe.Function(types.Name{
			Package: "k8s.io/client-go/tools/cache",
			Name:    "SplitMetaNamespaceKey",
		}),
		"retryRetryOnConflict": c.Universe.Function(types.Name{
			Package: "k8s.io/client-go/util/retry",
			Name:    "RetryOnConflict",
		}),
		"apierrsIsNotFound": c.Universe.Function(types.Name{
			Package: "k8s.io/apimachinery/pkg/api/errors",
			Name:    "IsNotFound",
		}),
		"apierrsIsConflict": c.Universe.Function(types.Name{
			Package: "k8s.io/apimachinery/pkg/api/errors",
			Name:    "IsConflict",
		}),
		"metav1GetOptions": c.Universe.Function(types.Name{
			Package: "k8s.io/apimachinery/pkg/apis/meta/v1",
			Name:    "GetOptions",
		}),
		"metav1PatchOptions": c.Universe.Function(types.Name{
			Package: "k8s.io/apimachinery/pkg/apis/meta/v1",
			Name:    "PatchOptions",
		}),
		"metav1UpdateOptions": c.Universe.Function(types.Name{
			Package: "k8s.io/apimachinery/pkg/apis/meta/v1",
			Name:    "UpdateOptions",
		}),
		"zapSugaredLogger": c.Universe.Type(types.Name{
			Package: "go.uber.org/zap",
			Name:    "SugaredLogger",
		}),
		"zapDebugLevel": c.Universe.Type(types.Name{
			Package: "go.uber.org/zap/zapcore",
			Name:    "DebugLevel",
		}),
		"setsNewString": c.Universe.Function(types.Name{
			Package: "k8s.io/apimachinery/pkg/util/sets",
			Name:    "New[string]",
		}),
		"setsString": c.Universe.Type(types.Name{
			Package: "k8s.io/apimachinery/pkg/util/sets",
			Name:    "Set[string]",
		}),
		"controllerOptions": c.Universe.Type(types.Name{
			Package: "knative.dev/pkg/controller",
			Name:    "Options",
		}),
		"contextContext": c.Universe.Type(types.Name{
			Package: "context",
			Name:    "Context",
		}),
		"kmpSafeDiff": c.Universe.Function(types.Name{
			Package: "knative.dev/pkg/kmp",
			Name:    "SafeDiff",
		}),
		"fmtErrorf":           c.Universe.Package("fmt").Function("Errorf"),
		"equalitySemantic":    c.Universe.Package("k8s.io/apimachinery/pkg/api/equality").Variable("Semantic"),
		"jsonMarshal":         c.Universe.Package("encoding/json").Function("Marshal"),
		"typesMergePatchType": c.Universe.Package("k8s.io/apimachinery/pkg/types").Constant("MergePatchType"),
		"typesApplyPatchType": c.Universe.Package("k8s.io/apimachinery/pkg/types").Constant("ApplyPatchType"),
		"schemeScheme": c.Universe.Function(types.Name{
			Package: "k8s.io/client-go/kubernetes/scheme",
			Name:    "Scheme",
		}),
		"syncRWMutex": c.Universe.Type(types.Name{
			Package: "sync",
			Name:    "RWMutex",
		}),
		"reconcilerLeaderAware": c.Universe.Type(types.Name{
			Package: "knative.dev/pkg/reconciler",
			Name:    "LeaderAware",
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
		"doReconcileKind": c.Universe.Type(types.Name{
			Package: "knative.dev/pkg/reconciler",
			Name:    "DoReconcileKind",
		}),
		"doObserveKind": c.Universe.Type(types.Name{
			Package: "knative.dev/pkg/reconciler",
			Name:    "DoObserveKind",
		}),
		"doFinalizeKind": c.Universe.Type(types.Name{
			Package: "knative.dev/pkg/reconciler",
			Name:    "DoFinalizeKind",
		}),
		"controllerIsSkipKey": c.Universe.Function(types.Name{
			Package: "knative.dev/pkg/controller",
			Name:    "IsSkipKey",
		}),
		"controllerIsRequeueKey": c.Universe.Function(types.Name{
			Package: "knative.dev/pkg/controller",
			Name:    "IsRequeueKey",
		}),
	}

	sw.Do(reconcilerInterfaceFactory, m)
	sw.Do(reconcilerNewReconciler, m)
	sw.Do(reconcilerImplFactory, m)
	if len(g.reconcilerClasses) > 1 {
		sw.Do(reconcilerLookupClass, m)
	}
	if g.hasStatus {
		sw.Do(reconcilerStatusFactory, m)
	}
	sw.Do(reconcilerFinalizerFactory, m)

	return sw.Error()
}

var reconcilerLookupClass = `
func lookupClass(annotations map[string]string) (string, bool) {
	for _, key := range ClassAnnotationKeys {
		 if val, ok := annotations[key]; ok {
		   return val, true
		 }
	}
	return "", false
}
`

var reconcilerInterfaceFactory = `
// Interface defines the strongly typed interfaces to be implemented by a
// controller reconciling {{.type|raw}}.
type Interface interface {
	// ReconcileKind implements custom logic to reconcile {{.type|raw}}. Any changes
	// to the objects .Status or .Finalizers will be propagated to the stored
	// object. It is recommended that implementors do not call any update calls
	// for the Kind inside of ReconcileKind, it is the responsibility of the calling
	// controller to propagate those properties. The resource passed to ReconcileKind
 	// will always have an empty deletion timestamp.
	ReconcileKind(ctx {{.contextContext|raw}}, o *{{.type|raw}}) {{.reconcilerEvent|raw}}
}

// Finalizer defines the strongly typed interfaces to be implemented by a
// controller finalizing {{.type|raw}}.
type Finalizer interface {
	// FinalizeKind implements custom logic to finalize {{.type|raw}}. Any changes
	// to the objects .Status or .Finalizers will be ignored. Returning a nil or
	// Normal type {{.reconcilerEvent|raw}} will allow the finalizer to be deleted on
	// the resource. The resource passed to FinalizeKind will always have a set
	// deletion timestamp.
	FinalizeKind(ctx {{.contextContext|raw}}, o *{{.type|raw}}) {{.reconcilerEvent|raw}}
}

// ReadOnlyInterface defines the strongly typed interfaces to be implemented by a
// controller reconciling {{.type|raw}} if they want to process resources for which
// they are not the leader.
type ReadOnlyInterface interface {
	// ObserveKind implements logic to observe {{.type|raw}}.
	// This method should not write to the API.
	ObserveKind(ctx {{.contextContext|raw}}, o *{{.type|raw}}) {{.reconcilerEvent|raw}}
}

type doReconcile func(ctx {{.contextContext|raw}}, o *{{.type|raw}}) {{.reconcilerEvent|raw}}

// reconcilerImpl implements controller.Reconciler for {{.type|raw}} resources.
type reconcilerImpl struct {
	// LeaderAwareFuncs is inlined to help us implement {{.reconcilerLeaderAware|raw}}.
	{{.reconcilerLeaderAwareFuncs|raw}}

	// Client is used to write back status updates.
	Client {{.clientsetInterface|raw}}

	// Listers index properties about resources.
	Lister {{.resourceLister|raw}}

	// Recorder is an event recorder for recording Event resources to the
	// Kubernetes API.
	Recorder {{.recordEventRecorder|raw}}

	// configStore allows for decorating a context with config maps.
	// +optional
	configStore {{.reconcilerConfigStore|raw}}

	// reconciler is the implementation of the business logic of the resource.
	reconciler Interface

	// finalizerName is the name of the finalizer to reconcile.
	finalizerName string

	// useServerSideApplyForFinalizers configures whether to use server-side apply for finalizer management
	useServerSideApplyForFinalizers bool

	// finalizerFieldManager is the field manager name for server-side apply of finalizers
	finalizerFieldManager string

	// forceApplyFinalizers configures whether to force server-side apply for finalizers
	forceApplyFinalizers bool

	{{if .hasStatus}}
	// skipStatusUpdates configures whether or not this reconciler automatically updates
	// the status of the reconciled resource.
	skipStatusUpdates bool
	{{end}}

	{{if len .classes | eq 1 }}
	// classValue is the resource annotation[{{ index .classes 0 }}] instance value this reconciler instance filters on.
	classValue string
	{{else if gt (len .classes) 1 }}
	// classValue is the resource annotation instance value this reconciler instance filters on.
	// The annotations key are:
	{{- range $class := .classes}}
	//   {{$class}}
	{{- end}}
	classValue string
	{{end}}
}

// Check that our Reconciler implements controller.Reconciler.
var _ controller.Reconciler = (*reconcilerImpl)(nil)
// Check that our generated Reconciler is always LeaderAware.
var _ {{.reconcilerLeaderAware|raw}}  = (*reconcilerImpl)(nil)

`

var reconcilerNewReconciler = `
func NewReconciler(ctx {{.contextContext|raw}}, logger *{{.zapSugaredLogger|raw}}, client {{.clientsetInterface|raw}}, lister {{.resourceLister|raw}}, recorder {{.recordEventRecorder|raw}}, r Interface{{if .hasClass}}, classValue string{{end}}, options ...{{.controllerOptions|raw}} ) {{.controllerReconciler|raw}} {
	// Check the options function input. It should be 0 or 1.
	if len(options) > 1 {
		logger.Fatal("Up to one options struct is supported, found: ", len(options))
	}

	// Fail fast when users inadvertently implement the other LeaderAware interface.
	// For the typed reconcilers, Promote shouldn't take any arguments.
	if _, ok := r.({{.reconcilerLeaderAware|raw}}); ok {
		logger.Fatalf("%T implements the incorrect LeaderAware interface. Promote() should not take an argument as genreconciler handles the enqueuing automatically.", r)
	}

	rec := &reconcilerImpl{
		LeaderAwareFuncs: {{.reconcilerLeaderAwareFuncs|raw}}{
			PromoteFunc: func(bkt {{.reconcilerBucket|raw}}, enq func({{.reconcilerBucket|raw}}, {{.typesNamespacedName|raw}})) error {
				all, err := lister.List({{.labelsEverything|raw}}())
				if err != nil {
					return err
				}
				for _, elt := range all {
					// TODO: Consider letting users specify a filter in options.
					enq(bkt, {{.typesNamespacedName|raw}}{
						Namespace: elt.GetNamespace(),
						Name: elt.GetName(),
					})
				}
				return nil
			},
		},
		Client: client,
		Lister: lister,
		Recorder: recorder,
		reconciler:    r,
		finalizerName: defaultFinalizerName,
		{{if .hasClass}}classValue: classValue,{{end}}
	}

	for _, opts := range options {
		if opts.ConfigStore != nil {
			rec.configStore = opts.ConfigStore
		}
		if opts.FinalizerName != "" {
			rec.finalizerName = opts.FinalizerName
		}
		{{- if .hasStatus}}
		if opts.SkipStatusUpdates {
			rec.skipStatusUpdates = true
		}
		{{- end}}
		if opts.DemoteFunc != nil {
			rec.DemoteFunc = opts.DemoteFunc
		}
		if opts.UseServerSideApplyForFinalizers {
			if opts.FinalizerFieldManager == "" {
				logger.Fatal("FinalizerFieldManager must be provided when UseServerSideApplyForFinalizers is enabled")
			}
			rec.useServerSideApplyForFinalizers = true
			rec.finalizerFieldManager = opts.FinalizerFieldManager
			rec.forceApplyFinalizers = opts.ForceApplyFinalizers
		}
	}

	return rec
}
`

var reconcilerImplFactory = `
// Reconcile implements controller.Reconciler
func (r *reconcilerImpl) Reconcile(ctx {{.contextContext|raw}}, key string) error {
	logger := {{.loggingFromContext|raw}}(ctx)

	// Initialize the reconciler state. This will convert the namespace/name
	// string into a distinct namespace and name, determine if this instance of
	// the reconciler is the leader, and any additional interfaces implemented
	// by the reconciler. Returns an error is the resource key is invalid.
	s, err := newState(key, r)
	if err != nil {
		logger.Error("Invalid resource key: ", key)
		return nil
	}

	// If we are not the leader, and we don't implement either ReadOnly
	// observer interfaces, then take a fast-path out.
	if s.isNotLeaderNorObserver() {
		return {{.controllerNewSkipKey|raw}}(key)
	}

	// If configStore is set, attach the frozen configuration to the context.
	if r.configStore != nil {
		ctx = r.configStore.ToContext(ctx)
	}

	// Add the recorder to context.
	ctx = {{.controllerWithEventRecorder|raw}}(ctx, r.Recorder)

	// Get the resource with this namespace/name.
	{{if .nonNamespaced}}
	getter := r.Lister
	{{else}}
	getter := r.Lister.{{.type|apiGroup}}(s.namespace)
	{{end}}
	original, err := getter.Get(s.name)

	if {{.apierrsIsNotFound|raw}}(err) {
		// The resource may no longer exist, in which case we stop processing and call
		// the ObserveDeletion handler if appropriate.
		logger.Debugf("Resource %q no longer exists", key)
		if del, ok := r.reconciler.({{.reconcilerOnDeletionInterface|raw}}); ok {
			return del.ObserveDeletion(ctx, {{.typesNamespacedName|raw}}{
				Namespace: s.namespace,
				Name: s.name,
			})
		}
		return nil
	} else if err != nil {
		return err
	}

{{if len .classes | eq 1 }}
	if classValue, found := original.GetAnnotations()[ClassAnnotationKey]; !found || classValue != r.classValue {
		logger.Debugw("Skip reconciling resource, class annotation value does not match reconciler instance value.",
			zap.String("classKey", ClassAnnotationKey),
			zap.String("issue", classValue+"!="+r.classValue))
		return nil
	}
{{else if gt (len .classes) 1 }}
	if classValue, found := lookupClass(original.GetAnnotations()); !found || classValue != r.classValue {
		logger.Debugw("Skip reconciling resource, class annotation value does not match reconciler instance value.",
			zap.Strings("classKeys", ClassAnnotationKeys),
			zap.String("issue", classValue+"!="+r.classValue))
		return nil
	}
{{end}}

	// Don't modify the informers copy.
	resource := original.DeepCopy()

	var reconcileEvent {{.reconcilerEvent|raw}}

	name, do := s.reconcileMethodFor(resource)
	// Append the target method to the logger.
	logger = logger.With(zap.String("targetMethod", name))
	switch name {
	case {{.doReconcileKind|raw}}:
		// Set and update the finalizer on resource if r.reconciler
		// implements Finalizer.
		if resource, err = r.setFinalizerIfFinalizer(ctx, resource); err != nil {
			return {{.fmtErrorf|raw}}("failed to set finalizers: %w", err)
		}
		{{if .isKRShaped}}
		if !r.skipStatusUpdates {
			reconciler.PreProcessReconcile(ctx, resource)
		}
		{{end}}

		// Reconcile this copy of the resource and then write back any status
		// updates regardless of whether the reconciliation errored out.
		reconcileEvent = do(ctx, resource)

		{{if .isKRShaped}}
		if !r.skipStatusUpdates {
			reconciler.PostProcessReconcile(ctx, resource, original)
		}
		{{end}}

	case {{.doFinalizeKind|raw}}:
		// For finalizing reconcilers, if this resource being marked for deletion
		// and reconciled cleanly (nil or normal event), remove the finalizer.
		reconcileEvent = do(ctx, resource)

		if resource, err = r.clearFinalizer(ctx, resource, reconcileEvent); err != nil {
			return {{.fmtErrorf|raw}}("failed to clear finalizers: %w", err)
		}

	case {{.doObserveKind|raw}}:
		// Observe any changes to this resource, since we are not the leader.
		reconcileEvent = do(ctx, resource)

	}

	{{if .hasStatus}}
	// Synchronize the status.
	switch {
	case r.skipStatusUpdates:
		// This reconciler implementation is configured to skip resource updates.
		// This may mean this reconciler does not observe spec, but reconciles external changes.
	case {{.equalitySemantic|raw}}.DeepEqual(original.Status, resource.Status):
		// If we didn't change anything then don't call updateStatus.
		// This is important because the copy we loaded from the injectionInformer's
		// cache may be stale and we don't want to overwrite a prior update
		// to status with this stale state.
	case !s.isLeader:
		// High-availability reconcilers may have many replicas watching the resource, but only
		// the elected leader is expected to write modifications.
		logger.Warn("Saw status changes when we aren't the leader!")
	default:
		if err = r.updateStatus(ctx, logger, original, resource); err != nil {
			logger.Warnw("Failed to update resource status", zap.Error(err))
			r.Recorder.Eventf(resource, {{.corev1EventTypeWarning|raw}}, "UpdateFailed",
				"Failed to update status for %q: %v", resource.Name, err)
			return err
		}
	}
	{{end}}

	// Report the reconciler event, if any.
	if reconcileEvent != nil {
		var event *{{.reconcilerReconcilerEvent|raw}}
		if reconciler.EventAs(reconcileEvent, &event) {
			logger.Infow("Returned an event", zap.Any("event", reconcileEvent))
			r.Recorder.Event(resource, event.EventType, event.Reason, event.Error())

			// the event was wrapped inside an error, consider the reconciliation as failed
			if _, isEvent := reconcileEvent.(*reconciler.ReconcilerEvent); !isEvent {
				return reconcileEvent
			}
			return nil
		}

		if {{ .controllerIsSkipKey|raw }}(reconcileEvent) {
			// This is a wrapped error, don't emit an event.
		} else if ok, _ := {{ .controllerIsRequeueKey|raw }}(reconcileEvent); ok {
			// This is a wrapped error, don't emit an event.
		} else if {{ .apierrsIsConflict|raw }}(reconcileEvent) {
			// Conflict errors are expected, don't emit an event.
		} else {
			logger.Errorw("Returned an error", zap.Error(reconcileEvent))
			r.Recorder.Event(resource, {{.corev1EventTypeWarning|raw}}, "InternalError", reconcileEvent.Error())
		}
		return reconcileEvent
	}

	return nil
}
`

var reconcilerStatusFactory = `
func (r *reconcilerImpl) updateStatus(ctx {{.contextContext|raw}}, logger *{{.zapSugaredLogger|raw}}, existing *{{.type|raw}}, desired *{{.type|raw}}) error {
	existing = existing.DeepCopy()
	return {{.reconcilerRetryUpdateConflicts|raw}}(func(attempts int) (err error) {
		// The first iteration tries to use the injectionInformer's state, subsequent attempts fetch the latest state via API.
		if attempts > 0 {
			{{if .nonNamespaced}}
			getter := r.Client.{{.group}}{{.version}}().{{.type|apiGroup}}()
			{{else}}
			getter := r.Client.{{.group}}{{.version}}().{{.type|apiGroup}}(desired.Namespace)
			{{end}}
			existing, err = getter.Get(ctx, desired.Name, {{.metav1GetOptions|raw}}{})
			if err != nil {
				return err
			}
		}

		// If there's nothing to update, just return.
		if {{.equalitySemantic|raw}}.DeepEqual(existing.Status, desired.Status) {
			return nil
		}

		if logger.Desugar().Core().Enabled({{.zapDebugLevel|raw}}) {
			if diff, err := {{.kmpSafeDiff|raw}}(existing.Status, desired.Status); err == nil && diff != "" {
				logger.Debug("Updating status with: ", diff)
			}
		}

		existing.Status = desired.Status

		{{if .nonNamespaced}}
		updater := r.Client.{{.group}}{{.version}}().{{.type|apiGroup}}()
		{{else}}
		updater := r.Client.{{.group}}{{.version}}().{{.type|apiGroup}}(existing.Namespace)
		{{end}}
		_, err = updater.UpdateStatus(ctx, existing, {{.metav1UpdateOptions|raw}}{})
		return err
	})
}
`

var reconcilerFinalizerFactory = `
// updateFinalizersFiltered will update the Finalizers of the resource.
// TODO: this method could be generic and sync all finalizers. For now it only
// updates defaultFinalizerName or its override.
func (r *reconcilerImpl) updateFinalizersFiltered(ctx {{.contextContext|raw}}, resource *{{.type|raw}}, desiredFinalizers {{.setsString|raw}}) (*{{.type|raw}}, error) {
	if r.useServerSideApplyForFinalizers {
		return r.updateFinalizersFilteredServerSideApply(ctx, resource, desiredFinalizers)
	}
	return r.updateFinalizersFilteredMergePatch(ctx, resource, desiredFinalizers)
}

// updateFinalizersFilteredServerSideApply uses server-side apply to manage only this controller's finalizer.
func (r *reconcilerImpl) updateFinalizersFilteredServerSideApply(ctx {{.contextContext|raw}}, resource *{{.type|raw}}, desiredFinalizers {{.setsString|raw}}) (*{{.type|raw}}, error) {
	// Check if we need to do anything
	existingFinalizers := {{.setsNewString|raw}}(resource.Finalizers...)

	var finalizers []string
	if desiredFinalizers.Has(r.finalizerName) {
		if existingFinalizers.Has(r.finalizerName) {
			// Nothing to do.
			return resource, nil
		}
		// Apply configuration with only our finalizer to add it.
		finalizers = []string{r.finalizerName}
	} else {
		if !existingFinalizers.Has(r.finalizerName) {
			// Nothing to do.
			return resource, nil
		}
		// For removal, we apply an empty configuration for our finalizer field manager.
		// This effectively removes our finalizer while preserving others.
		finalizers = []string{} // Empty array removes our managed finalizers
	}

	// Determine GVK
	gvks, _, err := {{.schemeScheme|raw}}.ObjectKinds(resource)
	if err != nil || len(gvks) == 0 {
		return resource, {{.fmtErrorf|raw}}("failed to determine GVK for resource: %w", err)
	}
	gvk := gvks[0]

	// Create apply configuration
	applyConfig := map[string]interface{}{
		"apiVersion": gvk.GroupVersion().String(),
		"kind":       gvk.Kind,
		"metadata": map[string]interface{}{
			"name":       resource.Name,
			"uid":        resource.UID,
			"finalizers": finalizers,
		},
	}
	{{if not .nonNamespaced}}
	applyConfig["metadata"].(map[string]interface{})["namespace"] = resource.Namespace
	{{end}}

	patch, err := {{.jsonMarshal|raw}}(applyConfig)
	if err != nil {
		return resource, err
	}

	{{if .nonNamespaced}}
	patcher := r.Client.{{.group}}{{.version}}().{{.type|apiGroup}}()
	{{else}}
	patcher := r.Client.{{.group}}{{.version}}().{{.type|apiGroup}}(resource.Namespace)
	{{end}}

	patchOpts := {{.metav1PatchOptions|raw}}{
		FieldManager:    r.finalizerFieldManager,
		Force:           &r.forceApplyFinalizers,
	}

	updated, err := patcher.Patch(ctx, resource.Name, {{.typesApplyPatchType|raw}}, patch, patchOpts)
	if err != nil {
		if !{{ .apierrsIsConflict|raw }}(err) {
			r.Recorder.Eventf(resource, {{.corev1EventTypeWarning|raw}}, "FinalizerUpdateFailed",
				"Failed to update finalizers for %q via server-side apply: %v", resource.Name, err)
		}
	} else {
		r.Recorder.Eventf(updated, {{.corev1EventTypeNormal|raw}}, "FinalizerUpdate",
			"Updated finalizers for %q via server-side apply", resource.GetName())
	}
	return updated, err
}

// updateFinalizersFilteredMergePatch uses merge patch to manage finalizers (legacy behavior).
func (r *reconcilerImpl) updateFinalizersFilteredMergePatch(ctx {{.contextContext|raw}}, resource *{{.type|raw}}, desiredFinalizers {{.setsString|raw}}) (*{{.type|raw}}, error) {
	// Don't modify the informers copy.
	existing := resource.DeepCopy()

	var finalizers []string

	// If there's nothing to update, just return.
	existingFinalizers := {{.setsNewString|raw}}(existing.Finalizers...)

	if desiredFinalizers.Has(r.finalizerName) {
		if existingFinalizers.Has(r.finalizerName) {
			// Nothing to do.
			return resource, nil
		}
		// Add the finalizer.
		finalizers = append(existing.Finalizers, r.finalizerName)
	} else {
		if !existingFinalizers.Has(r.finalizerName) {
			// Nothing to do.
			return resource, nil
		}
		// Remove the finalizer.
		existingFinalizers.Delete(r.finalizerName)
		finalizers = sets.List(existingFinalizers)
	}

	mergePatch := map[string]interface{}{
		"metadata": map[string]interface{}{
			"finalizers":      finalizers,
			"resourceVersion": existing.ResourceVersion,
		},
	}

	patch, err := {{.jsonMarshal|raw}}(mergePatch)
	if err != nil {
		return resource, err
	}

	{{if .nonNamespaced}}
	patcher := r.Client.{{.group}}{{.version}}().{{.type|apiGroup}}()
	{{else}}
	patcher := r.Client.{{.group}}{{.version}}().{{.type|apiGroup}}(resource.Namespace)
	{{end}}
	resourceName := resource.Name
	updated, err := patcher.Patch(ctx, resourceName, {{.typesMergePatchType|raw}}, patch, {{.metav1PatchOptions|raw}}{})
	if err != nil {
		if !{{ .apierrsIsConflict|raw }}(err) {
			r.Recorder.Eventf(existing, {{.corev1EventTypeWarning|raw}}, "FinalizerUpdateFailed",
				"Failed to update finalizers for %q: %v", resourceName, err)
		}
	} else {
		r.Recorder.Eventf(updated, {{.corev1EventTypeNormal|raw}}, "FinalizerUpdate",
			"Updated %q finalizers", resource.GetName())
	}
	return updated, err
}

func (r *reconcilerImpl) setFinalizerIfFinalizer(ctx {{.contextContext|raw}}, resource *{{.type|raw}}) (*{{.type|raw}}, error) {
	if _, ok := r.reconciler.(Finalizer); !ok {
		return resource, nil
	}

	finalizers := {{.setsNewString|raw}}(resource.Finalizers...)

	// If this resource is not being deleted, mark the finalizer.
	if resource.GetDeletionTimestamp().IsZero() {
		finalizers.Insert(r.finalizerName)
	}

	// Synchronize the finalizers filtered by r.finalizerName.
	return r.updateFinalizersFiltered(ctx, resource, finalizers)
}

func (r *reconcilerImpl) clearFinalizer(ctx {{.contextContext|raw}}, resource *{{.type|raw}}, reconcileEvent {{.reconcilerEvent|raw}}) (*{{.type|raw}}, error) {
	if _, ok := r.reconciler.(Finalizer); !ok {
		return resource, nil
	}
	if resource.GetDeletionTimestamp().IsZero() {
		return resource, nil
	}

	finalizers := {{.setsNewString|raw}}(resource.Finalizers...)

	if reconcileEvent != nil {
		var event *{{.reconcilerReconcilerEvent|raw}}
		if reconciler.EventAs(reconcileEvent, &event) {
			if event.EventType == {{.corev1EventTypeNormal|raw}} {
				finalizers.Delete(r.finalizerName)
			}
		}
	} else {
		finalizers.Delete(r.finalizerName)
	}

	// Synchronize the finalizers filtered by r.finalizerName.
	updated, err := r.updateFinalizersFiltered(ctx, resource, finalizers)
	if err != nil {
		// Check if the resource still exists by querying the API server to avoid logging errors
		// when reconciling stale object from cache while the object is actually deleted.
		logger := {{.loggingFromContext|raw}}(ctx)

		{{if .nonNamespaced}}
		getter := r.Client.{{.group}}{{.version}}().{{.type|apiGroup}}()
		{{else}}
		getter := r.Client.{{.group}}{{.version}}().{{.type|apiGroup}}(resource.Namespace)
		{{end}}

		_, getErr := getter.Get(ctx, resource.Name, {{.metav1GetOptions|raw}}{})
		if {{.apierrsIsNotFound|raw}}(getErr) {
			// Resource no longer exists, which could happen during deletion
			logger.Debugw("Resource no longer exists while clearing finalizers",
				"resource", resource.GetName(),
				"namespace", resource.GetNamespace(),
				"originalError", err)
			// Return the original resource since the finalizer clearing is effectively complete
			return resource, nil
		}

		// For other errors, return the original error
		return updated, err
	}

	return updated, nil
}

`
