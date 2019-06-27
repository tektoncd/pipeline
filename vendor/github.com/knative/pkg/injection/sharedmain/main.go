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
	"log"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/knative/pkg/configmap"
	"github.com/knative/pkg/controller"
	"github.com/knative/pkg/injection"
	"github.com/knative/pkg/injection/clients/kubeclient"
	"github.com/knative/pkg/logging"
	"github.com/knative/pkg/metrics"
	"github.com/knative/pkg/signals"
	"github.com/knative/pkg/system"
	"go.uber.org/zap"
)

func Main(component string, ctors ...injection.ControllerConstructor) {
	// Set up signals so we handle the first shutdown signal gracefully.
	MainWithContext(signals.NewContext(), component, ctors...)
}

func MainWithContext(ctx context.Context, component string, ctors ...injection.ControllerConstructor) {
	var (
		masterURL  = flag.String("master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
		kubeconfig = flag.String("kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	)
	flag.Parse()

	cfg, err := clientcmd.BuildConfigFromFlags(*masterURL, *kubeconfig)
	if err != nil {
		log.Fatal("Error building kubeconfig", err)
	}
	MainWithConfig(ctx, component, cfg, ctors...)
}

func MainWithConfig(ctx context.Context, component string, cfg *rest.Config, ctors ...injection.ControllerConstructor) {
	// Set up our logger.
	loggingConfigMap, err := configmap.Load("/etc/config-logging")
	if err != nil {
		log.Fatal("Error loading logging configuration:", err)
	}
	loggingConfig, err := logging.NewConfigFromMap(loggingConfigMap)
	if err != nil {
		log.Fatal("Error parsing logging configuration:", err)
	}
	logger, atomicLevel := logging.NewLoggerFromConfig(loggingConfig, component)
	defer flush(logger)
	ctx = logging.WithLogger(ctx, logger)

	logger.Infof("Registering %d clients", len(injection.Default.GetClients()))
	logger.Infof("Registering %d informer factories", len(injection.Default.GetInformerFactories()))
	logger.Infof("Registering %d informers", len(injection.Default.GetInformers()))
	logger.Infof("Registering %d controllers", len(ctors))

	// Adjust our client's rate limits based on the number of controller's we are running.
	cfg.QPS = float32(len(ctors)) * rest.DefaultQPS
	cfg.Burst = len(ctors) * rest.DefaultBurst

	ctx, informers := injection.Default.SetupInformers(ctx, cfg)

	// TODO(mattmoor): This should itself take a context and be injection-based.
	cmw := configmap.NewInformedWatcher(kubeclient.Get(ctx), system.Namespace())

	// Based on the reconcilers we have linked, build up the set of controllers to run.
	controllers := make([]*controller.Impl, 0, len(ctors))
	for _, cf := range ctors {
		controllers = append(controllers, cf(ctx, cmw))
	}

	// Watch the logging config map and dynamically update logging levels.
	cmw.Watch(logging.ConfigMapName(), logging.UpdateLevelFromConfigMap(logger, atomicLevel, component))
	// Watch the observability config map and dynamically update metrics exporter.
	cmw.Watch(metrics.ConfigMapName(), metrics.UpdateExporterFromConfigMap(component, logger))
	if err := cmw.Start(ctx.Done()); err != nil {
		logger.Fatalw("failed to start configuration manager", zap.Error(err))
	}

	// Start all of the informers and wait for them to sync.
	logger.Info("Starting informers.")
	if err := controller.StartInformers(ctx.Done(), informers...); err != nil {
		logger.Fatalw("Failed to start informers", err)
	}

	// Start all of the controllers.
	logger.Info("Starting controllers...")
	controller.StartAll(ctx.Done(), controllers...)
}

func flush(logger *zap.SugaredLogger) {
	logger.Sync()
	metrics.FlushExporter()
}
