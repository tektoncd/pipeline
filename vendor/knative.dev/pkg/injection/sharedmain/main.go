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
	"log"
	"net/http"
	"os"
	"os/user"
	"path/filepath"
	"time"

	"go.opencensus.io/stats/view"
	"golang.org/x/sync/errgroup"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog"

	_ "go.uber.org/automaxprocs" // automatically set GOMAXPROCS based on cgroups
	"go.uber.org/zap"

	kubeclient "knative.dev/pkg/client/injection/kube/client"
	"knative.dev/pkg/configmap"
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

// GetConfig returns a rest.Config to be used for kubernetes client creation.
// It does so in the following order:
//   1. Use the passed kubeconfig/serverURL.
//   2. Fallback to the KUBECONFIG environment variable.
//   3. Fallback to in-cluster config.
//   4. Fallback to the ~/.kube/config.
func GetConfig(serverURL, kubeconfig string) (*rest.Config, error) {
	if kubeconfig == "" {
		kubeconfig = os.Getenv("KUBECONFIG")
	}

	// We produce configs a bunch of ways, this gives us a single place
	// to "decorate" them with common useful things (e.g. for debugging)
	decorate := func(cfg *rest.Config) *rest.Config {
		return cfg
	}

	// If we have an explicit indication of where the kubernetes config lives, read that.
	if kubeconfig != "" {
		c, err := clientcmd.BuildConfigFromFlags(serverURL, kubeconfig)
		if err != nil {
			return nil, err
		}
		return decorate(c), nil
	}
	// If not, try the in-cluster config.
	if c, err := rest.InClusterConfig(); err == nil {
		return decorate(c), nil
	}
	// If no in-cluster config, try the default location in the user's home directory.
	if usr, err := user.Current(); err == nil {
		if c, err := clientcmd.BuildConfigFromFlags("", filepath.Join(usr.HomeDir, ".kube", "config")); err == nil {
			return decorate(c), nil
		}
	}

	return nil, errors.New("could not create a valid kubeconfig")
}

