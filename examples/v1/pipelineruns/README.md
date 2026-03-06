# PipelineRun Examples

This directory contains example **PipelineRun** configurations demonstrating various features of Tekton Pipelines. These examples showcase how to define, configure, and run pipelines with different capabilities such as parameters, workspaces, results, conditional execution, and advanced execution patterns.

These files are intended as reference implementations to help users understand how different PipelineRun configurations work in practice.

---

# Example Categories

## Basic Examples

Simple PipelineRun examples demonstrating basic pipeline execution.

- `pipelinerun.yaml`
- `6139-regression.yaml`
- `using_context_variables.yaml`

---

## Parameters and Results

Examples demonstrating how parameters and results are passed between pipelines and tasks.

- `pipelinerun-with-params.yaml`
- `pipelinerun-with-extra-params.yaml`
- `pipelinerun-param-array-indexing.yaml`
- `pipelinerun-array-results-substitution.yaml`
- `pipelinerun-results.yaml`
- `pipelinerun-results-with-params.yaml`
- `pipeline-object-param-and-result.yaml`
- `pipeline-object-results.yaml`
- `task_results_example.yaml`
- `propagating_params_implicit_parameters.yaml`
- `propagating_params_in_pipeline.yaml`
- `propagating_params_with_scope_precedence.yaml`
- `propagating_params_with_scope_precedence_default_only.yaml`
- `propagating_results_implicit_resultref.yaml`

---

## Workspaces

Examples demonstrating how workspaces are used to share data between tasks.

- `workspaces.yaml`
- `workspaces-projected.yaml`
- `mapping-workspaces.yaml`
- `optional-workspaces.yaml`
- `propagating-workspaces.yaml`
- `propagating-workspaces-in-pipelines.yaml`
- `propagating_workspaces_with_referenced_resources.yaml`
- `pipelinerun-using-different-subpaths-of-workspace.yaml`
- `pipelinerun-using-parameterized-subpath-of-workspace.yaml`
- `workspace-from-volumeclaimtemplate.yaml`
- `pipelinerun-with-parallel-tasks-using-pvc.yaml`

---

## Conditional Execution (When Expressions)

Examples showing how to conditionally run tasks based on `when` expressions.

- `pipelinerun-with-when-expressions.yaml`
- `using-optional-workspaces-in-when-expressions.yaml`

---

## Pipeline Specification Examples

Examples demonstrating inline pipeline or task specifications.

- `pipelinerun-with-pipelinespec.yaml`
- `pipelinerun-with-pipelinespec-and-taskspec.yaml`

---

## Execution Behavior and Status

Examples illustrating pipeline execution behavior and reporting.

- `pipelinerun-task-execution-status.yaml`
- `ignore-step-error.yaml`

---

## Final Tasks and Results

Examples demonstrating the use of final tasks and pipeline results.

- `pipelinerun-with-final-results.yaml`
- `pipelinerun-with-final-tasks.yaml`

---

## StepActions

Examples demonstrating the use of StepActions.

- `stepaction-params.yaml`

---

## Display Name

Examples demonstrating display name usage.

- `pipeline-with-displayname.yaml`

---

## Regression / Edge Case Examples

Examples created to validate bug fixes or regression scenarios.

- `4808-regression.yaml`

---

## Prerequisites

Before running these examples, ensure the following components are installed and configured:

### Kubernetes Cluster

You need access to a running Kubernetes cluster. This can be a local cluster (e.g. Kind, Minikube) or a managed cluster from a cloud provider.

### Tekton Pipelines

Tekton Pipelines must be installed in the cluster, as these examples rely on Tekton custom resources such as `Pipeline`, `Task`, and `PipelineRun`.

Installation instructions:  
https://tekton.dev/docs/pipelines/install/

### kubectl

The Kubernetes CLI (`kubectl`) must be installed and configured to communicate with your cluster.

Documentation:  
https://kubernetes.io/docs/tasks/tools/

### (Optional) Tekton CLI

The Tekton CLI (`tkn`) is useful for inspecting PipelineRuns and streaming logs.

Installation instructions:  
https://tekton.dev/docs/cli/

### Feature Flags

Some examples may require specific Tekton feature flags to be enabled depending on the functionality being demonstrated.

---

## Running the Examples

Some examples in this directory define a `PipelineRun` resource that can be created directly in your Kubernetes cluster.

If the file defines a fixed `metadata.name`, you can run it with:

```bash
kubectl apply -f <file>.yaml
```

Some examples instead use `metadata.generateName`. This allows the resource to be created multiple times with unique names.

For these files you should use:

```bash
kubectl create -f <file>.yaml
```

Using `kubectl apply` on resources that use `generateName` will fail because apply requires a fixed resource name.

After creating a PipelineRun, you can inspect it with:

```bash
kubectl get pipelineruns
kubectl describe pipelinerun <name>
```

You can also view logs for a PipelineRun using the Tekton CLI (`tkn`):

```bash
tkn pipelinerun logs <name> -f
```

---

## Additional Resources

- Tekton Pipelines documentation (https://tekton.dev/docs/pipelines/)

- Tekton GitHub repository (https://github.com/tektoncd/pipeline)
