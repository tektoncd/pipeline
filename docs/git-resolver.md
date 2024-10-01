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

| Param Name   | Description                                                                                                                                                                | Example Value                                               |
|--------------|----------------------------------------------------------------------------------------------------------------------------------------------------------------------------|-------------------------------------------------------------|
| `url`        | URL of the repo to fetch and clone anonymously. Either `url`, or `repo` (with `org`) must be specified, but not both.                                                      | `https://github.com/tektoncd/catalog.git`                   |
| `repo`       | The repository to find the resource in. Either `url`, or `repo` (with `org`) must be specified, but not both.                                                              | `pipeline`, `test-infra`                                    |
| `org`        | The organization to find the repository in. Default can be set in [configuration](#configuration).                                                                         | `tektoncd`, `kubernetes`                                    |
| `token`      | An optional secret name in the `PipelineRun` namespace to fetch the token from. Defaults to empty, meaning it will try to use the configuration from the global configmap. | `secret-name`, (empty)                                      |
| `tokenKey`   | An optional key in the token secret name in the `PipelineRun` namespace to fetch the token from. Defaults to `token`.                                                      | `token`                                                     |
| `revision`   | Git revision to checkout a file from. This can be commit SHA, branch or tag.                                                                                               | `aeb957601cf41c012be462827053a21a420befca` `main` `v0.38.2` |
| `pathInRepo` | Where to find the file in the repo.                                                                                                                                        | `task/golang-build/0.3/golang-build.yaml`                   |
| `serverURL`  | An optional server URL (that includes the https:// prefix) to connect for API operations                                                                                   | `https:/github.mycompany.com`                               |
| `scmType`    | An optional SCM type to use for API operations                                                                                                                             | `github`, `gitlab`, `gitea`                                 |

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

#### Task Resolution with a custom token to a custom SCM provider

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
    # my-secret-token should be created in the namespace where the
    # pipelinerun is created and contain a GitHub personal access
    # token in the token key of the secret.
    - name: token
      value: my-secret-token
    - name: tokenKey
      value: token
    - name: scmType
      value: github
    - name: serverURL
      value: https://ghe.mycompany.com
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

### Specifying Configuration for Multiple Git Providers

It is possible to specify configurations for multiple providers and even multiple configurations for same provider to use in 
different tekton resources. Firstly, details need to be added in configmap with the unique identifier key prefix.
To use them in tekton resources, pass the unique key mentioned in configmap as an extra param to resolver with key 
`configKey` and value will be the unique key. If no `configKey` param is passed, `default` will be used. Default 
configuration to be used for git resolver can be specified in configmap by either mentioning no unique identifier or 
using identifier `default`

**Note**: `configKey` should not contain `.` while specifying configurations in configmap

### Example Configmap

Multiple configurations can be specified in `git-resolver-config` configmap like this. All keys mentioned above are supported.

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: git-resolver-config
  namespace: tekton-pipelines-resolvers
  labels:
    app.kubernetes.io/component: resolvers
    app.kubernetes.io/instance: default
    app.kubernetes.io/part-of: tekton-pipelines
data:
  # configuration 1, default one to use if no configKey provided or provided with value default
  fetch-timeout: "1m"
  default-url: "https://github.com/tektoncd/catalog.git"
  default-revision: "main"
  scm-type: "github"
  server-url: ""
  api-token-secret-name: ""
  api-token-secret-key: ""
  api-token-secret-namespace: "default"
  default-org: ""

  # configuration 2, will be used if configKey param passed with value test1
  test1.fetch-timeout: "5m"
  test1.default-url: ""
  test1.default-revision: "stable"
  test1.scm-type: "github"
  test1.server-url: "api.internal-github.com"
  test1.api-token-secret-name: "test1-secret"
  test1.api-token-secret-key: "token"
  test1.api-token-secret-namespace: "test1"
  test1.default-org: "tektoncd"

  # configuration 3, will be used if configKey param passed with value test2
  test2.fetch-timeout: "10m"
  test2.default-url: ""
  test2.default-revision: "stable"
  test2.scm-type: "gitlab"
  test2.server-url: "api.internal-gitlab.com"
  test2.api-token-secret-name: "test2-secret"
  test2.api-token-secret-key: "pat"
  test2.api-token-secret-namespace: "test2"
  test2.default-org: "tektoncd-infra"
```

#### Task Resolution

A specific configurations from the configMap can be selected by passing the parameter `configKey` with the value 
matching one of the configuration keys used in the configMap.

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
    - name: configKey
      value: test1
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
    - name: configKey
      value: test2
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
  labels:
    resolution.tekton.dev/type: git
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
