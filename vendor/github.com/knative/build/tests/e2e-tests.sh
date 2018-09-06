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

# This script runs the end-to-end tests against the build controller
# built from source.
# It is started by prow for each PR.
# For convenience, it can also be executed manually.

# If you already have the *_OVERRIDE environment variables set, call
# this script with the --run-tests arguments and it will use the cluster
# and run the tests.

# Calling this script without arguments will create a new cluster in
# project $PROJECT_ID, start the controller, run the tests and delete
# the cluster.
# $KO_DOCKER_REPO must point to a valid writable docker repo.

source "$(dirname $(readlink -f ${BASH_SOURCE}))/library.sh"

# Test cluster parameters and location of generated test images
readonly E2E_CLUSTER_NAME=build-e2e-cluster${BUILD_NUMBER}
readonly E2E_NETWORK_NAME=build-e2e-net${BUILD_NUMBER}
readonly E2E_CLUSTER_ZONE=us-central1-a
readonly E2E_CLUSTER_NODES=2
readonly E2E_CLUSTER_MACHINE=n1-standard-2
readonly TEST_RESULT_FILE=/tmp/build-e2e-result

# This script.
readonly SCRIPT_CANONICAL_PATH="$(readlink -f ${BASH_SOURCE})"

# Helper functions.

function teardown() {
  header "Tearing down test environment"
  # Free resources in GCP project.
  if (( ! USING_EXISTING_CLUSTER )); then
    ko delete --ignore-not-found=true -R -f tests/
    ko delete --ignore-not-found=true -f config/
  fi

  # Delete images when using prow.
  if (( IS_PROW )); then
    echo "Images in ${KO_DOCKER_REPO}:"
    gcloud container images list --repository=${KO_DOCKER_REPO}
    delete_gcr_images ${KO_DOCKER_REPO}
  else
    restore_override_vars
  fi
}

function exit_if_test_failed() {
  [[ $? -eq 0 ]] && return 0
  echo "***************************************"
  echo "***           TEST FAILED           ***"
  echo "***    Start of information dump    ***"
  echo "***************************************"
  echo ">>> All resources:"
  kubectl get all --all-namespaces
  echo "***************************************"
  echo "***           TEST FAILED           ***"
  echo "***     End of information dump     ***"
  echo "***************************************"
  exit 1
}

function abort_test() {
  echo "$1"
  # If formatting fails for any reason, use yaml as a fall back.
  kubectl get builds -o=custom-columns-file=./tests/columns.txt || \
    kubectl get builds -oyaml
  false  # Force exit
  exit_if_test_failed
}

# Script entry point.

cd ${BUILD_ROOT_DIR}

# Show help if bad arguments are passed.
if [[ -n $1 && $1 != "--run-tests" ]]; then
  echo "usage: $0 [--run-tests]"
  exit 1
fi

# No argument provided, create the test cluster.

