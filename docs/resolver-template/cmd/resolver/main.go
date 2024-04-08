/*
Copyright 2024 The Tekton Authors
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
	"errors"
	"fmt"
	neturl "net/url"

	pipelinev1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/apis/resolution/v1beta1"
	"github.com/tektoncd/pipeline/pkg/remoteresolution/resolver/framework"
	"github.com/tektoncd/pipeline/pkg/resolution/common"
	frameworkV1 "github.com/tektoncd/pipeline/pkg/resolution/resolver/framework"
	filteredinformerfactory "knative.dev/pkg/client/injection/kube/informers/factory/filtered"
	"knative.dev/pkg/injection/sharedmain"
)

func main() {
	ctx := filteredinformerfactory.WithSelectors(context.Background(), v1beta1.ManagedByLabelKey)
	sharedmain.MainWithContext(ctx, "controller",
		framework.NewController(ctx, &resolver{}),
	)
}

type resolver struct{}

// Initialize sets up any dependencies needed by the resolver. None atm.
func (r *resolver) Initialize(context.Context) error {
	return nil
}

// GetName returns a string name to refer to this resolver by.
func (r *resolver) GetName(context.Context) string {
	return "Demo"
}

// GetSelector returns a map of labels to match requests to this resolver.
func (r *resolver) GetSelector(context.Context) map[string]string {
	return map[string]string{
		common.LabelKeyResolverType: "demo",
	}
}

// Validate ensures resolution spec from a request is as expected.
func (r *resolver) Validate(ctx context.Context, req *v1beta1.ResolutionRequestSpec) error {
	if len(req.Params) > 0 {
		return errors.New("no params allowed")
	}
	u, err := neturl.ParseRequestURI(req.URL)
	if err != nil {
		return err
	}
	if u.Scheme != "demoscheme" {
		return fmt.Errorf("Invalid Scheme. Want %s, Got %s", "demoscheme", u.Scheme)
	}
	return nil
}

// Resolve uses the given resolution spec to resolve the requested file or resource.
func (r *resolver) Resolve(ctx context.Context, req *v1beta1.ResolutionRequestSpec) (frameworkV1.ResolvedResource, error) {
	return &myResolvedResource{}, nil
}

// our hard-coded resolved file to return
const pipeline = `
apiVersion: tekton.dev/v1
kind: Pipeline
metadata:
  name: my-pipeline
spec:
  tasks:
  - name: hello-world
    taskSpec:
      steps:
      - image: alpine:3.15.1
        script: |
          echo "hello world"
`

// myResolvedResource wraps the data we want to return to Pipelines
type myResolvedResource struct{}

// Data returns the bytes of our hard-coded Pipeline
func (*myResolvedResource) Data() []byte {
	return []byte(pipeline)
}

// Annotations returns any metadata needed alongside the data. None atm.
func (*myResolvedResource) Annotations() map[string]string {
	return nil
}

// RefSource is the source reference of the remote data that records where the remote
// file came from including the url, digest and the entrypoint. None atm.
func (*myResolvedResource) RefSource() *pipelinev1.RefSource {
	return nil
}
