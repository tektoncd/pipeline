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

set -euo pipefail

cd "$(dirname "${BASH_SOURCE[0]}")/.."

format='{{range .Imports}}{{$.ImportPath}} -> {{.}}
{{end}}'
for tags in "" "disable_spire"; do
  args=(go list -deps -f "$format")
  if [[ -n "$tags" ]]; then
    args+=(-tags "$tags")
  fi
  edges=$("${args[@]}" ./cmd/entrypoint)
  if forbidden=$(grep -E ' -> (knative[.]dev/|k8s[.]io/client-go(/|$)|github[.]com/spiffe/spire-api-sdk(/|$))' <<<"$edges"); then
    printf 'Forbidden entrypoint dependencies (build tags: %s):\n%s\n' "${tags:-default}" "$forbidden" >&2
    exit 1
  fi
  printf 'Entrypoint dependency check passed (%s).\n' "${tags:-default}"
done
