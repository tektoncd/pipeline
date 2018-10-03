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

# This script calls out to scripts in knative/test-infra to setup a cluster
# and deploy the Pipeline CRD to it for running integration tests.

source $(dirname $0)/../vendor/github.com/knative/test-infra/scripts/e2e-tests.sh

set -o xtrace
set -o errexit
set -o pipefail

# The logic to create resource names (which are used by kubetest) in `e2e-tests.sh` requires
# that the variable `BUILD_NUMBER` be set, which is provided by Prow, so if we aren't running
# this from Prow we need to provide our own.
export BUILD_NUMBER=${BUILD_NUMBER:-$RANDOM}

# The scripts were initially setup to use the knative/serving style `DOCKER_REPO_OVERRIDE`
# before knative/serving was updated to use `ko` + `KO_DOCKER_REPO`. If the scripts were
# called with `KO_DOCKER_REPO` already set (i.e. using a user's own local cluster, we should
# respect that).
if ! [[ -z ${KO_DOCKER_REPO} ]]; then
    export DOCKER_REPO_OVERRIDE=${KO_DOCKER_REPO}
fi

function take_down_pipeline() {
    header "Tearing down Pipeline CRD"
    ko delete --ignore-not-found=true -f config/
}
# TODO: add a wait for resources to be up and change take_down_pipeline to
# teardown so that it will be called auto-magically.
trap take_down_pipeline EXIT

# Called by `fail_test` (provided by `e2e-tests.sh`) to dump info on test failure
function dump_extra_cluster_state() {
  for crd in pipelines pipelineruns tasks taskruns resources pipelineparams
  do
    echo ">>> $crd:"
    kubectl get $crd -o yaml --all-namespaces
  done
  echo ">>> Pipeline controller log:"
  kubectl -n knative-build-pipeline logs $(get_app_pod build-pipeline-controller knative-build-pipeline)
  echo ">>> Pipeline webhook log:"
  kubectl -n knative-build-pipeline logs $(get_app_pod build-pipeline-webhook knative-build-pipeline)
}

set +o xtrace
header "Setting up environment"
initialize $@
set -o xtrace

# If this was kicked off from Prow, then `DOCKER_REPO_OVERRIDE` will be set (by `initialize`) but
# `KO_DOCKER_REPO` will not be, and we need the latter to run `ko` so we should set that to
# the same value
if [[ -z ${KO_DOCKER_REPO} ]]; then
    export KO_DOCKER_REPO=${DOCKER_REPO_OVERRIDE}
fi

# Deploy the latest version of the Pipeline CRD.
# TODO(#59) do we need to deploy the Build CRD as well?
header "Deploying Pipeline CRD"
ko apply -f config/

# Wait for pods to be running in the namespaces we are deploying to
set +o xtrace
wait_until_pods_running knative-build-pipeline || fail_test "Pipeline CRD did not come up"
set -o xtrace

report_go_test \
-v -tags=e2e -count=1 -timeout=20m ./test \
${options} || fail_test

success
