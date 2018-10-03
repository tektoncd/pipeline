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

function take_down_pipeline() {
    header "Tearing down Pipeline CRD"
    ko delete --ignore-not-found=true -f config/
}
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
# The intialize method will attempt to create a new cluster in $PROJECT_ID unless
# the `--run-tests` parameter is provided and `K8S_CLUSTER_OVERRIDE` is set, however
# since knative/test-infra/scripts/presubmit-tests.sh doesn't propagate `--run-tests`
# (and it's kind of a confusing param name), we'll infer it from the presence of
# `K8S_CLUSTER_OVERRIDE`
if [[ -z ${K8S_CLUSTER_OVERRIDE} ]]; then
    initialize
else
    initialize --run-tests
fi
set -o xtrace

# The scripts were initially setup to use the knative/serving style `DOCKER_REPO_OVERRIDE`
# before knative/serving was updated to use `ko` + `KO_DOCKER_REPO`. If the scripts were
# called with `KO_DOCKER_REPO` already set (i.e. using a user's own local cluster, we should
# respect that).
if ! [[ -z ${KO_DOCKER_REPO} ]]; then
    export DOCKER_REPO_OVERRIDE=${KO_DOCKER_REPO}
fi

# Deploy the latest version of the Pipeline CRD.
# TODO(#59) do we need to deploy the Build CRD as well?
header "Deploying Pipeline CRD"
ko apply -f config/

# Wait for pods to be running in the namespaces we are deploying to
set +o xtrace
wait_until_pods_running knative-build-pipeline || fail_test "Pipeline CRD did not come up"
set -o xtrace
