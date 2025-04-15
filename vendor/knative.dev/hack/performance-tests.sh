#!/bin/bash

# Copyright 2019 The Knative Authors
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

# This is a helper script for Knative performance test scripts.
# See README.md for instructions on how to use it.

source "$(dirname "${BASH_SOURCE[0]}")"/library.sh

# Configurable parameters.
# If not provided, they will fall back to the default values.
readonly BENCHMARK_ROOT_PATH=${BENCHMARK_ROOT_PATH:-test/performance/benchmarks}
readonly PROJECT_NAME=${PROJECT_NAME:-knative-performance}
readonly SERVICE_ACCOUNT_NAME=${SERVICE_ACCOUNT_NAME:-mako-job@knative-performance.iam.gserviceaccount.com}

# Setup env vars.
export KO_DOCKER_REPO="gcr.io/${PROJECT_NAME}"
# Constants
readonly GOOGLE_APPLICATION_CREDENTIALS="/etc/performance-test/service-account.json"
readonly GITHUB_TOKEN="/etc/performance-test/github-token"
readonly SLACK_READ_TOKEN="/etc/performance-test/slack-read-token"
readonly SLACK_WRITE_TOKEN="/etc/performance-test/slack-write-token"

# Set up the user for cluster operations.
function setup_user() {
  echo ">> Setting up user"
  echo "Using gcloud user ${SERVICE_ACCOUNT_NAME}"
  gcloud config set core/account "${SERVICE_ACCOUNT_NAME}"
  echo "Using gcloud project ${PROJECT_NAME}"
  gcloud config set core/project "${PROJECT_NAME}"
}

# Update resources installed on the cluster.
# Parameters: $1 - cluster name
#             $2 - cluster region/zone
function update_cluster() {
  # --zone option can work with both region and zone, (e.g. us-central1 and
  # us-central1-a), so we don't need to add extra check here.
  gcloud container clusters get-credentials "$1" --zone="$2" --project="${PROJECT_NAME}" || abort "failed to get cluster creds"
  # Set up the configmap to run benchmarks in production
  echo ">> Setting up 'prod' config-mako on cluster $1 in zone $2"
  cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: ConfigMap
metadata:
  name: config-mako
data:
  # This should only be used by our performance automation.
  environment: prod
EOF
  # Create secrets required for running benchmarks on the cluster
  echo ">> Creating secrets on cluster $1 in zone $2"
  kubectl create secret generic mako-secrets \
    --from-file=robot.json=${GOOGLE_APPLICATION_CREDENTIALS} \
    --from-file=github-token=${GITHUB_TOKEN} \
    --from-file=slack-read-token=${SLACK_READ_TOKEN} \
    --from-file=slack-write-token=${SLACK_WRITE_TOKEN}
  # Delete all benchmark jobs to avoid noise in the update process
  echo ">> Deleting all cronjobs and jobs on cluster $1 in zone $2"
  kubectl delete cronjob --all
  kubectl delete job --all
  
  if function_exists update_knative; then
    update_knative || abort "failed to update knative"
  fi
  # get benchmark name from the cluster name
  local benchmark_name
  benchmark_name=$(get_benchmark_name "$1")
  if function_exists update_benchmark; then
    update_benchmark "${benchmark_name}" || abort "failed to update benchmark"
  fi
}

# Get benchmark name from the cluster name.
# Parameters: $1 - cluster name
function get_benchmark_name() {
  # get benchmark_name by removing the prefix from cluster name, e.g. get "load-test" from "serving--load-test"
  echo "${1#$REPO_NAME"--"}"
}

# Update the clusters related to the current repo.
function update_clusters() {
  header "Updating all clusters for ${REPO_NAME}"
  local all_clusters
  all_clusters=$(gcloud container clusters list --project="${PROJECT_NAME}" --format="csv[no-heading](name,zone)")
  echo ">> Project contains clusters:" "${all_clusters}"
  for cluster in ${all_clusters}; do
    local name
    name=$(echo "${cluster}" | cut -f1 -d",")
    # the cluster name is prefixed with "${REPO_NAME}--", here we should only handle clusters belonged to the current repo
    [[ ! ${name} =~ ^${REPO_NAME}-- ]] && continue
    local zone
    zone=$(echo "${cluster}" | cut -f2 -d",")

    # Update all resources installed on the cluster
    update_cluster "${name}" "${zone}"
  done
  header "Done updating all clusters"
}

# Run the perf-tests tool
# Parameters: $1..$n - parameters passed to the tool
function run_perf_cluster_tool() {
  perf-tests "$@"
}

# Delete the old clusters belonged to the current repo, and recreate them with the same configuration.
function recreate_clusters() {
  header "Recreating clusters for ${REPO_NAME}"
  run_perf_cluster_tool --recreate \
    --gcp-project="${PROJECT_NAME}" --repository="${REPO_NAME}" --benchmark-root="${BENCHMARK_ROOT_PATH}" \
    || abort "failed recreating clusters for ${REPO_NAME}"
  header "Done recreating clusters"
  # Update all clusters after they are recreated
  update_clusters
}

# Try to reconcile clusters for benchmarks in the current repo.
# This function will be run as postsubmit jobs.
function reconcile_benchmark_clusters() {
  header "Reconciling clusters for ${REPO_NAME}"
  run_perf_cluster_tool --reconcile \
    --gcp-project="${PROJECT_NAME}" --repository="${REPO_NAME}" --benchmark-root="${BENCHMARK_ROOT_PATH}" \
    || abort "failed reconciling clusters for ${REPO_NAME}"
  header "Done reconciling clusters"
  # For now, do nothing after reconciling the clusters, and the next update_clusters job will automatically 
  # update them. So there will be a period that the newly created clusters are being idle, and the duration
  # can be as long as <update_clusters interval>.
}

# Parse flags and excute the command.
function main() {
  if (( ! IS_PROW )); then
    abort "this script should only be run by Prow since it needs secrets created on Prow cluster"
  fi

  # Set up the user credential for cluster operations
  setup_user || abort "failed to set up user"

  # Try parsing the first flag as a command.  
  case $1 in
    --recreate-clusters) recreate_clusters ;;
    --update-clusters) update_clusters ;;
    --reconcile-benchmark-clusters) reconcile_benchmark_clusters ;;
    *) abort "unknown command $1, must be --recreate-clusters, --update-clusters or --reconcile_benchmark_clusters"
  esac
  shift
}
