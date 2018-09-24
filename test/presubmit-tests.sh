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

# This script runs the presubmit tests; it is started by prow for each PR.

function build_tests() {
  echo "Running build tests"
  # TODO(aaron-prindle) add build tests
  echo "TODO(aaron-prindle) add build tests"

}

function unit_tests() {
  echo "Running unit tests"
  # TODO(aaron-prindle) add unit tests
  echo "TODO(aaron-prindle) add unit tests"
}

function integration_tests() {
  echo "Running integration tests"
  # TODO(aaron-prindle) add integration tests
  echo "TODO(aaron-prindle): add integration tests"
}

# Process flags and run tests accordingly.
function main() {
  exit_if_presubmit_not_required

  local all_parameters=$@
  [[ -z $1 ]] && all_parameters="--all-tests"

  for parameter in ${all_parameters}; do
    case ${parameter} in
      --all-tests)
        RUN_BUILD_TESTS=1
        RUN_UNIT_TESTS=1
        RUN_INTEGRATION_TESTS=1
        shift
        ;;
      --build-tests)
        RUN_BUILD_TESTS=1
        shift
        ;;
      --unit-tests)
        RUN_UNIT_TESTS=1
        shift
        ;;
      --integration-tests)
        RUN_INTEGRATION_TESTS=1
        shift
        ;;
      --emit-metrics)
        EMIT_METRICS=1
        shift
        ;;
      *)
        echo "error: unknown option ${parameter}"
        exit 1
        ;;
    esac
  done

  readonly RUN_BUILD_TESTS
  readonly RUN_UNIT_TESTS
  readonly RUN_INTEGRATION_TESTS
  readonly EMIT_METRICS

  # cd ${REPO_ROOT_DIR}

  # Tests to be performed, in the right order if --all-tests is passed.

  local result=0
  if (( RUN_BUILD_TESTS )); then
    build_tests || result=1
  fi
  if (( RUN_UNIT_TESTS )); then
    unit_tests || result=1
  fi
  if (( RUN_INTEGRATION_TESTS )); then
    integration_tests || result=1
  fi
  exit ${result}
}

main $@
