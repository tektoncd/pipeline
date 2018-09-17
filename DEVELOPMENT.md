# Developing

## Getting started

1. Create [a GitHub account](https://github.com/join)
1. Setup [GitHub access via
   SSH](https://help.github.com/articles/connecting-to-github-with-ssh/)
1. Install [requirements](#requirements)
1. [Set up a kubernetes cluster](https://github.com/knative/serving/blob/master/docs/creating-a-kubernetes-cluster.md)
1. [Configure kubectl to use your cluster](https://kubernetes.io/docs/tasks/access-application-cluster/configure-access-multiple-clusters/)

### Requirements

You must install these tools:

1. [`go`](https://golang.org/doc/install): The language `Pipeline CRD` is built in
1. [`git`](https://help.github.com/articles/set-up-git/): For source control
1. [`dep`](https://github.com/golang/dep): For managing external Go
   dependencies.
1. [`kubectl`](https://kubernetes.io/docs/tasks/tools/install-kubectl/): For interacting with your kube cluster (also required for kubebuidler)
1. [`kustomize`](https://github.com/kubernetes-sigs/kustomize): Required for kubebuilder
1. [`kubebuilder`](https://book.kubebuilder.io/quick_start.html): For generating CRD related
boilerplate (see [docs on iterating with kubebuilder](#installing-and-running)) - Note that
the installation instructions default to `mac`, use the tabs at the top to switch to `linux`

## Iterating

### Dependencies

This repo uses [`dep`](https://golang.github.io/dep/docs/daily-dep.html) for dependency management:

* Update the deps with `dep ensure -update`
* `dep ensure` should be a no-op
* Add a dep with `dep ensure -add $MY_DEP`

### Changing types

When updating types, you should regenerate any generated code with:

```bash
make generate
```

Then test this by [installing and running](#installing-and-running).

### Installing and running

The skeleton for this project was generated using [kubebuilder](https://book.kubebuilder.io/quick_start.html),
which created our [Makefile](./Makefile). The `Makefile` will call out to `kubectl`,
so you must [configure your `kubeconfig` to use your cluster](https://kubernetes.io/docs/tasks/access-application-cluster/configure-access-multiple-clusters/).

Then a typical development cycle will look like this:

```bash
# Add/update CRDs in your kubernetes cluster
make install

# Run your controller locally, (stop execution with `ctrl-c`)
make run

# In another terminal, deploy tasks
kubectl apply -f config/samples
```

You will also want to [run tests](#running-tests).

### Running tests

Run the tests with:

```bash
make test
```

### Where's the code?

To make changes to these CRDs, you will probably interact with:

* The CRD type definitions in [./pkg/apis/pipeline/v1beta1](./pkg/apis/pipeline/v1beta1)
* The controllers in [./pkg/controller](./pkg/controller)
