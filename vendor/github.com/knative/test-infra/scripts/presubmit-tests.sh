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

# This is a helper script for Knative presubmit test scripts.
# See README.md for instructions on how to use it.

source $(dirname ${BASH_SOURCE})/library.sh

# Custom configuration of presubmit tests
readonly DISABLE_MD_LINTING=${DISABLE_MD_LINTING:-0}
readonly DISABLE_MD_LINK_CHECK=${DISABLE_MD_LINK_CHECK:-0}
readonly PRESUBMIT_TEST_FAIL_FAST=${PRESUBMIT_TEST_FAIL_FAST:-0}

# Extensions or file patterns that don't require presubmit tests.
readonly NO_PRESUBMIT_FILES=(\.png \.gitignore \.gitattributes ^OWNERS ^OWNERS_ALIASES ^AUTHORS)

# Flag if this is a presubmit run or not.
[[ IS_PROW && -n "${PULL_PULL_SHA}" ]] && IS_PRESUBMIT=1 || IS_PRESUBMIT=0
readonly IS_PRESUBMIT

# List of changed files on presubmit, LF separated.
CHANGED_FILES=""

# Flags that this PR is exempt of presubmit tests.
IS_PRESUBMIT_EXEMPT_PR=0

# Flags that this PR contains only changes to documentation.
IS_DOCUMENTATION_PR=0

# Returns true if PR only contains the given file regexes.
# Parameters: $1 - file regexes, space separated.
function pr_only_contains() {
  [[ -z "$(echo "${CHANGED_FILES}" | grep -v \(${1// /\\|}\)$))" ]]
}

# List changed files in the current PR.
# This is implemented as a function so it can be mocked in unit tests.
function list_changed_files() {
  /workspace/githubhelper -list-changed-files
}

# Initialize flags and context for presubmit tests:
# CHANGED_FILES, IS_PRESUBMIT_EXEMPT_PR and IS_DOCUMENTATION_PR.
function initialize_environment() {
  CHANGED_FILES=""
  IS_PRESUBMIT_EXEMPT_PR=0
  IS_DOCUMENTATION_PR=0
  (( ! IS_PRESUBMIT )) && return
  CHANGED_FILES="$(list_changed_files)"
  if [[ -n "${CHANGED_FILES}" ]]; then
    echo -e "Changed files in commit ${PULL_PULL_SHA}:\n${CHANGED_FILES}"
    local no_presubmit_files="${NO_PRESUBMIT_FILES[*]}"
    pr_only_contains "${no_presubmit_files}" && IS_PRESUBMIT_EXEMPT_PR=1
    pr_only_contains "\.md ${no_presubmit_files}" && IS_DOCUMENTATION_PR=1
  else
    header "NO CHANGED FILES REPORTED, ASSUMING IT'S AN ERROR AND RUNNING TESTS ANYWAY"
  fi
  readonly CHANGED_FILES
  readonly IS_DOCUMENTATION_PR
  readonly IS_PRESUBMIT_EXEMPT_PR
}

# Display a pass/fail banner for a test group.
# Parameters: $1 - test group name (e.g., build)
#             $2 - result (0=passed, 1=failed)
function results_banner() {
  local result
  [[ $2 -eq 0 ]] && result="PASSED" || result="FAILED"
  header "$1 tests ${result}"
}

# Run build tests. If there's no `build_tests` function, run the default
# build test runner.
function run_build_tests() {
  (( ! RUN_BUILD_TESTS )) && return 0
  header "Running build tests"
  local failed=0
  # Run pre-build tests, if any
  if function_exists pre_build_tests; then
    pre_build_tests || failed=1
  fi
  # Don't run build tests if pre-build tests failed
  if (( ! failed )); then
    if function_exists build_tests; then
      build_tests || failed=1
    else
      default_build_test_runner || failed=1
    fi
  fi
  # Don't run post-build tests if pre/build tests failed
  if (( ! failed )) && function_exists post_build_tests; then
    post_build_tests || failed=1
  fi
  results_banner "Build" ${failed}
  return ${failed}
}

