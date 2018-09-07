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
	"net/http"
	"time"

	cloudbuild "google.golang.org/api/cloudbuild/v1"

	"github.com/golang/glog"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	// Uncomment the following line to load the gcp plugin (only required to authenticate against GKE clusters).
	// _ "k8s.io/client-go/plugin/pkg/client/auth/gcp"

	"cloud.google.com/go/compute/metadata"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	"github.com/knative/build/pkg/builder"
	onclusterbuilder "github.com/knative/build/pkg/builder/cluster"
	gcb "github.com/knative/build/pkg/builder/google"
	"github.com/knative/build/pkg/controller"
	"github.com/knative/build/pkg/controller/build"
	"github.com/knative/build/pkg/controller/buildtemplate"

	buildclientset "github.com/knative/build/pkg/client/clientset/versioned"
	informers "github.com/knative/build/pkg/client/informers/externalversions"
	"github.com/knative/build/pkg/signals"
)

const (
	threadsPerController = 2
)

var (
	kubeconfig  = flag.String("kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	masterURL   = flag.String("master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
	builderName = flag.String("builder", "", "The builder implementation to use to execute builds (supports: cluster, google).")
)

func newCloudBuilder() builder.Interface {
	client := &http.Client{
		Transport: &oauth2.Transport{
			// If no account is specified, "default" is used.
			Source: google.ComputeTokenSource(""),
		},
	}
	cb, err := cloudbuild.New(client)
	if err != nil {
		glog.Fatalf("Unable to initialize cloudbuild client: %v", err)
	}
	project, err := metadata.ProjectID()
	if err != nil {
		glog.Fatalf("Unable to determine project-id, are you running on GCE? error: %v", err)
	}
	return gcb.NewBuilder(cb, project)
}

func newOnClusterBuilder(kubeclientset kubernetes.Interface, kubeinformers kubeinformers.SharedInformerFactory) builder.Interface {
	return onclusterbuilder.NewBuilder(kubeclientset, kubeinformers)
}

func main() {
	flag.Parse()

	// set up signals so we handle the first shutdown signal gracefully
	stopCh := signals.SetupSignalHandler()

	cfg, err := clientcmd.BuildConfigFromFlags(*masterURL, *kubeconfig)
	if err != nil {
		glog.Fatalf("Error building kubeconfig: %s", err.Error())
	}

	kubeClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		glog.Fatalf("Error building kubernetes clientset: %s", err.Error())
	}

	buildClient, err := buildclientset.NewForConfig(cfg)
	if err != nil {
		glog.Fatalf("Error building Build clientset: %s", err.Error())
	}

	kubeInformerFactory := kubeinformers.NewSharedInformerFactory(kubeClient, time.Second*30)
	buildInformerFactory := informers.NewSharedInformerFactory(buildClient, time.Second*30)

	// Add new controllers here.
	ctors := []controller.Constructor{
		build.NewController,
		buildtemplate.NewController,
	}

	var bldr builder.Interface
	switch *builderName {
	case "cluster":
		bldr = newOnClusterBuilder(kubeClient, kubeInformerFactory)
	case "google":
		bldr = newCloudBuilder()
	default:
		glog.Fatalf("Unrecognized builder: %v (supported: google, cluster)", builderName)
	}

	// Build all of our controllers, with the clients constructed above.
	controllers := make([]controller.Interface, 0, len(ctors))
	for _, ctor := range ctors {
		controllers = append(controllers,
			ctor(bldr, kubeClient, buildClient, kubeInformerFactory, buildInformerFactory))
	}

	go kubeInformerFactory.Start(stopCh)
	go buildInformerFactory.Start(stopCh)

	// Start all of the controllers.
	for _, ctrlr := range controllers {
		go func(ctrlr controller.Interface) {
			// We don't expect this to return until stop is called,
			// but if it does, propagate it back.
			if err := ctrlr.Run(threadsPerController, stopCh); err != nil {
				glog.Fatalf("Error running controller: %s", err.Error())
			}
		}(ctrlr)
	}

	// TODO(mattmoor): Use a sync.WaitGroup instead?
	<-stopCh
	glog.Flush()
}
