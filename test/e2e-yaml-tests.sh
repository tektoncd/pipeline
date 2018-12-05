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
source $(dirname $0)/e2e-common.sh

# Handle failures ourselves, so we can dump useful info.
set +o errexit
set +o pipefail

echo ">> Deploying Build CRD"
kubectl apply -f ./third_party/config/build/release.yaml || fail_test "Build installation failed"

echo ">> Deploying Pipeline CRD"
ko apply -f config/ || fail_test "Build pipeline installation failed"

kubectl delete --ignore-not-found=true pipelineresources.pipeline.knative.dev --all
kubectl delete --ignore-not-found=true tasks.pipeline.knative.dev --all
kubectl delete --ignore-not-found=true pipelines.pipeline.knative.dev --all
kubectl delete --ignore-not-found=true taskruns.pipeline.knative.dev --all
kubectl delete --ignore-not-found=true pipelineruns.pipeline.knative.dev --all

# Run the tests
failed=0

# Run the smoke tests for the examples dir to make sure they are valid
# Run these _after_ the integration tests b/c they don't quite work all the way
# and they cause a lot of noise in the logs, making it harder to debug integration
# test failures.
header "Running YAML e2e tests for taskruns"
if ! run_yaml_tests "taskrun"; then
  failed=1
  echo "ERROR: one or more YAML tests failed"
  # If formatting fails for any reason, use yaml as a fall back.
  kubectl get taskrun.pipeline.knative.dev -o=custom-columns-file=./test/columns.txt || \
    kubectl get taskrun.pipeline.knative.dev -oyaml
fi
header "Running YAML e2e tests for pipelineruns"
if ! run_yaml_tests "pipelinerun"; then
  failed=1
  echo "ERROR: one or more YAML tests failed"
  # If formatting fails for any reason, use yaml as a fall back.
  kubectl get pipelinerun.pipeline.knative.dev -o=custom-columns-file=./test/columns.txt || \
    kubectl get pipelinerun.pipeline.knative.dev -oyaml
fi