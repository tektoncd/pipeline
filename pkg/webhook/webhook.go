package webhook

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/gobuffalo/flect"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"go.uber.org/zap"
	admissionv1 "k8s.io/api/admission/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	authzv1 "k8s.io/api/authorization/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	authnclient "k8s.io/client-go/kubernetes/typed/authentication/v1"
	authzclient "k8s.io/client-go/kubernetes/typed/authorization/v1"
	admissionlisters "k8s.io/client-go/listers/admissionregistration/v1"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"knative.dev/pkg/apis"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	vwhinformer "knative.dev/pkg/client/injection/kube/informers/admissionregistration/v1/validatingwebhookconfiguration"
	"knative.dev/pkg/controller"
	secretinformer "knative.dev/pkg/injection/clients/namespacedkube/informers/core/v1/secret"
	"knative.dev/pkg/kmp"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/ptr"
	pkgreconciler "knative.dev/pkg/reconciler"
	"knative.dev/pkg/system"
	"knative.dev/pkg/webhook"
	certresources "knative.dev/pkg/webhook/certificates/resources"
	"knative.dev/pkg/webhook/json"
	"knative.dev/pkg/webhook/resourcesemantics"
)

// reconciler implements the AdmissionController for resources
type reconciler struct {
	webhook.StatelessAdmissionImpl
	pkgreconciler.LeaderAwareFuncs

	key       types.NamespacedName
	path      string
	handlers  map[schema.GroupVersionKind]resourcesemantics.GenericCRD
	callbacks map[schema.GroupVersionKind]Callback

	withContext func(context.Context) context.Context

	client       kubernetes.Interface
	vwhlister    admissionlisters.ValidatingWebhookConfigurationLister
	secretlister corelisters.SecretLister

	disallowUnknownFields bool
	secretName            string
	authn                 authnclient.AuthenticationV1Interface
	authz                 authzclient.AuthorizationV1Interface
}

var _ controller.Reconciler = (*reconciler)(nil)
var _ pkgreconciler.LeaderAware = (*reconciler)(nil)
var _ webhook.AdmissionController = (*reconciler)(nil)
var _ webhook.StatelessAdmissionController = (*reconciler)(nil)

// NewAdmissionControllerWithConfig constructs a reconciler and registers the
// provided handlers with specified verbs and SubResources
func NewAdmissionControllerWithConfig(
	ctx context.Context,
	name, path string,
	handlers map[schema.GroupVersionKind]resourcesemantics.GenericCRD,
	wc func(context.Context) context.Context,
	disallowUnknownFields bool,
	callbacks map[schema.GroupVersionKind]Callback,
) *controller.Impl {
	client := kubeclient.Get(ctx)
	vwhInformer := vwhinformer.Get(ctx)
	secretInformer := secretinformer.Get(ctx)
	options := webhook.GetOptions(ctx)

	wh := &reconciler{
		LeaderAwareFuncs: pkgreconciler.LeaderAwareFuncs{
			// Have this reconciler enqueue our singleton whenever it becomes leader.
			PromoteFunc: func(bkt pkgreconciler.Bucket, enq func(pkgreconciler.Bucket, types.NamespacedName)) error {
				enq(bkt, types.NamespacedName{Name: name})
				return nil
			},
		},

		key: types.NamespacedName{
			Name: name,
		},
		path:      path,
		handlers:  handlers,
		callbacks: callbacks,

		withContext:           wc,
		disallowUnknownFields: disallowUnknownFields,
		secretName:            options.SecretName,

		client:       client,
		vwhlister:    vwhInformer.Lister(),
		secretlister: secretInformer.Lister(),
		authn:        client.AuthenticationV1(),
		authz:        client.AuthorizationV1(),
	}

	logger := logging.FromContext(ctx)
	const queueName = "ValidationWebhook"
	c := controller.NewContext(ctx, wh, controller.ControllerOptions{WorkQueueName: queueName, Logger: logger.Named(queueName)})

	// Reconcile when the named ValidatingWebhookConfiguration changes.
	vwhInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.FilterWithName(name),
		// It doesn't matter what we enqueue because we will always Reconcile
		// the named VWH resource.
		Handler: controller.HandleAll(c.Enqueue),
	})

	// Reconcile when the cert bundle changes.
	secretInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.FilterWithNameAndNamespace(system.Namespace(), wh.secretName),
		// It doesn't matter what we enqueue because we will always Reconcile
		// the named VWH resource.
		Handler: controller.HandleAll(c.Enqueue),
	})

	return c

}

