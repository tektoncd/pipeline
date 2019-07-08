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
	"flag"
	"log"
	"time"

	"github.com/knative/pkg/logging"

	tklogging "github.com/tektoncd/pipeline/pkg/logging"
	corev1 "k8s.io/api/core/v1"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"

	sharedclientset "github.com/knative/pkg/client/clientset/versioned"
	"github.com/knative/pkg/controller"
	"github.com/tektoncd/pipeline/pkg/reconciler"
	"github.com/tektoncd/pipeline/pkg/reconciler/v1alpha1/pipelinerun"
	"github.com/tektoncd/pipeline/pkg/reconciler/v1alpha1/taskrun"
	"github.com/tektoncd/pipeline/pkg/system"

	"github.com/knative/pkg/configmap"
	"github.com/knative/pkg/signals"
	clientset "github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	pipelineinformers "github.com/tektoncd/pipeline/pkg/client/informers/externalversions"
)

const (
	threadsPerController = 2
	resyncPeriod         = 10 * time.Hour
	// ControllerLogKey is the name of the logger for the controller cmd
	ControllerLogKey = "controller"
)

var (
	masterURL  string
	kubeconfig string
	namespace  string
)

func main() {
	flag.Parse()
	loggingConfigMap, err := configmap.Load("/etc/config-logging")
	if err != nil {
		log.Fatalf("Error loading logging configuration: %v", err)
	}
	loggingConfig, err := logging.NewConfigFromMap(loggingConfigMap)
	if err != nil {
		log.Fatalf("Error parsing logging configuration: %v", err)
	}
	logger, atomicLevel := logging.NewLoggerFromConfig(loggingConfig, ControllerLogKey)
	defer logger.Sync()

	logger.Info("Starting the Pipeline Controller")

	// set up signals so we handle the first shutdown signal gracefully
	stopCh := signals.SetupSignalHandler()

	cfg, err := clientcmd.BuildConfigFromFlags(masterURL, kubeconfig)
	if err != nil {
		logger.Fatalf("Error building kubeconfig: %v", err)
	}

	kubeClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		logger.Fatalf("Error building kubernetes clientset: %v", err)
	}

	sharedClient, err := sharedclientset.NewForConfig(cfg)
	if err != nil {
		logger.Fatalf("Error building shared clientset: %v", err)
	}

	pipelineClient, err := clientset.NewForConfig(cfg)
	if err != nil {
		logger.Fatalf("Error building pipeline clientset: %v", err)
	}

	configMapWatcher := configmap.NewInformedWatcher(kubeClient, system.GetNamespace())

	opt := reconciler.Options{
		KubeClientSet:     kubeClient,
		SharedClientSet:   sharedClient,
		PipelineClientSet: pipelineClient,
		ConfigMapWatcher:  configMapWatcher,
		ResyncPeriod:      resyncPeriod,
		Logger:            logger,
	}

	kubeInformerFactory := kubeinformers.NewSharedInformerFactoryWithOptions(kubeClient, opt.ResyncPeriod, kubeinformers.WithNamespace(namespace))
	pipelineInformerFactory := pipelineinformers.NewSharedInformerFactoryWithOptions(pipelineClient, opt.ResyncPeriod, pipelineinformers.WithNamespace(namespace))

	taskInformer := pipelineInformerFactory.Tekton().V1alpha1().Tasks()
	clusterTaskInformer := pipelineInformerFactory.Tekton().V1alpha1().ClusterTasks()
	taskRunInformer := pipelineInformerFactory.Tekton().V1alpha1().TaskRuns()
	resourceInformer := pipelineInformerFactory.Tekton().V1alpha1().PipelineResources()
	podInformer := kubeInformerFactory.Core().V1().Pods()

	pipelineInformer := pipelineInformerFactory.Tekton().V1alpha1().Pipelines()
	pipelineRunInformer := pipelineInformerFactory.Tekton().V1alpha1().PipelineRuns()

	timeoutHandler := reconciler.NewTimeoutHandler(stopCh, logger)

	trc := taskrun.NewController(opt,
		taskRunInformer,
		taskInformer,
		clusterTaskInformer,
		resourceInformer,
		podInformer,
		nil, //entrypoint cache will be initialized by controller if not provided
		timeoutHandler,
	)
	prc := pipelinerun.NewController(opt,
		pipelineRunInformer,
		pipelineInformer,
		taskInformer,
		clusterTaskInformer,
		taskRunInformer,
		resourceInformer,
		timeoutHandler,
	)
	// Build all of our controllers, with the clients constructed above.
	controllers := []*controller.Impl{
		// Pipeline Controllers
		trc,
		prc,
	}
	timeoutHandler.SetTaskRunCallbackFunc(trc.Enqueue)
	timeoutHandler.SetPipelineRunCallbackFunc(prc.Enqueue)
	timeoutHandler.CheckTimeouts(kubeClient, pipelineClient)

	// Watch the logging config map and dynamically update logging levels.
	configMapWatcher.Watch(tklogging.ConfigName, logging.UpdateLevelFromConfigMap(logger, atomicLevel, ControllerLogKey))

	kubeInformerFactory.Start(stopCh)
	pipelineInformerFactory.Start(stopCh)
	if err := configMapWatcher.Start(stopCh); err != nil {
		logger.Fatalf("failed to start configuration manager: %v", err)
	}

	// Wait for the caches to be synced before starting controllers.
	logger.Info("Waiting for informer caches to sync")
	for i, synced := range []cache.InformerSynced{
		taskInformer.Informer().HasSynced,
		clusterTaskInformer.Informer().HasSynced,
		taskRunInformer.Informer().HasSynced,
		resourceInformer.Informer().HasSynced,
		podInformer.Informer().HasSynced,
	} {
		if ok := cache.WaitForCacheSync(stopCh, synced); !ok {
			logger.Fatalf("failed to wait for cache at index %v to sync", i)
		}
	}

	logger.Info("Starting controllers")
	// Start all of the controllers.
	for _, ctrlr := range controllers {
		go func(ctrlr *controller.Impl) {
			// We don't expect this to return until stop is called,
			// but if it does, propagate it back.
			if runErr := ctrlr.Run(threadsPerController, stopCh); runErr != nil {
				logger.Fatalf("Error running controller: %v", runErr)
			}
		}(ctrlr)
	}

	<-stopCh
}

func init() {
	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&masterURL, "master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&namespace, "namespace", corev1.NamespaceAll, "Namespace to restrict informer to. Optional, defaults to all namespaces.")
}
