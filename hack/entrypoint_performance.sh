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
if [[ "$(uname -s)" != Linux ]]; then
  echo "Entrypoint performance checks require native Linux." >&2
  exit 1
fi
base_ref=$(git rev-parse "${1:?Usage: $0 BASE_REF OUTPUT_DIR [test flags]}^{commit}")
output=${2:?Usage: $0 BASE_REF OUTPUT_DIR [test flags]}
shift 2
mkdir -p "$output"
output=$(cd "$output" && pwd)
baseline=$(mktemp -d)
trap 'rm -rf "$baseline"' EXIT
git archive "$base_ref" | tar -x -C "$baseline"

# Force the same toolchain and build settings for both revisions.
export GOTOOLCHAIN
GOTOOLCHAIN=$(go env GOVERSION)
export GOMAXPROCS=4
{
  printf 'base: %s\nhead: %s\n' "$base_ref" "$(git rev-parse HEAD)"
  go version
  go env GOOS GOARCH CGO_ENABLED
  printf 'entrypoint flags: CGO_ENABLED=0 -trimpath -buildvcs=false -ldflags="-s -w"\n'
} | tee "$output/build.txt"
for variant in default disable_spire; do
  tags=()
  if [[ "$variant" == disable_spire ]]; then
    tags=(-tags disable_spire)
  fi
  for revision in baseline candidate; do
    source=.
    if [[ "$revision" == baseline ]]; then
      source=$baseline
    fi
    (
      cd "$source"
      CGO_ENABLED=0 go build -p 4 -trimpath -buildvcs=false -ldflags="-s -w" \
        "${tags[@]}" -o "$output/$revision-$variant" ./cmd/entrypoint
    )
  done
  # Keep this binary test outside test/, whose e2e scanner walks all test files.
  go test -race -shuffle=on -count=1 -v -tags=entrypoint_performance ./hack/entrypoint_performance \
    -args -baseline "$output/baseline-$variant" -candidate "$output/candidate-$variant" \
    -report "$output/$variant.json" "$@" | tee "$output/$variant.txt"
done
