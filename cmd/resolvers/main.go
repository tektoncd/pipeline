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
	"fmt"
	"os"
	"strings"

	"github.com/tektoncd/pipeline/pkg/apis/resolution/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/resolution/resolver/bundle"
	"github.com/tektoncd/pipeline/pkg/resolution/resolver/framework"
	"github.com/tektoncd/pipeline/pkg/resolution/resolver/git"
	"github.com/tektoncd/pipeline/pkg/resolution/resolver/hub"
	filteredinformerfactory "knative.dev/pkg/client/injection/kube/informers/factory/filtered"
	"knative.dev/pkg/injection/sharedmain"
	"knative.dev/pkg/signals"
)

func main() {
	ctx := filteredinformerfactory.WithSelectors(signals.NewContext(), v1alpha1.ManagedByLabelKey)

	apiURL := os.Getenv("HUB_API")
	hubURL := hub.DefaultHubURL
	if apiURL == "" {
		hubURL = hub.DefaultHubURL
	} else {
		if !strings.HasSuffix(apiURL, "/") {
			apiURL += "/"
		}
		hubURL = apiURL + hub.YamlEndpoint
	}
	fmt.Println("RUNNING WITH HUB URL PATTERN:", hubURL)

	sharedmain.MainWithContext(ctx, "controller",
		framework.NewController(ctx, &git.Resolver{}),
		framework.NewController(ctx, &hub.Resolver{HubURL: hubURL}),
		framework.NewController(ctx, &bundle.Resolver{}))
}
