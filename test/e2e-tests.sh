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
EMBEDDED_STATUS_GATE=${EMBEDDED_STATUS_GATE:-full}
SKIP_INITIALIZE=${SKIP_INITIALIZE:="false"}
RUN_YAML_TESTS=${RUN_YAML_TESTS:="true"}
SKIP_GO_E2E_TESTS=${SKIP_GO_E2E_TESTS:="false"}
E2E_GO_TEST_TIMEOUT=${E2E_GO_TEST_TIMEOUT:="20m"}
failed=0

# Script entry point.

if [ "${SKIP_INITIALIZE}" != "true" ]; then
  initialize $@
fi

header "Setting up environment"

install_pipeline_crd

failed=0

function set_feature_gate() {
  local gate="$1"
  local resolver="false"
  if [ "$gate" != "alpha" ] && [ "$gate" != "stable" ] && [ "$gate" != "beta" ] ; then
    printf "Invalid gate %s\n" ${gate}
    exit 255
  fi
  if [ "$gate" == "alpha" ]; then
    resolver="true"
  fi
  printf "Setting feature gate to %s\n", ${gate}
  jsonpatch=$(printf "{\"data\": {\"enable-api-fields\": \"%s\"}}" $1)
  echo "feature-flags ConfigMap patch: ${jsonpatch}"
  kubectl patch configmap feature-flags -n tekton-pipelines -p "$jsonpatch"
  if [ "$gate" == "alpha" ]; then
    printf "enabling resolvers\n"
    jsonpatch=$(printf "{\"data\": {\"enable-git-resolver\": \"true\", \"enable-hub-resolver\": \"true\", \"enable-bundles-resolver\": \"true\", \"enable-cluster-resolver\": \"true\", \"enable-provenance-in-status\": \"true\"}}")
    echo "resolvers-feature-flags ConfigMap patch: ${jsonpatch}"
    kubectl patch configmap resolvers-feature-flags -n tekton-pipelines-resolvers -p "$jsonpatch"
  fi
}

function set_embedded_status() {
  local status="$1"
  if [ "$status" != "full" ] && [ "$status" != "minimal" ] && [ "$status" != "both" ] ; then
    printf "Invalid embedded status %s\n" ${status}
    exit 255
  fi
  printf "Setting embedded status to %s\n", ${status}
  jsonpatch=$(printf "{\"data\": {\"embedded-status\": \"%s\"}}" $1)
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

set_feature_gate "$PIPELINE_FEATURE_GATE"
set_embedded_status "$EMBEDDED_STATUS_GATE"
run_e2e

(( failed )) && fail_test
success
