# Tekton PipelineRun Examples

This directory contains examples of Tekton PipelineRun resources. A Pipeline is a collection of Tasks that you define and arrange in a specific order of execution as part of your continuous integration flow. A PipelineRun instantiates and executes a Pipeline with specific inputs, outputs, and execution parameters on a Kubernetes cluster.


## Table of Contents

- [Prerequisites](#prerequisites)
- [Understanding `metadata.generateName` vs `metadata.name`](#understanding-metadatageneratename-vs-metadataname)
  - [Using `generateName`](#using-generatename)
  - [Using `name` (Most Examples)](#using-name-most-examples)
- [Monitoring PipelineRuns](#monitoring-pipelineruns)
- [Basic Examples](#basic-examples)
  - [pipelinerun.yaml](#pipelinerunyaml)
  - [pipelinerun-with-pipelinespec.yaml](#pipelinerun-with-pipelinespecyaml)
  - [pipelinerun-with-pipelinespec-and-taskspec.yaml](#pipelinerun-with-pipelinespec-and-taskspecyaml)
- [Parameters and Results](#parameters-and-results)
  - [pipelinerun-with-params.yaml](#pipelinerun-with-paramsyaml)
  - [pipelinerun-with-extra-params.yaml](#pipelinerun-with-extra-paramsyaml)
  - [pipelinerun-results.yaml](#pipelinerun-resultsyaml)
  - [pipelinerun-results-with-params.yaml](#pipelinerun-results-with-paramsyaml)
  - [pipelinerun-array-results-substitution.yaml](#pipelinerun-array-results-substitutionyaml)
  - [pipeline-object-param-and-result.yaml](#pipeline-object-param-and-resultyaml)
  - [pipeline-object-results.yaml](#pipeline-object-resultsyaml)
  - [task_results_example.yaml](#task_results_exampleyaml)
  - [pipelinerun-param-array-indexing.yaml](#pipelinerun-param-array-indexingyaml)
- [Propagating Parameters and Results](#propagating-parameters-and-results)
  - [propagating_params_in_pipeline.yaml](#propagating_params_in_pipelineyaml)
  - [propagating_params_implicit_parameters.yaml](#propagating_params_implicit_parametersyaml)
  - [propagating_params_with_scope_precedence.yaml](#propagating_params_with_scope_precedenceyaml)
  - [propagating_params_with_scope_precedence_default_only.yaml](#propagating_params_with_scope_precedence_default_onlyyaml)
  - [propagating_results_implicit_resultref.yaml](#propagating_results_implicit_resultrefyaml)
- [Workspaces](#workspaces)
  - [workspaces.yaml](#workspacesyaml)
  - [workspace-from-volumeclaimtemplate.yaml](#workspace-from-volumeclaimtemplateyaml)
  - [workspaces-projected.yaml](#workspaces-projectedyaml)
  - [optional-workspaces.yaml](#optional-workspacesyaml)
  - [mapping-workspaces.yaml](#mapping-workspacesyaml)
  - [propagating-workspaces.yaml](#propagating-workspacesyaml)
  - [propagating-workspaces-in-pipelines.yaml](#propagating-workspaces-in-pipelinesyaml)
  - [propagating_workspaces_with_referenced_resources.yaml](#propagating_workspaces_with_referenced_resourcesyaml)
  - [pipelinerun-using-different-subpaths-of-workspace.yaml](#pipelinerun-using-different-subpaths-of-workspaceyaml)
  - [pipelinerun-using-parameterized-subpath-of-workspace.yaml](#pipelinerun-using-parameterized-subpath-of-workspaceyaml)
- [Task Execution Control](#task-execution-control)
  - [pipelinerun-with-when-expressions.yaml](#pipelinerun-with-when-expressionsyaml)
  - [using-optional-workspaces-in-when-expressions.yaml](#using-optional-workspaces-in-when-expressionsyaml)
  - [pipelinerun-with-parallel-tasks-using-pvc.yaml](#pipelinerun-with-parallel-tasks-using-pvcyaml)
  - [pipelinerun-task-execution-status.yaml](#pipelinerun-task-execution-statusyaml)
- [Final Tasks](#final-tasks)
  - [pipelinerun-with-final-tasks.yaml](#pipelinerun-with-final-tasksyaml)
  - [pipelinerun-with-final-results.yaml](#pipelinerun-with-final-resultsyaml)
- [Timeouts and Retries](#timeouts-and-retries)
  - [pipelinerun-with-task-timeout-override.yaml](#pipelinerun-with-task-timeout-overrideyaml)
- [StepActions](#stepactions)
  - [stepaction-params.yaml](#stepaction-paramsyaml)
- [Context Variables](#context-variables)
  - [using_context_variables.yaml](#using_context_variablesyaml)
- [Error Handling](#error-handling)
  - [ignore-step-error.yaml](#ignore-step-erroryaml)
- [Display Names](#display-names)
  - [pipeline-with-displayname.yaml](#pipeline-with-displaynameyaml)
- [Additional example sets](#additional-examples-sets)
- [Additional Resources](#additional-resources)


---

## Prerequisites

Before running these examples, ensure you have:

1. **Kubernetes Cluster** (v1.28 or later)
   Follow the instructions to [setup a Kubernetes Cluster](../../../DEVELOPMENT.md#setup-a-kubernetes-cluster). For Local Kubernetes Cluster setup, you can use tools such as [kind](https://kind.sigs.k8s.io/), [minikube](https://minikube.sigs.k8s.io/), or [Docker Desktop](https://docs.docker.com/desktop/). The managed Kubernetes services offered by the cloud providers can also be used.

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

4. **Jq** for parsing PipelineRun results (Optional)

---

## Understanding `metadata.generateName` vs `metadata.name`

PipelineRuns can be created with either `generateName` or `name` in their metadata, and this choice affects how you create and manage them.

### Using `generateName`

**YAML structure:**
```yaml
apiVersion: tekton.dev/v1
kind: PipelineRun
metadata:
  generateName: my-pipelinerun-  # No fixed name!
spec:
  # ... pipeline specification
```

- Creates a **new instance every time** with a unique name (e.g., `my-pipelinerun-abc123`, `my-pipelinerun-xyz789`)
- Kubernetes automatically appends a random suffix to the base name
- Allows running the same pipeline multiple times without name conflicts

```bash
# MUST use kubectl create (generates new instance)
kubectl create -f my-pipelinerun.yaml
# Output: pipelinerun.tekton.dev/my-pipelinerun-abc123 created

kubectl create -f my-pipelinerun.yaml
# Output: pipelinerun.tekton.dev/my-pipelinerun-xyz789 created (different name!)

# CANNOT use kubectl apply (requires fixed name)
kubectl apply -f my-pipelinerun.yaml
# Error: error: from my-pipelinerun.yaml: cannot use generate name with apply

#example
#kubectl create -f pipelinerun-using-parameterized-subpath-of-workspace.yaml                           
#task.tekton.dev/writer created
#task.tekton.dev/read-both created
#pipeline.tekton.dev/pipeline-using-parameterized-subpaths created
#pipelinerun.tekton.dev/pr-parameterized-subpath-2ln9t created
```

### Using `name` (Most Examples)

**YAML structure:**
```yaml
apiVersion: tekton.dev/v1
kind: PipelineRun
metadata:
  name: my-pipelinerun  # fixed name!
spec:
  # ... pipeline specification
```

- Always creates the PipelineRun resource with the **same name** (`my-pipelinerun`)
- First execution creates the resource
- You can use either `kubectl create` or `kubectl apply` for the first time to create a new instance
- Once `my-pipelinerun` already exists:
  - `kubectl create` will fail with "already exists" error
  - `kubectl apply` will update/replace the existing resource
- Only one instance can exist at a time with this name
- To run again with `create`, you must delete the existing PipelineRun first

```bash
# First time - both commands work
kubectl create -f my-pipelinerun.yaml
# Output: pipelinerun.tekton.dev/my-pipelinerun created

# OR
kubectl apply -f my-pipelinerun.yaml
# Output: pipelinerun.tekton.dev/my-pipelinerun created

# Second time - create fails, apply works
kubectl create -f my-pipelinerun.yaml
# Error: pipelinerun.tekton.dev "my-pipelinerun" already exists

kubectl apply -f my-pipelinerun.yaml
# Output: pipelinerun.tekton.dev/my-pipelinerun configured
```

---

## Monitoring PipelineRuns
**Understanding PipelineRun status**
```bash
kubectl get pipelinerun <PIPELINERUN_NAME>
#kubectl get pipelinerun pr-parameterized-subpath-2ln9t
#NAME                             SUCCEEDED   REASON      STARTTIME   COMPLETIONTIME
#pr-parameterized-subpath-2ln9t   True        Succeeded   100s        78s
```

**Debugging PipelineRun**
```bash
kubectl get pipelinerun pr-parameterized-subpath-2ln9t -o yaml   
```
Retrieves the complete, detailed YAML representation of the PipelineRun resource from Kubernetes. Main sections of the output of above command:
1. apiVersion & kind
2. metadata - Resource Information
3. spec - What You Asked For
4. status - What Actually Happened (status, completionTime, etc.)

---

## Basic Examples

### [pipelinerun.yaml](pipelinerun.yaml)

A comprehensive example demonstrating a complete CI/CD pipeline that clones a Git repository and builds a container image using Kaniko. This example includes:

**ClusterRoleBinding** for service account permissions. This grants admin privileges to the default service account. This is needed because the pipeline will pull images, build containers, push images to a registry which need permission.

    ```yaml
    kind: ClusterRoleBinding
    ```
This part of the example grants permissions to the default service account in the default namespace.
```yaml
subjects:
  - kind: ServiceAccount
    name: default
    namespace: default
```

The service account cluster admin privileges are defined in this part:
```yaml
roleRef:
  kind: ClusterRole
  name: cluster-admin
  apiGroup: rbac.authorization.k8s.io
```

**git-clone** task definition clones a Git repository into a workspace `output`. This is where the repository will be downloaded. Important parameters:
- url: the URL of the Git repository to clone
- revision: the branch, tag, or commit to checkout
- depth: Shallow clone depth (only recent commits are fetched)
- submodules: whether to fetch submodules
- deleteExisting: whether to delete the contents of the directory before cloning

**Kaniko** build task definition allows building container images from `Dockerfile` without requiring a Docker daemon. 

**Pipeline** `demo.pipeline` chains tasks together. 
Task 1: `fetch-from-git` which clones the repo `https://github.com/GoogleContainerTools/skaffold`.
Task 2: `build-container-image` runs after `fetch-from-git` which builds a container image from Dockerfile `examples/getting-started/Dockerfile` inside the cloned repo using Kaniko. The image destination used in the example `gcr.io/christiewilson-catfactory/getting-started`.

**PipelineRun** starts the pipeline execution. 
```yaml
kind: PipelineRun
```
This PipelineRun references the `demo.pipeline`.
``` yaml
  pipelineRef:
    name: "demo.pipeline"
```

Tekton dynamically creates a `PersistentVolumeClaim` to store the cloned repo and build context.
```yaml
volumeClaimTemplate:
      spec:
        accessModes:
          - ReadWriteOnce
        resources:
          requests:
            storage: 1Gi
``` 
The pipeline fetches source code from GitHub (GoogleContainerTools/skaffold) and builds a container image, demonstrating a CI workflow.


**Create:**
```bash
cd pipeline
kubectl create -f examples/v1/pipelineruns/pipelinerun.yaml
# pipelinerun.tekton.dev/demo-pipeline-run-1 created

# Monitor the PipelineRun
kubectl get pipelinerun demo-pipeline-run-1 -o yaml

# Check task execution
kubectl get taskruns
```

**Cleanup:**
```bash
kubectl delete pipelinerun demo-pipeline-run-1
kubectl delete pipeline demo.pipeline
kubectl delete task git-clone kaniko
kubectl delete clusterrolebinding <generated-name>
```


### [pipelinerun-with-pipelinespec.yaml](pipelinerun-with-pipelinespec.yaml)

Demonstrates inline pipeline definition using `pipelineSpec` instead of referencing an external Pipeline resource. This approach is useful for:
- Quick prototyping and testing
- Self-contained pipeline definitions
- Avoiding the need to create separate Pipeline resources

The pipeline is defined directly within the PipelineRun specification, making it easier to manage single-use or experimental pipelines.


### [pipelinerun-with-pipelinespec-and-taskspec.yaml](pipelinerun-with-pipelinespec-and-taskspec.yaml)

Shows how to define both the pipeline and tasks inline within a single PipelineRun resource. This example demonstrates:
- Inline pipeline specification (`pipelineSpec`)
- Inline task specifications (`taskSpec`)
- Complete self-contained pipeline definition

This approach is ideal for simple pipelines where you want everything defined in one place without creating separate Pipeline and Task resources.

---

## Parameters and Results

### [pipelinerun-with-params.yaml](pipelinerun-with-params.yaml)

Demonstrates parameter passing in pipelines. This example shows:
1. Pipeline-level parameters with default values.
This pipeline accepts two string parameters. If you don't provide values, I'll use '1' for both.
```yaml
spec:
  params:
  - name: pl-param-x
    type: string
    default: "1"      # Default if not provided
  - name: pl-param-y
    type: string
    default: "1"
```
2. Pipeline Passes to Tasks
Take the pipeline's parameters and pass them to the task's parameters (with different names: pl-param-x → a)
```yaml
tasks:
- name: sum-params
  taskRef:
    name: sum-params
  params:
  - name: a
    value: "$(params.pl-param-x)"  # Pass pipeline param to task
  - name: b
    value: "$(params.pl-param-y)"
- Passing parameters from Pipeline to Tasks
- Multiple tasks using the same parameters
```
The pipeline defines two parameters (`pl-param-x` and `pl-param-y`) that are passed to two different tasks (sum-params and multiply-params), demonstrating how parameters flow through the pipeline hierarchy.


### [pipelinerun-with-extra-params.yaml](pipelinerun-with-extra-params.yaml)

Shows how to handle extra parameters that are provided in the PipelineRun but not declared in the Pipeline specification. Each Pipeline uses what it needs and ignores the extra parameters.


### [pipelinerun-results.yaml](pipelinerun-results.yaml)

Demonstrates how tasks emit results and how those results can be consumed by subsequent tasks in the pipeline. This example shows:
- Task result declaration `results: - name: sum`
- Writing results in task steps `tee $(results.sum.path)`
- Consuming task results in other tasks using `$(tasks.<task-name>.results.<result-name>)`


### [pipelinerun-results-with-params.yaml](pipelinerun-results-with-params.yaml)

Combines parameters and results, showing how to:
- Pass parameters to tasks
- Emit results from tasks
- Use both parameters and results together in a pipeline workflow

**PipelineRun provides parameter**
```yaml
params:
- name: prefix
  value: prefix
```
**Task generates result**
```yaml
- name: generate-suffix
  taskSpec:
    results:
    - name: suffix
    steps:
    - script: |
        echo -n "suffix" > $(results.suffix.path)
```
**Combine parameter and result**
```yaml
- name: do-something
  params:
  - name: arg
    value: "$(params.prefix):$(tasks.generate-suffix.results.suffix)"
```
**Task validates**
```yaml
steps:
- script: |
    echo "$(params.arg)" | grep "prefix:suffix"

```
### [pipelinerun-array-results-substitution.yaml](pipelinerun-array-results-substitution.yaml)

Demonstrates array-type results and how to substitute them in subsequent tasks. Shows:
- Emitting array results from tasks
```yaml
results:
- name: array-results-1
  type: array              # ← Array type!
  description: The array results
```
- Consuming array results in other tasks
```yaml
# Get All elements in array
params:
- name: images
  value: "$(tasks.task1.results.array-results-1[*])"  # All elements!

# Get a specific element in array
params:
- name: image-1
  value: "$(tasks.task1.results.array-results-1[2])"  # Third element (0-indexed)
```

### [pipeline-object-param-and-result.yaml](pipeline-object-param-and-result.yaml)

Shows object-type parameters and results in pipelines. This example demonstrates:
- Defining object parameters in Task
```yaml
params:
- name: arg
  type: object          # ← Object type!
  properties:
    url:
      type: string
    commit:
      type: string
```

- Defining object parameters in Pipeline
```yaml
params:
- name: gitrepo
  properties:
    url: {}             # Empty {} means "any value"
    commit: {}
```

- Defining object parameters in PipelineRun
```yaml
params:
- name: gitrepo
  value:
    url: abc.com        # Provide actual values
    commit: sha123
```

- Accessing object properties using dot notation
```yaml
$(params.arg.commit)
```
- Accessing whole object using Array Expansion
```yaml
$(params.gitrepo[*])
```
- Receive Object Result
```yaml
params:
- name: arg
  value: "$(tasks.task1.results.object-result[*])"  # Whole object

# Access properties
script: |
  echo "Foo: $(params.arg.foo)"
  echo "Bar: $(params.arg.bar)"
```

### [pipeline-object-results.yaml](pipeline-object-results.yaml)

Focuses specifically on object-type results, showing how Tasks can emit structured data as JSON objects and how subsequent Tasks can consume these object results.
**Get Whole Object**
```yaml
params:
- name: whole-object
  value: "$(tasks.task1.results.object-results[*])"
  type: object
```

**Get Single Property**
```yaml
params:
- name: object-url
  value: "$(tasks.task1.results.object-results.url)"  # Just the URL!
  type: string
```

### [task_results_example.yaml](task_results_example.yaml)

The pipeline accepts two inputs:
```yaml
params:
  - name: a
  - name: b
```

Task 1 `sum-inputs` uses the sum Task. The result is stored as:

```
$(tasks.sum-inputs.results.sum)
```

Task 2 `multiply-inputs` uses the multiply Task. The result is stored as:

```
$(tasks.multiply-inputs.results.product)
``` 

Task 3 `sum-and-multiply` runs after the first two Tasks because it uses their results.

It builds parameters a and b like this (20 + 12 → combined as string → "2012"). Then again adds a and b to produce final results.

```
$(tasks.multiply-inputs.results.product)$(tasks.sum-inputs.results.sum)
```


### [pipelinerun-param-array-indexing.yaml](pipelinerun-param-array-indexing.yaml)

Demonstrates array parameter indexing, showing how to:
- Define array parameters
```yaml
Array Indexing Basics
# Array parameter
params:
- name: environments
  type: array
  value:
    - 'staging'    # Index 0
    - 'qa'         # Index 1
    - 'prod'       # Index 2

```

- Access individual array elements using index notation `$(params.array[0])`
- Use array expansion `$(params.array[*])`

---

## Propagating Parameters and Results

---

### [propagating_params_in_pipeline.yaml](propagating_params_in_pipeline.yaml)

Shows explicit parameter propagation from PipelineRun to Pipeline to Tasks. This is the standard approach where:
- Parameters are declared at each level
- Values flow explicitly through the hierarchy
- Each component declares what parameters it expects

**Level 1: PipelineRun Provides Value**
```yaml
spec:
  params:
  - name: HELLO
    value: "Hello from pipeline run"

```

**Level 2: Pipeline Declares Parameter**
```yaml
spec:
  params:
  - name: HELLO
    default: "Pipeline Hello World!"  # Used if PipelineRun doesn't provide

```
**Level 3: Task Uses Parameter**
```yaml
tasks:
- name: echo-hello
  taskSpec:
    steps:
    - script: |
        echo "$(params.HELLO)"  # Automatically available!
```

---

### [propagating_params_implicit_parameters.yaml](propagating_params_implicit_parameters.yaml)

Demonstrates implicit parameter propagation where parameters defined in the PipelineRun are automatically available to tasks without explicit declaration in the Pipeline specification. This simplifies pipeline definitions when many parameters need to be passed through.

**Explicit (Previous Example):**
```yaml
# Pipeline MUST declare
spec:
  params:
  - name: HELLO
    default: "..."
```

**Implicit (This Example):**
```yaml
# Pipeline does NOT declare!
spec:
  # No params section!
  tasks:
  - name: echo-hello
    taskSpec:
      steps:
      - script: echo "$(params.HELLO)"  # Still works!
```

The pipeline provides the parameter.
```yaml
# PipelineRun provides
params:
- name: HELLO
  value: "Pipeline Hello World!"

# Task uses directly (no Pipeline declaration needed!)
script: echo "$(params.HELLO)"
```


---


### [propagating_params_with_scope_precedence.yaml](propagating_params_with_scope_precedence.yaml)

Shows parameter scope precedence rules when the same parameter is defined at multiple levels:
- PipelineRun level
- Pipeline level (defaults)
- Task level (defaults)


#### PipelineRun level
```yaml
params:
  - name: HELLO
    value: "Pipeline Hello World!"
```
Default value for the pipeline.

#### Task level
```yaml
tasks:
  - name: echo-hello
    params:
      - name: HELLO
        value: "Task Hello World!"
```
This overrides the PipelineRun parameter for this task. So inside this task:

`echo "$(params.HELLO)"`

Output:

`Task Hello World!`

#### Finally task level
```yaml
finally:
  - name: echo-hello-finally
    params:
      - name: HELLO
        value: "Finally Hello World!"
```
This overrides the pipeline parameter for the finally task.

Output:
`And finally.. Finally Hello World!`

Precedence order in this example

```bash
Highest priority → Lowest priority

Task / Finally params
        |
        v
PipelineRun params
        |
        v
Task default params (if defined)
```

Demonstrates which value takes precedence when parameters are defined at multiple scopes.


### [propagating_params_with_scope_precedence_default_only.yaml](propagating_params_with_scope_precedence_default_only.yaml)

Similar to the above but focuses on default values only, showing how defaults at different levels interact when no explicit value is provided.

#### PipelineRun parameter
```yaml
params:
  - name: HELLO
    value: "Pipeline Hello World!"
```
This provides the value:

`Pipeline Hello World!`

for the whole pipeline.

#### Task default parameter

Inside both tasks:
```yaml
params:
  - name: HELLO
    default: "Default Hello World!"
```
This value is used only if no other value is provided.

Default value:

`Default Hello World!`
#### Precedence rule

In Tekton Pipelines the precedence is:
```yaml
PipelineRun params
        ↓
Task default params
```
Higher level input overrides defaults.

#### What happens during execution
Task echo-hello

The script runs:

`echo "$(params.HELLO)"`

Output:

`Pipeline Hello World!`

#### Finally task echo-hello-finally

Script:

```bash
echo "And finally... $(params.HELLO)"
```
Again Tekton finds the PipelineRun value.

Output:

`And finally... Pipeline Hello World!`

#### Why the default is ignored

Because defaults are only used when no value is provided externally. Here the PipelineRun already provides a value, so:

`Default Hello World!`

is never used.



### [propagating_results_implicit_resultref.yaml](propagating_results_implicit_resultref.yaml)

Demonstrates implicit result references where Task results can be consumed without explicit result declarations in the Pipeline specification.
The Task `show-uid` needs the result `$(tasks.add-uid.results.uid)` from the Task `add-uid`. The dependency between these Tasks is not explicitly declared using `runAfter`. However, Tekton will automatically infer the dependency and run `show-uid` after `add-uid` has completed.


---


## Workspaces

### [workspaces.yaml](workspaces.yaml)

A comprehensive example showing multiple workspace types in a Pipeline:
- ConfigMap workspace for configuration data.
```yaml
- name: recipe-store
  configMap:
    name: sensitive-recipe-storage
    items:
    - key: brownies
      path: recipe.txt
```

- Secret workspace for sensitive data.
```yaml
- name: password-vault
  secret:
    secretName: secret-password
```
- PersistentVolumeClaim workspace for shared storage or storing build artifacts.
```yaml
- name: shared-data
  persistentVolumeClaim:
    claimName: shared-task-storage
```

The Pipeline demonstrates how different workspace types can be used together, with Tasks reading from ConfigMap and Secret workspaces, and sharing data through a PVC workspace.

**Task 1: fetch-secure-data**
1. Reads password from Secret workspace
2. Verifies it's correct ("hunter2")
3. Copies recipe from ConfigMap to PVC

```yaml
steps:
- name: fetch-and-write
  script: |
    # Check password from Secret workspace
    if [ "hunter2" = "$(cat $(workspaces.super-secret-password.path)/password)" ]; then
      # Copy recipe from ConfigMap to PVC
      cp $(workspaces.secure-store.path)/recipe.txt $(workspaces.filedrop.path)
    else
      echo "wrong password!"
      exit 1
    fi
```

**Task 2: print-the-recipe**
```yaml
steps:
- name: print-secrets
  script: cat $(workspaces.storage.path)/$(params.filename)
```
What happens:

1. Reads the recipe from the PVC (shared from Task 1)
2. Prints it to the console


### [workspace-from-volumeclaimtemplate.yaml](workspace-from-volumeclaimtemplate.yaml)

Shows dynamic workspace provisioning using volumeClaimTemplate. Instead of pre-creating a PVC, the PipelineRun automatically creates one based on the template specification. These are perfect for temporary storage; each PipelineRun gets its own storage.


#### Manual PVC Creation and Referencing
**Step 1: Create PVC first**
```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: shared-task-storage
spec:
  resources:
    requests:
      storage: 16Mi
```
**Step 2: Reference it**
```yaml
workspaces:
- name: ws
  persistentVolumeClaim:
    claimName: shared-task-storage

```
#### Automatic PVC (this example):
```yaml
# No pre-creation needed!
workspaces:
- name: ws
  volumeClaimTemplate:  # Creates PVC automatically
    spec:
      accessModes:
      - ReadWriteOnce
      resources:
        requests:
          storage: 1Gi
```

This means that the workspace `ws` is backed by the dynamically created PVC. So the workspace is mounted from that PVC.

- The `accessModes:` is set to `ReadWriteOnce`. ReadWriteOnce is the most commonly available Access Mode. A volume with this Access Mode can only be mounted on one Node at a time. This can be problematic for a Pipeline that has parallel Tasks that access the volume concurrently. The Affinity Assistant helps with this problem by scheduling all Tasks that use the same PersistentVolumeClaim to the same Node.

Other accessModes are:

- **ReadOnlyMany** is read-only and is less common in a CI/CD-pipeline. These volumes often need to be “prepared” with data in some way before use. Dynamically provided volumes can usually not be used in read-only mode.

- **ReadWriteMany** is the least commonly available Access Mode. If you use this access mode and these volumes are available to all Nodes within your cluster, you may want to disable the Affinity Assistant.



#### Tasks mount the same workspace
Both Tasks map their workspaces to the same Pipeline workspace:

**Writer:**
```yaml
workspaces:
  - name: task-ws
    workspace: ws
```
**Reader:**
```yaml
workspaces:
  - name: myws
    workspace: ws
```
Both ultimately point to the same PVC.
`writer` Task writes to the workspace, `reader` Task reads from the workspace.

### [workspaces-projected.yaml](workspaces-projected.yaml)

Demonstrates projected volumes that combine multiple sources (ConfigMaps, Secrets) into a single workspace. This allows Tasks to access multiple configuration sources through one workspace mount.

Instead of Two Separate Workspaces:

```yaml
workspaces:
- name: password-vault
  secret: ...
- name: recipe-store
  configMap: ...
```
Use ONE Projected Workspace:
```yaml
  workspaces:
  - name: data-store
    projected:
      sources:
        - secret:
            name: secret-password
        - configMap:
            name: sensitive-recipe-storage
            items:
              - key: brownies
                path: recipe.txt
```


### [optional-workspaces.yaml](optional-workspaces.yaml)

Shows how to define optional workspaces in Pipelines. Tasks can check if a workspace is bound using `$(workspaces.<name>.bound)` and adapt their behavior accordingly.


### [mapping-workspaces.yaml](mapping-workspaces.yaml)

Demonstrates workspace name mapping between Pipeline and Task levels. Shows how a workspace can have different names at the Pipeline level versus the Task level.
**Pipeline Level (Generic Names):**
```yaml
workspaces:
- name: password-vault
- name: recipe-store
- name: shared-data
```
**Task Level (Specific Names):**
```yaml
workspaces:
- name: password-vault  # Task's name for the workspace
- name: recipe-store
- name: shared-data
```
**Mapping in Pipeline:**

```yaml
tasks:
- name: fetch-the-recipe
  workspaces:
  - name: password-vault  # Task expects this name
    workspace: password-vault  # Maps to pipeline's workspace
```

### [propagating-workspaces-in-pipelines.yaml](propagating-workspaces-in-pipelines.yaml)

A workspace declared at the Pipeline level automatically becomes available to Tasks.
So if the Pipeline defines:
```yaml
workspaces:
  - name: shared-data
```
Tasks can reference it using:
```yaml
$(workspaces.shared-data.path)
```

Execution flow:
```
PipelineRun starts
      │
      v
PVC created (16Mi)
      │
      v
Task t1 Pod
      │
      v
Workspace mounted
/workspace/shared-data
      │
      v
ls /workspace/shared-data

```


### [propagating-workspaces.yaml](propagating-workspaces.yaml)

Shows workspace propagation from PipelineRun through Pipeline to Tasks, demonstrating how workspaces flow through the Pipeline hierarchy.
- A ConfigMap stores a brownie recipe.
- A Secret stores a password (hunter2, base64 encoded).
- A PipelineRun creates a shared workspace (PVC) to pass data between tasks.

#### Pipeline flow:
**Task 1 – fetch-secure-data**
Reads the password from the Secret. If it matches hunter2, it copies the recipe from the ConfigMap to the shared workspace.
**Task 2 – print-the-recipe**
Runs after Task 1. Reads the recipe from the shared workspace and prints it.



### [propagating_workspaces_with_referenced_resources.yaml](propagating_workspaces_with_referenced_resources.yaml)

Demonstrates how workspaces are propagated from a PipelineRun -> Pipeline -> referenced Task when using TaskRef.
The Task `fetch-secure-data` declares a workspace `shared-data`. The PipelineRun creates a workspace `shared-data` that uses `PersistentVolumeClaim` to store the workspace. The Task `fetch-the-recipe` references the Task which writes 'hi' into a file recipe.txt inside the workspace.
```yaml
      - name: shared-data
      taskRef:  # Referencing a resource
        name: fetch-secure-data
```

Because the external Task uses a workspace, the Pipeline must explicitly declare the workspace. This connects the Pipeline workspace to the Task workspace.
```yaml
      workspaces:  # If referencing resources, Workspaces need to be explicitly declared
      - name: shared-data
```
The Task `print-the-recipe` runs after the first Task. The script reads the file recipe.txt and prints the contents of the file.



### [pipelinerun-using-different-subpaths-of-workspace.yaml](pipelinerun-using-different-subpaths-of-workspace.yaml)

Shows how different Tasks can use different subdirectories within the same workspace.
**The Folder Structure**
```
Workspace (ws)
|-- dir-1/
|   |-- foo (written by writer-1)
|-- dir-2/
    |-- foo (written by writer-2)

```
In this example:
- Task 1 `writer-1`: Write to dir-1
- Task 2 `writer-2`: Write to dir-2
- Task 3 `read-all`: Read from both

### [pipelinerun-using-parameterized-subpath-of-workspace.yaml](pipelinerun-using-parameterized-subpath-of-workspace.yaml)

Demonstrates using parameters to dynamically specify workspace subpaths, allowing flexible workspace organization based on runtime values.

```yaml
# Tekton Parameter to store the workspace subpath value
params:
- name: subPath-path
  value: dir-2  # Can be changed at runtime!

...
# The task uses the workspace subpath value from parameter
  workspaces:
  - name: task-ws
    workspace: ws
    subPath: $(params.subPath-path)  # Dynamic!

```

---

## Task Execution Control

### [pipelinerun-with-when-expressions.yaml](pipelinerun-with-when-expressions.yaml)

This example demonstrates conditional Task execution in Tekton Pipelines using "when expressions". Think of it like an "if statement" in programming - Tasks only run when certain conditions are true.

**Pipeline parameters**
The Pipeline accepts two inputs:
 - path: A filename (in this case, "README.md")
 - branches: A list of branch names (["main", "hotfix"])
```yaml
params:
  - name: path
    type: string
    description: The path of the file to be created
  - name: branches
    type: array
    description: The list of branch names
```
**Shared Workspace**
What it means: All Tasks share a common storage area (like a shared folder) where they can read/write files.

``` yaml
workspaces:
  - name: source
    description: This workspace is shared among all tasks
```

**Task 1: create-file RUNS**
```yaml
- name: create-file
  when:
    - input: "$(params.path)"
      operator: in
      values: ["README.md"]
```
When Expression: "Run this task ONLY if the path parameter equals 'README.md'"
What it does: Creates a README.md file in the workspace
Result: RUNS because path = "README.md" (matches the condition)


**Task 2: check-file**
```yaml
- name: check-file
  runAfter:
    - create-file
```
No when expression - always runs after create-file

What it does:
Checks if the file exists
Outputs a result: "yes" if exists, "no" if missing
Result: RUNS and outputs "yes" (because create-file created it)

**Task 3: echo-file-exists - RUNS**
```yaml
- name: echo-file-exists
  when:
    - input: "$(tasks.check-file.results.exists)"
      operator: in
      values: ["yes"]
```
When Expression: "Run ONLY if check-file's result is 'yes'"
What it does: Prints "file exists"
Result: RUNS because check-file returned "yes"

**Task 4: sample-task-with-array-values - RUNS**
```yaml
- name: sample-task-with-array-values
  when:
    - input: "main"
      operator: in
      values: ["$(params.branches[*])"]
```
When Expression: "Run ONLY if 'main' is in the branches array"
What it does: Prints "hello"
Result: RUNS because branches = ["main", "hotfix"] (contains "main")

**Task 5: task-should-be-skipped-1 - SKIPPED**
```yaml
- name: task-should-be-skipped-1
  when:
    - input: "$(tasks.check-file.results.exists)"
      operator: in
      values: ["missing"]
```
When Expression: "Run ONLY if check-file's result is 'missing'"
Result: SKIPPED because check-file returned "yes", not "missing"

**Task 6: task-should-be-skipped-2 - SKIPPED**
```yaml
- name: task-should-be-skipped-2
  when:
    - input: "$(params.path)"
      operator: notin
      values: ["README.md"]
```
When Expression: "Run ONLY if path is NOT 'README.md'"
Result: SKIPPED because path IS "README.md"

**Task 7: task-should-be-skipped-3  - SKIPPED**
```yaml
- name: task-should-be-skipped-3
  when:
    - input: "monday"
      operator: in
      values: ["friday"]
```
When Expression: "Run ONLY if 'monday' equals 'friday'"
Result: SKIPPED because "monday" ≠ "friday" (obviously!)

**Task 8: task-should-be-skipped-4 - SKIPPED**
```yaml
- name: task-should-be-skipped-4
  when:
    - input: "master"
      operator: in
      values: ["$(params.branches[*])"]
```
When Expression: "Run ONLY if 'master' is in the branches array"
Result: SKIPPED because branches = ["main", "hotfix"] (no "master")

**Finally Tasks**
Finally Tasks are special - they ALWAYS run at the end, regardless of whether previous Tasks succeeded or failed. But they can still use when expressions!

**finally-task-should-be-skipped-1 - SKIPPED**
Checks if echo-file-exists failed (it didn't - it succeeded)

**finally-task-should-be-skipped-2 - SKIPPED**
Checks if file is missing (it's not - it exists)

**finally-task-should-be-skipped-3 - SKIPPED**
Checks if path is NOT README.md (it is README.md)

**finally-task-should-be-skipped-4 - SKIPPED**
Checks if overall status is "Failure" (it's not - tasks succeeded)

**finally-task-should-be-skipped-5 - SKIPPED**
Checks if overall status is "Succeeded" (it's "Completed", not "Succeeded")

**finally-task-should-be-executed - RUNS**
When you have multiple when expressions, ALL must be true (AND logic):

```yaml
when:
  - condition1  # Must be true
  - condition2  # AND this must be true
  - condition3  # AND this must be true
```
The example:

```yaml
- name: finally-task-should-be-executed
  when:
    - input: "$(tasks.echo-file-exists.status)"
      operator: in
      values: ["Succeeded"]
    - input: "$(tasks.status)"
      operator: in
      values: ["Completed"]
    - input: "$(tasks.check-file.results.exists)"
      operator: in
      values: ["yes"]
    - input: "$(params.path)"
      operator: in
      values: ["README.md"]
```
Multiple When Expressions: ALL conditions must be true (AND logic).

```Result: RUNS because:
echo-file-exists succeeded 
Overall status is "Completed" 
File exists ("yes") 
Path is "README.md" 
```
### [using-optional-workspaces-in-when-expressions.yaml](using-optional-workspaces-in-when-expressions.yaml)

Shows how to use workspace binding status in when expressions to conditionally execute Tasks based on whether optional workspaces are provided.
The key concept is the variable `$(workspaces.<name>.bound)`, which is a boolean indicating whether the named workspace is attached (true) or not (false). The Task `print-motd` runs only if the workspace is attached. The Task `print-default provided` runs only if the workspace is not provided.


### [pipelinerun-with-parallel-tasks-using-pvc.yaml](pipelinerun-with-parallel-tasks-using-pvc.yaml)

This example shows how both sequential and parallel Tasks can share data using a PersistentVolumeClaim as a workspace. A PipelineRun will pass a message parameter to the Pipeline in this example. The STARTER Task will write the message to a file in the workspace. The Tasks can run at the same time if they don't depend on each other. The UPPER and LOWER Tasks will execute in parallel and process the message written by the STARTER, and transform it to upper case and lower case. The REPORTER Task will use the Task Result from the UPPER Task and print it - it is intended to mimic a Task that sends data to an external service and shows a Task that doesn't use a workspace. The VALIDATOR Task will validate the result from UPPER and LOWER.

The runAfter property in a Pipeline is used to configure that a Task depends on another Task (Sequential execution). Output can be shared both via Task Result (e.g. like REPORTER Task) or via files in a workspace.

Pipeline Structure:
```
             -- (upper) -- (reporter)
           /                         \
  (starter)                           (validator)
           \                         /
             -- (lower) ------------

```

### [pipelinerun-task-execution-status.yaml](pipelinerun-task-execution-status.yaml)

Shows how to access and use Task execution status in when expressions and subsequent Tasks. Demonstrates:
- Checking if a Task succeeded or failed
- Using execution status in conditional logic `when`
- Aggregate execution status (`$(tasks.status)`)

The PipelineRun `kind: PipelineRun` directly runs a Pipeline defined inline using `pipelineSpec:`. The first Task simply prints "Hello world!", so it runs successfully resulting in status "Succeeded". The second Task `task2` has a `when` condition which means run task2 only if "true" is NOT in ["true"]. So the condition fails and task2 does not run resulting in status `None`.
```yaml
      when:
        - input: "true"
          operator: "notin"
          values: ["true"]
```


---


## Final Tasks

### [pipelinerun-with-final-tasks.yaml](pipelinerun-with-final-tasks.yaml)

Demonstrates finally Tasks that always execute after all Pipeline Tasks complete, regardless of success or failure. This example shows:
- Defining finally Tasks in a Pipeline
- Cleanup operations in finally Tasks
- Accessing Task results in finally Tasks
- Finally Tasks always run even if Pipeline Tasks fail

The example clones a Git repository and then cleans up the workspace in a finally Task, ensuring cleanup happens regardless of the Pipeline outcome.


### [pipelinerun-with-final-results.yaml](pipelinerun-with-final-results.yaml)

Demonstrates how Tekton Pipelines pass results between Tasks and also return results from both the `tasks` section and the `finally` section. The Pipeline performs two mathematical operations using the same reusable Task `multiply`.
1. Multiply two numbers (`multiply-inputs` Task)
Parameters:
```yaml
  params:
    - name: a
      type: string
      default: "1"
    - name: b
      type: string
      default: "1"
```

Result:
The container executes multiplication logic and stores the result in `product`.
```yaml
  results:
    - name: product
      description: The product of the two provided integers
```
The Pipeline exposes this Task result as `task-result`.

2. Use the result to compute another multiplication in the `finally` section (`exponent` Task)

The Pipeline takes the previous result stored in product and repeats it (2 becomes 22).
```yaml
"$(tasks.multiply-inputs.results.product)$(tasks.multiply-inputs.results.product)"
```
Then it multiplies a and b using reusable logic `multiply`.


---

## Timeouts and Retries

### [pipelinerun-with-task-timeout-override.yaml](pipelinerun-with-task-timeout-override.yaml)

Demonstrates how a Tekton Task timeout works and how a `PipelineRun` can override a timeout defined in the `Pipeline`. This example shows:
**sleep-task**
This means that the container will wait for 10 seconds before finishing. This is to simulate a long running task.
```yaml
kind: Task
metadata:
  name: sleep-task
spec:
  steps:
  - name: sleep-long
    image: mirror.gcr.io/alpine
    script: |
      #!/bin/sh
      sleep 10

```
**Setting timeouts in Pipeline**

This means that the Task should fail if it runs longer than 60 seconds.
```yaml
  - name: long-running-task
    taskRef:
      name: sleep-task
      kind: Task
    timeout: "60s"  
```

**Overriding timeouts in PipelineRun**
PipelineRun can override the timeout defined in the `Pipeline` by setting the `timeout` field. This overrides the Pipeline timeout. Instead of the 60 seconds, the `PipelineRun` will fail if the Task runs longer than 40 seconds.

```yaml
  pipelineRef:
    name: taskrun-timeout-override
  taskRunSpecs:
  - pipelineTaskName: long-running-task
    timeout: "40s"      # 40s timeout (accounts for image pull time + 10s sleep)

```


---

## StepActions

### [stepaction-params.yaml](stepaction-params.yaml)

Demonstrates using StepActions in a Pipeline context, showing how reusable step definitions can be used across multiple Tasks in a Pipeline.
A StepAction is like a templated step that can be reused inside tasks.
```yaml
apiVersion: tekton.dev/v1alpha1
kind: StepAction
metadata:
  name: step-action
```
It defines three parameters:

| Parameter | Type | Default Value |
|:-------------:|:--------------:|:--------------:|
| string-param	| string	| "a string param" |
| array-param	| array	| [ "an", "array", "param" ] |
| object-param	| object	| { key1: ..., key2: ..., key3: ... } |

**StepAction args**
```yaml
 args: [
  "echo",
  "$(params.array-param[*])",
  "$(params.string-param)",
  "$(params.object-param.key1)",
  "$(params.object-param.key2)"
]
```

$(params.array-param[*]) - expands all array items separated by space.

$(params.string-param) - inserts the string parameter.

$(params.object-param.key1) - inserts a specific key from the object.

$(params.object-param.key2) - inserts another key.

**PipelineRun: step-action-pipeline-run-propagated**

This PipelineRun overrides some parameters and propagates them to the StepAction.
```yaml
params:
  - name: stringparam
    value: "pipelinerun stringparam"
  - name: arrayparam
    value: ["pipelinerun", "array", "param"]
  - name: objectparam
    value:
      key2: "pipelinerun key2"
```
Key points:

- Only key2 is overridden in the object. The other keys use the StepAction default values.
- The array and string parameters are overridden completely.

**Propagation is done by:**

- Array items expand to: pipelinerun array param
  - array-param in StepAction -> receives the PipelineRun arrayparam array.
- String parameter: pipelinerun stringparam
  - string-param in StepAction -> receives the PipelineRun stringparam value.
- Object keys: key1 default, key2 overridden
- object-param in StepAction -> merges PipelineRun object with StepAction defaults:
  - key2 = "pipelinerun key2" (overridden)
  - key1 = "step-action default key1" (default)
  - key3 = "step-action default key3" (default)

---

## Context Variables

### [using_context_variables.yaml](using_context_variables.yaml)

Shows how to use built-in context variables in Pipelines, including:
- `$(context.pipelineRun.name)` - PipelineRun name
- `$(context.pipelineRun.namespace)` - PipelineRun namespace
- `$(context.pipelineRun.uid)` - PipelineRun unique ID
- `$(context.pipeline.name)` - Pipeline name
- Task-level context variables


---

## Error Handling

### [ignore-step-error.yaml](ignore-step-error.yaml)

Demonstrates how to configure steps to continue execution even when they fail, using the `onError: continue` field. The `onError: continue` field tells Tekton to continue executing subsequent steps even though this step exits with code 1 (failure). Without this field, the PipelineRun would fail immediately and the remaining pipeline would never execute. This allows Pipelines to handle non-critical failures gracefully.


---

## Display Names

### [pipeline-with-displayname.yaml](pipeline-with-displayname.yaml)

Shows how to use display names for better readability in UI and logs. Display names provide human-friendly names while maintaining technical identifiers.
#### At the Task level
```yaml
spec:
  displayName: Sum the two provided integers
```
The task sum has a friendly display name. When viewed in Tekton Dashboard or CLI, this is what users will see instead of just sum.

#### Step inside the task
```yaml
  steps:
    - name: sum
      displayName: Do the sum
```
Each step can also have a display name. Example: Do the sum is more readable than just sum. Helpful when tasks have multiple steps—users can quickly identify what each step does.

#### At the Pipeline level
```yaml
  name: sum-pipeline
spec:
  displayName: Sum Multiple Numbers
  description: Calculate the sum of multiple numbers
```
displayName gives the pipeline a friendly, descriptive name.
description provides more details (optional).
Useful for dashboards, reporting, or when multiple pipelines exist.

#### At the Pipeline task level
```yaml
  tasks:
    - name: sum-two-numbers
      displayName: "Calculate the first two numbers $(params.a) + $(params.b)"
      description: Calculate the sum of the first two numbers
```
Here, displayName can include parameters using Tekton variable syntax.

#### At the finally task level
```yaml
  finally:
    - name: print-result
      displayName: Print the result
      description: Print the final result
```
Makes it clear in logs that this step is for printing the final result.
Again, doesn’t affect execution, just improves readability.



---


## Additional example sets

- Alpha Features
  - [alpha/pipelinerun-large-results.yaml](alpha/pipelinerun-large-results.yaml)
  - [alpha/pipelinerun-with-cel-when-expressions.yaml](alpha/pipelinerun-with-cel-when-expressions.yaml)
  - [alpha/consume-artifacts-from-task.yaml](alpha/consume-artifacts-from-task.yaml)
  - [alpha/param-enum.yaml](alpha/param-enum.yaml)
- Beta Features
  - [beta/pipelinerun-with-matrix-emitting-results.yaml](beta/pipelinerun-with-matrix-emitting-results.yaml)
  - [beta/pipelinerun-with-matrix-and-results.yaml](beta/pipelinerun-with-matrix-and-results.yaml)
  - [beta/pipelinerun-with-matrix-context-variables.yaml](beta/pipelinerun-with-matrix-context-variables.yaml)
  - [beta/pipelinerun-with-matrix-include.yaml](#betapipelinerun-with-matrix-includeyaml)
  - [beta/pipelinerun-with-matrix-include-explicit.yaml](#betapipelinerun-with-matrix-include-explicityaml)
  - [beta/pipelinerun-with-matrix-array-references.yaml](#betapipelinerun-with-matrix-array-referencesyaml)
  - [beta/pipelinerun-with-matrix-and-taskrunspecs-param-substitution.yaml](#betapipelinerun-with-matrix-and-taskrunspecs-param-substitutionyaml)
  - [beta/pipeline-emitting-results.yaml](#betapipeline-emitting-resultsyaml)
  - [beta/pipeline-with-when-expressions-using-array-results.yaml](#betapipeline-with-when-expressions-using-array-resultsyaml)
  - [beta/isolated-workspaces.yaml](#betaisolated-workspacesyaml)
  - [beta/ignore-task-error.yaml](#betaignore-task-erroryaml)
  - [beta/propagated-pipeline-object-param.yaml](#betapropagated-pipeline-object-paramyaml)
  - [beta/git-resolver.yaml](#betagit-resolveryaml)
  - [beta/http-resolver.yaml](#betahttp-resolveryaml)
  - [beta/http-resolver-credentials.yaml](#betahttp-resolver-credentialsyaml)
  - [beta/7392-regression.yaml](#beta7392-regressionyaml)
- [Regression Tests](#regression-tests)
  - [4808-regression.yaml](#4808-regressionyaml)
  - [6139-regression.yaml](#6139-regressionyaml)

---

## Additional Resources

- [Tekton Pipelines Documentation](https://tekton.dev/docs/pipelines/)
- [PipelineRun Documentation](https://tekton.dev/docs/pipelines/pipelineruns/)
- [Pipeline Documentation](https://tekton.dev/docs/pipelines/pipelines/)
- [Tekton CLI (tkn)](https://tekton.dev/docs/cli/)