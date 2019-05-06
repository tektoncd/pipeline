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

# This script runs the presubmit tests; it is started by prow for each PR.
# For convenience, it can also be executed manually.
# Running the script without parameters, or with the --all-tests
# flag, causes all tests to be executed, in the right order.
# Use the flags --build-tests, --unit-tests and --integration-tests
# to run a specific set of tests.

# Markdown linting failures don't show up properly in Gubernator resulting
# in a net-negative contributor experience.
export DISABLE_MD_LINTING=1

# I probably shouldn't hardcode this here, but only viable alternatives at the moment are
# to hardcode it in tektoncd/plumbing, which doesn't seem much better, or to put it into
# a secret in the Prow cluster, which we could do later if this becomes a problem
export CODECOV_TOKEN="ccdc8320-3d8d-4e0c-b88c-ee54ec95d25d"

source $(dirname $0)/../vendor/github.com/knative/test-infra/scripts/presubmit-tests.sh

# Override the default unit test runner so we can add arguments to produce coverage
# reporting and report it to codecov.io
function unit_tests() {
  report_go_test -coverprofile=coverage.txt -covermode=atomic ./...
  bash <(curl -s https://codecov.io/bash) -t $CODECOV_TOKEN
}

# We use the default build, integration test runners.

main $@
