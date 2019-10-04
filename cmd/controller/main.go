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

	"knative.dev/pkg/injection/sharedmain"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	"github.com/tektoncd/pipeline/pkg/reconciler/pipelinerun"
	"github.com/tektoncd/pipeline/pkg/reconciler/taskrun"
)

const (
	// ControllerLogKey is the name of the logger for the controller cmd
	ControllerLogKey = "controller"
)

var (
	entrypointImage = flag.String("entrypoint-image", "override-with-entrypoint:latest",
		"The container image containing our entrypoint binary.")
	nopImage = flag.String("nop-image", "override-with-nop:latest",
		"The container image used to kill sidecars")
	gitImage = flag.String("git-image", "override-with-git:latest",
		"The container image containing our Git binary.")
	credsImage = flag.String("creds-image", "override-with-creds:latest",
		"The container image for preparing our Build's credentials.")
	kubeconfigWriterImage = flag.String("kubeconfig-writer-image", "override-with-kubeconfig-writer:latest",
		"The container image containing our kubeconfig writer binary.")
	bashNoopImage = flag.String("bash-noop-image", "override-with-bash-noop:latest",
		"The container image containing bash shell")
	gsutilImage = flag.String("gsutil-image", "override-with-gsutil-image:latest",
		"The container image containing gsutil")
	buildGCSFetcherImage = flag.String("build-gcs-fetcher-image", "gcr.io/cloud-builders/gcs-fetcher:latest",
		"The container image containing our GCS fetcher binary.")
	prImage = flag.String("pr-image", "override-with-pr:latest",
		"The container image containing our PR binary.")
	imageDigestExporterImage = flag.String("imagedigest-exporter-image", "override-with-imagedigest-exporter-image:latest",
		"The container image containing our image digest exporter binary.")
)

func main() {
	flag.Parse()
	images := pipeline.Images{
		EntryPointImage:          *entrypointImage,
		NopImage:                 *nopImage,
		GitImage:                 *gitImage,
		CredsImage:               *credsImage,
		KubeconfigWriterImage:    *kubeconfigWriterImage,
		BashNoopImage:            *bashNoopImage,
		GsutilImage:              *gsutilImage,
		BuildGCSFetcherImage:     *buildGCSFetcherImage,
		PRImage:                  *prImage,
		ImageDigestExporterImage: *imageDigestExporterImage,
	}
	sharedmain.Main(ControllerLogKey,
		taskrun.NewController(images),
		pipelinerun.NewController(images),
	)
}
