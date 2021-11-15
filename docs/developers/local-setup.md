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
