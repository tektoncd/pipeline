# Container Contract

Each container image used as a step in a [`Task`](task.md) must comply with a
specific contract.

## Entrypoint

When containers are run in a `Task`, the `entrypoint` of the container will be
overwritten with a custom binary that redirects the logs to a separate location
for aggregating the log output (this currently does nothing but is to support
future work, see [#107](https://github.com/knative/build-pipeline/issues/107)).
As such, it is always recommended to explicitly specify a command.

When `command` is not explicitly set, the controller will attempt to lookup the
entrypoint from the remote registry.

Due to this metadata lookup, if you use a private image as a step inside a
`Task`, the build-pipeline controller needs to be able to access that registry.
The simplest way to accomplish this is to add a `.docker/config.json` at
`$HOME/.docker/config.json`, which will then be used by the controller when
performing the lookup

For example, in the following Task with the images,
`gcr.io/cloud-builders/gcloud` and `gcr.io/cloud-builders/docker`, the
entrypoint would be resolved from the registry, resulting in the tasks running
`gcloud` and `docker` respectively.

```yaml
spec:
  steps:
    - image: gcr.io/cloud-builders/gcloud
      command: [gcloud]
    - image: gcr.io/cloud-builders/docker
      command: [docker]
```

However, if the steps specified a custom `command`, that is what would be used.

```yaml
spec:
  steps:
    - image: gcr.io/cloud-builders/gcloud
      command:
        - bash
        - -c
        - echo "Hello!"
```

You can also provide `args` to the image's `command`:

```yaml
steps:
  - image: ubuntu
    command: ["/bin/bash"]
    args: ["-c", "echo hello $FOO"]
    env:
      - name: "FOO"
        value: "world"
```

---

Except as otherwise noted, the content of this page is licensed under the
[Creative Commons Attribution 4.0 License](https://creativecommons.org/licenses/by/4.0/),
and code samples are licensed under the
[Apache 2.0 License](https://www.apache.org/licenses/LICENSE-2.0).
