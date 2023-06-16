<!--
---
linkTitle: "How to write a Resolver"
weight: 104
---
-->

# How to write a Resolver

This how-to will outline the steps a developer needs to take when creating
a new (very basic) Resolver. Rather than focus on support for a particular version
control system or cloud platform this Resolver will simply respond with
with some hard-coded YAML.

If you aren't yet familiar with the meaning of "resolution" when it
comes to Tekton, a short summary follows. You might also want to read a
little bit into Tekton Pipelines, particularly [the docs on specifying a
target Pipeline to
run](./pipelineruns.md#specifying-the-target-pipeline)
and, if you're feeling particularly brave or bored, the [really long
design doc describing Tekton
Resolution](https://github.com/tektoncd/community/blob/main/teps/0060-remote-resource-resolution.md).

## What's a Resolver?

A Resolver is a program that runs in a Kubernetes cluster alongside
[Tekton Pipelines](https://github.com/tektoncd/pipeline) and "resolves"
requests for `Tasks` and `Pipelines` from remote locations. More
concretely: if a user submitted a `PipelineRun` that needed a Pipeline
YAML stored in a git repo, then it would be a `Resolver` that's
responsible for fetching the YAML file from git and returning it to
Tekton Pipelines.

This pattern extends beyond just git, allowing a developer to integrate
support for other version control systems, cloud buckets, or storage systems
without having to modify Tekton Pipelines itself.

## Just want to see the working example?

If you'd prefer to look at the end result of this howto you can take a
visit the
[`./resolver-template`](./resolver-template)
in the Tekton Resolution repo. That template is built on the code from
this howto to get you up and running quickly.

## Pre-requisites

Before getting started with this howto you'll need to be comfortable
developing in Go and have a general understanding of how Tekton
Resolution works.

You'll also need the following:

- A computer with
  [`kubectl`](https://kubernetes.io/docs/tasks/tools/#kubectl) and
  [`ko`](https://github.com/google/ko) installed.
- A Kubernetes cluster running at least Kubernetes 1.24. A [`kind`
  cluster](https://kind.sigs.k8s.io/docs/user/quick-start/#installation)
  should work fine for following the guide on your local machine.
- An image registry that you can push images to. If you're using `kind`
  make sure your `KO_DOCKER_REPO` environment variable is set to
  `kind.local`.
- Tekton Pipelines and remote resolvers installed in your Kubernetes
  cluster. See [the installation
  guide](./install.md#installing-and-configuring-remote-task-and-pipeline-resolution) for
  instructions on installing it.

## First Steps

The first thing to do is create an initial directory structure for your
project. For this example we'll create a directory and initialize a new
go module with a few subdirectories for our code:

```bash
$ mkdir demoresolver

$ cd demoresolver

$ go mod init example.com/demoresolver

$ mkdir -p cmd/demoresolver

$ mkdir config
```

The `cmd/demoresolver` directory will contain code for the resolver and the
`config` directory will eventually contain a yaml file for deploying the
resolver to Kubernetes.

## Initializing the resolver's binary

A Resolver is ultimately just a program running in your cluster, so the
first step is to fill out the initial code for starting that program.
Our resolver here is going to be extremely simple and doesn't need any
flags or special environment variables, so we'll just initialize it with
a little bit of boilerplate.

Create `cmd/demoresolver/main.go` with the following setup code:

```go
package main

import (
  "context"
  "github.com/tektoncd/pipeline/pkg/resolution/resolver/framework"
  "knative.dev/pkg/injection/sharedmain"
)

func main() {
  sharedmain.Main("controller",
    framework.NewController(context.Background(), &resolver{}),
  )
}

type resolver struct {}
```

This won't compile yet but you can download the dependencies by running:

```bash
# Depending on your go version you might not need the -compat flag
$ go mod tidy -compat=1.17
```

## Writing the Resolver

If you try to build the binary right now you'll receive the following
error:

```bash
$ go build -o /dev/null ./cmd/demoresolver

cmd/demoresolver/main.go:11:78: cannot use &resolver{} (type *resolver) as
type framework.Resolver in argument to framework.NewController:
        *resolver does not implement framework.Resolver (missing GetName method)
```

We've already defined our own `resolver` type but in order to get the
resolver running you'll need to add the methods defined in [the
`framework.Resolver` interface](../pkg/resolution/resolver/framework/interface.go)
to your `main.go` file. Going through each method in turn:

## The `Initialize` method

This method is used to start any libraries or otherwise setup any
prerequisites your resolver needs. For this example we won't need
anything so this method can just return `nil`.

```go
// Initialize sets up any dependencies needed by the resolver. None atm.
func (r *resolver) Initialize(context.Context) error {
  return nil
}
```

## The `GetName` method

This method returns a string name that will be used to refer to this
resolver. You'd see this name show up in places like logs. For this
simple example we'll return `"Demo"`:

```go
// GetName returns a string name to refer to this resolver by.
func (r *resolver) GetName(context.Context) string {
  return "Demo"
}
```

## The `GetSelector` method

This method should return a map of string labels and their values that
will be used to direct requests to this resolver. For this example the
only label we're interested in matching on is defined by
`tektoncd/resolution`:

```go
// GetSelector returns a map of labels to match requests to this resolver.
func (r *resolver) GetSelector(context.Context) map[string]string {
  return map[string]string{
    common.LabelKeyResolverType: "demo",
  }
}
```

What this does is tell the resolver framework that any
`ResolutionRequest` object with a label of
`"resolution.tekton.dev/type": "demo"` should be routed to our
example resolver.

We'll also need to add another import for this package at the top:

```go
import (
  "context"
  
  // Add this one; it defines LabelKeyResolverType we use in GetSelector
  "github.com/tektoncd/pipeline/pkg/resolution/common"
  "github.com/tektoncd/pipeline/pkg/resolution/resolver/framework"
  "knative.dev/pkg/injection/sharedmain"
  pipelinev1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
)
```

## The `ValidateParams` method

The `ValidateParams` method checks that the params submitted as part of
a resolution request are valid. Our example resolver doesn't expect
any params so we'll simply ensure that the given map is empty.

```go
// ValidateParams ensures parameters from a request are as expected.
func (r *resolver) ValidateParams(ctx context.Context, params map[string]string) error {
  if len(params) > 0 {
    return errors.New("no params allowed")
  }
  return nil
}
```

You'll also need to add the `"errors"` package to your list of imports at
the top of the file.

## The `Resolve` method

We implement the `Resolve` method to do the heavy lifting of fetching
the contents of a file and returning them. For this example we're just
going to return a hard-coded string of YAML. Since Tekton Pipelines
currently only supports fetching Pipeline resources via remote
resolution that's what we'll return.

The method signature we're implementing here has a
`framework.ResolvedResource` interface as one of its return values. This
is another type we have to implement but it has a small footprint:

```go
// Resolve uses the given params to resolve the requested file or resource.
func (r *resolver) Resolve(ctx context.Context, params map[string]string) (framework.ResolvedResource, error) {
  return &myResolvedResource{}, nil
}

// our hard-coded resolved file to return
const pipeline = `
apiVersion: tekton.dev/v1beta1
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
type myResolvedResource struct {}

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
```

Best practice: In order to enable Tekton Chains to record the source 
information of the remote data in the SLSA provenance, the resolver should 
implement the `RefSource()` method to return a correct RefSource value. See the 
following example.

```go
// RefSource is the source reference of the remote data that records where the remote 
// file came from including the url, digest and the entrypoint.
func (*myResolvedResource) RefSource() *pipelinev1.RefSource {
	return &v1.RefSource{
		URI: "https://github.com/user/example",
		Digest: map[string]string{
			"sha1": "example",
		},
		EntryPoint: "foo/bar/task.yaml",
	}
}
```

## The deployment configuration

Finally, our resolver needs some deployment configuration so that it can
run in Kubernetes.

A full description of the config is beyond the scope of a short howto
but in summary we'll tell Kubernetes to run our resolver application
along with some environment variables and other configuration that the
underlying `knative` framework expects. The deployed application is put
in the `tekton-pipelines` namespace and uses `ko` to build its
container image. Finally the `ServiceAccount` our deployment uses is
`tekton-pipelines-resolvers`, which is the default `ServiceAccount` shared by all
resolvers in the `tekton-pipelines-resolvers` namespace.

The full configuration follows:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: demoresolver
  namespace: tekton-pipelines-resolvers
spec:
  replicas: 1
  selector:
    matchLabels:
      app: demoresolver
  template:
    metadata:
      labels:
        app: demoresolver
    spec:
      affinity:
        podAntiAffinity:
          preferredDuringSchedulingIgnoredDuringExecution:
          - podAffinityTerm:
              labelSelector:
                matchLabels:
                  app: demoresolver
              topologyKey: kubernetes.io/hostname
            weight: 100
      serviceAccountName: tekton-pipelines-resolvers
      containers:
      - name: controller
        image: ko://example.com/demoresolver/cmd/demoresolver
        resources:
          requests:
            cpu: 100m
            memory: 100Mi
          limits:
            cpu: 1000m
            memory: 1000Mi
        ports:
        - name: metrics
          containerPort: 9090
        env:
        - name: SYSTEM_NAMESPACE
          valueFrom:
            fieldRef:
              fieldPath: metadata.namespace
        - name: CONFIG_LOGGING_NAME
          value: config-logging
        - name: CONFIG_OBSERVABILITY_NAME
          value: config-observability
        - name: METRICS_DOMAIN
          value: tekton.dev/resolution
        securityContext:
          allowPrivilegeEscalation: false
          readOnlyRootFilesystem: true
          runAsNonRoot: true
          capabilities:
            drop:
            - all
```

Phew, ok, put all that in a file at `config/demo-resolver-deployment.yaml`
and you'll be ready to deploy your application to Kubernetes and see it
work!

## Trying it out

Now that all the code is written your new resolver should be ready to
deploy to a Kubernetes cluster. We'll use `ko` to build and deploy the
application:

```bash
$ ko apply -f ./config/demo-resolver-deployment.yaml
```

Assuming the resolver deployed successfully you should be able to see it
in the output from the following command:

```bash
$ kubectl get deployments -n tekton-pipelines

# And here's approximately what you should see when you run this command:
NAME          READY   UP-TO-DATE   AVAILABLE   AGE
controller    1/1     1            1           2d21h
demoresolver  1/1     1            1           91s
webhook       1/1     1            1           2d21
```

To exercise your new resolver, let's submit a request for its hard-coded
pipeline. Create a file called `test-request.yaml` with the following
content:

```yaml
apiVersion: resolution.tekton.dev/v1beta1
kind: ResolutionRequest
metadata:
  name: test-request
  labels:
    resolution.tekton.dev/type: demo
```

And submit this request with the following command:

```bash
$ kubectl apply -f ./test-request.yaml && kubectl get --watch resolutionrequests
```

You should soon see your ResolutionRequest printed to screen with a True
value in its SUCCEEDED column:

```bash
resolutionrequest.resolution.tekton.dev/test-request created
NAME           SUCCEEDED   REASON
test-request   True
```

Press Ctrl-C to get back to the command line.

If you now take a look at the ResolutionRequest's YAML you'll see the
hard-coded pipeline yaml in its `status.data` field. It won't be totally
recognizable, though, because it's encoded as base64. Have a look with the
following command:

```bash
$ kubectl get resolutionrequest test-request -o yaml
```

You can convert that base64 data back into yaml with the following
command:

```bash
$ kubectl get resolutionrequest test-request -o jsonpath="{$.status.data}" | base64 -d
```

Great work, you've successfully written a Resolver from scratch!

## Next Steps

At this point you could start to expand the `Resolve()` method in your
Resolver to fetch data from your storage backend of choice.

Or if you prefer to take a look at a more fully-realized example of a
Resolver, see the [code for the `gitresolver` hosted in the Tekton
Pipeline repo](https://github.com/tektoncd/pipeline/tree/main/pkg/resolution/resolver/git/).

Finally, another direction you could take this would be to try writing a
`PipelineRun` for Tekton Pipelines that speaks to your Resolver. Can
you get a `PipelineRun` to execute successfully that uses the hard-coded
`Pipeline` your Resolver returns?

---

Except as otherwise noted, the content of this page is licensed under the
[Creative Commons Attribution 4.0 License](https://creativecommons.org/licenses/by/4.0/),
and code samples are licensed under the
[Apache 2.0 License](https://www.apache.org/licenses/LICENSE-2.0).
