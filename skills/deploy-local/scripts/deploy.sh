#!/usr/bin/env bash
set -euo pipefail

# Deploy Tekton Pipelines images to a local Kind cluster.
# Usage: deploy.sh <component> [component...]
# Components: controller, webhook, resolvers, events
#
# Example:
#   deploy.sh controller
#   deploy.sh controller webhook

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
cd "${REPO_ROOT}"

declare -A DEPLOYMENT_MAP=(
  [controller]="tekton-pipelines-controller"
  [webhook]="tekton-pipelines-webhook"
  [resolvers]="tekton-pipelines-remote-resolvers"
  [events]="tekton-events-controller"
)

declare -A CONTAINER_MAP=(
  [controller]="tekton-pipelines-controller"
  [webhook]="webhook"
  [resolvers]="controller"
  [events]="tekton-events-controller"
)

if [[ $# -eq 0 ]]; then
  echo "Usage: $0 <component> [component...]"
  echo "Components: ${!DEPLOYMENT_MAP[*]}"
  exit 1
fi

for comp in "$@"; do
  if [[ -z "${DEPLOYMENT_MAP[$comp]:-}" ]]; then
    echo "ERROR: Unknown component '$comp'. Valid: ${!DEPLOYMENT_MAP[*]}"
    exit 1
  fi
done

# Detect container runtime and socket
detect_runtime() {
  # Podman: check common socket locations (Linux, then Mac)
  if command -v podman &>/dev/null; then
    local linux_sock="/run/user/$(id -u)/podman/podman.sock"
    local mac_sock="${HOME}/.local/share/containers/podman/machine/podman.sock"
    if [ -S "${linux_sock}" ]; then
      export DOCKER_HOST="unix://${linux_sock}"
    elif [ -S "${mac_sock}" ]; then
      export DOCKER_HOST="unix://${mac_sock}"
    else
      # Try podman info to find the socket
      local sock
      sock=$(podman info --format '{{.Host.RemoteSocket.Path}}' 2>/dev/null || true)
      if [ -n "${sock}" ] && [ -S "${sock}" ]; then
        export DOCKER_HOST="unix://${sock}"
      fi
    fi
    RUNTIME="podman"
    echo "Detected Podman"
    return
  fi

  # Docker: available if the command exists (Docker Desktop handles the socket)
  if command -v docker &>/dev/null; then
    RUNTIME="docker"
    echo "Detected Docker"
    return
  fi

  echo "ERROR: Neither podman nor docker found"
  exit 1
}
detect_runtime

# Detect Kind cluster from current context
CURRENT_CTX=$(kubectl config current-context)
if [[ "${CURRENT_CTX}" == kind-* ]]; then
  KIND_CLUSTER_NAME="${CURRENT_CTX#kind-}"
else
  echo "ERROR: Current context '${CURRENT_CTX}' doesn't look like a Kind cluster (expected kind-<name>)"
  exit 1
fi
export KIND_CLUSTER_NAME
echo "Using Kind cluster: ${KIND_CLUSTER_NAME}"

# Ensure ko is available
if ! command -v ko &>/dev/null; then
  echo "Installing ko..."
  go install github.com/google/ko@latest
fi

export KO_DOCKER_REPO="kind.local"

# If using podman with kind, set the provider
if [[ "${RUNTIME}" == "podman" ]]; then
  export KIND_EXPERIMENTAL_PROVIDER=podman
fi

# Fix kodata symlinks
FIXED_KODATA=()
for comp in "$@"; do
  KODATA="cmd/${comp}/kodata"
  if [ -d "${KODATA}" ] && [ -L "${KODATA}/LICENSE" ]; then
    rm "${KODATA}/LICENSE"
    cp LICENSE "${KODATA}/LICENSE"
    FIXED_KODATA+=("${comp}")
    echo "Fixed kodata symlink for ${comp}"
  fi
done

restore_kodata() {
  for comp in "${FIXED_KODATA[@]}"; do
    KODATA="cmd/${comp}/kodata"
    if [ -f "${KODATA}/LICENSE" ] && [ ! -L "${KODATA}/LICENSE" ]; then
      rm "${KODATA}/LICENSE"
      ln -s ../../../LICENSE "${KODATA}/LICENSE"
      echo "Restored kodata symlink for ${comp}"
    fi
  done
}
trap restore_kodata EXIT

# Load an image into the Kind cluster.
# Docker Desktop: kind can pull directly from the daemon.
# Podman: save to archive, then load.
load_image() {
  local image="$1"
  local comp="$2"
  if [[ "${RUNTIME}" == "docker" ]]; then
    kind load docker-image "${image}" --name "${KIND_CLUSTER_NAME}"
  else
    local archive="/tmp/tekton-${comp}-$$.tar"
    ${RUNTIME} save "${image}" -o "${archive}"
    kind load image-archive "${archive}" --name "${KIND_CLUSTER_NAME}"
    rm -f "${archive}"
  fi
}

# Build and load each component
declare -A IMAGES=()
for comp in "$@"; do
  echo ""
  echo "=== Building ${comp} ==="
  IMAGE=$(ko build --local --sbom=none --bare "./cmd/${comp}" 2>&1 | tail -1)
  echo "Built: ${IMAGE}"

  echo "Loading into Kind cluster..."
  load_image "${IMAGE}" "${comp}"

  IMAGES[${comp}]="${IMAGE}"
  echo "Loaded ${comp} into Kind"
done

# Scale down operator
echo ""
echo "=== Scaling down Tekton Operator ==="
kubectl scale deploy tekton-operator -n tekton-operator --replicas=0
echo "Operator scaled to 0"

# Patch deployments
for comp in "$@"; do
  DEPLOY="${DEPLOYMENT_MAP[$comp]}"
  CONTAINER="${CONTAINER_MAP[$comp]}"
  IMAGE="${IMAGES[$comp]}"

  echo ""
  echo "=== Deploying ${comp} ==="
  kubectl set image "deployment/${DEPLOY}" -n tekton-pipelines "${CONTAINER}=${IMAGE}"
  kubectl patch deployment "${DEPLOY}" -n tekton-pipelines \
    --type='json' \
    -p='[{"op": "replace", "path": "/spec/template/spec/containers/0/imagePullPolicy", "value": "Never"}]'
  echo "Waiting for rollout..."
  kubectl rollout status "deployment/${DEPLOY}" -n tekton-pipelines --timeout=90s
  echo "Deployed ${comp} successfully"
done

echo ""
echo "=== Done ==="
echo "To restore original state: kubectl scale deploy tekton-operator -n tekton-operator --replicas=1"
