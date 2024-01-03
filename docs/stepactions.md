<!--
---
linkTitle: "StepActions"
weight: 201
---
-->

# StepActions

- [Overview](#overview)
- [Configuring a StepAction](#configuring-a-stepaction)
  - [Declaring Parameters](#declaring-parameters)
    - [Passing Params to StepAction](#passing-params-to-stepaction)
  - [Emitting Results](#emitting-results)
    - [Fetching Emitted Results from StepActions](#fetching-emitted-results-from-stepactions)
  - [Declaring WorkingDir](#declaring-workingdir)
  - [Declaring SecurityContext](#declaring-securitycontext)
  - [Declaring VolumeMounts](#declaring-volumemounts)
  - [Referencing a StepAction](#referencing-a-stepaction)
    - [Specifying Remote StepActions](#specifying-remote-stepactions)
- [Known Limitations](#known-limitations)
  - [Cannot pass Step Results between Steps](#cannot-pass-step-results-between-steps)

## Overview
> :seedling: **`StepActions` is an [alpha](additional-configs.md#alpha-features) feature.**
> The `enable-step-actions` feature flag must be set to `"true"` to specify a `StepAction` in a `Step`.

A `StepAction` is the reusable and scriptable unit of work that is performed by a `Step`.

A `Step` is not reusable, the work it performs is reusable and referenceable. `Steps` are in-lined in the `Task` definition and either perform work directly or perform a `StepAction`. A `StepAction` cannot be run stand-alone (unlike a `TaskRun` or a `PipelineRun`). It has to be referenced by a `Step`. Another way to think about this is that a `Step` is not composed of `StepActions` (unlike a `Task` being composed of `Steps` and `Sidecars`). Instead, a `Step` is an actionable component, meaning that it has the ability to refer to a `StepAction`. The author of the `StepAction` must be able to compose a `Step` using a `StepAction` and provide all the necessary context (or orchestration) to it.


## Configuring a `StepAction`

A `StepAction` definition supports the following fields:

- Required
  - [`apiVersion`][kubernetes-overview] - Specifies the API version. For example,
    `tekton.dev/v1alpha1`.
  - [`kind`][kubernetes-overview] - Identifies this resource object as a `StepAction` object.
  - [`metadata`][kubernetes-overview] - Specifies metadata that uniquely identifies the
    `StepAction` resource object. For example, a `name`.
  - [`spec`][kubernetes-overview] - Specifies the configuration information for this `StepAction` resource object.
  - `image` - Specifies the image to use for the `Step`.
    - The container image must abide by the [container contract](./container-contract.md).
- Optional
  - `command`
    - cannot be used at the same time as using `script`.
  - `args`
  - `script`
    - cannot be used at the same time as using `command`.
  - `env`
  - [`params`](#declaring-parameters)
  - [`results`](#emitting-results)
  - [`workingDir`](#declaring-workingdir)
  - [`securityContext`](#declaring-securitycontext)
  - [`volumeMounts`](#declaring-volumemounts)

[kubernetes-overview]:
  https://kubernetes.io/docs/concepts/overview/working-with-objects/kubernetes-objects/#required-fields

The example below demonstrates the use of most of the above-mentioned fields:

```yaml
apiVersion: tekton.dev/v1alpha1
kind: StepAction
metadata:
  name: example-stepaction-name
spec:
  env:
    - name: HOME
      value: /home
  image: ubuntu
  command: ["ls"]
  args: ["-lh"]
```

### Declaring Parameters

Like with `Tasks`, a `StepAction` must declare all the parameters that it uses. The same rules for `Parameter` [name](./tasks.md/#parameter-name), [type](./tasks.md/#parameter-type) (including [object](./tasks.md/#object-type), [array](./tasks.md/#array-type) and [string](./tasks.md/#string-type)) apply as when declaring them in `Tasks`. A `StepAction` can also provide [default value](./tasks.md/#default-value) to a `Parameter`.

 `Parameters` are passed to the `StepAction` from its corresponding `Step` referencing it.

```yaml
apiVersion: tekton.dev/v1alpha1
kind: StepAction
metadata:
  name: stepaction-using-params
spec:
  params:
    - name: gitrepo
      type: object
      properties:
        url:
          type: string
        commit:
          type: string
    - name: flags
      type: array
    - name: outputPath
      type: string
      default: "/workspace"
  image: some-git-image
  args: [
    "-url=$(params.gitrepo.url)",
    "-revision=$(params.gitrepo.commit)",
    "-output=$(params.outputPath)",
    "$(params.flags[*])",
  ]
```

> :seedling: **`params` cannot be directly used in a `script` in `StepActions`.**
> Directly substituting `params` in `scripts` makes the workload prone to shell attacks. Therefore, we do not allow direct usage of `params` in `scripts` in `StepActions`. Instead, rely on passing `params` to `env` variables and reference them in `scripts`. We cannot do the same for `inlined-steps` because it breaks `v1 API` compatibility for existing users.

#### Passing Params to StepAction

A `StepAction` may require [params](#declaring-parameters). In this case, a `Task` needs to ensure that the `StepAction` has access to all the required `params`.
When referencing a `StepAction`, a `Step` can also provide it with `params`, just like how a `TaskRun` provides params to the underlying `Task`.

```yaml
apiVersion: tekton.dev/v1
kind: Task
metadata:
  name: step-action
spec:
  params:
    - name: param-for-step-action
      description: "this is a param that the step action needs."
  steps:
    - name: action-runner
      ref:
        name: step-action
      params:
        - name: step-action-param
          value: $(params.param-for-step-action)
```

**Note:** If a `Step` declares `params` for an `inlined Step`, it will also lead to a validation error. This is because an `inlined Step` gets its `params` from the `TaskRun`.

### Emitting Results

A `StepAction` also declares the results that it will emit.

```yaml
apiVersion: tekton.dev/v1alpha1
kind: StepAction
metadata:
  name: stepaction-declaring-results
spec:
  results:
    - name: current-date-unix-timestamp
      description: The current date in unix timestamp format
    - name: current-date-human-readable
      description: The current date in human readable format
  image: bash:latest
  script: |
    #!/usr/bin/env bash
    date +%s | tee $(results.current-date-unix-timestamp.path)
    date | tee $(results.current-date-human-readable.path)
```

It is possible that a `StepAction` with `Results` is used multiple times in the same `Task` or multiple `StepActions` in the same `Task` produce `Results` with the same name. Resolving the `Result` names becomes critical otherwise there could be unexpected outcomes. The `Task` needs to be able to resolve these `Result` names clashes by mapping it to a different `Result` name. For this reason, we introduce the capability to store results on a `Step` level.

`StepActions` can also emit `Results` to `$(step.results.<resultName>.path)`.

```yaml
apiVersion: tekton.dev/v1alpha1
kind: StepAction
metadata:
  name: stepaction-declaring-results
spec:
  results:
    - name: current-date-unix-timestamp
      description: The current date in unix timestamp format
    - name: current-date-human-readable
      description: The current date in human readable format
  image: bash:latest
  script: |
    #!/usr/bin/env bash
    date +%s | tee $(step.results.current-date-unix-timestamp.path)
    date | tee $(step.results.current-date-human-readable.path)
```

`Results` from the above `StepAction` can be [fetched by the `Task`](#fetching-emitted-results-from-stepactions) or in [another `Step/StepAction`](#passing-step-results-between-steps) via `$(steps.<stepName>.results.<resultName>)`.

#### Fetching Emitted Results from StepActions

A `Task` can fetch `Results` produced by the `StepActions` (i.e. only `Results` emitted to `$(step.results.<resultName>.path)`, NOT `$(results.<resultName>.path)`) using variable replacement syntax. We introduce a field to [`Task Results`](./tasks.md#emitting-results) called `Value` whose value can be set to the variable `$(steps.<stepName>.results.<resultName>)`.

```yaml
apiVersion: tekton.dev/v1
kind: Task
metadata:
  name: task-fetching-results
spec:
  results:
    - name: git-url
      description: "url of git repo"
      value: $(steps.git-clone.results.url)
    - name: registry-url
      description: "url of docker registry"
      value: $(steps.kaniko.results.url)
  steps:
    - name: git-clone
      ref:
        name: clone-step-action
    - name: kaniko
      ref:
        name: kaniko-step-action
```

`Results` emitted to `$(step.results.<resultName>.path)` are not automatically available as `TaskRun Results`. The `Task` must explicitly fetch it from the underlying `Step` referencing `StepActions`.

For example, lets assume that in the previous example, the "kaniko" `StepAction` also produced a `Result` named "digest". In that case, the `Task` should also fetch the "digest" from "kaniko" `Step`.

```yaml
apiVersion: tekton.dev/v1
kind: Task
metadata:
  name: task-fetching-results
spec:
  results:
    - name: git-url
      description: "url of git repo"
      value: $(steps.git-clone.results.url)
    - name: registry-url
      description: "url of docker registry"
      value: $(steps.kaniko.results.url)
    - name: digest
      description: "digest of the image"
      value: $(steps.kaniko.results.digest)
  steps:
    - name: git-clone
      ref:
        name: clone-step-action
    - name: kaniko
      ref:
        name: kaniko-step-action
```

#### Passing Results between Steps

`StepResults` (i.e. results written to `$(step.results.<result-name>.path)`, NOT `$(results.<result-name>.path)`) can be shared with following steps via replacement variable `$(steps.<step-name>.results.<result-name>)`.

Pipeline supports two new types of results and parameters: array `[]string` and object `map[string]string`.

| Result Type | Parameter Type | Specification                                    | `enable-api-fields` |
|-------------|----------------|--------------------------------------------------|---------------------|
| string      | string         | `$(steps.<step-name>.results.<result-name>)`     | stable              |
| array       | array          | `$(steps.<step-name>.results.<result-name>[*])`  | alpha or beta       |
| array       | string         | `$(steps.<step-name>.results.<result-name>[i])`  | alpha or beta       |
| object      | string         | `$(tasks.<task-name>.results.<result-name>.key)` | alpha or beta       |

**Note:** Whole Array `Results` (using star notation) cannot be referred in `script` and `env`.

The example below shows how you could pass `step results` from a `step` into following steps, in this case, into a `StepAction`.

```yaml
apiVersion: tekton.dev/v1
kind: TaskRun
metadata:
  name: step-action-run
spec:
  TaskSpec:
    steps:
      - name: inline-step
        results:
          - name: result1
            type: array
          - name: result2
            type: string
          - name: result3
            type: object
            properties:
              IMAGE_URL:
                type: string
              IMAGE_DIGEST:
                type: string
        image: alpine
        script: |
          echo -n "[\"image1\", \"image2\", \"image3\"]" | tee $(step.results.result1.path)
          echo -n "foo" | tee $(step.results.result2.path)
          echo -n "{\"IMAGE_URL\":\"ar.com\", \"IMAGE_DIGEST\":\"sha234\"}" | tee $(step.results.result3.path)
      - name: action-runner
        ref:
          name: step-action
        params:
          - name: param1
            value: $(steps.inline-step.results.result1[*])
          - name: param2
            value: $(steps.inline-step.results.result2)
          - name: param3
            value: $(steps.inline-step.results.result3[*])
```

**Note:** `Step Results` can only be referenced in a `Step's/StepAction's` `env`, `command` and `args`. Referencing in any other field will throw an error.

### Declaring WorkingDir

You can declare `workingDir` in a `StepAction`:

```yaml
apiVersion: tekton.dev/v1alpha1
kind: StepAction
metadata:
  name: example-stepaction-name
spec:
  image:  gcr.io/tekton-releases/github.com/tektoncd/pipeline/cmd/git-init:latest
  workingDir: /workspace
  script: |
    # clone the repo
    ...
```

The `Task` using the `StepAction` has more context about how the `Steps` have been orchestrated. As such, the `Task` should be able to update the `workingDir` of the `StepAction` so that the `StepAction` is executed from the correct location.
The `StepAction` can parametrize the `workingDir` and work relative to it. This way, the `Task` does not really need control over the workingDir, it just needs to pass the path as a parameter.

```yaml
apiVersion: tekton.dev/v1alpha1
kind: StepAction
metadata:
  name: example-stepaction-name
spec:
  image: ubuntu
  params:
    - name: source
      description: "The path to the source code."
  workingDir: $(params.source)
```


### Declaring SecurityContext

You can declare `securityContext` in a `StepAction`:

```yaml
apiVersion: tekton.dev/v1alpha1
kind: StepAction
metadata:
  name: example-stepaction-name
spec:
  image:  gcr.io/tekton-releases/github.com/tektoncd/pipeline/cmd/git-init:latest
  securityContext:
      runAsUser: 0
  script: |
    # clone the repo
    ...
```

Note that the `securityContext` from `StepAction` will overwrite the `securityContext` from [`TaskRun`](./taskruns.md/#example-of-running-step-containers-as-a-non-root-user).

### Declaring VolumeMounts

You can define `VolumeMounts` in `StepActions`. The `name` of the `VolumeMount` MUST be a single reference to a string `Parameter`. For example, `$(params.registryConfig)` is valid while `$(params.registryConfig)-foo` and `"unparametrized-name"` are invalid. This is to ensure reusability of `StepActions` such that `Task` authors have control of which `Volumes` they bind to the `VolumeMounts`.

```yaml
apiVersion: tekton.dev/v1alpha1
kind: StepAction
metadata:
  name: myStep
spec:
  params:
    - name: registryConfig
    - name: otherConfig
  volumeMounts:
    - name: $(params.registryConfig)
      mountPath: /registry-config
    - name: $(params.otherConfig)
      mountPath: /other-config
  image: ...
  script: ...
```


### Referencing a StepAction

`StepActions` can be referenced from the `Step` using the `ref` field, as follows:

```yaml
apiVersion: tekton.dev/v1
kind: TaskRun
metadata:
  name: step-action-run
spec:
  taskSpec:
    steps:
      - name: action-runner
        ref:
          name: step-action
```

Upon resolution and execution of the `TaskRun`, the `Status` will look something like:

```yaml
status:
  completionTime: "2023-10-24T20:28:42Z"
  conditions:
  - lastTransitionTime: "2023-10-24T20:28:42Z"
    message: All Steps have completed executing
    reason: Succeeded
    status: "True"
    type: Succeeded
  podName: step-action-run-pod
  provenance:
    featureFlags:
      EnableStepActions: true
      ...
  startTime: "2023-10-24T20:28:32Z"
  steps:
  - container: step-action-runner
    imageID: docker.io/library/alpine@sha256:eece025e432126ce23f223450a0326fbebde39cdf496a85d8c016293fc851978
    name: action-runner
    terminated:
      containerID: containerd://46a836588967202c05b594696077b147a0eb0621976534765478925bb7ce57f6
      exitCode: 0
      finishedAt: "2023-10-24T20:28:42Z"
      reason: Completed
      startedAt: "2023-10-24T20:28:42Z"
  taskSpec:
    steps:
    - computeResources: {}
      image: alpine
      name: action-runner
```

If a `Step` is referencing a `StepAction`, it cannot contain the fields supported by `StepActions`. This includes:
- `image`
- `command`
- `args`
- `script`
- `env`
- `volumeMounts`

Using any of the above fields and referencing a `StepAction` in the same `Step` is not allowed and will cause a validation error.

```yaml
# This is not allowed and will result in a validation error
# because the image is expected to be provided by the StepAction
# and not inlined.
apiVersion: tekton.dev/v1
kind: TaskRun
metadata:
  name: step-action-run
spec:
  taskSpec:
    steps:
      - name: action-runner
        ref:
          name: step-action
        image: ubuntu
```
Executing the above `TaskRun` will result in an error that looks like:

```
Error from server (BadRequest): error when creating "STDIN": admission webhook "validation.webhook.pipeline.tekton.dev" denied the request: validation failed: image cannot be used with Ref: spec.taskSpec.steps[0].image
```

When a `Step` is referencing a `StepAction`, it can contain the following fields:
- `computeResources`
- `workspaces` (Isolated workspaces)
- `volumeDevices`
- `imagePullPolicy`
- `onError`
- `stdoutConfig`
- `stderrConfig`
- `securityContext`
- `envFrom`
- `timeout`
- `ref`
- `params`

Using any of the above fields and referencing a `StepAction` is allowed and will not cause an error. For example, the `TaskRun` below will execute without any errors:

```yaml
apiVersion: tekton.dev/v1
kind: TaskRun
metadata:
  name: step-action-run
spec:
  taskSpec:
    steps:
      - name: action-runner
        ref:
          name: step-action
        params:
          - name: step-action-param
            value: hello
        computeResources:
          requests:
            memory: 1Gi
            cpu: 500m
        timeout: 1h
        onError: continue
```

#### Specifying Remote StepActions

A `ref` field may specify a `StepAction` in a remote location such as git.
Support for specific types of remote will depend on the `Resolvers` your
cluster's operator has installed. For more information including a tutorial, please check [resolution docs](resolution.md). The below example demonstrates referencing a `StepAction` in git:

```yaml
apiVersion: tekton.dev/v1
kind: TaskRun
metadata:
  generateName: step-action-run-
spec:
  taskSpec:
    steps:
      - name: action-runner
        ref:
          resolver: git
          params:
            - name: url
              value: https://github.com/repo/repo.git
            - name: revision
              value: main
            - name: pathInRepo
              value: remote_step.yaml
```

The default resolver type can be configured by the `default-resolver-type` field in the `config-defaults` ConfigMap (`alpha` feature). See [additional-configs.md](./additional-configs.md) for details.
