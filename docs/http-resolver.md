<!--
---

linkTitle: "HTTP Resolver"
weight: 311
---
-->

# HTTP Resolver

This resolver responds to type `http`.

## Parameters

| Param Name                 | Description                                                                                                                                                                | Example Value                                                                                   |   |
|----------------------------|----------------------------------------------------------------------------------------------------------------------------------------------------------------------------|-------------------------------------------------------------------------------------------------|---|
| `url`                      | The URL to fetch from                                                                                                                                                      | <https://raw.githubusercontent.com/tektoncd-catalog/git-clone/main/task/git-clone/git-clone.yaml> |   |
| `http-username`            | An optional username when fetching a task with credentials (need to be used in conjunction with `http-password-secret`)                                                    | `git`                                                                                           |   |
| `http-password-secret`     | An optional secret in the PipelineRun namespace with a reference to a password when fetching a task with credentials (need to be used in conjunction with `http-username`) | `http-password`                                                                                 |   |
| `http-password-secret-key` | An optional key in the `http-password-secret` to be used when fetching a task with credentials                                                                             | Default: `password`                                                                             |   |

A valid URL must be provided. Only HTTP or HTTPS URLs are supported.

## Requirements

- A cluster running Tekton Pipeline v0.41.0 or later.
- The [built-in remote resolvers installed](./install.md#installing-and-configuring-remote-task-and-pipeline-resolution).
- The `enable-http-resolver` feature flag in the `resolvers-feature-flags` ConfigMap in the
  `tekton-pipelines-resolvers` namespace set to `true`.
- [Beta features](./additional-configs.md#beta-features) enabled.

## Configuration

This resolver uses a `ConfigMap` for its settings. See
[`../config/resolvers/http-resolver-config.yaml`](../config/resolvers/http-resolver-config.yaml)
for the name, namespace and defaults that the resolver ships with.

### Options

| Option Name                 | Description                                          | Example Values         |
|-----------------------------|------------------------------------------------------|------------------------|
| `fetch-timeout`              | The maximum time any fetching of URL resolution may take. **Note**: a global maximum timeout of 1 minute is currently enforced on _all_ resolution requests. | `1m`, `2s`, `700ms`                                              |

## Usage

### Task Resolution

```yaml
apiVersion: tekton.dev/v1beta1
kind: TaskRun
metadata:
  name: remote-task-reference
spec:
  taskRef:
    resolver: http
    params:
    - name: url
      value: https://raw.githubusercontent.com/tektoncd-catalog/git-clone/main/task/git-clone/git-clone.yaml
```

### Task Resolution with Basic Auth

```yaml
apiVersion: tekton.dev/v1beta1
kind: TaskRun
metadata:
  name: remote-task-reference
spec:
  taskRef:
    resolver: http
    params:
    - name: url
      value: https://raw.githubusercontent.com/owner/private-repo/main/task/task.yaml
    - name: http-username
      value: git
    - name: http-password-secret
      value: git-secret
    - name: http-password-secret-key
      value: git-token
```

### Pipeline Resolution

```yaml
apiVersion: tekton.dev/v1beta1
kind: PipelineRun
metadata:
  name: http-demo
spec:
  pipelineRef:
    resolver: http
    params:
    - name: url
      value: https://raw.githubusercontent.com/tektoncd/catalog/main/pipeline/build-push-gke-deploy/0.1/build-push-gke-deploy.yaml
```

---

Except as otherwise noted, the content of this page is licensed under the
[Creative Commons Attribution 4.0 License](https://creativecommons.org/licenses/by/4.0/),
and code samples are licensed under the
[Apache 2.0 License](https://www.apache.org/licenses/LICENSE-2.0).
