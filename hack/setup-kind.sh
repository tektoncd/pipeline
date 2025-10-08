#!/usr/bin/env bash

# Copyright 2021 The Tekton Authors
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# Attribution:
# Adapted for Tekton from https://github.com/mattmoor/mink/blob/master/hack/setup-kind.sh

set -o errexit
set -o nounset
set -o pipefail
set -x

# Print error message and exit 1
# Parameters: $1..$n - error message to be displayed
function abort() {
  echo "error: $*"
  exit 1
}

# Defaults
K8S_VERSION="v1.30.x"
REGISTRY_NAME="registry.local"
REGISTRY_PORT="5000"
CLUSTER_SUFFIX="cluster.local"
NODE_COUNT="1"
REGISTRY_AUTH="0"
ESTARGZ_SUPPORT="0"
E2E_SCRIPT="test/e2e-tests.sh"
E2E_ENV=""

while [[ $# -ne 0 ]]; do
  parameter="$1"
  case "${parameter}" in
    --k8s-version)
      shift
      K8S_VERSION="$1"
      ;;
    --registry-url)
      shift
      REGISTRY_NAME="$(echo "$1" | cut -d':' -f 1)"
      REGISTRY_PORT="$(echo "$1" | cut -d':' -f 2)"
      ;;
    --cluster-suffix)
      shift
      CLUSTER_SUFFIX="$1"
      ;;
    --nodes)
      shift
      NODE_COUNT="$1"
      ;;
    --authenticated-registry)
      REGISTRY_AUTH="1"
      ;;
    --e2e-script)
      shift
      E2E_SCRIPT="$1"
      ;;
    --e2e-env)
      shift
      E2E_ENV="$1"
      ;;
    *) abort "unknown option ${parameter}" ;;
  esac
  shift
done

# If E2E_ENV is set but the file doesn't exist, fall back on the old approach of invoking presubmit-tests.sh directly.
if [[ "${E2E_ENV}" != "" && ! -f "${E2E_ENV}" ]]; then
  ./test/presubmit-tests.sh --integration-tests
  exit $?
fi

# The version map correlated with this version of KinD
case ${K8S_VERSION} in
  v1.28.x)
    K8S_VERSION="1.28.9"
    KIND_IMAGE_SHA="sha256:dca54bc6a6079dd34699d53d7d4ffa2e853e46a20cd12d619a09207e35300bd0"
    KIND_IMAGE="kindest/node:${K8S_VERSION}@${KIND_IMAGE_SHA}"
    ;;
  v1.29.x)
    K8S_VERSION="1.29.14"
    KIND_IMAGE_SHA="sha256:8703bd94ee24e51b778d5556ae310c6c0fa67d761fae6379c8e0bb480e6fea29"
    KIND_IMAGE="kindest/node:${K8S_VERSION}@${KIND_IMAGE_SHA}"
    ;;
  v1.30.x)
    K8S_VERSION="1.30.13"
    KIND_IMAGE_SHA="sha256:397209b3d947d154f6641f2d0ce8d473732bd91c87d9575ade99049aa33cd648"
    KIND_IMAGE="kindest/node:${K8S_VERSION}@${KIND_IMAGE_SHA}"
    ;;
  v1.31.x)
    K8S_VERSION="1.31.12"
    KIND_IMAGE_SHA="sha256:0f5cc49c5e73c0c2bb6e2df56e7df189240d83cf94edfa30946482eb08ec57d2"
    KIND_IMAGE="kindest/node:${K8S_VERSION}@${KIND_IMAGE_SHA}"
    ;;
  v1.32.x)
    K8S_VERSION="1.32.8"
    KIND_IMAGE_SHA="sha256:abd489f042d2b644e2d033f5c2d900bc707798d075e8186cb65e3f1367a9d5a1"
    KIND_IMAGE="kindest/node:${K8S_VERSION}@${KIND_IMAGE_SHA}"
    ;;
  v1.33.x)
    K8S_VERSION="1.33.4"
    KIND_IMAGE_SHA="sha256:0f5cc49c5e73c0c2bb6e2df56e7df189240d83cf94edfa30946482eb08ec57d2"
    KIND_IMAGE="kindest/node:${K8S_VERSION}@${KIND_IMAGE_SHA}"
    ;;
  v1.34.x)
    K8S_VERSION="1.34.0"
    KIND_IMAGE_SHA="sha256:7416a61b42b1662ca6ca89f02028ac133a309a2a30ba309614e8ec94d976dc5a"
    KIND_IMAGE="kindest/node:${K8S_VERSION}@${KIND_IMAGE_SHA}"
    ;;
  *) abort "Unsupported version: ${K8S_VERSION}" ;;