# Perform markdown build tests if necessary, unless disabled.
function markdown_build_tests() {
  (( DISABLE_MD_LINTING && DISABLE_MD_LINK_CHECK )) && return 0
  # Get changed markdown files (ignore /vendor and deleted files)
  local mdfiles=""
  for file in $(echo "${CHANGED_FILES}" | grep \.md$ | grep -v ^vendor/); do
    [[ -f "${file}" ]] && mdfiles="${mdfiles} ${file}"
  done
  [[ -z "${mdfiles}" ]] && return 0
  local failed=0
  if (( ! DISABLE_MD_LINTING )); then
    subheader "Linting the markdown files"
    lint_markdown ${mdfiles} || failed=1
  fi
  if (( ! DISABLE_MD_LINK_CHECK )); then
    subheader "Checking links in the markdown files"
    check_links_in_markdown ${mdfiles} || failed=1
  fi
  return ${failed}
}

# Default build test runner that:
# * check markdown files
# * `go build` on the entire repo
# * run `/hack/verify-codegen.sh` (if it exists)
# * check licenses in all go packages
function default_build_test_runner() {
  local failed=0
  # Perform markdown build checks first
  markdown_build_tests || failed=1
  # For documentation PRs, just check the md files
  (( IS_DOCUMENTATION_PR )) && return ${failed}
  # Skip build test if there is no go code
  local go_pkg_dirs="$(go list ./...)"
  [[ -z "${go_pkg_dirs}" ]] && return ${failed}
  # Ensure all the code builds
  subheader "Checking that go code builds"
  go build -v ./... || failed=1
  # Get all build tags in go code (ignore /vendor)
  local tags="$(grep -r '// +build' . \
      | grep -v '^./vendor/' | cut -f3 -d' ' | sort | uniq | tr '\n' ' ')"
  if [[ -n "${tags}" ]]; then
    go test -run=^$ -tags="${tags}" ./... || failed=1
  fi
  if [[ -f ./hack/verify-codegen.sh ]]; then
    subheader "Checking autogenerated code is up-to-date"
    ./hack/verify-codegen.sh || failed=1
  fi
  # Check that we don't have any forbidden licenses in our images.
  subheader "Checking for forbidden licenses"
  check_licenses ${go_pkg_dirs} || failed=1
  return ${failed}
}

# Run unit tests. If there's no `unit_tests` function, run the default
# unit test runner.
function run_unit_tests() {
  (( ! RUN_UNIT_TESTS )) && return 0
  header "Running unit tests"
  local failed=0
  # Run pre-unit tests, if any
  if function_exists pre_unit_tests; then
    pre_unit_tests || failed=1
  fi
  # Don't run unit tests if pre-unit tests failed
  if (( ! failed )); then
    if function_exists unit_tests; then
      unit_tests || failed=1
    else
      default_unit_test_runner || failed=1
    fi
  fi
  # Don't run post-unit tests if pre/unit tests failed
  if (( ! failed )) && function_exists post_unit_tests; then
    post_unit_tests || failed=1
  fi
  results_banner "Unit" ${failed}
  return ${failed}
}

# Default unit test runner that runs all go tests in the repo.
function default_unit_test_runner() {
  report_go_test ./...
}

# Run integration tests. If there's no `integration_tests` function, run the
# default integration test runner.
function run_integration_tests() {
  # Don't run integration tests if not requested OR on documentation PRs
  (( ! RUN_INTEGRATION_TESTS )) && return 0
  (( IS_DOCUMENTATION_PR )) && return 0
  header "Running integration tests"
  local failed=0
  # Run pre-integration tests, if any
  if function_exists pre_integration_tests; then
    pre_integration_tests || failed=1
  fi
  # Don't run integration tests if pre-integration tests failed
  if (( ! failed )); then
    if function_exists integration_tests; then
      integration_tests || failed=1
    else
      default_integration_test_runner || failed=1
    fi
  fi
  # Don't run integration tests if pre/integration tests failed
  if (( ! failed )) && function_exists post_integration_tests; then
    post_integration_tests || failed=1
  fi
  results_banner "Integration" ${failed}
  return ${failed}
}

