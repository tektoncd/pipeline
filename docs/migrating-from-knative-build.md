# Migrating from [Knative Build](https://github.com/knative/build)

This doc describes a process for users who are familiar with Knative `Build` and
`BuildTemplate` resources to migrate to Tekton `TaskRuns` and `Tasks`, respectively.

Tekton's resources are heavily influenced by Knative's Build-related resources,
with some additional features that enable them to be chained together inside a
Pipeline, and provide additional flexibility and reusability.

| **Knative**          | **Tekton**  |
|----------------------|-------------|
| Build                | TaskRun     |
| BuildTemplate        | Task        |
| ClusterBuildTemplate | ClusterTask |

## Important differences

* All `steps` must have a `name`.

* BuildTemplate
  [`parameters`](https://github.com/tektoncd/pipeline/blob/master/docs/tasks.md#parameters)
  are moved inside Task's `input.params` field, and parameter placeholder
  strings (e.g., `${FOO}`) must be specified like `$(input.parameters.FOO)`
  (see [variable substitution](tasks.md#variable-substitution)).

* Tasks must specify
  [`input.resources`](https://github.com/tektoncd/pipeline/blob/master/docs/tasks.md#input-resources)
  if they need to operate on a resource (e.g., source from a Git repo).
  BuildTemplates did not specify input resource requirements, and just assumed
  whatever source was available.

* Input resources must specify a `name`, which is the directory within
  `/workspace` where the resource's source will be placed. So if you specify a
  `git`-type resource named `foo`, the source will be cloned into
  `/workspace/foo` -- make sure to either set `workingDir: /workspace/foo` in
  the Task's `steps`, or at least be aware that source will not be cloned into
  `/workspace` as was the case with Knative Builds. See [Controlling where
  resources are
  mounted](https://github.com/tektoncd/pipeline/blob/master/docs/tasks.md#controlling-where-resources-are-mounted)
  for more information.

* TaskRuns which specify a PipelineResource to satisfy a Task's `input.resources`
  can do so either by referencing an existing PipelineResource resource in its
  `resourceRef`, or by fully specifying the resource in its
  [`resourceSpec`](https://github.com/tektoncd/pipeline/blob/master/docs/taskruns.md#providing-resources).

* Because of how steps are serialized without relying on init containers, steps
  should specify a `command` instead of
  [`entrypoint`](https://github.com/tektoncd/pipeline/blob/master/docs/container-contract.md#entrypoint)
  and `args`. If the image's `entrypoint` isn't defined, Tekton will attempt
  to determine the image's entrypoint. This field can be a list of strings,
  so instead of specifying `args: ['foo', 'bar']` and assuming the image's
  entrypoint (e.g., `/bin/entrypoint`) as before, you can specify
  `command: ['/bin/entrypoint', 'foo, bar']`.

## Example

### BuildTemplate -> Task

Given this example BuildTemplate which runs `go test`*:

```
apiVersion: build.knative.dev/v1alpha1
kind: BuildTemplate
metadata:
  name: go-test
spec:
  parameters:
  - name: TARGET
    description: The Go target to test
    default: ./...

  steps:
  - image: golang
    args: ['go', 'test', '${TARGET}']
```

*This is just an example BuildTemplate, for illustration purposes only.

This is the equivalent Task:

```
apiVersion: tekton.dev/v1alpha1
kind: Task
metadata:
  name: go-test
spec:
  inputs:
    params:
    - name: TARGET
      description: The Go target to test
      default: ./...

    # The Task must operate on some source, e.g., in a Git repo.
    resources:
    - name: source
      type: git

  steps:
  - name: go-test  # <-- the step must specify a name.
    image: golang
    workingDir: /workspace/source  # <-- set workingdir
    command: ['go', 'test', '$(inputs.params.TARGET)']  # <-- specify inputs.params.TARGET
```

### Build -> TaskRun

Given this example Build which instantiates and runs the above
BuildTemplate:

```
apiVersion: build.knative.dev/v1alpha1
kind: Build
metadata:
  name: go-test
spec:
  source:
    git:
      url: https://github.com/my-user/my-repo
      revision: master
  template:
    name: go-test
    arguments:
    - name: TARGET
      value: ./path/to/test/...
```

This is the equivalent TaskRun:

```
apiVersion: tekton.dev/v1alpha1
kind: TaskRun
metadata:
  name: example-run
spec:
  taskRef:
    name: go-test
  inputs:
    params:
    - name: TARGET
      value: ./path/to/test/...
    resources:
    - name: source
      resourceSpec:
        type: git
        params:
        - name: url
          value: https://github.com/my-user/my-repo
```
