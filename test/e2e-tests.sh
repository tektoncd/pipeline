#!/usr/bin/env bash

# Copyright 2018 The Tekton Authors
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

# TODO: fix common.sh and  enable set  -euo pipefail
#set -euxo pipefail

source $(dirname $0)/e2e-common.sh

# Script entry point.

#set -euo pipefail

# ci_run allows certain steps to be skipped when running locally
# usage: 
# ci_run && {
#   commands to be run in CI
# }
# 
ci_run()  {
  ${LOCAL_CI_RUN:-false} || return 0
  return 1
}

# skips the test if it fails unless NO_SKIP is set to true
# to skip a test: 
#   run_test "list pipelines" skip ./tkn pipeline list -n default
# to turn off skipping:
#   NO_SKIP=true ./e2e-tests.sh ...
SKIP() { 
  ${NO_SKIP:-false} && {
      $@
      return $?
  }

  $@ || { 
    echo "SKIPPING: $@ returned $?"
    return 0
  } 
}

run_test() {
  local desc="$1"; shift

  echo "Running $@"
  $@ || fail_test "failed to $desc"
  echo 
}

ci_run && {
  header "Setting up environment"
  initialize $@
}

# FIXME(vdemeester) To Do later
# install_pipeline_crd


# Run the integration tests
ci_run && {
  header "Running Go e2e tests"
  local failed=0
  go_test_e2e ./test || failed=1
  (( failed )) && fail_test
}


go build github.com/tektoncd/cli/cmd/tkn

tkn() {
 ./tkn "$@"
}

# run_test "list pipelines" tkn pipeline list

# ### Skipped as the command fails
# run_test "list pipelines in an empty namespace" \
#   SKIP tkn pipeline list -n default

run_test  "list pipelines" SKIP tkn pipeline list
run_test  "describe pipeline" SKIP tkn pipeline describe foo
run_test  "list pipelinerun" SKIP tkn pipelinerun list
run_test  "describe pipelinerun" SKIP tkn pipelinerun describe foo
run_test  "show logs" SKIP tkn pipelinerun logs foo
run_test  "list task" SKIP tkn task list
run_test  "show logs" SKIP tkn taskrun logs foo
run_test  "list taskrun" SKIP tkn taskrun list

# 0| dt"| ut"d$ |0|irun_test| p|li lxxt|d$|0t"| lldwdw
ci_run &&{
    install_pipeline_crd
}
# change default namespace to tektoncd
kubectl create namespace tektoncd
kubectl config set-context $(kubectl config current-context) --namespace=tektoncd

# create pipeline, pipelinerun, task, and taskrun
kubectl apply -f ./test/resources/output-pipelinerun.yaml
kubectl apply -f ./test/resources/task-volume.yaml

wait_till_ready(){
    local notready=true
    while $notready
    do
        sleep 2 
        res=$(kubectl get $@ -o json)
        if [ "$(echo "$res" | jq -r .status.conditions[0].status)" = "True" ]; then
          notready=false
        elif [ "$(echo "$res" | jq -r .status.conditions[0].status)" = "False" ]; then
          exit 1
        fi
    done
}

wait_till_ready pipelinerun/output-pipeline-run
wait_till_ready taskrun/test-template-volume

# commands after install_pipeline
run_test  "list pipelines" tkn pipeline list
run_test  "describe pipeline" tkn pipeline describe output-pipeline
run_test  "list pipelinerun" tkn pipelinerun list
run_test  "describe pipelinerun" tkn pipelinerun describe output-pipeline-run
run_test  "show logs" tkn pipelinerun logs output-pipeline-run
run_test  "list task" tkn task list
run_test  "list taskrun" tkn taskrun list
run_test  "show logs" tkn taskrun logs test-template-volume

success
