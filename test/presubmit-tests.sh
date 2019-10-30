#!/usr/bin/env bash

# Copyright 2018 The Tekton Authors
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
# For convenience, it can also be executed manually.
# Running the script without parameters, or with the --all-tests
# flag, causes all tests to be executed, in the right order.
# Use the flags --build-tests, --unit-tests and --integration-tests
# to run a specific set of tests.

# Markdown linting failures don't show up properly in Gubernator resulting
# in a net-negative contributor experience.
export DISABLE_MD_LINTING=1

source $(dirname $0)/../vendor/github.com/tektoncd/plumbing/scripts/presubmit-tests.sh

function test_documentation_has_been_generated() {
    header "Testing if documentation has been generated"

    make docs
    make man

    if [[ -n $(git status --porcelain docs/) ]];then
        echo "-- FATAL: The documentation or manpages didn't seem to be generated :"
        git status docs
        git diff docs
        results_banner "Documentation" 1
        exit 1
    fi

    results_banner "Documentation" 0
}

function check_lint() {
    header "Testing if golint has been done"

    golangci-lint run ./... --timeout 5m

    if [[ "$?" = "1" ]]; then
        results_banner "Lint" 1
        exit 1
    fi

    results_banner "Lint" 0
}

function pre_build_tests() {
    go get -u github.com/knative/test-infra/tools/dep-collector
}

function post_build_tests() {
    test_documentation_has_been_generated
    check_lint
}

# We use the default build, unit and integration test runners.
if [[ "$1" == "--build-cross-tests" ]]; then
    make cross
else
    main $@
fi
