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

OLDGOFLAGS="${GOFLAGS:-}"
GOFLAGS=""

CRD_PATH=$(dirname "${0}")/../config/300-crds
API_PATH=$(dirname "${0}")/../pkg/apis

# collect all existing crds into the FILES array using the recommended
# mapfile: https://www.shellcheck.net/wiki/SC2207
mapfile -d '' FILES < <(find "$CRD_PATH" -type f -name '*.yaml' -print0)
for file in "${FILES[@]}"; do
  echo "Generating CRD schema for $file"
  # NOTE: Workaround for https://github.com/kubernetes-sigs/controller-tools/pull/627
  #
  # Tekton splits its CRD definitions into multiple sub-packages. E.g. under
  # the same `tekton.dev/v1alpha1` we find the following packages:
  #  pkg/apis/
  #  ├── pipeline
  #  │   └── v1alpha1
  #  ├── resource
  #  │   └── v1alpha1
  #  └── run
  #      └── v1alpha1
  # This breaks controller-gen's assumption of 1 group/version -> 1 go package.
  # As a workaround, we patch every crd in isolation (tmp dir) with the correct
  # API sub dir path.
  TEMP_DIR=$(mktemp -d)
  cp -p "$file" "$TEMP_DIR"
  case "$(basename "$file" | tr '[:upper:]' '[:lower:]')" in
    *customrun*)
      API_SUBDIR="run" ;;
    *resolutionrequest*)
      API_SUBDIR="resolution" ;;
    *)
      API_SUBDIR="pipeline" ;;
  esac

  # FIXME:(burigolucas): add schema for fields with generic type once supported by controller-tools
  # FIXME:(burigolucas): add schema for recursive fields once supported by controller-tools
  # FIXME:(burigolucas): add reference for dependent external/internal schemas once supported in CRD specification
  go run sigs.k8s.io/controller-tools/cmd/controller-gen@v0.18.0 \
    schemapatch:manifests="$TEMP_DIR",generateEmbeddedObjectMeta=false \
    output:dir="$CRD_PATH" \
    paths="$API_PATH/$API_SUBDIR/..."
  rm -rf "$TEMP_DIR"
done

GOFLAGS="${OLDGOFLAGS}"
