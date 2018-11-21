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

# This script calls out to scripts in knative/test-infra to setup a cluster
# and deploy the Pipeline CRD to it for running integration tests.

source $(dirname $0)/../vendor/github.com/knative/test-infra/scripts/e2e-tests.sh

# Helper functions.

function teardown() {
    header "Tearing down Pipeline CRD"
    ko delete --ignore-not-found=true -f config/
    kubectl delete --ignore-not-found=true -f ./third_party/config/build/release.yaml
    # teardown will be called when run against an existing cluster to cleanup before
    # continuing, so we must wait for the cleanup to complete or the subsequent attempt
    # to deploy to the same namespace will fail
    wait_until_object_does_not_exist namespace knative-build-pipeline
    wait_until_object_does_not_exist namespace knative-build
}

# Called by `fail_test` (provided by `e2e-tests.sh`) to dump info on test failure
function dump_extra_cluster_state() {
  echo ">>> Pipeline controller log:"
  kubectl -n knative-build-pipeline logs $(get_app_pod build-pipeline-controller knative-build-pipeline)
  echo ">>> Pipeline webhook log:"
  kubectl -n knative-build-pipeline logs $(get_app_pod build-pipeline-webhook knative-build-pipeline)
}

# Script entry point.

initialize $@

header "Setting up environment"

# Handle failures ourselves, so we can dump useful info.
set +o errexit
set +o pipefail

echo ">> Deploying Build CRD"
kubectl apply -f ./third_party/config/build/release.yaml || fail_test "Build installation failed"

echo ">> Deploying Pipeline CRD"
ko apply -f config/ || fail_test "Build pipeline installation failed"

# Wait for pods to be running in the namespaces we are deploying to
wait_until_pods_running knative-build-pipeline || fail_test "Pipeline CRD did not come up"

# Run the integration tests
go_test_e2e -timeout=20m ./test || fail_test

# Run the smoke tests for the examples dir to make sure they are valid
# Run these _after_ the integration tests b/c they don't quite work all the way
# and they cause a lot of noise in the logs, making it harder to debug integration
# test failures.
./examples/smoke-test.sh || fail_test

success