// NewAdmissionController constructs a reconciler
func NewAdmissionController(
	ctx context.Context,
	name, path string,
	handlers map[schema.GroupVersionKind]resourcesemantics.GenericCRD,
	wc func(context.Context) context.Context,
	disallowUnknownFields bool,
	callbacks ...map[schema.GroupVersionKind]Callback,
) *controller.Impl {
	// This not ideal, we are using a variadic argument to effectively make callbacks optional
	// This allows this addition to be non-breaking to consumers of /pkg
	// TODO: once all sub-repos have adopted this, we might move this back to a traditional param.
	var unwrappedCallbacks map[schema.GroupVersionKind]Callback
	switch len(callbacks) {
	case 0:
		unwrappedCallbacks = map[schema.GroupVersionKind]Callback{}
	case 1:
		unwrappedCallbacks = callbacks[0]
	default:
		panic("NewAdmissionController may not be called with multiple callback maps")
	}
	return NewAdmissionControllerWithConfig(ctx, name, path, handlers, wc, disallowUnknownFields, unwrappedCallbacks)
}

// Path implements AdmissionController
func (ac *reconciler) Path() string {
	return ac.path
}

// Reconcile implements controller.Reconciler
func (ac *reconciler) Reconcile(ctx context.Context, key string) error {
	logger := logging.FromContext(ctx)

	if !ac.IsLeaderFor(ac.key) {
		return controller.NewSkipKey(key)
	}

	// Look up the webhook secret, and fetch the CA cert bundle.
	secret, err := ac.secretlister.Secrets(system.Namespace()).Get(ac.secretName)
	if err != nil {
		logger.Errorw("Error fetching secret", zap.Error(err))
		return err
	}
	caCert, ok := secret.Data[certresources.CACert]
	if !ok {
		return fmt.Errorf("secret %q is missing %q key", ac.secretName, certresources.CACert)
	}

	// Reconcile the webhook configuration.
	return ac.reconcileValidatingWebhook(ctx, caCert)
}

