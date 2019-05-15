#!/usr/bin/env bash

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

# This script calls out to scripts in tektoncd/plumbing to setup a cluster
# and deploy Tekton Pipelines to it for running integration tests.

source $(dirname $0)/e2e-common.sh

# Script entry point.

initialize $@

header "Setting up environment"

# FIXME(vdemeester) To Do later
# install_pipeline_crd

failed=0

# Run the integration tests
header "Running Go e2e tests"
go_test_e2e ./test || failed=1

(( failed )) && fail_test
success
