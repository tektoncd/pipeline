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

# This script calls out to scripts in knative/test-infra to setup a cluster
# and deploy the Pipeline CRD to it for running integration tests.

source $(dirname $0)/../vendor/github.com/knative/test-infra/scripts/e2e-tests.sh

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
}

function run_yaml_tests() {
  echo ">> Starting tests"

  sed -i.bak 's/christiewilson-catfactory/${KO_DOCKER_REPO}/' examples/resources.yaml

  ko apply -R -f examples/ || return 1

  failed=$(validate_run)

  (( failed )) && return 1
  echo ">> All YAML tests passed"
  return 0
}