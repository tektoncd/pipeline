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
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/user"
	"path/filepath"
	"time"

	"go.opencensus.io/stats/view"
	"golang.org/x/sync/errgroup"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/client-go/tools/record"

	"go.uber.org/zap"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/injection"
	kle "knative.dev/pkg/leaderelection"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/metrics"
	"knative.dev/pkg/profiling"
	"knative.dev/pkg/signals"
	"knative.dev/pkg/system"
	"knative.dev/pkg/version"
	"knative.dev/pkg/webhook"
)

// GetConfig returns a rest.Config to be used for kubernetes client creation.
// It does so in the following order:
//   1. Use the passed kubeconfig/masterURL.
//   2. Fallback to the KUBECONFIG environment variable.
//   3. Fallback to in-cluster config.
//   4. Fallback to the ~/.kube/config.
func GetConfig(masterURL, kubeconfig string) (*rest.Config, error) {
	if kubeconfig == "" {
		kubeconfig = os.Getenv("KUBECONFIG")
	}
	// If we have an explicit indication of where the kubernetes config lives, read that.
	if kubeconfig != "" {
		return clientcmd.BuildConfigFromFlags(masterURL, kubeconfig)
	}
	// If not, try the in-cluster config.
	if c, err := rest.InClusterConfig(); err == nil {
		return c, nil
	}
	// If no in-cluster config, try the default location in the user's home directory.
	if usr, err := user.Current(); err == nil {
		if c, err := clientcmd.BuildConfigFromFlags("", filepath.Join(usr.HomeDir, ".kube", "config")); err == nil {
			return c, nil
		}
	}

	return nil, fmt.Errorf("could not create a valid kubeconfig")
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
		loggingConfigMap, err = kubeclient.Get(ctx).CoreV1().ConfigMaps(system.Namespace()).Get(logging.ConfigMapName(), metav1.GetOptions{})
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
func GetLeaderElectionConfig(ctx context.Context) (*kle.Config, error) {
	leaderElectionConfigMap, err := kubeclient.Get(ctx).CoreV1().ConfigMaps(system.Namespace()).Get(kle.ConfigMapName(), metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return kle.NewConfigFromConfigMap(nil)
	} else if err != nil {
		return nil, err
	}
	return kle.NewConfigFromConfigMap(leaderElectionConfigMap)
}

// Main runs the generic main flow for non-webhook controllers with a new
// context. Use WebhookMainWith* if you need to serve webhooks.
func Main(component string, ctors ...injection.ControllerConstructor) {
	// Set up signals so we handle the first shutdown signal gracefully.
	MainWithContext(signals.NewContext(), component, ctors...)
}

// MainWithContext runs the generic main flow for non-webhook controllers. Use
// WebhookMainWithContext if you need to serve webhooks.
func MainWithContext(ctx context.Context, component string, ctors ...injection.ControllerConstructor) {
	MainWithConfig(ctx, component, ParseAndGetConfigOrDie(), ctors...)
}

// MainWithConfig runs the generic main flow for non-webhook controllers. Use
// WebhookMainWithConfig if you need to serve webhooks.
func MainWithConfig(ctx context.Context, component string, cfg *rest.Config, ctors ...injection.ControllerConstructor) {
	log.Printf("Registering %d clients", len(injection.Default.GetClients()))
	log.Printf("Registering %d informer factories", len(injection.Default.GetInformerFactories()))
	log.Printf("Registering %d informers", len(injection.Default.GetInformers()))
	log.Printf("Registering %d controllers", len(ctors))

	MemStatsOrDie(ctx)

	// Adjust our client's rate limits based on the number of controllers we are running.
	cfg.QPS = float32(len(ctors)) * rest.DefaultQPS
	cfg.Burst = len(ctors) * rest.DefaultBurst
	ctx = injection.WithConfig(ctx, cfg)

	ctx, informers := injection.Default.SetupInformers(ctx, cfg)

	logger, atomicLevel := SetupLoggerOrDie(ctx, component)
	defer flush(logger)
	ctx = logging.WithLogger(ctx, logger)
	profilingHandler := profiling.NewHandler(logger, false)
	profilingServer := profiling.NewServer(profilingHandler)
	eg, egCtx := errgroup.WithContext(ctx)
	eg.Go(profilingServer.ListenAndServe)
	go func() {
		// This will block until either a signal arrives or one of the grouped functions
		// returns an error.
		<-egCtx.Done()

		profilingServer.Shutdown(context.Background())
		if err := eg.Wait(); err != nil && err != http.ErrServerClosed {
			logger.Errorw("Error while running server", zap.Error(err))
		}
	}()
	CheckK8sClientMinimumVersionOrDie(ctx, logger)

	run := func(ctx context.Context) {
		cmw := SetupConfigMapWatchOrDie(ctx, logger)
		controllers, _ := ControllersAndWebhooksFromCtors(ctx, cmw, ctors...)
		WatchLoggingConfigOrDie(ctx, cmw, logger, atomicLevel, component)
		WatchObservabilityConfigOrDie(ctx, cmw, profilingHandler, logger, component)

		logger.Info("Starting configuration manager...")
		if err := cmw.Start(ctx.Done()); err != nil {
			logger.Fatalw("Failed to start configuration manager", zap.Error(err))
		}
		logger.Info("Starting informers...")
		if err := controller.StartInformers(ctx.Done(), informers...); err != nil {
			logger.Fatalw("Failed to start informers", zap.Error(err))
		}
		logger.Info("Starting controllers...")
		go controller.StartAll(ctx, controllers...)

		<-ctx.Done()
	}

	// Set up leader election config
	leaderElectionConfig, err := GetLeaderElectionConfig(ctx)
	if err != nil {
		logger.Fatalw("Error loading leader election configuration", zap.Error(err))
	}
	leConfig := leaderElectionConfig.GetComponentConfig(component)

	if !leConfig.LeaderElect {
		logger.Infof("%v will not run in leader-elected mode", component)
		run(ctx)
	} else {
		RunLeaderElected(ctx, logger, run, leConfig)
	}
}

// WebhookMainWithContext runs the generic main flow for controllers and
// webhooks. Use MainWithContext if you do not need to serve webhooks.
func WebhookMainWithContext(ctx context.Context, component string, ctors ...injection.ControllerConstructor) {
	WebhookMainWithConfig(ctx, component, ParseAndGetConfigOrDie(), ctors...)
}

// WebhookMainWithConfig runs the generic main flow for controllers and webhooks
// with the given config. Use MainWithConfig if you do not need to serve
// webhooks.
func WebhookMainWithConfig(ctx context.Context, component string, cfg *rest.Config, ctors ...injection.ControllerConstructor) {
	log.Printf("Registering %d clients", len(injection.Default.GetClients()))
	log.Printf("Registering %d informer factories", len(injection.Default.GetInformerFactories()))
	log.Printf("Registering %d informers", len(injection.Default.GetInformers()))
	log.Printf("Registering %d controllers", len(ctors))

	MemStatsOrDie(ctx)

	// Adjust our client's rate limits based on the number of controllers we are running.
	cfg.QPS = float32(len(ctors)) * rest.DefaultQPS
	cfg.Burst = len(ctors) * rest.DefaultBurst
	ctx, informers := injection.Default.SetupInformers(ctx, cfg)

	logger, atomicLevel := SetupLoggerOrDie(ctx, component)
	defer flush(logger)
	ctx = logging.WithLogger(ctx, logger)
	profilingHandler := profiling.NewHandler(logger, false)
	profilingServer := profiling.NewServer(profilingHandler)

	CheckK8sClientMinimumVersionOrDie(ctx, logger)
	cmw := SetupConfigMapWatchOrDie(ctx, logger)
	controllers, webhooks := ControllersAndWebhooksFromCtors(ctx, cmw, ctors...)
	WatchLoggingConfigOrDie(ctx, cmw, logger, atomicLevel, component)
	WatchObservabilityConfigOrDie(ctx, cmw, profilingHandler, logger, component)

	eg, egCtx := errgroup.WithContext(ctx)
	eg.Go(profilingServer.ListenAndServe)

	// If we have one or more admission controllers, then start the webhook
	// and pass them in.
	var wh *webhook.Webhook
	var err error
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

	logger.Info("Starting configuration manager...")
	if err := cmw.Start(ctx.Done()); err != nil {
		logger.Fatalw("Failed to start configuration manager", zap.Error(err))
	}
	logger.Info("Starting informers...")
	if err := controller.StartInformers(ctx.Done(), informers...); err != nil {
		logger.Fatalw("Failed to start informers", zap.Error(err))
	}
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
		masterURL = flag.String("master", "",
			"The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
		kubeconfig = flag.String("kubeconfig", "",
			"Path to a kubeconfig. Only required if out-of-cluster.")
	)
	flag.Parse()

	cfg, err := GetConfig(*masterURL, *kubeconfig)
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
	return logging.NewLoggerFromConfig(loggingConfig, component)
}

// CheckK8sClientMinimumVersionOrDie checks that the hosting Kubernetes cluster
// is at least the minimum allowable version or dies by calling log.Fatalf.
func CheckK8sClientMinimumVersionOrDie(ctx context.Context, logger *zap.SugaredLogger) {
	kc := kubeclient.Get(ctx)
	if err := version.CheckMinimumVersion(kc.Discovery()); err != nil {
		logger.Fatalw("Version check failed", zap.Error(err))
	}
}

// SetupConfigMapWatchOrDie establishes a watch of the configmaps in the system
// namespace that are labeled to be watched or dies by calling log.Fatalf.
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
// calling log.Fatalf. Note, if the config does not exist, it will be defaulted
// and this method will not die.
func WatchLoggingConfigOrDie(ctx context.Context, cmw *configmap.InformedWatcher, logger *zap.SugaredLogger, atomicLevel zap.AtomicLevel, component string) {
	if _, err := kubeclient.Get(ctx).CoreV1().ConfigMaps(system.Namespace()).Get(logging.ConfigMapName(),
		metav1.GetOptions{}); err == nil {
		cmw.Watch(logging.ConfigMapName(), logging.UpdateLevelFromConfigMap(logger, atomicLevel, component))
	} else if !apierrors.IsNotFound(err) {
		logger.Fatalw("Error reading ConfigMap "+logging.ConfigMapName(), zap.Error(err))
	}
}

// WatchObservabilityConfigOrDie establishes a watch of the logging config or
// dies by calling log.Fatalf. Note, if the config does not exist, it will be
// defaulted and this method will not die.
func WatchObservabilityConfigOrDie(ctx context.Context, cmw *configmap.InformedWatcher, profilingHandler *profiling.Handler, logger *zap.SugaredLogger, component string) {
	if _, err := kubeclient.Get(ctx).CoreV1().ConfigMaps(system.Namespace()).Get(metrics.ConfigMapName(),
		metav1.GetOptions{}); err == nil {
		cmw.Watch(metrics.ConfigMapName(),
			metrics.ConfigMapWatcher(component, SecretFetcher(ctx), logger),
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
		return kubeclient.Get(ctx).CoreV1().Secrets(system.Namespace()).Get(name, metav1.GetOptions{})
	}
}

// ControllersAndWebhooksFromCtors returns a list of the controllers and a list
// of the webhooks created from the given constructors.
func ControllersAndWebhooksFromCtors(ctx context.Context,
	cmw *configmap.InformedWatcher,
	ctors ...injection.ControllerConstructor) ([]*controller.Impl, []interface{}) {
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
	}

	return controllers, webhooks
}

// RunLeaderElected runs the given function in leader elected mode. The function
// will be run only once the leader election lock is obtained.
func RunLeaderElected(ctx context.Context, logger *zap.SugaredLogger, run func(context.Context), leConfig kle.ComponentConfig) {
	recorder := controller.GetEventRecorder(ctx)
	if recorder == nil {
		// Create event broadcaster
		logger.Debug("Creating event broadcaster")
		eventBroadcaster := record.NewBroadcaster()
		watches := []watch.Interface{
			eventBroadcaster.StartLogging(logger.Named("event-broadcaster").Infof),
			eventBroadcaster.StartRecordingToSink(
				&typedcorev1.EventSinkImpl{Interface: kubeclient.Get(ctx).CoreV1().Events(system.Namespace())}),
		}
		recorder = eventBroadcaster.NewRecorder(
			scheme.Scheme, corev1.EventSource{Component: leConfig.Component})
		go func() {
			<-ctx.Done()
			for _, w := range watches {
				w.Stop()
			}
		}()
	}

	// Create a unique identifier so that two controllers on the same host don't
	// race.
	id, err := kle.UniqueID()
	if err != nil {
		logger.Fatalw("Failed to get unique ID for leader election", zap.Error(err))
	}
	logger.Infof("%v will run in leader-elected mode with id %v", leConfig.Component, id)

	// rl is the resource used to hold the leader election lock.
	rl, err := resourcelock.New(leConfig.ResourceLock,
		system.Namespace(), // use namespace we are running in
		leConfig.Component, // component is used as the resource name
		kubeclient.Get(ctx).CoreV1(),
		kubeclient.Get(ctx).CoordinationV1(),
		resourcelock.ResourceLockConfig{
			Identity:      id,
			EventRecorder: recorder,
		})
	if err != nil {
		logger.Fatalw("Error creating lock", zap.Error(err))
	}

	// Execute the `run` function when we have the lock.
	leaderelection.RunOrDie(ctx, leaderelection.LeaderElectionConfig{
		Lock:          rl,
		LeaseDuration: leConfig.LeaseDuration,
		RenewDeadline: leConfig.RenewDeadline,
		RetryPeriod:   leConfig.RetryPeriod,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: run,
			OnStoppedLeading: func() {
				logger.Fatal("Leader election lost")
			},
		},
		ReleaseOnCancel: true,
		// TODO: use health check watchdog, knative/pkg#1048
		Name: leConfig.Component,
	})
}
