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

source $(git rev-parse --show-toplevel)/vendor/github.com/tektoncd/plumbing/scripts/library.sh

function fail_test() {
  echo "***************************************"
  echo "***        LOAD TESTS FAILED        ***"
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
  echo "***        LOAD TESTS FAILED        ***"
  echo "***     End of information dump     ***"
  echo "***************************************"
  exit 1
}

function success() {
  echo "**************************************"
  echo "***       LOAD TESTS PASSED        ***"
  echo "**************************************"
  exit 0
}

TIMEOUT="5m"
TASKRUN_NUM=50

while [[ $# -ne 0 ]]; do
  parameter=$1
  [[ $# -ge 2 ]] || abort "missing parameter after $1"
  shift
  case ${parameter} in
    --timeout) TIMEOUT=$1 ;;
    --taskrun-num) TASKRUN_NUM=$1 ;;
    *) abort "unknown option ${parameter}" ;;
  esac
  shift
done

# Script entry point.
header "Running load tests"

report_go_test -tags load -timeout ${TIMEOUT} -taskrun-num ${TASKRUN_NUM} ./test/... || failed=1

(( failed )) && fail_test
success
