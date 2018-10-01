/*
Copyright 2018 The Knative Authors

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

	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/knative/build-pipeline/pkg/logging"

	"github.com/knative/build-pipeline/pkg/reconciler"
	"github.com/knative/build-pipeline/pkg/reconciler/v1alpha1/taskrun"
	"github.com/knative/build-pipeline/pkg/system"
	sharedclientset "github.com/knative/pkg/client/clientset/versioned"
	"github.com/knative/pkg/controller"

	clientset "github.com/knative/build-pipeline/pkg/client/clientset/versioned"
	pipelineinformers "github.com/knative/build-pipeline/pkg/client/informers/externalversions"
	knativebuildclientset "github.com/knative/build/pkg/client/clientset/versioned"
	knativeinformers "github.com/knative/build/pkg/client/informers/externalversions"
	"github.com/knative/pkg/configmap"
	"github.com/knative/pkg/signals"
)

const (
	threadsPerController = 2
)

var (
	masterURL  string
	kubeconfig string
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
	logger, atomicLevel := logging.NewLoggerFromConfig(loggingConfig, logging.ControllerLogKey)
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
		logger.Fatalf("Error building serving clientset: %v", err)
	}

	knativebuildClient, err := knativebuildclientset.NewForConfig(cfg)
	if err != nil {
		logger.Fatalf("Error building Build clientset: %v", err)
	}

	kubeInformerFactory := kubeinformers.NewSharedInformerFactory(kubeClient, time.Second*30)
	pipelineInformerFactory := pipelineinformers.NewSharedInformerFactory(pipelineClient, time.Second*30)
	knativebuildInformerFactory := knativeinformers.NewSharedInformerFactory(knativebuildClient, time.Second*30)

	configMapWatcher := configmap.NewInformedWatcher(kubeClient, system.Namespace)

	opt := reconciler.Options{
		KubeClientSet:     kubeClient,
		SharedClientSet:   sharedClient,
		PipelineClientSet: pipelineClient,
		Logger:            logger,
	}

	taskInformer := pipelineInformerFactory.Pipeline().V1alpha1().Tasks()
	taskRunInformer := pipelineInformerFactory.Pipeline().V1alpha1().TaskRuns()
	buildInformer := knativebuildInformerFactory.Build().V1alpha1().Builds()
	buildTemplateInformer := knativebuildInformerFactory.Build().V1alpha1().BuildTemplates()
	clusterBuildTemplateInformer := knativebuildInformerFactory.Build().V1alpha1().ClusterBuildTemplates()

	// Build all of our controllers, with the clients constructed above.
	controllers := []*controller.Impl{
		// Pipeline Controllers
		taskrun.NewController(opt,
			taskRunInformer,
			taskInformer,
			buildInformer,
			buildTemplateInformer,
			clusterBuildTemplateInformer,
		),
	}

	// Watch the logging config map and dynamically update logging levels.
	configMapWatcher.Watch(logging.ConfigName, logging.UpdateLevelFromConfigMap(logger, atomicLevel, logging.ControllerLogKey))

	kubeInformerFactory.Start(stopCh)
	pipelineInformerFactory.Start(stopCh)
	knativebuildInformerFactory.Start(stopCh)
	if err := configMapWatcher.Start(stopCh); err != nil {
		logger.Fatalf("failed to start configuration manager: %v", err)
	}

	// Wait for the caches to be synced before starting controllers.
	logger.Info("Waiting for informer caches to sync")
	for i, synced := range []cache.InformerSynced{
		taskInformer.Informer().HasSynced,
		taskRunInformer.Informer().HasSynced,
		buildInformer.Informer().HasSynced,
		buildTemplateInformer.Informer().HasSynced,
		clusterBuildTemplateInformer.Informer().HasSynced,
	} {
		if ok := cache.WaitForCacheSync(stopCh, synced); !ok {
			logger.Fatalf("failed to wait for cache at index %v to sync", i)
		}
	}

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
}
