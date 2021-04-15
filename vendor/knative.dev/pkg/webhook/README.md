## Knative Webhooks

Knative provides infrastructure for authoring webhooks under
`knative.dev/pkg/webhook` and has a few built-in helpers for certain common
admission control scenarios. The built-in admission controllers are:

1. Resource validation and defaulting (builds around `apis.Validatable` and
   `apis.Defaultable` under `knative.dev/pkg/apis`).
2. ConfigMap validation, which builds around similar patterns from
   `knative.dev/pkg/configmap` (in particular the `store` concept)

To illustrate standing up the webhook, let's start with one of these built-in
admission controllers and then talk about how you can write your own admission
controller.

## Standing up a Webhook from an Admission Controller

We provide facilities in `knative.dev/pkg/injection/sharedmain` to try and
eliminate much of the boilerplate involved in standing up a webhook. For this
example we will show how to stand up the webhook using the built-in admission
controller for validating and defaulting resources.

The code to stand up such a webhook looks roughly like this:

```go
// Create a function matching this signature to pass into sharedmain.
func NewResourceAdmissionController(ctx context.Context, cmw configmap.Watcher) *controller.Impl {
	return validation.NewAdmissionController(ctx,
		// Name of the resource webhook (created via yaml)
		fmt.Sprintf("resources.webhook.%s.knative.dev", system.Namespace()),

		// The path on which to serve the webhook.
		"/resource-validation",

		// The resources to validate and default.
		map[schema.GroupVersionKind]resourcesemantics.GenericCRD{
			// List the types to validate, this from knative.dev/sample-controller
			v1alpha1.SchemeGroupVersion.WithKind("AddressableService"): &v1alpha1.AddressableService{},
		},

		// A function that infuses the context passed to Validate/SetDefaults with custom metadata.
		func(ctx context.Context) context.Context {
			// Here is where you would infuse the context with state
			// (e.g. attach a store with configmap data, like knative.dev/serving attaches config-defaults)
			return ctx
		},

		// Whether to disallow unknown fields when parsing the resources' JSON.
		true,
	)
}

func main() {
	// Set up a signal context with our webhook options.
	ctx := webhook.WithOptions(signals.NewContext(), webhook.Options{
		// The name of the Kubernetes service selecting over this deployment's pods.
		ServiceName: "webhook",

		// The port on which to serve.
		Port:        8443,

		// The name of the secret containing certificate data.
		SecretName:  "webhook-certs",
	})

	sharedmain.MainWithContext(ctx, "webhook",
		// The certificate controller will ensure that the named secret (above) has
		// the appropriate shape for our webhook's admission controllers.
		certificates.NewController,

		// This invokes the method defined above to instantiate the resource admission
		// controller.
		NewResourceAdmissionController,
	)
}
```

There is also a config map validation admission controller built in under
`knative.dev/pkg/webhook/configmaps`.

## Writing new Admission Controllers

To implement your own admission controller akin to the resource defaulting and
validation controller above, you implement a
`knative.dev/pkg/controller.Reconciler` as with any you would with any other
type of controller, but the `Reconciler` that gets embedded in the
`*controller.Impl` should _also_ implement:

```go
// AdmissionController provides the interface for different admission controllers
type AdmissionController interface {
	// Path returns the path that this particular admission controller serves on.
	Path() string

	// Admit is the callback which is invoked when an HTTPS request comes in on Path().
	Admit(context.Context, *admissionv1beta1.AdmissionRequest) *admissionv1beta1.AdmissionResponse
}
```

The `Reconciler` part is responsible for the mutating or validating webhook
configuration. The `AdmissionController` part is responsible for guiding request
dispatch (`Path()`) and handling admission requests (`Admit()`).
