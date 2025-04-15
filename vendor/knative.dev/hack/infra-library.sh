#!/usr/bin/env bash

# Copyright 2020 The Knative Authors
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

# This is a collection of functions for infra related setups, mainly
# cluster provisioning. It doesn't do anything when called from command line.

source "$(dirname "${BASH_SOURCE[0]:-$0}")/library.sh"

# Default Kubernetes version to use for GKE, if not overridden with
# the `--cluster-version` parameter.
readonly GKE_DEFAULT_CLUSTER_VERSION="1.31"

# Dumps the k8s api server metrics. Spins up a proxy, waits a little bit and
# dumps the metrics to ${ARTIFACTS}/k8s.metrics.txt
function dump_metrics() {
  header ">> Starting kube proxy"
  kubectl proxy --port=8080 &
  local proxy_pid=$!
  sleep 5
  header ">> Grabbing k8s metrics"
  curl -s http://localhost:8080/metrics > "${ARTIFACTS}"/k8s.metrics.txt
  # Clean up proxy so it doesn't interfere with job shutting down
  kill $proxy_pid || true
}

# Dump info about the test cluster. If dump_extra_cluster_info() is defined, calls it too.
# This is intended to be called when a test fails to provide debugging information.
function dump_cluster_state() {
  echo "***************************************"
  echo "***         E2E TEST FAILED         ***"
  echo "***    Start of information dump    ***"
  echo "***************************************"

  local output
  output="${ARTIFACTS}/k8s.dump-$(basename "${E2E_SCRIPT}").txt"
  echo ">>> The dump is located at ${output}"

  for crd in $(kubectl api-resources --verbs=list -o name | sort); do
    local count
    count="$(kubectl get "$crd" --all-namespaces --no-headers 2>/dev/null | wc -l)"
    echo ">>> ${crd} (${count} objects)"
    if [[ "${count}" -gt "0" ]]; then
      {
        echo ">>> ${crd} (${count} objects)"

        echo ">>> Listing"
        kubectl get "${crd}" --all-namespaces

        echo ">>> Details"
        if [[ "${crd}" == "secrets" ]]; then
          echo "Secrets are ignored for security reasons"
        elif [[ "${crd}" == "events" ]]; then
          echo "events are ignored as making a lot of noise"
        else
          kubectl get "${crd}" --all-namespaces -o yaml
        fi
      } >> "${output}"
    fi
  done

  if function_exists dump_extra_cluster_state; then
    echo ">>> Extra dump" >> "${output}"
    dump_extra_cluster_state >> "${output}"
  fi
  echo "***************************************"
  echo "***         E2E TEST FAILED         ***"
  echo "***     End of information dump     ***"
  echo "***************************************"
}

# Create a test cluster and run the tests if provided.
# Parameters: $1 - cluster provider name, e.g. gke
#             $2 - custom flags supported by kntest
#             $3 - test command to run after cluster is created
function create_test_cluster() {
  # Fail fast during setup.
  set -o errexit
  set -o pipefail

  if function_exists cluster_setup; then
    cluster_setup || fail_test "cluster setup failed"
  fi

  case "$1" in
    gke) create_gke_test_cluster "$2" "$3" "$4" "${5:-}" ;;
    kind) create_kind_test_cluster "$2" "$3" "${4:-}" ;;
    *) echo "unsupported provider: $1"; exit 1 ;;
  esac

  local result="$?"
  # Ignore any errors below, this is a best-effort cleanup and shouldn't affect the test result.
  set +o errexit
  set +o pipefail
  function_exists cluster_teardown && cluster_teardown
  echo "Artifacts were written to ${ARTIFACTS}"
  echo "Test result code is ${result}"
  exit "${result}"
}

# Create a KIND test cluster with kubetest2 and run the test command.
# Parameters: $1 - extra cluster creation flags
#             $2 - test command to run by the kubetest2 tester
function create_kind_test_cluster() {
  local -n _custom_flags=$1
  local -n _test_command=$2

  kubetest2 kind "${_custom_flags[@]}" --up --down --test=exec -- "${_test_command[@]}"
}

# Create a GKE test cluster with kubetest2 and run the test command.
# Parameters: $1 - custom flags defined in kubetest2
#             $2 - custom flags to pass directly to gcloud
#             $3 - test command to run after the cluster is created (optional)
function create_gke_test_cluster() {
  local -n _custom_flags=$1
  local -n _extra_gcloud_flags=$2
  local -n _test_command=$3

  # We are disabling logs and metrics on Boskos Clusters by default as they are not used. Manually set ENABLE_GKE_TELEMETRY to true to enable telemetry
  # and ENABLE_PREEMPTIBLE_NODES to true to create preemptible/spot VMs. VM Preemption is a rare event and shouldn't be distruptive given the fault tolerant nature of our tests.
  if [[ "${ENABLE_GKE_TELEMETRY:-}" != "true" ]]; then
    _extra_gcloud_flags+=("--logging=NONE --monitoring=NONE")
  fi

  if [[ "${CLOUD_PROVIDER}" == "gke" ]]; then
      extra_gcloud_flags+=("--addons=NodeLocalDNS")
  fi

  if [[ "${ENABLE_PREEMPTIBLE_NODES:-}" == "true" ]]; then
    _extra_gcloud_flags+=("--preemptible")
  fi

  _extra_gcloud_flags+=("--quiet")
  if ! command -v kubetest2 >/dev/null; then
    tmpbin="$(mktemp -d)"
    echo "kubetest2 not found, installing in temp path: ${tmpbin}"
    GOBIN="$tmpbin" go install sigs.k8s.io/kubetest2/...@latest
    export PATH="${tmpbin}:${PATH}"
  fi
  if [[ ! " ${_custom_flags[*]} " =~ "--machine-type=" ]]; then
      _custom_flags+=("--machine-type=e2-standard-4")
  fi
  if [[ ! " ${_custom_flags[*]} " =~ "--cluster-version=" ]]; then
      _custom_flags+=("--cluster-version=${GKE_DEFAULT_CLUSTER_VERSION}")
  fi
  kubetest2 gke "${_custom_flags[@]}" \
    --rundir-in-artifacts \
    --up \
    --down \
    --boskos-heartbeat-interval-seconds=20 \
    --v=1 \
    --network=e2e-network \
    --boskos-acquire-timeout-seconds=1200 \
    --region="${E2E_CLUSTER_REGION:-us-east1},us-central1,us-west1" \
    --gcloud-extra-flags="${_extra_gcloud_flags[*]}" \
    --retryable-error-patterns='.*does not have enough resources available to fulfill.*,.*only \\d+ nodes out of \\d+ have registered; this is likely due to Nodes failing to start correctly.*,.*All cluster resources were brought up.+ but: component .+ from endpoint .+ is unhealthy.*' \
    --test=exec \
    -- \
    "${_test_command[@]}"
}