func (ac *reconciler) reconcileValidatingWebhook(ctx context.Context, caCert []byte) error {
	logger := logging.FromContext(ctx)

	rules := make([]admissionregistrationv1.RuleWithOperations, 0, len(ac.handlers)+len(ac.callbacks))
	for gvk, config := range ac.handlers {
		plural := strings.ToLower(flect.Pluralize(gvk.Kind))

		// If SupportedVerbs has not been given, provide the legacy defaults
		// of Create, Update, and Delete
		supportedVerbs := []admissionregistrationv1.OperationType{
			admissionregistrationv1.Create,
			admissionregistrationv1.Update,
			admissionregistrationv1.Delete,
		}

		if vl, ok := config.(resourcesemantics.VerbLimited); ok {
			logging.FromContext(ctx).Debugf("Using custom Verbs")
			supportedVerbs = vl.SupportedVerbs()
		}
		logging.FromContext(ctx).Debugf("Registering verbs: %s", supportedVerbs)

		resources := []string{}
		// If SupportedSubResources has not been given, provide the legacy
		// defaults of main resource, and status
		if srl, ok := config.(resourcesemantics.SubResourceLimited); ok {
			logging.FromContext(ctx).Debugf("Using custom SubResources")
			for _, subResource := range srl.SupportedSubResources() {
				if subResource == "" {
					// Special case the actual plural if given
					resources = append(resources, plural)
				} else {
					resources = append(resources, plural+subResource)
				}
			}
		} else {
			resources = append(resources, plural, plural+"/status")
		}
		logging.FromContext(ctx).Debugf("Registering SubResources: %s", resources)
		rules = append(rules, admissionregistrationv1.RuleWithOperations{
			Operations: supportedVerbs,
			Rule: admissionregistrationv1.Rule{
				APIGroups:   []string{gvk.Group},
				APIVersions: []string{gvk.Version},
				Resources:   resources,
			},
		})
	}
	for gvk, callback := range ac.callbacks {
		if _, ok := ac.handlers[gvk]; ok {
			continue
		}
		plural := strings.ToLower(flect.Pluralize(gvk.Kind))
		resources := []string{plural, plural + "/status"}

		verbs := make([]admissionregistrationv1.OperationType, 0, len(callback.supportedVerbs))
		for verb := range callback.supportedVerbs {
			verbs = append(verbs, admissionregistrationv1.OperationType(verb))
		}
		// supportedVerbs is a map which doesn't provide a stable order in for loops.
		sort.Slice(verbs, func(i, j int) bool { return string(verbs[i]) < string(verbs[j]) })

		rules = append(rules, admissionregistrationv1.RuleWithOperations{
			Operations: verbs,
			Rule: admissionregistrationv1.Rule{
				APIGroups:   []string{gvk.Group},
				APIVersions: []string{gvk.Version},
				Resources:   resources,
			},
		})
	}

	for _, r := range rules {
		logging.FromContext(ctx).Debugf("Rule: %+v", r)
	}

	// Sort the rules by Group, Version, Kind so that things are deterministically ordered.
	sort.Slice(rules, func(i, j int) bool {
		lhs, rhs := rules[i], rules[j]
		if lhs.APIGroups[0] != rhs.APIGroups[0] {
			return lhs.APIGroups[0] < rhs.APIGroups[0]
		}
		if lhs.APIVersions[0] != rhs.APIVersions[0] {
			return lhs.APIVersions[0] < rhs.APIVersions[0]
		}
		return lhs.Resources[0] < rhs.Resources[0]
	})

	configuredWebhook, err := ac.vwhlister.Get(ac.key.Name)
	if err != nil {
		return fmt.Errorf("error retrieving webhook: %w", err)
	}

	current := configuredWebhook.DeepCopy()

	// Set the owner to namespace.
	ns, err := ac.client.CoreV1().Namespaces().Get(ctx, system.Namespace(), metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to fetch namespace: %w", err)
	}
	nsRef := *metav1.NewControllerRef(ns, corev1.SchemeGroupVersion.WithKind("Namespace"))
	current.OwnerReferences = []metav1.OwnerReference{nsRef}

	for i, wh := range current.Webhooks {
		if wh.Name != current.Name {
			continue
		}
		cur := &current.Webhooks[i]
		cur.Rules = rules

		cur.NamespaceSelector = webhook.EnsureLabelSelectorExpressions(
			cur.NamespaceSelector,
			&metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{{
					Key:      "webhooks.knative.dev/exclude",
					Operator: metav1.LabelSelectorOpDoesNotExist,
				}},
			})

		cur.ClientConfig.CABundle = caCert
		if cur.ClientConfig.Service == nil {
			return fmt.Errorf("missing service reference for webhook: %s", wh.Name)
		}
		cur.ClientConfig.Service.Path = ptr.String(ac.Path())
	}

	if ok, err := kmp.SafeEqual(configuredWebhook, current); err != nil {
		return fmt.Errorf("error diffing webhooks: %w", err)
	} else if !ok {
		logger.Info("Updating webhook")
		vwhclient := ac.client.AdmissionregistrationV1().ValidatingWebhookConfigurations()
		if _, err := vwhclient.Update(ctx, current, metav1.UpdateOptions{}); err != nil {
			return fmt.Errorf("failed to update webhook: %w", err)
		}
	} else {
		logger.Info("Webhook is valid")
	}
	return nil
}

var errMissingNewObject = errors.New("the new object may not be nil")

// Callback is a generic function to be called by a consumer of validation
type Callback struct {
	// function is the callback to be invoked
	function func(ctx context.Context, unstructured *unstructured.Unstructured) error

	// supportedVerbs are the verbs supported for the callback.
	supportedVerbs map[webhook.Operation]struct{}
}