// GetLoggingConfig gets the logging config from either the file system if present
// or via reading a configMap from the API.
// The context is expected to be initialized with injection.
func GetLoggingConfig(ctx context.Context) (*logging.Config, error) {
	var loggingConfigMap *corev1.ConfigMap
	// These timeout and retry interval are set by heuristics.
	// e.g. istio sidecar needs a few seconds to configure the pod network.
	if err := wait.PollImmediate(1*time.Second, 5*time.Second, func() (bool, error) {
		var err error
		loggingConfigMap, err = kubeclient.Get(ctx).CoreV1().ConfigMaps(system.Namespace()).Get(ctx, logging.ConfigMapName(), metav1.GetOptions{})
		return err == nil || apierrors.IsNotFound(err), nil
	}); err != nil {
		return nil, err
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
func EnableInjectionOrDie(ctx context.Context, cfg *rest.Config) context.Context {
	if ctx == nil {
		ctx = signals.NewContext()
	}
	if cfg == nil {
		cfg = ParseAndGetConfigOrDie()
	}

	// Respect user provided settings, but if omitted customize the default behavior.
	if cfg.QPS == 0 {
		cfg.QPS = rest.DefaultQPS
	}
	if cfg.Burst == 0 {
		cfg.Burst = rest.DefaultBurst
	}
	ctx = injection.WithConfig(ctx, cfg)

	ctx, informers := injection.Default.SetupInformers(ctx, cfg)

	// Start the injection clients and informers.
	logging.FromContext(ctx).Info("Starting informers...")
	go func(ctx context.Context) {
		if err := controller.StartInformers(ctx.Done(), informers...); err != nil {
			logging.FromContext(ctx).Fatalw("Failed to start informers", zap.Error(err))
		}
		<-ctx.Done()
	}(ctx)

	return ctx
}

// Main runs the generic main flow with a new context.
// If any of the contructed controllers are AdmissionControllers or Conversion webhooks,
// then a webhook is started to serve them.
func Main(component string, ctors ...injection.ControllerConstructor) {
	// Set up signals so we handle the first shutdown signal gracefully.
	MainWithContext(signals.NewContext(), component, ctors...)
}

// Legacy aliases for back-compat.
var (
	WebhookMainWithContext = MainWithContext
	WebhookMainWithConfig  = MainWithConfig
)

// MainWithContext runs the generic main flow for controllers and
// webhooks. Use MainWithContext if you do not need to serve webhooks.
func MainWithContext(ctx context.Context, component string, ctors ...injection.ControllerConstructor) {

	// TODO(mattmoor): Remove this once HA is stable.
	disableHighAvailability := flag.Bool("disable-ha", false,
		"Whether to disable high-availability functionality for this component.  This flag will be deprecated "+
			"and removed when we have promoted this feature to stable, so do not pass it without filing an "+
			"issue upstream!")

	// HACK: This parses flags, so the above should be set once this runs.
	cfg := ParseAndGetConfigOrDie()

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

	MemStatsOrDie(ctx)

	// Respect user provided settings, but if omitted customize the default behavior.
	if cfg.QPS == 0 {
		// Adjust our client's rate limits based on the number of controllers we are running.
		cfg.QPS = float32(len(ctors)) * rest.DefaultQPS
	}
	if cfg.Burst == 0 {
		cfg.Burst = len(ctors) * rest.DefaultBurst
	}

	ctx = EnableInjectionOrDie(ctx, cfg)

	logger, atomicLevel := SetupLoggerOrDie(ctx, component)
	defer flush(logger)
	ctx = logging.WithLogger(ctx, logger)
	profilingHandler := profiling.NewHandler(logger, false)
	profilingServer := profiling.NewServer(profilingHandler)

	CheckK8sClientMinimumVersionOrDie(ctx, logger)
	cmw := SetupConfigMapWatchOrDie(ctx, logger)

	// Set up leader election config
	leaderElectionConfig, err := GetLeaderElectionConfig(ctx)
	if err != nil {
		logger.Fatalf("Error loading leader election configuration: %v", err)
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
	// So make sure that we have synchonized our configuration state before launching the
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
	if err := eg.Wait(); err != nil && err != http.ErrServerClosed {
		logger.Errorw("Error while running server", zap.Error(err))
	}
}

func flush(logger *zap.SugaredLogger) {
	logger.Sync()
	metrics.FlushExporter()
}

// ParseAndGetConfigOrDie parses the rest config flags and creates a client or
// dies by calling log.Fatalf.
func ParseAndGetConfigOrDie() *rest.Config {
	var (
		serverURL = flag.String("server", "",
			"The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
		kubeconfig = flag.String("kubeconfig", "",
			"Path to a kubeconfig. Only required if out-of-cluster.")
	)
	klog.InitFlags(flag.CommandLine)
	flag.Parse()

	cfg, err := GetConfig(*serverURL, *kubeconfig)
	if err != nil {
		log.Fatalf("Error building kubeconfig: %v", err)
	}

	return cfg
}

// MemStatsOrDie sets up reporting on Go memory usage every 30 seconds or dies
// by calling log.Fatalf.
func MemStatsOrDie(ctx context.Context) {
	msp := metrics.NewMemStatsAll()
	msp.Start(ctx, 30*time.Second)

	if err := view.Register(msp.DefaultViews()...); err != nil {
		log.Fatalf("Error exporting go memstats view: %v", err)
	}
}

// SetupLoggerOrDie sets up the logger using the config from the given context
// and returns a logger and atomic level, or dies by calling log.Fatalf.
func SetupLoggerOrDie(ctx context.Context, component string) (*zap.SugaredLogger, zap.AtomicLevel) {
	loggingConfig, err := GetLoggingConfig(ctx)
	if err != nil {
		log.Fatalf("Error reading/parsing logging configuration: %v", err)
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
func SetupConfigMapWatchOrDie(ctx context.Context, logger *zap.SugaredLogger) *configmap.InformedWatcher {
	kc := kubeclient.Get(ctx)
	// Create ConfigMaps watcher with optional label-based filter.
	var cmLabelReqs []labels.Requirement
	if cmLabel := system.ResourceLabel(); cmLabel != "" {
		req, err := configmap.FilterConfigByLabelExists(cmLabel)
		if err != nil {
			logger.Fatalw("Failed to generate requirement for label "+cmLabel, zap.Error(err))
		}
		logger.Info("Setting up ConfigMap watcher with label selector: ", req)
		cmLabelReqs = append(cmLabelReqs, *req)
	}
	// TODO(mattmoor): This should itself take a context and be injection-based.
	return configmap.NewInformedWatcher(kc, system.Namespace(), cmLabelReqs...)
}

// WatchLoggingConfigOrDie establishes a watch of the logging config or dies by
// calling log.Fatalw. Note, if the config does not exist, it will be defaulted
// and this method will not die.
func WatchLoggingConfigOrDie(ctx context.Context, cmw *configmap.InformedWatcher, logger *zap.SugaredLogger, atomicLevel zap.AtomicLevel, component string) {
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
func WatchObservabilityConfigOrDie(ctx context.Context, cmw *configmap.InformedWatcher, profilingHandler *profiling.Handler, logger *zap.SugaredLogger, component string) {
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
	cmw *configmap.InformedWatcher,
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
