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

source $(dirname $0)/../vendor/github.com/knative/test-infra/scripts/e2e-tests.sh

function teardown() {
    subheader "Tearing down Pipeline CRD"
    ko delete --ignore-not-found=true -f config/
    # teardown will be called when run against an existing cluster to cleanup before
    # continuing, so we must wait for the cleanup to complete or the subsequent attempt
    # to deploy to the same namespace will fail
    wait_until_object_does_not_exist namespace knative-build-pipeline
}

function output_yaml_test_results() {
  # If formatting fails for any reason, use yaml as a fall back.
  kubectl get $1.pipeline.knative.dev -o=custom-columns-file=${REPO_ROOT_DIR}/test/columns.txt || \
    kubectl get $1.pipeline.knative.dev -oyaml
}

# Called by `fail_test` (provided by `e2e-tests.sh`) to dump info on test failure
function dump_extra_cluster_state() {
  echo ">>> Pipeline controller log:"
  kubectl -n knative-build-pipeline logs $(get_app_pod build-pipeline-controller knative-build-pipeline)
  echo ">>> Pipeline webhook log:"
  kubectl -n knative-build-pipeline logs $(get_app_pod build-pipeline-webhook knative-build-pipeline)
}

function validate_run() {
  # Wait for tests to finish.
  echo ">> Waiting for tests to finish"
  local tests_finished=0
  for i in {1..60}; do
    local finished="$(kubectl get $1.pipeline.knative.dev --output=jsonpath='{.items[*].status.conditions[*].status}')"
    if [[ ! "$finished" == *"Unknown"* ]]; then
      tests_finished=1
      break
    fi
    sleep 5
  done
  if (( ! tests_finished )); then
    echo "ERROR: tests timed out"
    return 1
  fi

  # Check that tests passed.
  local failed=0
  echo ">> Checking test results"
  for expected_status in succeeded failed; do
    results="$(kubectl get $1.pipeline.knative.dev -l expect=${expected_status} \
	--output=jsonpath='{range .items[*]}{.metadata.name}={.status.conditions[*].type}{.status.conditions[*].status}{" "}{end}')"
    case $expected_status in
      succeeded)
	want=succeededtrue
	;;
      failed)
	want=succeededfalse
	;;
      *)
	echo "ERROR: Invalid expected status '${expected_status}'"
	failed=1
	;;
    esac
    for result in ${results}; do
      if [[ ! "${result,,}" == *"=${want}" ]]; then
	echo "ERROR: test ${result} but should be ${want}"
	failed=1
      fi
    done
  done
  return ${failed}
}

function run_yaml_tests() {
  echo ">> Starting tests"

  find ${REPO_ROOT_DIR}/examples/ -name *.yaml -exec cat {} \; \
      | sed 's/christiewilson-catfactory/${KO_DOCKER_REPO}/' \
      | ko apply -f - \
      || return 1

  if validate_run $1; then
    echo ">> All YAML tests passed"
    return 0
  fi

  return 1
}
