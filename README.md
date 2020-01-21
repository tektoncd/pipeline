# ![pipe](./pipe.png) Tekton Pipelines

[![Go Report Card](https://goreportcard.com/badge/tektoncd/pipeline)](https://goreportcard.com/report/tektoncd/pipeline)

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
- Jump in with [the tutorial!](docs/tutorial.md)

### Read the docs

| Version | Docs | Examples |
| ------- | ---- | -------- |
| [HEAD](DEVELOPMENT.md#install-pipeline) | [Docs @ HEAD](/docs/README.md) | [Examples @ HEAD](/examples) |
| [v0.9.2](https://github.com/tektoncd/pipeline/releases/tag/v0.9.2) | [Docs @ v0.9.2](https://github.com/tektoncd/pipeline/tree/v0.9.2/docs#tekton-pipelines) | [Examples @ v0.9.2](https://github.com/tektoncd/pipeline/tree/v0.9.2/examples#examples) |
| [v0.9.1](https://github.com/tektoncd/pipeline/releases/tag/v0.9.1) | [Docs @ v0.9.1](https://github.com/tektoncd/pipeline/tree/v0.9.1/docs#tekton-pipelines) | [Examples @ v0.9.1](https://github.com/tektoncd/pipeline/tree/v0.9.1/examples#examples) |
| [v0.9.0](https://github.com/tektoncd/pipeline/releases/tag/v0.9.0) | [Docs @ v0.9.0](https://github.com/tektoncd/pipeline/tree/v0.9.0/docs#tekton-pipelines) | [Examples @ v0.9.0](https://github.com/tektoncd/pipeline/tree/v0.9.0/examples#examples) |
| [v0.8.0](https://github.com/tektoncd/pipeline/releases/tag/v0.8.0) | [Docs @ v0.8.0](https://github.com/tektoncd/pipeline/tree/v0.8.0/docs#tekton-pipelines) | [Examples @ v0.8.0](https://github.com/tektoncd/pipeline/tree/v0.8.0/examples#examples) |
| [v0.7.0](https://github.com/tektoncd/pipeline/releases/tag/v0.7.0) | [Docs @ v0.7.0](https://github.com/tektoncd/pipeline/tree/v0.7.0/docs#tekton-pipelines) | [Examples @ v0.7.0](https://github.com/tektoncd/pipeline/tree/v0.7.0/examples#examples) |
| [v0.6.0](https://github.com/tektoncd/pipeline/releases/tag/v0.6.0) | [Docs @ v0.6.0](https://github.com/tektoncd/pipeline/tree/release-v0.6.x/docs#tekton-pipelines) | [Examples @ v0.6.0](https://github.com/tektoncd/pipeline/tree/v0.6.0/examples#examples) |
| [v0.5.2](https://github.com/tektoncd/pipeline/releases/tag/v0.5.2) | [Docs @ v0.5.2](https://github.com/tektoncd/pipeline/tree/v0.5.2/docs#tekton-pipelines) | [Examples @ v0.5.2](https://github.com/tektoncd/pipeline/tree/v0.5.2/examples#examples) |
| [v0.5.1](https://github.com/tektoncd/pipeline/releases/tag/v0.5.1) | [Docs @ v0.5.1](https://github.com/tektoncd/pipeline/tree/v0.5.1/docs#tekton-pipelines) | [Examples @ v0.5.1](https://github.com/tektoncd/pipeline/tree/v0.5.1/examples#examples) |
| [v0.5.0](https://github.com/tektoncd/pipeline/releases/tag/v0.5.0) | [Docs @ v0.5.0](https://github.com/tektoncd/pipeline/tree/v0.5.0/docs#tekton-pipelines) | [Examples @ v0.5.0](https://github.com/tektoncd/pipeline/tree/v0.5.0/examples#examples) |
| [v0.4.0](https://github.com/tektoncd/pipeline/releases/tag/v0.4.0) | [Docs @ v0.4.0](https://github.com/tektoncd/pipeline/tree/v0.4.0/docs#tekton-pipelines) | [Examples @ v0.4.0](https://github.com/tektoncd/pipeline/tree/v0.4.0/examples#examples) |
| [v0.3.1](https://github.com/tektoncd/pipeline/releases/tag/v0.3.1) | [Docs @ v0.3.1](https://github.com/tektoncd/pipeline/tree/v0.3.1/docs#tekton-pipelines) | [Examples @ v0.3.1](https://github.com/tektoncd/pipeline/tree/v0.3.1/examples#examples) |
| [v0.3.0](https://github.com/tektoncd/pipeline/releases/tag/v0.3.0) | [Docs @ v0.3.0](https://github.com/tektoncd/pipeline/tree/v0.3.0/docs#tekton-pipelines) | [Examples @ v0.3.0](https://github.com/tektoncd/pipeline/tree/v0.3.0/examples#examples) |
| [v0.2.0](https://github.com/tektoncd/pipeline/releases/tag/v0.2.0) | [Docs @ v0.2.0](https://github.com/tektoncd/pipeline/tree/v0.2.0/docs#tekton-pipelines) | [Examples @ v0.2.0](https://github.com/tektoncd/pipeline/tree/v0.2.0/examples#examples) |
| [v0.1.0](https://github.com/tektoncd/pipeline/releases/tag/v0.1.0) | [Docs @ v0.1.0](https://github.com/tektoncd/pipeline/tree/v0.1.0/docs#tekton-pipelines) | [Examples @ v0.1.0](https://github.com/tektoncd/pipeline/tree/v0.1.0/examples#examples) |

_See [our API compatibility policy](api_compatibility_policy.md) for info on the
stability level of the API._

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
