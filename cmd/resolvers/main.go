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
	"os"
	"strings"

	"github.com/tektoncd/pipeline/pkg/apis/resolution/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/resolution/resolver/bundle"
	"github.com/tektoncd/pipeline/pkg/resolution/resolver/cluster"
	"github.com/tektoncd/pipeline/pkg/resolution/resolver/framework"
	"github.com/tektoncd/pipeline/pkg/resolution/resolver/git"
	"github.com/tektoncd/pipeline/pkg/resolution/resolver/hub"
	filteredinformerfactory "knative.dev/pkg/client/injection/kube/informers/factory/filtered"
	"knative.dev/pkg/injection/sharedmain"
	"knative.dev/pkg/signals"
)

func main() {
	ctx := filteredinformerfactory.WithSelectors(signals.NewContext(), v1alpha1.ManagedByLabelKey)
	tektonHubURL := buildHubURL(os.Getenv("TEKTON_HUB_API"), "", hub.TektonHubYamlEndpoint)
	artifactHubURL := buildHubURL(os.Getenv("ARTIFACT_HUB_API"), hub.DefaultArtifactHubURL, hub.ArtifactHubYamlEndpoint)

	sharedmain.MainWithContext(ctx, "controller",
		framework.NewController(ctx, &git.Resolver{}),
		framework.NewController(ctx, &hub.Resolver{TektonHubURL: tektonHubURL, ArtifactHubURL: artifactHubURL}),
		framework.NewController(ctx, &bundle.Resolver{}),
		framework.NewController(ctx, &cluster.Resolver{}))
}

func buildHubURL(configAPI, defaultURL, yamlEndpoint string) string {
	var hubURL string
	if configAPI == "" {
		hubURL = defaultURL
	} else {
		if !strings.HasSuffix(configAPI, "/") {
			configAPI += "/"
		}
		hubURL = configAPI + yamlEndpoint
	}

	return hubURL
}
