# v1 TaskRun examples

This directory contains `TaskRun` examples for Tekton Pipelines `v1`.
They cover basic task execution, params/results, workspaces, sidecars, step templates,
security settings, and regression-focused cases.

## Prerequisites

- A Kubernetes cluster
- Tekton Pipelines installed (v1 CRDs/controller)
- `kubectl` configured for your cluster
- Any required feature flags enabled (for examples under `alpha/` and `beta/`)

## How to run examples

Most files here use `metadata.generateName`, so use:

```bash
kubectl create -f examples/v1/taskruns/<file>.yaml
```

If an example uses a fixed `metadata.name`, `kubectl apply -f ...` also works.

## Example groups

### Basic examples

- `array-default.yaml`
- `default_task_params.yaml`
- `entrypoint-resolution.yaml`
- `steps-run-in-order.yaml`
- `workingdir.yaml`
- `unnamed-steps.yaml`

### Parameters and results

- `task-result.yaml`
- `object-param-result.yaml`
- `propagating_params_implicit.yaml`
- `stepaction-passing-results.yaml`
- `stepaction-results.yaml`

### Workspaces

- `workspace.yaml`
- `workspace-volume.yaml`
- `workspace-with-volumeClaimTemplate.yaml`
- `workspace-readonly.yaml`
- `workspace-in-sidecar.yaml`
- `optional-workspaces.yaml`
- `propagating_workspaces.yaml`

### StepActions / step templates

- `stepaction.yaml`
- `stepaction-params.yaml`
- `stepactions-steptemplate.yaml`
- `step-script.yaml`
- `steptemplate-env-merge.yaml`
- `steptemplate-variable-interop.yaml`

### Sidecars

- `dind-sidecar.yaml`
- `sidecar-ready.yaml`
- `sidecar-ready-script.yaml`
- `sidecar-interp.yaml`

### Volumes, env and credentials

- `configmap.yaml`
- `custom-env.yaml`
- `custom-volume.yaml`
- `template-volume.yaml`
- `task-volume-args.yaml`
- `secret-volume.yaml`
- `secret-volume-params.yaml`
- `secret-env.yaml`
- `creds-init-only-mounts-provided-credentials.yaml`

### Security / runtime behavior

- `run-steps-as-non-root.yaml`
- `readonly-internal-dir.yaml`
- `home-is-set.yaml`
- `home-volume.yaml`
- `ignore-step-error.yaml`
- `step-by-digest.yaml`
- `using_context_variables.yaml`

### Regression-focused examples

- `5080-entrypoint-init-regression.yaml`

### Extra example sets

- `alpha/` — examples requiring alpha features
- `beta/` — examples requiring beta features
- `no-ci/` — examples not used in CI
