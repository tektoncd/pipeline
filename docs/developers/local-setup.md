# Local Setup

This section provides guidelines for running Tekton on your local workstation via the following methods:

- [Docker for Desktop](#using-docker-desktop)
- [Minikube](#using-minikube)

## Using Docker Desktop

### Prerequisites

Complete these prerequisites to run Tekton locally using Docker Desktop:

- Install the [required tools](https://github.com/tektoncd/pipeline/blob/main/DEVELOPMENT.md#requirements).
- Install [Docker Desktop](https://www.docker.com/products/docker-desktop)
- Configure Docker Desktop ([Mac](https://docs.docker.com/docker-for-mac/#resources), [Windows](https://docs.docker.com/docker-for-windows/#resources))to use six CPUs, 10 GB of RAM and 2GB of swap space.
- Set `host.docker.internal:5000` as an insecure registry with Docker for Desktop. See the [Docker insecure registry documentation](https://docs.docker.com/registry/insecure/).
  for details.
- Pass `--insecure` as an argument to your Kaniko tasks so that you can push to an insecure registry.
- Run a local (insecure) Docker registry as follows:

  `docker run -d -p 5000:5000 --name registry-srv -e REGISTRY_STORAGE_DELETE_ENABLED=true registry:2`

- (Optional) Install a Docker registry viewer to verify the images have been pushed:

`docker run -it -p 8080:8080 --name registry-web --link registry-srv -e REGISTRY_URL=http://registry-srv:5000/v2 -e REGISTRY_NAME=localhost:5000 hyper/docker-registry-web`

- Verify that you can push to `host.docker.internal:5000/myregistry/<image_name>`.

### Reconfigure `image` resources

You must reconfigure any `image` resource definitions in your `PipelineResources` as follows:

- Set the URL to `host.docker.internal:5000/myregistry/<image_name>`
- Set the `KO_DOCKER_REPO` variable to `localhost:5000/myregistry` before using `ko`
- Set your applications (such as deployment definitions) to push to
  `localhost:5000/myregistry/<image name>`.

> :warning: **`PipelineResources` are [deprecated](deprecations.md#deprecation-table).**
>
> Consider using replacement features instead. Read more in [documentation](migrating-v1alpha1-to-v1beta1.md#replacing-pipelineresources-with-tasks)
> and [TEP-0074](https://github.com/tektoncd/community/blob/main/teps/0074-deprecate-pipelineresources.md).

### Reconfigure logging

- You can keep your logs in memory only without sending them to a logging service
  such as [Stackdriver](https://cloud.google.com/logging/).
- You can deploy Elasticsearch, Beats, or Kibana locally to view logs. You can find an
  example configuration at <https://github.com/mgreau/tekton-pipelines-elastic-tutorials>.
- To learn more about obtaining logs, see [Logs](logs.md).

## Using Minikube

### Prerequisites

Complete these prerequisites to run Tekton locally using Minikube:

- Install the [required tools](https://github.com/tektoncd/pipeline/blob/main/DEVELOPMENT.md#requirements).
- Install [minikube](https://kubernetes.io/docs/tasks/tools/install-minikube/) and start a session as follows:
```bash
minikube start --memory 6144 --cpus 2
```
- Point your shell to minikube's docker-daemon by running `eval $(minikube -p minikube docker-env)`
- Set up a [registry on minikube](https://github.com/kubernetes/minikube/tree/master/deploy/addons/registry-aliases) by running `minikube addons enable registry` and `minikube addons enable registry-aliases`

### Reconfigure `image` resources

The `registry-aliases` addon will create several aliases for the minikube registry. You'll need to reconfigure your `image` resource definitions to use one of these aliases in your `PipelineResources` (for this tutorial, we use `example.com`; for a full list of aliases, you can run `minikube ssh -- cat /etc/hosts`. You can also configure your own alias by editing minikube's `/etc/hosts` file and the `coredns` configmap in the `kube-system` namespace).

- Set the URL to `example.com/<image_name>`
- When using `ko`, be sure to [use the `-L` flag](https://github.com/google/ko/blob/master/README.md#with-minikube) (i.e. `ko apply -L -f config/`)
- Set your applications (such as deployment definitions) to push to
  `example.com/<image name>`.

If you wish to use a different image URL, you can add the appropriate line to minikube's `/etc/hosts`.

> :warning: **`PipelineResources` are [deprecated](deprecations.md#deprecation-table).**
>
> Consider using replacement features instead. Read more in [documentation](migrating-v1alpha1-to-v1beta1.md#replacing-pipelineresources-with-tasks)
> and [TEP-0074](https://github.com/tektoncd/community/blob/main/teps/0074-deprecate-pipelineresources.md).

### Reconfigure logging

See the information in the "Docker for Desktop" section

## Using kind and local docker registry

### Prerequisites

Complete these prerequisites to run Tekton locally using Kind:

- Install the [required tools](https://github.com/tektoncd/pipeline/blob/main/DEVELOPMENT.md#requirements).
- Install [Docker](https://www.docker.com/get-started).
- Install [kind](https://kind.sigs.k8s.io/).

### Use local registry without authentication

See [Using KinD](https://github.com/tektoncd/pipeline/blob/main/DEVELOPMENT.md#using-kind).

### Use local private registry

1. Create password file with basic auth.

```bash
export TEST_USER=testuser
export TEST_PASS=testpassword
if [ ! -f auth ]; then
    mkdir auth
fi
docker run \
 --entrypoint htpasswd \
 httpd:2 -Bbn $TEST_USER $TEST_PASS > auth/htpasswd
```

2. Start kind cluster and local private registry


Execute the script.

```shell
#!/bin/sh
set -o errexit

# create registry container unless it already exists
reg_name='kind-registry'
reg_port='5000'
running="$(docker inspect -f '{{.State.Running}}' "${reg_name}" 2>/dev/null || true)"
if [ "${running}" != 'true' ]; then
 docker run \
   -d --restart=always -p "127.0.0.1:${reg_port}:5000" --name "${reg_name}" \
   -v "$(pwd)"/auth:/auth \
   -e "REGISTRY_AUTH=htpasswd" \
   -e "REGISTRY_AUTH_HTPASSWD_REALM=Registry Realm" \
   -e REGISTRY_AUTH_HTPASSWD_PATH=/auth/htpasswd \
   registry:2
fi

# create a cluster with the local registry enabled in containerd
cat <<EOF | kind create cluster --config=-
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
containerdConfigPatches:
- |-
 [plugins."io.containerd.grpc.v1.cri".registry.mirrors."localhost:${reg_port}"]
   endpoint = ["http://${reg_name}:5000"]
EOF

# connect the registry to the cluster network
# (the network may already be connected)
docker network connect "kind" "${reg_name}" || true

# Document the local registry
# https://github.com/kubernetes/enhancements/tree/master/keps/sig-cluster-lifecycle/generic/1755-communicating-a-local-registry
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: ConfigMap
metadata:
 name: local-registry-hosting
 namespace: kube-public
data:
 localRegistryHosting.v1: |
   host: "localhost:${reg_port}"
   help: "https://kind.sigs.k8s.io/docs/user/local-registry/"
EOF

```

3. Install tekton [pipeline](https://github.com/tektoncd/pipeline/blob/main/docs/install.md) and create the secret in cluster.

```bash
kubectl create secret docker-registry secret-tekton \
  --docker-username=$TEST_USER \
  --docker-password=$TEST_PASS \
  --docker-server=localhost:5000 \
   --namespace=tekton-pipelines
```

4. Config [ko](https://github.com/google/ko#install) and add secret to service acount.

```bash
export KO_DOCKER_REPO='localhost:5000'
```

`200-serviceaccount.yaml`

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
 name: tekton-pipelines-controller
 namespace: tekton-pipelines
 labels:
   app.kubernetes.io/component: controller
   app.kubernetes.io/instance: default
   app.kubernetes.io/part-of: tekton-pipelines
imagePullSecrets:
 - name: secret-tekton
---
apiVersion: v1
kind: ServiceAccount
metadata:
 name: tekton-pipelines-webhook
 namespace: tekton-pipelines
 labels:
   app.kubernetes.io/component: webhook
   app.kubernetes.io/instance: default
   app.kubernetes.io/part-of: tekton-pipelines
imagePullSecrets:
 - name: secret-tekton
```
