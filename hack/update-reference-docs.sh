#!/usr/bin/env bash

# Copyright 2020 The Tekton Authors
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

echo "Generating API reference docs ..."
go run github.com/elastic/crd-ref-docs \
    --config "./hack/reference-docs-gen-config.yaml" \
    --source-path "./pkg/apis" \
    --renderer markdown \
    --max-depth 15 \
    --output-path "./docs/pipeline-api.md"
sed -i".backup" '1s/^/<!--\n---\ntitle: Pipeline API\nlinkTitle: Pipeline API\nweight: 404\n---\n-->\n\n/' ./docs/pipeline-api.md && rm docs/pipeline-api.md.backup
