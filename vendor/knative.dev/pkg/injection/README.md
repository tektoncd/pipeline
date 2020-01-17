# Knative Dependency Injection

This library supports the production of controller processes with minimal
boilerplate outside of the reconciler implementation.

## Building Controllers

To adopt this model of controller construction, implementations should start
with the following controller constructor:

```go
import (
	"context"

	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"
)

func NewController(ctx context.Context, cmw configmap.Watcher) *controller.Impl {
	logger := logging.FromContext(ctx)

	// TODO(you): Access informers

	c := &Reconciler{
		// TODO(you): Pass listers, clients, and other stuff.
	}
	impl := controller.NewImpl(c, logger, "NameOfController")

	// TODO(you): Set up event handlers.

	return impl
}
```

## Consuming Informers

Knative controllers use "informers" to set up the various event hooks needed to
queue work, and pass the "listers" fed by the informers' caches to the nested
"Reconciler" for accessing objects.

Our controller constructor is passed a `context.Context` onto which we inject
any informers we access. The accessors for these informers are in little stub
libraries, which we have hand rolled for Kubernetes (more on how to generate
these below).

```go
import (
	// These are how you access a client or informer off of the "ctx" passed
	// to set up the controller.
	"knative.dev/pkg/client/injection/kube/client"
	svcinformer "knative.dev/pkg/client/injection/kube/informers/core/v1/service"

	// Other imports ...
)

func NewController(ctx context.Context, cmw configmap.Watcher) *controller.Impl {
	logger := logging.FromContext(ctx)

	// Access informers
	svcInformer := svcinformer.Get(ctx)

	c := &Reconciler{
		// Pass the lister and client to the Reconciler.
		Client:        kubeclient.Get(ctx),
		ServiceLister: svcInformer.Lister(),
	}
	impl := controller.NewImpl(c, logger, "NameOfController")

	// Set up event handlers.
	svcInformer.Informer().AddEventHandler(...)

	return impl
}

```

> How it works: by importing the accessor for a client or informer we link it
> and trigger the `init()` method for its package to run at startup. Each of
> these libraries registers themselves similar to our `init()` and controller
> processes can leverage this to setup and inject all of the registered things
> onto a context to pass to your `NewController()`.

## Testing Controllers

Similar to `injection.Default`, we also have `injection.Fake`. While linking the
normal accessors sets up the former, linking their fakes set up the latter.

```
import (
	"testing"

	// Link the fakes for any informers our controller accesses.
	_ "knative.dev/pkg/client/injection/kube/informers/core/v1/service/fake"

	"k8s.io/client-go/rest"
	"knative.dev/pkg/injection"
	logtesting "knative.dev/pkg/logging/testing"
)

func TestFoo(t *testing.T) {
	ctx := logtesting.TestContextWithLogger(t)

	// Setup a context from all of the injected fakes.
	ctx, _ = injection.Fake.SetupInformers(ctx, &rest.Config{})
	cmw := configmap.NewStaticWatcher(...)
	ctrl := NewController(ctx, cmw)

	// Test the controller process.
}
```

The fake clients also support manually setting up contexts seeded with objects:

```
import (
	"testing"

	fakekubeclient "knative.dev/pkg/client/injection/kube/client/fake"

	"k8s.io/client-go/rest"
	"knative.dev/pkg/injection"
	logtesting "knative.dev/pkg/logging/testing"
)

func TestFoo(t *testing.T) {
	ctx := logtesting.TestContextWithLogger(t)

	objs := []runtime.Object{
		// Some list of initial objects in the client.
	}

	ctx, kubeClient := fakekubeclient.With(ctx, objs...)

	// The fake clients returned by our library are the actual fake type,
	// which enables us to access test-specific methods, e.g.
	kubeClient.AppendReactor(...)

	c := &Reconciler{
		Client: kubeClient,
	}

	// Test the reconciler...
}
```

## Starting controllers

All we do is import the controller packages and pass their constructors along
with a component name (single word) to our shared main. Then our shared main
method sets it all up and runs our controllers.

```go
package main

import (
	// The set of controllers this process will run.
	"github.com/knative/foo/pkg/reconciler/bar"
	"github.com/knative/baz/pkg/reconciler/blah"

	// This defines the shared main for injected controllers.
	"knative.dev/pkg/injection/sharedmain"
)

func main() {
	sharedmain.Main("componentname",
       bar.NewController,
       blah.NewController,
    )
}

```

## Generating Injection Stubs.

To make generating stubs simple, we have harnessed the Kubernetes
code-generation tooling to produce `injection-gen`. Similar to how you might
ordinarily run the other `foo-gen` processed:

```shell
CODEGEN_PKG=${CODEGEN_PKG:-$(cd ${REPO_ROOT}; ls -d -1 ./vendor/k8s.io/code-generator 2>/dev/null || echo ../code-generator)}

${CODEGEN_PKG}/generate-groups.sh "deepcopy,client,informer,lister" \
  github.com/knative/sample-controller/pkg/client github.com/knative/sample-controller/pkg/apis \
  "samples:v1alpha1" \
  --go-header-file ${REPO_ROOT}/hack/boilerplate/boilerplate.go.txt
```

To run `injection-gen` you run the following (replacing the import path and api
group):

```shell

KNATIVE_CODEGEN_PKG=${KNATIVE_CODEGEN_PKG:-$(cd ${REPO_ROOT}; ls -d -1 ./vendor/knative.dev/pkg 2>/dev/null || echo ../pkg)}

${KNATIVE_CODEGEN_PKG}/hack/generate-knative.sh "injection" \
  github.com/knative/sample-controller/pkg/client github.com/knative/sample-controller/pkg/apis \
  "samples:v1alpha1" \
  --go-header-file ${REPO_ROOT}/hack/boilerplate/boilerplate.go.txt

```

To ensure the appropriate tooling is vendored, add the following to
`Gopkg.toml`:

```toml
required = [
  "knative.dev/pkg/codegen/cmd/injection-gen",
]

# .. Constraints

# Keeps things like the generate-knative.sh script
[[prune.project]]
  name = "knative.dev/pkg"
  unused-packages = false
  non-go = false
```
