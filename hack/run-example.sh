#!/usr/bin/env bash

# Copyright 2025 The Tekton Authors
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

# This script applies an example YAML file with registry substitution.
# It replaces the hardcoded registry (gcr.io/christiewilson-catfactory)
# with the value from KO_DOCKER_REPO environment variable.
#
# Usage:
#   KO_DOCKER_REPO=kind.local ./hack/run-example.sh examples/v1/pipelineruns/pipelinerun.yaml
#   KO_DOCKER_REPO=localhost:5000 ./hack/run-example.sh examples/v1/taskruns/task-output-image.yaml
#
# This allows you to use local registries and avoid rate limiting issues
# with remote registries.

set -o errexit
set -o nounset
set -o pipefail

# Print usage information
function usage() {
  echo "Usage: $0 <example-yaml-file>"
  echo ""
  echo "Applies a Tekton example YAML file with registry substitution."
  echo ""
  echo "The script replaces 'gcr.io/christiewilson-catfactory' with the value"
  echo "from the KO_DOCKER_REPO environment variable, allowing you to use"
  echo "local registries or different remote registries."
  echo ""
  echo "Environment Variables:"
  echo "  KO_DOCKER_REPO  - The container registry to use (required)"
  echo ""
  echo "Examples:"
  echo "  KO_DOCKER_REPO=kind.local $0 examples/v1/pipelineruns/pipelinerun.yaml"
  echo "  KO_DOCKER_REPO=localhost:5000 $0 examples/v1/taskruns/task-output-image.yaml"
  echo ""
  echo "For setting up a local registry with kind, see:"
  echo "  https://kind.sigs.k8s.io/docs/user/local-registry/"
  exit 1
}

# Check if file argument is provided
if [[ $# -ne 1 ]]; then
  usage
fi

EXAMPLE_FILE="$1"

# Check if the example file exists
if [[ ! -f "${EXAMPLE_FILE}" ]]; then
  echo "error: Example file not found: ${EXAMPLE_FILE}"
  exit 1
fi

# Check if KO_DOCKER_REPO is set
if [[ -z "${KO_DOCKER_REPO:-}" ]]; then
  echo "error: KO_DOCKER_REPO environment variable is not set"
  echo ""
  echo "Please set KO_DOCKER_REPO to your container registry."
  echo "For example:"
  echo "  export KO_DOCKER_REPO=kind.local"
  echo "  export KO_DOCKER_REPO=localhost:5000"
  echo "  export KO_DOCKER_REPO=your-registry.example.com"
  exit 1
fi

echo "Applying ${EXAMPLE_FILE} with registry: ${KO_DOCKER_REPO}"

# Substitute the registry and apply
# Using sed to replace the hardcoded registry with KO_DOCKER_REPO
sed "s|gcr.io/christiewilson-catfactory|${KO_DOCKER_REPO}|g" "${EXAMPLE_FILE}" | kubectl apply -f -

echo "Successfully applied ${EXAMPLE_FILE}"
