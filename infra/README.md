# Tekton Test Infra

This directory contains configuration and documentation for the test
infrastructure for the Tekton Project.

## Prow

Prow is currently running on GKE in the project `tekton-releases`. The cluster
name is `prow`.

Configuration lives in the `prow` directory here.

## Ingress

Ingress for prow is configured using [cert-manager](https://github.com/jetstack/cert-manager/).

`cert-manager` was installed via `Helm` using this [guide](https://docs.cert-manager.io/en/latest/getting-started/)

`prow.tekton.dev` is configured as a host on the prow `Ingress` resource.
https://prow.tekton.dev is pointed at the Cluster ingress address.

## Gubernator

Gubernator is configured on App Engine in the project `tekton-releases`.

It was deployed using `gcloud app deploy .` with no configuration changes.

## Boskos

Boskos configuration lives in the `boskos` directory here. It runs in the `prow`
cluster of the `tekton-releases` project, in the namespace `test-pods`.

### Adding a project

Projects are created in GCP and added to the `boskos/boskos-config.yaml` file.

Make sure the IAM account:
`prow-account@tekton-releases.iam.gserviceaccount.com` has Editor permissions.
