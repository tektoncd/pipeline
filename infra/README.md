# Tekton Test Infra

This directory contains configuration and documentation for the test
infrastructure for the Tekton Project.

- [Access](#access)
- [Support](#support)
- [Prow](#prow)
- [Ingress](#ingress)
- [Gubernator](#guberator)
- [Boskos](#boskos)

## Access

Currently users have been manually added by @dlorenc to have access to
[the Prow GCP project](#prow) and [the Boskos projects](#boskos).

TODO(dlorenc, bobcatfish): Create a role/group that has access to these clusters
and guidelines about who should be added.

## Support

Currently you will need to ping @dlorenc or @bobcatfish personally for support.

TODO(dlorenc, bobcatfish): Create support guidelines, add monitoring

## Prow

- Prow runs in
  [the GCP project `tekton-releases`](http://console.cloud.google.com/home/dashboard?project=tekton-releases)
- Prow runs in
  [the GKE cluster `prow`](https://console.cloud.google.com/kubernetes/clusters/details/us-central1-a/prow?project=tekton-releases)
- Prow uses the service account
  `prow-account@tekton-releases.iam.gserviceaccount.com`
  - Secrets for this account are configured in
    [Prow's config.yaml](prow/config.yaml) via
    `gcs_credentials_secret: "test-account"`
- Prow configuration is in [infra/prow](./prow)

_[Prow docs](https://github.com/kubernetes/test-infra/tree/master/prow)._
_[See the CONTRIBUTING.md](../CONTRIBUTING.md#pull-request-process) for more on
Prow and the PR process._

### Updating Prow

TODO(#652) Apply config.yaml changes automatically

Changes to [config.yaml](./prow/config.yaml) are not automatically reflected in
the Prow cluster and must be manually applied.

```bash
# Step 1: Configure kubectl to use the cluster, doesn't have to be via gcloud but gcloud makes it easy
gcloud container clusters get-credentials prow --zone us-central1-a --project tekton-releases

# Step 2: Update the configuration used by Prow
kubectl create configmap config --from-file=config.yaml=infra/prow/config.yaml --dry-run -o yaml | kubectl replace configmap config -f -

# Step 3: Remember to configure kubectl to connect to your regular cluster!
gcloud beta container clusters get-credentials ...
```

## Ingress

- Ingress for prow is configured using
  [cert-manager](https://github.com/jetstack/cert-manager/).
- `cert-manager` was installed via `Helm` using this
  [guide](https://docs.cert-manager.io/en/latest/getting-started/)
- `prow.tekton.dev` is configured as a host on the prow `Ingress` resource.
- https://prow.tekton.dev is pointed at the Cluster ingress address.

## Gubernator

- Gubernator is configured on App Engine in the project `tekton-releases`.
- It was deployed using `gcloud app deploy .` with no configuration changes.

_[Gubernator docs](https://github.com/kubernetes/test-infra/tree/master/gubernator)._

## Boskos

We use Boskos to manage GCP projects which end to end tests are run against.

- Boskos configuration lives [in the `boskos` directory](./boskos)
- It runs [in the `prow` cluster of the `tekton-releases` project](#prow), in
  the namespace `test-pods`

_[Boskos docs](https://github.com/kubernetes/test-infra/tree/master/boskos)._

### Adding a project

Projects are created in GCP and added to the `boskos/boskos-config.yaml` file.

Make sure the IAM account:
`prow-account@tekton-releases.iam.gserviceaccount.com` has Editor permissions.
