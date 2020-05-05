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

function teardown() {
    subheader "Tearing down Tekton Pipelines"
    ko delete --ignore-not-found=true -f config/
    # teardown will be called when run against an existing cluster to cleanup before
    # continuing, so we must wait for the cleanup to complete or the subsequent attempt
    # to deploy to the same namespace will fail
    wait_until_object_does_not_exist namespace tekton-pipelines
}

function output_yaml_test_results() {
  # If formatting fails for any reason, use yaml as a fall back.
  kubectl get $1.tekton.dev -o=custom-columns-file=${REPO_ROOT_DIR}/test/columns.txt || \
    kubectl get $1.tekton.dev -oyaml
}

function output_pods_logs() {
    echo ">>> $1"
    kubectl get $1.tekton.dev -o yaml
    local runs=$(kubectl get $1.tekton.dev --output=jsonpath="{.items[*].metadata.name}")
    set +e
    for run in ${runs}; do
	echo ">>>> $1 ${run}"
	case "$1" in
	    "taskrun")
		tkn taskrun logs --nocolour ${run}
		;;
	    "pipelinerun")
		tkn pipelinerun logs --nocolour ${run}
		;;
	esac
    done
    set -e
    echo ">>>> Pods"
    kubectl get pods -o yaml
}

# Called by `fail_test` (provided by `e2e-tests.sh`) to dump info on test failure
function dump_extra_cluster_state() {
  echo ">>> Pipeline controller log:"
  kubectl -n tekton-pipelines logs $(get_app_pod tekton-pipelines-controller tekton-pipelines)
  echo ">>> Pipeline webhook log:"
  kubectl -n tekton-pipelines logs $(get_app_pod tekton-pipelines-webhook tekton-pipelines)
}

function install_pipeline_crd() {
  echo ">> Deploying Tekton Pipelines"
  ko resolve -f config/ \
      | sed -e 's%"level": "info"%"level": "debug"%' \
      | sed -e 's%loglevel.controller: "info"%loglevel.controller: "debug"%' \
      | sed -e 's%loglevel.webhook: "info"%loglevel.webhook: "debug"%' \
      | kubectl apply -f - || fail_test "Build pipeline installation failed"
  verify_pipeline_installation
}

# Install the Tekton pipeline crd based on the release number
function install_pipeline_crd_version() {
  echo ">> Deploying Tekton Pipelines of Version $1"
  kubectl apply -f "https://github.com/tektoncd/pipeline/releases/download/$1/release.yaml" || fail_test "Build pipeline installation failed of Version $1"
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
  ko delete --ignore-not-found=true -f config/

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
