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
	"flag"
	"fmt"
	"time"

	pipelinev1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/apis/resolution/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/apis/resolution/v1beta1"
	"github.com/tektoncd/pipeline/pkg/remoteresolution/resolver/framework"
	"github.com/tektoncd/pipeline/pkg/resolution/common"
	frameworkV1 "github.com/tektoncd/pipeline/pkg/resolution/resolver/framework"
	"k8s.io/client-go/rest"
	filteredinformerfactory "knative.dev/pkg/client/injection/kube/informers/factory/filtered"
	"knative.dev/pkg/injection"
	"knative.dev/pkg/injection/sharedmain"
	"knative.dev/pkg/signals"
)

func main() {
	timeout := flag.Int("delay", 300, "timeout in seconds before resolver returns")
	flag.Parse()
	ctx := filteredinformerfactory.WithSelectors(signals.NewContext(), v1alpha1.ManagedByLabelKey)

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

	r := &resolver{
		timeout: time.Duration(*timeout) * time.Second,
	}
	sharedmain.MainWithConfig(ctx, "controller", cfg,
		framework.NewController(ctx, r),
	)
}

type resolver struct{ timeout time.Duration }

// Initialize sets up any dependencies needed by the resolver. None atm.
func (r *resolver) Initialize(context.Context) error {
	return nil
}

// GetName returns a string name to refer to this resolver by.
func (r *resolver) GetName(context.Context) string {
	return "resolver-with-timeout"
}

// GetSelector returns a map of labels to match requests to this resolver.
func (r *resolver) GetSelector(context.Context) map[string]string {
	return map[string]string{
		common.LabelKeyResolverType: "resolver-with-timeout",
	}
}

// Validate ensures resolution spec from a request is as expected.
func (r *resolver) Validate(ctx context.Context, req *v1beta1.ResolutionRequestSpec) error {
	if len(req.Params) > 0 {
		return errors.New("no params allowed")
	}
	return nil
}

func (r *resolver) Resolve(ctx context.Context, req *v1beta1.ResolutionRequestSpec) (frameworkV1.ResolvedResource, error) {
	fmt.Printf("Resolver will wait for %v before responding\n", r.timeout)

	ctxWithTimeout, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	select {
	case <-ctxWithTimeout.Done():
		return nil, ctxWithTimeout.Err()
	case <-time.After(r.timeout):
		return &myResolvedResource{}, nil
	}
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
