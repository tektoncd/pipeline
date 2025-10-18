/*
Copyright 2022 The Tekton Authors

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
	"os"
	"strconv"
	"strings"

	resolverconfig "github.com/tektoncd/pipeline/pkg/apis/config/resolver"
	"github.com/tektoncd/pipeline/pkg/apis/resolution/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/remoteresolution/resolver/bundle"
	"github.com/tektoncd/pipeline/pkg/remoteresolution/resolver/cluster"
	"github.com/tektoncd/pipeline/pkg/remoteresolution/resolver/framework"
	"github.com/tektoncd/pipeline/pkg/remoteresolution/resolver/framework/cache"
	"github.com/tektoncd/pipeline/pkg/remoteresolution/resolver/git"
	"github.com/tektoncd/pipeline/pkg/remoteresolution/resolver/http"
	"github.com/tektoncd/pipeline/pkg/remoteresolution/resolver/hub"
	hubresolution "github.com/tektoncd/pipeline/pkg/resolution/resolver/hub"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	filteredinformerfactory "knative.dev/pkg/client/injection/kube/informers/factory/filtered"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/injection"
	"knative.dev/pkg/injection/sharedmain"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/signals"
	"knative.dev/pkg/system"
)

func main() {
	if val, ok := os.LookupEnv("THREADS_PER_CONTROLLER"); ok {
		threadsPerController, err := strconv.Atoi(val)
		if err != nil {
			log.Fatalf("failed to parse value %q of THREADS_PER_CONTROLLER: %v\n", val, err)
		}
		controller.DefaultThreadsPerController = threadsPerController
	}
	flag.IntVar(&controller.DefaultThreadsPerController, "threads-per-controller", controller.DefaultThreadsPerController, "Threads (goroutines) to create per controller")

	ctx := filteredinformerfactory.WithSelectors(signals.NewContext(), v1alpha1.ManagedByLabelKey)
	tektonHubURL := buildHubURL(os.Getenv("TEKTON_HUB_API"), "")
	artifactHubURL := buildHubURL(os.Getenv("ARTIFACT_HUB_API"), hubresolution.DefaultArtifactHubURL)

	// This parses flags.
	cfg := injection.ParseAndGetRESTConfigOrDie()

	if cfg.QPS == 0 {
		cfg.QPS = 2 * rest.DefaultQPS
	}
	if cfg.Burst == 0 {
		cfg.Burst = rest.DefaultBurst
	}
	// multiply by no of controllers being created
	cfg.QPS = 5 * cfg.QPS
	cfg.Burst = 5 * cfg.Burst

	// Initialize shared resolver cache from ConfigMap
	logger := logging.FromContext(ctx)
	kubeClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		log.Fatalf("failed to create kube client: %v", err)
	}

	resolverNS := resolverconfig.ResolversNamespace(system.Namespace())
	configMap, err := kubeClient.CoreV1().ConfigMaps(resolverNS).Get(ctx, "resolver-cache-config", metav1.GetOptions{})
	if err != nil {
		logger.Debugf("Could not load resolver-cache-config ConfigMap: %v. Using default cache configuration.", err)
	} else {
		cache.InitializeSharedCache(configMap)
		logger.Info("Initialized resolver cache from ConfigMap")
	}

	sharedmain.MainWithConfig(ctx, "controller", cfg,
		framework.NewController(ctx, &git.Resolver{}),
		framework.NewController(ctx, &hub.Resolver{TektonHubURL: tektonHubURL, ArtifactHubURL: artifactHubURL}),
		framework.NewController(ctx, &bundle.Resolver{}),
		framework.NewController(ctx, &cluster.Resolver{}),
		framework.NewController(ctx, &http.Resolver{}))
}

func buildHubURL(configAPI, defaultURL string) string {
	var hubURL string
	if configAPI == "" {
		hubURL = defaultURL
	} else {
		hubURL = configAPI
	}
	return strings.TrimSuffix(hubURL, "/")
}
