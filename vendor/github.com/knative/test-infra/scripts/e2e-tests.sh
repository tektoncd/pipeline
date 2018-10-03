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

source $(dirname ${BASH_SOURCE})/library.sh

# Build a resource name based on $E2E_BASE_NAME, a suffix and $BUILD_NUMBER.
# Restricts the name length to 40 chars (the limit for resource names in GCP).
# Name will have the form $E2E_BASE_NAME-<PREFIX>$BUILD_NUMBER.
# Parameters: $1 - name suffix
function build_resource_name() {
  local prefix=${E2E_BASE_NAME}-$1
  local suffix=${BUILD_NUMBER}
  # Restrict suffix length to 20 chars
  if [[ -n "${suffix}" ]]; then
    suffix=${suffix:${#suffix}<20?0:-20}
  fi
  echo "${prefix:0:20}${suffix}"
}

# Test cluster parameters
readonly E2E_BASE_NAME=k$(basename ${REPO_ROOT_DIR})
readonly E2E_CLUSTER_NAME=$(build_resource_name e2e-cls)
readonly E2E_NETWORK_NAME=$(build_resource_name e2e-net)
readonly E2E_CLUSTER_REGION=us-central1
readonly E2E_CLUSTER_ZONE=${E2E_CLUSTER_REGION}-a
readonly E2E_CLUSTER_NODES=3
readonly E2E_CLUSTER_MACHINE=n1-standard-4
readonly TEST_RESULT_FILE=/tmp/${E2E_BASE_NAME}-e2e-result

# Tear down the test resources.
function teardown_test_resources() {
  header "Tearing down test environment"
  # Free resources in GCP project.
  if (( ! USING_EXISTING_CLUSTER )) && [[ "$(type -t teardown)" == "function" ]]; then
    teardown
  fi

  # Delete Knative Serving images when using prow.
  if (( IS_PROW )); then
    echo "Images in ${DOCKER_REPO_OVERRIDE}:"
    gcloud container images list --repository=${DOCKER_REPO_OVERRIDE}
    delete_gcr_images ${DOCKER_REPO_OVERRIDE}
  else
    # Delete the kubernetes source downloaded by kubetest
    rm -fr kubernetes kubernetes.tar.gz
  fi
}

# Exit test, dumping current state info.
# Parameters: $1 - error message (optional).
function fail_test() {
  [[ -n $1 ]] && echo "ERROR: $1"
  dump_cluster_state
  exit 1
}

# Run the given E2E tests (must be tagged as such).
# Parameters: $1..$n - directories containing the tests to run.
function go_test_e2e() {
  local options=""
  (( EMIT_METRICS )) && options="-emitmetrics"
  report_go_test -v -tags=e2e -count=1 -timeout=20m $@ ${options}
}

# Download the k8s binaries required by kubetest.
function download_k8s() {
  local version=${SERVING_GKE_VERSION}
  if [[ "${version}" == "latest" ]]; then
    # Fetch latest valid version
    local versions="$(gcloud container get-server-config \
        --project=${GCP_PROJECT} \
        --format='value(validMasterVersions)' \
        --region=${E2E_CLUSTER_REGION})"
    local gke_versions=(`echo -n ${versions//;/ /}`)
    # Get first (latest) version, excluding the "-gke.#" suffix
    version="${gke_versions[0]%-*}"
    echo "Latest GKE is ${version}, from [${versions//;/, }]"
  elif [[ "${version}" == "default" ]]; then
    echo "ERROR: `default` GKE version is not supported yet"
    return 1
  fi
  # Download k8s to staging dir
  version=v${version}
  local staging_dir=${GOPATH}/src/k8s.io/kubernetes/_output/gcs-stage
  rm -fr ${staging_dir}
  staging_dir=${staging_dir}/${version}
  mkdir -p ${staging_dir}
  pushd ${staging_dir}
  export KUBERNETES_PROVIDER=gke
  export KUBERNETES_RELEASE=${version}
  curl -fsSL https://get.k8s.io | bash
  local result=$?
  if [[ ${result} -eq 0 ]]; then
    mv kubernetes/server/kubernetes-server-*.tar.gz .
    mv kubernetes/client/kubernetes-client-*.tar.gz .
    rm -fr kubernetes
    # Create an empty kubernetes test tarball; we don't use it but kubetest will fetch it
    tar -czf kubernetes-test.tar.gz -T /dev/null
  fi
  popd
  return ${result}
}

# Dump info about the test cluster. If dump_extra_cluster_info() is defined, calls it too.
# This is intended to be called when a test fails to provide debugging information.
function dump_cluster_state() {
  echo "***************************************"
  echo "***           TEST FAILED           ***"
  echo "***    Start of information dump    ***"
  echo "***************************************"
  echo ">>> All resources:"
  kubectl get all --all-namespaces
  echo ">>> Services:"
  kubectl get services --all-namespaces
  echo ">>> Events:"
  kubectl get events --all-namespaces
  [[ "$(type -t dump_extra_cluster_state)" == "function" ]] && dump_extra_cluster_state
  echo "***************************************"
  echo "***           TEST FAILED           ***"
  echo "***     End of information dump     ***"
  echo "***************************************"
}

# Create a test cluster with kubetest and call the current script again.
function create_test_cluster() {
  # Fail fast during setup.
  set -o errexit
  set -o pipefail

  header "Creating test cluster"
  # Smallest cluster required to run the end-to-end-tests
  local CLUSTER_CREATION_ARGS=(
    --gke-create-args="--enable-autoscaling --min-nodes=1 --max-nodes=${E2E_CLUSTER_NODES} --scopes=cloud-platform --enable-basic-auth --no-issue-client-certificate"
    --gke-shape={\"default\":{\"Nodes\":${E2E_CLUSTER_NODES}\,\"MachineType\":\"${E2E_CLUSTER_MACHINE}\"}}
    --provider=gke
    --deployment=gke
    --cluster="${E2E_CLUSTER_NAME}"
    --gcp-zone="${E2E_CLUSTER_ZONE}"
    --gcp-network="${E2E_NETWORK_NAME}"
    --gke-environment=prod
  )
  if (( ! IS_PROW )); then
    CLUSTER_CREATION_ARGS+=(--gcp-project=${PROJECT_ID:?"PROJECT_ID must be set to the GCP project where the tests are run."})
  else
    CLUSTER_CREATION_ARGS+=(--gcp-service-account=/etc/service-account/service-account.json)
  fi
  # SSH keys are not used, but kubetest checks for their existence.
  # Touch them so if they don't exist, empty files are create to satisfy the check.
  mkdir -p $HOME/.ssh
  touch $HOME/.ssh/google_compute_engine.pub
  touch $HOME/.ssh/google_compute_engine
  # Clear user and cluster variables, so they'll be set to the test cluster.
  # DOCKER_REPO_OVERRIDE is not touched because when running locally it must
  # be a writeable docker repo.
  export K8S_USER_OVERRIDE=
  export K8S_CLUSTER_OVERRIDE=
  # Get the current GCP project
  export GCP_PROJECT=${PROJECT_ID}
  [[ -z ${GCP_PROJECT} ]] && export GCP_PROJECT=$(gcloud config get-value project)
  # Assume test failed (see more details at the end of this script).
  echo -n "1"> ${TEST_RESULT_FILE}
  local test_cmd_args="--run-tests"
  (( EMIT_METRICS )) && test_cmd_args+=" --emit-metrics"
  echo "Test script is ${E2E_SCRIPT}"
  download_k8s || return 1
  # Don't fail test for kubetest, as it might incorrectly report test failure
  # if teardown fails (for details, see success() below)
  set +o errexit
  kubetest "${CLUSTER_CREATION_ARGS[@]}" \
    --up \
    --down \
    --extract local \
    --gcp-node-image ${SERVING_GKE_IMAGE} \
    --test-cmd "${E2E_SCRIPT}" \
    --test-cmd-args "${test_cmd_args}"
  echo "Test subprocess exited with code $?"
  # Ignore any errors below, this is a best-effort cleanup and shouldn't affect the test result.
  set +o errexit
  # Delete target pools and health checks that might have leaked.
  # See https://github.com/knative/serving/issues/959 for details.
  # TODO(adrcunha): Remove once the leak issue is resolved.
  local http_health_checks="$(gcloud compute target-pools list \
    --project=${GCP_PROJECT} --format='value(healthChecks)' --filter="instances~-${E2E_CLUSTER_NAME}-" | \
    grep httpHealthChecks | tr '\n' ' ')"
  local target_pools="$(gcloud compute target-pools list \
    --project=${GCP_PROJECT} --format='value(name)' --filter="instances~-${E2E_CLUSTER_NAME}-" | \
    tr '\n' ' ')"
  if [[ -n "${target_pools}" ]]; then
    echo "Found leaked target pools, deleting"
    gcloud compute forwarding-rules delete -q --project=${GCP_PROJECT} --region=${E2E_CLUSTER_REGION} ${target_pools}
    gcloud compute target-pools delete -q --project=${GCP_PROJECT} --region=${E2E_CLUSTER_REGION} ${target_pools}
  fi
  if [[ -n "${http_health_checks}" ]]; then
    echo "Found leaked health checks, deleting"
    gcloud compute http-health-checks delete -q --project=${GCP_PROJECT} ${http_health_checks}
  fi
  local result="$(cat ${TEST_RESULT_FILE})"
  echo "Test result code is $result"
  exit ${result}
}

# Setup the test cluster for running the tests.
function setup_test_cluster() {
  # Fail fast during setup.
  set -o errexit
  set -o pipefail

  # Set the required variables if necessary.
  if [[ -z ${K8S_USER_OVERRIDE} ]]; then
    export K8S_USER_OVERRIDE=$(gcloud config get-value core/account)
  fi

  if [[ -z ${K8S_CLUSTER_OVERRIDE} ]]; then
    USING_EXISTING_CLUSTER=0
    export K8S_CLUSTER_OVERRIDE=$(kubectl config current-context)
    acquire_cluster_admin_role ${K8S_USER_OVERRIDE} ${E2E_CLUSTER_NAME} ${E2E_CLUSTER_ZONE}
    # Make sure we're in the default namespace. Currently kubetest switches to
    # test-pods namespace when creating the cluster.
    kubectl config set-context $K8S_CLUSTER_OVERRIDE --namespace=default
  fi
  readonly USING_EXISTING_CLUSTER

  if [[ -z ${DOCKER_REPO_OVERRIDE} ]]; then
    export DOCKER_REPO_OVERRIDE=gcr.io/$(gcloud config get-value project)/${E2E_BASE_NAME}-e2e-img
  fi

  echo "- Cluster is ${K8S_CLUSTER_OVERRIDE}"
  echo "- User is ${K8S_USER_OVERRIDE}"
  echo "- Docker is ${DOCKER_REPO_OVERRIDE}"

  trap teardown_test_resources EXIT

  if (( USING_EXISTING_CLUSTER )) && [[ "$(type -t teardown)" == "function" ]]; then
    echo "Deleting any previous SUT instance"
    teardown
  fi

  readonly K8S_CLUSTER_OVERRIDE
  readonly K8S_USER_OVERRIDE
  readonly DOCKER_REPO_OVERRIDE

  # Handle failures ourselves, so we can dump useful info.
  set +o errexit
  set +o pipefail
}

function success() {
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
}

RUN_TESTS=0
EMIT_METRICS=0
USING_EXISTING_CLUSTER=1
E2E_SCRIPT=""

# Parse flags and initialize the test cluster.
function initialize() {
  # Normalize calling script path; we can't use readlink because it's not available everywhere
  E2E_SCRIPT=$0
  [[ ${E2E_SCRIPT} =~ ^[\./].* ]] || E2E_SCRIPT="./$0"
  E2E_SCRIPT="$(cd ${E2E_SCRIPT%/*} && echo $PWD/${E2E_SCRIPT##*/})"
  readonly E2E_SCRIPT

  cd ${REPO_ROOT_DIR}
  for parameter in $@; do
    case $parameter in
      --run-tests) RUN_TESTS=1 ;;
      --emit-metrics) EMIT_METRICS=1 ;;
      *)
        echo "error: unknown option ${parameter}"
        echo "usage: $0 [--run-tests][--emit-metrics]"
        exit 1
        ;;
    esac
    shift
  done
  readonly RUN_TESTS
  readonly EMIT_METRICS

  if (( ! RUN_TESTS )); then
    create_test_cluster
  else
    setup_test_cluster
  fi
}
