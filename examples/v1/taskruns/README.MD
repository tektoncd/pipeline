# Tekton TaskRun Examples

This directory contains examples of Tekton TaskRun resources. A task is a collection of steps in order. Tekton runs a task in the form of a Kubernetes pod, where each step becomes a running container in the pod. TaskRun allows you to instantiate and execute a Task with specific inputs and execution parameters. TaskRun executes the Steps in the Task in the order they are specified until all Steps have executed successfully or a failure occurs.


## Table of Contents

- [Prerequisites](#prerequisites)
- [Understanding `metadata.generateName` vs `metadata.name`](#understanding-metadatageneratename-vs-metadataname)
  - [Using `generateName`](#using-generatename)
  - [Using `name` (Most Examples)](#using-name-most-examples)
- [Monitoring TaskRuns](#monitoring-taskruns)
- [Basic Examples](#basic-examples)
  - [unnamed-steps.yaml](#unnamed-stepsyaml)
  - [steps-run-in-order.yaml](#steps-run-in-orderyaml)
  - [task-result.yaml](#task-resultyaml)
  - [step-script.yaml](#step-scriptyaml)
  - [using_context_variables.yaml](#using_context_variablesyaml)
- [Workspaces](#workspaces)
  - [workspace.yaml](#workspaceyaml)
  - [workspace-readonly.yaml](#workspace-readonlyyaml)
  - [workspace-with-volumeClaimTemplate.yaml](#workspace-with-volumeclaimtemplateyaml)
  - [propagating_workspaces.yaml](#propagating_workspacesyaml)
  - [optional-workspaces.yaml](#optional-workspacesyaml)
  - [workingdir.yaml](#workingdiryaml)
- [Parameters and Results](#parameters-and-results)
  - [default_task_params.yaml](#default_task_paramsyaml)
  - [object-param-result.yaml](#object-param-resultyaml)
  - [array-default.yaml](#array-defaultyaml)
  - [propagating_params_implicit.yaml](#propagating_params_implicityaml)
- [StepActions](#stepactions)
  - [stepaction.yaml](#stepactionyaml)
  - [stepactions-steptemplate.yaml](#stepactions-steptemplateyaml)
  - [stepaction-params.yaml](#stepaction-paramsyaml)
  - [stepaction-results.yaml](#stepaction-resultsyaml)
  - [stepaction-passing-results.yaml](#stepaction-passing-resultsyaml)
- [Sidecars](#sidecars)
  - [dind-sidecar.yaml](#dind-sidecaryaml)
  - [sidecar-ready.yaml](#sidecar-readyyaml)
  - [sidecar-ready-script.yaml](#sidecar-ready-scriptyaml)
  - [sidecar-interp.yaml](#sidecar-interpyaml)
  - [workspace-in-sidecar.yaml](#workspace-in-sidecaryaml)
- [Error Handling](#error-handling)
  - [ignore-step-error.yaml](#ignore-step-erroryaml)
- [Security and Authentication](#security-and-authentication)
  - [authenticating-git-commands.yaml](#authenticating-git-commandsyaml)
  - [creds-init-only-mounts-provided-credentials.yaml](#creds-init-only-mounts-provided-credentialsyaml)
  - [secret-env.yaml](#secret-envyaml)
  - [secret-volume.yaml](#secret-volumeyaml)
  - [secret-volume-params.yaml](#secret-volume-paramsyaml)
  - [run-steps-as-non-root.yaml](#run-steps-as-non-rootyaml)
- [Configuration and Customization](#configuration-and-customization)
  - [custom-env.yaml](#custom-envyaml)
  - [custom-volume.yaml](#custom-volumeyaml)
  - [configmap.yaml](#configmapyaml)
  - [home-volume.yaml](#home-volumeyaml)
  - [home-is-set.yaml](#home-is-setyaml)
  - [template-volume.yaml](#template-volumeyaml)
  - [task-volume-args.yaml](#task-volume-argsyaml)
- [Step Templates](#step-templates)
  - [steptemplate-env-merge.yaml](#steptemplate-env-mergeyaml)
  - [steptemplate-variable-interop.yaml](#steptemplate-variable-interopyaml)
  - [stepactions-steptemplate.yaml](#stepactions-steptemplateyaml-1)
- [Advanced Features](#advanced-features)
  - [step-by-digest.yaml](#step-by-digestyaml)
  - [entrypoint-resolution.yaml](#entrypoint-resolutionyaml)
  - [readonly-internal-dir.yaml](#readonly-internal-diryaml)
- [Additional Resources](#additional-resources)


## Prerequisites

Before running these examples, ensure you have:

1. **Kubernetes Cluster** (v1.28 or later)
   Follow the instructions to [setup a Kubernetes Cluster](../../../DEVELOPMENT.md#setup-a-kubernetes-cluster). For Local Kubernetes Cluster setup, you can use tools such as  [kind](https://kind.sigs.k8s.io/), [minikube](https://minikube.sigs.k8s.io/), or [Docker Desktop](https://docs.docker.com/desktop/). The managed Kubernetes services offered by the cloud providers can also be used.

2. **kubectl** configured
   ```bash
   kubectl version --client
   kubectl cluster-info
   ```

3. **Tekton Pipelines** installed on your cluster
   Follow the instructions to [install Tekton Pipelines](https://tekton.dev/docs/installation/pipelines/)
   ```bash

    # Monitor the installation:
    kubectl get pods --namespace tekton-pipelines --watch
   ```
   When all components show 1/1 under the READY column, the installation is complete. Hit Ctrl + C to stop monitoring.
   

4. **Jq** for parsing TaskRun results (Optional)


## Understanding `metadata.generateName` vs `metadata.name`

TaskRuns can be created with either `generateName` or `name` in their metadata, and this choice affects how you create and manage them.

### Using `generateName`

**YAML structure:**
```yaml
apiVersion: tekton.dev/v1
kind: TaskRun
metadata:
  generateName: my-taskrun-  # No fixed name!
spec:
  # ... task specification
```

- Creates a **new instance every time** with a unique name (e.g., `my-taskrun-abc123`, `my-taskrun-xyz789`)
- Kubernetes automatically appends a random suffix to the base name
- Allows running the same task multiple times without name conflicts

```bash
# MUST use kubectl create (generates new instance)
kubectl create -f my-taskrun.yaml
# Output: taskrun.tekton.dev/my-taskrun-abc123 created

kubectl create -f my-taskrun.yaml
# Output: taskrun.tekton.dev/my-taskrun-xyz789 created (different name!)

# CANNOT use kubectl apply (requires fixed name)
kubectl apply -f my-taskrun.yaml
# Error: error: from my-taskrun.yaml: cannot use generate name with apply
error: from my-taskrun-: cannot use generate name with apply
```


### Using `name` (Most Examples)

**YAML structure:**
```yaml
apiVersion: tekton.dev/v1
kind: TaskRun
metadata:
  name: my-taskrun  # fixed name!
spec:
  # ... task specification
```

- Always creates the TaskRun resource with the **same name** (`my-taskrun`)
- First execution creates the resource
- You can use either `kubectl create` or `kubectl apply` for the first time to create a new instance
- Once `my-taskrun` already exists:
  - `kubectl create` will fail with "already exists" error
  - `kubectl apply` will update/replace the existing resource
- Only one instance can exist at a time with this name
- To run again with `create`, you must delete the existing TaskRun first

```bash
# First time - both commands work
kubectl create -f my-taskrun.yaml
# Output: taskrun.tekton.dev/my-taskrun created

# OR
kubectl apply -f my-taskrun.yaml
# Output: taskrun.tekton.dev/my-taskrun created

# Second time - create fails, apply works
kubectl create -f my-taskrun.yaml
# Error: taskrun.tekton.dev "my-taskrun" already exists

kubectl apply -f my-taskrun.yaml
# Output: taskrun.tekton.dev/my-taskrun configured
```

## Monitoring TaskRuns

The command

```bash
kubectl get taskrun my-taskrun -o json
```
is used to retrieve the full details of a specific Tekton TaskRun resource in JSON format from the Kubernetes cluster. Breaking down the command:
**kubectl** is the Kubernetes command-line tool.  
**get** is a subcommand of kubectl that is used to retrieve one or more resources from the Kubernetes cluster.  
**taskrun** is the resource type that we want to retrieve.  
**my-taskrun** is the name of the specific resource that we want to retrieve.  
**-o json** is an option that specifies the output format of the retrieved resource. In this case, we want the output to be in JSON format.
Other formats include
**-o yaml** for YAML format
**-o wide** for a wider output.
**-o jsonpath={.status.conditions[?(@.type=="Succeeded")].status}** is an option that specifies the path to the desired value in the JSON output. In this case, we want to retrieve the value of the "Succeeded" condition from the "status" field of the TaskRun resource.

### Example output

```bash

kubectl create -f examples/v1/taskruns/unnamed-steps.yaml
# taskrun.tekton.dev/unnamed-steps-zbh8v created

kubectl get taskruns unnamed-steps-zbh8v
# NAME                  SUCCEEDED   REASON      STARTTIME   COMPLETIONTIME
# unnamed-steps-zbh8v   True        Succeeded   9m12s       9m4s

kubectl get taskruns unnamed-steps-zbh8v -o json     
```

```json    
{
    "apiVersion": "tekton.dev/v1",
    "kind": "TaskRun",
    "metadata": {
        "annotations": {
            "pipeline.tekton.dev/release": "fcba522"
        },
        "creationTimestamp": "2026-03-16T03:30:27Z",
        "generateName": "unnamed-steps-",
        "generation": 1,
        "labels": {
            "app.kubernetes.io/managed-by": "tekton-pipelines"
        },
        "name": "unnamed-steps-zbh8v",
        "namespace": "default",
        "resourceVersion": "238547",
        "uid": "3e6b79bf-772f-466c-89fa-fc250f22aa41"
    },
    "spec": {
        "serviceAccountName": "default",
        "taskSpec": {
            "steps": [
                {
                    "computeResources": {},
                    "image": "mirror.gcr.io/ubuntu",
                    "name": "",
                    "script": "true"
                },
                {
                    "computeResources": {},
                    "image": "mirror.gcr.io/ubuntu",
                    "name": "",
                    "script": "true"
                },
                {
                    "computeResources": {},
                    "image": "mirror.gcr.io/ubuntu",
                    "name": "",
                    "script": "true"
                }
            ]
        },
        "timeout": "1h0m0s"
    },
    "status": {
        "artifacts": {},
        "completionTime": "2026-03-16T03:30:35Z",
        "conditions": [
            {
                "lastTransitionTime": "2026-03-16T03:30:35Z",
                "message": "All Steps have completed executing",
                "reason": "Succeeded",
                "status": "True",
                "type": "Succeeded"
            }
        ],
        "podName": "unnamed-steps-zbh8v-pod",
        "provenance": {
            "featureFlags": {
                "awaitSidecarReadiness": true,
                "coschedule": "workspaces",
                "enableAPIFields": "beta",
                "enableProvenanceInStatus": true,
                "enforceNonfalsifiability": "none",
                "maxResultSize": 4096,
                "resultExtractionMethod": "termination-message",
                "runningInEnvWithInjectedSidecars": true,
                "verificationNoMatchPolicy": "ignore"
            }
        },
        "startTime": "2026-03-16T03:30:27Z",
        "steps": [
            {
                "container": "step-unnamed-0",
                "imageID": "docker-pullable://mirror.gcr.io/ubuntu@sha256:d1e2e92c075e5ca139d51a140fff46f84315c0fdce203eab2807c7e495eff4f9",
                "name": "unnamed-0",
                "provenance": {},
                "terminated": {
                    "containerID": "docker://8a074eac548598967ec06f4af77aaa43c1898b7c55ba67fd6125034b5963e7c9",
                    "exitCode": 0,
                    "finishedAt": "2026-03-16T03:30:34Z",
                    "reason": "Completed",
                    "startedAt": "2026-03-16T03:30:34Z"
                },
                "terminationReason": "Completed"
            },
            {
                "container": "step-unnamed-1",
                "imageID": "docker-pullable://mirror.gcr.io/ubuntu@sha256:d1e2e92c075e5ca139d51a140fff46f84315c0fdce203eab2807c7e495eff4f9",
                "name": "unnamed-1",
                "provenance": {},
                "terminated": {
                    "containerID": "docker://edf3432c175f840b8c92fce77071bc47b94b6902b45e022da95e32c12b110c35",
                    "exitCode": 0,
                    "finishedAt": "2026-03-16T03:30:34Z",
                    "reason": "Completed",
                    "startedAt": "2026-03-16T03:30:34Z"
                },
                "terminationReason": "Completed"
            },
            {
                "container": "step-unnamed-2",
                "imageID": "docker-pullable://mirror.gcr.io/ubuntu@sha256:d1e2e92c075e5ca139d51a140fff46f84315c0fdce203eab2807c7e495eff4f9",
                "name": "unnamed-2",
                "provenance": {},
                "terminated": {
                    "containerID": "docker://ee756cdd2895a7f664dbc70b0598c6f448e30128f3115ee195ed848121ea6fdc",
                    "exitCode": 0,
                    "finishedAt": "2026-03-16T03:30:35Z",
                    "reason": "Completed",
                    "startedAt": "2026-03-16T03:30:35Z"
                },
                "terminationReason": "Completed"
            }
        ],
        "taskSpec": {
            "steps": [
                {
                    "computeResources": {},
                    "image": "mirror.gcr.io/ubuntu",
                    "name": "",
                    "script": "true"
                },
                {
                    "computeResources": {},
                    "image": "mirror.gcr.io/ubuntu",
                    "name": "",
                    "script": "true"
                },
                {
                    "computeResources": {},
                    "image": "mirror.gcr.io/ubuntu",
                    "name": "",
                    "script": "true"
                }
            ]
        }
    }
}

```

This information helps us see:

**Task Configuration** `.spec` - parameters, workspaces, task references.
**Execution Status** `.status` - whether the task succeeded, start and completion times, step results.
**Metadata** `.metadata` - name, namespace, UID, creation time, etc.


## Basic Examples

### [unnamed-steps.yaml](unnamed-steps.yaml)

In Tekton TaskRuns resources, steps can be unnamed. If you don't provide a name field to a step, Tekton automatically generates names like:

```
unnamed-0
unnamed-1
unnamed-2
```

Multiple unnamed steps execute in order. Execute the example [unnamed-steps.yaml](unnamed-steps.yaml) to see how this works. `image: mirror.gcr.io/ubuntu` tells Tekton to run this step inside an Ubuntu Linux container, and script: 'true' executes the true command (a Unix command that does nothing and always succeeds with exit code 0). This is the simplest possible step - it just proves Tekton can create a container and run a command successfully.

**Create:**
```bash 
# 1. Run the TaskRun
cd pipeline
kubectl create -f examples/v1/taskruns/unnamed-steps.yaml 
# taskrun.tekton.dev/unnamed-steps-wdp6v created

kubectl get taskruns unnamed-steps-wdp6v -o json | jq .

```
**Cleanup:**
Get the taskrun name 

```bash
kubectl get taskruns  
```
Delete the taskrun using command (Replace suffix with actual suffix from above command):

```bash
kubectl delete taskrun unnamed-steps-suffix
```


### [steps-run-in-order.yaml](steps-run-in-order.yaml)
Demonstrates that steps run in order and share the same filesystem. First step waits 3 seconds and creates a file named `foo`, second step reads it

``` bash
# 1. Run the TaskRun
cd pipeline
kubectl create -f examples/v1/taskruns/steps-run-in-order.yaml
# taskrun.tekton.dev/steps-run-in-order-mxcpl created

kubectl get taskruns unnamed-steps-wdp6v -o json | jq .
```

### [task-result.yaml](task-result.yaml)

Executes a simple pipeline task that retrieves the current system date and outputs it in two formats. 
1. Unix timestamp format
2. Human readable date format
This task runs two sequential steps. First it captures the current system date and then stores the output as Task results so that the other tasks in a pipeline can consume them.


### [step-script.yaml](step-script.yaml)
Demonstrates how Script execution works across multiple environments and configurations.
This TaskRun defines a Task Parameter `PARAM` with default value `param-value`. This value can be accessed inside scripts as `'$(params.PARAM)'`.

This TaskRun executes multiple steps sequentially to verify various features:
 * noshebang - This step runs a script without a Shebang (`#!`). This executes the script using container's default shell.
 * bash - Runs a Bash script with environment variables as a user with ID 2000. 
 Security Context `securityContext` runs the container as a non-root user with ID 2000 `runAsUser: 2000`.
 `#!/usr/bin/env bash` specifies Bash interpreter. 
 * place-file - Creates a script file `/workspace/hello` inside `workspace` directory. Read [reserved directories](https://tekton.dev/docs/pipelines/tasks/#reserved-directories) for more details.
 * run-file - Executes the file `/workspace/hello`.
 * Steps node, python & perl demonstrates multi-language support. Each step prints a confirmation message.
 * params-applied step tests parameter substitution
 * Step args-allowed tests argument passing
 * Step dollar-signs-allowed tests whether `$` signs are handled properly
 * Step bash-variable-evaluations - tests Bash variable evaluation using eval and writes the result to file


### [using_context_variables.yaml](using_context_variables.yaml)

This example demonstrates how a Tekton TaskRun can access runtime information using context variables. The first step prints the built-in context variable `$(context.taskRun.uid)`, which is a UUID that uniquely identifies the TaskRun. The second step prints the built-in context variables `$(context.task.name)` and `$(context.taskRun.name)`, which represent the names of the task and the TaskRun, respectively.


## Workspaces

### [workspace.yaml](workspace.yaml)

This example demonstrates how a Tekton TaskRun mounts and uses different workspace types. A `PersistentVolumeClaim`, `ConfigMap`, `Secret` and `emptyDir` volume are defined and attached as workspaces to the task. The task steps verify that these volumes are correctly mounted by writing and reading files from the workspace paths. Additional steps read configuration data from a ConfigMap and sensitive data from a Secret to confirm they are accessible during task execution.

A `PersistentVolumeClaim` requests persistent storage. A `configMap` stores non-sensitive configuration data and a `Secret` stores sensitive data such as tokens/credentials. An `emptyDir` is a temporary directory created when the pods start.


### [workspace-readonly.yaml](workspace-readonly.yaml)
This example demonstrates how a Tekton workspaces can be configured with different access permissions. A `PersistentVolumeClaim` is mounted into two workspaces within a TaskRun - one with normal read-write access and another marked as readonly. The steps verify that writing to the read-only workspace fails while reading succeeds.

### [workspace-with-volumeClaimTemplate.yaml](workspace-with-volumeClaimTemplate.yaml)
This example demonstrates dynamic workspace provisioning using a VolumeClaimTemplate. Instead of requiring a pre-existing PersistentVolumeClaim, the TaskRun automatically creates a new PVC based on the template specification. This is useful for ephemeral workspaces that are created on-demand and can be automatically cleaned up after the TaskRun completes.


### [propagating_workspaces.yaml](propagating_workspaces.yaml)
This example shows workspace propagation from the TaskRun level to the task specification. The workspace is defined at the TaskRun level with an emptyDir volume, and the task can reference it using workspace variables like $(workspaces.tr-workspace.path). This demonstrates how workspaces flow from the TaskRun configuration into the task's execution context.

### [optional-workspaces.yaml](optional-workspaces.yaml)
This example demonstrates optional workspace handling. The task declares two workspaces as optional, but only one is provided in the TaskRun. The step uses the $(workspaces.source-code.bound) variable to check whether each workspace was actually provided, allowing the task to adapt its behavior based on which workspaces are available. This enables flexible task definitions that can work with or without certain workspaces.

### [workingdir.yaml](workingdir.yaml)
This example shows how to set and override the working directory for steps. The first step explicitly sets its working directory to /workspace and verifies it. The second step overrides the working directory to a custom path /a/path/too/far and confirms the change. This demonstrates how each step can have its own working directory independent of other steps.


## Parameters and Results

### [default_task_params.yaml](default_task_params.yaml)
Demonstrates parameters with default values & TaskRef (referencing external Task). 
The `task` `example-default-task-param` defines a parameter `input` with default value `No input provided, but that's okay!`. If no value is provided to the parameter at runtime, the default value will be used. The commented block under `taskRef:` shows how to override the default parameter value defined in the task. With commented block, TaskRun references the Task and executes it without supplying a parameter value, which causes the Task to use the default paramaters.


### [object-param-result.yaml](object-param-result.yaml)
Demonstrates object-type parameters and results in TaskRuns. The task defines an object parameter `gitrepo` of type `object` with two properties: `url` and `commit`. The TaskRun provides only the value ("sha123") for the property `commit`, while the url property uses its default value `("github.example.com")`. The first step validates that both properties are correctly resolved by accessing them using `$(params.gitrepo.url)` and `$(params.gitrepo.commit)`. The second step demonstrates writing an object result by outputting a JSON string containing `IMAGE_URL` and `IMAGE_DIGEST` properties to the result path. This shows object results must be written as valid JSON.


### [array-default.yaml](array-default.yaml)
Demonstrates array parameters with default values. Inside `taskspec`, parameters are explicitly declared and described. The task defines three parameters: `array-to-echo` (array type), `another-array-to-echo` (array type with default values ["foo-default", "bar-default"]), and `string-to-echo` (string type with default "baz"). `spec.params` supplies the values. The TaskRun provides values only for `array-to-echo` (["foo", "bar"]), while the other parameters use their defaults. The step echoes all parameters using array expansion syntax `$(params.array-to-echo[*])` to demonstrate that both provided and default array values are correctly resolved. The expected output is "foo bar foo-default bar-default baz".

### [propagating_params_implicit.yaml](propagating_params_implicit.yaml)

In most examples you saw, the parameters are explicitly defined in the `Task` (`taskspec.params`), while the values are provided by `TaskRun` (`spec.params`). This is the explicit way of propagating parameters.
In this example, the step still uses `$(params.message)` to access the parameter value, but the parameter is not explicitly declared in the `Task` (`taskspec.params`). Instead, it is implicitly propagated from the `TaskRun` (`spec.params`) to the `Task` (`taskspec.params`). This is the implicit way of propagating parameters.


## StepActions

Examples showing StepAction usage:


### [stepaction.yaml](stepaction.yaml)
This example demonstrates the basic usage of StepAction. A StepAction is a reusable, modular component that defines a specific unit of work (script or command) to be executed by a Tekton Task step. Instead of writing the same step logic repeatedly across multiple tasks, you define it once as a StepAction:

```yaml

kind: StepAction
...

```

Then tasks can reference it using:
```yaml
steps:
  - name: action-runner
    ref:
      name: step-action
```

In this example, the StepAction simply prints a message. The TaskRun references the StepAction by StepAction name (`step-action`), and Tekton executes the script defined in the StepAction. This demonstrates the fundamental concept of StepActions: defining reusable step logic that can be shared across multiple tasks without duplication.

### [stepactions-steptemplate.yaml](stepactions-steptemplate.yaml)
This example shows how a StepAction works together with a stepTemplate in a TaskRun. A stepTemplate allows you to define common configuration (like environment variables, volume mounts, or resource limits) that applies to all steps in a task. Anything defined here automatically applies to all steps.

The StepAction defines a simple command:
```yaml
apiVersion: tekton.dev/v1beta1
kind: StepAction
metadata:
  name: step-action
spec:
  image: mirror.gcr.io/alpine
  command: ["env"]

```
The container will run using the Alpine Linux image, and the command will display all environment variables. 

The TaskRun references this StepAction:

```yaml
taskSpec:
  steps:
    - ref:
        name: step-action
```
The TaskRun also defines a stepTemplate with an environment variable `foo=bar`:

```yaml
  stepTemplate:
    env:
      - name: foo
        value: bar
```

When executed, the step inherits both the command from the StepAction (env) and the environment variable from the stepTemplate (foo=bar). The env command will display all environment variables, including the foo variable set by the stepTemplate. This demonstrates how stepTemplate provides a way to inject common configuration into StepActions without modifying the StepAction definition itself.


### [stepaction-params.yaml](stepaction-params.yaml)

This example shows how to use `StepAction` parameters. In this example, the StepAction defines several parameter types with default values. 

**String parameter**
```yaml
    - name: string-param
      default: "a string param"
```
**Array parameter**
```yaml
    - name: array-param
      type: array
      default:
        - an
        - array
        - param
```
**Object parameter**
```yaml
    - name: object-param
      type: object
      properties:
        key1:
          type: string
        key2:
          type: string
        key3:
          type: string
      default:
        key1: "step-action default key1"
        key2: "step-action default key2"
        key3: "step-action default key3"
```
**Enum parameter**
```yaml
    - name: enum-param
      type: object
      properties:
        v1:
          type: string
        v2:
          type: string
        v3:
          type: string
      default:
        v1: "enum1"
        v2: "enum2"
        v3: "enum3"
```

The StepAction exposes parameter values to the script as environment variables.
```yaml

  env:
    - name: arrayparam0
      value: $(params.array-param[0])
```

So inside the script, you can access the parameter values using the environment variables `${arrayparam0}`. However, the order in which Tekton resolves the parameters is as follows:
`TaskSpec defaults overrides StepAction defaults. TaskRun parameters overrides TaskSpec defaults`

This example demonstrates how the StepAction parameters receive values from StepAction defaults, TaskSpec defaults and TaskRun overrides and the script verifies that Tekton resolves them correctly.


### [stepaction-results.yaml](stepaction-results.yaml)

This example demonstrates how StepAction can produce results that are accessible to the Task. A StepAction can define results just like a regular step, making it possible to create reusable components that produce outputs.

The StepAction defines two results - (`result1` and `result2`) and writes values to them using `$(step.results.result1.path)` and `$(step.results.result2.path)`. :

```yaml
  results:
    - name: result1
    - name: result2
  ...

  script: |
    echo "I am a Step Action!!!" >> $(step.results.result1.path)
    echo "I am a hidden step action!!!" >> $(step.results.result2.path)
```

The task then exposes result1 as a task-level result named step-result using the syntax $(steps.action-runner.results.result1). The TaskRun references the StepAction:

```yaml
taskSpec:
  results:
    - name: step-result
      value: $(steps.action-runner.results.result1)
  steps:
    - name: action-runner
      ref:
        name: step-action
```
The task then exposes result1 as a task-level result named step-result using the syntax $(steps.action-runner.results.result1). Note that result2 is produced by the StepAction but not exposed at the task level, demonstrating selective result propagation. This shows how StepAction results can be surfaced at the task level, enabling reusable step logic that produces outputs.


### [stepaction-passing-results.yaml](stepaction-passing-results.yaml)
Demonstrates passing results from one step to a StepAction as parameters. The first step (inline-step) produces three results: an array result (result1), a string result (result2), and an object result (result3). 
```yaml
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
  script: |
    echo -n "[\"image1\", \"image2\", \"image3\"]" | tee $(step.results.result1.path)
    echo -n "foo" | tee $(step.results.result2.path)
    echo -n "{\"IMAGE_URL\":\"ar.com\", \"IMAGE_DIGEST\":\"sha234\"}" | tee $(step.results.result3.path)
```

The second step references a StepAction and passes these results as parameters using the syntax `$(steps.inline-step.results.result1[*])`, `$(steps.inline-step.results.result2)`, and `$(steps.inline-step.results.result3[*])`. 

```yaml
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

The StepAction receives these as parameters and validates their values. This demonstrates inter-step communication where one step's outputs become another step's inputs, enabling complex data flow patterns within a task. The example shows that all result types (array, string, object) can be passed between steps.


## Sidecars

### [dind-sidecar.yaml](dind-sidecar.yaml)
This example demonstrates the Docker-in-Docker (DinD) pattern using a sidecar container. Sidecars are containers that run alongside the main task steps, providing supporting services. In this case, the sidecar runs a Docker daemon that the main step can use to build and run Docker containers.

The sidecar runs a Docker daemon with specific configuration:
```yaml
sidecars:
  - image: mirror.gcr.io/library/docker@sha256:...
    name: server
    args:
      - --storage-driver=vfs
      - --userland-proxy=false
      - --debug
    securityContext:
      privileged: true
    env:
      - name: DOCKER_TLS_CERTDIR
        value: /certs
```
The sidecar requires privileged mode to run the Docker daemon and generates TLS certificates for secure communication. It uses both startupProbe and readinessProbe to ensure the daemon is fully initialized:
```yaml
startupProbe:
  periodSeconds: 1
  failureThreshold: 30
  exec:
    command: ['ls', '/certs/client/ca.pem']
readinessProbe:
  periodSeconds: 2
  exec:
    command: ['docker', 'info']
```

The main step connects to the sidecar's Docker daemon using environment variables:
```yaml
steps:
  - name: client
    env:
      - name: DOCKER_HOST
        value: tcp://localhost:2376
      - name: DOCKER_TLS_VERIFY
        value: '1'
      - name: DOCKER_CERT_PATH
        value: /certs/client
```
Both containers share a volume for TLS certificates. The step then uses standard Docker commands to run containers and build images. This pattern is useful for CI/CD pipelines that need to build container images as part of the task execution.

### [sidecar-ready.yaml](sidecar-ready.yaml)
This example demonstrates how to use readiness and startup probes to ensure a sidecar is fully initialized before steps begin execution. Without proper readiness checks, steps might fail if they try to access sidecar resources before the sidecar is ready.

The sidecar defines both readiness and startup probes:
```yaml
sidecars:
  - name: slow-sidecar
    image: mirror.gcr.io/ubuntu
    command: ['sleep', 'infinity']
    readinessProbe:
      exec:
        command:
          - sh
          - -c
          - sleep 5 && touch /shared/ready
      timeoutSeconds: 10
    startupProbe:
      exec:
        command:
          - sh
          - -c
          - sleep 5 && touch /shared/ready
      timeoutSeconds: 10
```
The probes wait 5 seconds and then create a file at /shared/ready. The readinessProbe is used for traditional sidecar support, while startupProbe is honored when using Kubernetes native sidecar support (where readinessProbe may not be honored).

The step waits for the sidecar to be ready by checking for the file:
```yaml
steps:
  - name: check-ready
    image: mirror.gcr.io/ubuntu
    script: cat /shared/ready
```
Both containers share a volume mounted at /shared. This demonstrates how Tekton ensures steps don't execute until sidecars report as ready, preventing race conditions and ensuring reliable task execution.

### [sidecar-ready-script.yaml](sidecar-ready-script.yaml)
This example shows a simpler approach to sidecar readiness using a script instead of explicit readiness probes. The sidecar uses a script to perform initialization work and signal completion through a shared volume.

The sidecar runs a script that writes a message and then sleeps:
```yaml
sidecars:
  - name: slow-sidecar
    image: mirror.gcr.io/ubuntu
    script: |
      echo "hello from sidecar" > /shared/message
      sleep 2
    volumeMounts:
      - name: shared
        mountPath: /shared
```
The step reads the message written by the sidecar:

```yaml
steps:
  - name: check-ready
    image: mirror.gcr.io/ubuntu
    script: cat /shared/message
    volumeMounts:
      - name: shared
        mountPath: /shared
```
Unlike the previous example, this doesn't use explicit readiness probes. Instead, it relies on the sidecar's script to complete its initialization before the step accesses the shared resource. This is a simpler pattern suitable for sidecars with quick initialization times. The shared volume enables communication between the sidecar and steps through the filesystem.

### [sidecar-interp.yaml](sidecar-interp.yaml)
This example demonstrates parameter interpolation in sidecar configurations. Sidecars can access task parameters just like steps, allowing them to be configured dynamically based on runtime values.

The task defines a parameter with a default value:
```yaml
taskSpec:
  params:
    - name: some-input
      default: foo
```
The sidecar uses this parameter in its command arguments:
```yaml
sidecars:
  - name: value-sidecar
    image: mirror.gcr.io/ubuntu
    command:
      - /bin/bash
    args:
      - -c
      - "echo $(params.some-input) > /shared/value && sleep infinity"
```
The parameter value is interpolated using $(params.some-input) syntax, just like in steps. The sidecar writes the parameter value to a shared volume, and the step reads and verifies it:
```yaml
steps:
  - name: check-value
    image: mirror.gcr.io/ubuntu
    script: |
      #!/bin/bash
      VALUE=$(cat /shared/value)
      [[ $VALUE == 'foo' ]]
```

This demonstrates that sidecars are first-class citizens in Tekton tasks with full access to parameters, enabling dynamic configuration of supporting services based on task inputs. This is useful for scenarios where sidecars need to be configured differently based on the task's runtime parameters.

### [workspace-in-sidecar.yaml](workspace-in-sidecar.yaml)
This example shows how to share a workspace between a step and a sidecar container. A `sidecar` is a helper container that runs alongside the main container. It supports the main container by providing additional functionality such as logging or background services. 

The Step creates a named pipe `FIFO pipe` to send messages to sidecar. The `FIFO pipe` behaves like a communication channel between the step and the sidecar. 
`mkfifo "$(workspaces.signals.path)/channel"`

The Step writes a message to the pipe and then waits for sidecar to write its response.
`read response < "$(workspaces.signals.path)/channel"`

The sidecar listens for messages and responds via the shared workspace. 
`read message < "$(workspaces.signals.path)"/channel` --> listen
`echo "good day" > "$(workspaces.signals.path)"/channel` --> write

The step receives the response from the sidecar and prints it.


## Error Handling

### [ignore-step-error.yaml](ignore-step-error.yaml)

This example demonstrates how to handle step failures gracefully using the `onError: continue` field. By default, when a step fails (exits with a non-zero exit code), the entire TaskRun fails and subsequent steps are not executed. However, sometimes you want to continue execution even when a step fails, such as for cleanup operations or when a failure is expected and should be handled by later steps.

The first step is configured to ignore errors and continue execution:

```yaml
steps:
  - image: mirror.gcr.io/alpine
    onError: continue
    name: exit-with-1
    script: |
      exit 1
```

The `onError: continue` field tells Tekton to continue executing subsequent steps even though this step exits with code 1 (failure). Without this field, the TaskRun would fail immediately and the second step would never execute.

When a step completes (whether successfully or with an error), Tekton records its exit code in multiple locations that subsequent steps can access:

**Using the step exit code path variable:**
```yaml
- name: verify-step-path
  script: |
    exitCode=`cat $(steps.step-exit-with-1.exitCode.path)`
    if [ $exitCode == 1 ]; then
        echo "Yay! the exit code can be accessed using the path variable"
    fi
```

**Using the direct file path:**
```yaml
FILE=/tekton/steps/step-exit-with-1/exitCode
if [ -f "$FILE" ]; then
  echo "$FILE exists."
fi
```

**Using the numeric symlink (step index):**
```yaml
FILE=/tekton/steps/0/exitCode
if [ -f "$FILE" ]; then
  echo "$FILE exists."
fi
```

The second step verifies that:
1. The exit code can be accessed using the `$(steps.step-exit-with-1.exitCode.path)` variable
2. The exit code file exists at `/tekton/steps/step-exit-with-1/exitCode`
3. A symlink exists at `/tekton/steps/0/exitCode` (where 0 is the step index)
4. All methods return the same exit code (1)

This demonstrates how Tekton provides multiple ways to access step exit codes, enabling sophisticated error handling patterns where later steps can inspect and react to failures in earlier steps. This is useful for scenarios like:
- Running cleanup operations regardless of step success
- Implementing custom retry logic
- Collecting diagnostic information after failures
- Conditional execution based on previous step outcomes


## Security and Authentication

### [authenticating-git-commands.yaml](authenticating-git-commands.yaml)

This example demonstrates how to use Tekton's credential helper (creds-init) to authenticate Git commands over SSH without using Git PipelineResources or the git-clone catalog task. This is an advanced example that shows the underlying credential management mechanism.

**Architecture:**
The example uses a sidecar container to run a local Git server with SSH authentication, demonstrating how creds-init credentials work in practice:

**Sidecar Git Server Setup:**
```yaml
sidecars:
  - name: server
    image: mirror.gcr.io/alpine/git:v2.26.2
    securityContext:
      runAsUser: 0
    script: |
      # Generate host key pair
      ssh-keygen -t rsa -f /etc/ssh/ssh_host_rsa_key
      
      # Share public key with steps via shared volume
      HOST_PUBLIC_KEY=$(cat /etc/ssh/ssh_host_rsa_key.pub | awk '{ print $2 }')
      echo "localhost ssh-rsa $HOST_PUBLIC_KEY" > /messages/known_hosts
      
      # Wait for authorized_keys from creds-init
      while [ ! -f /messages/authorized_keys ] ; do
        sleep 1
      done
      
      # Configure SSH and create bare git repo
      mkdir /root/.ssh
      cp /messages/authorized_keys /root/.ssh/
      cd /root/ && mkdir repo && cd repo && git init . --bare
      
      # Start SSH daemon
      /usr/sbin/sshd -E /var/log/sshd
```

**Secret Configuration:**
```yaml
apiVersion: v1
kind: Secret
type: kubernetes.io/ssh-auth
metadata:
  name: ssh-key-for-git
  annotations:
    tekton.dev/git-0: localhost
data:
  ssh-privatekey: <base64-encoded-private-key>
```

The secret is attached to a ServiceAccount that the TaskRun uses:
```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: ssh-key-service-account
secrets:
- name: ssh-key-for-git
```

**Step 1 - Setup (Test-specific):**
Generates the authorized_keys file from the creds-init private key:
```yaml
- name: setup
  script: |
    ssh-keygen -y -f $(credentials.path)/.ssh/id_ssh-key-for-git > /messages/authorized_keys
    # Wait for sshd to be ready
    while [ ! -f /messages/sshd-ready ] ; do
      sleep 1
    done
```

**Step 2 - Git Clone and Push (as root):**
```yaml
- name: git-clone-and-push
  securityContext:
    runAsUser: 0
  script: |
    if [ -d /tekton/home/.ssh ] ; then
      # When disable-home-env-overwrite is "false"
      cp /messages/known_hosts $(credentials.path)/.ssh/
      ln -s $(credentials.path)/.ssh /root/.ssh
    else
      # When disable-home-env-overwrite is "true"
      cp /messages/known_hosts /root/.ssh/known_hosts
    fi
    
    git clone root@localhost:/root/repo ./repo
    cd repo
    git config user.email "example@example.com"
    git config user.name "Example"
    echo "Hello, world!" > README
    git add README
    git commit -m "Test commit!"
    git push origin master
```

**Step 3 - Git Clone and Check (as non-root):**
```yaml
- name: git-clone-and-check
  securityContext:
    runAsUser: 1000
  workingDir: /home/nonroot
  script: |
    if [ -d /tekton/home/.ssh ] ; then
      cp /messages/known_hosts $(credentials.path)/.ssh/
      ln -s $(credentials.path)/.ssh /home/nonroot/.ssh
    else
      cp /messages/known_hosts /home/nonroot/.ssh/known_hosts
    fi
    
    git clone root@localhost:/root/repo ./repo
    cd repo
    cat README | grep "Hello, world!"
```

**Key Concepts:**
- **Creds-init**: Tekton's credential helper automatically mounts SSH credentials from secrets
- **$(credentials.path)**: Variable pointing to where Tekton stores credentials (typically `/tekton/creds`)
- **disable-home-env-overwrite flag**: Controls where credentials are copied (either `/tekton/home` or the user's actual home directory)
- **Non-root execution**: Demonstrates credential handling with different user contexts

**Create and run:**
```bash
# Create the secret and service account first
kubectl apply -f examples/v1/taskruns/authenticating-git-commands.yaml

# Check the TaskRun status
kubectl get taskrun authenticating-git-commands

# View logs
kubectl logs -f authenticating-git-commands-pod
```

This example is particularly useful for understanding how to work with Git credentials in custom tasks without relying on higher-level abstractions.

### [creds-init-only-mounts-provided-credentials.yaml](creds-init-only-mounts-provided-credentials.yaml)

This example verifies that Tekton's credential helper (creds-init) only mounts credentials that are explicitly provided via the ServiceAccount, and doesn't create unnecessary credential files or directories.

**Secret and ServiceAccount Setup:**
```yaml
apiVersion: v1
kind: Secret
type: kubernetes.io/ssh-auth
metadata:
  name: phony-ssh-key
  annotations:
    tekton.dev/git-0: localhost
data:
  ssh-privatekey: SGVsbG8sIHdvcmxkCg==  # "Hello, world!"
  known_hosts: SGVsbG8sIHdvcmxkCg==      # "Hello, world!"
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: phony-ssh-key-service-account
secrets:
- name: phony-ssh-key
```

**Verification Step:**
```yaml
steps:
  - name: check-credentials
    image: mirror.gcr.io/alpine
    script: |
      #!/usr/bin/env ash
      set -xe
      
      # We expect the ssh credential to show up because it's
      # declared on the service account above.
      test -f $HOME/.ssh/id_phony-ssh-key
      
      # We don't expect any docker credentials to appear in the container.
      test ! -d $HOME/.docker
      
      # We don't expect any git basic auth credentials or related
      # configuration to appear in the container.
      test ! -f $HOME/.git-credentials
      test ! -f $HOME/.gitconfig
```

**What this validates:**
- SSH credentials ARE mounted when declared in the ServiceAccount
- Docker credentials are NOT created when not provided
- Git basic auth credentials are NOT created when not provided
- Git configuration files are NOT created when not needed

This demonstrates Tekton's principle of least privilege - only mounting the credentials that are actually needed, reducing the attack surface and avoiding unnecessary file system clutter.

**Create and run:**
```bash
kubectl create -f examples/v1/taskruns/creds-init-only-mounts-provided-credentials.yaml
# taskrun.tekton.dev/creds-init-only-mounts-provided-credentials-abc123 created

# Verify the TaskRun succeeds
kubectl get taskrun creds-init-only-mounts-provided-credentials-abc123
```

### [secret-env.yaml](secret-env.yaml)

This example demonstrates how to inject Kubernetes Secrets as environment variables in TaskRun steps. This is useful for passing sensitive data like passwords, API keys, or tokens to your tasks without hardcoding them.

**Secret Definition:**
```yaml
kind: Secret
apiVersion: v1
metadata:
  name: secret-env-password
stringData:
  ninja: SECRET_PASSWORD
```

**Using Secret as Environment Variable:**
```yaml
steps:
  - name: secret
    image: mirror.gcr.io/ubuntu
    script: |
      #!/usr/bin/env bash
      [[ $SECRET_PASSWORD == SECRET_PASSWORD ]]
    env:
      - name: SECRET_PASSWORD
        valueFrom:
          secretKeyRef:
            name: secret-env-password
            key: ninja
```

**Key Points:**
- The secret key `ninja` contains the value `SECRET_PASSWORD`
- The environment variable `SECRET_PASSWORD` is populated from the secret
- The script verifies the environment variable has the expected value
- Secrets are base64-encoded in Kubernetes but automatically decoded when used

**Create and run:**
```bash
kubectl create -f examples/v1/taskruns/secret-env.yaml
# secret/secret-env-password created
# taskrun.tekton.dev/secret-env created

# Check status
kubectl get taskrun secret-env
```

**Best Practices:**
- Use environment variables for simple key-value secrets
- Never log or print secret values in scripts
- Use descriptive environment variable names
- Consider using secret volumes for files or certificates

### [secret-volume.yaml](secret-volume.yaml)

This example demonstrates how to mount Kubernetes Secrets as volumes in TaskRun steps. This approach is useful when you need to access secrets as files rather than environment variables, such as for configuration files, certificates, or keys.

**Secret Definition:**
```yaml
kind: Secret
apiVersion: v1
metadata:
  name: secret-vol-password
stringData:
  ninja: SECRET_PASSWORD
```

**Mounting Secret as Volume:**
```yaml
taskSpec:
  steps:
    - image: mirror.gcr.io/ubuntu
      script: |
        #!/usr/bin/env bash
        SECRET_PASSWORD=$(cat /var/secret/ninja)
        [[ $SECRET_PASSWORD == SECRET_PASSWORD ]]
      volumeMounts:
        - name: secret-volume
          mountPath: /var/secret
  volumes:
    - name: secret-volume
      secret:
        secretName: secret-vol-password
```

**How it works:**
1. The secret `secret-vol-password` is defined with a key `ninja`
2. A volume named `secret-volume` references the secret
3. The volume is mounted at `/var/secret` in the step container
4. Each key in the secret becomes a file in the mount directory
5. The file `/var/secret/ninja` contains the value `SECRET_PASSWORD`

**Create and run:**
```bash
kubectl create -f examples/v1/taskruns/secret-volume.yaml
# secret/secret-vol-password created
# taskrun.tekton.dev/secret-volume created

# Check status
kubectl get taskrun secret-volume
```

**When to use secret volumes vs environment variables:**
- **Use volumes** for: certificates, key files, configuration files, multi-line secrets
- **Use environment variables** for: simple passwords, API keys, single-value secrets

### [secret-volume-params.yaml](secret-volume-params.yaml)

This example demonstrates how to parameterize secret volume sources, making tasks more flexible and reusable by allowing the secret name to be specified at runtime rather than hardcoded in the task definition.

**Secret Definition:**
```yaml
kind: Secret
apiVersion: v1
metadata:
  name: secret-vol-param-password
stringData:
  ninja: SECRET_PASSWORD
```

**Parameterized Volume Configuration:**
```yaml
taskSpec:
  params:
    - name: SCNAME
      description: Name of secret
      type: string
  steps:
    - image: mirror.gcr.io/ubuntu
      script: |
        #!/usr/bin/env bash
        SECRET_PASSWORD=$(cat /var/secret/ninja)
        [[ $SECRET_PASSWORD == SECRET_PASSWORD ]]
      volumeMounts:
        - name: secret-volume
          mountPath: /var/secret
  volumes:
    - name: secret-volume
      secret:
        secretName: "$(params.SCNAME)"
```

**TaskRun Parameter:**
```yaml
spec:
  params:
    - name: SCNAME
      value: secret-vol-param-password
```

**Benefits of parameterization:**
- **Reusability**: Same task can work with different secrets
- **Flexibility**: Secret name can be changed without modifying the task
- **Environment-specific**: Use different secrets for dev/staging/prod
- **Dynamic configuration**: Secret selection can be based on runtime conditions

**Create and run:**
```bash
kubectl create -f examples/v1/taskruns/secret-volume-params.yaml
# secret/secret-vol-param-password created
# taskrun.tekton.dev/secret-volume-params created

# Check status
kubectl get taskrun secret-volume-params
```

This pattern is particularly useful in CI/CD pipelines where the same task needs to access different secrets based on the environment or context.

### [run-steps-as-non-root.yaml](run-steps-as-non-root.yaml)

This example demonstrates how to run TaskRun steps as non-root users for improved security. Running containers as non-root is a security best practice that limits the potential damage from container breakouts or compromised processes.

**Task Definition with Security Contexts:**
```yaml
apiVersion: tekton.dev/v1
kind: Task
metadata:
  name: show-non-root-steps
spec:
  steps:
    # No securityContext specified, inherits from TaskRun podTemplate
    - name: show-user-1001
      image: mirror.gcr.io/ubuntu
      command: [ps]
      args: ["aux"]
    
    # Explicit securityContext overrides podTemplate
    - name: show-user-2000
      image: mirror.gcr.io/ubuntu
      command: [ps]
      args: ["aux"]
      securityContext:
        runAsUser: 2000
```

**TaskRun with Pod-level Security Context:**
```yaml
apiVersion: tekton.dev/v1
kind: TaskRun
metadata:
  generateName: show-non-root-steps-run-
spec:
  taskRef:
    name: show-non-root-steps
  podTemplate:
    securityContext:
      runAsNonRoot: true
      runAsUser: 1001
```

**Security Context Hierarchy:**
1. **Pod-level** (`podTemplate.securityContext`): Applies to all containers by default
2. **Step-level** (`step.securityContext`): Overrides pod-level settings for specific steps

**In this example:**
- First step runs as user `1001` (inherited from podTemplate)
- Second step runs as user `2000` (explicit override)
- Both steps run as non-root users (`runAsNonRoot: true`)

**Create and run:**
```bash
# Create the task first
kubectl apply -f examples/v1/taskruns/run-steps-as-non-root.yaml
# task.tekton.dev/show-non-root-steps created

# Create a TaskRun instance
kubectl create -f examples/v1/taskruns/run-steps-as-non-root.yaml
# taskrun.tekton.dev/show-non-root-steps-run-abc123 created

# View the process list to see different user IDs
kubectl logs show-non-root-steps-run-abc123-pod -c step-show-user-1001
kubectl logs show-non-root-steps-run-abc123-pod -c step-show-user-2000
```

**Security Benefits:**
- **Principle of least privilege**: Processes run with minimal permissions
- **Container breakout protection**: Limits damage if container is compromised
- **Compliance**: Meets security requirements for many organizations
- **File system protection**: Prevents unauthorized writes to system directories

**Common non-root user IDs:**
- `1000-1999`: Typical range for regular users
- `65534`: Often used for "nobody" user
- Avoid `0`: Root user (highest privileges)

## Configuration and Customization

### [custom-env.yaml](custom-env.yaml)

Demonstrates environment variable usage. `env:` defines a custom environment variable `MY_VAR1` and set it's value to `foo`. 
```yaml
    env:
      - name: MY_VAR1
        value: foo
```
In this example, the variable is hardcoded in the task definition. The custom environment variable is then injected into an Ubuntu container. A simple Bash script is then executed inside the container that checks whether the environment variable `MY_VAR1` has the expected value `foo`. If the value matches, the script succeeds. if not the step fails. This task does not produce explicit Tekton results.

### [custom-volume.yaml](custom-volume.yaml)
This example demonstrates how to use custom volumes to share data between steps with different mount paths. A volume is defined at the task level and can be mounted at different paths in different steps.

The task defines a custom emptyDir volume:
```yaml

volumes:
  - name: custom
    emptyDir: {}
```

The first step mounts this volume and writes a file:

```yaml

- name: write
  image: mirror.gcr.io/ubuntu
  script: |
    #!/usr/bin/env bash
    echo some stuff > /im/a/custom/mount/path/file
  volumeMounts:
    - name: custom
      mountPath: /im/a/custom/mount/path
```

The second step mounts the same volume at a different path and reads the file:

```yaml

- name: read
  image: mirror.gcr.io/ubuntu
  script: |
    #!/usr/bin/env bash
    cat /short/and/stout/file | grep stuff
  volumeMounts:
    - name: custom
      mountPath: /short/and/stout
```
This demonstrates that the same volume can be mounted at different paths in different steps, and data written by one step is accessible to subsequent steps regardless of the mount path used.


### [configmap.yaml](configmap.yaml)

This example shows how to use ConfigMaps in tasks, both as environment variables and as mounted volumes. ConfigMaps are Kubernetes resources that store configuration data as key-value pairs.

First, a ConfigMap is created with test data:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: config-for-testing-configmaps
data:
  test.data: tasks are my jam
```
The task accesses this ConfigMap in two ways:

As an environment variable:
```yaml
env:
  - name: TEST_DATA
    valueFrom:
      configMapKeyRef:
        name: config-for-testing-configmaps
        key: test.data
```

As a mounted volume:
```yaml
volumeMounts:
  - name: config-volume
    mountPath: /config
volumes:
  - name: config-volume
    configMap:
      name: config-for-testing-configmaps
```

The script verifies that both methods provide access to the same data by comparing the environment variable value with the file content at /config/test.data. This demonstrates the flexibility of ConfigMaps for injecting configuration into tasks.


### [home-volume.yaml](home-volume.yaml)

This example demonstrates how Tekton manages the home directory volume (/tekton/home) and how user-defined volume mounts can override it. By default, Tekton provides a shared home directory volume for all steps in a task.

The first two steps use the default home volume:

```yaml
- name: write
  image: mirror.gcr.io/ubuntu
  script: echo some stuff > /tekton/home/stuff
- name: read
  image: mirror.gcr.io/ubuntu
  script: cat /tekton/home/stuff
```

The third step explicitly mounts a different volume to /tekton/home:

```yaml
- name: override-homevol
  image: mirror.gcr.io/ubuntu
  script: |
    #!/usr/bin/env bash
    [[ ! -f /tekton/home/stuff ]]
  volumeMounts:
    - name: empty
      mountPath: /tekton/home
volumes:
  - name: empty
    emptyDir: {}
```

When a user explicitly declares a volume mount that conflicts with Tekton's default home volume, the user's declaration takes precedence. This means the file written in the first step is not accessible in the third step because it uses a different volume. This demonstrates Tekton's volume mount precedence rules.


### [home-is-set.yaml](home-is-set.yaml)
This example verifies that the HOME environment variable is correctly set in task steps. Tekton automatically sets the `HOME` environment variable to `/tekton/home` for all steps.

The step explicitly sets and verifies the `HOME` environment variable:
```yaml
steps:
  - image: mirror.gcr.io/ubuntu
    env:
      - name: HOME
        value: /tekton/home
    script: |
      #!/usr/bin/env bash
      [[ $HOME == /tekton/home ]]
```
This demonstrates that Tekton ensures a consistent home directory location across all steps, which is important for tools and applications that rely on the `HOME` environment variable for storing configuration files, caches, or temporary data.


### [template-volume.yaml](template-volume.yaml)
This example demonstrates how to define volumes using podTemplate instead of the task-level volumes field. The podTemplate allows you to specify pod-level configuration that applies to the entire TaskRun.

The volume is defined in the podTemplate:
```yaml
podTemplate:
  volumes:
    - name: custom
      emptyDir: {}
```

Steps can then reference this volume using volumeMounts:

```yaml
steps:
  - name: write
    image: mirror.gcr.io/ubuntu
    script: |
      #!/usr/bin/env bash
      echo some stuff > /im/a/custom/mount/path/file
    volumeMounts:
      - name: custom
        mountPath: /im/a/custom/mount/path
  - name: read
    image: mirror.gcr.io/ubuntu
    script: |
      #!/usr/bin/env bash
      cat /short/and/stout/file
    volumeMounts:
      - name: custom
        mountPath: /short/and/stout
```
This approach is useful when you need to specify additional pod-level configuration alongside volumes, such as security contexts, node selectors, or tolerations. The volume behaves identically to task-level volumes but is defined at the pod template level.

### [task-volume-args.yaml](task-volume-args.yaml)
This example demonstrates how to use parameters to dynamically configure volume sources. Instead of hardcoding the ConfigMap name in the volume definition, it uses a parameter to make the configuration flexible.

The task defines a parameter for the ConfigMap name:
```yaml
taskSpec:
  params:
    - name: CFGNAME
      description: Name of config map
      type: string
```

The volume definition uses this parameter:
```yaml
volumes:
  - name: custom
    configMap:
      name: "$(params.CFGNAME)"
```

The TaskRun provides the actual ConfigMap name:
```yaml
params:
  - name: CFGNAME
    value: config-for-testing-volume-args
```

This demonstrates parameter substitution in volume configurations, allowing tasks to be reusable with different ConfigMaps, Secrets, or other volume sources. The step mounts and reads from the ConfigMap specified by the parameter, showing how parameterization enables flexible, reusable task definitions.


## Step Templates

### [steptemplate-env-merge.yaml](steptemplate-env-merge.yaml)

This example demonstrates how StepTemplate environment variables are merged with step-specific environment variables. StepTemplate provides a way to define common configuration that applies to all steps, while individual steps can add their own environment variables or override template values.

**StepTemplate Definition:**
```yaml
taskSpec:
  stepTemplate:
    env:
      - name: QUX
        value: original
```

**Parameter-based Environment Variables:**
```yaml
params:
  - name: FOO
    value: foo
  - name: BAR
    value: bar

taskSpec:
  params:
    - name: FOO
      description: FOO variable
    - name: BAR
      description: BAR variable
    - name: FOOBAR
      description: FOOBAR variable
      default: foobar
```

**Steps with Environment Variables:**

**1. Step with parameter-based env var:**
```yaml
- name: foo
  image: mirror.gcr.io/ubuntu
  script: |
    #!/usr/bin/env bash
    [[ $FOO == "foo" ]]
  env:
    - name: FOO
      value: $(params.FOO)
```

**2. Step using default parameter:**
```yaml
- name: foobar
  image: mirror.gcr.io/ubuntu
  script: |
    #!/usr/bin/env bash
    [[ $FOOBAR == "foobar" ]]
  env:
    - name: FOOBAR
      value: $(params.FOOBAR)
```

**3. Step inheriting from stepTemplate:**
```yaml
- name: qux-no-override
  image: mirror.gcr.io/ubuntu
  script: |
    #!/usr/bin/env bash
    [[ $QUX == "original" ]]
  # No env specified, inherits QUX from stepTemplate
```

**4. Step overriding stepTemplate value:**
```yaml
- name: qux-override
  image: mirror.gcr.io/ubuntu
  script: |
    #!/usr/bin/env bash
    [[ $QUX == "override" ]]
  env:
    - name: QUX
      value: override
```

**Environment Variable Merge Rules:**
1. **StepTemplate env vars** are applied to all steps by default
2. **Step-specific env vars** are added to the step's environment
3. **Step-specific env vars with same name** override stepTemplate values
4. **Parameters** can be used in both stepTemplate and step env vars

**Create and run:**
```bash
kubectl create -f examples/v1/taskruns/steptemplate-env-merge.yaml
# taskrun.tekton.dev/steptemplate-env-merge-abc123 created

# Check status
kubectl get taskrun steptemplate-env-merge-abc123
```

**Use Cases:**
- Setting common environment variables (API endpoints, log levels)
- Providing default values that can be overridden per step
- Reducing duplication across multiple steps
- Centralizing configuration management

### [steptemplate-variable-interop.yaml](steptemplate-variable-interop.yaml)

This example demonstrates parameter interpolation in StepTemplate fields, showing how parameters can be used to dynamically configure common step properties like the container image.

**Parameter Definition:**
```yaml
spec:
  params:
    - name: image
      value: ubuntu

taskSpec:
  params:
    - name: image
```

**StepTemplate with Parameter Interpolation:**
```yaml
stepTemplate:
  image: $(params.image)
```

**Step Using Template:**
```yaml
steps:
  - name: foo
    script: |
      echo hello from $(params.image)
```

**How it works:**
1. The TaskRun provides the parameter value `ubuntu`
2. The stepTemplate uses `$(params.image)` to reference this parameter
3. All steps inherit the image from stepTemplate (unless explicitly overridden)
4. The step can also reference the parameter in its script

**Benefits:**
- **Dynamic image selection**: Choose container images at runtime
- **Consistency**: All steps use the same image by default
- **Flexibility**: Easy to switch images for different environments
- **Parameterization**: Makes tasks more reusable

**Create and run:**
```bash
kubectl create -f examples/v1/taskruns/steptemplate-variable-interop.yaml
# taskrun.tekton.dev/steptemplate-variables-abc123 created

# Check logs to see the output
kubectl logs steptemplate-variables-abc123-pod
# hello from ubuntu
```

**Common StepTemplate Fields that Support Parameters:**
- `image`: Container image to use
- `env`: Environment variables
- `volumeMounts`: Volume mount configurations
- `resources`: CPU/memory limits and requests
- `workingDir`: Working directory

**Example with multiple parameterized fields:**
```yaml
stepTemplate:
  image: $(params.image)
  env:
    - name: LOG_LEVEL
      value: $(params.logLevel)
  workingDir: $(params.workDir)
```

This pattern is particularly useful for creating flexible, reusable tasks that can adapt to different execution contexts.

### [stepactions-steptemplate.yaml](stepactions-steptemplate.yaml)

This example was already documented in the [StepActions section](#stepactions-steptempleyaml) above. It demonstrates how StepActions work together with StepTemplates, where the stepTemplate provides common configuration (like environment variables) that automatically applies to all steps, including those referencing StepActions.

**Quick Summary:**
- StepTemplate defines common configuration (e.g., environment variables)
- StepAction defines reusable step logic (e.g., commands)
- When combined, the step inherits both:
  - The command/script from the StepAction
  - The environment variables from the stepTemplate

**Example:**
```yaml
stepTemplate:
  env:
    - name: foo
      value: bar

steps:
  - ref:
      name: step-action  # References a StepAction
    # Inherits env var foo=bar from stepTemplate
```

For complete details, see the [stepactions-steptemplate.yaml](#stepactions-steptempleyaml) documentation in the StepActions section.

## Advanced Features

### [step-by-digest.yaml](step-by-digest.yaml)

This example demonstrates how to reference container images by their cryptographic digest instead of by tag. Using digests ensures that you always get the exact same image, providing immutability and security guarantees that tags cannot offer (since tags can be moved to point to different images).

The step specifies the image using a SHA256 digest:
```yaml
steps:
  - image: mirror.gcr.io/busybox@sha256:1303dbf110c57f3edf68d9f5a16c082ec06c4cf7604831669faf2c712260b5a0
    args: ['-c', 'echo hello']
```

When using digest-based image references:
- The image is specified as `registry/image@sha256:digest` instead of `registry/image:tag`
- Tekton must query the container registry to look up the image's entrypoint/command since no command is explicitly set
- This provides stronger guarantees about image immutability compared to tags
- It's particularly useful for production environments where reproducibility is critical

This example uses `generateName` so it creates a new TaskRun instance each time:
```bash
kubectl create -f examples/v1/taskruns/step-by-digest.yaml
# taskrun.tekton.dev/step-by-digest-abc123 created
```

**Note:** When the command is not explicitly set in the step definition, Tekton automatically retrieves it from the container registry's image metadata. This is why the example includes the comment "command is not set, so it must be looked up from the registry."

### [entrypoint-resolution.yaml](entrypoint-resolution.yaml)

This example demonstrates how Tekton resolves container entrypoints (commands) for different scenarios, particularly with multi-architecture images. Understanding entrypoint resolution is important for writing portable tasks that work across different platforms (amd64, arm64, etc.).

The TaskRun shows four different entrypoint resolution patterns:

**1. Multi-arch image with no command or args:**
```yaml
- image: ghcr.io/tektoncd/pipeline/nop-8eac7c133edad5df719dc37b36b62482:latest
```
Tekton looks up the command for each platform-specific image and passes it to the Pod. Kubernetes then selects the appropriate command at runtime based on the node's platform.

**2. Multi-arch image with args but no command:**
```yaml
- image: mirror.gcr.io/ubuntu
  args: ['-c', 'echo', 'hello']
```
Tekton looks up the image's default command from the registry, passes it to the entrypoint binary via environment variable, then appends the specified args.

**3. Multi-arch image with script:**
```yaml
- image: mirror.gcr.io/ubuntu
  script: echo hello
```
When `script` is specified, Tekton doesn't need to look up the entrypoint because it wraps the script in its own shell execution.

**4. Multi-arch image with explicit command and args:**
```yaml
- image: mirror.gcr.io/ubuntu
  command: ['sh', '-c']
  args: ['echo hello']
```
When both `command` and `args` are explicitly specified, Tekton doesn't need to query the registry since the execution is fully defined.

**Create and run:**
```bash
kubectl create -f examples/v1/taskruns/entrypoint-resolution.yaml
# taskrun.tekton.dev/entrypoint-resolution created

# Check the status
kubectl get taskrun entrypoint-resolution
```

This example demonstrates Tekton's intelligent entrypoint resolution that balances flexibility (allowing images to define their own entrypoints) with explicitness (allowing users to override when needed), while supporting multi-architecture deployments.

### [readonly-internal-dir.yaml](readonly-internal-dir.yaml)

This example demonstrates Tekton's security model for internal directories. Tekton enforces read-only access to certain internal directories to prevent steps from interfering with each other's execution state or metadata. This is primarily used for test validation and demonstrates important security boundaries within TaskRuns.

**Important:** This is not a typical user configuration example, but rather a test validation file that demonstrates Tekton's internal security mechanisms.

The TaskRun contains three steps that verify directory access restrictions:

**Step 1 - Baseline step:**
```yaml
- image: mirror.gcr.io/ubuntu
  script: exit 0
```
A simple step that completes successfully, creating its run directory structure.

**Step 2 - Verify run directory isolation:**
```yaml
- image: mirror.gcr.io/ubuntu
  script: |
    set +e # dont fail the script on error
    
    # Steps should not be able to write to other step's run directories.
    echo "hello world" > /tekton/run/0/out
    if [ $? -eq 0 ] ; then
      echo "able to write to run directory of non-current step"
      exit 1
    fi
```
This step attempts to write to the first step's run directory (`/tekton/run/0/out`). The write should fail because Tekton makes other steps' run directories read-only. If the write succeeds, the test fails.

**Step 3 - Verify steps directory isolation:**
```yaml
- image: mirror.gcr.io/ubuntu
  script: |
    set +e # dont fail the script on error
    
    # Steps should not be able to write to other step's step directories.
    echo "hello world" > /tekton/steps/0/exitCode
    if [ $? -eq 0 ] ; then
      echo "able to write to run directory of non-current step"
      exit 1
    fi
```
This step attempts to write to the first step's exit code file (`/tekton/steps/0/exitCode`). The write should fail because Tekton protects step metadata from modification by other steps.

**Security implications:**
- Each step can only write to its own `/tekton/run/<step-index>/` directory
- Steps can read but not modify other steps' exit codes and metadata
- This prevents malicious or buggy steps from corrupting the execution state of other steps
- It ensures the integrity of Tekton's internal tracking mechanisms

**Create and run:**
```bash
kubectl create -f examples/v1/taskruns/readonly-internal-dir.yaml
# taskrun.tekton.dev/readonly-internal-dir-xyz789 created

# The TaskRun should succeed, confirming that write attempts to protected directories fail as expected
kubectl get taskrun readonly-internal-dir-xyz789
```

This example validates that Tekton properly enforces isolation between steps, which is crucial for secure multi-step task execution.

# Additional Resources

Tekton Pipelines documentation (https://tekton.dev/docs/pipelines/)
Tekton TaskRun documentation (https://tekton.dev/docs/pipelines/taskruns/)