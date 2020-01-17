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

function validate_run() {
  local tests_finished=0
  for i in {1..60}; do
    local finished="$(kubectl get $1.tekton.dev --output=jsonpath='{.items[*].status.conditions[*].status}')"
    if [[ ! "$finished" == *"Unknown"* ]]; then
      tests_finished=1
      break
    fi
    sleep 10
  done

  return ${tests_finished}
}

function check_results() {
  local failed=0
  results="$(kubectl get $1.tekton.dev --output=jsonpath='{range .items[*]}{.metadata.name}={.status.conditions[*].type}{.status.conditions[*].status}{" "}{end}')"
  for result in ${results}; do
    if [[ ! "${result,,}" == *"=succeededtrue" ]]; then
      echo "ERROR: test ${result} but should be succeededtrue"
      failed=1
    fi
  done

  return ${failed}
}

function create_resources() {
  local resource=$1
  echo ">> Creating resources ${resource}"

  # Applying the resources, either *taskruns or * *pipelineruns except those
  # in the no-ci directory
  for file in $(find ${REPO_ROOT_DIR}/examples/${resource}s/ -name '*.yaml' -not -path '*/no-ci/*' | sort); do
    perl -p -e 's/gcr.io\/christiewilson-catfactory/$ENV{KO_DOCKER_REPO}/g' ${file} | ko create -f - || return 1
  done
}

function run_tests() {
  local resource=$1

  # Wait for tests to finish.
  echo ">> Waiting for tests to finish for ${resource}"
  if validate_run $resource; then
    echo "ERROR: tests timed out"
  fi

  # Check that tests passed.
  echo ">> Checking test results for ${resource}"
  if check_results $resource; then
    echo ">> All YAML tests passed"
    return 0
  fi
  return 1
}

function run_yaml_tests() {
  echo ">> Starting tests for the resource ${1}"
  create_resources ${1} || fail_test "Could not create ${1} from the examples"
  if ! run_tests ${1}; then
    return 1
  fi
  return 0
}

function install_pipeline_crd() {
  echo ">> Deploying Tekton Pipelines"
  ko apply -f config/ || fail_test "Build pipeline installation failed"
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
  for res in conditions pipelineresources tasks pipelines taskruns pipelineruns; do
    kubectl delete --ignore-not-found=true ${res}.tekton.dev --all
  done

  # Wait for pods to be running in the namespaces we are deploying to
  wait_until_pods_running tekton-pipelines || fail_test "Tekton Pipeline did not come up"
}

function uninstall_pipeline_crd() {
  echo ">> Uninstalling Tekton Pipelines"
  ko delete --ignore-not-found=true -f config/

  # Make sure that everything is cleaned up in the current namespace.
  for res in conditions pipelineresources tasks pipelines taskruns pipelineruns; do
    kubectl delete --ignore-not-found=true ${res}.tekton.dev --all
  done
}

function uninstall_pipeline_crd_version() {
  echo ">> Uninstalling Tekton Pipelines of version $1"
  kubectl delete --ignore-not-found=true -f "https://github.com/tektoncd/pipeline/releases/download/$1/release.yaml"

  # Make sure that everything is cleaned up in the current namespace.
  for res in conditions pipelineresources tasks pipelines taskruns pipelineruns; do
    kubectl delete --ignore-not-found=true ${res}.tekton.dev --all
  done
}
