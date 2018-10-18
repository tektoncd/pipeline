## Task Parameters

Tasks can declare input parameters that must be supplied to the task during a TaskRun.
Some example use-cases of this include:

* A task that needs to know what compilation flags to use when building an application.
* A task that needs to know what to name a built artifact.
* A task that supports several different strategies, and leaves the choice up to the other.

### Usage

The following example shows how Tasks can be parameterized, and these parameters can be passed to the `Task` from a `TaskRun`.

Input parameters in the form of `${inputs.params.foo}` are replaced inside of the buildSpec.

The following `Task` declares an input parameter called 'flags', and uses it in the `buildSpec.steps.args` list.

```yaml
apiVersion: pipeline.knative.dev/v1alpha1
kind: Task
metadata:
  name: task-with-parameters
spec:
    inputs:
        params:
        - name: flags
          value: string
    buildSpec:
        steps:
        - name: build
          image: my-builder
          args: ['build', '--flags=${inputs.params.flags}']
```

The following `TaskRun` supplies a value for `flags`:

```yaml
apiVersion: pipeline.knative.dev/v1alpha1
kind: TaskRun
metadata:
  name: run-with-parameters
spec:
    taskRef: task-with-parameters
    inputs:
        params:
        - name: 'flags'
          value: 'foo=bar,baz=bat'
```
