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

set -o errexit
set -o nounset
set -o pipefail

source $(dirname $0)/../vendor/github.com/knative/test-infra/scripts/library.sh

readonly TMP_DIFFROOT="$(mktemp -d ${REPO_ROOT_DIR}/tmpdiffroot.XXXXXX)"

cleanup() {
  rm -rf "${TMP_DIFFROOT}"
}

trap "cleanup" EXIT SIGINT

cleanup

# Save working tree state
mkdir -p "${TMP_DIFFROOT}"

cp -aR \
  "${REPO_ROOT_DIR}/Gopkg.lock" \
  "${REPO_ROOT_DIR}/apis" \
  "${REPO_ROOT_DIR}/logging" \
  "${REPO_ROOT_DIR}/testing" \
  "${TMP_DIFFROOT}"

"${REPO_ROOT_DIR}/hack/update-codegen.sh"
echo "Diffing ${REPO_ROOT_DIR} against freshly generated codegen"
ret=0

diff -Naupr --no-dereference \
  "${REPO_ROOT_DIR}/Gopkg.lock" "${TMP_DIFFROOT}/Gopkg.lock" || ret=1

diff -Naupr --no-dereference \
  "${REPO_ROOT_DIR}/apis" "${TMP_DIFFROOT}/apis" || ret=1

diff -Naupr --no-dereference \
  "${REPO_ROOT_DIR}/logging" "${TMP_DIFFROOT}/logging" || ret=1

diff -Naupr --no-dereference \
  "${REPO_ROOT_DIR}/testing" "${TMP_DIFFROOT}/testing" || ret=1

# Restore working tree state
rm -fr \
  "${REPO_ROOT_DIR}/Gopkg.lock" \
  "${REPO_ROOT_DIR}/apis" \
  "${REPO_ROOT_DIR}/logging" \
  "${REPO_ROOT_DIR}/testing"

cp -aR "${TMP_DIFFROOT}"/* "${REPO_ROOT_DIR}"

if [[ $ret -eq 0 ]]
then
  echo "${REPO_ROOT_DIR} up to date."
 else
  echo "${REPO_ROOT_DIR} is out of date. Please run hack/update-codegen.sh"
  exit 1
fi
