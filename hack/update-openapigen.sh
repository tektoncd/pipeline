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

echo "Generating OpenAPI specification ..."
go run k8s.io/kube-openapi/cmd/openapi-gen \
    --input-dirs ./pkg/apis/pipeline/v1beta1,./pkg/apis/pipeline/pod,./pkg/apis/resource/v1alpha1,knative.dev/pkg/apis,knative.dev/pkg/apis/duck/v1beta1 \
    --output-package ./pkg/apis/pipeline/v1beta1 -o ./ \
    --go-header-file hack/boilerplate/boilerplate.go.txt \
    -r "${TMP_DIFFROOT}/api-report"

violations=$(diff --changed-group-format='%>' --unchanged-group-format='' "hack/ignored-openapi-violations.list" "${TMP_DIFFROOT}/api-report" || echo "")
if [ -n "${violations}" ]; then
  echo ""
  echo "New API rule violations found which are not present in hack/ignored-openapi-violations.list. Please fix these violations:"
  echo ""
  echo "${violations}"
  echo ""
  exit 1
fi

echo "Generating swagger file ..."
go run hack/spec-gen/main.go v0.17.2 > pkg/apis/pipeline/v1beta1/swagger.json
