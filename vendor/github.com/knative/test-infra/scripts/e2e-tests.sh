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

# This is a helper script for Knative E2E test scripts.
# See README.md for instructions on how to use it.

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
  local name="${prefix:0:20}${suffix}"
  # Ensure name doesn't end with "-"
  echo "${name%-}"
}

# Test cluster parameters

# Configurable parameters
readonly E2E_CLUSTER_REGION=${E2E_CLUSTER_REGION:-us-central1}
# By default we use regional clusters.
readonly E2E_CLUSTER_ZONE=${E2E_CLUSTER_ZONE:-}
readonly E2E_CLUSTER_MACHINE=${E2E_CLUSTER_MACHINE:-n1-standard-4}

# Each knative repository may have a different cluster size requirement here,
# so we allow calling code to set these parameters.  If they are not set we
# use some sane defaults.
readonly E2E_MIN_CLUSTER_NODES=${E2E_MIN_CLUSTER_NODES:-1}
readonly E2E_MAX_CLUSTER_NODES=${E2E_MAX_CLUSTER_NODES:-3}

readonly E2E_BASE_NAME="k${REPO_NAME}"
readonly E2E_CLUSTER_NAME=$(build_resource_name e2e-cls)
readonly E2E_NETWORK_NAME=$(build_resource_name e2e-net)
readonly TEST_RESULT_FILE=/tmp/${E2E_BASE_NAME}-e2e-result

# Flag whether test is using a boskos GCP project
IS_BOSKOS=0

# Tear down the test resources.
function teardown_test_resources() {
  header "Tearing down test environment"
  # Free resources in GCP project.
  if (( ! USING_EXISTING_CLUSTER )) && function_exists teardown; then
    teardown
  fi

  # Delete the kubernetes source downloaded by kubetest
  rm -fr kubernetes kubernetes.tar.gz
}

# Run the given E2E tests. Assume tests are tagged e2e, unless `-tags=XXX` is passed.
# Parameters: $1..$n - any go test flags, then directories containing the tests to run.
function go_test_e2e() {
  local test_options=""
  local go_options=""
  (( EMIT_METRICS )) && test_options="-emitmetrics"
  [[ ! " $@" == *" -tags="* ]] && go_options="-tags=e2e"
  report_go_test -v -count=1 ${go_options} $@ ${test_options}
}

# Download the k8s binaries required by kubetest.
# Parameters: $1 - GCP project that will host the test cluster.
function download_k8s() {
  # Fetch valid versions
  local versions="$(gcloud container get-server-config \
      --project=$1 \
      --format='value(validMasterVersions)' \
      --region=${E2E_CLUSTER_REGION})"
  local gke_versions=(`echo -n ${versions//;/ /}`)
  echo "Valid GKE versions are [${versions//;/, }]"
  if [[ "${E2E_CLUSTER_VERSION}" == "latest" ]]; then
    # Get first (latest) version, excluding the "-gke.#" suffix
    E2E_CLUSTER_VERSION="${gke_versions[0]%-*}"
    echo "Using latest version, ${E2E_CLUSTER_VERSION}"
  elif [[ "${E2E_CLUSTER_VERSION}" == "default" ]]; then
    echo "ERROR: `default` GKE version is not supported yet"
    return 1
  else
    echo "Using command-line supplied version ${E2E_CLUSTER_VERSION}"
  fi
  readonly E2E_CLUSTER_VERSION
  export KUBERNETES_PROVIDER=gke
  export KUBERNETES_RELEASE=v${E2E_CLUSTER_VERSION}
  # Download k8s to staging dir
  local staging_dir=${GOPATH}/src/k8s.io/kubernetes/_output/gcs-stage
  rm -fr ${staging_dir}
  staging_dir=${staging_dir}/${KUBERNETES_RELEASE}
  mkdir -p ${staging_dir}
  pushd ${staging_dir}
  curl -fsSL https://get.k8s.io | bash
  local result=$?
  if [[ ${result} -eq 0 ]]; then
    mv kubernetes/server/kubernetes-server-*.tar.gz .
    mv kubernetes/client/kubernetes-client-*.tar.gz .
    rm -fr kubernetes
    # Create an empty kubernetes test tarball; we don't use it but kubetest will fetch it
    # As of August 21 2018 this means avoiding a useless 1.2GB download
    tar -czf kubernetes-test.tar.gz -T /dev/null
  fi
  popd
  return ${result}
}

