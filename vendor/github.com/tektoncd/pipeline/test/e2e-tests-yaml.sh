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

# This script calls out to scripts in tektoncd/plumbing to setup a cluster
# and deploy Tekton Pipelines to it for running integration tests.

source $(dirname $0)/e2e-common.sh

# Script entry point.

initialize $@

header "Setting up environment"

# Handle failures ourselves, so we can dump useful info.
set +o errexit
set +o pipefail

install_pipeline_crd

# Run the tests
failed=0
for test in taskrun pipelinerun; do
  header "Running YAML e2e tests for ${test}s"
  if ! run_yaml_tests ${test}; then
    echo "ERROR: one or more YAML tests failed"
    output_yaml_test_results ${test}
    output_pods_logs ${test}
    failed=1
  fi
done

(( failed )) && fail_test

success
