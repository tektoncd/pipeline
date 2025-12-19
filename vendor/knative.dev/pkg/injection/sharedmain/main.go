/*
Copyright 2019 The Knative Authors

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

package sharedmain

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/runtime"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/trace"

	"github.com/go-logr/zapr"
	"go.uber.org/automaxprocs/maxprocs" // automatically set GOMAXPROCS based on cgroups
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/rest"
	k8stoolmetrics "k8s.io/client-go/tools/metrics"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	kubeclient "knative.dev/pkg/client/injection/kube/client"
	"knative.dev/pkg/configmap"
	cminformer "knative.dev/pkg/configmap/informer"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/injection"
	"knative.dev/pkg/leaderelection"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/logging/logkey"
	"knative.dev/pkg/observability"
	o11yconfigmap "knative.dev/pkg/observability/configmap"
	"knative.dev/pkg/observability/metrics"
	k8smetrics "knative.dev/pkg/observability/metrics/k8s"
	"knative.dev/pkg/observability/resource"
	k8sruntime "knative.dev/pkg/observability/runtime/k8s"
	"knative.dev/pkg/observability/tracing"
	"knative.dev/pkg/reconciler"
	"knative.dev/pkg/signals"
	"knative.dev/pkg/system"
	"knative.dev/pkg/version"
	"knative.dev/pkg/webhook"
)

func init() {
	maxprocs.Set()
}

// GetLoggingConfig gets the logging config from the (in order):
// 1. provided context,
// 2. reading from the API server,
// 3. defaults (if not found).
// The context is expected to be initialized with injection.
func GetLoggingConfig(ctx context.Context) (*logging.Config, error) {
	if cfg := logging.GetConfig(ctx); cfg != nil {
		return cfg, nil
	}

	var loggingConfigMap *corev1.ConfigMap
	// These timeout and retry interval are set by heuristics.
	// e.g. istio sidecar needs a few seconds to configure the pod network.
	var lastErr error
	if err := wait.PollUntilContextTimeout(ctx, 1*time.Second, 5*time.Second, true, func(ctx context.Context) (bool, error) {
		loggingConfigMap, lastErr = kubeclient.Get(ctx).CoreV1().ConfigMaps(system.Namespace()).Get(ctx, logging.ConfigMapName(), metav1.GetOptions{})
		return lastErr == nil || apierrors.IsNotFound(lastErr), nil
	}); err != nil {
		return nil, fmt.Errorf("timed out waiting for the condition: %w", lastErr)
	}
	if loggingConfigMap == nil {
		return logging.NewConfigFromMap(nil)
	}
	return logging.NewConfigFromConfigMap(loggingConfigMap)
}

// GetLeaderElectionConfig gets the leader election config from the (in order):
// 1. provided context,
// 2. reading from the API server,
// 3. defaults (if not found).
func GetLeaderElectionConfig(ctx context.Context) (*leaderelection.Config, error) {
	if cfg := leaderelection.GetConfig(ctx); cfg != nil {
		return cfg, nil
	}

	leaderElectionConfigMap, err := kubeclient.Get(ctx).CoreV1().ConfigMaps(system.Namespace()).Get(ctx, leaderelection.ConfigMapName(), metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return leaderelection.NewConfigFromConfigMap(nil)
	} else if err != nil {
		return nil, err
	}
	return leaderelection.NewConfigFromConfigMap(leaderElectionConfigMap)
}

// GetObservabilityConfig gets the observability config from the (in order):
// 1. provided context,
// 2. reading from the API server,
// 3. defaults (if not found).
func GetObservabilityConfig(ctx context.Context) (*observability.Config, error) {
	if cfg := observability.GetConfig(ctx); cfg != nil {
		return cfg, nil
	}

	client := kubeclient.Get(ctx).CoreV1().ConfigMaps(system.Namespace())
	cm, err := client.Get(ctx, o11yconfigmap.Name(), metav1.GetOptions{})

	if apierrors.IsNotFound(err) {
		return observability.DefaultConfig(), nil
	} else if err != nil {
		return nil, err
	}

	return o11yconfigmap.Parse(cm)
}

// EnableInjectionOrDie enables Knative Injection and starts the informers.
// Both Context and Config are optional.
// Deprecated: use injection.EnableInjectionOrDie
func EnableInjectionOrDie(ctx context.Context, cfg *rest.Config) context.Context {
	ctx, startInformers := injection.EnableInjectionOrDie(ctx, cfg)
	go startInformers()
	return ctx
}

// Main runs the generic main flow with a new context.
// If any of the constructed controllers are AdmissionControllers or Conversion
// webhooks, then a webhook is started to serve them.
func Main(component string, ctors ...injection.ControllerConstructor) {
	// Set up signals so we handle the first shutdown signal gracefully.
	MainWithContext(signals.NewContext(), component, ctors...)
}

// Legacy aliases for back-compat.
var (
	WebhookMainWithContext = MainWithContext
	WebhookMainWithConfig  = MainWithConfig
)

// MainNamed runs the generic main flow for controllers and webhooks.
//
// In addition to the MainWithConfig flow, it defines a `disabled-controllers` flag that allows disabling controllers
// by name.
func MainNamed(ctx context.Context, component string, ctors ...injection.NamedControllerConstructor) {
	disabledControllers := flag.String("disable-controllers", "", "Comma-separated list of disabled controllers.")

	// HACK: This parses flags, so the above should be set once this runs.
	cfg := injection.ParseAndGetRESTConfigOrDie()

	enabledCtors := enabledControllers(strings.Split(*disabledControllers, ","), ctors)
	MainWithConfig(ctx, component, cfg, toControllerConstructors(enabledCtors)...)
}

func enabledControllers(disabledControllers []string, ctors []injection.NamedControllerConstructor) []injection.NamedControllerConstructor {
	disabledControllersSet := sets.NewString(disabledControllers...)
	activeCtors := make([]injection.NamedControllerConstructor, 0, len(ctors))
	for _, ctor := range ctors {
		if disabledControllersSet.Has(ctor.Name) {
			log.Printf("Disabling controller %s", ctor.Name)
			continue
		}
		activeCtors = append(activeCtors, ctor)
	}
	return activeCtors
}

func toControllerConstructors(namedCtors []injection.NamedControllerConstructor) []injection.ControllerConstructor {
	ctors := make([]injection.ControllerConstructor, 0, len(namedCtors))
	for _, ctor := range namedCtors {
		ctors = append(ctors, ctor.ControllerConstructor)
	}
	return ctors
}

// MainWithContext runs the generic main flow for controllers and
// webhooks. Use MainWithContext if you do not need to serve webhooks.
func MainWithContext(ctx context.Context, component string, ctors ...injection.ControllerConstructor) {
	// Allow configuration of threads per controller
	if val, ok := os.LookupEnv("K_THREADS_PER_CONTROLLER"); ok {
		threadsPerController, err := strconv.Atoi(val)
		if err != nil {
			log.Fatalf("failed to parse value %q of K_THREADS_PER_CONTROLLER: %v\n", val, err)
		}
		controller.DefaultThreadsPerController = threadsPerController
	}

	// TODO(mattmoor): Remove this once HA is stable.
	disableHighAvailability := flag.Bool("disable-ha", false,
		"Whether to disable high-availability functionality for this component.  This flag will be deprecated "+
			"and removed when we have promoted this feature to stable, so do not pass it without filing an "+
			"issue upstream!")

	// HACK: This parses flags, so the above should be set once this runs.
	cfg := injection.ParseAndGetRESTConfigOrDie()

	if *disableHighAvailability {
		ctx = WithHADisabled(ctx)
	}

	MainWithConfig(ctx, component, cfg, ctors...)
}

type haDisabledKey struct{}

// WithHADisabled signals to MainWithConfig that it should not set up an appropriate leader elector for this component.
func WithHADisabled(ctx context.Context) context.Context {
	return context.WithValue(ctx, haDisabledKey{}, struct{}{})
}

// IsHADisabled checks the context for the desired to disabled leader elector.
func IsHADisabled(ctx context.Context) bool {
	return ctx.Value(haDisabledKey{}) != nil
}

// MainWithConfig runs the generic main flow for controllers and webhooks
// with the given config.
func MainWithConfig(ctx context.Context, component string, cfg *rest.Config, ctors ...injection.ControllerConstructor) {
	log.Printf("Registering %d clients", len(injection.Default.GetClients()))
	log.Printf("Registering %d informer factories", len(injection.Default.GetInformerFactories()))
	log.Printf("Registering %d informers", len(injection.Default.GetInformers()))
	log.Printf("Registering %d controllers", len(ctors))

	// Respect user provided settings, but if omitted customize the default behavior.
	if cfg.QPS == 0 {
		// Adjust our client's rate limits based on the number of controllers we are running.
		cfg.QPS = float32(len(ctors)) * rest.DefaultQPS
	}
	if cfg.Burst == 0 {
		cfg.Burst = len(ctors) * rest.DefaultBurst
	}

	ctx, startInformers := injection.EnableInjectionOrDie(ctx, cfg)

	logger, atomicLevel := SetupLoggerOrDie(ctx, component)
	defer logger.Sync()
	ctx = logging.WithLogger(ctx, logger)

	klog.SetLogger(zapr.NewLogger(logger.Desugar()))

	// Override client-go's warning handler to give us nicely printed warnings.
	rest.SetDefaultWarningHandler(&logging.WarningHandler{Logger: logger})

	pprof := k8sruntime.NewProfilingServer(logger.Named("pprof"))

	CheckK8sClientMinimumVersionOrDie(ctx, logger)
	cmw := SetupConfigMapWatchOrDie(ctx, logger)

	// Set up leader election config
	leaderElectionConfig, err := GetLeaderElectionConfig(ctx)
	if err != nil {
		logger.Fatal("Error loading leader election configuration: ", err)
	}

	if !IsHADisabled(ctx) {
		// Signal that we are executing in a context with leader election.
		ctx = leaderelection.WithDynamicLeaderElectorBuilder(ctx, kubeclient.Get(ctx),
			leaderElectionConfig.GetComponentConfig(component))
	}

	mp, tp := SetupObservabilityOrDie(ctx, component, logger, pprof)
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := mp.Shutdown(ctx); err != nil {
			logger.Errorw("Error flushing metrics", zap.Error(err))
		}
		if err := tp.Shutdown(ctx); err != nil {
			logger.Errorw("Error flushing traces", zap.Error(err))
		}
	}()

	controllers, webhooks := ControllersAndWebhooksFromCtors(ctx, cmw, ctors...)
	WatchLoggingConfigOrDie(ctx, cmw, logger, atomicLevel, component)
	WatchObservabilityConfigOrDie(ctx, cmw, pprof, logger, component)

	eg, egCtx := errgroup.WithContext(ctx)
	eg.Go(pprof.ListenAndServe)

	// Many of the webhooks rely on configuration, e.g. configurable defaults, feature flags.
	// So make sure that we have synchronized our configuration state before launching the
	// webhooks, so that things are properly initialized.
	logger.Info("Starting configmap watcher...")
	if err := cmw.Start(ctx.Done()); err != nil {
		logger.Fatalw("Failed to start configmap watcher", zap.Error(err))
	}

	// If we have one or more admission controllers, then start the webhook
	// and pass them in.
	var wh *webhook.Webhook
	if len(webhooks) > 0 {
		wh, err = webhook.New(ctx, webhooks)
		if err != nil {
			logger.Fatalw("Failed to create webhook", zap.Error(err))
		}
		eg.Go(func() error {
			return wh.Run(ctx.Done())
		})
	}

	// Start the injection clients and informers.
	startInformers()

	// Wait for webhook informers to sync.
	if wh != nil {
		wh.InformersHaveSynced()
	}
	logger.Info("Starting controllers...")
	eg.Go(func() error {
		return controller.StartAll(ctx, controllers...)
	})

	// Setup default health checks to catch issues with cache sync etc.
	if !healthProbesDisabled(ctx) {
		eg.Go(func() error {
			return injection.ServeHealthProbes(ctx, injection.HealthCheckDefaultPort)
		})
	}

	// This will block until either a signal arrives or one of the grouped functions
	// returns an error.
	<-egCtx.Done()

	pprof.Shutdown(context.Background())

	// Don't forward ErrServerClosed as that indicates we're already shutting down.
	if err := eg.Wait(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Errorw("Error while running server", zap.Error(err))
	}
}

type healthProbesDisabledKey struct{}

// WithHealthProbesDisabled signals to MainWithContext that it should disable default probes (readiness and liveness).
func WithHealthProbesDisabled(ctx context.Context) context.Context {
	return context.WithValue(ctx, healthProbesDisabledKey{}, struct{}{})
}

func healthProbesDisabled(ctx context.Context) bool {
	return ctx.Value(healthProbesDisabledKey{}) != nil
}

// SetupLoggerOrDie sets up the logger using the config from the given context
// and returns a logger and atomic level, or dies by calling log.Fatalf.
func SetupLoggerOrDie(ctx context.Context, component string) (*zap.SugaredLogger, zap.AtomicLevel) {
	loggingConfig, err := GetLoggingConfig(ctx)
	if err != nil {
		log.Fatal("Error reading/parsing logging configuration: ", err)
	}
	l, level := logging.NewLoggerFromConfig(loggingConfig, component)

	if pn := system.PodName(); pn != "" {
		l = l.With(zap.String(logkey.Pod, pn))
	}

	return l, level
}

// SetupObservabilityOrDie sets up the observability using the config from the given context
// or dies by calling log.Fatalf.
func SetupObservabilityOrDie(
	ctx context.Context,
	component string,
	logger *zap.SugaredLogger,
	pprof *k8sruntime.ProfilingServer,
) (*metrics.MeterProvider, *tracing.TracerProvider) {
	cfg, err := GetObservabilityConfig(ctx)
	if err != nil {
		logger.Fatal("Error loading observability configuration: ", err)
	}

	pprof.UpdateFromConfig(cfg.Runtime)

	resource := resource.Default(component)

	meterProvider, err := metrics.NewMeterProvider(
		ctx,
		cfg.Metrics,
		metric.WithView(OTelViews(ctx)...),
		metric.WithResource(resource),
	)
	if err != nil {
		logger.Fatalw("Failed to setup meter provider", zap.Error(err))
	}

	otel.SetMeterProvider(meterProvider)

	workQueueMetrics, err := k8smetrics.NewWorkqueueMetricsProvider(
		k8smetrics.WithMeterProvider(meterProvider),
	)
	if err != nil {
		logger.Fatalw("Failed to setup k8s workqueue metrics", zap.Error(err))
	}

	workqueue.SetProvider(workQueueMetrics)
	controller.SetMetricsProvider(workQueueMetrics)

	clientMetrics, err := k8smetrics.NewClientMetricProvider(
		k8smetrics.WithMeterProvider(meterProvider),
	)
	if err != nil {
		logger.Fatalw("Failed to setup k8s client go metrics", zap.Error(err))
	}

	k8stoolmetrics.Register(k8stoolmetrics.RegisterOpts{
		RequestLatency: clientMetrics.RequestLatencyMetric(),
		RequestResult:  clientMetrics.RequestResultMetric(),
	})

	err = runtime.Start(
		runtime.WithMinimumReadMemStatsInterval(cfg.Runtime.ExportInterval),
	)
	if err != nil {
		logger.Fatalw("Failed to start runtime metrics", zap.Error(err))
	}

	tracerProvider, err := tracing.NewTracerProvider(
		ctx,
		cfg.Tracing,
		trace.WithResource(resource),
	)
	if err != nil {
		logger.Fatalw("Failed to setup trace provider", zap.Error(err))
	}

	otel.SetTextMapPropagator(tracing.DefaultTextMapPropagator())
	otel.SetTracerProvider(tracerProvider)

	return meterProvider, tracerProvider
}

// CheckK8sClientMinimumVersionOrDie checks that the hosting Kubernetes cluster
// is at least the minimum allowable version or dies by calling log.Fatalw.
func CheckK8sClientMinimumVersionOrDie(ctx context.Context, logger *zap.SugaredLogger) {
	kc := kubeclient.Get(ctx)
	if err := version.CheckMinimumVersion(kc.Discovery()); err != nil {
		logger.Fatalw("Version check failed", zap.Error(err))
	}
}

// SetupConfigMapWatchOrDie establishes a watch of the configmaps in the system
// namespace that are labeled to be watched or dies by calling log.Fatalw.
func SetupConfigMapWatchOrDie(ctx context.Context, logger *zap.SugaredLogger) *cminformer.InformedWatcher {
	kc := kubeclient.Get(ctx)
	// Create ConfigMaps watcher with optional label-based filter.
	var cmLabelReqs []labels.Requirement
	if cmLabel := system.ResourceLabel(); cmLabel != "" {
		req, err := cminformer.FilterConfigByLabelExists(cmLabel)
		if err != nil {
			logger.Fatalw("Failed to generate requirement for label "+cmLabel, zap.Error(err))
		}
		logger.Info("Setting up ConfigMap watcher with label selector: ", req)
		cmLabelReqs = append(cmLabelReqs, *req)
	}
	// TODO(mattmoor): This should itself take a context and be injection-based.
	return cminformer.NewInformedWatcher(kc, system.Namespace(), cmLabelReqs...)
}

// WatchLoggingConfigOrDie establishes a watch of the logging config or dies by
// calling log.Fatalw. Note, if the config does not exist, it will be defaulted
// and this method will not die.
func WatchLoggingConfigOrDie(ctx context.Context, cmw *cminformer.InformedWatcher, logger *zap.SugaredLogger, atomicLevel zap.AtomicLevel, component string) {
	if _, err := kubeclient.Get(ctx).CoreV1().ConfigMaps(system.Namespace()).Get(ctx, logging.ConfigMapName(),
		metav1.GetOptions{}); err == nil {
		cmw.Watch(logging.ConfigMapName(), logging.UpdateLevelFromConfigMap(logger, atomicLevel, component))
	} else if !apierrors.IsNotFound(err) {
		logger.Fatalw("Error reading ConfigMap "+logging.ConfigMapName(), zap.Error(err))
	}
}

// WatchObservabilityConfigOrDie establishes a watch of the observability config
// or dies by calling log.Fatalw. Note, if the config does not exist, it will be
// defaulted and this method will not die.
func WatchObservabilityConfigOrDie(
	ctx context.Context,
	cmw *cminformer.InformedWatcher,
	pprof *k8sruntime.ProfilingServer,
	logger *zap.SugaredLogger,
	component string,
) {
	cmName := o11yconfigmap.Name()
	client := kubeclient.Get(ctx).CoreV1().ConfigMaps(system.Namespace())

	observers := []configmap.Observer{
		pprof.UpdateFromConfigMap,
	}

	if _, err := client.Get(ctx, cmName, metav1.GetOptions{}); err == nil {
		cmw.Watch(cmName, observers...)
	} else if !apierrors.IsNotFound(err) {
		logger.Fatalw("Error reading ConfigMap "+cmName, zap.Error(err))
	}
}

// ControllersAndWebhooksFromCtors returns a list of the controllers and a list
// of the webhooks created from the given constructors.
func ControllersAndWebhooksFromCtors(ctx context.Context,
	cmw *cminformer.InformedWatcher,
	ctors ...injection.ControllerConstructor,
) ([]*controller.Impl, []any) {
	// Check whether the context has been infused with a leader elector builder.
	// If it has, then every reconciler we plan to start MUST implement LeaderAware.
	leEnabled := leaderelection.HasLeaderElection(ctx)

	controllers := make([]*controller.Impl, 0, len(ctors))
	webhooks := make([]any, 0)
	for _, cf := range ctors {
		ctrl := cf(ctx, cmw)
		controllers = append(controllers, ctrl)

		// Build a list of any reconcilers that implement webhook.AdmissionController
		switch c := ctrl.Reconciler.(type) {
		case webhook.AdmissionController, webhook.ConversionController:
			webhooks = append(webhooks, c)
		}

		if leEnabled {
			if _, ok := ctrl.Reconciler.(reconciler.LeaderAware); !ok {
				log.Fatalf("%T is not leader-aware, all reconcilers must be leader-aware to enable fine-grained leader election.", ctrl.Reconciler)
			}
		}
	}

	return controllers, webhooks
}
