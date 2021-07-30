# Test

This directory contains tests and testing docs.

- [Test library](#test-library) contains code you can use in your `knative`
  tests
- [Flags](#flags) added by [the test library](#test-library)
- [Unit tests](#running-unit-tests) currently reside in the codebase alongside
  the code they test

## Running unit tests

To run all unit tests:

```bash
go test ./...
```

## Test library

You can use the test library in this dir to:

- [Use common test flags](#use-common-test-flags)
- [Output logs](#output-logs)
- [Ensure test cleanup](#ensure-test-cleanup)

### Use common test flags

These flags are useful for running against an existing cluster, making use of
your existing
[environment setup](https://github.com/knative/serving/blob/main/DEVELOPMENT.md#environment-setup).

By importing `knative.dev/pkg/test` you get access to a global variable called
`test.Flags` which holds the values of
[the command line flags](/test/README.md#flags).

```go
logger.Infof("Using namespace %s", test.Flags.Namespace)
```

_See [e2e_flags.go](./e2e_flags.go)._

### Output logs

[When tests are run with `--logverbose` option](README.md#output-verbose-logs),
debug logs will be emitted to stdout.

We are using a generic
[FormatLogger](https://github.com/knative/pkg/blob/main/test/logging/logging.go#L49)
that can be passed in any existing logger that satisfies it. Test can use the
generic [logging methods](https://golang.org/pkg/testing/#T) to log info and
error logs. All the common methods accept generic FormatLogger as a parameter
and tests can pass in `t.Logf` like this:

```go
_, err = pkgTest.WaitForEndpointState(
    kubeClient,
    t.Logf,
    ...),
```

_See [logging.go](./logging/logging.go)._

### Check Knative Serving resources

_WARNING: this code also exists in
[`knative/serving`](https://github.com/knative/serving/blob/main/test/adding_tests.md#make-requests-against-deployed-services)._

After creating Knative Serving resources or making changes to them, you will
need to wait for the system to realize those changes. You can use the Knative
Serving CRD check and polling methods to check the resources are either in or
reach the desired state.

The `WaitFor*` functions use the kubernetes
[`wait` package](https://godoc.org/k8s.io/apimachinery/pkg/util/wait). To poll
they use
[`PollImmediate`](https://godoc.org/k8s.io/apimachinery/pkg/util/wait#PollImmediate)
and the return values of the function you provide behave the same as
[`ConditionFunc`](https://godoc.org/k8s.io/apimachinery/pkg/util/wait#ConditionFunc):
a `bool` to indicate if the function should stop or continue polling, and an
`error` to indicate if there has been an error.

For example, you can poll a `Configuration` object to find the name of the
`Revision` that was created for it:

```go
var revisionName string
err := test.WaitForConfigurationState(
    clients.ServingClient, configName, func(c *v1alpha1.Configuration) (bool, error) {
        if c.Status.LatestCreatedRevisionName != "" {
            revisionName = c.Status.LatestCreatedRevisionName
            return true, nil
        }
        return false, nil
    }, "ConfigurationUpdatedWithRevision")
```

_See [kube_checks.go](./kube_checks.go)._

### Ensure test cleanup

To ensure your test is cleaned up, you should defer cleanup to execute after
your test completes and also ensure the cleanup occurs if the test is
interrupted:

```go
defer tearDown(clients)
test.CleanupOnInterrupt(func() { tearDown(clients) })
```

_See [cleanup.go](./cleanup.go)._

## Flags

Importing [the test library](#test-library) adds flags that are useful for end
to end tests that need to run against a cluster.

Tests importing [`knative.dev/pkg/test`](#test-library) recognize these flags:

- [`--kubeconfig`](#specifying-kubeconfig)
- [`--cluster`](#specifying-cluster)
- [`--namespace`](#specifying-namespace)
- [`--logverbose`](#output-verbose-logs)
- [`--ingressendpoint`](#specifying-ingress-endpoint)
- [`--dockerrepo`](#specifying-docker-repo)
- [`--tag`](#specifying-tag)
- [`--imagetemplate`](#specifying-image-template)

### Specifying kubeconfig

By default the tests will use the
[kubeconfig file](https://kubernetes.io/docs/concepts/configuration/organize-cluster-access-kubeconfig/)
at `~/.kube/config`. If there is an error getting the current user, it will use
`kubeconfig` instead as the default value. You can specify a different config
file with the argument `--kubeconfig`.

To run tests with a non-default kubeconfig file:

```bash
go test ./test --kubeconfig /my/path/kubeconfig
```

### Specifying cluster

The `--cluster` argument lets you use a different cluster than
[your specified kubeconfig's](#specifying-kubeconfig) active context.

```bash
go test ./test --cluster your-cluster-name
```

The current cluster names can be obtained by running:

```bash
kubectl config get-clusters
```

### Specifying ingress endpoint

The `--ingressendpoint` argument lets you specify a static url to use as the
ingress server during tests. This is useful for Kubernetes configurations which
do not provide external IPs.

```bash
go test ./test --ingressendpoint <k8s-controller-ip>:32380
```

### Specifying namespace

The `--namespace` argument lets you specify the namespace to use for the tests.
By default, tests will use `serving-tests`.

```bash
go test ./test --namespace your-namespace-name
```

### Output verbose logs

The `--logverbose` argument lets you see verbose test logs and k8s logs.

```bash
go test ./test --logverbose
```

### Specifying docker repo

The `--dockerrepo` argument lets you specify a uri of the docker repo where you
have uploaded the test image to using `uploadtestimage.sh`. Defaults to
`$KO_DOCKER_REPO`

```bash
go test ./test --dockerrepo myspecialdockerrepo
```

### Specifying tag

The `--tag` argument lets you specify the version tag for the test images.

```bash
go test ./test --tag v1.0
```

### Specifying image template

The `--imagetemplate` argument lets you specify a template to generate the
reference to an image from the test. Defaults to
`{{.Repository}}/{{.Name}}:{{.Tag}}`

```bash
go test ./test --imagetemplate {{.Repository}}/{{.Name}}:{{.Tag}}
```

---

Except as otherwise noted, the content of this page is licensed under the
[Creative Commons Attribution 4.0 License](https://creativecommons.org/licenses/by/4.0/),
and code samples are licensed under the
[Apache 2.0 License](https://www.apache.org/licenses/LICENSE-2.0).
