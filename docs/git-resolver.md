# Simple Git Resolver

## Resolver Type

This Resolver responds to type `git`.

## Parameters

| Param Name | Description                                                                  | Example Value                                               |
|------------|------------------------------------------------------------------------------|-------------------------------------------------------------|
| `url`      | URL of the repo to fetch.                                                    | `https://github.com/tektoncd/catalog.git`                   |
| `revision` | Git revision to checkout a file from. This can be commit SHA, branch or tag. | `aeb957601cf41c012be462827053a21a420befca` `main` `v0.38.2` |
| `pathInRepo` | Where to find the file in the repo.                                        | `/task/golang-build/0.3/golang-build.yaml`                  |

## Requirements

- A cluster running Tekton Pipeline v0.40.0 or later, with the `alpha` feature gate enabled.
- The [built-in remote resolvers installed](./install.md#installing-and-configuring-remote-task-and-pipeline-resolution).
- The `enable-git-resolver` feature flag set to `true`.

## Configuration

This resolver uses a `ConfigMap` for its settings. See
[`../config/resolvers/git-resolver-config.yaml`](../config/resolvers/git-resolver-config.yaml)
for the name, namespace and defaults that the resolver ships with.

### Options

| Option Name      | Description                                                                                                                                             | Example Values                            |
|------------------|---------------------------------------------------------------------------------------------------------------------------------------------------------|-------------------------------------------|
| `fetch-timeout`  | The maximum time any single git resolution may take. **Note**: a global maximum timeout of 1 minute is currently enforced on _all_ resolution requests. | `1m`, `2s`, `700ms`                       |
| `default-url`    | The default git repository URL to use if none is specified                                                                                              | `https://github.com/tektoncd/catalog.git` |
| `default-branch` | The default git branch to use if none is specified                                                                                                      | `main`                                    |

## Usage

### Task Resolution

```yaml
apiVersion: tekton.dev/v1beta1
kind: TaskRun
metadata:
  name: remote-task-reference
spec:
  taskRef:
    resolver: git
    params:
    - name: url
      value: https://github.com/tektoncd/catalog.git
    - name: revision
      value: main
    - name: pathInRepo
      value: task/git-clone/0.6/git-clone.yaml
```

### Pipeline resolution

```yaml
apiVersion: tekton.dev/v1beta1
kind: PipelineRun
metadata:
  name: git-demo
spec:
  pipelineRef:
    resolver: git
    params:
    - name: url
      value: https://github.com/tektoncd/catalog.git
    - name: revision
      value: main
    - name: pathInRepo
      value: pipeline/simple/0.1/simple.yaml
  params:
  - name: name
    value: Ranni
```

## What's Supported?

- At the moment the git resolver can only access public repositories.

---

Except as otherwise noted, the content of this page is licensed under the
[Creative Commons Attribution 4.0 License](https://creativecommons.org/licenses/by/4.0/),
and code samples are licensed under the
[Apache 2.0 License](https://www.apache.org/licenses/LICENSE-2.0).
