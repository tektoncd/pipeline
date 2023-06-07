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

source $(git rev-parse --show-toplevel)/test/e2e-common.sh


# Setting defaults
PIPELINE_FEATURE_GATE=${PIPELINE_FEATURE_GATE:-stable}
SKIP_INITIALIZE=${SKIP_INITIALIZE:="false"}
RUN_YAML_TESTS=${RUN_YAML_TESTS:="true"}
SKIP_GO_E2E_TESTS=${SKIP_GO_E2E_TESTS:="false"}
E2E_GO_TEST_TIMEOUT=${E2E_GO_TEST_TIMEOUT:="20m"}
RESULTS_FROM=${RESULTS_FROM:-termination-message}
failed=0

# Script entry point.

if [ "${SKIP_INITIALIZE}" != "true" ]; then
  initialize $@
fi

header "Setting up environment"

install_pipeline_crd

failed=0

function add_spire() {
  local gate="$1"
  if [ "$gate" == "alpha" ] ; then
    DIR="$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"
    printf "Setting up environment for alpha features"
    install_spire
    patch_pipeline_spire
    kubectl apply -n tekton-pipelines -f "$DIR"/testdata/spire/config-spire.yaml
    failed=0
  fi
}

function set_feature_gate() {
  local gate="$1"
  if [ "$gate" != "alpha" ] && [ "$gate" != "stable" ] && [ "$gate" != "beta" ] ; then
    printf "Invalid gate %s\n" ${gate}
    exit 255
  fi
  printf "Setting feature gate to %s\n", ${gate}
  jsonpatch=$(printf "{\"data\": {\"enable-api-fields\": \"%s\"}}" $1)
  echo "feature-flags ConfigMap patch: ${jsonpatch}"
  kubectl patch configmap feature-flags -n tekton-pipelines -p "$jsonpatch"
}

function set_result_extraction_method() {
  local method="$1"
  if [ "$method" != "termination-message" ] && [ "$method" != "sidecar-logs" ]; then
    printf "Invalid value for results-from %s\n" ${method}
    exit 255
  fi
  printf "Setting results-from to %s\n", ${method}
  jsonpatch=$(printf "{\"data\": {\"results-from\": \"%s\"}}" $1)
  echo "feature-flags ConfigMap patch: ${jsonpatch}"
  kubectl patch configmap feature-flags -n tekton-pipelines -p "$jsonpatch"
}

function run_e2e() {
  # Run the integration tests
  header "Running Go e2e tests"
  # Skip ./test/*.go tests if SKIP_GO_E2E_TESTS == true
  if [ "${SKIP_GO_E2E_TESTS}" != "true" ]; then
    go_test_e2e -timeout=${E2E_GO_TEST_TIMEOUT} ./test/... || failed=1
  fi

  # Run these _after_ the integration tests b/c they don't quite work all the way
  # and they cause a lot of noise in the logs, making it harder to debug integration
  # test failures.
  if [ "${RUN_YAML_TESTS}" == "true" ]; then
    go_test_e2e -mod=readonly -tags=examples -timeout=${E2E_GO_TEST_TIMEOUT} ./test/ || failed=1
  fi
}

add_spire "$PIPELINE_FEATURE_GATE"
set_feature_gate "$PIPELINE_FEATURE_GATE"
set_result_extraction_method "$RESULTS_FROM"
run_e2e

(( failed )) && fail_test
success
