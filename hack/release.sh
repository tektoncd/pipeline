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

source $(dirname $0)/../vendor/github.com/knative/test-infra/scripts/release.sh

# Local generated yaml file
readonly OUTPUT_YAML=release.yaml

# Script entry point

initialize $@

set -o errexit
set -o pipefail

run_validation_tests ./test/presubmit-tests.sh

# Build the release

banner "Building the release"

# Build should not try to deploy anything, use a bogus value for cluster.
export K8S_CLUSTER_OVERRIDE=CLUSTER_NOT_SET
export K8S_USER_OVERRIDE=USER_NOT_SET
export DOCKER_REPO_OVERRIDE=DOCKER_NOT_SET

ko resolve ${KO_FLAGS} -f config/ > ${OUTPUT_YAML}
tag_images_in_yaml ${OUTPUT_YAML}

echo "New release built successfully"

if (( ! PUBLISH_RELEASE )); then
 exit 0
fi

publish_yaml ${OUTPUT_YAML}

branch_release "Knative Build Pipeline" "${OUTPUT_YAML}"

echo "New release published successfully"
