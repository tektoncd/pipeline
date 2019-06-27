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
- [Emit metrics](#emit-metrics)
- [Ensure test cleanup](#ensure-test-cleanup)

### Use common test flags

These flags are useful for running against an existing cluster, making use of
your existing
[environment setup](https://github.com/knative/serving/blob/master/DEVELOPMENT.md#environment-setup).

By importing `github.com/knative/pkg/test` you get access to a global variable
called `test.Flags` which holds the values of
[the command line flags](/test/README.md#flags).

```go
logger.Infof("Using namespace %s", test.Flags.Namespace)
```

_See [e2e_flags.go](./e2e_flags.go)._

### Output logs

[When tests are run with `--logverbose` option](README.md#output-verbose-logs),
debug logs will be emitted to stdout.

We are using a generic
[FormatLogger](https://github.com/knative/pkg/blob/master/test/logging/logging.go#L49)
that can be passed in any existing logger that satisfies it. Test can use the
generic [logging methods](https://golang.org/pkg/testing/#T) to log info and
error logs. All the common methods accept generic FormatLogger as a parameter
and tests can pass in `t.Logf` like this:

```go
_, err = pkgTest.WaitForEndpointState(
    clients.KubeClient,
    t.Logf,
    ...),
```

_See [logging.go](./logging/logging.go)._

### Emit metrics

You can emit metrics from your tests using
[the opencensus library](https://github.com/census-instrumentation/opencensus-go),
which
[is being used inside Knative as well](https://github.com/knative/serving/blob/master/docs/telemetry.md).
These metrics will be emitted by the test if the test is run with
[the `--emitmetrics` option](#metrics-flag).

You can record arbitrary metrics with
[`stats.Record`](https://github.com/census-instrumentation/opencensus-go#stats)
or measure latency by creating a instance of
[`trace.Span`](https://github.com/census-instrumentation/opencensus-go#traces)
by using the helper method [`logging.GetEmitableSpan()`](../logging/logger.go)

```go
span := logging.GetEmitableSpan(context.Background(), "MyMetric")
```

- These traces will be emitted automatically by
  [the generic crd polling functions](#check-knative-serving-resources).
- The traces are emitted by [a custom metric exporter](./logging/logging.go)
  that uses the global logger instance.

#### Metric format

When a `trace` metric is emitted, the format is
`metric <name> <startTime> <endTime> <duration>`. The name of the metric is
arbitrary and can be any string. The values are:

- `metric` - Indicates this log is a metric
- `<name>` - Arbitrary string identifying the metric
- `<startTime>` - Unix time in nanoseconds when measurement started
- `<endTime>` - Unix time in nanoseconds when measurement ended
- `<duration>` - The difference in ms between the startTime and endTime

For example:

```bash
metric WaitForConfigurationState/prodxiparjxt/ConfigurationUpdatedWithRevision 1529980772357637397 1529980772431586609 73.949212ms
```

_The [`Wait` methods](#check-knative-serving-resources) (which poll resources)
will prefix the metric names with the name of the function, and if applicable,
the name of the resource, separated by `/`. In the example above,
`WaitForConfigurationState` is the name of the function, and `prodxiparjxt` is
the name of the configuration resource being polled.
`ConfigurationUpdatedWithRevision` is the string passed to
`WaitForConfigurationState` by the caller to identify what state is being polled
for._

### Check Knative Serving resources

_WARNING: this code also exists in
[`knative/serving`](https://github.com/knative/serving/blob/master/test/adding_tests.md#make-requests-against-deployed-services)._

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

_[Metrics will be emitted](#emit-metrics) for these `Wait` method tracking how
long test poll for._

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

Tests importing [`github.com/knative/pkg/test`](#test-library) recognize these
flags:

- [`--kubeconfig`](#specifying-kubeconfig)
- [`--cluster`](#specifying-cluster)
- [`--namespace`](#specifying-namespace)
- [`--logverbose`](#output-verbose-logs)
- [`--emitmetrics`](#metrics-flag)

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

### Metrics flag

Running tests with the `--emitmetrics` argument will cause latency metrics to be
emitted by the tests.

```bash
go test ./test --emitmetrics
```

- To add additional metrics to a test, see
  [emitting metrics](https://github.com/knative/pkg/tree/master/test#emit-metrics).
- For more info on the format of the metrics, see
  [metric format](https://github.com/knative/pkg/tree/master/test#emit-metrics).

[minikube]: https://kubernetes.io/docs/setup/minikube/

---

Except as otherwise noted, the content of this page is licensed under the
[Creative Commons Attribution 4.0 License](https://creativecommons.org/licenses/by/4.0/),
and code samples are licensed under the
[Apache 2.0 License](https://www.apache.org/licenses/LICENSE-2.0).
