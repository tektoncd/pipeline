# Plumbing

This repo holds configuration for infrastructure used across the tektoncd org 🏗️:

- [Tekton](tekton/README.md) resources for:
  - [Continuous Delivery](tekton/README.md): release projects, build docker images and other periodic jobs
  - [Continuous Integration](tekton/ci/README.md): run CI jobs for the various Tekton Projects. *NOTE* this responsibility is used shared with [Prow](prow/README.md)
- [Prow](prow/README.md) manifests and configuration for:
  - Continuous Integration: run CI jobs, merge approved changes (via Tide)
  - Support functionality via various [plugins](prow/plugins.yaml)
- [Ingress](prow/README.md#ingress) configuration for access via `tekton.dev`
- [Gubernator](gubernator/README.md) is used for holding and displaying [Prow](prow/README.md) logs
- [Boskos](boskos/README.md) is used to control a pool of GCP projects which end to end tests can run against
- [Peribolos](tekton/resources/org-permissions/README.md) is used to control org and repo permissions
- [bots](bots/README.md)
- [custom interceptors](tekton/ci/interceptors), used for Tekton based CI
- [Catlin](catlin/), which provides validation for catalog resources

Automation runs [in the tektoncd GCP projects](docs/README.md#gcp-projects).

More details on the infrastructure are available in the [documentation](docs/README.md).

## Support

If you need support, reach out [in the tektoncd slack](https://github.com/tektoncd/community/blob/main/contact.md#slack)
via the `#plumbing` channel.

[Members of the Tekton governing board](https://github.com/tektoncd/community/blob/main/governance.md)
[have access to the underlying resources](https://github.com/tektoncd/community/blob/main/governance.md#permissions-and-access).
