<!--
---
linkTitle: "Container Contract"
weight: 401
---
-->

# Container Contract

Each container image that executes a `Step` in a [`Task`](tasks.md) must
comply with the container contract described in this document.

When a `Task` instantiates and runs containers that execute the `Steps` in the `Task`,
each container's `entrypoint` is overwritten with a custom binary that ensures the
containers within the `Task's` Pod are executed in the order specified in the `Task`
definition. Because of this, we highly recommend that you always specify a `command` value.
However, if you're using the [`script` field](tasks.md#running-scripts-within-steps) to
embed a script within a `Step`, **do not** specify a `command` value. For example:

```
        - name: setup-comment
          image: python:3-alpine
          script: |
            #!/usr/bin/env python
            import json
            (...)
```

If you do not specify a `command` value, the Pipelines controller performs a lookup for
the `entrypoint` value in the associated remote container registry. If the image is in
a private registry, you must include an [`ImagePullSecret`](https://kubernetes.io/docs/tasks/configure-pod-container/configure-service-account/#add-imagepullsecrets-to-a-service-account)
value in the service account definition used by the `Task`.
The Pipelines controller uses this value unless the service account is not 
defined, at which point it assumes the value of `default`.

The final fallback occurs to the Docker config specified in the `$HOME/.docker/config.json` file.
If no credentials are specified in any of the locations described above, the Pipelines
controller performs an anonymous lookup of the image.

For example, consider the following `Task`, which uses two images named
`gcr.io/cloud-builders/gcloud` and `gcr.io/cloud-builders/docker`. In this example, the
Pipelines controller retrieves the `entrypoint` value from the registry, which allows
the `Task` to execute the `gcloud` and `docker` commands, respectively.

```yaml
spec:
  steps:
    - image: gcr.io/cloud-builders/gcloud
      command: [gcloud]
    - image: gcr.io/cloud-builders/docker
      command: [docker]
```

However, if you specify a custom `command` value, the controller uses that value instead:

```yaml
spec:
  steps:
    - image: gcr.io/cloud-builders/gcloud
      command:
        - bash
        - -c
        - echo "Hello!"
```

You also have the option to specify `args` to go with your `command` value:

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
