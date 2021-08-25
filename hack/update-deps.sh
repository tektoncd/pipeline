#!/usr/bin/env bash

# Copyright 2019 The Tekton Authors
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

source $(git rev-parse --show-toplevel)/vendor/github.com/tektoncd/plumbing/scripts/library.sh

cd ${REPO_ROOT_DIR}

VERSION="release-0.23"

# The list of dependencies that we track at HEAD and periodically
# float forward in this repository.
FLOATING_DEPS=(
  "knative.dev/pkg@${VERSION}"
)

# Parse flags to determine any we should pass to dep.
GO_GET=0
while [[ $# -ne 0 ]]; do
  parameter=$1
  case ${parameter} in
    --upgrade) GO_GET=1 ;;
    *) abort "unknown option ${parameter}" ;;
  esac
  shift
done
readonly GO_GET

if (( GO_GET )); then
  go get -d ${FLOATING_DEPS[@]}
fi

# Prune modules.
go mod tidy
go mod vendor

# Applying patches
if [[ -d hack/patches ]];then
    for f in hack/patches/*.patch;do
        [[ -f ${f} ]] || continue
        # Apply patches but do not commit
        git apply ${f}
    done
fi

update_licenses third_party/
