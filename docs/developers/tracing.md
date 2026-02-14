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

## External Parent Trace Context

External systems can provide a parent trace context for PipelineRuns by setting
the `tekton.dev/deliveryTraceparent` annotation to a valid [W3C Trace Context](https://www.w3.org/TR/trace-context/) traceparent string.

When this annotation is present and valid, Tekton creates a child span under
the provided parent context. This enables distributed tracing across system
boundaries - for example, a CI/CD orchestrator can propagate its trace context
to PipelineRuns it creates, and each PipelineRun will appear as a distinct
child span in the trace.

### Difference from pipelinerunSpanContext

The `tekton.dev/pipelinerunSpanContext` annotation is used for internal Tekton
trace propagation and adopts the provided span context directly. In contrast,
`tekton.dev/deliveryTraceparent` creates a **new child span** under the external
parent. This distinction is important when multiple PipelineRuns share the same
delivery parent (e.g., multiple integration tests triggered by a single event) -
each PipelineRun will have its own execution span, making them distinguishable
in traces.

### Example

```yaml
apiVersion: tekton.dev/v1
kind: PipelineRun
metadata:
  name: my-pipelinerun
  annotations:
    tekton.dev/deliveryTraceparent: "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01"
spec:
  pipelineRef:
    name: my-pipeline
```

### Precedence Rules

The trace context is determined in the following order:

1. If `status.SpanContext` already exists, it is used (PipelineRun is already being traced)
2. If `tekton.dev/pipelinerunSpanContext` annotation exists, it is adopted directly
3. If `tekton.dev/deliveryTraceparent` annotation exists and is valid, a child span is created under it
4. Otherwise, a new root span is created

### Traceparent Format

The traceparent must follow the W3C Trace Context format:
`{version}-{trace-id}-{parent-id}-{flags}`

Example: `00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01`

- `version`: 2 hex digits (currently `00`)
- `trace-id`: 32 hex digits
- `parent-id`: 16 hex digits
- `flags`: 2 hex digits (e.g., `01` for sampled)
