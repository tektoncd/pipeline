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

Create a GitHub personal token with `admin:repo_hook` persmissions
at least. This will be used to create the webhook.

```bash
# Define GitHub variables, including the token and webhook secret
GITHUB_TOKEN=<set-the-token-here>
GITHUB_SECRET=$(ruby -rsecurerandom -e 'puts SecureRandom.hex(20)')
GITHUB_ORG=<org> # The org or user where your fork is hosted
GITHUB_REPO=<repo> # The name of the fork, typically "plumbing"
GITHUB_USER=<github-user> # Your GitHub username

# Deploy plumbing resources. Run from the root of your local clone
# The sed command injects your fork GitHub org in the CEL filters
kustomize build tekton/ci | \
  sed -E 's/tektoncd(\/p[^i]+|\(|\/'\'')/'$GITHUB_ORG'\1/g' | \
  kubectl create -f -

# Create the secret used by the GitHub interceptor
kubectl create secret generic ci-webhook -n tektonci --from-literal=secret=$GITHUB_SECRET

# Expose the event listener via Smee
kubectl port-forward service/el-tekton-ci-webhook -n tektonci 9999:8080 &> el-tekton-ci-webhook-pf.log &
smee --target http://127.0.0.1:9999/ &> smee.log &
SMEE_TARGET=$(tail -1 smee.log | cut -d'/' -f3-)

# Install a Task to create the webhook, create a secret used by it
kubectl apply -f https://raw.githubusercontent.com/tektoncd/triggers/master/docs/getting-started/create-webhook.yaml
kubectl create secret generic github --from-literal=token=$GITHUB_TOKEN --from-literal=secret=$GITHUB_SECRET

# Setup the webhook in your fork that points to the smee service
tkn task start create-webhook -p ExternalDomain=$SMEE_TARGET -p GitHubUser=$GITHUB_USER -p GitHubRepo=$GITHUB_REPO -p GitHubOrg=$GITHUB_ORG -p GitHubSecretName=github -p GitHubAccessTokenKey=token -p GitHubSecretStringKey=secret
```

Push or sync a PR to your fork, and watch the pipelines running in your
cluster. You can use `redeliver` from the GitHub webhook UI to resend
and event and re-trigger pipelines.

This guide does not include setting up status updates back to GitHub.
If you need those for development, you'll need to ensure that:

- Install resources under `tekton/resources` to deploy the
  `tekton-events` event listener
- Configure Tekton send cloud events to the `tekton-events`
  event listener like on the [dogfooding](https://github.com/tektoncd/plumbing/blob/master/tekton/cd/pipeline/overlays/dogfooding/config-defaults.yaml) cluster.
- Create the [secret](https://github.com/tektoncd/plumbing/blob/534861ab15eb5787cac51512eaae6ca2101a7573/tekton/resources/ci/github-template.yaml#L121-L123)
  needed by the GitHub update jobs to update status checks.

See also _[CONTRIBUTING.md](CONTRIBUTING.md).
