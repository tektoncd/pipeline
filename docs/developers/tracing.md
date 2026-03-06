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
