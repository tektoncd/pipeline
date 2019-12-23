#!/usr/bin/env bash

# Copyright 2017 The Kubernetes Authors.
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

# generate-openapi generates and patches the CRDs yamls
(
  # To support running this script from anywhere, we have to first cd into this directory
  # so we can install the tools.
  cd "$(dirname "${0}")"
  command -v ${GOPATH}/bin/controller-gen || go install sigs.k8s.io/controller-tools/cmd/controller-gen
)

MANIFESTS="$1"
APIS_PKG="$2"
GROUPS_WITH_VERSIONS="$3"

function codegen::join() { local IFS="$1"; shift; echo "$*"; }

# enumerate group versions
FQ_APIS=() # e.g. k8s.io/api/apps/v1
for GVs in ${GROUPS_WITH_VERSIONS}; do
  IFS=: read -r G Vs <<<"${GVs}"

  # enumerate versions
  for V in ${Vs//,/ }; do
    FQ_APIS+=("${APIS_PKG}/${G}/${V}")
  done
done

${GOPATH}/bin/controller-gen schemapatch:manifests="${MANIFESTS}" output:dir="${MANIFESTS}" paths="$(codegen::join , "${FQ_APIS[@]}")"
