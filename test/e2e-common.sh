#!/usr/bin/env bash

# Copyright 2019 The Tekton Authors
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

# Helper functions for E2E tests.

source $(git rev-parse --show-toplevel)/vendor/github.com/tektoncd/plumbing/scripts/e2e-tests.sh

function install_pipeline_crd() {
  echo ">> Deploying Tekton Pipelines"
  local ko_target="$(mktemp)"
  ko resolve -R -f config/ > "${ko_target}" || fail_test "Pipeline image resolve failed"
  ko resolve -R -f optional_config/ >> "${ko_target}" || fail_test "Pod log access resolve failed"
  cat "${ko_target}" | sed -e 's%"level": "info"%"level": "debug"%' \
      | sed -e 's%loglevel.controller: "info"%loglevel.controller: "debug"%' \
      | sed -e 's%loglevel.webhook: "info"%loglevel.webhook: "debug"%' \
      | kubectl apply -R -f - || fail_test "Build pipeline installation failed"

  verify_pipeline_installation
  verify_resolvers_installation
  verify_log_access_enabled

  export SYSTEM_NAMESPACE=tekton-pipelines
}

# Install the Tekton pipeline crd based on the release number
function install_pipeline_crd_version() {
  echo ">> Deploying Tekton Pipelines of Version $1"
  kubectl apply -f "https://github.com/tektoncd/pipeline/releases/download/$1/release.yaml" || fail_test "Build pipeline installation failed of Version $1"

  # If the version to be installed is v0.40.x, we need to install resolvers.yaml separately.
  if [[ "${1}" == v0.40.* ]]; then
    kubectl apply -f "https://github.com/tektoncd/pipeline/releases/download/$1/resolvers.yaml" || fail_test "Resolvers installation failed of Version $1"
  fi

  verify_pipeline_installation
}

# Add the provided spiffeId to the spire-server.
function spire_apply() {
  if [ $# -lt 2 -o "$1" != "-spiffeID" ]; then
    echo "spire_apply requires a spiffeID as the first arg" >&2
    exit 1
  fi
  echo "Checking if spiffeID $2 already exists..."
  show=$(kubectl exec -n spire deployment/spire-server -- \
    /opt/spire/bin/spire-server entry show $1 $2)
  if [ "$show" != "Found 0 entries" ]; then
    # delete to recreate
    entryid=$(echo "$show" | grep "^Entry ID" | cut -f2 -d:)
    echo "Deleting previously existing spiffeID $2 ..."
    kubectl exec -n spire deployment/spire-server -- \
      /opt/spire/bin/spire-server entry delete -entryID $entryid
  fi
  echo "Adding spiffeID $2 to spire-server."
  kubectl exec -n spire deployment/spire-server -- \
    /opt/spire/bin/spire-server entry create "$@"
}

function install_spire() {
  echo ">> Deploying Spire"
  DIR="$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"

  echo "Creating SPIRE namespace..."
  kubectl create ns spire

  echo "Applying SPIFFE CSI Driver configuration..."
  kubectl apply -f "$DIR"/testdata/spire/spiffe-csi-driver.yaml

  echo "Deploying SPIRE server"
  kubectl apply -f "$DIR"/testdata/spire/spire-server.yaml

  echo "Deploying SPIRE agent"
  kubectl apply -f "$DIR"/testdata/spire/spire-agent.yaml

  wait_until_pods_running spire || fail_test "SPIRE did not come up"

  spire_apply \
    -spiffeID spiffe://example.org/ns/spire/node/example \
    -selector k8s_psat:cluster:example-cluster \
    -selector k8s_psat:agent_ns:spire \
    -selector k8s_psat:agent_sa:spire-agent \
    -node
  spire_apply \
    -spiffeID spiffe://example.org/ns/tekton-pipelines/sa/tekton-pipelines-controller \
    -parentID spiffe://example.org/ns/spire/node/example \
    -selector k8s:ns:tekton-pipelines \
    -selector k8s:pod-label:app:tekton-pipelines-controller \
    -selector k8s:sa:tekton-pipelines-controller \
    -admin
}

function patch_pipeline_spire() {
  kubectl patch \
      deployment tekton-pipelines-controller \
      -n tekton-pipelines \
      --patch-file "$DIR"/testdata/patch/pipeline-controller-spire.json

  verify_pipeline_installation
}

function verify_pipeline_installation() {
  # Make sure that everything is cleaned up in the current namespace.
  delete_pipeline_resources

  # Wait for pods to be running in the namespaces we are deploying to
  wait_until_pods_running tekton-pipelines || fail_test "Tekton Pipeline did not come up"
}

function verify_resolvers_installation() {
  # Make sure that everything is cleaned up in the current namespace.
  delete_resolvers_resources

  # Wait for pods to be running in the namespaces we are deploying to
  wait_until_pods_running tekton-pipelines-resolvers || fail_test "Tekton Pipeline Resolvers did not come up"
}

function verify_log_access_enabled() {
  var=$(kubectl get clusterroles | grep tekton-pipelines-controller-pod-log-access)
  if [ -z "$var" ]
  then
    fail_test "Failed to create clusterrole granting pod/logs access to the tekton controller."
  fi
  var=$(kubectl get clusterrolebindings | grep tekton-pipelines-controller-pod-log-access)
  if [ -z "$var" ]
  then
    fail_test "Failed to create clusterrole binding granting pod/logs access to the tekton controller."
  fi
}

function uninstall_pipeline_crd() {
  echo ">> Uninstalling Tekton Pipelines"
  ko delete --ignore-not-found=true -R -f config/

  # Make sure that everything is cleaned up in the current namespace.
  delete_pipeline_resources
}

function uninstall_pipeline_crd_version() {
  echo ">> Uninstalling Tekton Pipelines of version $1"
  kubectl delete --ignore-not-found=true -f "https://github.com/tektoncd/pipeline/releases/download/$1/release.yaml"

  if [ "${PIPELINE_FEATURE_GATE}" == "alpha" ]; then
    kubectl delete --ignore-not-found=true -f "https://github.com/tektoncd/pipeline/releases/download/$1/resolvers.yaml"
  fi

  # Make sure that everything is cleaned up in the current namespace.
  delete_pipeline_resources
}

function delete_pipeline_resources() {
  for res in tasks clustertasks pipelines taskruns pipelineruns; do
    kubectl delete --ignore-not-found=true ${res}.tekton.dev --all
  done
}

function delete_resolvers_resources() {
  kubectl delete --ignore-not-found=true resolutionrequests.resolution.tekton.dev --all
}
