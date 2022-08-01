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

# This is a helper script for Tekton E2E test scripts.
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
# export E2E_CLUSTER_REGION and E2E_CLUSTER_ZONE as they're used in the cluster setup subprocess
export E2E_CLUSTER_REGION=${E2E_CLUSTER_REGION:-us-central1}
# By default we use regional clusters.
export E2E_CLUSTER_ZONE=${E2E_CLUSTER_ZONE:-}

# Default backup regions in case of stockouts; by default we don't fall back to a different zone in the same region
readonly E2E_CLUSTER_BACKUP_REGIONS=${E2E_CLUSTER_BACKUP_REGIONS:-us-west1 us-east1}
readonly E2E_CLUSTER_BACKUP_ZONES=${E2E_CLUSTER_BACKUP_ZONES:-}

readonly E2E_CLUSTER_MACHINE=${E2E_CLUSTER_MACHINE:-n1-standard-4}
readonly E2E_GKE_ENVIRONMENT=${E2E_GKE_ENVIRONMENT:-prod}
readonly E2E_GKE_COMMAND_GROUP=${E2E_GKE_COMMAND_GROUP:-beta}

# Each tekton repository may have a different cluster size requirement here,
# so we allow calling code to set these parameters.  If they are not set we
# use some sane defaults.
readonly E2E_MIN_CLUSTER_NODES=${E2E_MIN_CLUSTER_NODES:-1}
readonly E2E_MAX_CLUSTER_NODES=${E2E_MAX_CLUSTER_NODES:-3}

readonly E2E_BASE_NAME="t${REPO_NAME}"
readonly E2E_CLUSTER_NAME=$(build_resource_name e2e-cls)
readonly E2E_NETWORK_NAME=$(build_resource_name e2e-net)
readonly TEST_RESULT_FILE=/tmp/${E2E_BASE_NAME}-e2e-result

# Flag whether test is using a boskos GCP project
IS_BOSKOS=0

# Tear down the test resources.
function teardown_test_resources() {
  # On boskos, save time and don't teardown as the cluster will be destroyed anyway.
  (( IS_BOSKOS )) && return
  header "Tearing down test environment"
  function_exists test_teardown && test_teardown
  (( ! SKIP_TEKTON_SETUP )) && function_exists tekton_teardown && tekton_teardown
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
  local cluster_version="$(gcloud container clusters list --project=${E2E_PROJECT_ID} --format='value(currentMasterVersion)')"
  cat << EOF > ${ARTIFACTS}/metadata.json
{
  "E2E:${geo_key}": "${geo_value}",
  "E2E:Machine": "${E2E_CLUSTER_MACHINE}",
  "E2E:Version": "${cluster_version}",
  "E2E:MinNodes": "${E2E_MIN_CLUSTER_NODES}",
  "E2E:MaxNodes": "${E2E_MAX_CLUSTER_NODES}"
}
EOF
}

# Delete target pools and health checks that might have leaked.
# See https://github.com/knative/serving/issues/959 for details.
# TODO(adrcunha): Remove once the leak issue is resolved.
function delete_leaked_network_resources() {
  # On boskos, don't bother with leaks as the janitor will delete everything in the project.
  (( IS_BOSKOS )) && return
  # Ensure we're using the GCP project used by kubetest
  local gcloud_project="$(gcloud config get-value project)"
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
}