esac

#############################################################
#
#    Setup KinD cluster.
#
#############################################################
echo '--- Setup KinD Cluster'

cat > kind.yaml <<EOF
apiVersion: kind.x-k8s.io/v1alpha4
kind: Cluster
nodes:
- role: control-plane
  image: "${KIND_IMAGE}"
  labels:
    node-type: "control-plane"
EOF

for i in $(seq 1 1 "${NODE_COUNT}");
do
  cat >> kind.yaml <<EOF
- role: worker
  image: "${KIND_IMAGE}"
  labels:
    node-type: "worker-${i}"
EOF
done

function containerd_config() {
  # The bulk of this is to enable stargz support:
  # https://github.com/containerd/stargz-snapshotter/blob/v0.2.0/README.md#quick-start-with-kubernetes
  if [[ "${ESTARGZ_SUPPORT}" = "1" ]] ; then
    cat <<EOF
  # Plug stargz snapshotter into containerd
  # Containerd recognizes stargz snapshotter through specified socket address.
  # The specified address below is the default which stargz snapshotter listen to.
  [proxy_plugins]
    [proxy_plugins.stargz]
      type = "snapshot"
      address = "/run/containerd-stargz-grpc/containerd-stargz-grpc.sock"

  # Use stargz snapshotter through CRI
  [plugins."io.containerd.grpc.v1.cri".containerd]
    snapshotter = "stargz"
    disable_snapshot_annotations = false
EOF
  return
  fi

  # Default configuration
  cat <<EOF
  [plugins."io.containerd.grpc.v1.cri".containerd]
    # Support many layered images: https://kubernetes.slack.com/archives/CEKK1KTN2/p1602770111199000
    disable_snapshot_annotations = true
EOF
}

cat >> kind.yaml <<EOF
kubeadmConfigPatches:
  # This is needed in order to support projected volumes with service account tokens.
  # See: https://kubernetes.slack.com/archives/CEKK1KTN2/p1600268272383600
  - |
    apiVersion: kubeadm.k8s.io/v1beta2
    kind: ClusterConfiguration
    metadata:
      name: config
    apiServer:
      extraArgs:
        "service-account-issuer": "kubernetes.default.svc"
        "service-account-signing-key-file": "/etc/kubernetes/pki/sa.key"
    networking:
      dnsDomain: "${CLUSTER_SUFFIX}"

  # This is needed to avoid filling our disk.
  # See: https://kubernetes.slack.com/archives/CEKK1KTN2/p1603391142276400
  - |
    kind: KubeletConfiguration
    metadata:
      name: config
    imageGCHighThresholdPercent: 90

containerdConfigPatches:
- |-
$(containerd_config)

  # Support a local registry
  [plugins."io.containerd.grpc.v1.cri".registry.mirrors."$REGISTRY_NAME:$REGISTRY_PORT"]
    endpoint = ["http://$REGISTRY_NAME:$REGISTRY_PORT"]
EOF

echo '--- kind.yaml'
cat kind.yaml

# Check the version of kind
kind --version

# Check we can talk to docker
docker ps