# Default integration test runner that runs all `test/e2e-*tests.sh`.
function default_integration_test_runner() {
  local options=""
  local failed=0
  (( EMIT_METRICS )) && options="--emit-metrics"
  for e2e_test in $(find test/ -name e2e-*tests.sh); do
    echo "Running integration test ${e2e_test}"
    if ! ${e2e_test} ${options}; then
      failed=1
    fi
  done
  return ${failed}
}

# Options set by command-line flags.
RUN_BUILD_TESTS=0
RUN_UNIT_TESTS=0
RUN_INTEGRATION_TESTS=0
EMIT_METRICS=0

# Process flags and run tests accordingly.
function main() {
  initialize_environment
  if (( IS_PRESUBMIT_EXEMPT_PR )) && (( ! IS_DOCUMENTATION_PR )); then
    header "Commit only contains changes that don't require tests, skipping"
    exit 0
  fi

  # Show the version of the tools we're using
  if (( IS_PROW )); then
    # Disable gcloud update notifications
    gcloud config set component_manager/disable_update_check true
    header "Current test setup"
    echo ">> gcloud SDK version"
    gcloud version
    echo ">> kubectl version"
    kubectl version --client
    echo ">> go version"
    go version
    echo ">> git version"
    git version
    echo ">> bazel version"
    bazel version 2> /dev/null
    if [[ "${DOCKER_IN_DOCKER_ENABLED}" == "true" ]]; then
      echo ">> docker version"
      docker version
    fi
  fi

  [[ -z $1 ]] && set -- "--all-tests"

  local TEST_TO_RUN=""

  while [[ $# -ne 0 ]]; do
    local parameter=$1
    case ${parameter} in
      --build-tests) RUN_BUILD_TESTS=1 ;;
      --unit-tests) RUN_UNIT_TESTS=1 ;;
      --integration-tests) RUN_INTEGRATION_TESTS=1 ;;
      --emit-metrics) EMIT_METRICS=1 ;;
      --all-tests)
        RUN_BUILD_TESTS=1
        RUN_UNIT_TESTS=1
        RUN_INTEGRATION_TESTS=1
        ;;
      --run-test)
        shift
        [[ $# -ge 1 ]] || abort "missing executable after --run-test"
        TEST_TO_RUN=$1
        ;;
      *) abort "error: unknown option ${parameter}" ;;
    esac
    shift
  done

  readonly RUN_BUILD_TESTS
  readonly RUN_UNIT_TESTS
  readonly RUN_INTEGRATION_TESTS
  readonly EMIT_METRICS
  readonly TEST_TO_RUN

  cd ${REPO_ROOT_DIR}

  # Tests to be performed, in the right order if --all-tests is passed.

  local failed=0

  if [[ -n "${TEST_TO_RUN}" ]]; then
    if (( RUN_BUILD_TESTS || RUN_UNIT_TESTS || RUN_INTEGRATION_TESTS )); then
      abort "--run-test must be used alone"
    fi
    # If this is a presubmit run, but a documentation-only PR, don't run the test
    (( IS_PRESUBMIT && IS_DOCUMENTATION_PR )) && exit 0
    ${TEST_TO_RUN} || failed=1
  fi

  run_build_tests || failed=1
  # If PRESUBMIT_TEST_FAIL_FAST is set to true, don't run unit tests if build tests failed
  if (( ! PRESUBMIT_TEST_FAIL_FAST )) || (( ! failed )); then
    run_unit_tests || failed=1
  fi
  # If PRESUBMIT_TEST_FAIL_FAST is set to true, don't run integration tests if build/unit tests failed
  if (( ! PRESUBMIT_TEST_FAIL_FAST )) || (( ! failed )); then
    run_integration_tests || failed=1
  fi

  exit ${failed}
}
