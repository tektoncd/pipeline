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

	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	"github.com/tektoncd/pipeline/pkg/reconciler/pipelinerun"
	"github.com/tektoncd/pipeline/pkg/reconciler/taskrun"
	"github.com/tektoncd/pipeline/pkg/version"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/rest"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/injection"
	"knative.dev/pkg/injection/sharedmain"
	"knative.dev/pkg/signals"
)

const (
	// ControllerLogKey is the name of the logger for the controller cmd
	ControllerLogKey = "tekton"
)

var (
	entrypointImage          = flag.String("entrypoint-image", "", "The container image containing our entrypoint binary.")
	nopImage                 = flag.String("nop-image", "", "The container image used to stop sidecars")
	gitImage                 = flag.String("git-image", "", "The container image containing our Git binary.")
	credsImage               = flag.String("creds-image", "", "The container image for preparing our Build's credentials.")
	kubeconfigWriterImage    = flag.String("kubeconfig-writer-image", "", "The container image containing our kubeconfig writer binary.")
	shellImage               = flag.String("shell-image", "", "The container image containing a shell")
	gsutilImage              = flag.String("gsutil-image", "", "The container image containing gsutil")
	buildGCSFetcherImage     = flag.String("build-gcs-fetcher-image", "", "The container image containing our GCS fetcher binary.")
	prImage                  = flag.String("pr-image", "", "The container image containing our PR binary.")
	imageDigestExporterImage = flag.String("imagedigest-exporter-image", "", "The container image containing our image digest exporter binary.")
	namespace                = flag.String("namespace", corev1.NamespaceAll, "Namespace to restrict informer to. Optional, defaults to all namespaces.")
	versionGiven             = flag.String("version", "devel", "Version of Tekton running")
	qps                      = flag.Int("kube-api-qps", int(rest.DefaultQPS), "Maximum QPS to the master from this client")
	burst                    = flag.Int("kube-api-burst", rest.DefaultBurst, "Maximum burst for throttle")
	threadsPerController     = flag.Int("threads-per-controller", controller.DefaultThreadsPerController, "Threads (goroutines) to create per controller")
	disableHighAvailability  = flag.Bool("disable-ha", false, "Whether to disable high-availability functionality for this component.  This flag will be deprecated "+
		"and removed when we have promoted this feature to stable, so do not pass it without filing an "+
		"issue upstream!")
)

func main() {
	flag.Parse()
	version.SetVersion(*versionGiven)
	images := pipeline.Images{
		EntrypointImage:          *entrypointImage,
		NopImage:                 *nopImage,
		GitImage:                 *gitImage,
		CredsImage:               *credsImage,
		KubeconfigWriterImage:    *kubeconfigWriterImage,
		ShellImage:               *shellImage,
		GsutilImage:              *gsutilImage,
		BuildGCSFetcherImage:     *buildGCSFetcherImage,
		PRImage:                  *prImage,
		ImageDigestExporterImage: *imageDigestExporterImage,
	}
	if err := images.Validate(); err != nil {
		log.Fatal(err)
	}
	controller.DefaultThreadsPerController = *threadsPerController

	cfg := sharedmain.ParseAndGetConfigOrDie()
	// multiply by 2, no of controllers being created
	cfg.QPS = 2 * float32(*qps)
	cfg.Burst = 2 * *burst

	ctx := injection.WithNamespaceScope(signals.NewContext(), *namespace)
	if *disableHighAvailability {
		ctx = sharedmain.WithHADisabled(ctx)
	}
	sharedmain.MainWithConfig(ctx, ControllerLogKey, cfg,
		taskrun.NewController(*namespace, images),
		pipelinerun.NewController(*namespace, images),
	)
}
