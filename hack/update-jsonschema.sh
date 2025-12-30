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

set -o errexit
set -o nounset
set -o pipefail

REPO_ROOT_DIR="$(git rev-parse --show-toplevel)"
CRD_DIR="${REPO_ROOT_DIR}/config/300-crds"
OUTPUT_DIR="${REPO_ROOT_DIR}/schema"

echo "Generating JSON Schema files from CRDs..."

# Create output directory
mkdir -p "${OUTPUT_DIR}"

# Run the JSON Schema generator
go run "${REPO_ROOT_DIR}/hack/jsonschema-gen/main.go" \
  -crd-dir="${CRD_DIR}" \
  -output-dir="${OUTPUT_DIR}"

echo "JSON Schema generation complete."
echo "Generated schemas are in: ${OUTPUT_DIR}"
