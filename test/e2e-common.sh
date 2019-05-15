#!/usr/bin/env bash

# Copyright 2018 The Knative Authors
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

source $(dirname $0)/../vendor/github.com/tektoncd/plumbing/scripts/e2e-tests.sh

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
		go run ./test/logs/main.go -tr ${run}
		;;
	    "pipelinerun")
		go run ./test/logs/main.go -pr ${run}
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

function run_yaml_tests() {
  echo ">> Starting tests"

  # Applying *taskruns
  for file in $(find ${REPO_ROOT_DIR}/examples/taskruns/ -name *.yaml | sort); do
    perl -p -e 's/gcr.io\/christiewilson-catfactory/$ENV{KO_DOCKER_REPO}/g' ${file} | ko apply -f - || return 1
  done

  # Applying *pipelineruns
  for file in $(find ${REPO_ROOT_DIR}/examples/pipelineruns/ -name *.yaml | sort); do
    perl -p -e 's/gcr.io\/christiewilson-catfactory/$ENV{KO_DOCKER_REPO}/g' ${file} | ko apply -f - || return 1
  done

  # Wait for tests to finish.
  echo ">> Waiting for tests to finish for ${test}"
  if validate_run $1; then
    echo "ERROR: tests timed out"
  fi

  # Check that tests passed.
  echo ">> Checking test results for ${test}"
  if check_results $1; then
    echo ">> All YAML tests passed"
    return 0
  fi

  return 1
}

function install_pipeline_crd() {
  echo ">> Deploying Tekton Pipelines"
  ko apply -f config/ || fail_test "Build pipeline installation failed"

  # Make sure thateveything is cleaned up in the current namespace.
  for res in pipelineresources tasks pipelines taskruns pipelineruns; do
    kubectl delete --ignore-not-found=true ${res}.tekton.dev --all
  done

  # Wait for pods to be running in the namespaces we are deploying to
  wait_until_pods_running tekton-pipelines || fail_test "Tekton Pipeline did not come up"
}
