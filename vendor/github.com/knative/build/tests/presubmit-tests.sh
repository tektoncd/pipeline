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

# This script runs the presubmit tests, in the right order.
# It is started by prow for each PR.
# For convenience, it can also be executed manually.

set -o errexit
set -o pipefail

# Extensions or file patterns that don't require presubmit tests
readonly NO_PRESUBMIT_FILES=(\.md \.png ^OWNERS)

source "$(dirname $(readlink -f ${BASH_SOURCE}))/library.sh"

# Helper functions.

function cleanup() {
  echo "Cleaning up for teardown"
  restore_override_vars
}

function build_tests() {
  header "Running build tests"
  go build ./...
  # kubekins images don't have dep installed by default
  go get -u github.com/golang/dep/cmd/dep
  ./hack/verify-codegen.sh
}

function unit_tests() {
  header "Running unit tests"
  go test ./...
}

function integration_tests() {
  # Make sure environment variables are intact.
  restore_override_vars
  ./tests/e2e-tests.sh
}

# Script entry point.

# Parse script argument:
# --all-tests or no arguments: run all tests
# --build-tests: run only the build tests
# --unit-tests: run only the unit tests
# --integration-tests: run only the integration tests
RUN_BUILD_TESTS=0
RUN_UNIT_TESTS=0
RUN_INTEGRATION_TESTS=0
[[ -z "$1" || "$1" == "--all-tests" ]] && RUN_BUILD_TESTS=1 && RUN_UNIT_TESTS=1 && RUN_INTEGRATION_TESTS=1
[[ "$1" == "--build-tests" ]] && RUN_BUILD_TESTS=1
[[ "$1" == "--unit-tests" ]] && RUN_UNIT_TESTS=1
[[ "$1" == "--integration-tests" ]] && RUN_INTEGRATION_TESTS=1
readonly RUN_BUILD_TESTS
readonly RUN_UNIT_TESTS
readonly RUN_INTEGRATION_TESTS

if ! (( RUN_BUILD_TESTS+RUN_UNIT_TESTS+RUN_INTEGRATION_TESTS )); then
  echo "error: unknown argument $1";
  exit 1
fi

cd ${BUILD_ROOT_DIR}

# Skip presubmit tests if whitelisted files were changed.
if [[ -n "${PULL_PULL_SHA}" ]]; then
  # On a presubmit job
  changes="$(git diff --name-only ${PULL_PULL_SHA} ${PULL_BASE_SHA})"
  no_presubmit_pattern="${NO_PRESUBMIT_FILES[*]}"
  no_presubmit_pattern="\(${no_presubmit_pattern// /\\|}\)$"
  echo -e "Changed files in commit ${PULL_PULL_SHA}:\n${changes}"
  if [[ -z "$(echo "${changes}" | grep -v ${no_presubmit_pattern})" ]]; then
    # Nothing changed other than files that don't require presubmit tests
    header "Commit only contains changes that don't affect tests, skipping"
    exit 0
  fi
fi

# For local runs, cleanup before and after the tests.
if (( ! IS_PROW )); then
  trap cleanup EXIT
  echo "Cleaning up for setup"
fi

# Tests to be performed, in the right order if --all-tests is passed.

if (( RUN_BUILD_TESTS )); then build_tests; fi
if (( RUN_UNIT_TESTS )); then unit_tests; fi
if (( RUN_INTEGRATION_TESTS )); then integration_tests; fi
