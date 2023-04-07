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
	"flag"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/pkg/reconciler/customrun"
	"github.com/tektoncd/pipeline/pkg/reconciler/pipelinerun"
	"github.com/tektoncd/pipeline/pkg/reconciler/resolutionrequest"
	"github.com/tektoncd/pipeline/pkg/reconciler/taskrun"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/jaeger"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.12.0"
	"go.opentelemetry.io/otel/trace"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/utils/clock"
	filteredinformerfactory "knative.dev/pkg/client/injection/kube/informers/factory/filtered"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/injection"
	"knative.dev/pkg/injection/sharedmain"
	"knative.dev/pkg/signals"
)

const (
	// ControllerLogKey is the name of the logger for the controller cmd
	ControllerLogKey = "tekton-pipelines-controller"
	// TracerProviderPipelineRun is the name of TraceProvider used pipeline reconciler
	TracerProviderPipelineRun = "pipeline-reconciler"
	// TracerProviderTaskRun is the name of TracerProvider used in taskrun reconciler
	TracerProviderTaskRun = "taskrun-reconciler"
)

func main() {
	flag.IntVar(&controller.DefaultThreadsPerController, "threads-per-controller", controller.DefaultThreadsPerController, "Threads (goroutines) to create per controller")
	namespace := flag.String("namespace", corev1.NamespaceAll, "Namespace to restrict informer to. Optional, defaults to all namespaces.")
	disableHighAvailability := flag.Bool("disable-ha", false, "Whether to disable high-availability functionality for this component.  This flag will be deprecated "+
		"and removed when we have promoted this feature to stable, so do not pass it without filing an "+
		"issue upstream!")

	opts := &pipeline.Options{}
	flag.StringVar(&opts.Images.EntrypointImage, "entrypoint-image", "", "The container image containing our entrypoint binary.")
	flag.StringVar(&opts.Images.SidecarLogResultsImage, "sidecarlogresults-image", "", "The container image containing the binary for accessing results.")
	flag.StringVar(&opts.Images.NopImage, "nop-image", "", "The container image used to stop sidecars")
	flag.StringVar(&opts.Images.ShellImage, "shell-image", "", "The container image containing a shell")
	flag.StringVar(&opts.Images.ShellImageWin, "shell-image-win", "", "The container image containing a windows shell")
	flag.StringVar(&opts.Images.WorkingDirInitImage, "workingdirinit-image", "", "The container image containing our working dir init binary.")

	// This parses flags.
	cfg := injection.ParseAndGetRESTConfigOrDie()

	if err := opts.Images.Validate(); err != nil {
		log.Fatal(err)
	}
	if cfg.QPS == 0 {
		cfg.QPS = 2 * rest.DefaultQPS
	}
	if cfg.Burst == 0 {
		cfg.Burst = rest.DefaultBurst
	}
	// FIXME(vdemeester): this is here to not break current behavior
	// multiply by 2, no of controllers being created
	cfg.QPS = 2 * cfg.QPS
	cfg.Burst = 2 * cfg.Burst

	ctx := injection.WithNamespaceScope(signals.NewContext(), *namespace)
	if *disableHighAvailability {
		ctx = sharedmain.WithHADisabled(ctx)
	}

	// sets up liveness and readiness probes.
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

	// initialize opentelemetry
	tpPipelineRun, err := tracerProvider(TracerProviderPipelineRun)
	if err != nil {
		log.Printf("failed to initialize tracerProvider for pipelinerun, falling back to no-op provider, %s", err.Error())
		tpPipelineRun = trace.NewNoopTracerProvider()
	}
	tpTaskrun, err := tracerProvider(TracerProviderTaskRun)
	if err != nil {
		log.Printf("failed to initialize tracerProvider for taskrun, falling back to no-op provider, %s", err.Error())
		tpTaskrun = trace.NewNoopTracerProvider()
	}
	otel.SetTextMapPropagator(propagation.TraceContext{})
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	ctx = filteredinformerfactory.WithSelectors(ctx, v1beta1.ManagedByLabelKey)
	sharedmain.MainWithConfig(ctx, ControllerLogKey, cfg,
		taskrun.NewController(opts, clock.RealClock{}, tpTaskrun),
		pipelinerun.NewController(opts, clock.RealClock{}, tpPipelineRun),
		resolutionrequest.NewController(clock.RealClock{}),
		customrun.NewController(),
	)

	// Cleanly shutdown and flush telemetry when the application exits.
	defer func(ctx context.Context) {
		// Do not make the application hang when it is shutdown.
		ctx, cancel = context.WithTimeout(ctx, time.Second*5)
		defer cancel()

		// shutdown is only needed when tracerProvider is inialized with jaeger
		// not needed when tracerProvider is NewNoopTracerProvider
		if tp, ok := tpPipelineRun.(*tracesdk.TracerProvider); ok {
			if err := tp.Shutdown(ctx); err != nil {
				log.Printf("Unable to shutdown tracerProvider for pipelinerun, %s", err.Error())
			}
		}
		if tp, ok := tpTaskrun.(*tracesdk.TracerProvider); ok {
			if err := tp.Shutdown(ctx); err != nil {
				log.Printf("Unable to shutdown tracerProvider for taskrun, %s", err.Error())
			}
		}
	}(ctx)
}

func handler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

// tracerProvider returns an OpenTelemetry TracerProvider configured to use
// the Jaeger exporter that will send spans to the provided url. The returned
// TracerProvider will also use a Resource configured with all the information
// about the application.
func tracerProvider(service string) (trace.TracerProvider, error) {
	// Create the Jaeger exporter
	// The following env variables are used by the sdk for creating the exporter
	// - OTEL_EXPORTER_JAEGER_ENDPOINT is the HTTP endpoint for sending spans directly to a collector.
	// - OTEL_EXPORTER_JAEGER_USER is the username to be sent as authentication to the collector endpoint.
	// - OTEL_EXPORTER_JAEGER_PASSWORD is the password to be sent as authentication to the collector endpoint.

	if _, e := os.LookupEnv("OTEL_EXPORTER_JAEGER_ENDPOINT"); !e {
		// jaeger endpoint is not defined, disable tracing and return no-op tracerProvider
		return trace.NewNoopTracerProvider(), nil
	}

	exp, err := jaeger.New(jaeger.WithCollectorEndpoint())
	if err != nil {
		return nil, err
	}
	// Initialize tracerProvider with the jaeger exporter
	tp := tracesdk.NewTracerProvider(
		tracesdk.WithBatcher(exp),
		// Record information about the service in a Resource.
		tracesdk.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String(service),
		)),
	)
	return tp, nil
}
