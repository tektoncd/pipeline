# Plumbing

This repo holds configuration for infrastructure used across the tektoncd org 🏗️:

- Automation runs [in the tektoncd GCP project](gcp.md)
- The script [addpermissions.py](addpermissions.py) gives users access to
  [the GCP projects](gcp.md)
- [Prow](prow/README.md) is used for
  [pull request automation]((https://github.com/tektoncd/community/blob/master/process.md#reviews))
- [Tekton](tekton/README.md) is used to release projects, build docker images and run periodic jobs
- [Ingress](prow/README.md#ingress) configuration for access via `tekton.dev`
- [Gubernator](gubernator/README.md) is used for holding and displaying [Prow](prow/README.md) logs
- [Boskos](boskos/README.md) is used to control a pool of GCP projects which end to end tests can run against
- [Peribolos](tekton/resources/org-permissions/README.md) is used to control org and repo permissions


## Support

If you need support, reach out [in the tektoncd slack](https://github.com/tektoncd/community/blob/master/contact.md#slack)
via the `#plumbing` channel.

[Members of the Tekton governing board](goverance.md)
[have access to the underlying resources](https://github.com/tektoncd/community/blob/master/governance.md#permissions-and-access).