# Dump info about the test cluster. If dump_extra_cluster_info() is defined, calls it too.
# This is intended to be called when a test fails to provide debugging information.
function dump_cluster_state() {
  echo "***************************************"
  echo "***         E2E TEST FAILED         ***"
  echo "***    Start of information dump    ***"
  echo "***************************************"
  echo ">>> All resources:"
  kubectl get all --all-namespaces
  echo ">>> Services:"
  kubectl get services --all-namespaces
  echo ">>> Events:"
  kubectl get events --all-namespaces
  function_exists dump_extra_cluster_state && dump_extra_cluster_state
  echo "***************************************"
  echo "***         E2E TEST FAILED         ***"
  echo "***     End of information dump     ***"
  echo "***************************************"
}

# On a Prow job, save some metadata about the test for Testgrid.
function save_metadata() {
  (( ! IS_PROW )) && return
  local geo_key="Region"
  local geo_value="${E2E_CLUSTER_REGION}"
  if [[ -n "${E2E_CLUSTER_ZONE}" ]]; then
    geo_key="Zone"
    geo_value="${E2E_CLUSTER_REGION}-${E2E_CLUSTER_ZONE}"
  fi
  cat << EOF > ${ARTIFACTS}/metadata.json
{
  "E2E:${geo_key}": "${geo_value}",
  "E2E:Machine": "${E2E_CLUSTER_MACHINE}",
  "E2E:Version": "${E2E_CLUSTER_VERSION}",
  "E2E:MinNodes": "${E2E_MIN_CLUSTER_NODES}",
  "E2E:MaxNodes": "${E2E_MAX_CLUSTER_NODES}"
}
EOF
}

