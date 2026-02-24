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
* endpoint: API endpoint for jaeger collector to send the traces. By default the endpoint is configured to be `http://jaeger-collector.jaeger.svc.cluster.local:14268/api/traces`.
* credentialsSecret: Name of the secret which contains `username` and `password` to authenticate against the endpoint

## Remote Parent Trace Context

Per the [W3C Trace Context](https://www.w3.org/TR/trace-context/) specification,
a component receiving a `traceparent` should create child spans under that parent.
Tekton supports this for PipelineRuns via resource annotations, which serve as the
TextMapCarrier equivalent since PipelineRuns are created via Kubernetes manifests
rather than HTTP requests.

The following annotations are recognized:

| Annotation | W3C Spec | Required |
|---|---|---|
| `tekton.dev/traceparent` | [Trace Context `traceparent`](https://www.w3.org/TR/trace-context/#traceparent-header) | Yes |
| `tekton.dev/tracestate` | [Trace Context `tracestate`](https://www.w3.org/TR/trace-context/#tracestate-header) | No |

When `tekton.dev/traceparent` is present and valid, Tekton extracts a remote
SpanContext using a standard OpenTelemetry TextMapPropagator. The
`PipelineRun:ReconcileKind` span starts as a direct child of the remote parent.
This enables distributed tracing across system boundaries — for example, an
orchestrator can propagate its trace context to PipelineRuns it creates, and
each PipelineRun will appear as a child span in the same trace.

### Relationship to `pipelinerunSpanContext`

Both `tekton.dev/traceparent` and `tekton.dev/pipelinerunSpanContext` accept a
SpanContext and parent the `ReconcileKind` span under it. The behavioral outcome
is identical — the difference is in format and intended use:

| | `pipelinerunSpanContext` | `traceparent` |
|---|---|---|
| **Format** | JSON-encoded OTel TextMapCarrier (`{"traceparent":"00-...","tracestate":"..."}`) | Raw W3C `traceparent` string (`00-...`) |
| **Intended use** | Tekton-internal: parent PipelineRun propagating to child PipelineRuns | External systems providing a remote parent |
| **Validation** | None — malformed JSON silently produces a root span | Validates via `IsValid()`, logs a warning on invalid input |
| **Precedence** | Checked first (wins if both are present) | Checked second |

The `traceparent` annotation provides a standard W3C format that external systems
can produce without constructing Tekton's internal JSON carrier. An external system
*could* achieve the same result by injecting a correctly-formatted
`pipelinerunSpanContext`, but `traceparent` offers a documented, stable API for
this purpose.

### Example

```yaml
apiVersion: tekton.dev/v1
kind: PipelineRun
metadata:
  name: my-pipelinerun
  annotations:
    tekton.dev/traceparent: "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01"
    tekton.dev/tracestate: "vendorname=opaqueValue"
spec:
  pipelineRef:
    name: my-pipeline
```

### Precedence Rules

The trace context is determined in the following order:

1. If `status.SpanContext` already exists, it is used (PipelineRun is already being traced)
2. If `tekton.dev/pipelinerunSpanContext` annotation exists, it is adopted directly
3. If `tekton.dev/traceparent` annotation exists and is valid, it is adopted as a remote parent
4. Otherwise, a new root span is created

### Traceparent Format

The traceparent must follow the [W3C Trace Context](https://www.w3.org/TR/trace-context/#traceparent-header) format:
`{version}-{trace-id}-{parent-id}-{flags}`

Example: `00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01`

- `version`: 2 hex digits (currently `00`)
- `trace-id`: 32 hex digits
- `parent-id`: 16 hex digits
- `flags`: 2 hex digits (e.g., `01` for sampled)
