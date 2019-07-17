# Tests

To run tests:

```shell
# Land the latest codes
ko apply -f ./config/

# Unit tests
go test ./...

# Integration tests (against your current kube cluster)
go test -v -count=1 -tags=e2e -timeout=20m ./test
```

## Unit tests

Unit tests live side by side with the code they are testing and can be run with:

```shell
go test ./...
```

By default `go test` will not run [the end to end tests](#end-to-end-tests),
which need `-tags=e2e` to be enabled.

### Unit testing Controllers

Kubernetes [client-go](https://godoc.org/k8s.io/client-go) provides a number of
fake clients and objects for unit testing. The ones we are using are:

1. [Fake Kubernetes client](https://godoc.org/k8s.io/client-go/kubernetes/fake):
   Provides a fake REST interface to interact with Kubernetes API
1. [Fake pipeline client](./../pkg/client/clientset/versioned/fake/clientset_generated.go)
   : Provides a fake REST PipelineClient Interface to interact with Pipeline
   CRDs.

You can create a fake PipelineClient for the Controller under test like
[this](https://github.com/tektoncd/pipeline/blob/d97057a58e16c11ca5e38b780a7bb3ddae42bae1/pkg/reconciler/v1alpha1/pipelinerun/pipelinerun_test.go#L209):

```go
import (
    fakepipelineclientset "github.com/tektoncd/pipeline/pkg/client/clientset/versioned/fake
)
pipelineClient := fakepipelineclientset.NewSimpleClientset()
```

This
[pipelineClient](https://github.com/tektoncd/pipeline/blob/d97057a58e16c11ca5e38b780a7bb3ddae42bae1/pkg/client/clientset/versioned/clientset.go#L34)
is initialized with no runtime objects. You can also initialize the client with
Kubernetes objects and can interact with them using the
`pipelineClient.Pipeline()`

```go
import (
    v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

obj := &v1alpha1.PipelineRun {
    ObjectMeta: metav1.ObjectMeta {
        Name:      "name",
        Namespace: "namespace",
    },
    Spec: v1alpha1.PipelineRunSpec {
        PipelineRef: v1alpha1.PipelineRef {
            Name:       "test-pipeline",
            APIVersion: "a1",
        },
    }
}
pipelineClient := fakepipelineclientset.NewSimpleClientset(obj)
objs := pipelineClient.Pipeline().PipelineRuns("namespace").List(v1.ListOptions{})
// You can verify if List was called in your test like this
action :=  pipelineClient.Actions()[0]
if action.GetVerb() != "list" {
    t.Errorf("expected list to be called, found %s", action.GetVerb())
}
```

To test the Controller of _CRD (CustomResourceDefinitions)_, you need to add the
CRD to the [informers](./../pkg/client/informers) so that the
[listers](./../pkg/client/listers) can get the access.

For example, the following code will test `PipelineRun`

```go
pipelineClient := fakepipelineclientset.NewSimpleClientset()
sharedInfomer := informers.NewSharedInformerFactory(pipelineClient, 0)
pipelineRunsInformer := sharedInfomer.Pipeline().V1alpha1().PipelineRuns()

obj := &v1alpha1.PipelineRun {
    ObjectMeta: metav1.ObjectMeta {
        Name:      "name",
        Namespace: "namespace",
    },
    Spec: v1alpha1.PipelineRunSpec {
        PipelineRef: v1alpha1.PipelineRef {
            Name:       "test-pipeline",
            APIVersion: "a1",
        },
    }
}
pipelineRunsInformer.Informer().GetIndexer().Add(obj)
```

## End to end tests

### Setup

Environment variables used by end to end tests:

- `KO_DOCKER_REPO` - Set this to an image registry your tests can push images to
- `GCP_SERVICE_ACCOUNT_KEY_PATH` - Tests that need to interact with GCS buckets
  will use the json credentials at this path to authenticate with GCS.

- In Kaniko e2e test, setting `GCP_SERVICE_ACCOUNT_KEY_PATH` as the path of the
  GCP service account JSON key which has permissions to push to the registry
  specified in `KO_DOCKER_REPO` will enable Kaniko to use those credentials when
  pushing an image.
- In GCS taskrun test, GCP service account JSON key file at path
  `GCP_SERVICE_ACCOUNT_KEY_PATH`, if present, is used to generate Kubernetes
  secret to access GCS bucket.
- In Storage artifact bucket test, the `GCP_SERVICE_ACCOUNT_KEY_PATH` JSON key
  is used to create/delete a bucket which will be used for output to input
  linking by the `PipelineRun` controller.

To create a service account usable in the e2e tests:

```bash
PROJECT_ID=your-gcp-project
ACCOUNT_NAME=service-account-name
# gcloud configure project
gcloud config set project $PROJECT_ID

# create the service account
gcloud iam service-accounts create $ACCOUNT_NAME --display-name $ACCOUNT_NAME
EMAIL=$(gcloud iam service-accounts list | grep $ACCOUNT_NAME | awk '{print $2}')

# add the storage.admin policy to the account so it can push containers
gcloud projects add-iam-policy-binding $PROJECT_ID --member serviceAccount:$EMAIL --role roles/storage.admin

# create the JSON key
gcloud iam service-accounts keys create config.json --iam-account $EMAIL

export GCP_SERVICE_ACCOUNT_KEY_PATH="$PWD/config.json"
```

### Running

End to end tests live in this directory. To run these tests, you must provide
`go` with `-tags=e2e`. By default the tests run against your current kubeconfig
context, but you can change that and other settings with [the flags](#flags):

```shell
go test -v -count=1 -tags=e2e -timeout=20m ./test
go test -v -count=1 -tags=e2e -timeout=20m ./test --kubeconfig ~/special/kubeconfig --cluster myspecialcluster
```

You can also use
[all of flags defined in `knative/pkg/test`](https://github.com/knative/pkg/tree/master/test#flags).

### Flags

- By default the e2e tests against the current cluster in `~/.kube/config` using
  the environment specified in
  [your environment variables](/DEVELOPMENT.md#environment-setup).
- Since these tests are fairly slow, running them with logging enabled is
  recommended (`-v`).
- Using [`--logverbose`](#output-verbose-log) to see the verbose log output from
  test as well as from k8s libraries.
- Using `-count=1` is
  [the idiomatic way to disable test caching](https://golang.org/doc/go1.10#test).
- The end to end tests take a long time to run so a value like `-timeout=20m`
  can be useful depending on what you're running

You can [use test flags](#flags) to control the environment your tests run
against, i.e. override
[your environment variables](/DEVELOPMENT.md#environment-setup):

```bash
go test -v -tags=e2e -count=1 ./test --kubeconfig ~/special/kubeconfig --cluster myspecialcluster
```

Tests importing [`github.com/tektoncd/pipeline/test`](#adding-integration-tests)
recognize the
[flags added by `knative/pkg/test`](https://github.com/knative/pkg/tree/master/test#flags).

### Running specific test cases

To run all the test cases with their names starting with the same letters, e.g.
TestTaskRun, use
[the `-run` flag with `go test`](https://golang.org/cmd/go/#hdr-Testing_flags):

```bash
go test -v -tags=e2e -count=1 ./test -run ^TestTaskRun
```

### Running YAML tests

To run the YAML e2e tests, run the following command:

```bash
./test/e2e-tests-yaml.sh
```

### Adding integration tests

In the [`test`](/test/) dir you will find several libraries in the `test`
package you can use in your tests.

This library exists partially in this directory and partially in
[`knative/pkg/test`](https://github.com/knative/pkg/tree/master/test).

The libs in this dir can:

- [`init_test.go`](./init_test.go) initializes anything needed globally be the
  tests
- [Get access to client objects](#get-access-to-client-objects)
- [Generate random names](#generate-random-names)
- [Poll Pipeline resources](#poll-pipeline-resources)

All integration tests _must_ be marked with the `e2e`
[build constraint](https://golang.org/pkg/go/build/) so that `go test ./...` can
be used to run only [the unit tests](#unit-tests), i.e.:

```go
// +build e2e
```

#### Create Tekton objects

To create Tekton objects (e.g. `Task`, `Pipeline`, …), you can use the
[`github.com/tektoncd/pipeline/test/builder`](./builder) package to reduce
noise:

```go
import tb "github.com/tektoncd/pipeline/test/builder"

func MyTest(t *testing.T){
    // Pipeline
    pipeline := tb.Pipeline("tomatoes", "namespace",
        tb.PipelineSpec(tb.PipelineTask("foo", "banana")),
    )
    // … and PipelineRun
    pipelineRun := tb.PipelineRun("pear", "namespace",
        tb.PipelineRunSpec("tomatoes", tb.PipelineRunServiceAccount("inexistent")),
    )
    // And do something with them
    // […]
    if _, err := c.PipelineClient.Create(pipeline); err != nil {
        t.Fatalf("Failed to create Pipeline `%s`: %s", "tomatoes", err)
    }
    if _, err := c.PipelineRunClient.Create(pipelineRun); err != nil {
        t.Fatalf("Failed to create PipelineRun `%s`: %s", "pear", err)
    }
}
```

#### Get access to client objects

To initialize client objects use [the command line flags](#use-flags) which
describe the environment:

```go
func setup(t *testing.T) *test.Clients {
    clients, err := test.NewClients(kubeconfig, cluster, namespaceName)
    if err != nil {
        t.Fatalf("Couldn't initialize clients: %v", err)
    }
    return clients
}
```

The `Clients` struct contains initialized clients for accessing:

- Kubernetes objects
- [`Pipelines`](https://github.com/tektoncd/pipeline#pipeline)

For example, to create a `Pipeline`:

```bash
_, err = clients.PipelineClient.Pipelines.Create(test.Route(namespaceName, pipelineName))
```

And you can use the client to clean up resources created by your test (e.g. in
[your test cleanup](https://github.com/knative/pkg/tree/master/test#ensure-test-cleanup)):

```go
func tearDown(clients *test.Clients) {
    if clients != nil {
        clients.Delete([]string{routeName}, []string{configName})
    }
}
```

_See [clients.go](./clients.go)._

#### Generate random names

You can use the function `GenerateName()` to append a random string for `crd`s
or anything else, so that your tests can use unique names each time they run.

```go
import "github.com/tektoncd/pipeline/pkg/names"

namespace := names.SimpleNameGenerator.GenerateName("arendelle")
```

#### Poll Pipeline resources

After creating Pipeline resources or making changes to them, you will need to
wait for the system to realize those changes. You can use polling methods to
check the resources reach the desired state.

The `WaitFor*` functions use the Kubernetes
[`wait` package](https://godoc.org/k8s.io/apimachinery/pkg/util/wait). For
polling they use
[`PollImmediate`](https://godoc.org/k8s.io/apimachinery/pkg/util/wait#PollImmediate)
behind the scene. And the callback function is
[`ConditionFunc`](https://godoc.org/k8s.io/apimachinery/pkg/util/wait#ConditionFunc),
which returns a `bool` to indicate if the function should stop, and an `error`
to indicate if there was an error.

For example, you can poll a `TaskRun` until having a `Status.Condition`:

```go
err = WaitForTaskRunState(c, hwTaskRunName, func(tr *v1alpha1.TaskRun) (bool, error) {
    if len(tr.Status.Conditions) > 0 {
        return true, nil
    }
    return false, nil
}, "TaskRunHasCondition")
```

_[Metrics will be emitted](https://github.com/knative/pkg/tree/master/test#emit-metrics)
for these `Wait` methods tracking how long test poll for._

## Presubmit tests

[`presubmit-tests.sh`](./presubmit-tests.sh) is the entry point for all tests
[run on presubmit by Prow](../CONTRIBUTING.md#pull-request-process).

You can run this locally with:

```shell
test/presubmit-tests.sh
test/presubmit-tests.sh --build-tests
test/presubmit-tests.sh --unit-tests
```

Prow is configured in
[the knative `config.yaml` in `tektoncd/plumbing`](https://github.com/tektoncd/plumbing/blob/master/ci/prow/config.yaml)
via the sections for `tektoncd/pipeline`.

### Running presubmit integration tests

The presubmit integration tests entrypoint will run:

- [The integration tests](#integration-tests)
- A test of [our example CRDs](../examples/README.md#testing-the-examples)

When run using Prow, integration tests will try to get a new cluster using
[boskos](https://github.com/kubernetes/test-infra/tree/master/boskos) and
[these hardcoded GKE projects](https://github.com/tektoncd/plumbing/blob/master/ci/prow/boskos/resources.yaml#L15),
which only
[the `tektoncd/plumbing` OWNERS](https://github.com/tektoncd/plumbing/blob/master/OWNERS)
have access to.

If you would like to run the integration tests against your cluster, you can use
the current context in your kubeconfig, provide `KO_DOCKER_REPO` (as specified
in the [DEVELOPMENT.md](../DEVELOPMENT.md#environment-setup)), use
`e2e-tests.sh` directly and provide the `--run-tests` argument:

```shell
export KO_DOCKER_REPO=gcr.io/my_docker_repo
test/e2e-tests.sh --run-tests
```

Or you can set `$PROJECT_ID` to a GCP project and rely on
[kubetest](https://github.com/kubernetes/test-infra/tree/master/kubetest) to
setup a cluster for you:

```shell
export PROJECT_ID=my_gcp_project
test/presubmit-tests.sh --integration-tests
```