# Create a test cluster with kubetest and call the current script again.
function create_test_cluster() {
  # Fail fast during setup.
  set -o errexit
  set -o pipefail

  header "Creating test cluster"

  echo "Cluster will have a minimum of ${E2E_MIN_CLUSTER_NODES} and a maximum of ${E2E_MAX_CLUSTER_NODES} nodes."

  # Smallest cluster required to run the end-to-end-tests
  local geoflag="--gcp-region=${E2E_CLUSTER_REGION}"
  [[ -n "${E2E_CLUSTER_ZONE}" ]] && geoflag="--gcp-zone=${E2E_CLUSTER_REGION}-${E2E_CLUSTER_ZONE}"
  local CLUSTER_CREATION_ARGS=(
    --gke-create-command="beta container clusters create --quiet --enable-autoscaling --min-nodes=${E2E_MIN_CLUSTER_NODES} --max-nodes=${E2E_MAX_CLUSTER_NODES} --scopes=cloud-platform --enable-basic-auth --no-issue-client-certificate ${EXTRA_CLUSTER_CREATION_FLAGS[@]}"
    --gke-shape={\"default\":{\"Nodes\":${E2E_MIN_CLUSTER_NODES}\,\"MachineType\":\"${E2E_CLUSTER_MACHINE}\"}}
    --provider=gke
    --deployment=gke
    --cluster="${E2E_CLUSTER_NAME}"
    ${geoflag}
    --gcp-network="${E2E_NETWORK_NAME}"
    --gke-environment=prod
  )
  if (( ! IS_BOSKOS )); then
    CLUSTER_CREATION_ARGS+=(--gcp-project=${GCP_PROJECT})
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
  # Assume test failed (see details in set_test_return_code()).
  set_test_return_code 1
  # Get the current GCP project for downloading kubernetes
  local gcloud_project="${GCP_PROJECT}"
  [[ -z "${gcloud_project}" ]] && gcloud_project="$(gcloud config get-value project)"
  echo "gcloud project is ${gcloud_project}"
  (( IS_BOSKOS )) && echo "Using boskos for the test cluster"
  [[ -n "${GCP_PROJECT}" ]] && echo "GCP project for test cluster is ${GCP_PROJECT}"
  echo "Test script is ${E2E_SCRIPT}"
  download_k8s ${gcloud_project} || return 1
  # Save some metadata about cluster creation for using in prow and testgrid
  save_metadata
  # Set arguments for this script again
  local test_cmd_args="--run-tests --cluster-version ${E2E_CLUSTER_VERSION}"
  (( EMIT_METRICS )) && test_cmd_args+=" --emit-metrics"
  [[ -n "${GCP_PROJECT}" ]] && test_cmd_args+=" --gcp-project ${GCP_PROJECT}"
  [[ -n "${E2E_SCRIPT_CUSTOM_FLAGS[@]}" ]] && test_cmd_args+=" ${E2E_SCRIPT_CUSTOM_FLAGS[@]}"
  # Don't fail test for kubetest, as it might incorrectly report test failure
  # if teardown fails (for details, see success() below)
  set +o errexit
  run_go_tool k8s.io/test-infra/kubetest \
    kubetest "${CLUSTER_CREATION_ARGS[@]}" \
    --up \
    --down \
    --extract local \
    --gcp-node-image "${SERVING_GKE_IMAGE}" \
    --test-cmd "${E2E_SCRIPT}" \
    --test-cmd-args "${test_cmd_args}"
  echo "Test subprocess exited with code $?"
  # Ignore any errors below, this is a best-effort cleanup and shouldn't affect the test result.
  set +o errexit
  # Ensure we're using the GCP project used by kubetest
  gcloud_project="$(gcloud config get-value project)"
  # Delete target pools and health checks that might have leaked.
  # See https://github.com/knative/serving/issues/959 for details.
  # TODO(adrcunha): Remove once the leak issue is resolved.
  local http_health_checks="$(gcloud compute target-pools list \
    --project=${gcloud_project} --format='value(healthChecks)' --filter="instances~-${E2E_CLUSTER_NAME}-" | \
    grep httpHealthChecks | tr '\n' ' ')"
  local target_pools="$(gcloud compute target-pools list \
    --project=${gcloud_project} --format='value(name)' --filter="instances~-${E2E_CLUSTER_NAME}-" | \
    tr '\n' ' ')"
  if [[ -n "${target_pools}" ]]; then
    echo "Found leaked target pools, deleting"
    gcloud compute forwarding-rules delete -q --project=${gcloud_project} --region=${E2E_CLUSTER_REGION} ${target_pools}
    gcloud compute target-pools delete -q --project=${gcloud_project} --region=${E2E_CLUSTER_REGION} ${target_pools}
  fi
  if [[ -n "${http_health_checks}" ]]; then
    echo "Found leaked health checks, deleting"
    gcloud compute http-health-checks delete -q --project=${gcloud_project} ${http_health_checks}
  fi
  local result="$(cat ${TEST_RESULT_FILE})"
  echo "Test result code is ${result}"

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
    acquire_cluster_admin_role ${K8S_USER_OVERRIDE} ${E2E_CLUSTER_NAME} ${E2E_CLUSTER_REGION} ${E2E_CLUSTER_ZONE}
    # Make sure we're in the default namespace. Currently kubetest switches to
    # test-pods namespace when creating the cluster.
    kubectl config set-context ${K8S_CLUSTER_OVERRIDE} --namespace=default
  fi
  readonly USING_EXISTING_CLUSTER

  if [[ -z ${DOCKER_REPO_OVERRIDE} ]]; then
    export DOCKER_REPO_OVERRIDE=gcr.io/$(gcloud config get-value project)/${E2E_BASE_NAME}-e2e-img
  fi

  echo "- Cluster is ${K8S_CLUSTER_OVERRIDE}"
  echo "- User is ${K8S_USER_OVERRIDE}"
  echo "- Docker is ${DOCKER_REPO_OVERRIDE}"

  export KO_DOCKER_REPO="${DOCKER_REPO_OVERRIDE}"
  export KO_DATA_PATH="${REPO_ROOT_DIR}/.git"

  trap teardown_test_resources EXIT

  if (( USING_EXISTING_CLUSTER )) && function_exists teardown; then
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

# Set the return code that the test script will return.
# Parameters: $1 - return code (0-255)
function set_test_return_code() {
  # kubetest teardown might fail and thus incorrectly report failure of the
  # script, even if the tests pass.
  # We store the real test result to return it later, ignoring any teardown
  # failure in kubetest.
  # TODO(adrcunha): Get rid of this workaround.
  echo -n "$1"> ${TEST_RESULT_FILE}
}

function success() {
  set_test_return_code 0
  echo "**************************************"
  echo "***        E2E TESTS PASSED        ***"
  echo "**************************************"
  exit 0
}

# Exit test, dumping current state info.
# Parameters: $1 - error message (optional).
function fail_test() {
  set_test_return_code 1
  [[ -n $1 ]] && echo "ERROR: $1"
  dump_cluster_state
  exit 1
}

RUN_TESTS=0
EMIT_METRICS=0
USING_EXISTING_CLUSTER=1
GCP_PROJECT=""
E2E_SCRIPT=""
E2E_CLUSTER_VERSION=""
EXTRA_CLUSTER_CREATION_FLAGS=()
E2E_SCRIPT_CUSTOM_FLAGS=()

# Parse flags and initialize the test cluster.
function initialize() {
  E2E_SCRIPT="$(get_canonical_path $0)"
  E2E_CLUSTER_VERSION="${SERVING_GKE_VERSION}"

  cd ${REPO_ROOT_DIR}
  while [[ $# -ne 0 ]]; do
    local parameter=$1
    # Try parsing flag as a custom one.
    if function_exists parse_flags; then
      parse_flags $@
      local skip=$?
      if [[ ${skip} -ne 0 ]]; then
        # Skip parsed flag (and possibly argument) and continue
        # Also save it to it's passed through to the test script
        for ((i=1;i<=skip;i++)); do
          E2E_SCRIPT_CUSTOM_FLAGS+=("$1")
          shift
        done
        continue
      fi
    fi
    # Try parsing flag as a standard one.
    case ${parameter} in
      --run-tests) RUN_TESTS=1 ;;
      --emit-metrics) EMIT_METRICS=1 ;;
      *)
        [[ $# -ge 2 ]] || abort "missing parameter after $1"
        shift
        case ${parameter} in
          --gcp-project) GCP_PROJECT=$1 ;;
          --cluster-version)
            [[ $1 =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || abort "kubernetes version must be 'X.Y.Z'"
            E2E_CLUSTER_VERSION=$1
            ;;
          --cluster-creation-flag) EXTRA_CLUSTER_CREATION_FLAGS+=($1) ;;
          *) abort "unknown option ${parameter}" ;;
        esac
    esac
    shift
  done

  # Use PROJECT_ID if set, unless --gcp-project was used.
  if [[ -n "${PROJECT_ID:-}" && -z "${GCP_PROJECT}" ]]; then
    echo "\$PROJECT_ID is set to '${PROJECT_ID}', using it to run the tests"
    GCP_PROJECT="${PROJECT_ID}"
  fi
  if (( ! IS_PROW )) && [[ -z "${GCP_PROJECT}" ]]; then
    abort "set \$PROJECT_ID or use --gcp-project to select the GCP project where the tests are run"
  fi

  (( IS_PROW )) && [[ -z "${GCP_PROJECT}" ]] && IS_BOSKOS=1

  # Safety checks
  is_protected_gcr ${DOCKER_REPO_OVERRIDE} && \
    abort "\$DOCKER_REPO_OVERRIDE set to ${DOCKER_REPO_OVERRIDE}, which is forbidden"

  readonly RUN_TESTS
  readonly EMIT_METRICS
  readonly GCP_PROJECT
  readonly IS_BOSKOS
  readonly EXTRA_CLUSTER_CREATION_FLAGS

  if (( ! RUN_TESTS )); then
    create_test_cluster
  else
    setup_test_cluster
  fi
}