if [[ -z $1 ]]; then
  header "Creating test cluster"
  # Smallest cluster required to run the end-to-end-tests
  CLUSTER_CREATION_ARGS=(
    --gke-create-args="--enable-autoscaling --min-nodes=1 --max-nodes=${E2E_CLUSTER_NODES} --scopes=cloud-platform"
    --gke-shape={\"default\":{\"Nodes\":${E2E_CLUSTER_NODES}\,\"MachineType\":\"${E2E_CLUSTER_MACHINE}\"}}
    --provider=gke
    --deployment=gke
    --gcp-node-image=cos
    --cluster="${E2E_CLUSTER_NAME}"
    --gcp-zone="${E2E_CLUSTER_ZONE}"
    --gcp-network="${E2E_NETWORK_NAME}"
    --gke-environment=prod
  )
  if (( ! IS_PROW )); then
    CLUSTER_CREATION_ARGS+=(--gcp-project=${PROJECT_ID:?"PROJECT_ID must be set to the GCP project where the tests are run."})
  fi
  # SSH keys are not used, but kubetest checks for their existence.
  # Touch them so if they don't exist, empty files are create to satisfy the check.
  touch $HOME/.ssh/google_compute_engine.pub
  touch $HOME/.ssh/google_compute_engine
  # Clear user and cluster variables, so they'll be set to the test cluster.
  # KO_DOCKER_REPO is not touched because when running locally it must
  # be a writeable docker repo.
  export K8S_CLUSTER_OVERRIDE=
  # Assume test failed (see more details at the end of this script).
  echo -n "1"> ${TEST_RESULT_FILE}
  kubetest "${CLUSTER_CREATION_ARGS[@]}" \
    --up \
    --down \
    --extract "v${BUILD_GKE_VERSION}" \
    --test-cmd "${SCRIPT_CANONICAL_PATH}" \
    --test-cmd-args --run-tests
  result="$(cat ${TEST_RESULT_FILE})"
  echo "Test result code is $result"
  exit $result
fi

# --run-tests passed as first argument, run the tests.

# Set the required variables if necessary.

if [[ -z ${K8S_USER_OVERRIDE} ]]; then
  export K8S_USER_OVERRIDE=$(gcloud config get-value core/account)
fi

USING_EXISTING_CLUSTER=1
if [[ -z ${K8S_CLUSTER_OVERRIDE} ]]; then
  USING_EXISTING_CLUSTER=0
  export K8S_CLUSTER_OVERRIDE=$(kubectl config current-context)
  acquire_cluster_admin_role ${K8S_USER_OVERRIDE} ${E2E_CLUSTER_NAME} ${E2E_CLUSTER_ZONE}
  # Make sure we're in the default namespace
  kubectl config set-context $K8S_CLUSTER_OVERRIDE --namespace=default
fi
readonly USING_EXISTING_CLUSTER

if [[ -z ${KO_DOCKER_REPO} ]]; then
  export KO_DOCKER_REPO=gcr.io/$(gcloud config get-value project)/build-e2e-img
fi

# Build and start the controller.

echo "- Cluster is ${K8S_CLUSTER_OVERRIDE}"
echo "- User is ${K8S_USER_OVERRIDE}"
echo "- Docker is ${KO_DOCKER_REPO}"

header "Building and starting the controller"
trap teardown EXIT

install_ko

if (( USING_EXISTING_CLUSTER )); then
  echo "Deleting any previous controller instance"
  ko delete --ignore-not-found=true -f config/
fi
(( IS_PROW )) && gcr_auth

ko apply -f config/
exit_if_test_failed
# Make sure that are no builds or build templates in the current namespace.
kubectl delete builds --all
kubectl delete buildtemplates --all

# Run the tests

header "Running tests"

ko apply -R -f tests/
exit_if_test_failed

# Wait for tests to finish.
tests_finished=0
for i in {1..60}; do
  finished="$(kubectl get builds --output=jsonpath='{.items[*].status.conditions[*].status}')"
  echo $finished
  if [[ ! "$finished" == *"Unknown"* ]]; then
    tests_finished=1
    break
  fi
  sleep 5
done
(( tests_finished )) || abort_test "ERROR: tests timed out"

# Check that tests passed.
tests_passed=1
for expected_status in succeeded failed; do
  results="$(kubectl get builds -l expect=${expected_status} \
      --output=jsonpath='{range .items[*]}{.metadata.name}={.status.conditions[*].state}{.status.conditions[*].status}{" "}{end}')"
  case $expected_status in
    succeeded)
      want=succeededtrue
      ;;
    failed)
      want=succeededfalse
      ;;
    *)
      echo Invalid expected status $expected_status
      exit 1
  esac
  for result in ${results}; do
    if [[ ! "${result,,}" == *"=${want}" ]]; then
      echo "ERROR: test ${result} but should be ${want}"
      tests_passed=0
    fi
  done
done
(( tests_passed )) || abort_test "ERROR: one or more tests failed"

# kubetest teardown might fail and thus incorrectly report failure of the
# script, even if the tests pass.
# We store the real test result to return it later, ignoring any teardown
# failure in kubetest.
# TODO(adrcunha): Get rid of this workaround.
echo -n "0"> ${TEST_RESULT_FILE}
echo "**************************************"
echo "***        ALL TESTS PASSED        ***"
echo "**************************************"
exit 0
