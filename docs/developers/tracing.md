# Tracing setup

This sections shows how to enable tracing for tekton reconcilers and
capture traces in Jaeger

## Prerequisites

Jaeger should be installed and accessible from the cluster. The easiest
way to set it up is using helm as below

the following command installs Jaeger in `jaeger` namespace

```
helm repo add jaegertracing https://jaegertracing.github.io/helm-charts
helm upgrade -i jaeger jaegertracing/jaeger -n jaeger --create-namespace
```

Use port-forwarding to open the jaeger query UI or adjust the service
type to Loadbalancer for accessing the service directly

```
kubectl port-forward svc/jaeger-query -n jaeger 8080:80
```

Check the official [Jaeger docs](https://www.jaegertracing.io/docs/) on how to work with Jaeger

## Enabling tracing

The configmap `config/config-tracing.yaml` contains the configuration for tracing. It contains the following fields:

* enabled: Set this to true to enable tracing
* endpoint: API endpoint for jaeger collector to send the traces. By default the endpoint is configured to be `http://jaeger-collector.jaeger.svc.cluster.local:4318/v1/traces`.
* credentialsSecret: Name of the secret which contains `username` and `password` to authenticate against the endpoint

## Security considerations for multi-tenant environments

Exported spans from the TaskRun and PipelineRun reconciliation paths include
Kubernetes resource identifiers — specifically resource names and namespaces —
as span attributes. These identifiers are user-controlled and in some
deployments may encode tenant names, customer names, repository names, branch
names, ticket IDs, or internal environment names.

Trace collectors and backends can have different access controls and retention
policies than the Kubernetes API or CloudEvents sink. Operators should treat
their trace backend as a trusted observability system with access controls
equivalent to or stricter than the CloudEvents sink.

Before enabling tracing in multi-tenant environments, review your trace backend
retention and access control policies to ensure that resource identifiers
exposed in span attributes are appropriately protected.

## Reading the reconcile.write_intent attribute

The `PipelineRun:ReconcileKind` and `TaskRun:ReconcileKind` spans carry a
`reconcile.write_intent` attribute with one of four values:

| Value | Meaning | Update paths taken |
| --- | --- | --- |
| `no-op` | nothing changed, so the framework's `DeepEqual` check short-circuits the write | 0 |
| `status-only` | the status differs from what the reconcile started with, so the generated reconciler goes on to write the status subresource | up to 1 |
| `metadata-only` | the reconcile took the branch that updates the object's labels or annotations | up to 1 |
| `metadata-and-status` | both | up to 2 |

The last value is not a formality. Tekton updates labels and annotations while
`ReconcileKind` runs, and the framework updates the status subresource after it
returns; both land on the same etcd key, so such a pass can account for two
revisions. It is an upper bound rather than a count: either request can
conflict or be rejected, and one carrying nothing the stored object does not
already have is a no-op below the API.

It answers "how many reconciliations actually intend to write, versus being
short-circuited", which is the question behind profiling a workload's etcd
revision overhead.

Read it with its limits in mind:

- It is an intent, not an outcome. The metadata half is taken from the branch
  that issues the update, and the status half from the same comparison the
  generated reconciler makes after `ReconcileKind` returns, by which point this
  span has ended. Either update can still be rejected, conflict, or be retried,
  and it is counted here regardless.
- It describes the reconciled object, not everything the pass touched. Creating
  a Pod or a child TaskRun is a write to a different object and is not counted
  here, though it almost always changes the parent's status too. Writes by other
  actors, Chains and Results among them, raise the same key's `version` without
  appearing on these spans at all.
- It is computed only on recording spans, which is not the same set as the ones
  a backend ends up holding. A span can record while its trace is unsampled, and
  a Collector can drop a recorded span later, so what you can count is whatever
  the SDK, the export pipeline and the backend retained.
- Sampling is not an independent draw per reconcile. Tekton persists the
  `traceparent` in the status and restores it on the next pass, and a PipelineRun
  passes it to its TaskRuns, so with a parent-based sampler the decision is made
  once for a trace and inherited by everything under it. Scaling by `1/p` is
  reasonable across many independent runs, and not for one run or a handful of
  objects. Tail- or content-based sampling keeps traces for reasons correlated
  with what happened in them, and no single multiplier recovers a total from
  that.
- Turning tracing on is not free of the thing it measures. The first traced pass
  persists the span context into the status, which is itself a write: it often
  rides along with a status update the reconcile was making anyway, but on an
  otherwise idle or completed object it is the write, and the update it causes
  brings the next reconcile with it.
