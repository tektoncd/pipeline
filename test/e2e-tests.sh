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

cd $(dirname $(readlink -f $0))/../

ci_run && {
  header "Setting up environment"
  initialize $@
}

# Run the integration tests
ci_run && {
  header "Running Go e2e tests"
  failed=0
  go_test_e2e ./test || failed=1
  (( failed )) && fail_test
}

kubectl get crds|grep tekton\\.dev && fail_test "TektonCD CRDS should not be installed, you should reset them before each runs"

install_pipeline_crd

success
