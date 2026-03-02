/*
Copyright 2019 The Tekton Authors

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

package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	defaultconfig "github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/pkg/apis/resolution"
	resolutionv1alpha1 "github.com/tektoncd/pipeline/pkg/apis/resolution/v1alpha1"
	resolutionv1beta1 "github.com/tektoncd/pipeline/pkg/apis/resolution/v1beta1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/injection"
	"knative.dev/pkg/injection/sharedmain"
	pkgleaderelection "knative.dev/pkg/leaderelection"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/signals"
	"knative.dev/pkg/system"
	"knative.dev/pkg/webhook"
	"knative.dev/pkg/webhook/certificates"
	"knative.dev/pkg/webhook/configmaps"
	"knative.dev/pkg/webhook/resourcesemantics"
	"knative.dev/pkg/webhook/resourcesemantics/conversion"
	"knative.dev/pkg/webhook/resourcesemantics/defaulting"
	"knative.dev/pkg/webhook/resourcesemantics/validation"
)

var types = map[schema.GroupVersionKind]resourcesemantics.GenericCRD{
	// v1alpha1
	v1alpha1.SchemeGroupVersion.WithKind("VerificationPolicy"): &v1alpha1.VerificationPolicy{},
	v1alpha1.SchemeGroupVersion.WithKind("StepAction"):         &v1alpha1.StepAction{},
	// v1beta1
	v1beta1.SchemeGroupVersion.WithKind("Pipeline"):    &v1beta1.Pipeline{},
	v1beta1.SchemeGroupVersion.WithKind("Task"):        &v1beta1.Task{},
	v1beta1.SchemeGroupVersion.WithKind("TaskRun"):     &v1beta1.TaskRun{},
	v1beta1.SchemeGroupVersion.WithKind("PipelineRun"): &v1beta1.PipelineRun{},
	v1beta1.SchemeGroupVersion.WithKind("CustomRun"):   &v1beta1.CustomRun{},
	v1beta1.SchemeGroupVersion.WithKind("StepAction"):  &v1beta1.StepAction{},
	// v1
	v1.SchemeGroupVersion.WithKind("Task"):        &v1.Task{},
	v1.SchemeGroupVersion.WithKind("Pipeline"):    &v1.Pipeline{},
	v1.SchemeGroupVersion.WithKind("TaskRun"):     &v1.TaskRun{},
	v1.SchemeGroupVersion.WithKind("PipelineRun"): &v1.PipelineRun{},

	// resolution
	// v1alpha1
	resolutionv1alpha1.SchemeGroupVersion.WithKind("ResolutionRequest"): &resolutionv1alpha1.ResolutionRequest{},
	// v1beta1
	resolutionv1beta1.SchemeGroupVersion.WithKind("ResolutionRequest"): &resolutionv1beta1.ResolutionRequest{},
}

// parseTLSMinVersion converts environment variable string ("1.2" or "1.3") to tls.VersionTLS constant.
// Returns 0 if envValue is empty (use webhook default).
func parseTLSMinVersion(envValue string) (uint16, error) {
	switch envValue {
	case "1.2":
		return tls.VersionTLS12, nil
	case "1.3":
		return tls.VersionTLS13, nil
	case "":
		return 0, nil // Use webhook default
	default:
		return 0, fmt.Errorf("invalid TLS_MIN_VERSION: %s (expected 1.2 or 1.3)", envValue)
	}
}

// parseCipherSuites converts comma-separated cipher suite names to their uint16 identifiers.
// Returns nil if envValue is empty (use webhook defaults).
func parseCipherSuites(envValue string) ([]uint16, error) {
	if envValue == "" {
		return nil, nil // Use webhook defaults
	}

	// Map cipher suite names to their constants.
	// support all OpenShift TLS profiles (Modern, Intermediate, Old, Custom).
	cipherMap := map[string]uint16{
		// TLS 1.2 cipher suites - GCM modes (strongest, recommended)
		"TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256":     tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
		"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256":       tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
		"TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384":     tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
		"TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384":       tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
		"TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256": tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256,
		"TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305":      tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,

		// TLS 1.2 - CBC modes (less preferred than GCM)
		"TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA256": tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA256,
		"TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA256":   tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA256,

		// TLS 1.2 - RSA key exchange (legacy, but in Intermediate/Old profiles)
		"TLS_RSA_WITH_AES_128_GCM_SHA256": tls.TLS_RSA_WITH_AES_128_GCM_SHA256,
		"TLS_RSA_WITH_AES_256_GCM_SHA384": tls.TLS_RSA_WITH_AES_256_GCM_SHA384,
		"TLS_RSA_WITH_AES_128_CBC_SHA256": tls.TLS_RSA_WITH_AES_128_CBC_SHA256,

		// TLS 1.0/1.1 - CBC modes (old, but in Old profile for legacy clients)
		"TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA": tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA,
		"TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA":   tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA,
		"TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA": tls.TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA,
		"TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA":   tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,

		// SSL 3 / TLS 1.0 - Legacy ciphers (very old, but in Old profile)
		"TLS_RSA_WITH_AES_128_CBC_SHA":  tls.TLS_RSA_WITH_AES_128_CBC_SHA,
		"TLS_RSA_WITH_AES_256_CBC_SHA":  tls.TLS_RSA_WITH_AES_256_CBC_SHA,
		"TLS_RSA_WITH_3DES_EDE_CBC_SHA": tls.TLS_RSA_WITH_3DES_EDE_CBC_SHA,
	}

	names := strings.Split(envValue, ",")
	ciphers := make([]uint16, 0, len(names))

	for _, name := range names {
		name = strings.TrimSpace(name)
		if cipher, ok := cipherMap[name]; ok {
			ciphers = append(ciphers, cipher)
		} else {
			return nil, fmt.Errorf("unknown cipher suite: %s", name)
		}
	}

	return ciphers, nil
}

// parseCurvePreferences converts comma-separated elliptic curve names to their tls.CurveID identifiers.
// Returns nil if envValue is empty (use webhook defaults).
func parseCurvePreferences(envValue string) ([]tls.CurveID, error) {
	if envValue == "" {
		return nil, nil // Use webhook defaults
	}

	curveMap := map[string]tls.CurveID{
		"X25519":    tls.X25519,
		"CurveP256": tls.CurveP256,
		"CurveP384": tls.CurveP384,
		"CurveP521": tls.CurveP521,
	}

	names := strings.Split(envValue, ",")
	curves := make([]tls.CurveID, 0, len(names))

	for _, name := range names {
		name = strings.TrimSpace(name)
		if curve, ok := curveMap[name]; ok {
			curves = append(curves, curve)
		} else {
			return nil, fmt.Errorf("unknown elliptic curve: %s", name)
		}
	}

	return curves, nil
}

func newDefaultingAdmissionController(name string) func(context.Context, configmap.Watcher) *controller.Impl {
	return func(ctx context.Context, cmw configmap.Watcher) *controller.Impl {
		// Decorate contexts with the current state of the config.
		store := defaultconfig.NewStore(logging.FromContext(ctx).Named("config-store"))
		store.WatchConfigs(cmw)
		return defaulting.NewAdmissionController(ctx,

			// Name of the resource webhook, it is the value of the environment variable WEBHOOK_ADMISSION_CONTROLLER_NAME
			// default is "webhook.pipeline.tekton.dev"
			name,

			// The path on which to serve the webhook.
			"/defaulting",

			// The resources to validate and default.
			types,

			// A function that infuses the context passed to Validate/SetDefaults with custom metadata.
			func(ctx context.Context) context.Context {
				return store.ToContext(ctx)
			},

			// Whether to disallow unknown fields.
			true,
		)
	}
}

func newValidationAdmissionController(name string) func(context.Context, configmap.Watcher) *controller.Impl {
	return func(ctx context.Context, cmw configmap.Watcher) *controller.Impl {
		// Decorate contexts with the current state of the config.
		store := defaultconfig.NewStore(logging.FromContext(ctx).Named("config-store"))
		store.WatchConfigs(cmw)
		return validation.NewAdmissionController(ctx,

			// Name of the validation webhook, it is based on the value of the environment variable WEBHOOK_ADMISSION_CONTROLLER_NAME
			// default is "validation.webhook.pipeline.tekton.dev"
			strings.Join([]string{"validation", name}, "."),

			// The path on which to serve the webhook.
			"/resource-validation",

			// The resources to validate and default.
			types,

			// A function that infuses the context passed to Validate/SetDefaults with custom metadata.
			func(ctx context.Context) context.Context {
				return store.ToContext(ctx)
			},

			// Whether to disallow unknown fields.
			true,
		)
	}
}

func newConfigValidationController(name string) func(context.Context, configmap.Watcher) *controller.Impl {
	return func(ctx context.Context, cmw configmap.Watcher) *controller.Impl {
		return configmaps.NewAdmissionController(ctx,

			// Name of the configmap webhook, it is based on the value of the environment variable WEBHOOK_ADMISSION_CONTROLLER_NAME
			// default is "config.webhook.pipeline.tekton.dev"
			strings.Join([]string{"config", name}, "."),

			// The path on which to serve the webhook.
			"/config-validation",

			// The configmaps to validate.
			configmap.Constructors{
				logging.ConfigMapName():                   logging.NewConfigFromConfigMap,
				defaultconfig.GetDefaultsConfigName():     defaultconfig.NewDefaultsFromConfigMap,
				pkgleaderelection.ConfigMapName():         pkgleaderelection.NewConfigFromConfigMap,
				defaultconfig.GetFeatureFlagsConfigName(): defaultconfig.NewFeatureFlagsFromConfigMap,
			},
		)
	}
}

func newConversionController(ctx context.Context, cmw configmap.Watcher) *controller.Impl {
	var (
		v1alpha1GroupVersion           = v1alpha1.SchemeGroupVersion.Version
		v1beta1GroupVersion            = v1beta1.SchemeGroupVersion.Version
		v1GroupVersion                 = v1.SchemeGroupVersion.Version
		resolutionv1alpha1GroupVersion = resolutionv1alpha1.SchemeGroupVersion.Version
		resolutionv1beta1GroupVersion  = resolutionv1beta1.SchemeGroupVersion.Version
	)
	// Decorate contexts with the current state of the config.
	store := defaultconfig.NewStore(logging.FromContext(ctx).Named("config-store"))
	store.WatchConfigs(cmw)
	return conversion.NewConversionController(ctx,
		// The path on which to serve the webhook
		"/resource-conversion",

		// Specify the types of custom resource definitions that should be converted
		// "HubVersion" specifies which version of the CustomResource supports
		// conversions to and from all types.
		// "Zygotes" are the supported versions.
		map[schema.GroupKind]conversion.GroupKindConversion{
			v1beta1.Kind("StepAction"): {
				DefinitionName: pipeline.StepActionResource.String(),
				HubVersion:     v1alpha1GroupVersion,
				Zygotes: map[string]conversion.ConvertibleObject{
					v1alpha1GroupVersion: &v1alpha1.StepAction{},
					v1beta1GroupVersion:  &v1beta1.StepAction{},
				},
			},
			v1.Kind("Task"): {
				DefinitionName: pipeline.TaskResource.String(),
				HubVersion:     v1beta1GroupVersion,
				Zygotes: map[string]conversion.ConvertibleObject{
					v1beta1GroupVersion: &v1beta1.Task{},
					v1GroupVersion:      &v1.Task{},
				},
			},
			v1.Kind("Pipeline"): {
				DefinitionName: pipeline.PipelineResource.String(),
				HubVersion:     v1beta1GroupVersion,
				Zygotes: map[string]conversion.ConvertibleObject{
					v1beta1GroupVersion: &v1beta1.Pipeline{},
					v1GroupVersion:      &v1.Pipeline{},
				},
			},
			v1.Kind("TaskRun"): {
				DefinitionName: pipeline.TaskRunResource.String(),
				HubVersion:     v1beta1GroupVersion,
				Zygotes: map[string]conversion.ConvertibleObject{
					v1beta1GroupVersion: &v1beta1.TaskRun{},
					v1GroupVersion:      &v1.TaskRun{},
				},
			},
			v1.Kind("PipelineRun"): {
				DefinitionName: pipeline.PipelineRunResource.String(),
				HubVersion:     v1beta1GroupVersion,
				Zygotes: map[string]conversion.ConvertibleObject{
					v1beta1GroupVersion: &v1beta1.PipelineRun{},
					v1GroupVersion:      &v1.PipelineRun{},
				},
			},
			resolutionv1beta1.Kind("ResolutionRequest"): {
				DefinitionName: resolution.ResolutionRequestResource.String(),
				HubVersion:     resolutionv1alpha1GroupVersion,
				Zygotes: map[string]conversion.ConvertibleObject{
					resolutionv1alpha1GroupVersion: &resolutionv1alpha1.ResolutionRequest{},
					resolutionv1beta1GroupVersion:  &resolutionv1beta1.ResolutionRequest{},
				},
			},
		},

		// A function that infuses the context passed to ConvertTo/ConvertFrom/SetDefaults with custom metadata
		func(ctx context.Context) context.Context {
			return store.ToContext(ctx)
		},
	)
}

func main() {
	serviceName := os.Getenv("WEBHOOK_SERVICE_NAME")
	if serviceName == "" {
		serviceName = "tekton-pipelines-webhook"
	}

	secretName := os.Getenv("WEBHOOK_SECRET_NAME")
	if secretName == "" {
		secretName = "webhook-certs" // #nosec
	}

	webhookName := os.Getenv("WEBHOOK_ADMISSION_CONTROLLER_NAME")
	if webhookName == "" {
		webhookName = "webhook.pipeline.tekton.dev"
	}

	// Parse TLS configuration from environment variables
	baseCtx := signals.NewContext()
	logger := logging.FromContext(baseCtx)
	minVersion, err := parseTLSMinVersion(os.Getenv("TLS_MIN_VERSION"))
	if err != nil {
		logger.Warnf("Invalid TLS_MIN_VERSION: %v, using webhook default", err)
		minVersion = 0
	}

	cipherSuites, err := parseCipherSuites(os.Getenv("TLS_CIPHER_SUITES"))
	if err != nil {
		logger.Warnf("Invalid TLS_CIPHER_SUITES: %v, using webhook default", err)
		cipherSuites = nil
	}

	curvePrefs, err := parseCurvePreferences(os.Getenv("TLS_CURVE_PREFERENCES"))
	if err != nil {
		logger.Warnf("Invalid TLS_CURVE_PREFERENCES: %v, using webhook default", err)
		curvePrefs = nil
	}

	// Log TLS configuration if any was provided
	if minVersion != 0 || len(cipherSuites) > 0 || len(curvePrefs) > 0 {
		logger.Infof("Applying TLS configuration: MinVersion=%d, CipherSuites=%d, CurvePreferences=%d",
			minVersion, len(cipherSuites), len(curvePrefs))
	}

	// Scope informers to the webhook's namespace instead of cluster-wide
	ctx := injection.WithNamespaceScope(baseCtx, system.Namespace())

	// Set up a signal context with our webhook options
	ctx = webhook.WithOptions(ctx, webhook.Options{
		ServiceName: serviceName,
		Port:        webhook.PortFromEnv(8443),
		SecretName:  secretName,

		TLSMinVersion:       minVersion,
		TLSCipherSuites:     cipherSuites,
		TLSCurvePreferences: curvePrefs,
	})

	mux := http.NewServeMux()

	mux.HandleFunc("/", handler)
	mux.HandleFunc("/health", handler)
	mux.HandleFunc("/readiness", handler)

	port := os.Getenv("PROBES_PORT")
	if port == "" {
		port = "8080"
	}

	go func() {
		// start the web server on port and accept requests
		log.Printf("Readiness and health check server listening on port %s", port)
		log.Fatal(http.ListenAndServe(":"+port, mux)) // #nosec G114 -- see https://github.com/securego/gosec#available-rules
	}()

	sharedmain.MainWithContext(ctx, serviceName,
		certificates.NewController,
		newDefaultingAdmissionController(webhookName),
		newValidationAdmissionController(webhookName),
		newConfigValidationController(webhookName),
		newConversionController,
	)
}

func handler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}
