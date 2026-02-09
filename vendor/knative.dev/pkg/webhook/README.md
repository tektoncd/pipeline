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

## TLS Configuration

The webhook server supports configuring TLS parameters through the `webhook.Options` struct. This allows you to control the TLS version, cipher suites, and elliptic curve preferences for enhanced security.

### Available TLS Options

```go
type Options struct {
    // ... other fields ...

    // TLSMinVersion contains the minimum TLS version that is acceptable.
    // Default is TLS 1.3 if not specified.
    // Supported values: tls.VersionTLS12, tls.VersionTLS13
    TLSMinVersion uint16

    // TLSMaxVersion contains the maximum TLS version that is acceptable.
    // If not set (0), the maximum version supported by the implementation will be used.
    // Useful for enforcing Modern profile (TLS 1.3 only) by setting both
    // TLSMinVersion and TLSMaxVersion to tls.VersionTLS13.
    TLSMaxVersion uint16

    // TLSCipherSuites specifies the list of enabled cipher suites.
    // If empty, a default list of secure cipher suites will be used.
    // Note: Cipher suites are not configurable in TLS 1.3; they are
    // determined by the implementation.
    TLSCipherSuites []uint16

    // TLSCurvePreferences specifies the elliptic curves that will be used
    // in an ECDHE handshake. If empty, the default curves will be used.
    TLSCurvePreferences []tls.CurveID
}
```

### Environment Variable Configuration

You can also configure the minimum TLS version via the `WEBHOOK_TLS_MIN_VERSION` environment variable:

```yaml
env:
  - name: WEBHOOK_TLS_MIN_VERSION
    value: "1.3"  # or "1.2"
```

### Usage Examples

#### Example 1: Default Configuration (Recommended)

By default, the webhook uses TLS 1.3 as the minimum version with secure defaults:

```go
ctx := webhook.WithOptions(signals.NewContext(), webhook.Options{
    ServiceName: "webhook",
    Port:        8443,
    SecretName:  "webhook-certs",
    // TLS defaults: MinVersion=1.3, secure cipher suites and curves
})
```

#### Example 2: Modern Profile (TLS 1.3 Only)

To enforce TLS 1.3 only (highest security profile):

```go
ctx := webhook.WithOptions(signals.NewContext(), webhook.Options{
    ServiceName:   "webhook",
    Port:          8443,
    SecretName:    "webhook-certs",
    TLSMinVersion: tls.VersionTLS13,
    TLSMaxVersion: tls.VersionTLS13,  // Enforce TLS 1.3 only
})
```

#### Example 3: Intermediate Profile (TLS 1.2+)

For broader compatibility while maintaining security:

```go
ctx := webhook.WithOptions(signals.NewContext(), webhook.Options{
    ServiceName:   "webhook",
    Port:          8443,
    SecretName:    "webhook-certs",
    TLSMinVersion: tls.VersionTLS12,
})
```

#### Example 4: Custom Cipher Suites

To specify custom cipher suites (for TLS 1.2):

```go
ctx := webhook.WithOptions(signals.NewContext(), webhook.Options{
    ServiceName:   "webhook",
    Port:          8443,
    SecretName:    "webhook-certs",
    TLSMinVersion: tls.VersionTLS12,
    TLSCipherSuites: []uint16{
        tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
        tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
        tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
        tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
    },
})
```

#### Example 5: Custom Elliptic Curves

To specify elliptic curve preferences:

```go
ctx := webhook.WithOptions(signals.NewContext(), webhook.Options{
    ServiceName:   "webhook",
    Port:          8443,
    SecretName:    "webhook-certs",
    TLSCurvePreferences: []tls.CurveID{
        tls.X25519,    // Preferred
        tls.CurveP256,
        tls.CurveP384,
    },
})
```

#### Example 6: Complete Custom Configuration

For full control over TLS parameters:

```go
ctx := webhook.WithOptions(signals.NewContext(), webhook.Options{
    ServiceName:   "webhook",
    Port:          8443,
    SecretName:    "webhook-certs",
    TLSMinVersion: tls.VersionTLS12,
    TLSMaxVersion: tls.VersionTLS13,
    TLSCipherSuites: []uint16{
        tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
        tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
    },
    TLSCurvePreferences: []tls.CurveID{
        tls.X25519,
        tls.CurveP256,
    },
})
```

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
