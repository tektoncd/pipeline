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
	"fmt"
	"log"
	"net/http"
	"os"

	defaultconfig "github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	resolutionv1alpha1 "github.com/tektoncd/pipeline/pkg/apis/resolution/v1alpha1"
	resourcev1alpha1 "github.com/tektoncd/pipeline/pkg/apis/resource/v1alpha1"
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

var pipelineTypes = map[schema.GroupVersionKind]resourcesemantics.GenericCRD{
	// v1alpha1
	v1alpha1.SchemeGroupVersion.WithKind("PipelineResource"): &resourcev1alpha1.PipelineResource{},
	v1alpha1.SchemeGroupVersion.WithKind("Run"):              &v1alpha1.Run{},
	// v1beta1
	v1beta1.SchemeGroupVersion.WithKind("Pipeline"):    &v1beta1.Pipeline{},
	v1beta1.SchemeGroupVersion.WithKind("Task"):        &v1beta1.Task{},
	v1beta1.SchemeGroupVersion.WithKind("ClusterTask"): &v1beta1.ClusterTask{},
	v1beta1.SchemeGroupVersion.WithKind("TaskRun"):     &v1beta1.TaskRun{},
	v1beta1.SchemeGroupVersion.WithKind("PipelineRun"): &v1beta1.PipelineRun{},
}

var resolutionTypes = map[schema.GroupVersionKind]resourcesemantics.GenericCRD{
	// v1alpha1
	resolutionv1alpha1.SchemeGroupVersion.WithKind("ResolutionRequest"): &resolutionv1alpha1.ResolutionRequest{},
}

func newDefaultingAdmissionController(resourceBase string, types map[schema.GroupVersionKind]resourcesemantics.GenericCRD) func(context.Context, configmap.Watcher) *controller.Impl {
	return func(ctx context.Context, cmw configmap.Watcher) *controller.Impl {
		// Decorate contexts with the current state of the config.
		store := defaultconfig.NewStore(logging.FromContext(ctx).Named("config-store"))
		store.WatchConfigs(cmw)

		return defaulting.NewAdmissionController(ctx,

			// Name of the resource webhook.
			fmt.Sprintf("webhook.%s.tekton.dev", resourceBase),

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

func newValidationAdmissionController(resourceBase string, types map[schema.GroupVersionKind]resourcesemantics.GenericCRD) func(context.Context, configmap.Watcher) *controller.Impl {
	return func(ctx context.Context, cmw configmap.Watcher) *controller.Impl {
		// Decorate contexts with the current state of the config.
		store := defaultconfig.NewStore(logging.FromContext(ctx).Named("config-store"))
		store.WatchConfigs(cmw)
		return validation.NewAdmissionController(ctx,

			// Name of the resource webhook.
			fmt.Sprintf("validation.webhook.%s.tekton.dev", resourceBase),

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

func newConfigValidationController(resourceBase string) func(context.Context, configmap.Watcher) *controller.Impl {
	return func(ctx context.Context, cmw configmap.Watcher) *controller.Impl {
		return configmaps.NewAdmissionController(ctx,

			// Name of the configmap webhook.
			fmt.Sprintf("config.webhook.%s.tekton.dev", resourceBase),

			// The path on which to serve the webhook.
			"/config-validation",

			// The configmaps to validate.
			configmap.Constructors{
				logging.ConfigMapName():               logging.NewConfigFromConfigMap,
				defaultconfig.GetDefaultsConfigName(): defaultconfig.NewDefaultsFromConfigMap,
				pkgleaderelection.ConfigMapName():     pkgleaderelection.NewConfigFromConfigMap,
			},
		)
	}
}

func newConversionController(ctx context.Context, cmw configmap.Watcher) *controller.Impl {
	return conversion.NewConversionController(ctx,
		// The path on which to serve the webhook
		"/resource-conversion",

		// Specify the types of custom resource definitions that should be converted
		map[schema.GroupKind]conversion.GroupKindConversion{},

		// A function that infuses the context passed to ConvertTo/ConvertFrom/SetDefaults with custom metadata
		func(ctx context.Context) context.Context {
			return ctx
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

	// Scope informers to the webhook's namespace instead of cluster-wide
	ctx := injection.WithNamespaceScope(signals.NewContext(), system.Namespace())

	// Set up a signal context with our webhook options
	ctx = webhook.WithOptions(ctx, webhook.Options{
		ServiceName: serviceName,
		Port:        8443,
		SecretName:  secretName,
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
		log.Fatal(http.ListenAndServe(":"+port, mux))
	}()

	sharedmain.MainWithContext(ctx, serviceName,
		certificates.NewController,
		newDefaultingAdmissionController("pipeline", pipelineTypes),
		newDefaultingAdmissionController("resolution", resolutionTypes),
		newValidationAdmissionController("pipeline", pipelineTypes),
		newValidationAdmissionController("resolution", resolutionTypes),
		newConfigValidationController("pipeline"),
		newConfigValidationController("resolution"),
		newConversionController,
	)
}

func handler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}
