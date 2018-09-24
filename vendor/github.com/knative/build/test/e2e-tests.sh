#!/bin/bash

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

# This script runs the end-to-end tests against the build controller
# built from source. It is started by prow for each PR.
# For convenience, it can also be executed manually.

# If you already have the *_OVERRIDE environment variables set, call
# this script with the --run-tests arguments and it will use the cluster
# and run the tests.

# Calling this script without arguments will create a new cluster in
# project $PROJECT_ID, start the controller, run the tests and delete
# the cluster.

source $(dirname $0)/../vendor/github.com/knative/test-infra/scripts/e2e-tests.sh

# Helper functions.

function teardown() {
  ko delete --ignore-not-found=true -R -f test/
  ko delete --ignore-not-found=true -f config/
}

function run_yaml_tests() {
  echo ">> Starting tests"
  ko apply -R -f test/ || return 1

  # Wait for tests to finish.
  echo ">> Waiting for tests to finish"
  local tests_finished=0
  for i in {1..60}; do
    local finished="$(kubectl get builds --output=jsonpath='{.items[*].status.conditions[*].status}')"
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
    results="$(kubectl get builds -l expect=${expected_status} \
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
  (( failed )) && return 1
  echo ">> All YAML tests passed"
  return 0
}

# Script entry point.

initialize $@

# Fail fast during setup.
set -o errexit
set -o pipefail

header "Building and starting the controller"
export KO_DOCKER_REPO=${DOCKER_REPO_OVERRIDE}
ko apply -f config/ || fail_test

# Handle test failures ourselves, so we can dump useful info.
set +o errexit
set +o pipefail

# Make sure that are no builds or build templates in the current namespace.
kubectl delete --ignore-not-found=true builds --all
kubectl delete --ignore-not-found=true buildtemplates --all

# Run the tests

failed=0

header "Running Go e2e tests"
report_go_test -tags e2e ./test/e2e/... -count=1 || failed=1

header "Running YAML e2e tests"
if ! run_yaml_tests; then
  failed=1
  echo "ERROR: one or more YAML tests failed"
  # If formatting fails for any reason, use yaml as a fall back.
  kubectl get builds -o=custom-columns-file=./test/columns.txt || \
    kubectl get builds -oyaml
fi

(( failed )) && fail_test
success
