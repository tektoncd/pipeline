# Knative build

[![GoDoc](https://godoc.org/github.com/knative/build?status.svg)](https://godoc.org/github.com/knative/build)
[![Go Report Card](https://goreportcard.com/badge/knative/build)](https://goreportcard.com/report/knative/build)

This repository contains a work-in-progress build system that is designed to 
address a common need for cloud native development.

A Knative build extends 
[Kubernetes](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/)
and utilizes existing Kubernetes primitives to provide you with the
ability to run on-cluster container builds from source. For example, you can 
write a build that uses Kubernetes-native resources to obtain your source code
from a repository, build it into container a image, and then run that image. 

While Knative builds are optimized for building, testing, and deploying source 
code, you are still responsible for developing the corresponding components 
that:

* Retrieve source code from repositories.
* Run multiple sequential jobs against a shared filesystem, for example:
  * Install dependencies.
  * Run unit and integration tests.
* Build container images.
* Push container images to an image registry, or deploy them to a cluster.

The goal of a Knative build is to provide a standard, portable, reusable, 
and performance optimized method for defining and running on-cluster container 
image builds. By providing the “boring but difficult” task of running builds on
Kubernetes, Knative saves you from having to independently develop and reproduce
these common Kubernetes-based development processes.

While today, a Knative build does not provide a complete standalone CI/CD 
solution, it does however, provide a lower-level building block that was 
purposefully designed to enable integration and utilization in larger systems.

### Learn more

To learn more about builds in Knative, see the 
[Knative build documentation](https://github.com/knative/docs/tree/master/build).

To learn more about Knative in general, see the 
[Overview](https://github.com/knative/docs/blob/master/README.md).

### Developing Knative builds

If you are interested in contributing to Knative builds:

1. Visit the [How to contribute](./CONTRIBUTING.md) page for information about
   how to become a Knative contributor.
2. Learn how to [set up your development environment](DEVELOPMENT.md).
