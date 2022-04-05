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
  ko resolve -R -f config/ \
      | sed -e 's%"level": "info"%"level": "debug"%' \
      | sed -e 's%loglevel.controller: "info"%loglevel.controller: "debug"%' \
      | sed -e 's%loglevel.webhook: "info"%loglevel.webhook: "debug"%' \
      | kubectl apply -R -f - || fail_test "Build pipeline installation failed"
  verify_pipeline_installation

  export SYSTEM_NAMESPACE=tekton-pipelines
}

# Install the Tekton pipeline crd based on the release number
function install_pipeline_crd_version() {
  echo ">> Deploying Tekton Pipelines of Version $1"
  kubectl apply -f "https://github.com/tektoncd/pipeline/releases/download/$1/release.yaml" || fail_test "Build pipeline installation failed of Version $1"
  verify_pipeline_installation
}

function spire_apply() {
  if [ $# -lt 2 -o "$1" != "-spiffeID" ]; then
    echo "spire_apply requires a spiffeID as the first arg" >&2
    exit 1
  fi
  show=$(kubectl exec -n spire deployment/spire-server -- \
    /opt/spire/bin/spire-server entry show $1 $2)
  if [ "$show" != "Found 0 entries" ]; then
    # delete to recreate
    entryid=$(echo "$show" | grep "^Entry ID" | cut -f2 -d:)
    kubectl exec -n spire deployment/spire-server -- \
      /opt/spire/bin/spire-server entry delete -entryID $entryid
  fi
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

function patch_pipline_spire() {
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

function uninstall_pipeline_crd() {
  echo ">> Uninstalling Tekton Pipelines"
  ko delete --ignore-not-found=true -R -f config/

  # Make sure that everything is cleaned up in the current namespace.
  delete_pipeline_resources
}

function uninstall_pipeline_crd_version() {
  echo ">> Uninstalling Tekton Pipelines of version $1"
  kubectl delete --ignore-not-found=true -f "https://github.com/tektoncd/pipeline/releases/download/$1/release.yaml"

  # Make sure that everything is cleaned up in the current namespace.
  delete_pipeline_resources
}

function delete_pipeline_resources() {
  for res in conditions pipelineresources tasks clustertasks pipelines taskruns pipelineruns; do
    kubectl delete --ignore-not-found=true ${res}.tekton.dev --all
  done
}
