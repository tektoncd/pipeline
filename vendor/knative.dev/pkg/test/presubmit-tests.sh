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

# This script runs the presubmit tests, in the right order.
# It is started by prow for each PR.
# For convenience, it can also be executed manually.

# Markdown linting failures don't show up properly in Gubernator resulting
# in a net-negative contributor experience.
export DISABLE_MD_LINTING=1

export GO111MODULE=on

source $(dirname $0)/../vendor/knative.dev/hack/presubmit-tests.sh

# TODO(#17): Write integration tests.

# We use the default build, unit and integration test runners.

function pre_build_tests() {
  # Test the custom code generators. This makes sure we can compile the output
  # of the injection generators.
  $(dirname $0)/test-reconciler-codegen.sh
  return 0
}

main $@