# Create a test cluster with kubetest and call the current script again.
function create_test_cluster() {
  # Fail fast during setup.
  set -o errexit
  set -o pipefail

  if function_exists cluster_setup; then
    cluster_setup || fail_test "cluster setup failed"
  fi

  echo "Cluster will have a minimum of ${E2E_MIN_CLUSTER_NODES} and a maximum of ${E2E_MAX_CLUSTER_NODES} nodes."

  # Smallest cluster required to run the end-to-end-tests
  local CLUSTER_CREATION_ARGS=(
    --gke-create-command="container clusters create --quiet --enable-autoscaling --min-nodes=${E2E_MIN_CLUSTER_NODES} --max-nodes=${E2E_MAX_CLUSTER_NODES} --scopes=cloud-platform --no-issue-client-certificate ${EXTRA_CLUSTER_CREATION_FLAGS[@]}"
    --gke-shape={\"default\":{\"Nodes\":${E2E_MIN_CLUSTER_NODES}\,\"MachineType\":\"${E2E_CLUSTER_MACHINE}\"}}
    --provider=gke
    --deployment=gke
    --cluster="${E2E_CLUSTER_NAME}"
    --gcp-network="${E2E_NETWORK_NAME}"
    --gke-environment="${E2E_GKE_ENVIRONMENT}"
    --gke-command-group="${E2E_GKE_COMMAND_GROUP}"
    --test=false
  )
  if (( ! IS_BOSKOS )); then
    CLUSTER_CREATION_ARGS+=(--gcp-project=${GCP_PROJECT})
  fi
  # SSH keys are not used, but kubetest checks for their existence.
  # Touch them so if they don't exist, empty files are create to satisfy the check.
  mkdir -p $HOME/.ssh
  touch $HOME/.ssh/google_compute_engine.pub
  touch $HOME/.ssh/google_compute_engine
  # Assume test failed (see details in set_test_return_code()).
  set_test_return_code 1
  local gcloud_project="${GCP_PROJECT}"
  [[ -z "${gcloud_project}" ]] && gcloud_project="$(gcloud config get-value project)"
  echo "gcloud project is ${gcloud_project}"
  (( IS_BOSKOS )) && echo "Using boskos for the test cluster"
  [[ -n "${GCP_PROJECT}" ]] && echo "GCP project for test cluster is ${GCP_PROJECT}"
  echo "Test script is ${E2E_SCRIPT}"
  # Set arguments for this script again
  local test_cmd_args="--run-tests"
  (( EMIT_METRICS )) && test_cmd_args+=" --emit-metrics"
  (( SKIP_TEKTON_SETUP )) && test_cmd_args+=" --skip-tekton-setup"
  [[ -n "${GCP_PROJECT}" ]] && test_cmd_args+=" --gcp-project ${GCP_PROJECT}"
  [[ -n "${E2E_SCRIPT_CUSTOM_FLAGS[@]}" ]] && test_cmd_args+=" ${E2E_SCRIPT_CUSTOM_FLAGS[@]}"
  local extra_flags=()
  # If using boskos, save time and let it tear down the cluster
  (( ! IS_BOSKOS )) && extra_flags+=(--down)
  create_test_cluster_with_retries "${CLUSTER_CREATION_ARGS[@]}" \
    --up \
    --extract "${E2E_CLUSTER_VERSION}" \
    --gcp-node-image "${SERVING_GKE_IMAGE}" \
    --test-cmd "${E2E_SCRIPT}" \
    --test-cmd-args "${test_cmd_args}" \
    ${extra_flags[@]} \
    ${EXTRA_KUBETEST_FLAGS[@]}
  echo "Test subprocess exited with code $?"
  # Ignore any errors below, this is a best-effort cleanup and shouldn't affect the test result.
  set +o errexit
  function_exists cluster_teardown && cluster_teardown
  delete_leaked_network_resources
  local result=$(get_test_return_code)
  echo "Artifacts were written to ${ARTIFACTS}"
  echo "Test result code is ${result}"
  exit ${result}
}

# Retry backup regions/zones if cluster creations failed due to stockout.
# Parameters: $1..$n - any kubetest flags other than geo flag.
function create_test_cluster_with_retries() {
  local cluster_creation_log=/tmp/${E2E_BASE_NAME}-cluster_creation-log
  # zone_not_provided is a placeholder for e2e_cluster_zone to make for loop below work
  local zone_not_provided="zone_not_provided"

  local e2e_cluster_regions=(${E2E_CLUSTER_REGION})
  local e2e_cluster_zones=(${E2E_CLUSTER_ZONE})

  if [[ -n "${E2E_CLUSTER_BACKUP_ZONES}" ]]; then
    e2e_cluster_zones+=(${E2E_CLUSTER_BACKUP_ZONES})
  elif [[ -n "${E2E_CLUSTER_BACKUP_REGIONS}" ]]; then
    e2e_cluster_regions+=(${E2E_CLUSTER_BACKUP_REGIONS})
    e2e_cluster_zones=(${zone_not_provided})
  else
    echo "No backup region/zone set, cluster creation will fail in case of stockout"
  fi

  for e2e_cluster_region in "${e2e_cluster_regions[@]}"; do
    for e2e_cluster_zone in "${e2e_cluster_zones[@]}"; do
      E2E_CLUSTER_REGION=${e2e_cluster_region}
      E2E_CLUSTER_ZONE=${e2e_cluster_zone}
      [[ "${E2E_CLUSTER_ZONE}" == "${zone_not_provided}" ]] && E2E_CLUSTER_ZONE=""

      local geoflag="--gcp-region=${E2E_CLUSTER_REGION}"
      [[ -n "${E2E_CLUSTER_ZONE}" ]] && geoflag="--gcp-zone=${E2E_CLUSTER_REGION}-${E2E_CLUSTER_ZONE}"

      header "Creating test cluster in $E2E_CLUSTER_REGION $E2E_CLUSTER_ZONE"
      # Don't fail test for kubetest, as it might incorrectly report test failure
      # if teardown fails (for details, see success() below)
      set +o errexit
      { run_go_tool k8s.io/test-infra/kubetest \
        kubetest "$@" ${geoflag}; } 2>&1 | tee ${cluster_creation_log}

      # Exit if test succeeded
      [[ "$(get_test_return_code)" == "0" ]] && return
      # If test failed not because of cluster creation stockout, return
      [[ -z "$(grep -Eio 'does not have enough resources available' ${cluster_creation_log})" ]] && return
    done
  done
}

# Setup the test cluster for running the tests.
function setup_test_cluster() {
  # Fail fast during setup.
  set -o errexit
  set -o pipefail

  header "Setting up test cluster"

  # Set the actual project the test cluster resides in
  # It will be a project assigned by Boskos if test is running on Prow, 
  # otherwise will be ${GCP_PROJECT} set up by user.
  readonly export E2E_PROJECT_ID="$(gcloud config get-value project)"

  # Save some metadata about cluster creation for using in prow and testgrid
  save_metadata

  local k8s_user=$(gcloud config get-value core/account)
  local k8s_cluster=$(kubectl config current-context)

  # If KO_DOCKER_REPO isn't set, use default.
  if [[ -z "${KO_DOCKER_REPO}" ]]; then
    export KO_DOCKER_REPO=gcr.io/${E2E_PROJECT_ID}/${E2E_BASE_NAME}-e2e-img
  fi

  echo "- Project is ${E2E_PROJECT_ID}"
  echo "- Cluster is ${k8s_cluster}"
  echo "- User is ${k8s_user}"
  echo "- Docker is ${KO_DOCKER_REPO}"

  export KO_DATA_PATH="${REPO_ROOT_DIR}/.git"

  trap teardown_test_resources EXIT

  # Handle failures ourselves, so we can dump useful info.
  set +o errexit
  set +o pipefail

  if (( ! SKIP_TEKTON_SETUP )) && function_exists tekton_setup; then
    tekton_setup || fail_test "Tekton setup failed"
  fi
  if function_exists test_setup; then
    test_setup || fail_test "test setup failed"
  fi
}

# Gets the exit of the test script.
# For more details, see set_test_return_code().
function get_test_return_code() {
  echo $(cat "${TEST_RESULT_FILE}")
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

# Signal (as return code and in the logs) that all E2E tests passed.
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
SKIP_TEKTON_SETUP=0
GCP_PROJECT=""
E2E_SCRIPT=""
E2E_CLUSTER_VERSION=""
EXTRA_CLUSTER_CREATION_FLAGS=()
EXTRA_KUBETEST_FLAGS=()
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
      --skip-tekton-setup) SKIP_TEKTON_SETUP=1 ;;
      *)
        [[ $# -ge 2 ]] || abort "missing parameter after $1"
        shift
        case ${parameter} in
          --gcp-project) GCP_PROJECT=$1 ;;
          --cluster-version) E2E_CLUSTER_VERSION=$1 ;;
          --cluster-creation-flag) EXTRA_CLUSTER_CREATION_FLAGS+=($1) ;;
          --kubetest-flag) EXTRA_KUBETEST_FLAGS+=($1) ;;
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
  is_protected_gcr ${KO_DOCKER_REPO} && \
    abort "\$KO_DOCKER_REPO set to ${KO_DOCKER_REPO}, which is forbidden"

  readonly RUN_TESTS
  readonly EMIT_METRICS
  readonly GCP_PROJECT
  readonly IS_BOSKOS
  readonly EXTRA_CLUSTER_CREATION_FLAGS
  readonly EXTRA_KUBETEST_FLAGS
  readonly SKIP_TEKTON_SETUP

  if (( ! RUN_TESTS )); then
    create_test_cluster
  else
    setup_test_cluster
  fi
}
