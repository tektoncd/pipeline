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
# and deploy Tekton Pipelines to it for running upgrading tests. There are
# two scenarios we need to cover in this script:

# Scenario 1: install the previous release, upgrade to the current release, and
# validate whether the Tekton pipeline works.

# Scenario 2: install the previous release, create the pipelines and tasks, upgrade
# to the current release, and validate whether the Tekton pipeline works.

source $(git rev-parse --show-toplevel)/test/e2e-common.sh
RELEASES='https://github.com/tektoncd/pipeline/releases/latest'
PREVIOUS_PIPELINE_VERSION=$(curl -L -s -H 'Accept: application/json' $RELEASES |
  sed -e 's/.*"tag_name":"\([^"]*\)".*/\1/')

# Script entry point.

if [ "${SKIP_INITIALIZE}" != "true" ]; then
  initialize $@
fi

header "Setting up environment"

# Handle failures ourselves, so we can dump useful info.
set +o errexit
set +o pipefail

# First, we will verify if Scenario 1 works.
# Install the previous release.
header "Install the previous release of Tekton pipeline $PREVIOUS_PIPELINE_VERSION"
install_pipeline_crd_version $PREVIOUS_PIPELINE_VERSION

# Upgrade to the current release.
header "Upgrade to the current release of Tekton pipeline"
install_pipeline_crd

# Run the integration tests.
failed=0
go_test_e2e -timeout=20m ./test || failed=1

# Run the post-integration tests.
go_test_e2e -tags=examples -timeout=20m ./test/ || failed=1

# Remove all the pipeline CRDs, and clean up the environment for next Scenario.
uninstall_pipeline_crd
uninstall_pipeline_crd_version $PREVIOUS_PIPELINE_VERSION

# Next, we will verify if Scenario 2 works.
# Install the previous release.
header "Install the previous release of Tekton pipeline $PREVIOUS_PIPELINE_VERSION"
install_pipeline_crd_version $PREVIOUS_PIPELINE_VERSION

# Upgrade to the current release.
header "Upgrade to the current release of Tekton pipeline"
install_pipeline_crd

# Run the integration tests.
go_test_e2e -timeout=20m ./test || failed=1

# Run the post-integration tests. We do not need to install the resources again, since
# they are installed before the upgrade. We verify if they still work, after going through
# the upgrade.
go_test_e2e -tags=examples -timeout=20m ./test/ || failed=1

(( failed )) && fail_test

success