// NewCallback creates a new callback function to be invoked on supported verbs.
func NewCallback(function func(context.Context, *unstructured.Unstructured) error, supportedVerbs ...webhook.Operation) Callback {
	m := make(map[webhook.Operation]struct{})
	for _, op := range supportedVerbs {
		if _, has := m[op]; has {
			panic("duplicate verbs not allowed")
		}
		m[op] = struct{}{}
	}
	return Callback{function: function, supportedVerbs: m}
}

var _ webhook.AdmissionController = (*reconciler)(nil)

// Admit implements AdmissionController
func (ac *reconciler) Admit(ctx context.Context, request *admissionv1.AdmissionRequest) (resp *admissionv1.AdmissionResponse) {
	if ac.withContext != nil {
		ctx = ac.withContext(ctx)
	}

	kind := request.Kind
	gvk := schema.GroupVersionKind{
		Group:   kind.Group,
		Version: kind.Version,
		Kind:    kind.Kind,
	}

	ctx, resource, err := ac.decodeRequestAndPrepareContext(ctx, request, gvk)
	if err != nil {
		return webhook.MakeErrorStatus("decoding request failed: %v", err)
	}

	errors, warnings := ac.validate(ctx, resource, request)
	if warnings != nil {
		// If there were warnings, then keep processing things, but augment
		// whatever AdmissionResponse we send with the warnings.  We cannot
		// simply set `resp.Warnings` directly here because the return paths
		// below all overwrite `resp`, but the `defer` affords us one final
		// crack at things.
		defer func() {
			resp.Warnings = make([]string, 0, len(warnings))
			for _, w := range warnings {
				resp.Warnings = append(resp.Warnings, w.Error())
			}
		}()
	}
	if errors != nil {
		return webhook.MakeErrorStatus("validation failed: %v", errors)
	}

	if err := ac.callback(ctx, request, gvk); err != nil {
		return webhook.MakeErrorStatus("validation callback failed: %v", err)
	}

	return &admissionv1.AdmissionResponse{Allowed: true}
}

// decodeRequestAndPrepareContext deserializes the old and new GenericCrds from the incoming request and sets up the context.
// nil oldObj or newObj denote absence of `old` (create) or `new` (delete) objects.
func (ac *reconciler) decodeRequestAndPrepareContext(
	ctx context.Context,
	req *admissionv1.AdmissionRequest,
	gvk schema.GroupVersionKind) (context.Context, resourcesemantics.GenericCRD, error) {

	logger := logging.FromContext(ctx)
	handler, ok := ac.handlers[gvk]
	if !ok {
		logger.Error("Unhandled kind: ", gvk)
		return ctx, nil, fmt.Errorf("unhandled kind: %v", gvk)
	}

	newBytes := req.Object.Raw
	oldBytes := req.OldObject.Raw

	// Decode json to a GenericCRD
	var newObj resourcesemantics.GenericCRD
	if len(newBytes) != 0 {
		newObj = handler.DeepCopyObject().(resourcesemantics.GenericCRD)
		err := json.Decode(newBytes, newObj, ac.disallowUnknownFields)
		if err != nil {
			return ctx, nil, fmt.Errorf("cannot decode incoming new object: %w", err)
		}
	}

	var oldObj resourcesemantics.GenericCRD
	if len(oldBytes) != 0 {
		oldObj = handler.DeepCopyObject().(resourcesemantics.GenericCRD)
		err := json.Decode(oldBytes, oldObj, ac.disallowUnknownFields)
		if err != nil {
			return ctx, nil, fmt.Errorf("cannot decode incoming old object: %w", err)
		}
	}

	ctx = apis.WithUserInfo(ctx, &req.UserInfo)
	ctx = context.WithValue(ctx, kubeclient.Key{}, ac.client)
	if req.DryRun != nil && *req.DryRun {
		ctx = apis.WithDryRun(ctx)
	}

	switch req.Operation {
	case admissionv1.Update:
		if req.SubResource != "" {
			ctx = apis.WithinSubResourceUpdate(ctx, oldObj, req.SubResource)
		} else {
			ctx = apis.WithinUpdate(ctx, oldObj)
		}
	case admissionv1.Create:
		ctx = apis.WithinCreate(ctx)
	case admissionv1.Delete:
		ctx = apis.WithinDelete(ctx)
		return ctx, oldObj, nil
	}

	return ctx, newObj, nil
}

