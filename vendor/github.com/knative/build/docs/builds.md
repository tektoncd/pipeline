# Builds

This document serves to define what "Builds" are, and their capabilities.


## What is a Build?

A `Build` is the main custom resource introduced by this project.
Builds are a "run to completion" resource, which start evaluating upon
creation and run until they are `Complete` (or until the first failing
step, resulting in a `Failed` status).

### Elements of a Build

* [Source](#source)
* [Steps or Template](#steps-or-template)
* [Service Account](#service-account)
* [Volumes](#volumes)

#### Source

Builds may define a `source:` that describes the context with which to seed the
build.  This context is put into `/workspace`, a volume that is mounted into
the `source:` and all of the `steps:`.

Currently, the following types of source are supported:
 * `git:` which can specify a `url:` and a `revision:`.

 * `custom:` which can specify an arbitrary container specification, similar to
 steps (see below).

* `gcs:` which can specify an archive stored at Google Cloud Storage (GCS).


#### Steps or Template

The body of a build is defined through either a set of inlined `steps:` or by
instantiating a [build template](./build-templates.md).

`steps:` is a series of Kubernetes container references adhering to the [builder
contract](./builder-contract.md).  These containers are evaluated in order,
until the first failure (or the last container completes successfully).


#### Service Account

Builds (like Pods) run as a particular service account.  If none is specified, it
is run as the "default" service account in the namespace of the Build.

A custom service account may be specified via `serviceAccountName: build-bot`. Note, service account names other than `build-bot` are acceptable.

Service accounts may be used to project certain types of credentials into the
context of a Build automagically.  For more information on how this process is
configured and how it works, see the [credential](./auth.md).


#### Volumes

Builds can specify a collection of volumes to make available to their build
steps.  These complement the volumes that are implicitly created as part of
the [builder contract](./builder-contract.md).

Volumes can be used in a wide variety of ways, just as in Kubernetes itself.
Common examples include:

 * Mounting in Kubernetes secrets (a manual alternative to [our service account
 model](./cmd/creds-init/README.md)).

 * Creating an extra `emptyDir` volume to act as a multi-step cache (maybe even
 a persistent volume for inter-build caching).

 * Mounting in the host's Docker socket to perform Dockerfile builds.


### Example Builds

Here we will outline a number of simple illustrative builds with fully inlined
specifications.  For examples of Builds leveraging templates, see [the build
template documentation](./build-templates.md).


#### With `git` by `branch`

```yaml
spec:
  source:
    git:
      url: https://github.com/knative/build.git
      revision: master
  steps:
  - image: ubuntu
    args: ["cat", "README.md"]
```

#### With a `gcs` source

```yaml
spec:
  source:
    gcs:
      type: Archive
      location: gs://build-crd-tests/rules_docker-master.zip
  steps:
  - name: list-files
    image: ubuntu:latest
    args: ["ls"]
```

#### With a `custom` source

```yaml
spec:
  source:
    custom:
      image: gcr.io/cloud-builders/gsutil
      args: ["rsync", "gs://some-bucket", "."]
  steps:
  - image: ubuntu
    args: ["cat", "README.md"]
```

#### With an extra volume

```yaml
spec:
  steps:
  - image: ubuntu
    args: ["curl https://foo.com > /var/my-volume"]
    volumeMounts:
    - name: my-volume
      mountPath: /var/my-volume

  - image: ubuntu
    args: ["cat", "/etc/my-volume"]
    volumeMounts:
    - name: my-volume
      mountPath: /etc/my-volume

  volumes:
  - name: my-volume
    emptyDir: {}
```

#### With a private `git` repo via a custom service-account

```yaml
spec:
  # Here build-bot is a ServiceAccount that's had an extra Secret attached
  # with `type: kubernetes.io/basic-auth`.  The username and password are
  # specified per usual, and there is an additional annotation on the Secret
  # of the form: `build.knative.dev/git-0: https://github.com`, which
  # directs us to configure this basic authentication for use with github
  # via Git.
  serviceAccountName: build-bot

  source:
    git:
      url: https://github.com/google/secret-sauce.git
      revision: master
  steps:
  - image: ubuntu
    args: ["cat", "SECRETS.md"]
```

#### Lots 'o trivial examples

For a variety of additional (mostly trivial) examples, see also our [tests
directory](./tests).
