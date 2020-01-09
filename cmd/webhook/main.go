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
	"os"

	apiconfig "github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/contexts"
	tklogging "github.com/tektoncd/pipeline/pkg/logging"
	"github.com/tektoncd/pipeline/pkg/system"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/logging/logkey"
	"knative.dev/pkg/signals"
	"knative.dev/pkg/webhook"
)

// WebhookLogKey is the name of the logger for the webhook cmd
const WebhookLogKey = "webhook"

func main() {
	flag.Parse()
	cm, err := configmap.Load("/etc/config-logging")
	if err != nil {
		log.Fatalf("Error loading logging configuration: %v", err)
	}
	config, err := logging.NewConfigFromMap(cm)
	if err != nil {
		log.Fatalf("Error parsing logging configuration: %v", err)
	}
	logger, atomicLevel := logging.NewLoggerFromConfig(config, WebhookLogKey)
	defer func() {
		_ = logger.Sync()
	}()
	logger = logger.With(zap.String(logkey.ControllerType, "webhook"))

	logger.Info("Starting the Configuration Webhook")

	// set up signals so we handle the first shutdown signal gracefully
	stopCh := signals.SetupSignalHandler()

	clusterConfig, err := rest.InClusterConfig()
	if err != nil {
		logger.Fatal("Failed to get in cluster config", zap.Error(err))
	}

	kubeClient, err := kubernetes.NewForConfig(clusterConfig)
	if err != nil {
		logger.Fatal("Failed to get the client set", zap.Error(err))
	}
	// Watch the logging config map and dynamically update logging levels.
	configMapWatcher := configmap.NewInformedWatcher(kubeClient, system.GetNamespace())
	configMapWatcher.Watch(tklogging.ConfigName, logging.UpdateLevelFromConfigMap(logger, atomicLevel, WebhookLogKey))

	store := apiconfig.NewStore(logger.Named("config-store"))
	store.WatchConfigs(configMapWatcher)

	if err = configMapWatcher.Start(stopCh); err != nil {
		logger.Fatalf("failed to start configuration manager: %v", err)
	}

	serviceName := os.Getenv("WEBHOOK_SERVICE_NAME")
	if serviceName == "" {
		serviceName = "tekton-pipelines-webhook"
	}

	options := webhook.ControllerOptions{
		ServiceName:                     serviceName,
		DeploymentName:                  serviceName,
		Namespace:                       system.GetNamespace(),
		Port:                            8443,
		SecretName:                      "webhook-certs",
		WebhookName:                     "webhook.tekton.dev",
		ResourceAdmissionControllerPath: "/",
	}
	resourceHandlers := map[schema.GroupVersionKind]webhook.GenericCRD{
		v1alpha1.SchemeGroupVersion.WithKind("Pipeline"):         &v1alpha1.Pipeline{},
		v1alpha1.SchemeGroupVersion.WithKind("Task"):             &v1alpha1.Task{},
		v1alpha1.SchemeGroupVersion.WithKind("ClusterTask"):      &v1alpha1.ClusterTask{},
		v1alpha1.SchemeGroupVersion.WithKind("TaskRun"):          &v1alpha1.TaskRun{},
		v1alpha1.SchemeGroupVersion.WithKind("PipelineRun"):      &v1alpha1.PipelineRun{},
		v1alpha1.SchemeGroupVersion.WithKind("Condition"):        &v1alpha1.Condition{},
		v1alpha1.SchemeGroupVersion.WithKind("PipelineResource"): &v1alpha1.PipelineResource{},
	}

	resourceAdmissionController := webhook.NewResourceAdmissionController(resourceHandlers, options, true)
	admissionControllers := map[string]webhook.AdmissionController{
		options.ResourceAdmissionControllerPath: resourceAdmissionController,
	}

	// Decorate contexts with the current state of the config.
	ctxFunc := func(ctx context.Context) context.Context {
		return contexts.WithDefaultConfigurationName(store.ToContext(ctx))
	}

	controller, err := webhook.New(kubeClient, options, admissionControllers, logger, ctxFunc)
	if err != nil {
		logger.Fatal("Error creating admission controller", zap.Error(err))
	}

	if err := controller.Run(stopCh); err != nil {
		logger.Fatal("Error running admission controller", zap.Error(err))
	}
}
