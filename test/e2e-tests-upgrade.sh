#!/usr/bin/env bash

# Copyright 2019 The Tekton Authors
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
PREVIOUS_PIPELINE_VERSION=0.5.2

# Script entry point.

initialize $@

header "Setting up environment"

# Handle failures ourselves, so we can dump useful info.
set +o errexit
set +o pipefail

header "Install the previous release of Tekton pipeline $PREVIOUS_PIPELINE_VERSION"
install_pipeline_crd_version $PREVIOUS_PIPELINE_VERSION

header "Upgrade to the current release of Tekton pipeline"
install_pipeline_crd

# Run the tests
failed=0
go_test_e2e -timeout=20m ./test || failed=1

for test in taskrun pipelinerun; do
  header "Running YAML e2e tests for ${test}s"
  if ! run_yaml_tests ${test}; then
    echo "ERROR: one or more YAML tests failed"
    output_yaml_test_results ${test}
    output_pods_logs ${test}
    failed=1
  fi
done

# Remove all the pipeline CRDs
uninstall_pipeline_crd
uninstall_pipeline_crd_version $PREVIOUS_PIPELINE_VERSION

header "Install the previous release of Tekton pipeline $PREVIOUS_PIPELINE_VERSION"
install_pipeline_crd_version $PREVIOUS_PIPELINE_VERSION

for test in taskrun pipelinerun; do
  header "Applying the resources ${test}s"
  apply_resources ${test}
done

header "Upgrade to the current release of Tekton pipeline"
install_pipeline_crd

go_test_e2e -timeout=20m ./test || failed=1

for test in taskrun pipelinerun; do
  header "Running YAML e2e tests for ${test}s"
  if ! run_tests ${test}; then
    echo "ERROR: one or more YAML tests failed"
    output_yaml_test_results ${test}
    output_pods_logs ${test}
    failed=1
  fi
done

(( failed )) && fail_test

success
