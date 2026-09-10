---
name: deploy-local
description: Build and deploy Tekton Pipelines images to a local Kind cluster for testing. Use when the user says "deploy locally", "test on cluster", "build and deploy", "deploy to kind", or wants to test code changes on a local Kubernetes cluster.
---

# Deploy to Local Kind Cluster

Build Tekton Pipelines images from the current branch and deploy them to a local Kind cluster managed by the Tekton Operator.

## Prerequisites

- `ko` installed (`go install github.com/google/ko@latest` if missing)
- A running Kind cluster with Tekton installed via the Tekton Operator
- `podman` or `docker` available

## Component Map

The repo builds these deployable microservices:

| cmd path | Deployment | Container name | Namespace |
|----------|-----------|----------------|-----------|
| `./cmd/controller` | `tekton-pipelines-controller` | `tekton-pipelines-controller` | `tekton-pipelines` |
| `./cmd/webhook` | `tekton-pipelines-webhook` | `webhook` | `tekton-pipelines` |
| `./cmd/resolvers` | `tekton-pipelines-remote-resolvers` | `controller` | `tekton-pipelines` |
| `./cmd/events` | `tekton-events-controller` | `tekton-events-controller` | `tekton-pipelines` |

Utility images (`entrypoint`, `nop`, `workingdirinit`, `sidecarlogresults`) are not deployed as pods — they're referenced by the controller at runtime. Deploying them requires updating the controller's config, which is out of scope here.

## Workflow

### Step 1: Determine which components to build

Inspect the code changes to decide which component(s) need rebuilding. Use this mapping:

- `pkg/reconciler/taskrun/` or `pkg/reconciler/pipelinerun/` → **controller**
- `pkg/apis/` → **controller** and **webhook**
- `pkg/reconciler/notifications/` or `cmd/events/` → **events**
- `pkg/resolution/` or `cmd/resolvers/` → **resolvers**
- `cmd/webhook/` → **webhook**
- `config/` (non-CRD) → may not need a rebuild, just `kubectl apply`

If unsure, rebuild **controller** and **webhook** — they cover most changes.

### Step 2: Detect container runtime

```bash
# Check for podman first, then docker
if [ -S "/run/user/$(id -u)/podman/podman.sock" ]; then
  export DOCKER_HOST="unix:///run/user/$(id -u)/podman/podman.sock"
fi
```

### Step 3: Detect Kind cluster

```bash
# List clusters and find the current context
kind get clusters
kubectl config current-context
# Extract cluster name from context (format: kind-<name>)
```

Set `KIND_CLUSTER_NAME` to the cluster name (without the `kind-` prefix).

### Step 4: Fix kodata symlinks

`ko` v0.19+ rejects symlinks that resolve outside the kodata root. For each component being built, temporarily replace the symlink with a real file copy:

```bash
for cmd_dir in controller webhook resolvers events; do
  KODATA="cmd/${cmd_dir}/kodata"
  if [ -L "${KODATA}/LICENSE" ]; then
    rm "${KODATA}/LICENSE"
    cp LICENSE "${KODATA}/LICENSE"
  fi
done
```

**Restore symlinks after building** (Step 7).

### Step 5: Build and load images

For each component, build with `ko` and load into Kind:

```bash
export KO_DOCKER_REPO="kind.local"

# Build (returns image ref like localhost/kind.local:<hash>)
IMAGE=$(ko build --local --sbom=none --bare ./cmd/<component> 2>&1 | tail -1)

# Save from podman/docker and load into Kind node
podman save "${IMAGE}" -o /tmp/tekton-<component>.tar
kind load image-archive /tmp/tekton-<component>.tar --name "${KIND_CLUSTER_NAME}"
rm /tmp/tekton-<component>.tar
```

Record the image ref for each component — you'll need it for patching.

### Step 6: Scale down operator and patch deployments

```bash
# Prevent the operator from reverting your changes
kubectl scale deploy tekton-operator -n tekton-operator --replicas=0

# For each component, patch the deployment
kubectl set image deployment/<deployment-name> \
  -n tekton-pipelines \
  <container-name>="<image-ref>"

# Set imagePullPolicy to Never (image is already on the node)
kubectl patch deployment <deployment-name> -n tekton-pipelines \
  --type='json' \
  -p='[{"op": "replace", "path": "/spec/template/spec/containers/0/imagePullPolicy", "value": "Never"}]'

# Wait for rollout
kubectl rollout status deployment/<deployment-name> -n tekton-pipelines --timeout=90s
```

### Step 7: Restore kodata symlinks

```bash
for cmd_dir in controller webhook resolvers events; do
  KODATA="cmd/${cmd_dir}/kodata"
  if [ -f "${KODATA}/LICENSE" ] && [ ! -L "${KODATA}/LICENSE" ]; then
    rm "${KODATA}/LICENSE"
    ln -s ../../../LICENSE "${KODATA}/LICENSE"
  fi
done
```

### Step 8: Verify

```bash
kubectl get pods -n tekton-pipelines -l app=<deployment-name>
```

Confirm pods are `Running` and `1/1 Ready`.

## Restoring Original State

When done testing, scale the operator back up — it will reconcile the deployments back to the release images:

```bash
kubectl scale deploy tekton-operator -n tekton-operator --replicas=1
```

## Troubleshooting

| Symptom | Cause | Fix |
|---------|-------|-----|
| `ErrImagePull` | Kind node can't pull `kind.local:*` | Set `imagePullPolicy: Never` |
| `ErrImageNeverPull` | Image not on node | Use `kind load image-archive` |
| kodata symlink error | `ko` rejects symlinks outside kodata root | Replace symlinks with file copies before build |
| Deployment reverts | Operator is running | Scale operator to 0 replicas |
| `dial unix docker.sock: no such file` | Using Podman, not Docker | Set `DOCKER_HOST` to Podman socket |
