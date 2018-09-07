# Build CRD

This repository implements `Build` and `BuildTemplate` custom resources
for Kubernetes, and a controller for making them work.

If you are interested in contributing, see [CONTRIBUTING.md](./CONTRIBUTING.md)
and [DEVELOPMENT.md](./DEVELOPMENT.md).

## Objective

Kubernetes is emerging as the predominant (if not de facto) container
orchestration layer. It is also quickly becoming the foundational layer on top
of which the ecosystem is building higher-level compute abstractions (PaaS,
FaaS). In order to increase developer velocity, these higher-level compute
abstractions typically operate on source, not just containers, which must be
built.

This repository provides an implementation of the Build [CRD](
https://kubernetes.io/docs/concepts/api-extension/custom-resources/) that runs
Builds on-cluster (by default), because that is the lowest common denominator
that we expect users to have available. It is also possible to write a
[pkg/builder](./pkg/builder) implementation that delegates Builds to hosted
services (e.g. [Google Container Builder](./pkg/builder/google)), but these are
typically more restrictive.

This project as it exists today is not a complete standalone product that could
be used for CI/CD, but it provides a building block to facilitate the
expression of Builds as part of larger systems. It might provide a building
block for CI/CD in [the future](./roadmap-2018.md)

## Terminology and Conventions

* [Builds](./docs/builds.md)
* [Build Templates](./docs/build-templates.md)
* [Builders](./docs/builder-contract.md)
* [Authentication](./docs/auth.md)

## Getting Started

You can install the latest release of the Build CRD by running:
```shell
kubectl create -f https://storage.googleapis.com/build-crd/latest/release.yaml
```

Your account must have the `cluster-admin` role in order to do this. If your
account does not have this role, you can add it:

```
kubectl create clusterrolebinding myname-cluster-admin-binding \
    --clusterrole=cluster-admin \
    --user=myname@example.org
```

### Run your first `Build`

```yaml
apiVersion: build.knative.dev/v1alpha1
kind: Build
metadata:
  name: hello-build
spec:
  steps:
  - name: hello
    image: busybox
    args: ['echo', 'hello', 'build']
```

Run it on your Kubernetes cluster:

```shell
$ kubectl apply -f build.yaml
build "hello-build" created
```

Check that it was created:

```shell
$ kubectl get builds
NAME          AGE
hello-build   4s
```

Get more information about the build:

```shell
$ kubectl get build hello-build -oyaml
apiVersion: build.knative.dev/v1alpha1
kind: Build
...
status:
  builder: Cluster
  cluster:
    namespace: default
    podName: hello-build-jx4ql
  conditions:
  - state: Complete
    status: "True"
  stepStates:
  - terminated:
      reason: Completed
  - terminated:
      reason: Completed
```

This indicates that the build finished successfully, and that it ran on a
pod named `hello-build-jx4ql` -- your build will run on a pod with a
different name. Let's dive into that pod!

```shell
$ kubectl get pod hello-build-jx4ql -oyaml
...lots of interesting stuff!
```

Here's a shortcut for getting a build's underlying `podName`:

```shell
$ kubectl get build hello-build -ojsonpath={.status.cluster.podName}
```

The build's underlying pod also contains the build's logs, in an init
container named after the build step's name. In the case of this example, the
build step's name was `hello`, so the pod will have an init container named
`build-step-hello` which ran the step.

```shell
$ kubectl logs $(kubectl get build hello-build -ojsonpath={.status.cluster.podName}) -c build-step-hello
hello build
```

:tada:
