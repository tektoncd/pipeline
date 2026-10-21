#!/usr/bin/env bash

# Copyright 2026 The Tekton Authors
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

# Generate .github/dependabot.yml from .github/dependabot.config.yml

set -o errexit
set -o nounset
set -o pipefail

REPO_ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "${REPO_ROOT}"

echo "Generating release branch list from releases.md active releases..."
yq -i '.release-branches = []' .github/dependabot.config.yml
awk '/^## Release$/{found=1; next} found && /^## /{exit} found && /^### v[0-9]+\.[0-9]+ \(LTS\)/{sub(/^### /, "release-"); sub(/ \(LTS\)\s*$/, ".x"); print}' releases.md |
while IFS= read -r branch; do
    yq -i ".release-branches += \"${branch}\"" .github/dependabot.config.yml
done

echo "Generating .github/dependabot.yml..."

# Run the generator using go run
go run ./hack/generate-dependabot.go

echo ""
echo "Done! Review the changes and commit if they look correct:"
echo "  git diff .github/dependabot.yml"
