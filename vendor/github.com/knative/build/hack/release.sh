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

set -o errexit
set -o pipefail

source "$(dirname $(readlink -f ${BASH_SOURCE}))/../tests/library.sh"

function cleanup() {
  restore_override_vars
}

cd ${BUILD_ROOT_DIR}
trap cleanup EXIT

install_ko

echo "@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@"
echo "@@@@ RUNNING RELEASE VALIDATION TESTS @@@@"
echo "@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@"

# Run tests.
./tests/presubmit-tests.sh

echo "@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@"
echo "@@@@     BUILDING THE RELEASE    @@@@"
echo "@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@"

# Set the repository to the official one:
export KO_DOCKER_REPO=gcr.io/build-crd

# If this is a prow job, authenticate against GCR.
(( IS_PROW )) && gcr_auth

echo "Building build-crd"
ko resolve -f config/ > release.yaml

echo "Publishing release.yaml"
gsutil cp release.yaml gs://build-crd/latest/release.yaml

echo "New release published successfully"

# TODO(mattmoor): Create other aliases?
