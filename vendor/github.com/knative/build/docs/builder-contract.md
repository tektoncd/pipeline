# Builders

This document serves to define what we call `Builder` images, and the
conventions to which they are expected to adhere.

## What is a Builder?

A `Builder` image is a special classification for images that run as a part of
the Build CRD's `steps:`.

For example, in the following Build:

```yaml
spec:
  steps:
  - image: gcr.io/cloud-builders/gcloud
    ...
  - image: gcr.io/cloud-builders/docker
    ...
```

The images `gcr.io/cloud-builders/gcloud` and `gcr.io/cloud-builders/docker` are
"Builders".

### Typical Builders

A Builder is typically a purpose-built container whose entrypoint is a tool that
performs some action and exits with a zero status on success.  These are often
command-line tools, e.g. `git`, `docker`, `mvn`, ...

Typical builders set their `command:` (aka `ENTRYPOINT`) to be the command they
wrap and expect to take `args:` to direct their behavior.

See [here](https://github.com/googlecloudplatform/cloud-builders) and
[here](https://github.com/googlecloudplatform/cloud-builders-community) for more
builders.

### Atypical Builders

It it possible, although less typical to implement the Builder convention by
overriding `command:` and `args:` for example:

```yaml
steps:
- image: ubuntu
  command: ['/bin/bash']
  args: ['-c', 'echo hello $FOO']
  env:
  - name: 'FOO'
    value: 'world'
```

### Specialized Builders

It is also possible for advanced users to create very purpose-built builders.
One example of this are the ["FTL" builders](
https://github.com/GoogleCloudPlatform/runtimes-common/tree/master/ftl#ftl).


## What are the Builder conventions?

Builders should expect a Build to implement the following conventions:
 * `/workspace`: The default working directory will be `/workspace`, which is
 a volume that is filled by the `source:` step and shared across build `steps:`.
 
 * `/builder/home`: This volume is exposed to steps via `$HOME`.
 
 * Credentials attached to the Build's service account may be exposed as Git or
 Docker credentials as outlined [here](./cmd/creds-init/README.md).
