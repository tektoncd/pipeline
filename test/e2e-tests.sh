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

function test_setup() {
  # Setup a service account for kaniko and gcs / storage artifact tests
  gcloud config set project "${E2E_PROJECT_ID}"

  # create the service account
  ACCOUNT_NAME=${E2E_PROJECT_ID}
  gcloud iam service-accounts create $ACCOUNT_NAME --display-name $ACCOUNT_NAME
  EMAIL=$(gcloud iam service-accounts list | grep $ACCOUNT_NAME | awk '{print $2}')

  # add the storage.admin policy to the account so it can push containers
  gcloud projects add-iam-policy-binding ${E2E_PROJECT_ID} --member serviceAccount:$EMAIL --role roles/storage.admin

  # create the JSON key
  gcloud iam service-accounts keys create config.json --iam-account $EMAIL

  export GCP_SERVICE_ACCOUNT_KEY_PATH="$PWD/config.json"
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

function run_e2e() {
  # Run the integration tests
  header "Running Go e2e tests"
  go_test_e2e -timeout=20m ./test/... || failed=1

  # Run these _after_ the integration tests b/c they don't quite work all the way
  # and they cause a lot of noise in the logs, making it harder to debug integration
  # test failures.
  go_test_e2e -tags=examples -timeout=20m ./test/ || failed=1
}

# Script entry point.

initialize $@

header "Setting up environment"

install_pipeline_crd

failed=0

if [ "$PIPELINE_FEATURE_GATE" == "" ]; then
  set_feature_gate "stable"
  run_e2e
else
  set_feature_gate "$PIPELINE_FEATURE_GATE"
  run_e2e
fi

(( failed )) && fail_test
success
