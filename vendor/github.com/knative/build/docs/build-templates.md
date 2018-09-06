# Build Templates

This document serves to define "Build Templates", and their capabilities.

A set of curated and supported build templates is available in the
[`build-templates`](https://github.com/elafros/build-templates) repo.

## What is a Build Template?

A `BuildTemplate` serves to encapsulate a shareable [build](./builds.md)
process with some limited paramaterization capabilities.

### Example Template

For example, a `BuildTemplate` to encapsulate a `Dockerfile` build might look
something like this:

**NB:** Building a container image using `docker build` on-cluster is _very
unsafe_. Use [kaniko](https://github.com/GoogleContainerTools/kaniko) instead.
This is only used for the purposes of demonstration.

```yaml
spec:
  parameters:
  # This has no default, and is therefore required.
  - name: IMAGE
    description: Where to publish the resulting image.

  # These may be overriden, but provide sensible defaults.
  - name: DIRECTORY
    description: The directory containing the build context.
    default: /workspace
  - name: DOCKERFILE_NAME
    description: The name of the Dockerfile
    default: Dockerfile

  steps:
  - name: dockerfile-build
    image: gcr.io/cloud-builders/docker
    workingdir: "${DIRECTORY}"
    args: ["build", "--no-cache",
           "--tag", "${IMAGE}",
           "--file", "${DOCKERFILE_NAME}"
           "."]
    volumeMounts:
    - name: docker-socket
      mountPath: /var/run/docker.sock

  - name: dockerfile-push
    image: gcr.io/cloud-builders/docker
    args: ["push", "${IMAGE}"]
    volumeMounts:
    - name: docker-socket
      mountPath: /var/run/docker.sock

  # As an implementation detail, this template mounts the host's daemon socket.
  volumes:
  - name: docker-socket
    hostPath:
      path: /var/run/docker.sock
      type: Socket
```

In this example, `parameters` describes the formal arguments for the template.
The `description` is used for diagnostic messages during validation (and maybe
in the future for UI). The `default` value enables a template to have a
graduated complexity, where options are only overridden when the user strays
from some set of sane defaults.

`steps` and `volumes` are just like in a [`Build`(./builds.md) resource, but
may contain references to parameters in the form: `${PARAMETER_NAME}`.

The `steps` of a template replace those of its Build. The `volumes` of a
template augment those of its Build.

### Example Builds

For the sake of illustrating re-use, here are several example Builds
instantiating the `BuildTemplate` above (`dockerfile-build-and-push`).

Build `mchmarny/rester-tester`:
```yaml
spec:
  source:
    git:
      url: https://github.com/mchmarny/rester-tester.git
      revision: master
  template:
    name: dockerfile-build-and-push
    arguments:
    - name: IMAGE
      value: gcr.io/my-project/rester-tester
```

Build `googlecloudplatform/cloud-builder`'s `wget` builder:
```yaml
spec:
  source:
    git:
      url: https://github.com/googlecloudplatform/cloud-builders.git
      revision: master
  template:
    name: dockerfile-build-and-push
    arguments:
    - name: IMAGE
      value: gcr.io/my-project/wget
    # Optional override to specify the subdirectory containing the Dockerfile
    - name: DIRECTORY
      value: /workspace/wget
```

Build `googlecloudplatform/cloud-builder`'s `docker` builder with `17.06.1`:
```yaml
spec:
  source:
    git:
      url: https://github.com/googlecloudplatform/cloud-builders.git
      revision: master
  template:
    name: dockerfile-build-and-push
    arguments:
    - name: IMAGE
      value: gcr.io/my-project/docker
    # Optional overrides
    - name: DIRECTORY
      value: /workspace/docker
    - name: DOCKERFILE_NAME
      value: Dockerfile-17.06.1
```
