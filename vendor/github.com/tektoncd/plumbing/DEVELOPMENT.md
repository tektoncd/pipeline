# Development guide

## Local Kubernetes Cluster with Tekton

To setup a local development environment for Tekton based CI
and CD pipelines hosted in this repo, the first step is to run
Tekton in a local cluster.

There are several options available, `minikube`, `kind` and `k3c`
are good options to run a kubernetes cluster on your laptop.
If you have go 1.14+ and docker installed, you can use the
automated script for `kind`:

```bash
# Install Kind if not installed yet
GO111MODULE="on" go get sigs.k8s.io/kind@v0.9.0

# Delete any pre-existing Tekton cluster
kind delete cluster --name tekton

# Install the new cluster
./hack/tekton_in_kind.sh
```

If the deployment was successful, you should see a message:

```bash
Tekton Dashboard available at http://localhost:9097”
```

## Tekton Based CI

Setting up a working CI with a local cluster requires using
a service like [smee](https://smee.io) to forward GitHub webhooks
to the service in your local cluster.

In the following steps we use `smee`. You will need `npm` to install
the client. You will also need `tkn` installed to run the webhook
creation task.

```bash
# Install the smee client
npm install -g smee-client
```

You will use webhooks triggered by your personal fork of the
`tektoncd/plumbing` repo and forward them the cluster running on
your laptop.

Create a GitHub personal token with `admin:repo_hook` permissions
at least. This will be used to create the webhook.

Run the script below to deploy Tekton CI on the local kind cluster:

```bash
./hack/tekton_ci.sh -u <github-user> -t <github-token> -o <github-org> -r <github-repo>
```

Push or sync a PR to your fork, and watch the pipelines running in your
cluster. You can use `redeliver` from the GitHub webhook UI to resend
and event and re-trigger pipelines.

This guide does not include setting up status updates back to GitHub.
If you need those for development, you'll need to ensure that:

- Install resources under `tekton/resources` to deploy the
  `tekton-events` event listener
- Configure Tekton send cloud events to the `tekton-events`
  event listener like on the [dogfooding](https://github.com/tektoncd/plumbing/blob/main/tekton/cd/pipeline/overlays/dogfooding/config-defaults.yaml) cluster.
- Create the [secret](https://github.com/tektoncd/plumbing/blob/534861ab15eb5787cac51512eaae6ca2101a7573/tekton/resources/ci/github-template.yaml#L121-L123)
  needed by the GitHub update jobs to update status checks.

See also _[CONTRIBUTING.md](CONTRIBUTING.md).
