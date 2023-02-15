# ![pipe](./pipe.png) Tekton Pipelines

[![Go Report Card](https://goreportcard.com/badge/tektoncd/pipeline)](https://goreportcard.com/report/tektoncd/pipeline)
[![CII Best Practices](https://bestpractices.coreinfrastructure.org/projects/4020/badge)](https://bestpractices.coreinfrastructure.org/projects/4020)

The Tekton Pipelines project provides k8s-style resources for declaring
CI/CD-style pipelines.

Tekton Pipelines are **Cloud Native**:

- Run on Kubernetes
- Have Kubernetes clusters as a first class type
- Use containers as their building blocks

Tekton Pipelines are **Decoupled**:

- One Pipeline can be used to deploy to any k8s cluster
- The Tasks which make up a Pipeline can easily be run in isolation
- Resources such as git repos can easily be swapped between runs

Tekton Pipelines are **Typed**:

- The concept of typed resources means that for a resource such as an `Image`,
  implementations can easily be swapped out (e.g. building with
  [kaniko](https://github.com/GoogleContainerTools/kaniko) v.s.
  [buildkit](https://github.com/moby/buildkit))

## Want to start using Pipelines

- [Installing Tekton Pipelines](docs/install.md)
- Jump in with [the "Getting started" tutorial!](https://tekton.dev/docs/getting-started/tasks/)
- Take a look at our [roadmap](roadmap.md)
- Discover our [releases](releases.md)

### Required Kubernetes Version

- Starting from the v0.24.x release of Tekton: **Kubernetes version 1.18 or later**
- Starting from the v0.27.x release of Tekton: **Kubernetes version 1.19 or later**
- Starting from the v0.30.x release of Tekton: **Kubernetes version 1.20 or later**
- Starting from the v0.33.x release of Tekton: **Kubernetes version 1.21 or later**
- Starting from the v0.39.x release of Tekton: **Kubernetes version 1.22 or later**
- Starting from the v0.41.x release of Tekton: **Kubernetes version 1.23 or later**
- Starting from the v0.45.x release of Tekton: **Kubernetes version 1.24 or later**

### Read the docs

The latest version of our docs is available at:

- [Installation Guide @ HEAD](DEVELOPMENT.md#install-pipeline)
- [Docs @ HEAD](/docs/README.md)
- [Examples @ HEAD](/examples)

Version specific links are available in the [releases](releases.md) page and on the
[Tekton website](https://tekton.dev/docs).

_See [our API compatibility policy](api_compatibility_policy.md) for info on the
stability level of the API._

_See [our Deprecations table](docs/deprecations.md) for features that have been
deprecated and the earliest date they'll be removed._

## Migrating

### v1alpha1 to v1beta1

In the move from v1alpha1 to v1beta1 several spec fields and Tekton
CRDs were updated or removed .

For users migrating their Tasks and Pipelines from v1alpha1 to v1beta1, check
out [the spec changes and migration paths](./docs/migrating-v1alpha1-to-v1beta1.md).

## Want to contribute

We are so excited to have you!

- See [CONTRIBUTING.md](CONTRIBUTING.md) for an overview of our processes
- See [DEVELOPMENT.md](DEVELOPMENT.md) for how to get started
- [Deep dive](./docs/developers/README.md) into demystifying the inner workings
  (advanced reading material)
- Look at our
  [good first issues](https://github.com/tektoncd/pipeline/issues?q=is%3Aissue+is%3Aopen+label%3A%22good+first+issue%22)
  and our
  [help wanted issues](https://github.com/tektoncd/pipeline/issues?q=is%3Aissue+is%3Aopen+label%3A%22help+wanted%22)