func (ac *reconciler) validate(ctx context.Context, resource resourcesemantics.GenericCRD, req *admissionv1.AdmissionRequest) (err error, warn []error) { //nolint
	logger := logging.FromContext(ctx)

	// Only run validation for supported create and update validation.
	switch req.Operation {
	case admissionv1.Create, admissionv1.Update:
		// Supported verbs
	case admissionv1.Delete:
		return nil, nil // Validation handled by optional Callback, but not validatable.
	default:
		logger.Info("Unhandled webhook validation operation, letting it through ", req.Operation)
		return nil, nil
	}

	// None of the validators will accept a nil value for newObj.
	if resource == nil {
		return errMissingNewObject, nil
	}

	if result := resource.Validate(ctx); result != nil {
		logger.Errorw("Failed the resource specific validation", zap.Error(err))
		// While we have the strong typing of apis.FieldError, partition the
		// returned error into the error-level diagnostics and warning-level
		// diagnostics, so that the admission response can embed things into
		// the appropriate portions of the response.
		// This is expanded like to to avoid problems with typed nils.
		if errorResult := result.Filter(apis.ErrorLevel); errorResult != nil {
			err = errorResult
		}
		if warningResult := result.Filter(apis.WarningLevel); warningResult != nil {
			ws := warningResult.WrappedErrors()
			warn = make([]error, 0, len(ws))
			for _, w := range ws {
				warn = append(warn, w)
			}
		}
	}
	if err != nil {
		return err, warn
	}

	switch obj := resource.(type) {
	case *v1.TaskRun:
		if obj.Context == nil { // FIXME: what we actually need is whether the context changed
			return nil, nil
		}
		user := req.UserInfo
		// This code comes from Results: https://github.com/tektoncd/results/blob/main/pkg/api/server/v1alpha2/auth/rbac.go

		// Authorize the request by checking the RBAC permissions for the resource.
		sar, err := ac.authz.SubjectAccessReviews().Create(ctx, &authzv1.SubjectAccessReview{
			Spec: authzv1.SubjectAccessReviewSpec{
				User:   user.Username,
				UID:    user.UID,
				Groups: user.Groups,
				ResourceAttributes: &authzv1.ResourceAttributes{
					Namespace: req.Namespace,
					Group:     "tekton.dev",
					Resource:  "context",
					Verb:      strings.ToLower(string(req.Operation)),
				},
			},
		}, metav1.CreateOptions{})
		if err != nil {
			retMsg := fmt.Errorf("user %q in groups %q employing %q against %q: %s", user.Username, user.Groups, string(req.Operation), resource, err.Error())
			logger.Error(err)
			return retMsg, nil
		}
		if sar.Status.Allowed {
			logger.Error("permitted: %s", sar.Status.Reason)
			return nil, nil
		} else {
			logger.Errorf("not permitted: %s, %s", sar.Status.Reason, sar.Status.EvaluationError)
			return fmt.Errorf("not permitted: %s, %s", sar.Status.Reason, sar.Status.EvaluationError), nil
		}
	default:
		logger.Errorf("not a taskrun: %s", obj)
		return nil, nil
	}
}

// callback runs optional callbacks on admission
func (ac *reconciler) callback(ctx context.Context, req *admissionv1.AdmissionRequest, gvk schema.GroupVersionKind) error {
	var toDecode []byte
	if req.Operation == admissionv1.Delete {
		toDecode = req.OldObject.Raw
	} else {
		toDecode = req.Object.Raw
	}
	if toDecode == nil {
		logger := logging.FromContext(ctx)
		logger.Errorf("No incoming object found: %v for verb %v", gvk, req.Operation)
		return nil
	}

	// Generically callback if any are provided for the resource.
	if c, ok := ac.callbacks[gvk]; ok {
		if _, supported := c.supportedVerbs[req.Operation]; supported {
			unstruct := &unstructured.Unstructured{}
			if err := json.Unmarshal(toDecode, unstruct); err != nil {
				return fmt.Errorf("cannot decode incoming new object: %w", err)
			}

			return c.function(ctx, unstruct)
		}
	}

	return nil
}
