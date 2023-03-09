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

readonly TMP_DIFFROOT="$(mktemp -d ${REPO_ROOT_DIR}/tmpdiffroot.XXXXXX)"

cleanup() {
  rm -rf "${TMP_DIFFROOT}"
}

trap "cleanup" EXIT SIGINT

cleanup

# Save working tree state
mkdir -p "${TMP_DIFFROOT}/pkg"
cp -aR "${REPO_ROOT_DIR}/pkg" "${TMP_DIFFROOT}"

mkdir -p "${TMP_DIFFROOT}/vendor"
cp -aR "${REPO_ROOT_DIR}/vendor" "${TMP_DIFFROOT}"

mkdir -p "${TMP_DIFFROOT}/docs"
cp -aR "${REPO_ROOT_DIR}/docs" "${TMP_DIFFROOT}"

"${REPO_ROOT_DIR}/hack/update-codegen.sh"
echo "Diffing ${REPO_ROOT_DIR} against freshly generated codegen"
ret=0
diff -Naupr "${REPO_ROOT_DIR}/pkg" "${TMP_DIFFROOT}/pkg" || ret=1
diff -Naupr "${REPO_ROOT_DIR}/vendor" "${TMP_DIFFROOT}/vendor" || ret=1
diff -Naupr "${REPO_ROOT_DIR}/docs/pipeline-api.md" "${TMP_DIFFROOT}/docs/pipeline-api.md" || ret=1

# Restore working tree state
rm -fr "${REPO_ROOT_DIR}/pkg"
rm -fr "${REPO_ROOT_DIR}/vendor"
rm -fr "${REPO_ROOT_DIR}/docs"
cp -aR "${TMP_DIFFROOT}"/* "${REPO_ROOT_DIR}"

if [[ $ret -eq 0 ]]
then
  echo "${REPO_ROOT_DIR} up to date."
else
  echo "${REPO_ROOT_DIR} is out of date. Please run hack/update-codegen.sh."
  exit 1
fi
