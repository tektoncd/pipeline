# v1 PipelineRun examples

This directory contains `PipelineRun` examples for Tekton Pipelines `v1`.
They show how to run pipelines with parameters, workspaces, results, `when` expressions,
final tasks, and a few regression-focused scenarios.

## Prerequisites

- A Kubernetes cluster
- Tekton Pipelines installed (v1 CRDs/controller)
- `kubectl` configured for your cluster
- Any required feature flags enabled (for examples under `alpha/` and `beta/`)

## How to run examples

Most examples use `metadata.generateName`, so create them with:

```bash
kubectl create -f examples/v1/pipelineruns/<file>.yaml
```

If an example uses a fixed `metadata.name`, `kubectl apply -f ...` also works.

## Example groups

### Basic / execution flow

- `pipelinerun.yaml`
- `pipelinerun-with-pipelinespec.yaml`
- `pipelinerun-with-pipelinespec-and-taskspec.yaml`
- `pipelinerun-with-params.yaml`
- `pipelinerun-with-extra-params.yaml`
- `pipelinerun-with-final-tasks.yaml`
- `pipelinerun-with-task-timeout-override.yaml`
- `pipelinerun-with-parallel-tasks-using-pvc.yaml`

### Workspaces

- `workspaces.yaml`
- `workspaces-projected.yaml`
- `mapping-workspaces.yaml`
- `optional-workspaces.yaml`
- `using-optional-workspaces-in-when-expressions.yaml`
- `workspace-from-volumeclaimtemplate.yaml`
- `propagating-workspaces.yaml`
- `propagating_workspaces_with_referenced_resources.yaml`
- `pipelinerun-using-different-subpaths-of-workspace.yaml`
- `pipelinerun-using-parameterized-subpath-of-workspace.yaml`

### Parameters and results

- `pipelinerun-results.yaml`
- `pipelinerun-results-with-params.yaml`
- `pipelinerun-array-results-substitution.yaml`
- `pipelinerun-param-array-indexing.yaml`
- `pipeline-object-results.yaml`
- `pipeline-object-param-and-result.yaml`
- `pipeline-with-displayname.yaml`
- `task_results_example.yaml`
- `propagating_params_in_pipeline.yaml`
- `propagating_params_implicit_parameters.yaml`
- `propagating_params_with_scope_precedence.yaml`
- `propagating_params_with_scope_precedence_default_only.yaml`
- `propagating_results_implicit_resultref.yaml`
- `pipelinerun-with-final-results.yaml`

### Conditions / control behavior

- `pipelinerun-with-when-expressions.yaml`
- `ignore-step-error.yaml`
- `pipelinerun-task-execution-status.yaml`
- `using_context_variables.yaml`

### StepActions

- `stepaction-params.yaml`

### Regression or compatibility examples

- `4808-regression.yaml`
- `6139-regression.yaml`

### Extra example sets

- `alpha/` — examples requiring alpha features
- `beta/` — examples requiring beta features
- `no-ci/` — examples not used in CI
