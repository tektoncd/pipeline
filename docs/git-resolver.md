<!--
---
linkTitle: "Git Resolver"
weight: 309
---
-->

# Simple Git Resolver

## Resolver Type

This Resolver responds to type `git`.

## Parameters

| Param Name   | Description                                                                                                            | Example Value                                               |
|--------------|------------------------------------------------------------------------------------------------------------------------|-------------------------------------------------------------|
| `url`        | URL of the repo to fetch and clone anonymously. Either `url`, or `repo` (with `org`) must be specified, but not both.  | `https://github.com/tektoncd/catalog.git`                   |
| `repo`       | The repository to find the resource in. Either `url`, or `repo` (with `org`) must be specified, but not both.          | `pipeline`, `test-infra`                                    |
| `org`        | The organization to find the repository in. Default can be set in [configuration](#configuration).                     | `tektoncd`, `kubernetes`                                    |
| `revision`   | Git revision to checkout a file from. This can be commit SHA, branch or tag.                                           | `aeb957601cf41c012be462827053a21a420befca` `main` `v0.38.2` |
| `pathInRepo` | Where to find the file in the repo.                                                                                    | `task/golang-build/0.3/golang-build.yaml`                  |

## Requirements

- A cluster running Tekton Pipeline v0.41.0 or later.
- The [built-in remote resolvers installed](./install.md#installing-and-configuring-remote-task-and-pipeline-resolution).
- The `enable-git-resolver` feature flag in the `resolvers-feature-flags` ConfigMap in the
  `tekton-pipelines-resolvers` namespace set to `true`.
- [Beta features](./additional-configs.md#beta-features) enabled.

## Configuration

This resolver uses a `ConfigMap` for its settings. See
[`../config/resolvers/git-resolver-config.yaml`](../config/resolvers/git-resolver-config.yaml)
for the name, namespace and defaults that the resolver ships with.

### Options

| Option Name                  | Description                                                                                                                                                   | Example Values                                                   |
|------------------------------|---------------------------------------------------------------------------------------------------------------------------------------------------------------|------------------------------------------------------------------|
| `default-revision`           | The default git revision to use if none is specified                                                                                                          | `main`                                                           |
| `fetch-timeout`              | The maximum time any single git clone resolution may take. **Note**: a global maximum timeout of 1 minute is currently enforced on _all_ resolution requests. | `1m`, `2s`, `700ms`                                              |
| `default-url`                | The default git repository URL to use for anonymous cloning if none is specified.                                                                             | `https://github.com/tektoncd/catalog.git`                        |
| `scm-type`                   | The SCM provider type. Required if using the authenticated API with `org` and `repo`.                                                                         | `github`, `gitlab`, `gitea`, `bitbucketcloud`, `bitbucketserver` |
| `server-url`                 | The SCM provider's base URL for use with the authenticated API. Not needed if using github.com, gitlab.com, or BitBucket Cloud                                | `api.internal-github.com`                                        |
| `api-token-secret-name`      | The Kubernetes secret containing the SCM provider API token. Required if using the authenticated API with `org` and `repo`.                                   | `bot-token-secret`                                               |
| `api-token-secret-key`       | The key within the token secret containing the actual secret. Required if using the authenticated API with `org` and `repo`.                                  | `oauth`, `token`                                                 |
| `api-token-secret-namespace` | The namespace containing the token secret, if not `default`.                                                                                                  | `other-namespace`                                                |
| `default-org`                | The default organization to look for repositories under when using the authenticated API, if not specified in the resolver parameters. Optional.              | `tektoncd`, `kubernetes`                                         |

## Usage

The `git` resolver has two modes: cloning a repository anonymously, or fetching individual files via an SCM provider's API using an API token.

### Anonymous Cloning

Anonymous cloning is supported only for public repositories. This mode clones the full git repo.

#### Task Resolution

```yaml
apiVersion: tekton.dev/v1beta1
kind: TaskRun
metadata:
  name: git-clone-demo-tr
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

#### Pipeline resolution

```yaml
apiVersion: tekton.dev/v1beta1
kind: PipelineRun
metadata:
  name: git-clone-demo-pr
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

### Authenticated API

The authenticated API supports private repositories, and fetches only the file at the specified path rather than doing a full clone.

When using the authenticated API, [providers with implementations in `go-scm`](https://github.com/jenkins-x/go-scm/tree/main/scm/driver) can be used.
Note that not all `go-scm` implementations have been tested with the `git` resolver, but it is known to work with:
  * github.com and GitHub Enterprise
  * gitlab.com and self-hosted Gitlab
  * Gitea
  * BitBucket Server
  * BitBucket Cloud

#### Task Resolution

```yaml
apiVersion: tekton.dev/v1beta1
kind: TaskRun
metadata:
  name: git-api-demo-tr
spec:
  taskRef:
    resolver: git
    params:
    - name: org
      value: tektoncd
    - name: repo
      value: catalog
    - name: revision
      value: main
    - name: pathInRepo
      value: task/git-clone/0.6/git-clone.yaml
```

#### Pipeline resolution

```yaml
apiVersion: tekton.dev/v1beta1
kind: PipelineRun
metadata:
  name: git-api-demo-pr
spec:
  pipelineRef:
    resolver: git
    params:
    - name: org
      value: tektoncd
    - name: repo
      value: catalog
    - name: revision
      value: main
    - name: pathInRepo
      value: pipeline/simple/0.1/simple.yaml
  params:
  - name: name
    value: Ranni
```

## `ResolutionRequest` Status
`ResolutionRequest.Status.RefSource` field captures the source where the remote resource came from. It includes the 3 subfields: `url`, `digest` and `entrypoint`.
- `url`
  - If users choose to use anonymous cloning, the url is just user-provided value for the `url` param in the [SPDX download format](https://spdx.github.io/spdx-spec/package-information/#77-package-download-location-field). 
  - If scm api is used, it would be the clone URL of the repo fetched from scm repository service in the [SPDX download format](https://spdx.github.io/spdx-spec/package-information/#77-package-download-location-field). 
- `digest`
  - The algorithm name is fixed "sha1", but subject to be changed to "sha256" once Git eventually uses SHA256 at some point later. See https://git-scm.com/docs/hash-function-transition for more details.
  - The value is the actual commit sha at the moment of resolving the resource even if a user provides a tag/branch name for the param `revision`.
- `entrypoint`: the user-provided value for the `path` param.

Example:
- Pipeline Resolution
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
      value: https://github.com/<username>/<reponame>.git
    - name: revision
      value: main
    - name: pathInRepo
      value: pipeline.yaml
```

- `ResolutionRequest`
```yaml
apiVersion: resolution.tekton.dev/v1alpha1
kind: ResolutionRequest
metadata:
  ... 
spec:
  params:
    pathInRepo: pipeline.yaml
    revision: main
    url: https://github.com/<username>/<reponame>.git
status:
  refSource:
    uri: git+https://github.com/<username>/<reponame>.git
    digest:
      sha1: <The latest commit sha on main at the moment of resolving>
    entrypoint: pipeline.yaml
  data: a2luZDogUGxxxx...
```

---

Except as otherwise noted, the content of this page is licensed under the
[Creative Commons Attribution 4.0 License](https://creativecommons.org/licenses/by/4.0/),
and code samples are licensed under the
[Apache 2.0 License](https://www.apache.org/licenses/LICENSE-2.0).
