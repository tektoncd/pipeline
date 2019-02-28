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

function build_release() {
  # When building a versioned release, we must use .ko.yaml.release
  if (( PUBLISH_TO_GITHUB )); then
    # KO_CONFIG_PATH expects a path containing a .ko.yaml file
    export KO_CONFIG_PATH="$(mktemp -d)"
    cp .ko.yaml.release "${KO_CONFIG_PATH}/.ko.yaml"
    echo "Using .ko.yaml.release for base image overrides"
  fi
  # Build the base image for creds-init and git images.
  docker build -t "${KO_DOCKER_REPO}/github.com/knative/build-pipeline/build-base" -f images/Dockerfile images/
  echo "Building build-pipeline"
  ko resolve ${KO_FLAGS} -f config/ > ${OUTPUT_YAML}
  YAMLS_TO_PUBLISH="${OUTPUT_YAML}"
}

main $@
