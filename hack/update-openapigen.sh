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

source $(git rev-parse --show-toplevel)/vendor/github.com/tektoncd/plumbing/scripts/library.sh
source $(git rev-parse --show-toplevel)/hack/setup-temporary-gopath.sh

readonly TMP_DIFFROOT="$(mktemp -d ${REPO_ROOT_DIR}/tmpdiffroot.XXXXXX)"

cleanup() {
  rm -rf "${TMP_DIFFROOT}"
  shim_gopath_clean
}

cleanup

shim_gopath
mkdir -p "${TMP_DIFFROOT}"

trap cleanup EXIT

for APIVERSION in "v1beta1" "v1"
do
  input_dirs=./pkg/apis/pipeline/${APIVERSION},./pkg/apis/pipeline/pod,knative.dev/pkg/apis,knative.dev/pkg/apis/duck/v1beta1
  if [ ${APIVERSION} = "v1beta1" ]
  then
    input_dirs=${input_dirs},./pkg/apis/resolution/v1beta1
  fi

  echo "Generating OpenAPI specification for ${APIVERSION} ..."
  go run k8s.io/kube-openapi/cmd/openapi-gen \
      --input-dirs ${input_dirs} \
      --output-package ./pkg/apis/pipeline/${APIVERSION} -o ./ \
      --go-header-file hack/boilerplate/boilerplate.go.txt \
      -r "${TMP_DIFFROOT}/api-report"

  violations=$(diff --changed-group-format='%>' --unchanged-group-format='' <(sort "hack/ignored-openapi-violations.list") <(sort "${TMP_DIFFROOT}/api-report") || echo "")
  if [ -n "${violations}" ]; then
    echo ""
    echo "ERROR: New API rule violations found which are not present in hack/ignored-openapi-violations.list. Please fix these violations:"
    echo ""
    echo "${violations}"
    echo ""
    exit 1
  fi

  echo "Generating swagger file for ${APIVERSION} ..."
  go run hack/spec-gen/main.go -apiVersion=${APIVERSION} > pkg/apis/pipeline/${APIVERSION}/swagger.json
done
