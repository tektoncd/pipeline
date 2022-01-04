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
	"time"

	"github.com/spf13/pflag"

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

	kubeclient "knative.dev/pkg/client/injection/kube/client"
	cminformer "knative.dev/pkg/configmap/informer"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/injection"
	"knative.dev/pkg/leaderelection"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/logging/logkey"
	"knative.dev/pkg/metrics"
	"knative.dev/pkg/profiling"
	"knative.dev/pkg/reconciler"
	"knative.dev/pkg/signals"
	"knative.dev/pkg/system"
	"knative.dev/pkg/version"
	"knative.dev/pkg/webhook"
)

func init() {
	maxprocs.Set()
}

// GetLoggingConfig gets the logging config from either the file system if present
// or via reading a configMap from the API.
// The context is expected to be initialized with injection.
func GetLoggingConfig(ctx context.Context) (*logging.Config, error) {
	var loggingConfigMap *corev1.ConfigMap
	// These timeout and retry interval are set by heuristics.
	// e.g. istio sidecar needs a few seconds to configure the pod network.
	var lastErr error
	if err := wait.PollImmediate(1*time.Second, 5*time.Second, func() (bool, error) {
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

// GetLeaderElectionConfig gets the leader election config.
func GetLeaderElectionConfig(ctx context.Context) (*leaderelection.Config, error) {
	leaderElectionConfigMap, err := kubeclient.Get(ctx).CoreV1().ConfigMaps(system.Namespace()).Get(ctx, leaderelection.ConfigMapName(), metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return leaderelection.NewConfigFromConfigMap(nil)
	} else if err != nil {
		return nil, err
	}
	return leaderelection.NewConfigFromConfigMap(leaderElectionConfigMap)
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

	disabledControllers := pflag.StringSlice("disable-controllers", []string{}, "Comma-separated list of disabled controllers.")

	// HACK: This parses flags, so the above should be set once this runs.
	cfg := injection.ParseAndGetRESTConfigOrDie()

	enabledCtors := enabledControllers(*disabledControllers, ctors)

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

	metrics.MemStatsOrDie(ctx)

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
	defer flush(logger)
	ctx = logging.WithLogger(ctx, logger)

	// Override client-go's warning handler to give us nicely printed warnings.
	rest.SetDefaultWarningHandler(&logging.WarningHandler{Logger: logger})

	profilingHandler := profiling.NewHandler(logger, false)
	profilingServer := profiling.NewServer(profilingHandler)

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

	controllers, webhooks := ControllersAndWebhooksFromCtors(ctx, cmw, ctors...)
	WatchLoggingConfigOrDie(ctx, cmw, logger, atomicLevel, component)
	WatchObservabilityConfigOrDie(ctx, cmw, profilingHandler, logger, component)

	eg, egCtx := errgroup.WithContext(ctx)
	eg.Go(profilingServer.ListenAndServe)

	// Many of the webhooks rely on configuration, e.g. configurable defaults, feature flags.
	// So make sure that we have synchronized our configuration state before launching the
	// webhooks, so that things are properly initialized.
	logger.Info("Starting configuration manager...")
	if err := cmw.Start(ctx.Done()); err != nil {
		logger.Fatalw("Failed to start configuration manager", zap.Error(err))
	}

	// If we have one or more admission controllers, then start the webhook
	// and pass them in.
	var wh *webhook.Webhook
	if len(webhooks) > 0 {
		// Register webhook metrics
		webhook.RegisterMetrics()

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
	go controller.StartAll(ctx, controllers...)

	// This will block until either a signal arrives or one of the grouped functions
	// returns an error.
	<-egCtx.Done()

	profilingServer.Shutdown(context.Background())
	// Don't forward ErrServerClosed as that indicates we're already shutting down.
	if err := eg.Wait(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Errorw("Error while running server", zap.Error(err))
	}
}

func flush(logger *zap.SugaredLogger) {
	logger.Sync()
	metrics.FlushExporter()
}

// SetupLoggerOrDie sets up the logger using the config from the given context
// and returns a logger and atomic level, or dies by calling log.Fatalf.
func SetupLoggerOrDie(ctx context.Context, component string) (*zap.SugaredLogger, zap.AtomicLevel) {
	loggingConfig, err := GetLoggingConfig(ctx)
	if err != nil {
		log.Fatal("Error reading/parsing logging configuration: ", err)
	}
	l, level := logging.NewLoggerFromConfig(loggingConfig, component)

	// If PodName is injected into the env vars, set it on the logger.
	// This is needed for HA components to distinguish logs from different
	// pods.
	if pn := os.Getenv("POD_NAME"); pn != "" {
		l = l.With(zap.String(logkey.Pod, pn))
	}

	return l, level
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
func WatchObservabilityConfigOrDie(ctx context.Context, cmw *cminformer.InformedWatcher, profilingHandler *profiling.Handler, logger *zap.SugaredLogger, component string) {
	if _, err := kubeclient.Get(ctx).CoreV1().ConfigMaps(system.Namespace()).Get(ctx, metrics.ConfigMapName(),
		metav1.GetOptions{}); err == nil {
		cmw.Watch(metrics.ConfigMapName(),
			metrics.ConfigMapWatcher(ctx, component, SecretFetcher(ctx), logger),
			profilingHandler.UpdateFromConfigMap)
	} else if !apierrors.IsNotFound(err) {
		logger.Fatalw("Error reading ConfigMap "+metrics.ConfigMapName(), zap.Error(err))
	}
}

// SecretFetcher provides a helper function to fetch individual Kubernetes
// Secrets (for example, a key for client-side TLS). Note that this is not
// intended for high-volume usage; the current use is when establishing a
// metrics client connection in WatchObservabilityConfigOrDie.
func SecretFetcher(ctx context.Context) metrics.SecretFetcher {
	// NOTE: Do not use secrets.Get(ctx) here to get a SecretLister, as it will register
	// a *global* SecretInformer and require cluster-level `secrets.list` permission,
	// even if you scope down the Lister to a given namespace after requesting it. Instead,
	// we package up a function from kubeclient.
	// TODO(evankanderson): If this direct request to the apiserver on each TLS connection
	// to the opencensus agent is too much load, switch to a cached Secret.
	return func(name string) (*corev1.Secret, error) {
		return kubeclient.Get(ctx).CoreV1().Secrets(system.Namespace()).Get(ctx, name, metav1.GetOptions{})
	}
}

// ControllersAndWebhooksFromCtors returns a list of the controllers and a list
// of the webhooks created from the given constructors.
func ControllersAndWebhooksFromCtors(ctx context.Context,
	cmw *cminformer.InformedWatcher,
	ctors ...injection.ControllerConstructor) ([]*controller.Impl, []interface{}) {

	// Check whether the context has been infused with a leader elector builder.
	// If it has, then every reconciler we plan to start MUST implement LeaderAware.
	leEnabled := leaderelection.HasLeaderElection(ctx)

	controllers := make([]*controller.Impl, 0, len(ctors))
	webhooks := make([]interface{}, 0)
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
