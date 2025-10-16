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
RUN_FEATUREFLAG_TESTS=${RUN_FEATUREFLAG_TESTS:="false"}
RESULTS_FROM=${RESULTS_FROM:-termination-message}
KEEP_POD_ON_CANCEL=${KEEP_POD_ON_CANCEL:="false"}
ENABLE_CEL_IN_WHENEXPRESSION=${ENABLE_CEL_IN_WHENEXPRESSION:="false"}
ENABLE_PARAM_ENUM=${ENABLE_PARAM_ENUM:="false"}
ENABLE_ARTIFACTS=${ENABLE_ARTIFACTS:="false"}
ENABLE_CONCISE_RESOLVER_SYNTAX=${ENABLE_CONCISE_RESOLVER_SYNTAX:="false"}
ENABLE_KUBERNETES_SIDECAR=${ENABLE_KUBERNETES_SIDECAR:="false"}

# Script entry point.

if [ "${SKIP_INITIALIZE}" != "true" ]; then
  initialize $@
fi

header "Setting up environment"

set -x
install_pipeline_crd
export SYSTEM_NAMESPACE=tekton-pipelines
set +x


function add_spire() {
  local gate="$1"
  if [ "$gate" == "alpha" ] ; then
    DIR="$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"
    printf "Setting up environment for alpha features"
    install_spire
    patch_pipeline_spire
    kubectl apply -n tekton-pipelines -f "$DIR"/testdata/spire/config-spire.yaml
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
  with_retries kubectl patch configmap feature-flags -n tekton-pipelines -p "$jsonpatch"
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
  with_retries kubectl patch configmap feature-flags -n tekton-pipelines -p "$jsonpatch"
}

function set_keep_pod_on_cancel() {	
    local method="$1"	
    if [ "$method" != "false" ] && [ "$method" != "true" ]; then	
      printf "Invalid value for keep-pod-on-cancel %s\n" ${method}	
      exit 255	
    fi	
    printf "Setting keep-pod-on-cancel to %s\n", ${method}	
    jsonpatch=$(printf "{\"data\": {\"keep-pod-on-cancel\": \"%s\"}}" $1)	
    echo "feature-flags ConfigMap patch: ${jsonpatch}"	
    with_retries kubectl patch configmap feature-flags -n tekton-pipelines -p "$jsonpatch"
}

function set_cel_in_whenexpression() {
  local method="$1"
  if [ "$method" != "false" ] && [ "$method" != "true" ]; then
    printf "Invalid value for enable-cel-in-whenexpression %s\n" ${method}
    exit 255
  fi
  jsonpatch=$(printf "{\"data\": {\"enable-cel-in-whenexpression\": \"%s\"}}" $1)
  echo "feature-flags ConfigMap patch: ${jsonpatch}"
  with_retries kubectl patch configmap feature-flags -n tekton-pipelines -p "$jsonpatch"
}

function set_enable_param_enum() {
  local method="$1"
  if [ "$method" != "false" ] && [ "$method" != "true" ]; then
    printf "Invalid value for enable-param-enum %s\n" ${method}
    exit 255
  fi
  printf "Setting enable-param-enum to %s\n", ${method}
  jsonpatch=$(printf "{\"data\": {\"enable-param-enum\": \"%s\"}}" $1)
  echo "feature-flags ConfigMap patch: ${jsonpatch}"
  with_retries kubectl patch configmap feature-flags -n tekton-pipelines -p "$jsonpatch"
}

function set_enable_artifacts() {
  local method="$1"
  if [ "$method" != "false" ] && [ "$method" != "true" ]; then
    printf "Invalid value for enable-artifacts %s\n" ${method}
    exit 255
  fi
  printf "Setting enable-artifacts to %s\n", ${method}
  jsonpatch=$(printf "{\"data\": {\"enable-artifacts\": \"%s\"}}" $1)
  echo "feature-flags ConfigMap patch: ${jsonpatch}"
  with_retries kubectl patch configmap feature-flags -n tekton-pipelines -p "$jsonpatch"
}

function set_enable_concise_resolver_syntax() {
  local method="$1"
  if [ "$method" != "false" ] && [ "$method" != "true" ]; then
    printf "Invalid value for enable-concise-resolver-syntax %s\n" ${method}
    exit 255
  fi
  printf "Setting enable-concise-resolver-syntax to %s\n", ${method}
  jsonpatch=$(printf "{\"data\": {\"enable-concise-resolver-syntax\": \"%s\"}}" $1)
  echo "feature-flags ConfigMap patch: ${jsonpatch}"
  with_retries kubectl patch configmap feature-flags -n tekton-pipelines -p "$jsonpatch"
}

function set_enable_kubernetes_sidecar() {
  local method="$1"
  if [ "$method" != "false" ] && [ "$method" != "true" ]; then
    printf "Invalid value for enable-kubernetes-sidecar %s\n" ${method}
    exit 255
  fi
  printf "Setting enable-kubernetes-sidecar to %s\n", ${method}
  jsonpatch=$(printf "{\"data\": {\"enable-kubernetes-sidecar\": \"%s\"}}" $1)
  echo "feature-flags ConfigMap patch: ${jsonpatch}"
  with_retries kubectl patch configmap feature-flags -n tekton-pipelines -p "$jsonpatch"
}

function set_default_sidecar_log_polling_interval() {
  # Sets the default-sidecar-log-polling-interval in the config-defaults ConfigMap to 0ms for e2e tests
  echo "Patching config-defaults ConfigMap: setting default-sidecar-log-polling-interval to 0ms"
  jsonpatch='{"data": {"default-sidecar-log-polling-interval": "0ms"}}'
  with_retries kubectl patch configmap config-defaults -n tekton-pipelines -p "$jsonpatch"
}

function run_e2e() {
  # Run the integration tests
  header "Running Go e2e tests"
  # Skip ./test/*.go tests if SKIP_GO_E2E_TESTS == true
  if [ "${SKIP_GO_E2E_TESTS}" != "true" ]; then
    go_test_e2e -timeout=${E2E_GO_TEST_TIMEOUT} ./test/...
  fi

  # Run these _after_ the integration tests b/c they don't quite work all the way
  # and they cause a lot of noise in the logs, making it harder to debug integration
  # test failures.
  if [ "${RUN_YAML_TESTS}" == "true" ]; then
    go_test_e2e -mod=readonly -tags=examples -timeout=${E2E_GO_TEST_TIMEOUT} ./test/
  fi

  if [ "${RUN_FEATUREFLAG_TESTS}" == "true" ]; then
    go_test_e2e -mod=readonly -tags=featureflags -timeout=${E2E_GO_TEST_TIMEOUT} ./test/
  fi
}

set -eo pipefail
trap '[[ "$?" == "0" ]] || fail_test' EXIT

add_spire "$PIPELINE_FEATURE_GATE"
set_feature_gate "$PIPELINE_FEATURE_GATE"
set_result_extraction_method "$RESULTS_FROM"
set_keep_pod_on_cancel "$KEEP_POD_ON_CANCEL"
set_cel_in_whenexpression "$ENABLE_CEL_IN_WHENEXPRESSION"
set_enable_param_enum "$ENABLE_PARAM_ENUM"
set_enable_artifacts "$ENABLE_ARTIFACTS"
set_enable_concise_resolver_syntax "$ENABLE_CONCISE_RESOLVER_SYNTAX"
set_enable_kubernetes_sidecar "$ENABLE_KUBERNETES_SIDECAR"
set_default_sidecar_log_polling_interval
run_e2e

success
