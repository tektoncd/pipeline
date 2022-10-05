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
  cat "${ko_target}" | sed -e 's%"level": "info"%"level": "debug"%' \
      | sed -e 's%loglevel.controller: "info"%loglevel.controller: "debug"%' \
      | sed -e 's%loglevel.webhook: "info"%loglevel.webhook: "debug"%' \
      | kubectl apply -R -f - || fail_test "Build pipeline installation failed"

  verify_pipeline_installation
  verify_resolvers_installation

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
  for res in pipelineresources tasks clustertasks pipelines taskruns pipelineruns; do
    kubectl delete --ignore-not-found=true ${res}.tekton.dev --all
  done
}

function delete_resolvers_resources() {
  kubectl delete --ignore-not-found=true resolutionrequests.resolution.tekton.dev --all
}
