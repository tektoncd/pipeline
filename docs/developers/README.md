# Developer docs

This document is aimed at helping maintainers/developers of project understand
the complexity.

We also recommend checking out the
[Teknical Tekton](https://www.youtube.com/channel/UCUEuKDqyRnGFCE7FpainSpQ)
channel for training and tutorials on Tekton!

## Contents
- Developing on Tekton:
  - [Local Setup](./local-setup.md): Getting your local environment set up to develop on Tekton.
  - [Testing](../../test/README.md): Running Tekton tests.
  - [Tracing](./tracing.md): Enabling Jaeger tracing
- How Tekton is run on Kubernetes:
  - [Controller Logic](./controller-logic.md): How Tekton extends Kubernetes using Knative.
  - [TaskRun Logic](./taskruns.md): How TaskRuns are run in pods.
  - [Resources Labeling](./resources-labelling.md): Labels applied to Tekton resources.
  - [Multi-Tenant Support](./multi-tenant-support.md): Running Tekton in multi-tenant configurations.
- [API Versioning](./api-versioning.md): How Tekton supports multiple API versions and feature gates.
- How specific features are implemented:
  - [Results](./results-lifecycle.md)
  - [Affinity Assistant](./affinity-assistant.md)
