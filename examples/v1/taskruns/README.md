# TaskRun Examples

This directory contains example **TaskRun** configurations demonstrating various Tekton Pipelines features. These examples illustrate how to run individual Tasks and showcase functionality such as parameters, workspaces, sidecars, secrets, StepActions, environment configuration, and execution behavior.

These files serve as reference implementations to help users understand how different TaskRun configurations work in practice.

---

# Example Categories

## Basic Examples

Simple TaskRun examples demonstrating basic execution behavior.

- `entrypoint-resolution.yaml`
- `steps-run-in-order.yaml`
- `step-script.yaml`
- `unnamed-steps.yaml`
- `workingdir.yaml`
- `using_context_variables.yaml`

---

## Parameters and Results

Examples demonstrating how parameters and results are used within Tasks and TaskRuns.

- `array-default.yaml`
- `object-param-result.yaml`
- `task-result.yaml`
- `propagating_params_implicit.yaml`
- `default_task_params.yaml`

---

## Environment and Configuration

Examples showing how environment variables and configuration are provided to tasks.

- `custom-env.yaml`
- `configmap.yaml`
- `secret-env.yaml`
- `secret-volume.yaml`
- `secret-volume-params.yaml`
- `home-is-set.yaml`
- `home-volume.yaml`

---

## Volumes and Filesystems

Examples demonstrating volume mounting and filesystem configuration.

- `custom-volume.yaml`
- `template-volume.yaml`
- `task-volume-args.yaml`
- `workspace-volume.yaml`

---

## Workspaces

Examples showing how Tekton workspaces are used to share data between steps and sidecars.

- `workspace.yaml`
- `workspace-readonly.yaml`
- `workspace-volume.yaml`
- `workspace-with-volumeClaimTemplate.yaml`
- `optional-workspaces.yaml`
- `propagating_workspaces.yaml`
- `workspace-in-sidecar.yaml`

---

## Sidecars

Examples demonstrating the use of sidecar containers alongside task steps.

- `dind-sidecar.yaml`
- `sidecar-interp.yaml`
- `sidecar-ready.yaml`
- `sidecar-ready-script.yaml`
- `workspace-in-sidecar.yaml`

---

## StepActions

Examples demonstrating the use of StepActions.

- `stepaction.yaml`
- `stepaction-params.yaml`
- `stepaction-results.yaml`
- `stepaction-passing-results.yaml`
- `stepactions-steptemplate.yaml`

---

## Step Templates and Execution Behavior

Examples demonstrating step templates and execution configuration.

- `steptemplate-env-merge.yaml`
- `steptemplate-variable-interop.yaml`
- `step-by-digest.yaml`

---

## Security and Permissions

Examples demonstrating security-related configurations.

- `run-steps-as-non-root.yaml`
- `authenticating-git-commands.yaml`
- `creds-init-only-mounts-provided-credentials.yaml`

---

## Error Handling

Examples demonstrating error handling and failure behavior.

- `ignore-step-error.yaml`

---

## Regression and Internal Validation

Examples primarily used for testing or regression validation.

- `5080-entrypoint-init-regression.yaml`
- `readonly-internal-dir.yaml`

---

# Running the Examples

Most examples in this directory define a `TaskRun` resource that can be created directly in your Kubernetes cluster.

If the file defines a fixed `metadata.name`, you can run it with:

```bash
kubectl apply -f <file>.yaml
```

Some examples instead use `metadata.generateName`. This allows the resource to be created multiple times with unique names.

For those files you should use:

```bash
kubectl create -f <file>.yaml
```

Using `kubectl apply` on resources that use `generateName` will fail because `apply` requires a fixed resource name.

After creating a TaskRun, you can inspect it with:

```bash
kubectl get taskruns
kubectl describe taskrun <name>
```

You can also stream logs for a TaskRun using the Tekton CLI:

```bash
tkn taskrun logs <name> -f
```

---

# Prerequisites

Before running these examples, ensure the following components are installed and configured:

## Kubernetes Cluster

You need access to a running Kubernetes cluster. This can be a local cluster (such as Kind or Minikube) or a managed cluster from a cloud provider.

## Tekton Pipelines

Tekton Pipelines must be installed in the cluster, as these examples rely on Tekton custom resources such as `Task`, `TaskRun`, and related pipeline components.

Installation instructions:

https://tekton.dev/docs/pipelines/install/

## kubectl

The Kubernetes CLI (`kubectl`) must be installed and configured to communicate with your cluster.

Documentation:

https://kubernetes.io/docs/tasks/tools/

## (Optional) Tekton CLI

The Tekton CLI (`tkn`) is useful for inspecting TaskRuns and streaming logs.

Installation instructions:

https://tekton.dev/docs/cli/

## Feature Flags

Some examples may require specific Tekton feature flags to be enabled depending on the functionality being demonstrated.

---

# Additional Resources

Tekton Pipelines documentation (https://tekton.dev/docs/pipelines/)

Tekton GitHub repository (https://github.com/tektoncd/pipeline)
