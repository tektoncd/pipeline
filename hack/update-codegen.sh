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

source $(git rev-parse --show-toplevel)/hack/setup-temporary-gopath.sh
shim_gopath
trap shim_gopath_clean EXIT

# source $(git rev-parse --show-toplevel)/vendor/github.com/tektoncd/plumbing/scripts/library.sh
source $(git rev-parse --show-toplevel)/vendor/knative.dev/hack/codegen-library.sh

PREFIX=${GOBIN:-${GOPATH}/bin}

OLDGOFLAGS="${GOFLAGS:-}"
GOFLAGS="-mod=vendor"

source "${CODEGEN_PKG}/kube_codegen.sh"

bash ${REPO_ROOT_DIR}/hack/generate-groups.sh "deepcopy,client,informer,lister" \
  github.com/tektoncd/pipeline/pkg/client/resource github.com/tektoncd/pipeline/pkg/apis \
  "resource:v1alpha1" \
  --go-header-file ${REPO_ROOT_DIR}/hack/boilerplate/boilerplate.go.txt

# This generates deepcopy,client,informer and lister for the pipeline package (v1alpha1, v1beta1, and v1)
bash ${REPO_ROOT_DIR}/hack/generate-groups.sh "deepcopy,client,informer,lister" \
  github.com/tektoncd/pipeline/pkg/client github.com/tektoncd/pipeline/pkg/apis \
  "pipeline:v1alpha1,v1beta1,v1" \
  --go-header-file ${REPO_ROOT_DIR}/hack/boilerplate/boilerplate.go.txt
# This generates deepcopy,client,informer and lister for the resolution package (v1alpha1, v1beta1)
bash ${REPO_ROOT_DIR}/hack/generate-groups.sh "deepcopy,client,informer,lister" \
  github.com/tektoncd/pipeline/pkg/client/resolution github.com/tektoncd/pipeline/pkg/apis \
  "resolution:v1alpha1,v1beta1" \
  --go-header-file ${REPO_ROOT_DIR}/hack/boilerplate/boilerplate.go.txt

# Depends on generate-groups.sh to install bin/deepcopy-gen
${PREFIX}/deepcopy-gen \
  --output-file zz_generated.deepcopy.go \
  --go-header-file ${REPO_ROOT_DIR}/hack/boilerplate/boilerplate.go.txt \
  github.com/tektoncd/pipeline/pkg/apis/config

${PREFIX}/deepcopy-gen \
  --output-file zz_generated.deepcopy.go \
  --go-header-file ${REPO_ROOT_DIR}/hack/boilerplate/boilerplate.go.txt \
  github.com/tektoncd/pipeline/pkg/spire/config

${PREFIX}/deepcopy-gen \
  --output-file zz_generated.deepcopy.go \
  --go-header-file ${REPO_ROOT_DIR}/hack/boilerplate/boilerplate.go.txt \
  github.com/tektoncd/pipeline/pkg/apis/config/resolver

${PREFIX}/deepcopy-gen \
  --output-file zz_generated.deepcopy.go \
  --go-header-file ${REPO_ROOT_DIR}/hack/boilerplate/boilerplate.go.txt \
  github.com/tektoncd/pipeline/pkg/apis/pipeline/pod

${PREFIX}/deepcopy-gen \
  --output-file zz_generated.deepcopy.go \
  --go-header-file ${REPO_ROOT_DIR}/hack/boilerplate/boilerplate.go.txt \
  github.com/tektoncd/pipeline/pkg/apis/run/v1alpha1

# Knative Injection
# # This generates the knative injection packages for the resource package (v1alpha1).
# # This is separate from the pipeline package for the same reason as client and all (see above).

${REPO_ROOT_DIR}/vendor/knative.dev/pkg/hack/generate-knative.sh "injection" \
  github.com/tektoncd/pipeline/pkg/client/resource github.com/tektoncd/pipeline/pkg/apis \
  "resource:v1alpha1" \
  --go-header-file "${REPO_ROOT_DIR}/hack/boilerplate/boilerplate.go.txt"

${REPO_ROOT_DIR}/vendor/knative.dev/pkg/hack/generate-knative.sh "injection" \
  github.com/tektoncd/pipeline/pkg/client github.com/tektoncd/pipeline/pkg/apis \
  "pipeline:v1alpha1,v1beta1,v1" \
  --go-header-file "${REPO_ROOT_DIR}/hack/boilerplate/boilerplate.go.txt"

${REPO_ROOT_DIR}/vendor/knative.dev/pkg/hack/generate-knative.sh "injection" \
  github.com/tektoncd/pipeline/pkg/client/resolution github.com/tektoncd/pipeline/pkg/apis \
  "resolution:v1alpha1,v1beta1" \
  --go-header-file "${REPO_ROOT_DIR}/hack/boilerplate/boilerplate.go.txt"

# Make sure our dependencies are up-to-date
${REPO_ROOT_DIR}/hack/update-deps.sh

# Make sure the OpenAPI specification and Swagger file are up-to-date
${REPO_ROOT_DIR}/hack/update-openapigen.sh

# Make sure the generated API reference docs are up-to-date
# ${REPO_ROOT_DIR}/hack/update-reference-docs.sh

# Make sure the CRD's structural OpenAPI schema are up-to-date
${REPO_ROOT_DIR}/hack/update-schemas.sh