# Create a cluster!
kind create cluster --config kind.yaml

#############################################################
#
#    Setup metallb
#
#############################################################
echo '--- Setup metallb'

kubectl apply -f https://raw.githubusercontent.com/metallb/metallb/v0.13.10/config/manifests/metallb-native.yaml
kubectl create secret generic -n metallb-system memberlist --from-literal=secretkey="$(openssl rand -base64 128)"

network=$(docker network inspect kind -f "{{(index .IPAM.Config 0).Subnet}}" | cut -d '.' -f1,2)
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: ConfigMap
metadata:
  namespace: metallb-system
  name: config
data:
  config: |
    address-pools:
    - name: default
      protocol: layer2
      addresses:
      - $network.255.1-$network.255.250
EOF

#############################################################
#
#    Setup container registry
#
#############################################################
echo '--- Setup container registry'

EXTRA_ARGS=()
if [[ "${REGISTRY_AUTH}" == "1" ]]; then
  # Configure Auth
  USERNAME="user-${RANDOM}"
  PASSWORD="pass-${RANDOM}"

  AUTH_DIR=$(mktemp -d)

  # Docker removed htpasswd in a patch release, so pin to 2.7.0 so this works.
  docker run \
	 --entrypoint htpasswd \
	 registry:2.7.0 -Bbn "${USERNAME}" "${PASSWORD}" > "${AUTH_DIR}/htpasswd"

  # Run a registry protected with htpasswd
  EXTRA_ARGS=(
    -v "${AUTH_DIR}:/auth"
    -e "REGISTRY_AUTH=htpasswd"
    -e "REGISTRY_AUTH_HTPASSWD_REALM=Registry Realm"
    -e "REGISTRY_AUTH_HTPASSWD_PATH=/auth/htpasswd"
  )

fi

docker run -d --restart=always \
       "${EXTRA_ARGS[@]}" \
       -p "$REGISTRY_PORT:$REGISTRY_PORT" --name "$REGISTRY_NAME" registry:2

# Connect the registry to the KinD network.
docker network connect "kind" "$REGISTRY_NAME"

# Make the $REGISTRY_NAME -> 127.0.0.1, to tell `ko` to publish to
# local reigstry, even when pushing $REGISTRY_NAME:$REGISTRY_PORT/some/image
echo "127.0.0.1 $REGISTRY_NAME" | sudo tee -a /etc/hosts

# Create a registry-credentials secret and attach it to the list of service accounts in the namespace.
function sa_ips() {
  local ns="${1}"
  shift

  # Create a secret resource with the contents of the docker auth configured above.
  kubectl -n "${ns}" create secret generic registry-credentials \
	  --from-file=.dockerconfigjson=${HOME}/.docker/config.json \
	  --type=kubernetes.io/dockerconfigjson

  for sa in "${@}" ; do
    # Ensure the service account exists.
    kubectl -n "${ns}" create serviceaccount "${sa}" || true

    # Attach the secret resource to the service account in the namespace.
    kubectl -n "${ns}" patch serviceaccount "${sa}" -p '{"imagePullSecrets": [{"name": "registry-credentials"}]}'
  done
}

if [[ "${REGISTRY_AUTH}" == "1" ]]; then

  # This will create ~/.docker/config.json
  docker login "http://$REGISTRY_NAME:$REGISTRY_PORT/v2/" -u "${USERNAME}" -p "${PASSWORD}"

  sa_ips "default" "default"
fi

export KO_DOCKER_REPO=kind.local

echo '--- Debug file system'
pwd
ls -al


if [[ "${E2E_SCRIPT}" == "" ]]; then
  echo "Nothing else to do"
  exit 0
else
  if [[ "${E2E_ENV}" != "" ]]; then
    set -o allexport
    source "${E2E_ENV}"
    set +o allexport
  fi
  "${E2E_SCRIPT}"
fi
