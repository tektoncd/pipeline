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

Tekton pipelines controller expects the following environment variables to be able to connect to jaeger:

* `OTEL_EXPORTER_JAEGER_ENDPOINT` is the HTTP endpoint for sending spans directly to a collector.
* `OTEL_EXPORTER_JAEGER_USER` is the username to be sent as authentication to the collector endpoint.
* `OTEL_EXPORTER_JAEGER_PASSWORD` is the password to be sent as authentication to the collector endpoint.

`OTEL_EXPORTER_JAEGER_ENDPOINT` is the only manadatory variable to enable tracing. You can find these variables in the controller manifest as well.
