# Container Contract

Each container image used as a step in a [`Task`](tasks.md) must comply with a
specific contract.

## Entrypoint

When containers are run in a `Task`, the `entrypoint` of the container will be
overwritten with a custom binary that ensures the containers within the `Task`
pod are executed in the specified order. As such, it is always recommended to
explicitly specify a command.

When `command` is not explicitly set, the controller will attempt to lookup the
entrypoint from the remote registry. If the image is a private registry, the
service account should include an
[ImagePullSecret](https://kubernetes.io/docs/tasks/configure-pod-container/configure-service-account/#add-imagepullsecrets-to-a-service-account).
The Tekton controller will use the `ImagePullSecret` of the service account, and
if service account is empty, `default` is assumed. Next is falling back to
docker config added in a `.docker/config.json` at `$HOME/.docker/config.json`.
If none of these credentials are available the controller will try to lookup the
image anonymously.

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
